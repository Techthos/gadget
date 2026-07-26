import { beforeEach, describe, expect, it, vi } from "vitest";
import { actionMenuTrigger, createActionMenu, type MenuAction } from "../src/actionmenu";
import { enhanceSelects } from "../src/dropdown";

const ACTIONS: MenuAction[] = [
	{ label: "Open profile" },
	{ label: "Send invite" },
	{ label: "Delete", variant: "danger", confirm: "Really?" },
];

function shell(): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gadget-root";
	document.body.append(root);
	root.append(actionMenuTrigger({ "data-gadget-action-menu": "0" }));
	return root;
}

function parts(root: HTMLElement) {
	return {
		trigger: root.querySelector<HTMLButtonElement>("[data-gadget-action-menu]")!,
		panel: root.querySelector<HTMLElement>(".gadget-action-panel")!,
		items: () => [...root.querySelectorAll<HTMLElement>("[data-gadget-action-index]")],
	};
}

function key(el: Element, k: string): void {
	el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));
}

describe("action menu", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
		vi.useRealTimers();
	});

	it("opens on the trigger, renders the bound actions, and fires the chosen one", () => {
		const root = shell();
		const onSelect = vi.fn();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect }));
		const { trigger, panel, items } = parts(root);

		expect(panel.hidden).toBe(true);
		trigger.click();

		expect(panel.hidden).toBe(false);
		expect(trigger.getAttribute("aria-expanded")).toBe("true");
		expect(panel.getAttribute("role")).toBe("menu");
		expect(items().map((el) => el.textContent)).toEqual([
			"Open profile",
			"Send invite",
			"Delete",
		]);
		expect(items()[2]!.className).toContain("gadget-action-item--danger");

		items()[1]!.click();
		expect(onSelect).toHaveBeenCalledWith(1);
		expect(panel.hidden).toBe(true);
		expect(trigger.getAttribute("aria-expanded")).toBe("false");
	});

	it("labels are text, never markup", () => {
		const root = shell();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({
			items: [{ label: "<img src=x onerror=alert(1)>" }],
			onSelect: () => {},
		}));
		parts(root).trigger.click();

		const item = parts(root).items()[0]!;
		expect(item.querySelector("img")).toBeNull();
		expect(item.textContent).toBe("<img src=x onerror=alert(1)>");
	});

	it("asks a confirmed action twice, on the item itself", () => {
		const root = shell();
		const onSelect = vi.fn();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect }));
		const { panel, items } = parts(root);
		parts(root).trigger.click();

		items()[2]!.click();
		expect(onSelect).not.toHaveBeenCalled();
		expect(items()[2]!.textContent).toBe("Really?");
		expect(items()[2]!.hasAttribute("data-gadget-armed")).toBe(true);
		expect(panel.hidden).toBe(false);

		items()[2]!.click();
		expect(onSelect).toHaveBeenCalledWith(2);
		expect(panel.hidden).toBe(true);
	});

	it("disarms a confirmation left standing", () => {
		vi.useFakeTimers();
		const root = shell();
		const onSelect = vi.fn();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect }));
		parts(root).trigger.click();

		parts(root).items()[2]!.click();
		vi.advanceTimersByTime(5000);
		expect(parts(root).items()[2]!.textContent).toBe("Delete");

		parts(root).items()[2]!.click();
		expect(onSelect).not.toHaveBeenCalled();
	});

	it("navigates with the keyboard and closes back onto the trigger", () => {
		const root = shell();
		const onSelect = vi.fn();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect }));
		const { trigger, panel, items } = parts(root);

		key(trigger, "ArrowDown");
		expect(panel.hidden).toBe(false);
		expect(document.activeElement).toBe(items()[0]);

		key(items()[0]!, "ArrowUp"); // wraps to the last item
		expect(document.activeElement).toBe(items()[2]);
		key(items()[2]!, "Home");
		expect(document.activeElement).toBe(items()[0]);

		key(items()[0]!, "Enter");
		expect(onSelect).toHaveBeenCalledWith(0);

		key(trigger, "ArrowUp"); // opens onto the end of the menu
		expect(document.activeElement).toBe(items()[2]);
		key(items()[2]!, "Escape");
		expect(panel.hidden).toBe(true);
		expect(document.activeElement).toBe(trigger);
	});

	it("toggles on a second press of its own trigger", () => {
		const root = shell();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect: () => {} }));
		const { trigger, panel } = parts(root);

		trigger.click();
		expect(panel.hidden).toBe(false);
		trigger.click();
		expect(panel.hidden).toBe(true);
	});

	it("stays shut for a trigger that resolves to nothing", () => {
		const root = shell();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: [], onSelect: () => {} }));
		parts(root).trigger.click();
		expect(parts(root).panel.hidden).toBe(true);
	});

	it("closes when a press lands outside it", () => {
		const root = shell();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect: () => {} }));
		const { trigger, panel } = parts(root);
		trigger.click();

		document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
		expect(panel.hidden).toBe(true);
	});

	// Both popups share one open slot (see src/popup.ts), so a widget with a
	// sort dropdown and a row menu can never show two panels at once.
	it("closes an open dropdown, and is closed by one", () => {
		const root = shell();
		const select = document.createElement("select");
		select.innerHTML = `<option value="a">A</option><option value="b">B</option>`;
		root.append(select);
		enhanceSelects(root);
		const ddTrigger = root.querySelector<HTMLButtonElement>(".gadget-dd-trigger")!;
		const ddPanel = root.querySelector<HTMLElement>(".gadget-dd-panel")!;

		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect: () => {} }));
		const { trigger, panel } = parts(root);

		ddTrigger.click();
		expect(ddPanel.hidden).toBe(false);
		trigger.click();
		expect(ddPanel.hidden).toBe(true);
		expect(panel.hidden).toBe(false);

		ddTrigger.click();
		expect(panel.hidden).toBe(true);
		expect(ddPanel.hidden).toBe(false);
	});
});
