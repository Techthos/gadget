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
	root.className = "gomu-root";
	document.body.append(root);
	root.append(actionMenuTrigger({ "data-gomu-action-menu": "0" }));
	return root;
}

function parts(root: HTMLElement) {
	const panel = root.querySelector<HTMLElement>(".gomu-action-panel")!;
	return {
		trigger: root.querySelector<HTMLButtonElement>("[data-gomu-action-menu]")!,
		panel,
		// Visibility lives on the overlay that wraps the panel (see src/popup.ts).
		overlay: panel?.parentElement as HTMLElement,
		items: () => [...root.querySelectorAll<HTMLElement>("[data-gomu-action-index]")],
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
		const { trigger, panel, overlay, items } = parts(root);

		expect(overlay.hidden).toBe(true);
		trigger.click();

		expect(overlay.hidden).toBe(false);
		// Visibility is the overlay's job; the panel must never carry hidden, or
		// [hidden]'s display:none would blank it inside a shown overlay.
		expect(panel.hasAttribute("hidden")).toBe(false);
		expect(trigger.getAttribute("aria-expanded")).toBe("true");
		expect(panel.getAttribute("role")).toBe("menu");
		expect(items().map((el) => el.textContent)).toEqual([
			"Open profile",
			"Send invite",
			"Delete",
		]);
		expect(items()[2]!.className).toContain("gomu-action-item--danger");

		items()[1]!.click();
		expect(onSelect).toHaveBeenCalledWith(1);
		expect(overlay.hidden).toBe(true);
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

	it("asks over the frame before firing a confirmed action", () => {
		const root = shell();
		const onSelect = vi.fn();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect }));
		const { overlay, items } = parts(root);
		parts(root).trigger.click();

		items()[2]!.click(); // Delete carries a confirm
		// The menu closes and a confirmation takes the frame instead.
		expect(overlay.hidden).toBe(true);
		const ask = root.querySelector<HTMLElement>(".gomu-ask-panel")!;
		expect(ask.querySelector(".gomu-ask-message")!.textContent).toBe("Really?");
		const go = ask.querySelector<HTMLButtonElement>(".gomu-ask-confirm")!;
		expect(go.textContent).toBe("Delete");
		expect(go.classList.contains("gomu-btn--danger")).toBe(true);
		expect(onSelect).not.toHaveBeenCalled();

		// Cancel dismisses it without firing.
		ask.querySelector<HTMLButtonElement>(".gomu-ask-cancel")!.click();
		expect(onSelect).not.toHaveBeenCalled();
		expect(root.querySelector(".gomu-ask-panel")).toBeNull();

		// Reopen and confirm: now it fires once.
		parts(root).trigger.click();
		parts(root).items()[2]!.click();
		root.querySelector<HTMLButtonElement>(".gomu-ask-confirm")!.click();
		expect(onSelect).toHaveBeenCalledWith(2);
		expect(root.querySelector(".gomu-ask-panel")).toBeNull();
	});

	it("dismisses the confirmation on Escape without firing", () => {
		const root = shell();
		const onSelect = vi.fn();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect }));
		parts(root).trigger.click();
		parts(root).items()[2]!.click();

		key(root.querySelector<HTMLElement>(".gomu-ask-panel")!, "Escape");
		expect(onSelect).not.toHaveBeenCalled();
		expect(root.querySelector(".gomu-ask-panel")).toBeNull();
	});

	it("navigates with the keyboard and closes back onto the trigger", () => {
		const root = shell();
		const onSelect = vi.fn();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect }));
		const { trigger, overlay, items } = parts(root);

		key(trigger, "ArrowDown");
		expect(overlay.hidden).toBe(false);
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
		expect(overlay.hidden).toBe(true);
		expect(document.activeElement).toBe(trigger);
	});

	it("toggles on a second press of its own trigger", () => {
		const root = shell();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect: () => {} }));
		const { trigger, overlay } = parts(root);

		trigger.click();
		expect(overlay.hidden).toBe(false);
		trigger.click();
		expect(overlay.hidden).toBe(true);
	});

	it("stays shut for a trigger that resolves to nothing", () => {
		const root = shell();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: [], onSelect: () => {} }));
		parts(root).trigger.click();
		expect(parts(root).overlay.hidden).toBe(true);
	});

	it("closes when a press lands outside it", () => {
		const root = shell();
		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect: () => {} }));
		const { trigger, overlay } = parts(root);
		trigger.click();

		document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
		expect(overlay.hidden).toBe(true);
	});

	// Both popups share one open slot (see src/popup.ts), so a widget with a
	// sort dropdown and a row menu can never show two panels at once.
	it("closes an open dropdown, and is closed by one", () => {
		const root = shell();
		const select = document.createElement("select");
		select.innerHTML = `<option value="a">A</option><option value="b">B</option>`;
		root.append(select);
		enhanceSelects(root);
		const ddTrigger = root.querySelector<HTMLButtonElement>(".gomu-dd-trigger")!;
		const ddOverlay = root.querySelector<HTMLElement>(".gomu-dd-panel")!.parentElement as HTMLElement;

		const menu = createActionMenu(root);
		menu.bind(root, "action-menu", () => ({ items: ACTIONS, onSelect: () => {} }));
		const { trigger, overlay } = parts(root);

		ddTrigger.click();
		expect(ddOverlay.hidden).toBe(false);
		trigger.click();
		expect(ddOverlay.hidden).toBe(true);
		expect(overlay.hidden).toBe(false);

		ddTrigger.click();
		expect(overlay.hidden).toBe(true);
		expect(ddOverlay.hidden).toBe(false);
	});
});
