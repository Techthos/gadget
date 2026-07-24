// Card widget behavior: renders a single record (the first row under
// RowsKey) as a card, fires its actions as MCP tool calls, and updates from
// tool-result notifications. A read-only-ish detail view; no filter/sort/
// pagination/selection.
import type { MountContext } from "../index";
import { HOST_CONTEXT_EVENT } from "../host";
import { Row, rowsFrom } from "../data";
import { clear } from "../dom";
import { CallToolResult, M } from "../protocol";
import { ActionCfg, CardTemplateCfg, renderCard, resolveArgs, textOf } from "./card-common";

interface CardCfg {
	widget: string;
	rowsKey: string;
	rowId: string;
	card: CardTemplateCfg;
	empty?: { title?: string; body?: string };
	loadTool?: string;
	loadArgs?: Record<string, unknown>;
}

const CONFIRM_RESET_MS = 4000;
const STATUS_CLEAR_MS = 4000;

export function mountCard(ctx: MountContext): void {
	const cfg = ctx.config as unknown as CardCfg;
	const { root, bridge } = ctx;

	const hostEl = root.querySelector<HTMLElement>("[data-gadget-card]");
	if (!hostEl || typeof cfg.card !== "object") return;
	const host: HTMLElement = hostEl;
	const statusEl = root.querySelector<HTMLElement>("[data-gadget-status]");
	const emptyEl = root.querySelector<HTMLElement>("[data-gadget-empty]");

	const rowID = (row: Row): string => String(row[cfg.rowId] ?? "");

	let row: Row | null = rowsFrom(ctx.initialData, cfg.rowsKey)[0] ?? null;
	let busy = false;
	let statusTimer: ReturnType<typeof setTimeout> | undefined;

	function showStatus(kind: "loading" | "error" | "success" | "", msg: string): void {
		if (!statusEl) return;
		statusEl.hidden = msg === "";
		statusEl.textContent = msg;
		statusEl.className = "gadget-status" + (kind ? ` gadget-status--${kind}` : "");
	}

	function render(): void {
		clear(host);
		if (row) {
			host.append(renderCard(cfg.card, row, { id: rowID(row), busy }));
		}
		if (emptyEl) emptyEl.hidden = !!row;
	}

	function applyResult(res: CallToolResult): void {
		busy = false;
		if (res.structuredContent && cfg.rowsKey in res.structuredContent) {
			row = rowsFrom(res.structuredContent, cfg.rowsKey)[0] ?? null;
		}
		render();
		if (res.isError) {
			showStatus("error", textOf(res) ?? "The action failed.");
		} else {
			showStatus("success", textOf(res) ?? "Done");
			clearTimeout(statusTimer);
			statusTimer = setTimeout(() => showStatus("", ""), STATUS_CLEAR_MS);
		}
	}

	async function fire(action: ActionCfg): Promise<void> {
		if (action.kind === "link") {
			const href = row?.[action.hrefKey ?? ""];
			if (typeof href === "string" && href !== "") void bridge.openLink(href);
			return;
		}
		if (!action.tool) return;
		clearTimeout(statusTimer);
		busy = true;
		render();
		showStatus("loading", "Working…");
		try {
			applyResult(await bridge.callTool(action.tool, resolveArgs(action, row, [])));
		} catch (e) {
			busy = false;
			render();
			showStatus("error", e instanceof Error ? e.message : String(e));
		}
	}

	// Native confirm() is silently disabled in sandboxed MCP Apps iframes;
	// confirmation is a two-phase button (arm, then fire on a second click).
	function armOrFire(btn: HTMLElement, action: ActionCfg): void {
		if (action.confirm && !btn.hasAttribute("data-gadget-armed")) {
			const original = btn.textContent;
			btn.setAttribute("data-gadget-armed", "");
			btn.textContent = action.confirm;
			setTimeout(() => {
				btn.removeAttribute("data-gadget-armed");
				btn.textContent = original;
			}, CONFIRM_RESET_MS);
			return;
		}
		void fire(action);
	}

	host.addEventListener("click", (ev) => {
		const target = ev.target;
		if (!(target instanceof Element)) return;
		const btn = target.closest<HTMLElement>("[data-gadget-action]");
		if (btn && host.contains(btn)) {
			const action = cfg.card.actions?.[Number(btn.getAttribute("data-gadget-action"))];
			if (action) armOrFire(btn, action);
			return;
		}
		const link = target.closest<HTMLElement>("[data-gadget-link]");
		if (link && host.contains(link)) {
			const href = link.getAttribute("data-gadget-link") ?? "";
			if (href !== "") void bridge.openLink(href);
		}
	});

	// Host-pushed results and re-render on host context (Intl locale/timeZone).
	bridge.on(M.toolInput, () => {
		busy = true;
		render();
		showStatus("loading", "Loading…");
	});
	bridge.on(M.toolResult, (params) => applyResult((params ?? {}) as CallToolResult));
	bridge.on(M.toolCancelled, () => {
		busy = false;
		render();
		showStatus("", "");
	});
	document.addEventListener(HOST_CONTEXT_EVENT, render);

	render();

	// Load-time hydration: once a host is connected, fetch the fresh record
	// and replace the baked snapshot. Silent on success and on failure.
	if (cfg.loadTool) {
		void ctx.ready?.then((ok) => {
			if (!ok) return;
			busy = true;
			render();
			showStatus("loading", "Loading…");
			bridge.callTool(cfg.loadTool as string, cfg.loadArgs ?? {}).then(
				(res) => {
					busy = false;
					if (res.structuredContent && cfg.rowsKey in res.structuredContent) {
						row = rowsFrom(res.structuredContent, cfg.rowsKey)[0] ?? null;
					}
					render();
					showStatus("", "");
				},
				() => {
					busy = false;
					render();
					showStatus("", "");
				},
			);
		});
	}
}
