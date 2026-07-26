// Shared card rendering and action helpers, used by the single-record Card
// behavior and the CardList collection behavior. Pure DOM construction: data
// reaches the DOM only through textContent (via dom.h), never innerHTML.
import { Row } from "../data";
import { checkbox, h } from "../dom";
import { CallToolResult } from "../protocol";
import { DescriptionItemCfg, fillDescriptions } from "./descriptions";
import { badgeNode, FieldCfg } from "./value";

export interface ArgSourceCfg {
	static?: unknown;
	row?: string;
	selection?: string;
}

export interface ActionCfg {
	label: string;
	kind: "tool" | "link";
	tool?: string;
	args?: Record<string, ArgSourceCfg>;
	hrefKey?: string;
	confirm?: string;
	variant?: string;
}

/** The card's top section: title, description, and one badge-or-button slot. */
export interface CardHeaderCfg {
	titleKey: string;
	descriptionKey?: string;
	description?: string;
	badge?: FieldCfg;
	action?: ActionCfg;
}

/** The card's body: prose and/or a label/value detail list. */
export interface CardContentCfg {
	textKey?: string;
	text?: string;
	items?: DescriptionItemCfg[];
}

/** The card's bottom section: a note and the record's action buttons. */
export interface CardFooterCfg {
	textKey?: string;
	text?: string;
	actions?: ActionCfg[];
}

export interface CardTemplateCfg {
	header: CardHeaderCfg;
	content?: CardContentCfg;
	footer?: CardFooterCfg;
}

/**
 * Every action a card can fire, in the order their buttons are indexed:
 * the header slot first, then the footer row. A behavior resolves a clicked
 * button's data-gadget-action against this list.
 */
export function templateActions(tmpl: CardTemplateCfg): ActionCfg[] {
	const header = tmpl.header?.action ? [tmpl.header.action] : [];
	return [...header, ...(tmpl.footer?.actions ?? [])];
}

/** Row fields a card puts on screen — what a text filter matches against. */
export function templateKeys(tmpl: CardTemplateCfg): string[] {
	return [
		tmpl.header?.titleKey ?? "",
		tmpl.header?.descriptionKey ?? "",
		tmpl.content?.textKey ?? "",
		...(tmpl.content?.items ?? []).map((i) => i.key ?? ""),
	].filter((k) => k !== "");
}

/** Resolves a text slot: a record field, or the fixed words authored in Go. */
function slotText(row: Row, key: string | undefined, fixed: string | undefined): string {
	if (key) return String(row[key] ?? "");
	return fixed ?? "";
}

export interface RenderCardOpts {
	/** The record's stable id (data-gadget-card-id), used for actions. */
	id: string;
	/** When true, a selection checkbox is rendered in the header. */
	selectable?: boolean;
	/** When true, the record is currently selected. */
	selected?: boolean;
	/** When true, action buttons render disabled. */
	busy?: boolean;
}

/**
 * Builds a card element for one record: header, content and footer, in that
 * order. A section with nothing to show is left out entirely rather than
 * rendered empty, so the card keeps its spacing honest.
 */
export function renderCard(tmpl: CardTemplateCfg, row: Row, opts: RenderCardOpts): HTMLElement {
	const article = h("article", { class: "gadget-card-item", "data-gadget-card-id": opts.id });
	// Header and footer share one button index space (see templateActions).
	let actionIndex = 0;

	// --- header ---
	const header = h("header", { class: "gadget-card-item-header" });
	if (opts.selectable) {
		const cb = checkbox(
			{ "data-gadget-select-card": "", "aria-label": "Select card" },
			"gadget-card-select",
		);
		cb.input.checked = !!opts.selected;
		header.append(cb.wrap);
	}

	const heading = h("div", { class: "gadget-card-heading" });
	heading.append(h("div", { class: "gadget-card-title" }, String(row[tmpl.header.titleKey] ?? "")));
	const description = slotText(row, tmpl.header.descriptionKey, tmpl.header.description);
	if (description !== "") {
		heading.append(h("div", { class: "gadget-card-description" }, description));
	}
	header.append(heading);

	if (tmpl.header.badge) {
		const b = badgeNode(tmpl.header.badge, row);
		if (b !== "") header.append(h("div", { class: "gadget-card-action" }, b));
	} else if (tmpl.header.action) {
		header.append(
			h(
				"div",
				{ class: "gadget-card-action" },
				actionButton(tmpl.header.action, String(actionIndex++), !!opts.busy),
			),
		);
	}
	article.append(header);

	// --- content ---
	const text = slotText(row, tmpl.content?.textKey, tmpl.content?.text);
	const items = tmpl.content?.items ?? [];
	if (text !== "" || items.length > 0) {
		const content = h("div", { class: "gadget-card-content" });
		if (text !== "") content.append(h("p", { class: "gadget-card-text" }, text));
		if (items.length > 0) {
			const dl = h("dl", { class: "gadget-descriptions gadget-card-items" });
			fillDescriptions(dl, items, row);
			content.append(dl);
		}
		article.append(content);
	}

	// --- footer ---
	const note = slotText(row, tmpl.footer?.textKey, tmpl.footer?.text);
	const actions = tmpl.footer?.actions ?? [];
	if (note !== "" || actions.length > 0) {
		const footer = h("footer", { class: "gadget-card-item-footer" });
		if (note !== "") footer.append(h("span", { class: "gadget-card-note" }, note));
		if (actions.length > 0) {
			const bar = h("div", { class: "gadget-card-item-actions" });
			for (const a of actions) bar.append(actionButton(a, String(actionIndex++), !!opts.busy));
			footer.append(bar);
		}
		article.append(footer);
	}

	return article;
}

/** Renders a card action button carrying its index for event delegation. */
export function actionButton(action: ActionCfg, index: string, busy: boolean): HTMLElement {
	let cls = "gadget-btn";
	if (action.variant) cls += ` gadget-btn--${action.variant}`;
	return h(
		"button",
		{ type: "button", class: cls, "data-gadget-action": index, disabled: busy },
		action.label,
	);
}

/** Resolves an action's tool arguments from static values, the row, or the
 * current selection. */
export function resolveArgs(
	action: ActionCfg,
	row: Row | null,
	selectedRows: Row[],
): Record<string, unknown> {
	const out: Record<string, unknown> = {};
	for (const [name, src] of Object.entries(action.args ?? {})) {
		if ("static" in src) {
			out[name] = src.static;
		} else if (src.row !== undefined) {
			out[name] = row?.[src.row];
		} else if (src.selection !== undefined) {
			const field = src.selection;
			out[name] = selectedRows.map((r) => r[field]);
		}
	}
	return out;
}

/** First text content block of a tool result, if any. */
export function textOf(res: CallToolResult): string | undefined {
	for (const block of res.content ?? []) {
		if (block.type === "text" && typeof block.text === "string") return block.text;
	}
	return undefined;
}
