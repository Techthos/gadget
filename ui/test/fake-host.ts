// FakeHost implements the host side of the MCP Apps postMessage protocol
// for tests. In jsdom, window.parent === window, so host and view share one
// window; direction discipline (disjoint view->host / host->view method
// sets) keeps the two sides from consuming their own traffic.
import {
  CallToolResult,
  HostContext,
  JsonRpcMessage,
  M,
  SPEC_VERSION,
} from "../src/protocol";

const VIEW_TO_HOST = new Set<string>([
  M.initialize,
  M.initialized,
  M.toolsCall,
  M.resourcesRead,
  M.openLink,
  M.message,
  M.requestDisplayMode,
  M.updateModelContext,
  M.sizeChanged,
  M.log,
]);

/** Waits for queued postMessage deliveries and promise chains. */
export async function flush(rounds = 4): Promise<void> {
  for (let i = 0; i < rounds; i++) {
    await new Promise((r) => setTimeout(r, 0));
  }
}

export class FakeHost {
  /** Every view->host message seen, in order. */
  requests: JsonRpcMessage[] = [];
  /** Responses to host-initiated requests (requestToView). */
  responses: JsonRpcMessage[] = [];
  /** Methods the host should NOT answer (for timeout tests). */
  mute = new Set<string>();

  hostContext: HostContext;
  onToolCall: (
    name: string,
    args: Record<string, unknown>,
  ) => CallToolResult | Promise<CallToolResult> = () => ({ structuredContent: {} });

  private nextId = 1000;
  private readonly listener = (ev: MessageEvent): void => this.onMessage(ev);

  constructor(hostContext: HostContext = {}) {
    this.hostContext = hostContext;
    window.addEventListener("message", this.listener);
  }

  dispose(): void {
    window.removeEventListener("message", this.listener);
  }

  received(method: string): JsonRpcMessage[] {
    return this.requests.filter((r) => r.method === method);
  }

  notify(method: string, params?: unknown): void {
    this.post({ method, params });
  }

  requestToView(method: string, params?: unknown): number {
    const id = this.nextId++;
    this.post({ id, method, params });
    return id;
  }

  pushToolResult(result: CallToolResult): void {
    this.notify(M.toolResult, result);
  }

  changeHostContext(ctx: HostContext): void {
    this.notify(M.hostContextChanged, ctx);
  }

  private post(msg: Omit<JsonRpcMessage, "jsonrpc">): void {
    window.postMessage({ jsonrpc: "2.0", ...msg }, "*");
  }

  private onMessage(ev: MessageEvent): void {
    const msg = ev.data as JsonRpcMessage | null;
    if (!msg || typeof msg !== "object" || msg.jsonrpc !== "2.0") return;

    if (msg.method === undefined) {
      // Response traffic: track only replies to host-initiated ids.
      if (typeof msg.id === "number" && msg.id >= 1000) this.responses.push(msg);
      return;
    }
    if (!VIEW_TO_HOST.has(msg.method)) return; // own echo — ignore

    this.requests.push(msg);
    if (msg.id === undefined || this.mute.has(msg.method)) return;

    switch (msg.method) {
      case M.initialize:
        this.post({
          id: msg.id,
          result: { protocolVersion: SPEC_VERSION, hostContext: this.hostContext },
        });
        break;
      case M.toolsCall: {
        const p = msg.params as { name: string; arguments: Record<string, unknown> };
        Promise.resolve()
          .then(() => this.onToolCall(p.name, p.arguments))
          .then(
          (r) => this.post({ id: msg.id, result: r }),
          (e: unknown) =>
            this.post({ id: msg.id, error: { code: -32000, message: String(e) } }),
        );
        break;
      }
      default:
        this.post({ id: msg.id, result: {} });
    }
  }
}
