// JSON-RPC 2.0 over postMessage — the view side of the MCP Apps
// view <-> host protocol. Hand-rolled: the official SDK is optional sugar
// per spec, and this keeps the bundle free of dependencies.
//
// Interop: until an MCP Apps host has answered (ui/initialize resolved or any
// host->view method arrived), tool calls and links fall back to the legacy
// community mcp-ui postMessage protocol ({type:"tool"|"link", payload:{...}}),
// so widgets embedded as plain ui:// resources stay actionable in hosts like
// LibreChat that render mcp-ui but do not speak MCP Apps. If such a host sends
// a ui-message-response for our messageId it is used as the tool result;
// otherwise the call resolves as fire-and-forget ({dispatched: true}).
//
// When the caller supplies UIEventMeta, the fallback posts a prompt-type
// action instead, whose text is the UI Interaction Protocol v1 envelope
// (sentinel \uievent + JSON header + tool instruction) — protocol-aware hosts
// render it as an event chip ("You clicked: …") while the model receives the
// instruction; hosts without the protocol show the short readable first line.
import {
  CallToolResult,
  ContentBlock,
  HOST_TO_VIEW_METHODS,
  HostContext,
  InitializeResult,
  JsonRpcMessage,
  M,
  METHOD_NOT_FOUND,
  RequestID,
  RUNTIME_NAME,
  RUNTIME_VERSION,
  SPEC_VERSION,
} from "./protocol";

interface Pending {
  resolve: (v: unknown) => void;
  reject: (e: Error) => void;
  timer: ReturnType<typeof setTimeout>;
}

export class BridgeError extends Error {
  constructor(
    message: string,
    readonly code?: number,
    readonly data?: unknown,
  ) {
    super(message);
    this.name = "BridgeError";
  }
}

export interface BridgeOptions {
  /** Window to talk to; defaults to window.parent. */
  target?: Window;
  /** Per-request timeout in ms; tool calls can be slow. Default 60000. */
  timeoutMs?: number;
  /** How long a legacy mcp-ui dispatch waits for a ui-message-response
   * before resolving fire-and-forget. Default 3000. */
  uiResponseTimeoutMs?: number;
}

type Handler = (params: unknown) => unknown | Promise<unknown>;

/** Legacy mcp-ui action/response message shapes (community standard). */
interface UIActionMessage {
  type: "tool" | "prompt" | "link" | "ui-size-change";
  messageId?: string;
  payload: Record<string, unknown>;
}

/** Display metadata for a UI interaction dispatched over the legacy mcp-ui
 * fallback. Presence switches the dispatch to a prompt-type action carrying
 * the \uievent envelope. */
export interface UIEventMeta {
  /** Human text shown in the chat as the event chip; truncated to 80 chars. */
  label: string;
  /** Picks the chip verb ("You clicked/submitted/selected"). Default "click". */
  kind?: "click" | "submit" | "select";
}

const UIEVENT_LABEL_MAX = 80;

/** UI Interaction Protocol v1 envelope: sentinel + single-line JSON header,
 * then a model instruction naming the tool and its arguments precisely. */
function uiEventEnvelope(
  name: string,
  args: Record<string, unknown>,
  meta: UIEventMeta,
): string {
  const label =
    meta.label.length > UIEVENT_LABEL_MAX
      ? `${meta.label.slice(0, UIEVENT_LABEL_MAX - 1)}…`
      : meta.label;
  const header = JSON.stringify({ v: 1, label, kind: meta.kind ?? "click" });
  return `\\uievent${header}\nCall the tool "${name}" with arguments ${JSON.stringify(args)}.`;
}

interface UIResponseMessage {
  type?: unknown;
  messageId?: unknown;
  payload?: { response?: unknown; error?: unknown };
}

export class Bridge {
  private nextId = 1;
  private hostConfirmed = false;
  private readonly pending = new Map<RequestID, Pending>();
  private readonly pendingUI = new Map<string, Pending>();
  private readonly handlers = new Map<string, Handler[]>();
  private readonly target: Window;
  private readonly timeoutMs: number;
  private readonly uiResponseTimeoutMs: number;
  private readonly listener = (ev: MessageEvent): void => this.onMessage(ev);

  constructor(opts: BridgeOptions = {}) {
    this.target = opts.target ?? window.parent;
    this.timeoutMs = opts.timeoutMs ?? 60_000;
    this.uiResponseTimeoutMs = opts.uiResponseTimeoutMs ?? 3_000;
    window.addEventListener("message", this.listener);
    // Reply to host pings and teardown requests by default; handlers
    // registered later run in addition (their return values are ignored
    // once a response has been sent — first handler wins the response).
    this.on(M.ping, () => ({}));
    this.on(M.resourceTeardown, () => ({}));
  }

  dispose(): void {
    window.removeEventListener("message", this.listener);
    for (const [, p] of this.pending) {
      clearTimeout(p.timer);
      p.reject(new BridgeError("bridge disposed"));
    }
    this.pending.clear();
    for (const [, p] of this.pendingUI) {
      clearTimeout(p.timer);
      p.reject(new BridgeError("bridge disposed"));
    }
    this.pendingUI.clear();
  }

  /** Whether an MCP Apps host has been confirmed on this session. */
  get hasHost(): boolean {
    return this.hostConfirmed;
  }

  /** Registers a handler for a host-to-view notification or request. */
  on(method: string, handler: Handler): () => void {
    const list = this.handlers.get(method) ?? [];
    list.push(handler);
    this.handlers.set(method, list);
    return () => {
      const cur = this.handlers.get(method) ?? [];
      const i = cur.indexOf(handler);
      if (i >= 0) cur.splice(i, 1);
    };
  }

  request<T = unknown>(method: string, params?: unknown): Promise<T> {
    const id = this.nextId++;
    return new Promise<T>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new BridgeError(`request ${method} timed out`));
      }, this.timeoutMs);
      this.pending.set(id, {
        resolve: resolve as (v: unknown) => void,
        reject,
        timer,
      });
      this.post({ jsonrpc: "2.0", id, method, params });
    });
  }

  notify(method: string, params?: unknown): void {
    this.post({ jsonrpc: "2.0", method, params });
  }

  // --- MCP Apps helpers ---

  /** Performs the ui/initialize handshake; resolves with the host context. */
  async initialize(): Promise<HostContext> {
    const res = await this.request<InitializeResult>(M.initialize, {
      appInfo: { name: RUNTIME_NAME, version: RUNTIME_VERSION },
      appCapabilities: {},
      protocolVersion: SPEC_VERSION,
    });
    this.hostConfirmed = true;
    this.notify(M.initialized);
    return res?.hostContext ?? {};
  }

  callTool(
    name: string,
    args: Record<string, unknown>,
    uiEvent?: UIEventMeta,
  ): Promise<CallToolResult> {
    if (!this.hostConfirmed) {
      return this.dispatchUIAction(name, args, uiEvent);
    }
    return this.request<CallToolResult>(M.toolsCall, { name, arguments: args });
  }

  openLink(url: string): Promise<unknown> {
    if (!this.hostConfirmed) {
      this.postUI({ type: "link", payload: { url } });
      return Promise.resolve({});
    }
    return this.request(M.openLink, { url });
  }

  /** Legacy mcp-ui dispatch: post the community-standard action message and
   * wait briefly for a ui-message-response; without one, resolve as a
   * fire-and-forget dispatch (the host turns the action into a follow-up,
   * e.g. a conversation turn — no direct result will arrive). With UIEventMeta
   * the action is prompt-type carrying the \uievent envelope; without, the
   * plain tool-type action is kept for backward compatibility. */
  private dispatchUIAction(
    name: string,
    args: Record<string, unknown>,
    uiEvent?: UIEventMeta,
  ): Promise<CallToolResult> {
    const messageId = `gadget-${this.nextId++}`;
    return new Promise<CallToolResult>((resolve, reject) => {
      const timer = setTimeout(() => {
        this.pendingUI.delete(messageId);
        resolve({
          content: [{ type: "text", text: "Action sent to the host." }],
          dispatched: true,
        });
      }, this.uiResponseTimeoutMs);
      this.pendingUI.set(messageId, {
        resolve: resolve as (v: unknown) => void,
        reject,
        timer,
      });
      this.postUI(
        uiEvent
          ? {
              type: "prompt",
              messageId,
              payload: { prompt: uiEventEnvelope(name, args, uiEvent) },
            }
          : {
              type: "tool",
              messageId,
              payload: { toolName: name, params: args },
            },
      );
    });
  }

  /** Inserts a user message into the host's chat. */
  sendMessage(text: string): Promise<unknown> {
    const content: ContentBlock[] = [{ type: "text", text }];
    return this.request(M.message, { role: "user", content });
  }

  requestDisplayMode(mode: string): Promise<unknown> {
    return this.request(M.requestDisplayMode, { mode });
  }

  updateModelContext(structuredContent: Record<string, unknown>): Promise<unknown> {
    return this.request(M.updateModelContext, { structuredContent });
  }

  sizeChanged(width: number, height: number): void {
    if (!this.hostConfirmed) {
      // Legacy mcp-ui hosts auto-resize the iframe on this message so the
      // widget is always fully visible. Width is deliberately omitted: hosts
      // only apply the dimensions present, and the iframe's responsive CSS
      // width (100%) must win over a fixed pixel width.
      this.postUI({ type: "ui-size-change", payload: { height } });
    }
    this.notify(M.sizeChanged, { width, height });
  }

  // --- transport ---

  private post(msg: JsonRpcMessage): void {
    this.target.postMessage(msg, "*");
  }

  private postUI(msg: UIActionMessage): void {
    this.target.postMessage(msg, "*");
  }

  private onMessage(ev: MessageEvent): void {
    const msg = ev.data as JsonRpcMessage | null;
    if (!msg || typeof msg !== "object") return;
    if (msg.jsonrpc !== "2.0") {
      this.onUIMessage(msg as UIResponseMessage);
      return;
    }

    if (msg.method !== undefined) {
      // Incoming request/notification. Only host->view methods are accepted;
      // anything else is either our own echo (same-window tests) or out of
      // contract, and is ignored.
      if (!HOST_TO_VIEW_METHODS.has(msg.method)) return;
      // A host->view method is proof of a live MCP Apps host, even if the
      // ui/initialize response was lost or is still in flight.
      this.hostConfirmed = true;
      this.dispatch(msg);
      return;
    }

    if (msg.id === undefined) return;
    // Response: only meaningful if we have a matching pending request.
    const p = this.pending.get(msg.id);
    if (!p) return;
    this.pending.delete(msg.id);
    clearTimeout(p.timer);
    if (msg.error) {
      p.reject(new BridgeError(msg.error.message, msg.error.code, msg.error.data));
    } else {
      p.resolve(msg.result);
    }
  }

  /** Handles legacy mcp-ui host replies to a dispatched action. */
  private onUIMessage(msg: UIResponseMessage): void {
    if (msg.type !== "ui-message-response" || typeof msg.messageId !== "string") return;
    const p = this.pendingUI.get(msg.messageId);
    if (!p) return;
    this.pendingUI.delete(msg.messageId);
    clearTimeout(p.timer);
    const payload = msg.payload ?? {};
    if (payload.error !== undefined && payload.error !== null) {
      p.reject(new BridgeError(String(payload.error)));
      return;
    }
    const response = payload.response;
    if (response && typeof response === "object") {
      p.resolve(response as CallToolResult);
    } else {
      p.resolve({
        content: [{ type: "text", text: "Action sent to the host." }],
        dispatched: true,
      });
    }
  }

  private dispatch(msg: JsonRpcMessage): void {
    const handlers = this.handlers.get(msg.method as string) ?? [];
    const isRequest = msg.id !== undefined;

    if (handlers.length === 0) {
      if (isRequest) {
        this.post({
          jsonrpc: "2.0",
          id: msg.id,
          error: { code: METHOD_NOT_FOUND, message: `method not found: ${msg.method}` },
        });
      }
      return;
    }

    let responded = false;
    for (const h of handlers) {
      void Promise.resolve()
        .then(() => h(msg.params))
        .then((result) => {
          if (isRequest && !responded) {
            responded = true;
            this.post({ jsonrpc: "2.0", id: msg.id, result: result ?? {} });
          }
        })
        .catch((err: unknown) => {
          if (isRequest && !responded) {
            responded = true;
            this.post({
              jsonrpc: "2.0",
              id: msg.id,
              error: { code: -32000, message: String(err) },
            });
          }
        });
    }
  }
}
