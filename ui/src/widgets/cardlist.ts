// CardList widget behavior: renders records as cards in a grid, wires
// filter/sort/pagination/selection, and fires per-card and bulk actions as
// MCP tool calls — the same runtime model as the table, laid out as cards.
import type { MountContext } from "../index";
import { HOST_CONTEXT_EVENT } from "../host";
import { Row, rowsFrom } from "../data";
import { clear, delegate, h } from "../dom";
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
import {
	ActionCfg,
	CardTemplateCfg,
	renderCard,
	resolveArgs,
	textOf,
} from "./card-common";

interface CardListCfg {
	widget: string;
	rowsKey: string;
	rowId: string;
	pageSize: number;
	filterable: boolean;
	card: CardTemplateCfg;
	sort?: { key: string; label: string }[];
	defaultSort?: SortSpec;
	selection?: { bulk: ActionCfg[] };
	empty?: { title?: string; body?: string };
	loadTool?: string;
	loadArgs?: Record<string, unknown>;
}

interface CardListState {
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

export function mountCardList(ctx: MountContext): void {
	const cfg = ctx.config as unknown as CardListCfg;
	const { root, bridge } = ctx;

	const gridEl = root.querySelector<HTMLElement>("[data-gadget-cards]");
	if (!gridEl || typeof cfg.card !== "object") return;
	const grid: HTMLElement = gridEl;
	const statusEl = root.querySelector<HTMLElement>("[data-gadget-status]");
	const emptyEl = root.querySelector<HTMLElement>("[data-gadget-empty]");
	const emptyTitleEl = emptyEl?.querySelector("h3") ?? null;
	const emptyTitleDefault = emptyTitleEl?.textContent ?? "";
	const paginationEl = root.querySelector<HTMLElement>("[data-gadget-pagination]");
	const pageInfoEl = root.querySelector<HTMLElement>("[data-gadget-page-info]");
	const bulkEl = root.querySelector<HTMLElement>("[data-gadget-bulk]");
	const bulkCountEl = root.querySelector<HTMLElement>("[data-gadget-bulk-count]");
	const selectAllEl = root.querySelector<HTMLInputElement>("[data-gadget-select-all]");
	const sortSelectEl = root.querySelector<HTMLSelectElement>("[data-gadget-sort-select]");

	const filterKeys = [
		cfg.card.titleKey,
		...(cfg.card.subtitleKey ? [cfg.card.subtitleKey] : []),
		...cfg.card.fields.map((f) => f.key),
	].filter((k) => k !== "");
	const rowID = (row: Row): string => String(row[cfg.rowId] ?? "");

	const store = new Store<CardListState>({
		rows: rowsFrom(ctx.initialData, cfg.rowsKey),
		sort: cfg.defaultSort ?? null,
		filter: "",
		page: 0,
		selected: [],
		status: "idle",
	});

	let statusTimer: ReturnType<typeof setTimeout> | undefined;

	function visible(s: CardListState): { pageRows: Row[]; total: number } {
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

	function applyResult(res: CallToolResult): void {
		const patch: Partial<CardListState> = { status: "idle" };
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
			applyResult(await bridge.callTool(action.tool, resolveArgs(action, row, selectedRows())));
		} catch (e) {
			store.set({
				status: "idle",
				statusKind: "error",
				statusMsg: e instanceof Error ? e.message : String(e),
			});
		}
	}

	// Native confirm() is silently disabled in sandboxed MCP Apps iframes, so
	// confirmation is a two-phase button: first click arms it and shows the
	// confirm text, a second click within the window fires.
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

	function render(s: CardListState): void {
		const { pageRows, total } = visible(s);
		const busy = s.status === "loading";
		const selected = new Set(s.selected);

		clear(grid);
		for (const row of pageRows) {
			grid.append(
				renderCard(cfg.card, row, {
					id: rowID(row),
					selectable: !!cfg.selection,
					selected: selected.has(rowID(row)),
					busy,
				}),
			);
		}

		// sort control
		if (sortSelectEl) {
			sortSelectEl.value = s.sort ? `${s.sort.key}|${s.sort.desc ? "desc" : "asc"}` : "";
		}

		// empty state
		if (emptyEl) {
			emptyEl.hidden = total > 0;
			if (emptyTitleEl) {
				emptyTitleEl.textContent =
					total === 0 && s.rows.length > 0 ? "No matching cards" : emptyTitleDefault;
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

	if (sortSelectEl) {
		sortSelectEl.addEventListener("change", () => {
			const v = sortSelectEl.value;
			if (v === "") {
				store.set({ sort: null, page: 0 });
				return;
			}
			const sep = v.lastIndexOf("|");
			const key = v.slice(0, sep);
			const desc = v.slice(sep + 1) === "desc";
			store.set({ sort: { key, desc }, page: 0 });
		});
	}

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

	delegate(root, "change", "select-card", (el) => {
		const id = el.closest("[data-gadget-card-id]")?.getAttribute("data-gadget-card-id");
		if (id === null || id === undefined) return;
		const sel = new Set(store.get().selected);
		if ((el as HTMLInputElement).checked) sel.add(id);
		else sel.delete(id);
		store.set({ selected: [...sel] });
	});

	delegate(root, "change", "select-all", (el) => {
		const s = store.get();
		const sel = new Set(s.selected);
		const { pageRows } = visible(s);
		if ((el as HTMLInputElement).checked) {
			for (const r of pageRows) sel.add(rowID(r));
		} else {
			for (const r of pageRows) sel.delete(rowID(r));
		}
		store.set({ selected: [...sel] });
	});

	delegate(root, "click", "action", (el, value) => {
		const action = cfg.card.actions?.[Number(value)];
		if (!action) return;
		const id = el.closest("[data-gadget-card-id]")?.getAttribute("data-gadget-card-id");
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

	// Load-time hydration: once a host is connected, fetch fresh records and
	// replace the baked snapshot. Silent on success and on failure.
	async function hydrate(): Promise<void> {
		store.set({ status: "loading", statusKind: undefined, statusMsg: "Loading…" });
		try {
			const res = await bridge.callTool(cfg.loadTool as string, cfg.loadArgs ?? {});
			const patch: Partial<CardListState> = {
				status: "idle",
				statusKind: undefined,
				statusMsg: undefined,
			};
			if (res.structuredContent && cfg.rowsKey in res.structuredContent) {
				patch.rows = rowsFrom(res.structuredContent, cfg.rowsKey);
				patch.selected = [];
			}
			store.set(patch);
		} catch {
			store.set({ status: "idle", statusKind: undefined, statusMsg: undefined });
		}
	}
	if (cfg.loadTool) {
		void ctx.ready?.then((ok) => {
			if (ok) void hydrate();
		});
	}
}
