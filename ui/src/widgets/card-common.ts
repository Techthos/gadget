// Shared card rendering and action helpers, used by the single-record Card
// behavior and the CardList collection behavior. Pure DOM construction: data
// reaches the DOM only through textContent (via dom.h), never innerHTML.
import { Row } from "../data";
import { checkbox, h } from "../dom";
import {
	collectInputs,
	DescriptionItemCfg,
	fillDescriptions,
	type InputValues,
} from "./descriptions";
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
	/** Posts this as a user turn instead of calling tool. See Action.Prompt. */
	prompt?: string;
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
 * button's data-gomu-action against this list.
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
	/** The record's stable id (data-gomu-card-id), used for actions. */
	id: string;
	/** When true, a selection checkbox is rendered in the header. */
	selectable?: boolean;
	/** When true, the record is currently selected. */
	selected?: boolean;
	/** When true, action buttons render disabled. */
	busy?: boolean;
	/** What this record's content controls hold, kept across re-renders by the
	 * behavior (a card is rebuilt wholesale on every state change). */
	values?: InputValues;
}

/**
 * Builds a card element for one record: header, content and footer, in that
 * order. A section with nothing to show is left out entirely rather than
 * rendered empty, so the card keeps its spacing honest.
 */
export function renderCard(tmpl: CardTemplateCfg, row: Row, opts: RenderCardOpts): HTMLElement {
	const article = h("article", { class: "gomu-card-item", "data-gomu-card-id": opts.id });
	// Header and footer share one button index space (see templateActions).
	let actionIndex = 0;

	// --- header ---
	const header = h("header", { class: "gomu-card-item-header" });
	if (opts.selectable) {
		const cb = checkbox(
			{ "data-gomu-select-card": "", "aria-label": "Select card" },
			"gomu-card-select",
		);
		cb.input.checked = !!opts.selected;
		header.append(cb.wrap);
	}

	const heading = h("div", { class: "gomu-card-heading" });
	heading.append(h("div", { class: "gomu-card-title" }, String(row[tmpl.header.titleKey] ?? "")));
	const description = slotText(row, tmpl.header.descriptionKey, tmpl.header.description);
	if (description !== "") {
		heading.append(h("div", { class: "gomu-card-description" }, description));
	}
	header.append(heading);

	if (tmpl.header.badge) {
		const b = badgeNode(tmpl.header.badge, row);
		if (b !== "") header.append(h("div", { class: "gomu-card-action" }, b));
	} else if (tmpl.header.action) {
		header.append(
			h(
				"div",
				{ class: "gomu-card-action" },
				actionButton(tmpl.header.action, String(actionIndex++), !!opts.busy),
			),
		);
	}
	article.append(header);

	// --- content ---
	const text = slotText(row, tmpl.content?.textKey, tmpl.content?.text);
	const items = tmpl.content?.items ?? [];
	if (text !== "" || items.length > 0) {
		const content = h("div", { class: "gomu-card-content" });
		if (text !== "") content.append(h("p", { class: "gomu-card-text" }, text));
		if (items.length > 0) {
			const dl = h("dl", { class: "gomu-descriptions gomu-card-items" });
			fillDescriptions(dl, items, row, { values: opts.values, disabled: !!opts.busy });
			content.append(dl);
		}
		article.append(content);
	}

	// --- footer ---
	const note = slotText(row, tmpl.footer?.textKey, tmpl.footer?.text);
	const actions = tmpl.footer?.actions ?? [];
	if (note !== "" || actions.length > 0) {
		const footer = h("footer", { class: "gomu-card-item-footer" });
		if (note !== "") footer.append(h("span", { class: "gomu-card-note" }, note));
		if (actions.length > 0) {
			const bar = h("div", { class: "gomu-card-item-actions" });
			for (const a of actions) bar.append(actionButton(a, String(actionIndex++), !!opts.busy));
			footer.append(bar);
		}
		article.append(footer);
	}

	return article;
}

/** Renders a card action button carrying its index for event delegation. */
export function actionButton(action: ActionCfg, index: string, busy: boolean): HTMLElement {
	let cls = "gomu-btn";
	if (action.variant) cls += ` gomu-btn--${action.variant}`;
	return h(
		"button",
		{ type: "button", class: cls, "data-gomu-action": index, disabled: busy },
		action.label,
	);
}

/**
 * The arguments a card's own controls contribute to an action fired from it.
 * A bulk action gets none: what a control holds belongs to the one record it
 * was rendered under, and a selection spans many.
 */
export function cardInputs(el: Element | null): InputValues {
	const card = el?.closest("[data-gomu-card-id]");
	return card ? collectInputs(card) : {};
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
