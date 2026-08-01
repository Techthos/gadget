import { describe, expect, it } from "vitest";
import { actionVisible, anyVisible, matchesRow, visibleActions } from "../src/predicate";

const ROW = { id: 1, state: "paused", enabled: true, retries: 0, note: null };

describe("row predicates", () => {
	it("has no opinion when there is no predicate", () => {
		expect(matchesRow(undefined, ROW)).toBe(true);
		expect(actionVisible({}, ROW)).toBe(true);
		expect(actionVisible({}, null)).toBe(true);
	});

	it("matches equality on raw values, by type", () => {
		expect(matchesRow({ key: "state", equals: "paused" }, ROW)).toBe(true);
		expect(matchesRow({ key: "state", equals: "running" }, ROW)).toBe(false);
		expect(matchesRow({ key: "enabled", equals: true }, ROW)).toBe(true);
		expect(matchesRow({ key: "retries", equals: 0 }, ROW)).toBe(true);
		// A number is not its own printed form: both sides are machine values.
		expect(matchesRow({ key: "retries", equals: "0" }, ROW)).toBe(false);
		expect(matchesRow({ key: "enabled", equals: "true" }, ROW)).toBe(false);
	});

	it("matches a set, and its complement", () => {
		expect(matchesRow({ key: "state", in: ["paused", "failed"] }, ROW)).toBe(true);
		expect(matchesRow({ key: "state", in: ["running", "failed"] }, ROW)).toBe(false);
		expect(matchesRow({ key: "state", in: ["paused", "failed"], not: true }, ROW)).toBe(false);
		expect(matchesRow({ key: "state", equals: "running", not: true }, ROW)).toBe(true);
	});

	it("reads a missing field and a missing record as null", () => {
		expect(matchesRow({ key: "missing", equals: null }, ROW)).toBe(true);
		expect(matchesRow({ key: "note", equals: null }, ROW)).toBe(true);
		expect(matchesRow({ key: "state", equals: "paused" }, null)).toBe(false);
		expect(matchesRow({ key: "state", equals: "paused", not: true }, null)).toBe(true);
	});

	it("keeps each surviving action's index in the full list", () => {
		const actions = [
			{ label: "Activate", visibleWhen: { key: "state", equals: "paused" } },
			{ label: "Pause", visibleWhen: { key: "state", equals: "running" } },
			{ label: "Edit" },
			{ label: "Delete" },
		];
		const running = visibleActions(actions, { state: "running" });
		expect(running.map((v) => [v.action.label, v.index])).toEqual([
			["Pause", 1],
			["Edit", 2],
			["Delete", 3],
		]);
		expect(anyVisible(actions, { state: "running" })).toBe(true);
	});

	it("reports a record no action applies to", () => {
		const actions = [
			{ label: "Activate", visibleWhen: { key: "state", equals: "paused" } },
			{ label: "Pause", visibleWhen: { key: "state", equals: "running" } },
		];
		expect(visibleActions(actions, { state: "archived" })).toEqual([]);
		expect(anyVisible(actions, { state: "archived" })).toBe(false);
	});
});
