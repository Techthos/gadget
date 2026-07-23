// JSON-RPC 2.0 over postMessage — the view side of the MCP Apps
// view <-> host protocol. Hand-rolled: the official SDK is optional sugar
// per spec, and this keeps the bundle free of dependencies.
//
// MCP Apps only: no legacy mcp-ui interop. Hosts are expected to speak the
// standard (ui/initialize handshake, tools/call, ui/notifications/*).
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
}

type Handler = (params: unknown) => unknown | Promise<unknown>;

export class Bridge {
  private nextId = 1;
  private hostConfirmed = false;
  private readonly pending = new Map<RequestID, Pending>();
  private readonly handlers = new Map<string, Handler[]>();
  private readonly target: Window;
  private readonly timeoutMs: number;
  private readonly listener = (ev: MessageEvent): void => this.onMessage(ev);

  constructor(opts: BridgeOptions = {}) {
    this.target = opts.target ?? window.parent;
    this.timeoutMs = opts.timeoutMs ?? 60_000;
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

  callTool(name: string, args: Record<string, unknown>): Promise<CallToolResult> {
    return this.request<CallToolResult>(M.toolsCall, { name, arguments: args });
  }

  openLink(url: string): Promise<unknown> {
    return this.request(M.openLink, { url });
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
    this.notify(M.sizeChanged, { width, height });
  }

  // --- transport ---

  private post(msg: JsonRpcMessage): void {
    this.target.postMessage(msg, "*");
  }

  private onMessage(ev: MessageEvent): void {
    const msg = ev.data as JsonRpcMessage | null;
    if (!msg || typeof msg !== "object" || msg.jsonrpc !== "2.0") return;

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
