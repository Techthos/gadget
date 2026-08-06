import { beforeEach, describe, expect, it } from "vitest";
import { enhanceSelects, refreshDropdown } from "../src/dropdown";

// Mirrors what Go renders: a plain <select> inside the widget root. The
// runtime upgrades it in place.
function shell(inner: string): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gomu-root";
	root.setAttribute("data-gomu-widget", "form");
	root.innerHTML = inner;
	document.body.append(root);
	enhanceSelects(root);
	return root;
}

const SINGLE = `
  <label for="gomu-f-role">Role</label>
  <select id="gomu-f-role" name="role" class="gomu-input gomu-sort-select" aria-label="Role">
    <option value="user">User</option>
    <option value="admin">Admin</option>
    <option value="owner" disabled>Owner</option>
  </select>`;

const MULTI = `
  <select id="gomu-f-tags" name="tags" class="gomu-input" multiple placeholder="Pick tags">
    <option value="a">Alpha</option>
    <option value="b">Beta</option>
    <option value="c">Gamma</option>
  </select>`;

const PLACEHOLDER = `
  <select data-gomu-sort-select="" class="gomu-input">
    <option value="">Sort…</option>
    <option value="name|asc">Name ↑</option>
  </select>`;

// Ten options — past the threshold that adds the search field.
const LONG = `
  <select id="gomu-f-city" name="city" class="gomu-input" aria-label="City">
    <option value="ath">Athens</option>
    <option value="ber">Berlin</option>
    <option value="cai">Cairo</option>
    <option value="del">Delhi</option>
    <option value="edi">Edinburgh</option>
    <option value="flo">Florence</option>
    <option value="gen">Geneva</option>
    <option value="hel">Helsinki</option>
    <option value="ist">Istanbul</option>
    <option value="jak">Jakarta</option>
  </select>`;

function parts(root: HTMLElement) {
	return {
		select: root.querySelector<HTMLSelectElement>("select")!,
		trigger: root.querySelector<HTMLButtonElement>(".gomu-dd-trigger")!,
		overlay: root.querySelector<HTMLElement>(".gomu-pop-overlay")!,
		panel: root.querySelector<HTMLElement>(".gomu-dd-panel")!,
		list: root.querySelector<HTMLElement>(".gomu-dd-list")!,
		search: root.querySelector<HTMLInputElement>(".gomu-dd-search"),
		value: root.querySelector<HTMLElement>(".gomu-dd-value")!,
		options: [...root.querySelectorAll<HTMLElement>(".gomu-dd-option")],
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
		const { select, trigger, overlay, list, options } = parts(root);

		expect(select.isConnected).toBe(true);
		expect(select.name).toBe("role");
		expect(select.tabIndex).toBe(-1);
		expect(select.classList.contains("gomu-dd-native")).toBe(true);

		// The field label addresses the control by id, so the trigger takes it.
		expect(trigger.id).toBe("gomu-f-role");
		expect(select.id).toBe("");
		expect(trigger.getAttribute("role")).toBe("combobox");
		expect(trigger.getAttribute("aria-expanded")).toBe("false");
		expect(trigger.getAttribute("aria-label")).toBe("Role");
		// Author classes style what the user sees.
		expect(trigger.classList.contains("gomu-sort-select")).toBe(true);

		// The overlay escapes the card chrome that would clip it, and starts closed.
		expect(overlay.parentElement).toBe(root);
		expect(overlay.hidden).toBe(true);
		expect(list.getAttribute("role")).toBe("listbox");
		expect(options.map((o) => o.textContent)).toEqual(["User", "Admin", "Owner"]);
		expect(options[2]?.getAttribute("aria-disabled")).toBe("true");
		expect(parts(root).value.textContent).toBe("User");
	});

	it("opens on click and selects an option", () => {
		const root = shell(SINGLE);
		const { select, trigger, overlay } = parts(root);
		const changes: string[] = [];
		select.addEventListener("change", () => changes.push(select.value));

		trigger.click();
		expect(overlay.hidden).toBe(false);
		expect(trigger.getAttribute("aria-expanded")).toBe("true");

		parts(root).options[1]!.click();
		expect(select.value).toBe("admin");
		expect(changes).toEqual(["admin"]);
		expect(overlay.hidden).toBe(true);
		expect(parts(root).value.textContent).toBe("Admin");
		expect(parts(root).options[1]!.getAttribute("aria-selected")).toBe("true");
		expect(parts(root).options[0]!.getAttribute("aria-selected")).toBe("false");
	});

	it("navigates with the keyboard and skips disabled options", () => {
		const root = shell(SINGLE);
		const { select, trigger, overlay } = parts(root);

		key(trigger, "ArrowDown");
		expect(overlay.hidden).toBe(false);
		// Opens on the current selection, then steps past the disabled option
		// and stops at the end of the list.
		key(trigger, "ArrowDown");
		key(trigger, "ArrowDown");
		expect(trigger.getAttribute("aria-activedescendant")).toBe(parts(root).options[1]!.id);

		key(trigger, "Enter");
		expect(select.value).toBe("admin");
		expect(overlay.hidden).toBe(true);

		key(trigger, "ArrowDown");
		key(trigger, "Escape");
		expect(overlay.hidden).toBe(true);
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
		const { select, trigger, overlay } = parts(root);

		expect(parts(root).value.textContent).toBe("Pick tags");
		expect(parts(root).value.classList.contains("gomu-dd-value--placeholder")).toBe(true);

		trigger.click();
		parts(root).options[0]!.click();
		parts(root).options[2]!.click();
		expect(overlay.hidden).toBe(false);
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
		expect(parts(root).value.classList.contains("gomu-dd-value--placeholder")).toBe(true);

		parts(root).select.value = "name|asc";
		refreshDropdown(parts(root).select);
		expect(parts(root).value.textContent).toBe("Name ↑");
		expect(parts(root).value.classList.contains("gomu-dd-value--placeholder")).toBe(false);
	});

	it("closes on a press outside", () => {
		const root = shell(SINGLE);
		const { trigger, overlay } = parts(root);

		trigger.click();
		expect(overlay.hidden).toBe(false);
		document.body.dispatchEvent(new Event("pointerdown", { bubbles: true }));
		expect(overlay.hidden).toBe(true);
	});

	it("adds a search field only when the list is long", () => {
		expect(parts(shell(SINGLE)).search).toBe(null);
		expect(parts(shell(LONG)).search).not.toBe(null);
	});

	it("filters the list from the search field and picks a match", () => {
		const root = shell(LONG);
		const { select, trigger, search, overlay } = parts(root);

		trigger.click();
		search!.value = "ber";
		search!.dispatchEvent(new Event("input", { bubbles: true }));

		const visible = parts(root).options.filter((o) => !o.hidden);
		expect(visible.map((o) => o.textContent)).toEqual(["Berlin"]);
		// The active option follows the filter to the one match.
		expect(search!.getAttribute("aria-activedescendant")).toBe(visible[0]!.id);

		key(search!, "Enter");
		expect(select.value).toBe("ber");
		expect(overlay.hidden).toBe(true);
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
		expect(root.querySelectorAll(".gomu-dd-trigger").length).toBe(1);
		expect(root.querySelectorAll(".gomu-dd-panel").length).toBe(1);
	});
});
