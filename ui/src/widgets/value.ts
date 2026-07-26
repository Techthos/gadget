// Typed value rendering, shared by every block that shows a record field:
// card headers and content, description lists, and the cells behind them.
// Pure DOM construction — data reaches the DOM only through textContent (via
// dom.h), never innerHTML.
import { Row } from "../data";
import { h } from "../dom";
import { formatCell } from "../format";

export interface FieldCfg {
	key: string;
	label: string;
	type: string;
	// Absent wherever sorting cannot apply, e.g. description items.
	sortable?: boolean;
	align?: string;
	format?: string;
	badge?: Record<string, string>;
	link?: { hrefKey: string; textKey?: string; text?: string };
}

/** Renders a badge span for a value, or an empty string when unmapped/blank. */
export function badgeNode(field: FieldCfg, row: Row): Node | string {
	const value = String(row[field.key] ?? "");
	if (value === "") return "";
	const variant = field.badge?.[value];
	const cls = "gadget-badge" + (variant && variant !== "neutral" ? ` gadget-badge--${variant}` : "");
	return h("span", { class: cls }, value);
}

/** Renders a field's value node by type (badge, link, or formatted text). */
export function valueNode(field: FieldCfg, row: Row): Node | string {
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
