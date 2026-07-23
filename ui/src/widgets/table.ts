// Table widget behavior: renders rows from state, wires sort/filter/
// pagination/selection, and fires row/bulk actions as MCP tool calls.
import type { MountContext } from "../index";
import { HOST_CONTEXT_EVENT } from "../host";
import { Row, rowsFrom } from "../data";
import { clear, delegate, h } from "../dom";
import { formatCell } from "../format";
import { CallToolResult, M } from "../protocol";
import {
	clampPage,
	filterRows,
	pageCount,
	pageSlice,
	SortSpec,
	sortRows,
	Store,
} from "../state";

interface ArgSourceCfg {
	static?: unknown;
	row?: string;
	selection?: string;
}

interface ActionCfg {
	label: string;
	kind: "tool" | "link";
	tool?: string;
	args?: Record<string, ArgSourceCfg>;
	hrefKey?: string;
	confirm?: string;
	variant?: string;
}

interface ColumnCfg {
	key: string;
	label: string;
	type: string;
	sortable: boolean;
	align?: string;
	format?: string;
	badge?: Record<string, string>;
	link?: { hrefKey: string; textKey?: string; text?: string };
	actions?: ActionCfg[];
}

interface TableCfg {
	widget: string;
	rowsKey: string;
	rowId: string;
	pageSize: number;
	filterable: boolean;
	columns: ColumnCfg[];
	defaultSort?: SortSpec;
	selection?: { bulk: ActionCfg[] };
	empty?: { title?: string; body?: string };
}

interface TableState {
	rows: Row[];
	sort: SortSpec | null;
	filter: string;
	page: number;
	selected: string[];
	status: "idle" | "loading";
	statusKind?: "error" | "success";
	statusMsg?: string;
}

const CONFIRM_RESET_MS = 4000;
const STATUS_CLEAR_MS = 4000;
const FILTER_DEBOUNCE_MS = 150;

export function mountTable(ctx: MountContext): void {
	const cfg = ctx.config as unknown as TableCfg;
	const { root, bridge } = ctx;

	const tbodyEl = root.querySelector<HTMLElement>("[data-gadget-rows]");
	if (!tbodyEl || !Array.isArray(cfg.columns)) return;
	const tbody: HTMLElement = tbodyEl;
	const statusEl = root.querySelector<HTMLElement>("[data-gadget-status]");
	const emptyEl = root.querySelector<HTMLElement>("[data-gadget-empty]");
	const emptyTitleEl = emptyEl?.querySelector("h3") ?? null;
	const emptyTitleDefault = emptyTitleEl?.textContent ?? "";
	const paginationEl = root.querySelector<HTMLElement>("[data-gadget-pagination]");
	const pageInfoEl = root.querySelector<HTMLElement>("[data-gadget-page-info]");
	const bulkEl = root.querySelector<HTMLElement>("[data-gadget-bulk]");
	const bulkCountEl = root.querySelector<HTMLElement>("[data-gadget-bulk-count]");
	const selectAllEl = root.querySelector<HTMLInputElement>("[data-gadget-select-all]");

	const filterKeys = cfg.columns.map((c) => c.key).filter((k) => k !== "");
	const rowID = (row: Row): string => String(row[cfg.rowId] ?? "");

	const store = new Store<TableState>({
		rows: rowsFrom(ctx.initialData, cfg.rowsKey),
		sort: cfg.defaultSort ?? null,
		filter: "",
		page: 0,
		selected: [],
		status: "idle",
	});

	let statusTimer: ReturnType<typeof setTimeout> | undefined;

	function visible(s: TableState): { pageRows: Row[]; total: number } {
		const filtered = sortRows(filterRows(s.rows, s.filter, filterKeys), s.sort);
		return {
			pageRows: pageSlice(filtered, s.page, cfg.pageSize),
			total: filtered.length,
		};
	}

	function selectedRows(): Row[] {
		const ids = new Set(store.get().selected);
		return store.get().rows.filter((r) => ids.has(rowID(r)));
	}

	// --- actions ---

	function resolveArgs(action: ActionCfg, row: Row | null): Record<string, unknown> {
		const out: Record<string, unknown> = {};
		for (const [name, src] of Object.entries(action.args ?? {})) {
			if ("static" in src) {
				out[name] = src.static;
			} else if (src.row !== undefined) {
				out[name] = row?.[src.row];
			} else if (src.selection !== undefined) {
				const field = src.selection;
				out[name] = selectedRows().map((r) => r[field]);
			}
		}
		return out;
	}

	function textOf(res: CallToolResult): string | undefined {
		for (const block of res.content ?? []) {
			if (block.type === "text" && typeof block.text === "string") {
				return block.text;
			}
		}
		return undefined;
	}

	function applyResult(res: CallToolResult): void {
		const patch: Partial<TableState> = { status: "idle" };
		if (res.structuredContent && cfg.rowsKey in res.structuredContent) {
			patch.rows = rowsFrom(res.structuredContent, cfg.rowsKey);
			patch.selected = [];
		}
		if (res.isError) {
			patch.statusKind = "error";
			patch.statusMsg = textOf(res) ?? "The action failed.";
		} else {
			patch.statusKind = "success";
			patch.statusMsg = textOf(res) ?? "Done";
			clearTimeout(statusTimer);
			statusTimer = setTimeout(() => {
				store.set({ statusKind: undefined, statusMsg: undefined });
			}, STATUS_CLEAR_MS);
		}
		store.set(patch);
	}

	async function fire(action: ActionCfg, row: Row | null): Promise<void> {
		if (action.kind === "link") {
			const href = row?.[action.hrefKey ?? ""];
			if (typeof href === "string" && href !== "") void bridge.openLink(href);
			return;
		}
		if (!action.tool) return;
		clearTimeout(statusTimer);
		store.set({ status: "loading", statusKind: undefined, statusMsg: "Working…" });
		try {
			applyResult(await bridge.callTool(action.tool, resolveArgs(action, row)));
		} catch (e) {
			store.set({
				status: "idle",
				statusKind: "error",
				statusMsg: e instanceof Error ? e.message : String(e),
			});
		}
	}

	// Native confirm() is silently disabled in sandboxed MCP Apps iframes,
	// so confirmation is a two-phase button: first click arms it and shows
	// the confirm text, a second click within the window fires.
	function armOrFire(btn: HTMLElement, action: ActionCfg, row: Row | null): void {
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
		void fire(action, row);
	}

	// --- rendering ---

	function actionButton(action: ActionCfg, attr: string, value: string, busy: boolean): HTMLElement {
		let cls = "gadget-btn";
		if (action.variant) cls += ` gadget-btn--${action.variant}`;
		return h(
			"button",
			{ type: "button", class: cls, [`data-gadget-${attr}`]: value, disabled: busy },
			action.label,
		);
	}

	function cellFor(col: ColumnCfg, colIdx: number, row: Row, busy: boolean): HTMLElement {
		const alignCls = col.align ? ` gadget-align-${col.align}` : "";
		switch (col.type) {
			case "badge": {
				const value = String(row[col.key] ?? "");
				const variant = col.badge?.[value];
				const cls = "gadget-badge" + (variant && variant !== "neutral" ? ` gadget-badge--${variant}` : "");
				return h("td", { class: alignCls || null }, value === "" ? "" : h("span", { class: cls }, value));
			}
			case "link": {
				const href = row[col.link?.hrefKey ?? col.key];
				if (typeof href !== "string" || href === "") return h("td", {});
				const textKey = col.link?.textKey;
				const text =
					(textKey !== undefined ? String(row[textKey] ?? "") : "") ||
					col.link?.text ||
					href;
				return h("td", { class: alignCls || null },
					h("button", { type: "button", class: "gadget-link", "data-gadget-link": href }, text),
				);
			}
			case "actions": {
				const td = h("td", { class: "gadget-td-actions" });
				(col.actions ?? []).forEach((a, actIdx) => {
					td.append(actionButton(a, "action", `${colIdx}:${actIdx}`, busy));
				});
				return td;
			}
			default:
				return h("td", { class: alignCls || null }, formatCell(row[col.key], col.type, col.format));
		}
	}

	function render(s: TableState): void {
		const { pageRows, total } = visible(s);
		const busy = s.status === "loading";
		const selected = new Set(s.selected);

		// rows
		clear(tbody);
		for (const row of pageRows) {
			const tr = h("tr", { "data-gadget-row-id": rowID(row) });
			if (cfg.selection) {
				tr.append(
					h("td", { class: "gadget-td-select" },
						h("input", {
							type: "checkbox",
							"data-gadget-select-row": "",
							"aria-label": "Select row",
						}),
					),
				);
				const cb = tr.querySelector<HTMLInputElement>("input");
				if (cb) cb.checked = selected.has(rowID(row));
			}
			cfg.columns.forEach((col, i) => tr.append(cellFor(col, i, row, busy)));
			tbody.append(tr);
		}

		// sort indicators
		for (const th of root.querySelectorAll<HTMLElement>("th[data-gadget-sort]")) {
			const key = th.getAttribute("data-gadget-sort");
			th.setAttribute(
				"aria-sort",
				s.sort && s.sort.key === key ? (s.sort.desc ? "descending" : "ascending") : "none",
			);
		}

		// empty state
		if (emptyEl) {
			emptyEl.hidden = total > 0;
			if (emptyTitleEl) {
				emptyTitleEl.textContent =
					total === 0 && s.rows.length > 0 ? "No matching rows" : emptyTitleDefault;
			}
		}

		// pagination
		const pages = pageCount(total, cfg.pageSize);
		if (paginationEl) {
			paginationEl.hidden = cfg.pageSize <= 0 || pages <= 1;
			if (pageInfoEl) {
				const from = total === 0 ? 0 : s.page * cfg.pageSize + 1;
				const to = Math.min((s.page + 1) * cfg.pageSize, total);
				pageInfoEl.textContent = `${from}–${to} of ${total}`;
			}
			for (const btn of paginationEl.querySelectorAll<HTMLButtonElement>("[data-gadget-page]")) {
				const dir = btn.getAttribute("data-gadget-page");
				btn.disabled = busy || (dir === "prev" ? s.page <= 0 : s.page >= pages - 1);
			}
		}

		// selection
		if (selectAllEl) {
			selectAllEl.checked = pageRows.length > 0 && pageRows.every((r) => selected.has(rowID(r)));
		}
		if (bulkEl) {
			bulkEl.hidden = selected.size === 0;
			if (bulkCountEl) bulkCountEl.textContent = `${selected.size} selected`;
			for (const btn of bulkEl.querySelectorAll<HTMLButtonElement>("[data-gadget-bulk-action]")) {
				btn.disabled = busy;
			}
		}

		// status
		if (statusEl) {
			const msg = s.statusMsg ?? "";
			statusEl.hidden = msg === "";
			statusEl.textContent = msg;
			statusEl.className = "gadget-status";
			if (busy) statusEl.className += " gadget-status--loading";
			else if (s.statusKind) statusEl.className += ` gadget-status--${s.statusKind}`;
		}
	}

	// --- events ---

	delegate(root, "click", "sort", (_el, key) => {
		const s = store.get().sort;
		store.set({
			sort: { key, desc: s?.key === key ? !s.desc : false },
			page: 0,
		});
	});

	let filterTimer: ReturnType<typeof setTimeout> | undefined;
	delegate(root, "input", "filter", (el) => {
		clearTimeout(filterTimer);
		filterTimer = setTimeout(() => {
			store.set({ filter: (el as HTMLInputElement).value, page: 0 });
		}, FILTER_DEBOUNCE_MS);
	});

	delegate(root, "click", "page", (_el, dir) => {
		const s = store.get();
		const total = filterRows(s.rows, s.filter, filterKeys).length;
		const next = dir === "prev" ? s.page - 1 : s.page + 1;
		store.set({ page: clampPage(next, total, cfg.pageSize) });
	});

	delegate(root, "change", "select-row", (el) => {
		const id = el.closest("tr")?.getAttribute("data-gadget-row-id");
		if (id === null || id === undefined) return;
		const selected = new Set(store.get().selected);
		if ((el as HTMLInputElement).checked) selected.add(id);
		else selected.delete(id);
		store.set({ selected: [...selected] });
	});

	delegate(root, "change", "select-all", (el) => {
		const s = store.get();
		const selected = new Set(s.selected);
		const { pageRows } = visible(s);
		if ((el as HTMLInputElement).checked) {
			for (const r of pageRows) selected.add(rowID(r));
		} else {
			for (const r of pageRows) selected.delete(rowID(r));
		}
		store.set({ selected: [...selected] });
	});

	delegate(root, "click", "action", (el, value) => {
		const [colIdx, actIdx] = value.split(":").map(Number);
		const action = cfg.columns[colIdx ?? -1]?.actions?.[actIdx ?? -1];
		if (!action) return;
		const id = el.closest("tr")?.getAttribute("data-gadget-row-id");
		const row = store.get().rows.find((r) => rowID(r) === id) ?? null;
		armOrFire(el, action, row);
	});

	delegate(root, "click", "bulk-action", (el, value) => {
		const action = cfg.selection?.bulk[Number(value)];
		if (action) armOrFire(el, action, null);
	});

	delegate(root, "click", "link", (_el, href) => {
		if (href !== "") void bridge.openLink(href);
	});

	// --- host notifications ---

	bridge.on(M.toolInput, () => {
		clearTimeout(statusTimer);
		store.set({ status: "loading", statusKind: undefined, statusMsg: "Loading…" });
	});
	bridge.on(M.toolResult, (params) => {
		applyResult((params ?? {}) as CallToolResult);
	});
	bridge.on(M.toolCancelled, () => {
		store.set({ status: "idle", statusKind: undefined, statusMsg: undefined });
	});

	// Re-render when a host context lands: Intl formatting depends on the
	// host's locale/timeZone, which may arrive after the first paint.
	document.addEventListener(HOST_CONTEXT_EVENT, () => render(store.get()));

	store.subscribe(render);
	render(store.get());
}
