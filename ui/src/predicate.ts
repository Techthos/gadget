// Row predicates: the per-record test that decides whether an action applies
// to the record it would be drawn on (Action.VisibleWhen in Go).
//
// The document is rendered once, before any record exists, so the test travels
// in the config island as data and runs here — against the raw record values,
// never against what a cell displays, which the application localises.
import { Row } from "./data";

export interface RowPredicateCfg {
	/** The record field the test reads. */
	key: string;
	/** Matches when the field holds this value. */
	equals?: unknown;
	/** Matches when the field holds any of these values. */
	in?: unknown[];
	/** Inverts the match. */
	not?: boolean;
}

/** Anything carrying a predicate — an action config, in practice. */
export interface Predicated {
	visibleWhen?: RowPredicateCfg;
}

/** A record without the field and a predicate written against null agree. */
function normalize(v: unknown): unknown {
	return v === undefined ? null : v;
}

/** Whether row satisfies p. An absent predicate is satisfied by every row. */
export function matchesRow(p: RowPredicateCfg | undefined, row: Row | null): boolean {
	if (!p || typeof p.key !== "string" || p.key === "") return true;
	const value = normalize(row ? row[p.key] : undefined);
	const wanted = Array.isArray(p.in) ? p.in : [p.equals];
	// Strict: a predicate written against 3 does not match "3". Both sides are
	// machine values, and a loose compare would let a coincidence of JSON typing
	// decide whether a button exists.
	const hit = wanted.some((w) => normalize(w) === value);
	return p.not ? !hit : hit;
}

/** Whether the action applies to the record it would be drawn on. */
export function actionVisible(action: Predicated, row: Row | null): boolean {
	return matchesRow(action.visibleWhen, row);
}

/** Whether any of the actions applies to the record — what decides that a row
 * gets an actions trigger at all. */
export function anyVisible(actions: Predicated[], row: Row | null): boolean {
	return actions.some((a) => actionVisible(a, row));
}

/**
 * The actions that apply to row, each paired with its index in the full list.
 *
 * Buttons are addressed by position and a behavior resolves a click against the
 * unfiltered list, so the index has to come from that list rather than from the
 * filtered one — otherwise every action after a hidden one fires its neighbour.
 */
export function visibleActions<T extends Predicated>(
	actions: T[],
	row: Row | null,
): { action: T; index: number }[] {
	return actions
		.map((action, index) => ({ action, index }))
		.filter(({ action }) => actionVisible(action, row));
}
