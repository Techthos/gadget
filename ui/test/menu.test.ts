import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { Bridge } from "../src/bridge";
import { mountMenu } from "../src/widgets/menu";
import { M } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

// Mirrors what Go renders: tiles carry their index, the config island carries
// the tool call at the matching index.
function menuShell(): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gadget-root";
	root.setAttribute("data-gadget-widget", "menu");
	root.innerHTML = `
    <div class="gadget-toolbar"><h2 class="gadget-title">Demo app</h2></div>
    <div data-gadget-status="" hidden></div>
    <div class="gadget-menu" data-gadget-menu="">
      <button type="button" class="gadget-menu-item" data-gadget-menu-item="0">
        <span class="gadget-menu-label">Users</span>
        <span class="gadget-menu-desc">Browse the directory.</span>
      </button>
      <button type="button" class="gadget-menu-item" data-gadget-menu-item="1">
        <span class="gadget-menu-label">Edit Ada</span>
      </button>
    </div>`;
	document.body.append(root);
	return root;
}

function menuConfig(over: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		widget: "menu",
		items: [{ tool: "list_users" }, { tool: "edit_user", args: { id: 1 } }],
		...over,
	};
}

function tiles(root: HTMLElement): HTMLButtonElement[] {
	return [...root.querySelectorAll<HTMLButtonElement>("[data-gadget-menu-item]")];
}

function status(root: HTMLElement): HTMLElement {
	return root.querySelector<HTMLElement>("[data-gadget-status]")!;
}

describe("menu behavior", () => {
	let host: FakeHost;
	let bridge: Bridge;

	beforeEach(async () => {
		host = new FakeHost();
		bridge = new Bridge({ timeoutMs: 500 });
		await bridge.initialize();
		host.requests.length = 0;
	});

	afterEach(() => {
		bridge.dispose();
		host.dispose();
		document.body.innerHTML = "";
	});

	it("calls the tool at the clicked tile's index", async () => {
		const root = menuShell();
		mountMenu({ root, config: menuConfig(), initialData: null, bridge });

		tiles(root)[0]!.click();
		await flush();
		expect(host.received(M.toolsCall)[0]!.params).toMatchObject({
			name: "list_users",
			arguments: {},
		});
	});

	it("passes the item's static args", async () => {
		const root = menuShell();
		mountMenu({ root, config: menuConfig(), initialData: null, bridge });

		tiles(root)[1]!.click();
		await flush();
		const calls = host.received(M.toolsCall);
		expect(calls).toHaveLength(1);
		expect(calls[0]!.params).toMatchObject({ name: "edit_user", arguments: { id: 1 } });
	});

	it("disables every tile while a call is in flight, then re-enables", async () => {
		const root = menuShell();
		let release: (() => void) | undefined;
		host.onToolCall = () =>
			new Promise((resolve) => {
				release = () => resolve({ structuredContent: {} });
			});
		mountMenu({ root, config: menuConfig(), initialData: null, bridge });

		tiles(root)[0]!.click();
		await flush();
		expect(tiles(root).every((t) => t.disabled)).toBe(true);
		expect(status(root).hidden).toBe(false);
		expect(status(root).textContent).toBe("Opening Users…");
		expect(status(root).className).toContain("gadget-status--loading");

		release!();
		await flush();
		expect(tiles(root).some((t) => t.disabled)).toBe(false);
		expect(status(root).hidden).toBe(true);
	});

	it("ignores a second tile while the first call is pending", async () => {
		const root = menuShell();
		host.onToolCall = () => new Promise(() => {}); // never settles
		mountMenu({ root, config: menuConfig(), initialData: null, bridge });

		tiles(root)[0]!.click();
		await flush();
		tiles(root)[1]!.dispatchEvent(new MouseEvent("click", { bubbles: true }));
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(1);
	});

	it("surfaces a tool error and leaves the menu usable", async () => {
		const root = menuShell();
		host.onToolCall = () => ({
			isError: true,
			content: [{ type: "text", text: "Tool unavailable." }],
		});
		mountMenu({ root, config: menuConfig(), initialData: null, bridge });

		tiles(root)[0]!.click();
		await flush();
		expect(status(root).textContent).toBe("Tool unavailable.");
		expect(status(root).className).toContain("gadget-status--error");
		expect(tiles(root).some((t) => t.disabled)).toBe(false);
	});

	it("clears progress when the host pushes the tool result", async () => {
		const root = menuShell();
		host.onToolCall = () => new Promise(() => {}); // host answers by notification
		mountMenu({ root, config: menuConfig(), initialData: null, bridge });

		tiles(root)[0]!.click();
		await flush();
		expect(tiles(root).every((t) => t.disabled)).toBe(true);

		host.pushToolResult({ structuredContent: {} });
		await flush();
		expect(tiles(root).some((t) => t.disabled)).toBe(false);
		expect(status(root).hidden).toBe(true);
	});

	it("does nothing for a tile with no matching config entry", async () => {
		const root = menuShell();
		mountMenu({ root, config: menuConfig({ items: [{ tool: "list_users" }] }), initialData: null, bridge });

		tiles(root)[1]!.click();
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(0);
	});
});
