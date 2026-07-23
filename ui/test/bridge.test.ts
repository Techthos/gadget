import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { Bridge, BridgeError } from "../src/bridge";
import { M, SPEC_VERSION } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

describe("Bridge", () => {
  let host: FakeHost;
  let bridge: Bridge;

  beforeEach(() => {
    host = new FakeHost({
      theme: "dark",
      locale: "de-DE",
      timeZone: "Europe/Berlin",
      styles: { variables: { "--color-background-primary": "#111" } },
    });
    bridge = new Bridge({ timeoutMs: 500 });
  });

  afterEach(() => {
    bridge.dispose();
    host.dispose();
  });

  it("performs the initialize handshake", async () => {
    const ctx = await bridge.initialize();
    expect(ctx.theme).toBe("dark");
    expect(ctx.styles?.variables?.["--color-background-primary"]).toBe("#111");

    await flush();
    const init = host.received(M.initialize);
    expect(init).toHaveLength(1);
    const params = init[0]!.params as Record<string, unknown>;
    expect(params.protocolVersion).toBe(SPEC_VERSION);
    expect(params.appInfo).toMatchObject({ name: "gadget" });
    expect(params.appCapabilities).toEqual({});
    expect(host.received(M.initialized)).toHaveLength(1);
  });

  it("calls tools with {name, arguments} and returns the result", async () => {
    await bridge.initialize();
    host.onToolCall = (name, args) => ({
      structuredContent: { echo: name, args },
    });
    const res = await bridge.callTool("delete_user", { id: 7 });
    expect(res.structuredContent).toEqual({
      echo: "delete_user",
      args: { id: 7 },
    });
  });

  it("rejects with BridgeError on JSON-RPC error responses", async () => {
    await bridge.initialize();
    host.onToolCall = () => {
      throw new Error("boom");
    };
    await expect(bridge.callTool("x", {})).rejects.toBeInstanceOf(BridgeError);
    await expect(bridge.callTool("x", {})).rejects.toMatchObject({
      code: -32000,
    });
  });

  it("times out unanswered requests", async () => {
    host.mute.add(M.openLink);
    const fast = new Bridge({ timeoutMs: 30 });
    try {
      await fast.initialize();
      await expect(fast.openLink("https://example.com")).rejects.toThrow(
        /timed out/,
      );
    } finally {
      fast.dispose();
    }
  });

  it("responds to host ping and resource-teardown requests", async () => {
    const pingID = host.requestToView(M.ping);
    const teardownID = host.requestToView(M.resourceTeardown);
    await flush();
    const ids = host.responses.map((r) => r.id);
    expect(ids).toContain(pingID);
    expect(ids).toContain(teardownID);
    for (const r of host.responses) {
      expect(r.error).toBeUndefined();
    }
  });

  it("dispatches tool-result notifications to handlers", async () => {
    const seen: unknown[] = [];
    bridge.on(M.toolResult, (p) => {
      seen.push(p);
    });
    host.pushToolResult({ structuredContent: { rows: [{ id: 1 }] } });
    await flush();
    expect(seen).toHaveLength(1);
    expect(seen[0]).toMatchObject({
      structuredContent: { rows: [{ id: 1 }] },
    });
  });

  it("sends size-changed notifications with width and height", async () => {
    bridge.sizeChanged(320, 480);
    await flush();
    const sizes = host.received(M.sizeChanged);
    expect(sizes).toHaveLength(1);
    expect(sizes[0]!.params).toEqual({ width: 320, height: 480 });
  });

  it("unsubscribes handlers", async () => {
    const seen: unknown[] = [];
    const off = bridge.on(M.toolResult, (p) => {
      seen.push(p);
    });
    off();
    host.pushToolResult({});
    await flush();
    expect(seen).toHaveLength(0);
  });
});

// Without a confirmed MCP Apps host (no ui/initialize answer, no host->view
// traffic), actions fall back to the legacy community mcp-ui postMessage
// protocol so widgets embedded as plain ui:// resources stay actionable.
describe("Bridge legacy mcp-ui fallback", () => {
  let bridge: Bridge;
  let raw: Array<Record<string, unknown>>;
  const listener = (ev: MessageEvent): void => {
    const msg = ev.data as Record<string, unknown> | null;
    if (msg && typeof msg === "object" && msg.jsonrpc === undefined) {
      raw.push(msg);
    }
  };

  beforeEach(() => {
    raw = [];
    window.addEventListener("message", listener);
    bridge = new Bridge({ timeoutMs: 500, uiResponseTimeoutMs: 30 });
  });

  afterEach(() => {
    bridge.dispose();
    window.removeEventListener("message", listener);
  });

  it("posts an mcp-ui tool action and resolves fire-and-forget", async () => {
    const res = await bridge.callTool("install_app", { repo: "o/app" });
    expect(res.dispatched).toBe(true);
    expect(raw).toHaveLength(1);
    expect(raw[0]).toMatchObject({
      type: "tool",
      payload: { toolName: "install_app", params: { repo: "o/app" } },
    });
    expect(typeof raw[0]!.messageId).toBe("string");
  });

  it("posts a prompt action carrying the \\uievent envelope when UI event metadata is given", async () => {
    const res = await bridge.callTool(
      "install_app",
      { repo: "o/app" },
      { label: "Install o/app", kind: "click" },
    );
    expect(res.dispatched).toBe(true);
    expect(raw).toHaveLength(1);
    expect(raw[0]!.type).toBe("prompt");
    expect(typeof raw[0]!.messageId).toBe("string");
    const prompt = (raw[0]!.payload as { prompt: string }).prompt;
    const [header, instruction] = prompt.split("\n");
    expect(header!.startsWith("\\uievent{")).toBe(true);
    expect(JSON.parse(header!.slice("\\uievent".length))).toEqual({
      v: 1,
      label: "Install o/app",
      kind: "click",
    });
    expect(instruction).toContain('"install_app"');
    expect(instruction).toContain('"repo":"o/app"');
  });

  it("truncates envelope labels to 80 chars and defaults kind to click", async () => {
    await bridge.callTool("t", {}, { label: "x".repeat(100) });
    const prompt = (raw[0]!.payload as { prompt: string }).prompt;
    const header = JSON.parse(prompt.split("\n")[0]!.slice("\\uievent".length)) as {
      v: number;
      label: string;
      kind: string;
    };
    expect(header.kind).toBe("click");
    expect(header.label).toHaveLength(80);
    expect(header.label.endsWith("…")).toBe(true);
  });

  it("resolves with the ui-message-response payload when the host answers", async () => {
    const p = bridge.callTool("x", {});
    await flush();
    const messageId = raw[0]!.messageId as string;
    window.postMessage(
      {
        type: "ui-message-response",
        messageId,
        payload: { response: { structuredContent: { ok: true } } },
      },
      "*",
    );
    const res = await p;
    expect(res.dispatched).toBeUndefined();
    expect(res.structuredContent).toEqual({ ok: true });
  });

  it("rejects when the ui-message-response carries an error", async () => {
    const p = bridge.callTool("x", {});
    await flush();
    const messageId = raw[0]!.messageId as string;
    window.postMessage(
      { type: "ui-message-response", messageId, payload: { error: "denied" } },
      "*",
    );
    await expect(p).rejects.toBeInstanceOf(BridgeError);
  });

  it("posts an mcp-ui link action for openLink", async () => {
    await bridge.openLink("https://example.com");
    await flush();
    expect(raw).toHaveLength(1);
    expect(raw[0]).toMatchObject({
      type: "link",
      payload: { url: "https://example.com" },
    });
  });

  it("posts ui-size-change with height only, and stops once a host is confirmed", async () => {
    bridge.sizeChanged(320, 480);
    await flush();
    expect(raw).toHaveLength(1);
    expect(raw[0]).toMatchObject({ type: "ui-size-change", payload: { height: 480 } });
    expect((raw[0]!.payload as Record<string, unknown>).width).toBeUndefined();

    const host = new FakeHost();
    try {
      await bridge.initialize();
      bridge.sizeChanged(320, 500);
      await flush();
      // Confirmed MCP Apps host: only the JSON-RPC notification is sent.
      expect(raw).toHaveLength(1);
      const sizes = host.received(M.sizeChanged);
      expect(sizes[sizes.length - 1]!.params).toEqual({ width: 320, height: 500 });
    } finally {
      host.dispose();
    }
  });
});
