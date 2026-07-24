// Shared card rendering and action helpers, used by the single-record Card
// behavior and the CardList collection behavior. Pure DOM construction: data
// reaches the DOM only through textContent (via dom.h), never innerHTML.
import { Row } from "../data";
import { h } from "../dom";
import { formatCell } from "../format";
import { CallToolResult } from "../protocol";

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

export interface FieldCfg {
	key: string;
	label: string;
	type: string;
	sortable: boolean;
	align?: string;
	format?: string;
	badge?: Record<string, string>;
	link?: { hrefKey: string; textKey?: string; text?: string };
}

export interface CardTemplateCfg {
	titleKey: string;
	subtitleKey?: string;
	badge?: FieldCfg;
	fields: FieldCfg[];
	actions?: ActionCfg[];
}

/** Renders a badge span for a value, or an empty string when unmapped/blank. */
function badgeNode(field: FieldCfg, row: Row): Node | string {
	const value = String(row[field.key] ?? "");
	if (value === "") return "";
	const variant = field.badge?.[value];
	const cls = "gadget-badge" + (variant && variant !== "neutral" ? ` gadget-badge--${variant}` : "");
	return h("span", { class: cls }, value);
}

/** Renders a field's value node by type (badge, link, or formatted text). */
function valueNode(field: FieldCfg, row: Row): Node | string {
	switch (field.type) {
		case "badge":
			return badgeNode(field, row);
		case "link": {
			const href = row[field.link?.hrefKey ?? field.key];
			if (typeof href !== "string" || href === "") return "";
			const textKey = field.link?.textKey;
			const text =
				(textKey !== undefined ? String(row[textKey] ?? "") : "") || field.link?.text || href;
			return h("button", { type: "button", class: "gadget-link", "data-gadget-link": href }, text);
		}
		default:
			return formatCell(row[field.key], field.type, field.format);
	}
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

/** Builds a card element for one record. */
export function renderCard(tmpl: CardTemplateCfg, row: Row, opts: RenderCardOpts): HTMLElement {
	const article = h("article", { class: "gadget-card-item", "data-gadget-card-id": opts.id });

	const header = h("div", { class: "gadget-card-item-header" });
	if (opts.selectable) {
		const cb = h("input", {
			type: "checkbox",
			class: "gadget-card-select",
			"data-gadget-select-card": "",
			"aria-label": "Select card",
		}) as HTMLInputElement;
		cb.checked = !!opts.selected;
		header.append(cb);
	}

	const heading = h("div", { class: "gadget-card-heading" });
	heading.append(h("div", { class: "gadget-card-title" }, String(row[tmpl.titleKey] ?? "")));
	if (tmpl.subtitleKey) {
		const sub = String(row[tmpl.subtitleKey] ?? "");
		if (sub !== "") heading.append(h("div", { class: "gadget-card-subtitle" }, sub));
	}
	header.append(heading);

	if (tmpl.badge) {
		const b = badgeNode(tmpl.badge, row);
		if (b !== "") header.append(h("div", { class: "gadget-card-badge" }, b));
	}
	article.append(header);

	if (tmpl.fields.length > 0) {
		const dl = h("dl", { class: "gadget-card-fields" });
		for (const field of tmpl.fields) {
			const val = valueNode(field, row);
			const row_ = h("div", { class: "gadget-card-field" });
			row_.append(h("dt", {}, field.label || field.key));
			const dd = h("dd", { class: field.align ? `gadget-align-${field.align}` : null });
			if (val !== "") dd.append(typeof val === "string" ? document.createTextNode(val) : val);
			row_.append(dd);
			dl.append(row_);
		}
		article.append(dl);
	}

	if (tmpl.actions && tmpl.actions.length > 0) {
		const bar = h("div", { class: "gadget-card-item-actions" });
		tmpl.actions.forEach((a, i) => bar.append(actionButton(a, String(i), !!opts.busy)));
		article.append(bar);
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
