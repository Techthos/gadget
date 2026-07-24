import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { Bridge } from "../src/bridge";
import { mountCard } from "../src/widgets/card";
import { mountCardList } from "../src/widgets/cardlist";
import { M } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

const TEMPLATE = {
	titleKey: "name",
	subtitleKey: "email",
	badge: {
		key: "status",
		label: "Status",
		type: "badge",
		sortable: false,
		badge: { active: "success", banned: "danger" },
	},
	fields: [
		{ key: "balance", label: "Balance", type: "number", sortable: true, format: "currency:EUR" },
		{ key: "website", label: "Website", type: "link", sortable: false, link: { hrefKey: "website" } },
	],
	actions: [
		{ label: "Edit", kind: "tool", tool: "edit_user", args: { id: { row: "id" } } },
		{ label: "Delete", kind: "tool", tool: "delete_user", confirm: "Really delete?", args: { id: { row: "id" } } },
	],
};

const ROWS = [
	{ id: 1, name: "Carol", email: "carol@x.io", status: "active", balance: 30, website: "https://c.io" },
	{ id: 2, name: "Alice", email: "alice@x.io", status: "banned", balance: 25, website: "" },
	{ id: 3, name: "Bob", email: "bob@x.io", status: "active", balance: 35, website: "" },
];

// --- CardList ---

function listShell({ selection = false, bulk = 0, sort = true } = {}): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gadget-root";
	root.setAttribute("data-gadget-widget", "cardlist");
	root.innerHTML = `
    <div class="gadget-toolbar">
      ${selection ? `<label><input type="checkbox" data-gadget-select-all=""></label>` : ""}
      <input type="search" data-gadget-filter="">
      ${
			sort
				? `<select data-gadget-sort-select="">
             <option value="">Sort…</option>
             <option value="balance|asc">Balance ↑</option>
             <option value="balance|desc">Balance ↓</option>
           </select>`
				: ""
		}
      ${
			bulk > 0
				? `<div data-gadget-bulk="" hidden><span data-gadget-bulk-count=""></span>` +
					Array.from({ length: bulk }, (_, i) => `<button type="button" data-gadget-bulk-action="${i}">Bulk${i}</button>`).join("") +
					`</div>`
				: ""
		}
    </div>
    <div data-gadget-status="" hidden></div>
    <div class="gadget-card-grid" data-gadget-cards=""></div>
    <div data-gadget-empty="" hidden><h3>No records yet</h3></div>
    <div data-gadget-pagination="" hidden>
      <button type="button" data-gadget-page="prev">Prev</button>
      <span data-gadget-page-info=""></span>
      <button type="button" data-gadget-page="next">Next</button>
    </div>`;
	document.body.append(root);
	return root;
}

function listConfig(over: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		widget: "cardlist",
		rowsKey: "rows",
		rowId: "id",
		pageSize: 0,
		filterable: true,
		card: TEMPLATE,
		sort: [{ key: "balance", label: "Balance" }],
		...over,
	};
}

function titles(root: HTMLElement): string[] {
	return [...root.querySelectorAll(".gadget-card-title")].map((e) => e.textContent ?? "");
}

describe("cardlist behavior", () => {
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

	it("renders cards from the data island", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });
		expect(root.querySelectorAll(".gadget-card-item")).toHaveLength(3);
		expect(titles(root)).toEqual(["Carol", "Alice", "Bob"]);
		expect(root.querySelector(".gadget-card-subtitle")?.textContent).toBe("carol@x.io");
	});

	it("shows the empty state when there are no rows", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: null, bridge });
		expect(root.querySelectorAll(".gadget-card-item")).toHaveLength(0);
		expect(root.querySelector<HTMLElement>("[data-gadget-empty]")?.hidden).toBe(false);
	});

	it("renders badge and link fields", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: [ROWS[0]] }, bridge });
		const badge = root.querySelector(".gadget-card-badge .gadget-badge")!;
		expect(badge.textContent).toBe("active");
		expect(badge.className).toContain("gadget-badge--success");
		const link = root.querySelector<HTMLElement>("[data-gadget-link]")!;
		expect(link.getAttribute("data-gadget-link")).toBe("https://c.io");
	});

	it("sorts via the sort select", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });
		const sel = root.querySelector<HTMLSelectElement>("[data-gadget-sort-select]")!;
		sel.value = "balance|asc";
		sel.dispatchEvent(new Event("change", { bubbles: true }));
		expect(titles(root)).toEqual(["Alice", "Carol", "Bob"]);
		sel.value = "balance|desc";
		sel.dispatchEvent(new Event("change", { bubbles: true }));
		expect(titles(root)).toEqual(["Bob", "Carol", "Alice"]);
	});

	it("applies a default sort from config", () => {
		const root = listShell();
		mountCardList({
			root,
			config: listConfig({ defaultSort: { key: "balance", desc: true } }),
			initialData: { rows: ROWS },
			bridge,
		});
		expect(titles(root)).toEqual(["Bob", "Carol", "Alice"]);
		expect(root.querySelector<HTMLSelectElement>("[data-gadget-sort-select]")?.value).toBe("balance|desc");
	});

	it("filters across title, subtitle, and fields after the debounce", async () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });
		const input = root.querySelector<HTMLInputElement>("[data-gadget-filter]")!;
		input.value = "ali";
		input.dispatchEvent(new Event("input", { bubbles: true }));
		await new Promise((r) => setTimeout(r, 250));
		expect(titles(root)).toEqual(["Alice"]);
	});

	it("paginates and updates the page info", () => {
		const root = listShell();
		mountCardList({ root, config: listConfig({ pageSize: 2 }), initialData: { rows: ROWS }, bridge });
		expect(root.querySelectorAll(".gadget-card-item")).toHaveLength(2);
		const info = root.querySelector("[data-gadget-page-info]")!;
		expect(info.textContent).toBe("1–2 of 3");
		root.querySelector<HTMLElement>('[data-gadget-page="next"]')!.click();
		expect(root.querySelectorAll(".gadget-card-item")).toHaveLength(1);
		expect(info.textContent).toBe("3–3 of 3");
	});

	it("fires per-card actions with FromRow args and applies returned rows", async () => {
		const root = listShell();
		host.onToolCall = (_name, args) => ({
			content: [{ type: "text", text: "Deleted." }],
			structuredContent: { rows: ROWS.filter((r) => r.id !== args.id) },
		});
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });

		// First card (Carol, id 1), action index 1 = Delete (has confirm).
		const delBtn = root
			.querySelector(".gadget-card-item")!
			.querySelector<HTMLElement>('[data-gadget-action="1"]')!;
		delBtn.click(); // arms
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(0);
		expect(delBtn.textContent).toBe("Really delete?");
		delBtn.click(); // fires
		await flush();
		const calls = host.received(M.toolsCall);
		expect(calls).toHaveLength(1);
		expect(calls[0]!.params).toMatchObject({ name: "delete_user", arguments: { id: 1 } });
		expect(root.querySelectorAll(".gadget-card-item")).toHaveLength(2);
		expect(root.querySelector<HTMLElement>("[data-gadget-status]")?.textContent).toBe("Deleted.");
	});

	it("selects cards, shows bulk actions, and resolves FromSelection args", async () => {
		const root = listShell({ selection: true, bulk: 1 });
		const cfg = listConfig({
			selection: { bulk: [{ label: "Archive", kind: "tool", tool: "archive_users", args: { ids: { selection: "id" } } }] },
		});
		host.onToolCall = () => ({ structuredContent: { rows: [] } });
		mountCardList({ root, config: cfg, initialData: { rows: ROWS }, bridge });

		const bulkBar = root.querySelector<HTMLElement>("[data-gadget-bulk]")!;
		expect(bulkBar.hidden).toBe(true);

		const selectAll = root.querySelector<HTMLInputElement>("[data-gadget-select-all]")!;
		selectAll.checked = true;
		selectAll.dispatchEvent(new Event("change", { bubbles: true }));
		expect(bulkBar.hidden).toBe(false);
		expect(root.querySelector("[data-gadget-bulk-count]")?.textContent).toBe("3 selected");

		root.querySelector<HTMLElement>('[data-gadget-bulk-action="0"]')!.click();
		await flush();
		const calls = host.received(M.toolsCall);
		expect(calls).toHaveLength(1);
		expect(calls[0]!.params).toMatchObject({ name: "archive_users", arguments: { ids: [1, 2, 3] } });
		expect(root.querySelectorAll(".gadget-card-item")).toHaveLength(0);
		expect(bulkBar.hidden).toBe(true);
	});

	it("updates from tool-result notifications", async () => {
		const root = listShell();
		mountCardList({ root, config: listConfig(), initialData: { rows: ROWS }, bridge });
		host.pushToolResult({ structuredContent: { rows: [{ id: 9, name: "Zoe", email: "z@x.io", status: "active", balance: 1, website: "" }] } });
		await flush();
		expect(titles(root)).toEqual(["Zoe"]);
	});

	it("hydrates from loadTool on mount, replacing the baked snapshot", async () => {
		const root = listShell();
		host.onToolCall = (name) =>
			name === "list_users"
				? { structuredContent: { rows: [{ id: 9, name: "Zed", email: "z@x.io", status: "active", balance: 40, website: "" }] } }
				: { structuredContent: {} };
		mountCardList({
			root,
			config: listConfig({ loadTool: "list_users", loadArgs: { scope: "all" } }),
			initialData: { rows: ROWS },
			bridge,
			ready: Promise.resolve(true),
		});
		expect(titles(root)).toEqual(["Carol", "Alice", "Bob"]);
		await flush();
		const calls = host.received(M.toolsCall);
		expect(calls).toHaveLength(1);
		expect(calls[0]!.params).toMatchObject({ name: "list_users", arguments: { scope: "all" } });
		expect(titles(root)).toEqual(["Zed"]);
	});
});

// --- Card (single) ---

function cardShell(): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gadget-root";
	root.setAttribute("data-gadget-widget", "card");
	root.innerHTML = `
    <div data-gadget-status="" hidden></div>
    <div class="gadget-card-host" data-gadget-card=""></div>
    <div data-gadget-empty="" hidden><h3>Nothing</h3></div>`;
	document.body.append(root);
	return root;
}

function cardConfig(over: Record<string, unknown> = {}): Record<string, unknown> {
	return { widget: "card", rowsKey: "rows", rowId: "id", card: TEMPLATE, ...over };
}

describe("card behavior", () => {
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

	it("renders the first row as a card", () => {
		const root = cardShell();
		mountCard({ root, config: cardConfig(), initialData: { rows: ROWS }, bridge });
		expect(root.querySelectorAll(".gadget-card-item")).toHaveLength(1);
		expect(root.querySelector(".gadget-card-title")?.textContent).toBe("Carol");
		expect(root.querySelector<HTMLElement>("[data-gadget-empty]")?.hidden).toBe(true);
	});

	it("shows the empty state with no record", () => {
		const root = cardShell();
		mountCard({ root, config: cardConfig(), initialData: null, bridge });
		expect(root.querySelectorAll(".gadget-card-item")).toHaveLength(0);
		expect(root.querySelector<HTMLElement>("[data-gadget-empty]")?.hidden).toBe(false);
	});

	it("fires a card action with FromRow args and applies the result", async () => {
		const root = cardShell();
		host.onToolCall = () => ({
			content: [{ type: "text", text: "Saved." }],
			structuredContent: { rows: [{ ...ROWS[0], name: "Caroline" }] },
		});
		mountCard({ root, config: cardConfig(), initialData: { rows: ROWS }, bridge });

		root.querySelector<HTMLElement>('[data-gadget-action="0"]')!.click(); // Edit (no confirm)
		await flush();
		const calls = host.received(M.toolsCall);
		expect(calls).toHaveLength(1);
		expect(calls[0]!.params).toMatchObject({ name: "edit_user", arguments: { id: 1 } });
		expect(root.querySelector(".gadget-card-title")?.textContent).toBe("Caroline");
		expect(root.querySelector<HTMLElement>("[data-gadget-status]")?.textContent).toBe("Saved.");
	});

	it("updates from tool-result notifications", async () => {
		const root = cardShell();
		mountCard({ root, config: cardConfig(), initialData: { rows: ROWS }, bridge });
		host.pushToolResult({ structuredContent: { rows: [{ id: 5, name: "Dave", email: "d@x.io", status: "active", balance: 9, website: "" }] } });
		await flush();
		expect(root.querySelector(".gadget-card-title")?.textContent).toBe("Dave");
	});

	it("hydrates from loadTool on mount", async () => {
		const root = cardShell();
		host.onToolCall = () => ({ structuredContent: { rows: [{ id: 7, name: "Fetched", email: "f@x.io", status: "active", balance: 1, website: "" }] } });
		mountCard({
			root,
			config: cardConfig({ loadTool: "get_user", loadArgs: { id: 7 } }),
			initialData: { rows: ROWS },
			bridge,
			ready: Promise.resolve(true),
		});
		expect(root.querySelector(".gadget-card-title")?.textContent).toBe("Carol");
		await flush();
		expect(host.received(M.toolsCall)).toHaveLength(1);
		expect(root.querySelector(".gadget-card-title")?.textContent).toBe("Fetched");
	});
});
