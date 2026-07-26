// Menu widget behavior: a launcher grid. Every tile is already in the markup
// (Go renders it from the authored items), so this only maps a click to the
// tool call recorded at the same index in the config island.
//
// A menu item is navigation: the host answers the call by opening the tool's
// own widget, replacing or stacking this view. So nothing is rendered from
// the result — only progress while the call is in flight, and the failure if
// it comes back as one.
//
// An item carrying a prompt navigates through the host's chat instead: the
// host posts the prompt as a user turn (ui/message) and the model makes the
// call, which is the only path that opens anything on hosts that run a
// view-initiated tools/call out of band.
import type { MountContext } from "../index";
import { delegate } from "../dom";
import { M } from "../protocol";
import { errorText, textOf } from "../status";

interface MenuItemCfg {
	tool: string;
	args?: Record<string, unknown>;
	prompt?: string;
}

interface MenuCfg {
	widget: string;
	items: MenuItemCfg[];
}

export function mountMenu(ctx: MountContext): void {
	const cfg = ctx.config as unknown as MenuCfg;
	const { root, bridge } = ctx;

	const items = Array.isArray(cfg.items) ? cfg.items : [];
	const tiles = [...root.querySelectorAll<HTMLButtonElement>("[data-gomu-menu-item]")];
	const statusEl = root.querySelector<HTMLElement>("[data-gomu-status]");

	let busy = false;

	function showStatus(kind: "loading" | "error" | "", msg: string): void {
		if (!statusEl) return;
		statusEl.hidden = msg === "";
		statusEl.textContent = msg;
		statusEl.className = "gomu-status" + (kind ? ` gomu-status--${kind}` : "");
	}

	// The whole menu goes inert during a call: a second tile would race the
	// first one's view swap.
	function setBusy(value: boolean): void {
		busy = value;
		for (const tile of tiles) tile.disabled = value;
	}

	function labelOf(tile: HTMLElement): string {
		return tile.querySelector(".gomu-menu-label")?.textContent ?? "this";
	}

	async function open(item: MenuItemCfg, label: string): Promise<void> {
		if (busy || !item.tool) return;
		setBusy(true);
		showStatus("loading", `Opening ${label}…`);
		try {
			// ui/message carries no tool result: the request resolving means the
			// host accepted the turn, and what opens is the model's answer to it.
			if (item.prompt) {
				await bridge.sendMessage(item.prompt);
				setBusy(false);
				showStatus("", "");
				return;
			}
			const res = await bridge.callTool(item.tool, item.args ?? {});
			setBusy(false);
			if (res.isError) {
				showStatus("error", textOf(res) ?? `Could not open ${label}.`);
			} else {
				showStatus("", "");
			}
		} catch (e) {
			setBusy(false);
			showStatus("error", errorText(e, `Could not open ${label}.`));
		}
	}

	delegate(root, "click", "menu-item", (el, value) => {
		const item = items[Number(value)];
		if (item) void open(item, labelOf(el));
	});

	// A host that keeps this view alive still pushes the tool lifecycle
	// notifications; either one means the call is no longer ours to wait on.
	bridge.on(M.toolResult, () => {
		setBusy(false);
		showStatus("", "");
	});
	bridge.on(M.toolCancelled, () => {
		setBusy(false);
		showStatus("", "");
	});
}
