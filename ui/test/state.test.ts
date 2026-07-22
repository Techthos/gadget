import { describe, expect, it } from "vitest";
import {
  clampPage,
  compareValues,
  filterRows,
  pageCount,
  pageSlice,
  sortRows,
  Store,
} from "../src/state";

describe("compareValues", () => {
  it("orders numbers numerically, including numeric strings", () => {
    expect(compareValues(2, 10)).toBeLessThan(0);
    expect(compareValues("2", "10")).toBeLessThan(0);
    expect(compareValues("10", 2)).toBeGreaterThan(0);
  });

  it("orders strings case-insensitively", () => {
    expect(compareValues("apple", "Banana")).toBeLessThan(0);
    expect(compareValues("Apple", "apple")).toBe(0);
  });

  it("sorts null/undefined/empty last", () => {
    expect(compareValues(null, "a")).toBeGreaterThan(0);
    expect(compareValues("a", undefined)).toBeLessThan(0);
    expect(compareValues("", null)).toBe(0);
  });
});

describe("sortRows", () => {
  const rows = [
    { id: 1, name: "Carol", age: 30 },
    { id: 2, name: "alice", age: 25 },
    { id: 3, name: "Bob", age: 30 },
  ];

  it("sorts ascending and descending without mutating input", () => {
    const asc = sortRows(rows, { key: "name", desc: false });
    expect(asc.map((r) => r.id)).toEqual([2, 3, 1]);
    const desc = sortRows(rows, { key: "name", desc: true });
    expect(desc.map((r) => r.id)).toEqual([1, 3, 2]);
    expect(rows[0]!.id).toBe(1); // untouched
  });

  it("is stable for equal keys", () => {
    const byAge = sortRows(rows, { key: "age", desc: false });
    expect(byAge.map((r) => r.id)).toEqual([2, 1, 3]); // 30s keep input order
  });

  it("returns a copy when sort is null", () => {
    const out = sortRows(rows, null);
    expect(out).toEqual(rows);
    expect(out).not.toBe(rows);
  });
});

describe("filterRows", () => {
  const rows = [
    { name: "Ada Lovelace", role: "eng" },
    { name: "Grace Hopper", role: "admiral" },
  ];

  it("matches case-insensitive substrings across keys", () => {
    expect(filterRows(rows, "hopper", ["name"])).toHaveLength(1);
    expect(filterRows(rows, "ADMIRAL", ["name", "role"])).toHaveLength(1);
    expect(filterRows(rows, "zzz", ["name", "role"])).toHaveLength(0);
  });

  it("returns all rows for empty/whitespace queries", () => {
    expect(filterRows(rows, "  ", ["name"])).toHaveLength(2);
  });
});

describe("pagination", () => {
  it("computes page counts", () => {
    expect(pageCount(0, 10)).toBe(1);
    expect(pageCount(10, 10)).toBe(1);
    expect(pageCount(11, 10)).toBe(2);
    expect(pageCount(5, 0)).toBe(1); // pagination disabled
  });

  it("clamps pages into range", () => {
    expect(clampPage(-1, 25, 10)).toBe(0);
    expect(clampPage(99, 25, 10)).toBe(2);
  });

  it("slices pages and returns everything when disabled", () => {
    const items = Array.from({ length: 25 }, (_, i) => i);
    expect(pageSlice(items, 2, 10)).toEqual([20, 21, 22, 23, 24]);
    expect(pageSlice(items, 0, 0)).toHaveLength(25);
  });
});

describe("Store", () => {
  it("notifies subscribers on set and supports unsubscribe", () => {
    const store = new Store({ n: 0, s: "x" });
    const seen: number[] = [];
    const off = store.subscribe((st) => seen.push(st.n));
    store.set({ n: 1 });
    store.set({ n: 2 });
    off();
    store.set({ n: 3 });
    expect(seen).toEqual([1, 2]);
    expect(store.get()).toEqual({ n: 3, s: "x" });
  });
});
