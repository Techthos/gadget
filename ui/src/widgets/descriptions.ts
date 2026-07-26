// Descriptions: a label/value detail list. A shared block rather than a
// widget — a behavior owns the host element and calls fillDescriptions
// whenever its record changes.
//
// Values are typed and Intl-formatted exactly like table cells (valueNode
// does the work for both), or authored as fixed text in Go. Layout is CSS
// only: the items flow into as many columns as the widget's width allows.
import { Row } from "../data";
import { clear, h } from "../dom";
import { FieldCfg, valueNode } from "./value";

// A description item is a field config plus the fixed-text alternative to
// reading a record field.
export type DescriptionItemCfg = FieldCfg & { text?: string };

// Stands in for a value the record does not carry. A confirmation lists facts
// the reader decides on, so an absent one is shown as absent rather than
// dropped — and the grid keeps its shape either way.
const MISSING = "—";

/**
 * Renders items into an existing <dl>, replacing whatever it held. The list
 * is hidden while there is nothing to show, so an empty item set leaves no
 * gap in the widget.
 */
export function fillDescriptions(
	host: HTMLElement,
	items: DescriptionItemCfg[],
	row: Row | null,
): void {
	clear(host);
	host.hidden = items.length === 0;
	for (const item of items) {
		const cell = h("div", { class: "gadget-desc-item" });
		cell.append(h("dt", { class: "gadget-desc-label" }, item.label || item.key));

		const value = item.text !== undefined ? item.text : valueNode(item, row ?? {});
		const dd = h("dd", {
			class: "gadget-desc-value" + (item.align ? ` gadget-align-${item.align}` : ""),
		});
		if (value === "") {
			dd.append(document.createTextNode(MISSING));
			dd.classList.add("gadget-desc-value--missing");
		} else {
			dd.append(typeof value === "string" ? document.createTextNode(value) : value);
		}

		cell.append(dd);
		host.append(cell);
	}
}
