import { afterEach, beforeEach, describe, expect, it } from "vitest";
import { Bridge } from "../src/bridge";
import { mountTable } from "../src/widgets/table";
import { M } from "../src/protocol";
import { FakeHost, flush } from "./fake-host";

function shell({ selection = false, bulk = false, pageSizes = [] as number[] } = {}): HTMLElement {
  document.body.innerHTML = "";
  const root = document.createElement("div");
  root.className = "gadget-root";
  root.setAttribute("data-gadget-widget", "table");
  root.innerHTML = `
    <div class="gadget-toolbar">
      <input type="search" data-gadget-filter="">
      ${
        bulk
          ? `<div data-gadget-bulk="" hidden><span data-gadget-bulk-count=""></span>` +
            `<button type="button" data-gadget-bulk-menu="" aria-haspopup="menu" aria-expanded="false">Actions</button>` +
            `</div>`
          : ""
      }
    </div>
    <div data-gadget-status="" hidden></div>
    <div class="gadget-table-sort"><select data-gadget-sort-select="">
      <option value="">Sort…</option>
      <option value="name|asc">Name ↑</option>
      <option value="name|desc">Name ↓</option>
    </select></div>
    <div class="gadget-table-wrap"><table class="gadget-table" role="table"><thead><tr>
      ${selection ? `<th><input type="checkbox" data-gadget-select-all=""></th>` : ""}
      <th aria-sort="none" data-gadget-sort="name"><button type="button">Name</button></th>
      <th aria-sort="none" data-gadget-sort="age"><button type="button">Age</button></th>
    </tr></thead><tbody data-gadget-rows=""></tbody></table></div>
    <div data-gadget-empty="" hidden><h3>No records yet</h3></div>
    <div data-gadget-pagination="" hidden>
      ${
        pageSizes.length > 0
          ? `<div class="gadget-page-size"><span>Per page</span><select data-gadget-page-size="">` +
            pageSizes.map((n) => `<option value="${n}">${n}</option>`).join("") +
            `</select></div>`
          : ""
      }
      <button type="button" data-gadget-page="prev">Prev</button>
      <span data-gadget-page-info=""></span>
      <button type="button" data-gadget-page="next">Next</button>
    </div>`;
  document.body.append(root);
  return root;
}

/** Opens an action menu and returns its items, in order. */
function openMenu(root: HTMLElement, selector: string): HTMLElement[] {
  root.querySelector<HTMLElement>(selector)!.click();
  return [...root.querySelectorAll<HTMLElement>(".gadget-action-panel [data-gadget-action-index]")];
}

function config(over: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    widget: "table",
    rowsKey: "rows",
    rowId: "id",
    pageSize: 0,
    filterable: true,
    columns: [
      { key: "name", label: "Name", type: "text", sortable: true },
      { key: "age", label: "Age", type: "number", sortable: true },
    ],
    ...over,
  };
}

const ROWS = [
  { id: 1, name: "Carol", age: 30 },
  { id: 2, name: "Alice", age: 25 },
  { id: 3, name: "Bob", age: 35 },
];

function cellTexts(root: HTMLElement, col: number): string[] {
  return [...root.querySelectorAll("tbody tr")].map(
    (tr) => tr.querySelectorAll("td")[col]?.textContent ?? "",
  );
}

describe("table behavior", () => {
  let host: FakeHost;
  let bridge: Bridge;

  beforeEach(async () => {
    host = new FakeHost();
    bridge = new Bridge({ timeoutMs: 500 });
    // Mirrors boot(): initialize runs before user clicks.
    await bridge.initialize();
    host.requests.length = 0;
  });

  afterEach(() => {
    bridge.dispose();
    host.dispose();
    document.body.innerHTML = "";
  });

  // jsdom has no layout, so the two widths the decision rests on are stubbed.
  it("stacks rows only when the columns do not fit the wrap", async () => {
    const root = shell();
    const wrap = root.querySelector<HTMLElement>(".gadget-table-wrap") as HTMLElement;
    const table = root.querySelector<HTMLElement>(".gadget-table") as HTMLElement;
    let needed = 300;
    Object.defineProperty(wrap, "clientWidth", { configurable: true, get: () => 500 });
    Object.defineProperty(table, "scrollWidth", { configurable: true, get: () => needed });

    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    expect(root.hasAttribute("data-gadget-stacked")).toBe(false);

    // Same pane, wider columns: the verdict follows the content.
    needed = 800;
    root.querySelector<HTMLInputElement>("[data-gadget-filter]")!.dispatchEvent(
      new Event("input", { bubbles: true }),
    );
    await new Promise((r) => setTimeout(r, 200));
    expect(root.hasAttribute("data-gadget-stacked")).toBe(true);

    // …and back, so crossing the threshold is not one-way.
    needed = 300;
    root.querySelector<HTMLInputElement>("[data-gadget-filter]")!.dispatchEvent(
      new Event("input", { bubbles: true }),
    );
    await new Promise((r) => setTimeout(r, 200));
    expect(root.hasAttribute("data-gadget-stacked")).toBe(false);
  });

  it("labels every cell so stacked rows can reprint the header", () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    const cells = [...root.querySelectorAll("tbody tr:first-child td")];
    expect(cells.map((c) => c.getAttribute("data-gadget-label"))).toEqual(["Name", "Age"]);
    expect(cells.every((c) => c.getAttribute("role") === "cell")).toBe(true);
  });

  it("sorts from the compact control and keeps the header in step", async () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    const sel = root.querySelector<HTMLSelectElement>("[data-gadget-sort-select]") as HTMLSelectElement;
    sel.value = "name|desc";
    sel.dispatchEvent(new Event("change", { bubbles: true }));
    await flush();

    const names = [...root.querySelectorAll('td[data-gadget-label="Name"]')].map((c) => c.textContent);
    expect(names).toEqual([...names].sort().reverse());
    expect(
      root.querySelector('th[data-gadget-sort="name"]')?.getAttribute("aria-sort"),
    ).toBe("descending");
  });

  it("renders initial rows from the data island", () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    expect(root.querySelectorAll("tbody tr")).toHaveLength(3);
    expect(cellTexts(root, 0)).toEqual(["Carol", "Alice", "Bob"]);
    expect(root.querySelector<HTMLElement>("[data-gadget-empty]")?.hidden).toBe(true);
  });

  it("shows the empty state when there are no rows", () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: null, bridge });
    expect(root.querySelectorAll("tbody tr")).toHaveLength(0);
    expect(root.querySelector<HTMLElement>("[data-gadget-empty]")?.hidden).toBe(false);
  });

  it("sorts on header click, toggling direction", () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    const sortBtn = root.querySelector<HTMLElement>('[data-gadget-sort="name"] button')!;
    sortBtn.click();
    expect(cellTexts(root, 0)).toEqual(["Alice", "Bob", "Carol"]);
    expect(root.querySelector('[data-gadget-sort="name"]')?.getAttribute("aria-sort")).toBe("ascending");
    sortBtn.click();
    expect(cellTexts(root, 0)).toEqual(["Carol", "Bob", "Alice"]);
    expect(root.querySelector('[data-gadget-sort="name"]')?.getAttribute("aria-sort")).toBe("descending");
  });

  it("applies a default sort from config", () => {
    const root = shell();
    mountTable({
      root,
      config: config({ defaultSort: { key: "age", desc: true } }),
      initialData: { rows: ROWS },
      bridge,
    });
    expect(cellTexts(root, 0)).toEqual(["Bob", "Carol", "Alice"]);
  });

  it("filters rows after the debounce", async () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    const input = root.querySelector<HTMLInputElement>("[data-gadget-filter]")!;
    input.value = "ali";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await new Promise((r) => setTimeout(r, 250));
    expect(cellTexts(root, 0)).toEqual(["Alice"]);
    expect(root.querySelector<HTMLElement>("[data-gadget-empty]")?.hidden).toBe(true);
  });

  it("shows 'no matching rows' when the filter eliminates everything", async () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });
    const input = root.querySelector<HTMLInputElement>("[data-gadget-filter]")!;
    input.value = "zzz";
    input.dispatchEvent(new Event("input", { bubbles: true }));
    await new Promise((r) => setTimeout(r, 250));
    const empty = root.querySelector<HTMLElement>("[data-gadget-empty]")!;
    expect(empty.hidden).toBe(false);
    expect(empty.querySelector("h3")?.textContent).toBe("No matching rows");
  });

  it("paginates and updates the page info", () => {
    const root = shell();
    mountTable({ root, config: config({ pageSize: 2 }), initialData: { rows: ROWS }, bridge });
    expect(root.querySelectorAll("tbody tr")).toHaveLength(2);
    const info = root.querySelector("[data-gadget-page-info]")!;
    expect(info.textContent).toBe("1–2 of 3");
    root.querySelector<HTMLElement>('[data-gadget-page="next"]')!.click();
    expect(root.querySelectorAll("tbody tr")).toHaveLength(1);
    expect(info.textContent).toBe("3–3 of 3");
  });

  it("resizes the page from the page-size chooser", () => {
    const root = shell({ pageSizes: [2, 10] });
    mountTable({ root, config: config({ pageSize: 2 }), initialData: { rows: ROWS }, bridge });
    const pagination = root.querySelector<HTMLElement>("[data-gadget-pagination]")!;
    const sizeEl = root.querySelector<HTMLSelectElement>("[data-gadget-page-size]")!;
    expect(sizeEl.value).toBe("2");
    expect(root.querySelectorAll("tbody tr")).toHaveLength(2);

    sizeEl.value = "10";
    sizeEl.dispatchEvent(new Event("change", { bubbles: true }));
    expect(root.querySelectorAll("tbody tr")).toHaveLength(3);
    expect(root.querySelector("[data-gadget-page-info]")?.textContent).toBe("1–3 of 3");
    // One page now, but the bar stays: it is the way back to a smaller page.
    expect(pagination.hidden).toBe(false);
  });

  it("posts a row action's prompt as a chat message instead of calling the tool", async () => {
    const root = shell();
    const cfg = config({
      columns: [
        { key: "name", label: "Name", type: "text", sortable: true },
        {
          key: "",
          label: "",
          type: "actions",
          sortable: false,
          actions: [
            {
              label: "Edit",
              kind: "tool",
              tool: "edit_user",
              prompt: "Open the edit form for this user",
            },
          ],
        },
      ],
    });
    mountTable({ root, config: cfg, initialData: { rows: ROWS }, bridge });

    openMenu(root, 'tbody [data-gadget-action-menu="1"]')[0]!.click();
    await flush();

    expect(host.received(M.toolsCall)).toHaveLength(0);
    expect(host.received(M.message)[0]!.params).toMatchObject({
      role: "user",
      content: [{ type: "text", text: "Open the edit form for this user" }],
    });
    // Nothing came back to apply, so the rows are untouched and the working
    // status is cleared.
    expect(root.querySelectorAll("tbody tr")).toHaveLength(3);
    expect(root.querySelector<HTMLElement>("[data-gadget-status]")!.hidden).toBe(true);
  });

  it("fires row actions with FromRow args and applies returned rows", async () => {
    const root = shell();
    const cfg = config({
      columns: [
        { key: "name", label: "Name", type: "text", sortable: true },
        {
          key: "",
          label: "",
          type: "actions",
          sortable: false,
          actions: [
            {
              label: "Delete",
              kind: "tool",
              tool: "delete_user",
              args: { id: { row: "id" }, hard: { static: true } },
            },
          ],
        },
      ],
    });
    host.onToolCall = (name, args) => ({
      content: [{ type: "text", text: "Deleted." }],
      structuredContent: { rows: ROWS.filter((r) => r.id !== args.id) },
    });
    mountTable({ root, config: cfg, initialData: { rows: ROWS }, bridge });

    const items = openMenu(root, 'tbody [data-gadget-action-menu="1"]');
    expect(items.map((el) => el.textContent)).toEqual(["Delete"]);
    items[0]!.click();
    await flush();

    const calls = host.received(M.toolsCall);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.params).toMatchObject({
      name: "delete_user",
      arguments: { id: 1, hard: true },
    });
    expect(root.querySelectorAll("tbody tr")).toHaveLength(2);
    const status = root.querySelector<HTMLElement>("[data-gadget-status]")!;
    expect(status.hidden).toBe(false);
    expect(status.textContent).toBe("Deleted.");
    expect(status.className).toContain("gadget-status--success");
  });

  it("requires a second choice for confirm actions", async () => {
    const root = shell();
    const cfg = config({
      columns: [
        { key: "name", label: "Name", type: "text", sortable: true },
        {
          key: "",
          label: "",
          type: "actions",
          sortable: false,
          actions: [
            { label: "Delete", kind: "tool", tool: "delete_user", confirm: "Really delete?" },
          ],
        },
      ],
    });
    mountTable({ root, config: cfg, initialData: { rows: ROWS }, bridge });

    const item = openMenu(root, 'tbody [data-gadget-action-menu="1"]')[0]!;
    item.click();
    await flush();
    expect(host.received(M.toolsCall)).toHaveLength(0);
    expect(item.hasAttribute("data-gadget-armed")).toBe(true);
    expect(item.textContent).toBe("Really delete?");
    // The menu stays open while the question is standing.
    expect(root.querySelector<HTMLElement>(".gadget-action-panel")!.hidden).toBe(false);
    item.click();
    await flush();
    expect(host.received(M.toolsCall)).toHaveLength(1);
    expect(root.querySelector<HTMLElement>(".gadget-action-panel")!.hidden).toBe(true);
  });

  it("selects rows, shows bulk actions, and resolves FromSelection args", async () => {
    const root = shell({ selection: true, bulk: true });
    const cfg = config({
      selection: {
        bulk: [
          {
            label: "Archive",
            kind: "tool",
            tool: "archive_users",
            args: { ids: { selection: "id" } },
          },
        ],
      },
    });
    host.onToolCall = () => ({ structuredContent: { rows: [] } });
    mountTable({ root, config: cfg, initialData: { rows: ROWS }, bridge });

    const bulkBar = root.querySelector<HTMLElement>("[data-gadget-bulk]")!;
    expect(bulkBar.hidden).toBe(true);

    const selectAll = root.querySelector<HTMLInputElement>("[data-gadget-select-all]")!;
    selectAll.checked = true;
    selectAll.dispatchEvent(new Event("change", { bubbles: true }));
    expect(bulkBar.hidden).toBe(false);
    expect(root.querySelector("[data-gadget-bulk-count]")?.textContent).toBe("3 selected");

    openMenu(root, "[data-gadget-bulk-menu]")[0]!.click();
    await flush();
    const calls = host.received(M.toolsCall);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.params).toMatchObject({
      name: "archive_users",
      arguments: { ids: [1, 2, 3] }, // raw field values, not stringified selection keys
    });
    // rows replaced by result; selection cleared
    expect(root.querySelectorAll("tbody tr")).toHaveLength(0);
    expect(bulkBar.hidden).toBe(true);
  });

  it("renders badge and link cells", () => {
    const root = shell();
    const cfg = config({
      columns: [
        {
          key: "status",
          label: "Status",
          type: "badge",
          sortable: false,
          badge: { active: "success", banned: "danger" },
        },
        {
          key: "url",
          label: "Site",
          type: "link",
          sortable: false,
          link: { hrefKey: "url", text: "Visit" },
        },
      ],
    });
    mountTable({
      root,
      config: cfg,
      initialData: { rows: [{ id: 1, status: "active", url: "https://example.com" }] },
      bridge,
    });
    const badge = root.querySelector("tbody .gadget-badge")!;
    expect(badge.textContent).toBe("active");
    expect(badge.className).toContain("gadget-badge--success");
    const link = root.querySelector<HTMLElement>("tbody [data-gadget-link]")!;
    expect(link.textContent).toBe("Visit");

    link.click();
    // openLink request goes to the host
    return flush().then(() => {
      expect(host.received(M.openLink)).toHaveLength(1);
      expect(host.received(M.openLink)[0]!.params).toEqual({ url: "https://example.com" });
    });
  });

  it("updates rows from tool-result notifications and shows loading on tool-input", async () => {
    const root = shell();
    mountTable({ root, config: config(), initialData: { rows: ROWS }, bridge });

    host.notify(M.toolInput, { arguments: {} });
    await flush();
    const status = root.querySelector<HTMLElement>("[data-gadget-status]")!;
    expect(status.className).toContain("gadget-status--loading");

    host.pushToolResult({ structuredContent: { rows: [{ id: 9, name: "Zoe", age: 1 }] } });
    await flush();
    expect(cellTexts(root, 0)).toEqual(["Zoe"]);
    expect(status.className).not.toContain("gadget-status--loading");
  });

  it("hydrates from loadTool on mount, replacing the baked snapshot", async () => {
    const root = shell();
    host.onToolCall = (name) =>
      name === "list_users"
        ? { structuredContent: { rows: [{ id: 9, name: "Zed", age: 40 }] } }
        : { structuredContent: {} };
    mountTable({
      root,
      config: config({ loadTool: "list_users", loadArgs: { scope: "all" } }),
      initialData: { rows: ROWS },
      bridge,
      ready: Promise.resolve(true),
    });

    // Baked snapshot paints first...
    expect(cellTexts(root, 0)).toEqual(["Carol", "Alice", "Bob"]);
    await flush();

    // ...then the load tool fires (with its static args) and replaces it.
    const calls = host.received(M.toolsCall);
    expect(calls).toHaveLength(1);
    expect(calls[0]!.params).toMatchObject({ name: "list_users", arguments: { scope: "all" } });
    expect(cellTexts(root, 0)).toEqual(["Zed"]);
    const status = root.querySelector<HTMLElement>("[data-gadget-status]")!;
    expect(status.className).not.toContain("gadget-status--loading");
  });

  it("does not hydrate when no host answered the handshake (ready=false)", async () => {
    const root = shell();
    mountTable({
      root,
      config: config({ loadTool: "list_users" }),
      initialData: { rows: ROWS },
      bridge,
      ready: Promise.resolve(false),
    });
    await flush();
    expect(host.received(M.toolsCall)).toHaveLength(0);
    expect(cellTexts(root, 0)).toEqual(["Carol", "Alice", "Bob"]);
  });

  it("keeps the baked snapshot when loadTool fails", async () => {
    const root = shell();
    host.onToolCall = () => {
      throw new Error("boom");
    };
    mountTable({
      root,
      config: config({ loadTool: "list_users" }),
      initialData: { rows: ROWS },
      bridge,
      ready: Promise.resolve(true),
    });
    await flush();
    expect(cellTexts(root, 0)).toEqual(["Carol", "Alice", "Bob"]);
    const status = root.querySelector<HTMLElement>("[data-gadget-status]")!;
    expect(status.className).not.toContain("gadget-status--loading");
  });
});
