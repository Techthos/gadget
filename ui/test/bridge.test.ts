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
