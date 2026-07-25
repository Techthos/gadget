import { beforeEach, describe, expect, it } from "vitest";
import { enhanceSelects, refreshDropdown } from "../src/dropdown";

// Mirrors what Go renders: a plain <select> inside the widget root. The
// runtime upgrades it in place.
function shell(inner: string): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gadget-root";
	root.setAttribute("data-gadget-widget", "form");
	root.innerHTML = inner;
	document.body.append(root);
	enhanceSelects(root);
	return root;
}

const SINGLE = `
  <label for="gadget-f-role">Role</label>
  <select id="gadget-f-role" name="role" class="gadget-input gadget-sort-select" aria-label="Role">
    <option value="user">User</option>
    <option value="admin">Admin</option>
    <option value="owner" disabled>Owner</option>
  </select>`;

const MULTI = `
  <select id="gadget-f-tags" name="tags" class="gadget-input" multiple placeholder="Pick tags">
    <option value="a">Alpha</option>
    <option value="b">Beta</option>
    <option value="c">Gamma</option>
  </select>`;

const PLACEHOLDER = `
  <select data-gadget-sort-select="" class="gadget-input">
    <option value="">Sort…</option>
    <option value="name|asc">Name ↑</option>
  </select>`;

function parts(root: HTMLElement) {
	return {
		select: root.querySelector<HTMLSelectElement>("select")!,
		trigger: root.querySelector<HTMLButtonElement>(".gadget-dd-trigger")!,
		panel: root.querySelector<HTMLElement>(".gadget-dd-panel")!,
		value: root.querySelector<HTMLElement>(".gadget-dd-value")!,
		options: [...root.querySelectorAll<HTMLElement>(".gadget-dd-option")],
	};
}

function key(el: HTMLElement, k: string): void {
	el.dispatchEvent(new KeyboardEvent("keydown", { key: k, bubbles: true, cancelable: true }));
}

describe("dropdown", () => {
	beforeEach(() => {
		document.body.innerHTML = "";
	});

	it("upgrades a select without removing it", () => {
		const root = shell(SINGLE);
		const { select, trigger, panel, options } = parts(root);

		expect(select.isConnected).toBe(true);
		expect(select.name).toBe("role");
		expect(select.tabIndex).toBe(-1);
		expect(select.classList.contains("gadget-dd-native")).toBe(true);

		// The field label addresses the control by id, so the trigger takes it.
		expect(trigger.id).toBe("gadget-f-role");
		expect(select.id).toBe("");
		expect(trigger.getAttribute("role")).toBe("combobox");
		expect(trigger.getAttribute("aria-expanded")).toBe("false");
		expect(trigger.getAttribute("aria-label")).toBe("Role");
		// Author classes style what the user sees.
		expect(trigger.classList.contains("gadget-sort-select")).toBe(true);

		// The panel escapes the card chrome that would clip it.
		expect(panel.parentElement).toBe(root);
		expect(panel.hidden).toBe(true);
		expect(panel.getAttribute("role")).toBe("listbox");
		expect(options.map((o) => o.textContent)).toEqual(["User", "Admin", "Owner"]);
		expect(options[2]?.getAttribute("aria-disabled")).toBe("true");
		expect(parts(root).value.textContent).toBe("User");
	});

	it("opens on click and selects an option", () => {
		const root = shell(SINGLE);
		const { select, trigger, panel } = parts(root);
		const changes: string[] = [];
		select.addEventListener("change", () => changes.push(select.value));

		trigger.click();
		expect(panel.hidden).toBe(false);
		expect(trigger.getAttribute("aria-expanded")).toBe("true");

		parts(root).options[1]!.click();
		expect(select.value).toBe("admin");
		expect(changes).toEqual(["admin"]);
		expect(panel.hidden).toBe(true);
		expect(parts(root).value.textContent).toBe("Admin");
		expect(parts(root).options[1]!.getAttribute("aria-selected")).toBe("true");
		expect(parts(root).options[0]!.getAttribute("aria-selected")).toBe("false");
	});

	it("navigates with the keyboard and skips disabled options", () => {
		const root = shell(SINGLE);
		const { select, trigger, panel } = parts(root);

		key(trigger, "ArrowDown");
		expect(panel.hidden).toBe(false);
		// Opens on the current selection, then steps past the disabled option
		// and stops at the end of the list.
		key(trigger, "ArrowDown");
		key(trigger, "ArrowDown");
		expect(trigger.getAttribute("aria-activedescendant")).toBe(parts(root).options[1]!.id);

		key(trigger, "Enter");
		expect(select.value).toBe("admin");
		expect(panel.hidden).toBe(true);

		key(trigger, "ArrowDown");
		key(trigger, "Escape");
		expect(panel.hidden).toBe(true);
		expect(trigger.getAttribute("aria-expanded")).toBe("false");
	});

	it("jumps to an option by typing", () => {
		const root = shell(SINGLE);
		const { trigger } = parts(root);

		trigger.click();
		key(trigger, "a");
		expect(trigger.getAttribute("aria-activedescendant")).toBe(parts(root).options[1]!.id);
	});

	it("toggles several values and stays open for a multiple select", () => {
		const root = shell(MULTI);
		const { select, trigger, panel } = parts(root);

		expect(parts(root).value.textContent).toBe("Pick tags");
		expect(parts(root).value.classList.contains("gadget-dd-value--placeholder")).toBe(true);

		trigger.click();
		parts(root).options[0]!.click();
		parts(root).options[2]!.click();
		expect(panel.hidden).toBe(false);
		expect([...select.selectedOptions].map((o) => o.value)).toEqual(["a", "c"]);
		expect(parts(root).value.textContent).toBe("Alpha, Gamma");

		parts(root).options[1]!.click();
		expect(parts(root).value.textContent).toBe("3 selected");

		parts(root).options[0]!.click();
		expect([...select.selectedOptions].map((o) => o.value)).toEqual(["b", "c"]);
	});

	it("shows an empty value as a placeholder", () => {
		const root = shell(PLACEHOLDER);
		expect(parts(root).value.textContent).toBe("Sort…");
		expect(parts(root).value.classList.contains("gadget-dd-value--placeholder")).toBe(true);

		parts(root).select.value = "name|asc";
		refreshDropdown(parts(root).select);
		expect(parts(root).value.textContent).toBe("Name ↑");
		expect(parts(root).value.classList.contains("gadget-dd-value--placeholder")).toBe(false);
	});

	it("closes on a press outside", () => {
		const root = shell(SINGLE);
		const { trigger, panel } = parts(root);

		trigger.click();
		expect(panel.hidden).toBe(false);
		document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
		expect(panel.hidden).toBe(true);
	});

	it("follows the select's disabled and aria-invalid state", async () => {
		const root = shell(SINGLE);
		const { select, trigger } = parts(root);

		select.disabled = true;
		select.setAttribute("aria-invalid", "true");
		await new Promise((r) => setTimeout(r, 0));
		expect(trigger.disabled).toBe(true);
		expect(trigger.getAttribute("aria-invalid")).toBe("true");

		select.disabled = false;
		select.removeAttribute("aria-invalid");
		await new Promise((r) => setTimeout(r, 0));
		expect(trigger.disabled).toBe(false);
		expect(trigger.hasAttribute("aria-invalid")).toBe(false);
	});

	it("enhances a select only once", () => {
		const root = shell(SINGLE);
		enhanceSelects(root);
		expect(root.querySelectorAll(".gadget-dd-trigger").length).toBe(1);
		expect(root.querySelectorAll(".gadget-dd-panel").length).toBe(1);
	});
});
