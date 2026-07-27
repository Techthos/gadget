// Descriptions: a label/value detail list. A shared block rather than a
// widget — a behavior owns the host element and calls fillDescriptions
// whenever its record changes.
//
// Values are typed and Intl-formatted exactly like table cells (valueNode
// does the work for both), or authored as fixed text in Go. Layout is CSS
// only: the items flow into as many columns as the widget's width allows.
//
// An item can also ask instead of state: with an `input` config its value
// cell holds a control, and what the reader puts in it is collected by the
// embedding behavior and merged into that widget's own tool call. The list is
// rebuilt from scratch on every data change, so the values live in the
// behavior (passed back in as `values`) rather than in the DOM alone.
import { Row } from "../data";
import { checkbox, clear, h } from "../dom";
import { enhanceSelects, releaseDropdowns } from "../dropdown";
import { FieldCfg, valueNode } from "./value";

/** The control an editable item renders. Mirrors Go's Input. */
export interface DescriptionInputCfg {
	name: string;
	type: "text" | "number" | "select" | "checkbox";
	placeholder?: string;
	required?: boolean;
	default?: unknown;
	options?: Array<{ value: string; label: string }>;
	pattern?: string;
	min?: number;
	max?: number;
	step?: number;
	minLength?: number;
	maxLength?: number;
	/** Overrides the browser's validation message. */
	message?: string;
}

// A description item is a field config plus the fixed-text alternative to
// reading a record field, or a control instead of either.
export type DescriptionItemCfg = FieldCfg & { text?: string; input?: DescriptionInputCfg };

/** What a block's controls hold, by argument name. */
export type InputValues = Record<string, unknown>;

type InputControl = HTMLInputElement | HTMLSelectElement;

// Stands in for a value the record does not carry. A confirmation lists facts
// the reader decides on, so an absent one is shown as absent rather than
// dropped — and the grid keeps its shape either way.
const MISSING = "—";

// Ids only have to be unique within the document, and a card list renders the
// same item set once per record — so they are numbered rather than named.
let seq = 0;

export interface FillOptions {
	/** Values to open the controls on, overriding record prefill and defaults. */
	values?: InputValues;
	/** Renders every control disabled (the widget is working, or done). */
	disabled?: boolean;
}

/** True when any item asks for a value rather than stating one. */
export function hasInputs(items: DescriptionItemCfg[]): boolean {
	return items.some((i) => i.input);
}

/**
 * Renders items into an existing <dl>, replacing whatever it held. The list
 * is hidden while there is nothing to show, so an empty item set leaves no
 * gap in the widget.
 *
 * Selects are upgraded to gomukit dropdowns only once the list is on the page:
 * a dropdown's panel hangs off the widget root (see popup.ts), which a
 * detached node has no way to find. A behavior that builds the list detached —
 * a card being rendered — calls enhanceDescriptionInputs after appending it.
 */
export function fillDescriptions(
	host: HTMLElement,
	items: DescriptionItemCfg[],
	row: Row | null,
	opts: FillOptions = {},
): void {
	// The dropdown over a select outlives the select unless it is told to go:
	// its panel is a child of the widget root, not of this list.
	releaseDropdowns(host);
	clear(host);
	host.hidden = items.length === 0;
	for (const item of items) {
		const cell = h("div", {
			class: "gomu-desc-item" + (item.input ? " gomu-desc-item--input" : ""),
		});
		const dd = h("dd", {
			class: "gomu-desc-value" + (item.align ? ` gomu-align-${item.align}` : ""),
		});

		if (item.input) {
			const id = `gomu-di-${++seq}`;
			cell.append(h("dt", { class: "gomu-desc-label" }, inputLabel(item, id)));
			dd.append(
				inputControl(item.input, id, initialValue(item, row, opts.values), !!opts.disabled),
			);
			dd.append(h("p", { class: "gomu-desc-error", "data-gomu-input-error": "", hidden: true }));
			cell.append(dd);
			host.append(cell);
			continue;
		}

		cell.append(h("dt", { class: "gomu-desc-label" }, item.label || item.key));
		const value = item.text !== undefined ? item.text : valueNode(item, row ?? {});
		if (value === "") {
			dd.append(document.createTextNode(MISSING));
			dd.classList.add("gomu-desc-value--missing");
		} else {
			dd.append(typeof value === "string" ? document.createTextNode(value) : value);
		}

		cell.append(dd);
		host.append(cell);
	}
	if (host.isConnected) enhanceDescriptionInputs(host);
}

/** Upgrades the selects of a list that was built detached and then appended. */
export function enhanceDescriptionInputs(root: ParentNode): void {
	enhanceSelects(root);
}

/** The label, which addresses the control the way a form field's does. */
function inputLabel(item: DescriptionItemCfg, id: string): HTMLElement {
	const label = h("label", { for: id }, item.label || item.input?.name || item.key);
	if (item.input?.required) {
		label.append(h("span", { class: "gomu-required", "aria-hidden": "true" }, " *"));
	}
	return label;
}

/**
 * The value a control opens on: what the reader already put in it, else the
 * record field the item names, else the authored default. A control the reader
 * has touched keeps its value across a re-render — a tool result arriving
 * mid-answer must not wipe the answer.
 */
function initialValue(item: DescriptionItemCfg, row: Row | null, values?: InputValues): unknown {
	const name = item.input?.name ?? "";
	if (values && name in values) return values[name];
	if (item.key && row && row[item.key] !== undefined && row[item.key] !== null) {
		return row[item.key];
	}
	return item.input?.default;
}

function inputControl(
	cfg: DescriptionInputCfg,
	id: string,
	value: unknown,
	disabled: boolean,
): HTMLElement {
	const shared: Record<string, string | boolean> = {
		"data-gomu-input": cfg.name,
		id,
		name: cfg.name,
		disabled,
	};
	if (cfg.message) shared["data-gomu-input-message"] = cfg.message;
	if (cfg.required) shared.required = true;

	if (cfg.type === "checkbox") {
		// The box draws itself (ui/css/check.css) rather than wearing
		// .gomu-input, and none of the text or number constraints mean anything
		// on it.
		const box = checkbox(shared as Record<string, string>, "gomu-desc-check");
		box.input.checked = value === true || value === "true";
		box.input.disabled = disabled;
		return box.wrap;
	}

	if (cfg.type === "select") {
		const select = h("select", {
			...shared,
			class: "gomu-input",
			placeholder: cfg.placeholder,
		}) as HTMLSelectElement;
		const want = value === undefined || value === null ? "" : String(value);
		let matched = false;
		for (const opt of cfg.options ?? []) {
			const el = h("option", { value: opt.value }, opt.label) as HTMLOptionElement;
			if (opt.value === want) {
				el.selected = true;
				matched = true;
			}
			select.append(el);
		}
		// Nothing chosen yet: an empty first option keeps the placeholder
		// showing instead of silently standing for the first choice.
		if (!matched) {
			const blank = h("option", { value: "" }, cfg.placeholder ?? "") as HTMLOptionElement;
			blank.selected = true;
			select.prepend(blank);
		}
		return select;
	}

	const input = h("input", {
		...shared,
		type: cfg.type === "number" ? "number" : "text",
		class: "gomu-input",
		placeholder: cfg.placeholder,
		pattern: cfg.pattern,
		min: cfg.min,
		max: cfg.max,
		step: cfg.step,
		minlength: cfg.minLength,
		maxlength: cfg.maxLength,
	}) as HTMLInputElement;
	input.value = value === undefined || value === null ? "" : String(value);
	return input;
}

/** Every control in a block, in document order. */
function controls(host: ParentNode): InputControl[] {
	return [...host.querySelectorAll<InputControl>("[data-gomu-input]")];
}

/** What one control holds: a bool for a checkbox, a number for a number
 * input (undefined while it is empty), a string otherwise. */
export function readInput(el: InputControl): unknown {
	if (el instanceof HTMLInputElement) {
		if (el.type === "checkbox") return el.checked;
		if (el.type === "number") return el.value === "" ? undefined : Number(el.value);
	}
	return el.value;
}

/**
 * The arguments a block's controls contribute to the widget's call. An empty
 * number input contributes nothing rather than a NaN — the argument is simply
 * absent, which is what an unanswered optional question means.
 */
export function collectInputs(host: ParentNode): InputValues {
	const out: InputValues = {};
	for (const el of controls(host)) {
		const value = readInput(el);
		if (value === undefined) continue;
		out[el.getAttribute("data-gomu-input") ?? ""] = value;
	}
	return out;
}

/** Reports changes to a block's controls, so a behavior can keep the values
 * it feeds back into fillDescriptions. Both events fire: typing is `input`,
 * a dropdown or a checkbox is `change`. */
export function watchInputs(
	root: HTMLElement,
	onChange: (name: string, value: unknown, el: InputControl) => void,
): void {
	const handler = (ev: Event): void => {
		const el = ev.target;
		if (!(el instanceof HTMLInputElement) && !(el instanceof HTMLSelectElement)) return;
		const name = el.getAttribute("data-gomu-input");
		if (name === null || !root.contains(el)) return;
		onChange(name, readInput(el), el);
	};
	root.addEventListener("input", handler);
	root.addEventListener("change", handler);
}

/**
 * Runs native constraint validation over a block and shows what failed under
 * the control that failed it. Returns true when every control is valid, so a
 * behavior can gate its call on it.
 */
export function validateInputs(host: ParentNode): boolean {
	let ok = true;
	for (const el of controls(host)) {
		const valid = el.checkValidity();
		showInputError(el, valid ? "" : (el.getAttribute("data-gomu-input-message") ?? el.validationMessage));
		if (!valid) ok = false;
	}
	return ok;
}

/** Clears every message a previous validation or tool result left. */
export function clearInputErrors(host: ParentNode): void {
	for (const el of controls(host)) showInputError(el, "");
}

/** Shows a message under one control, by argument name. Server-side field
 * errors land here the same way client-side ones do. */
export function showInputErrors(host: ParentNode, errors: Record<string, unknown>): number {
	let n = 0;
	for (const el of controls(host)) {
		const message = errors[el.getAttribute("data-gomu-input") ?? ""];
		if (typeof message === "string" && message !== "") {
			showInputError(el, message);
			n++;
		}
	}
	return n;
}

function showInputError(el: InputControl, message: string): void {
	if (message === "") el.removeAttribute("aria-invalid");
	else el.setAttribute("aria-invalid", "true");
	const slot = el
		.closest(".gomu-desc-item")
		?.querySelector<HTMLElement>("[data-gomu-input-error]");
	if (!slot) return;
	slot.hidden = message === "";
	slot.textContent = message;
}
