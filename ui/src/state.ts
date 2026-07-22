// Widget state: a minimal store plus pure, DOM-free table reducers.
import { Row } from "./data";

export class Store<S extends object> {
  private subs = new Set<(s: S) => void>();

  constructor(private state: S) {}

  get(): S {
    return this.state;
  }

  set(patch: Partial<S>): void {
    this.state = { ...this.state, ...patch };
    for (const fn of this.subs) fn(this.state);
  }

  subscribe(fn: (s: S) => void): () => void {
    this.subs.add(fn);
    return () => this.subs.delete(fn);
  }
}

export interface SortSpec {
  key: string;
  desc: boolean;
}

/**
 * Total order over cell values: numbers (and numeric strings) numerically,
 * everything else as case-insensitive strings; null/undefined sort last.
 */
export function compareValues(a: unknown, b: unknown): number {
  const aNil = a === null || a === undefined || a === "";
  const bNil = b === null || b === undefined || b === "";
  if (aNil && bNil) return 0;
  if (aNil) return 1;
  if (bNil) return -1;

  const an = typeof a === "number" ? a : Number(a);
  const bn = typeof b === "number" ? b : Number(b);
  if (Number.isFinite(an) && Number.isFinite(bn)) {
    return an < bn ? -1 : an > bn ? 1 : 0;
  }
  return String(a).localeCompare(String(b), undefined, { sensitivity: "base" });
}

/** Stable sort; returns a new array, input untouched. */
export function sortRows(rows: Row[], sort: SortSpec | null): Row[] {
  if (!sort) return rows.slice();
  const sign = sort.desc ? -1 : 1;
  return rows
    .map((row, i) => ({ row, i }))
    .sort((a, b) => {
      const c = compareValues(a.row[sort.key], b.row[sort.key]);
      return c !== 0 ? sign * c : a.i - b.i;
    })
    .map((e) => e.row);
}

/** Case-insensitive substring match across the given keys. */
export function filterRows(rows: Row[], query: string, keys: string[]): Row[] {
  const q = query.trim().toLowerCase();
  if (!q) return rows.slice();
  return rows.filter((row) =>
    keys.some((k) => {
      const v = row[k];
      return v !== null && v !== undefined && String(v).toLowerCase().includes(q);
    }),
  );
}

export function pageCount(total: number, pageSize: number): number {
  if (pageSize <= 0) return 1;
  return Math.max(1, Math.ceil(total / pageSize));
}

export function clampPage(page: number, total: number, pageSize: number): number {
  return Math.min(Math.max(0, page), pageCount(total, pageSize) - 1);
}

export function pageSlice<T>(items: T[], page: number, pageSize: number): T[] {
  if (pageSize <= 0) return items.slice();
  const p = clampPage(page, items.length, pageSize);
  return items.slice(p * pageSize, (p + 1) * pageSize);
}
