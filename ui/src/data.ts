// JSON data-island access. IDs must match internal/htmlx island constants.
export const CONFIG_ISLAND_ID = "gomu-config";
export const DATA_ISLAND_ID = "gomu-data";

export type Row = Record<string, unknown>;

/** Parses a <script type="application/json"> island, or returns null. */
export function readIsland<T>(id: string): T | null {
  const el = document.getElementById(id);
  if (!el || el.tagName !== "SCRIPT") return null;
  try {
    return JSON.parse(el.textContent ?? "") as T;
  } catch {
    return null;
  }
}

/** Extracts the rows array from a structuredContent-shaped object. */
export function rowsFrom(
  data: Record<string, unknown> | null | undefined,
  rowsKey: string,
): Row[] {
  const v = data?.[rowsKey];
  if (!Array.isArray(v)) return [];
  return v.filter((r): r is Row => r !== null && typeof r === "object");
}
