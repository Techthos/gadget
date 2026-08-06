// The date grid every date control in the bundle is built from: the inline
// calendar of the DatePicker widget, and the popover a form's date fields open.
// One implementation, two mountings — see createCalendar and enhanceDateFields.
//
// Dates are calendar days, not instants. They travel as "YYYY-MM-DD" strings
// and are held internally as UTC midnights, so no arithmetic here can be moved
// a day by a time zone or a daylight-saving jump. The host's time zone is used
// for exactly one thing: deciding which day is today.
//
// Month names, weekday names and the first day of the week come from the host
// locale, which is not known until the handshake — which is why the grid is
// built at runtime rather than server-rendered, and why it rebuilds when the
// host context changes.
//
// Data reaches the DOM through textContent only (see dom.ts).
import { clear, h, icon } from "./dom";
import { getLocale, getTimeZone } from "./format";
import { enhanceSelects, refreshDropdown } from "./dropdown";
import { mountOverlay, openPopup, popupHost, releasePopup, type Popup } from "./popup";

const DAY_MS = 86400000;
const CHEVRON_LEFT = "M10 3 5.5 8 10 13";
const CHEVRON_RIGHT = "M6 3 10.5 8 6 13";
// A calendar page: the sheet, its two hangers, and the rule under the header.
const CALENDAR_ICON = ["M3.5 4.5h9a1 1 0 0 1 1 1v7a1 1 0 0 1-1 1h-9a1 1 0 0 1-1-1v-7a1 1 0 0 1 1-1z", "M5.5 2.5v3M10.5 2.5v3", "M2.5 7.5h11"];
// The Sunday the weekday headers are named from; any Sunday would do.
const REF_SUNDAY = Date.UTC(2024, 0, 7);

export interface PresetCfg {
  label: string;
  span?: string;
  start?: string;
  end?: string;
}

/** The grid's configuration, as Go's Calendar serializes it. */
export interface CalendarCfg {
  mode?: string;
  months?: number;
  min?: string;
  max?: string;
  disabled?: string[];
  disableWeekends?: boolean;
  weekNumbers?: boolean;
  monthDropdowns?: boolean;
  fromYear?: number;
  toYear?: number;
  weekStart?: string;
  startOn?: string;
  presets?: PresetCfg[];
}

/** A selection. Both ends are "" when nothing is picked; a single-date
 * calendar only ever fills start. */
export interface DateValue {
  start: string;
  end: string;
}

export interface CalendarOptions {
  /** Build into this element instead of a fresh one — the widget's grid is
   * server-rendered as an empty host. */
  host?: HTMLElement;
  /** Offers a button that empties the selection (an optional form field). */
  clearable?: boolean;
  /** Every change: a first click in a range reports complete false. */
  onChange?: (value: DateValue, complete: boolean) => void;
  /** A selection the reader has finished — the moment a popover should close. */
  onDone?: () => void;
}

export interface CalendarView {
  readonly el: HTMLElement;
  value(): DateValue;
  /** Writes the selection without reporting a change, and moves the grid to
   * the month holding it. */
  setValue(v: Partial<DateValue>): void;
  /** Merges runtime configuration — bounds and blocked days a tool result
   * carries — over the authored grid. */
  patch(cfg: Partial<CalendarCfg>): void;
  /** True when the selection is something the reader could submit. */
  complete(): boolean;
  render(): void;
  /** Puts keyboard focus in the grid, on the selection or on today. */
  focusGrid(): void;
}

// --- Dates as days ------------------------------------------------------

function pad(n: number, width: number): string {
  return String(Math.abs(n)).padStart(width, "0");
}

/** UTC midnight of a "YYYY-MM-DD" date, or null for anything else. Rejects
 * dates that do not exist ("2026-02-31") rather than rolling them forward. */
export function parseISO(s: unknown): number | null {
  if (typeof s !== "string") return null;
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s.trim());
  if (!m) return null;
  const y = Number(m[1]);
  const mo = Number(m[2]);
  const d = Number(m[3]);
  if (mo < 1 || mo > 12 || d < 1 || d > 31) return null;
  const dt = new Date(0);
  dt.setUTCFullYear(y, mo - 1, d);
  dt.setUTCHours(0, 0, 0, 0);
  if (dt.getUTCMonth() !== mo - 1 || dt.getUTCDate() !== d) return null;
  return dt.getTime();
}

export function toISO(ms: number): string {
  const d = new Date(ms);
  return `${pad(d.getUTCFullYear(), 4)}-${pad(d.getUTCMonth() + 1, 2)}-${pad(d.getUTCDate(), 2)}`;
}

function addDays(ms: number, n: number): number {
  return ms + n * DAY_MS;
}

/** The same day-of-month n months away, clamped to the end of a shorter month
 * (31 January plus one month is 28 February, not 3 March). */
function addMonths(ms: number, n: number): number {
  const d = new Date(ms);
  const day = d.getUTCDate();
  d.setUTCDate(1);
  d.setUTCMonth(d.getUTCMonth() + n);
  d.setUTCDate(Math.min(day, daysInMonth(d.getUTCFullYear(), d.getUTCMonth())));
  return d.getTime();
}

function daysInMonth(year: number, month: number): number {
  const d = new Date(0);
  d.setUTCFullYear(year, month + 1, 0);
  return d.getUTCDate();
}

function startOfMonth(ms: number): number {
  const d = new Date(ms);
  d.setUTCDate(1);
  return d.getTime();
}

/** ISO 8601 week number: weeks start Monday, week 1 holds 4 January. */
function isoWeek(ms: number): number {
  const target = new Date(ms);
  target.setUTCDate(target.getUTCDate() - ((target.getUTCDay() + 6) % 7) + 3);
  const first = new Date(0);
  first.setUTCFullYear(target.getUTCFullYear(), 0, 4);
  first.setUTCDate(first.getUTCDate() - ((first.getUTCDay() + 6) % 7) + 3);
  return 1 + Math.round((target.getTime() - first.getTime()) / (7 * DAY_MS));
}

/**
 * Today, in the host's time zone. The reader's today is the one that matters —
 * a widget read in Auckland must not ring yesterday because the server is in
 * Berlin — and the host names its zone in hostContext.
 */
export function todayISO(): string {
  const now = new Date();
  const tz = getTimeZone();
  if (tz) {
    try {
      const parts = new Intl.DateTimeFormat("en-US", {
        timeZone: tz,
        calendar: "gregory",
        year: "numeric",
        month: "2-digit",
        day: "2-digit",
      }).formatToParts(now);
      const at = (type: string): string => parts.find((p) => p.type === type)?.value ?? "";
      const y = at("year");
      const m = at("month");
      const d = at("day");
      if (y && m && d) return `${pad(Number(y), 4)}-${m}-${d}`;
    } catch {
      // An unknown zone falls through to the system's own day.
    }
  }
  return `${pad(now.getFullYear(), 4)}-${pad(now.getMonth() + 1, 2)}-${pad(now.getDate(), 2)}`;
}

/** Resolves a preset to the window it names, against the reader's today. */
export function resolvePreset(p: PresetCfg, mode: "single" | "range"): DateValue | null {
  if (p.span === undefined || p.span === "") {
    const start = parseISO(p.start) === null ? "" : (p.start as string);
    if (start === "") return null;
    const end = mode === "range" && parseISO(p.end) !== null ? (p.end as string) : "";
    return { start, end };
  }
  const today = parseISO(todayISO()) as number;
  const span = (from: number, to: number): DateValue =>
    mode === "range"
      ? { start: toISO(Math.min(from, to)), end: toISO(Math.max(from, to)) }
      : // A single-date calendar takes the day the window opens on: "last 7
        // days" is one date's worth of shortcut there, and it is the earliest.
        { start: toISO(Math.min(from, to)), end: "" };

  // Monday-based week, independent of where the grid draws the week break: a
  // "this week" that moved with the locale would name a different span in two
  // otherwise identical widgets.
  const monday = addDays(today, -((new Date(today).getUTCDay() + 6) % 7));
  const first = startOfMonth(today);
  const yearStart = (offset: number): number => {
    const d = new Date(today);
    d.setUTCFullYear(d.getUTCFullYear() + offset, 0, 1);
    return d.getTime();
  };

  switch (p.span) {
    case "today":
      return span(today, today);
    case "yesterday":
      return span(addDays(today, -1), addDays(today, -1));
    case "tomorrow":
      return span(addDays(today, 1), addDays(today, 1));
    case "last-7-days":
      return span(addDays(today, -6), today);
    case "last-30-days":
      return span(addDays(today, -29), today);
    case "last-90-days":
      return span(addDays(today, -89), today);
    case "next-7-days":
      return span(today, addDays(today, 6));
    case "next-30-days":
      return span(today, addDays(today, 29));
    case "this-week":
      return span(monday, addDays(monday, 6));
    case "last-week":
      return span(addDays(monday, -7), addDays(monday, -1));
    case "this-month":
      return span(first, addMonths(first, 1) - DAY_MS);
    case "last-month":
      return span(addMonths(first, -1), first - DAY_MS);
    case "this-year":
      return span(yearStart(0), yearStart(1) - DAY_MS);
    case "year-to-date":
      return span(yearStart(0), today);
    default:
      return null;
  }
}

// --- Locale text --------------------------------------------------------

// Intl objects are not cheap and a grid asks for a lot of them; they are keyed
// by locale so a host-context change replaces them rather than reusing stale
// ones.
// null until the first build, so the first call always creates them.
let fmtLocale: string | null = null;
let fmtMonth!: Intl.DateTimeFormat;
let fmtWeekday!: Intl.DateTimeFormat;
let fmtWeekdayLong!: Intl.DateTimeFormat;
let fmtFull!: Intl.DateTimeFormat;
let fmtMedium!: Intl.DateTimeFormat;

function formatters(): void {
  const locale = getLocale() ?? "";
  if (locale === fmtLocale) return;
  fmtLocale = locale;
  const loc = locale === "" ? undefined : locale;
  const utc = { timeZone: "UTC" } as const;
  fmtMonth = new Intl.DateTimeFormat(loc, { ...utc, month: "long", year: "numeric" });
  fmtWeekday = new Intl.DateTimeFormat(loc, { ...utc, weekday: "narrow" });
  fmtWeekdayLong = new Intl.DateTimeFormat(loc, { ...utc, weekday: "long" });
  fmtFull = new Intl.DateTimeFormat(loc, { ...utc, dateStyle: "full" });
  fmtMedium = new Intl.DateTimeFormat(loc, { ...utc, dateStyle: "medium" });
}

/** A date as the reader reads it: "20 Jan 2026". */
export function formatDate(iso: string): string {
  const ms = parseISO(iso);
  if (ms === null) return "";
  formatters();
  return fmtMedium.format(new Date(ms));
}

/** A window as the reader reads it, using the locale's own range form where
 * the engine has one ("20 Jan – 9 Feb 2026"). */
export function formatDateRange(startISO: string, endISO: string): string {
  const a = parseISO(startISO);
  const b = parseISO(endISO);
  if (a === null) return "";
  if (b === null) return formatDate(startISO);
  formatters();
  const range = (fmtMedium as unknown as { formatRange?: (a: Date, b: Date) => string })
    .formatRange;
  if (typeof range === "function") {
    try {
      return range.call(fmtMedium, new Date(a), new Date(b));
    } catch {
      // Fall through to the two dates spelled out.
    }
  }
  return `${formatDate(startISO)} – ${formatDate(endISO)}`;
}

/** The name of a month, for the caption dropdown. Any year would do; the names
 * do not depend on it. */
function monthName(index: number): string {
  const locale = getLocale();
  const d = new Date(0);
  d.setUTCFullYear(2024, index, 1);
  return new Intl.DateTimeFormat(locale === "" ? undefined : locale, {
    timeZone: "UTC",
    month: "long",
  }).format(d);
}

// --- The grid -----------------------------------------------------------

export function createCalendar(cfg: CalendarCfg, opts: CalendarOptions = {}): CalendarView {
  const el = opts.host ?? h("div", { class: "gomu-cal" });
  const mode: "single" | "range" = cfg.mode === "range" ? "range" : "single";

  let conf: CalendarCfg = { ...cfg };
  let blocked = new Set(conf.disabled ?? []);
  let sel: DateValue = { start: "", end: "" };
  // The month in the leftmost column, as its first day.
  let view = startOfMonth(parseISO(conf.startOn) ?? parseISO(todayISO()) ?? Date.now());
  // The day the keyboard is on. Also what a rebuild restores focus to.
  let focus = view;
  // While a range is half-made: the day it was started from, and the day the
  // pointer is over, so the span the next click would make is visible.
  let anchor = "";
  let hover = "";
  let dropdownsEnhanced = false;
  // Intl.Locale is not free either, and the week break is asked for once per
  // rendered month.
  let weekStartCache: { locale: string; day: number } | null = null;

  const months = Math.min(4, Math.max(1, conf.months ?? (mode === "range" ? 2 : 1)));

  // Built once. A rebuild would orphan the popup panels of the caption
  // dropdowns, which hang off the widget root rather than the header.
  const nav = { prev: navButton("prev"), next: navButton("next") };
  const caption = h("div", { class: "gomu-cal-caption" });
  const monthSelect = h("select", {
    class: "gomu-input gomu-cal-select gomu-cal-select--month",
    "aria-label": "Month",
  }) as HTMLSelectElement;
  const yearSelect = h("select", {
    class: "gomu-input gomu-cal-select gomu-cal-select--year",
    "aria-label": "Year",
  }) as HTMLSelectElement;
  const monthsEl = h("div", { class: "gomu-cal-months" });

  el.classList.add("gomu-cal");
  el.classList.add(`gomu-cal--${mode}`);
  el.classList.add(`gomu-cal--months-${months}`);

  if (conf.monthDropdowns) {
    for (let m = 0; m < 12; m++) {
      monthSelect.append(h("option", { value: String(m) }, monthName(m)));
    }
    const from = conf.fromYear ?? new Date().getUTCFullYear() - 100;
    const to = Math.max(from, conf.toYear ?? new Date().getUTCFullYear() + 10);
    for (let y = from; y <= to; y++) {
      yearSelect.append(h("option", { value: String(y) }, String(y)));
    }
    caption.append(monthSelect, yearSelect);
  }

  const header = h("div", { class: "gomu-cal-header" }, nav.prev, caption, nav.next);
  const main = h("div", { class: "gomu-cal-main" }, header, monthsEl);
  if (conf.presets?.length) el.append(presetsNode(conf.presets));
  el.append(main);
  if (opts.clearable) {
    main.append(
      h(
        "div",
        { class: "gomu-cal-foot" },
        h(
          "button",
          { type: "button", class: "gomu-btn gomu-cal-clear", "data-gomu-cal-clear": "" },
          "Clear",
        ),
      ),
    );
  }

  function navButton(dir: "prev" | "next"): HTMLButtonElement {
    const b = h("button", {
      type: "button",
      class: `gomu-cal-nav gomu-cal-nav--${dir}`,
      "aria-label": dir === "prev" ? "Previous month" : "Next month",
      "data-gomu-cal-nav": dir,
    }) as HTMLButtonElement;
    b.append(icon("gomu-cal-nav-icon", dir === "prev" ? CHEVRON_LEFT : CHEVRON_RIGHT));
    return b;
  }

  function presetsNode(presets: PresetCfg[]): HTMLElement {
    const wrap = h("div", { class: "gomu-cal-presets" });
    presets.forEach((p, i) => {
      wrap.append(
        h(
          "button",
          {
            type: "button",
            class: "gomu-btn gomu-cal-preset",
            "data-gomu-cal-preset": String(i),
          },
          p.label,
        ),
      );
    });
    return wrap;
  }

  // --- What may be picked ---

  function minMs(): number | null {
    return parseISO(conf.min);
  }

  function maxMs(): number | null {
    return parseISO(conf.max);
  }

  function isDisabled(ms: number): boolean {
    const lo = minMs();
    const hi = maxMs();
    if (lo !== null && ms < lo) return true;
    if (hi !== null && ms > hi) return true;
    if (blocked.has(toISO(ms))) return true;
    if (conf.disableWeekends) {
      const day = new Date(ms).getUTCDay();
      if (day === 0 || day === 6) return true;
    }
    return false;
  }

  /** True when every day from a to b may be picked. A span may not straddle a
   * blocked day: a booking over a day that is taken is not a booking. */
  function spanFree(a: number, b: number): boolean {
    for (let ms = Math.min(a, b); ms <= Math.max(a, b); ms = addDays(ms, 1)) {
      if (isDisabled(ms)) return false;
    }
    return true;
  }

  /** The first selectable day from ms in direction dir, ms itself included.
   * Gives up after four years rather than walking to the end of time. */
  function nearest(ms: number, dir: 1 | -1): number {
    for (let i = 0, at = ms; i < 366 * 4; i++, at = addDays(at, dir)) {
      if (!isDisabled(at)) return at;
    }
    return ms;
  }

  // --- Travel ---

  /** The window the leftmost month may sit in, so the grid never shows only
   * months in which nothing can be picked. */
  function viewBounds(): { lo: number | null; hi: number | null } {
    const lo = minMs();
    const hi = maxMs();
    return {
      lo: lo === null ? null : startOfMonth(lo),
      hi: hi === null ? null : addMonths(startOfMonth(hi), -(months - 1)),
    };
  }

  function setView(ms: number): void {
    const { lo, hi } = viewBounds();
    let next = startOfMonth(ms);
    if (hi !== null && next > hi) next = hi;
    if (lo !== null && next < lo) next = lo;
    view = next;
  }

  /** Brings ms into view: the grid only moves when the day is off it, so
   * arrowing within the shown months does not shift the columns. */
  function reveal(ms: number): void {
    const first = view;
    const last = addMonths(view, months - 1);
    if (ms < first) setView(ms);
    else if (ms > addMonths(last, 1) - DAY_MS) setView(addMonths(startOfMonth(ms), -(months - 1)));
  }

  // --- Picking ---

  function report(complete: boolean): void {
    opts.onChange?.(value(), complete);
    if (complete) opts.onDone?.();
  }

  function pick(iso: string): void {
    const ms = parseISO(iso);
    if (ms === null || isDisabled(ms)) return;
    focus = ms;
    if (mode === "single") {
      sel = { start: iso, end: "" };
      anchor = "";
      hover = "";
      render();
      report(true);
      return;
    }
    // A range in three states: nothing yet, half-made, and made. A click on a
    // made range starts a new one — the alternative, growing whichever end is
    // nearer, guesses at what the reader meant.
    const half = anchor !== "" && sel.start !== "" && sel.end === "";
    const from = parseISO(anchor);
    if (!half || from === null || !spanFree(from, ms)) {
      anchor = iso;
      hover = "";
      sel = { start: iso, end: "" };
      render();
      report(false);
      return;
    }
    sel = { start: toISO(Math.min(from, ms)), end: toISO(Math.max(from, ms)) };
    anchor = "";
    hover = "";
    render();
    report(true);
  }

  /**
   * The selection a shortcut would make, or null where it would make none.
   *
   * A span that runs past the ends of what may be picked is trimmed to them
   * rather than refused: against a calendar that opens on the 1st, "last 30
   * days" is the days of it there are. Refusing it left a shortcut that looked
   * live and did nothing at all, which is how a calendar whose Min is a day
   * after today ends up with a rail of dead buttons.
   *
   * Two things are never trimmed. A span that straddles a blocked day is not a
   * span the reader could have drawn by hand, and handing back a shorter one
   * names a window nobody asked for. And a shortcut that picks a single day
   * picks that day or nothing: a "Today" that quietly picks the 1st because
   * today is before Min is not the day it says it is.
   */
  function presetWindow(p: PresetCfg): DateValue | null {
    const window = resolvePreset(p, mode);
    if (!window) return null;
    const lo = minMs();
    const hi = maxMs();
    const clamp = (ms: number): number => {
      if (lo !== null && ms < lo) return lo;
      if (hi !== null && ms > hi) return hi;
      return ms;
    };
    let start = parseISO(window.start);
    if (start === null) return null;
    let end = mode === "range" ? parseISO(window.end) : null;
    if (end === null) {
      return isDisabled(start) ? null : { start: toISO(start), end: "" };
    }
    // A window wholly outside the bounds collapses onto one end of them, which
    // is a day the reader never asked for; only an overlap survives.
    if ((lo !== null && end < lo) || (hi !== null && start > hi)) return null;
    start = clamp(start);
    end = clamp(end);
    if (!spanFree(start, end)) return null;
    return { start: toISO(start), end: toISO(end) };
  }

  function usePreset(index: number): void {
    const p = conf.presets?.[index];
    if (!p) return;
    const window = presetWindow(p);
    if (!window) return;
    sel = window;
    anchor = "";
    hover = "";
    focus = parseISO(window.start) as number;
    setView(focus);
    render();
    report(mode === "single" || sel.end !== "");
  }

  /** Lights the shortcuts that have something to offer against the bounds and
   * blocked days as they now stand — which a tool result can change. */
  function syncPresets(): void {
    const presets = conf.presets;
    if (!presets?.length) return;
    for (const btn of el.querySelectorAll<HTMLButtonElement>("[data-gomu-cal-preset]")) {
      const p = presets[Number(btn.getAttribute("data-gomu-cal-preset"))];
      btn.disabled = !p || presetWindow(p) === null;
    }
  }

  function clearSelection(): void {
    sel = { start: "", end: "" };
    anchor = "";
    hover = "";
    render();
    report(true);
  }

  // --- Rendering ---

  function value(): DateValue {
    return { ...sel };
  }

  function complete(): boolean {
    if (parseISO(sel.start) === null) return false;
    return mode === "single" || parseISO(sel.end) !== null;
  }

  /** How a day sits in the selection, including the span a half-made range is
   * hovering over — the reader sees what the next click would take. */
  function selectionOf(ms: number): { start: boolean; end: boolean; inside: boolean } {
    const a = parseISO(sel.start);
    let b = parseISO(sel.end);
    if (mode === "range" && b === null && a !== null && hover !== "") {
      const h0 = parseISO(hover);
      if (h0 !== null && spanFree(a, h0)) b = h0;
    }
    if (a === null) return { start: false, end: false, inside: false };
    if (b === null) return { start: ms === a, end: ms === a, inside: ms === a };
    const lo = Math.min(a, b);
    const hi = Math.max(a, b);
    return { start: ms === lo, end: ms === hi, inside: ms >= lo && ms <= hi };
  }

  function monthNode(offset: number): HTMLElement {
    formatters();
    const first = addMonths(view, offset);
    const y = new Date(first).getUTCFullYear();
    const m = new Date(first).getUTCMonth();
    const today = todayISO();

    const table = h("table", { class: "gomu-cal-grid", role: "grid" });
    // The month names itself over its own grid, except where the header
    // already does (one month, one name) — there the caption stays as the
    // grid's accessible name and out of view.
    const cap = h("caption", { class: "gomu-cal-month-name" }, fmtMonth.format(new Date(first)));
    if (months === 1) cap.classList.add("gomu-sr-only");
    table.append(cap);

    const headRow = h("tr");
    if (conf.weekNumbers) {
      headRow.append(h("th", { scope: "col", class: "gomu-cal-weeknum" }, "#"));
    }
    const start = firstDayOfWeek();
    for (let i = 0; i < 7; i++) {
      const day = new Date(REF_SUNDAY + ((start + i) % 7) * DAY_MS);
      headRow.append(
        h(
          "th",
          { scope: "col", class: "gomu-cal-weekday", "aria-label": fmtWeekdayLong.format(day) },
          fmtWeekday.format(day),
        ),
      );
    }
    table.append(h("thead", {}, headRow));

    const body = h("tbody");
    // Start on the week break at or before the 1st, and run whole weeks until
    // the month is out: a month is 4 to 6 rows depending on where it lands.
    let cursor = addDays(first, -((new Date(first).getUTCDay() - start + 7) % 7));
    const monthEnd = addMonths(first, 1) - DAY_MS;
    while (cursor <= monthEnd) {
      const row = h("tr");
      if (conf.weekNumbers) {
        row.append(
          h("th", { scope: "row", class: "gomu-cal-weeknum" }, String(isoWeek(cursor))),
        );
      }
      for (let i = 0; i < 7; i++, cursor = addDays(cursor, 1)) {
        row.append(dayCell(cursor, m, y, today));
      }
      body.append(row);
    }
    table.append(body);
    return h("div", { class: "gomu-cal-month" }, table);
  }

  function dayCell(ms: number, month: number, year: number, today: string): HTMLElement {
    const iso = toISO(ms);
    const d = new Date(ms);
    const outside = d.getUTCMonth() !== month || d.getUTCFullYear() !== year;
    const cell = h("td", { class: "gomu-cal-cell", role: "gridcell" });

    // Days from the neighbouring month keep the week whole but are not offered
    // twice: the month they belong to shows them.
    if (outside) {
      cell.classList.add("gomu-cal-cell--outside");
      // A span, not a button, and deliberately not a .gomu-cal-day: it takes
      // none of a day's states, including the hover that would suggest it could
      // be picked here.
      cell.append(h("span", { class: "gomu-cal-outside" }, String(d.getUTCDate())));
      return cell;
    }

    const disabled = isDisabled(ms);
    const btn = h("button", {
      type: "button",
      class: "gomu-cal-day",
      "data-gomu-cal-day": iso,
      // One tab stop for the whole grid; the arrows move within it.
      tabindex: iso === toISO(focus) ? "0" : "-1",
      "aria-label": fmtFull.format(d),
      // Disabled in the ARIA sense only: a disabled button is unreachable by
      // keyboard, and a reader arrowing through a month must be able to pass
      // over a day that is taken.
      "aria-disabled": disabled ? "true" : null,
      "aria-current": iso === today ? "date" : null,
    }) as HTMLButtonElement;
    if (disabled) btn.classList.add("gomu-cal-day--disabled");
    if (iso === today) btn.classList.add("gomu-cal-day--today");
    btn.append(document.createTextNode(String(d.getUTCDate())));
    cell.append(btn);
    paintDay(btn, ms);
    return cell;
  }

  /** Writes where a day sits in the selection onto its button and cell. Split
   * out of dayCell because a hover over a half-made range restates it for every
   * day on screen, and rebuilding two months of DOM per pointer move to move
   * one shadow is a lot of work for a shadow. */
  function paintDay(btn: HTMLElement, ms: number): void {
    const { start, end, inside } = selectionOf(ms);
    btn.classList.toggle("gomu-cal-day--in-range", inside);
    btn.classList.toggle("gomu-cal-day--start", start);
    btn.classList.toggle("gomu-cal-day--end", end);
    btn.classList.toggle("gomu-cal-day--only", start && end);
    btn.setAttribute("aria-selected", inside ? "true" : "false");
    const cell = btn.parentElement;
    if (!cell) return;
    cell.classList.toggle("gomu-cal-cell--in-range", inside);
    cell.classList.toggle("gomu-cal-cell--start", start);
    cell.classList.toggle("gomu-cal-cell--end", end);
  }

  /** Repaints the selection over the grid already on screen. */
  function paint(): void {
    for (const btn of monthsEl.querySelectorAll<HTMLElement>("[data-gomu-cal-day]")) {
      const ms = parseISO(btn.getAttribute("data-gomu-cal-day"));
      if (ms !== null) paintDay(btn, ms);
    }
  }

  /** The locale's first day of the week, as 0=Sunday. Monday where the host
   * names no locale, or names one the engine has no week data for. */
  function firstDayOfWeek(): number {
    switch (conf.weekStart) {
      case "sunday":
        return 0;
      case "monday":
        return 1;
      case "saturday":
        return 6;
    }
    const locale = getLocale() ?? "";
    if (weekStartCache?.locale === locale) return weekStartCache.day;
    let day = 1;
    if (locale !== "") {
      try {
        const loc = new Intl.Locale(locale) as unknown as {
          getWeekInfo?: () => { firstDay?: number };
          weekInfo?: { firstDay?: number };
        };
        const info = loc.getWeekInfo?.() ?? loc.weekInfo;
        // Intl counts 1=Monday to 7=Sunday.
        if (typeof info?.firstDay === "number") day = info.firstDay % 7;
      } catch {
        // No week data: Monday.
      }
    }
    weekStartCache = { locale, day };
    return day;
  }

  function syncHeader(): void {
    const { lo, hi } = viewBounds();
    nav.prev.disabled = lo !== null && view <= lo;
    nav.next.disabled = hi !== null && view >= hi;
    if (conf.monthDropdowns) {
      const d = new Date(view);
      monthSelect.value = String(d.getUTCMonth());
      yearSelect.value = String(d.getUTCFullYear());
      refreshDropdown(monthSelect);
      refreshDropdown(yearSelect);
      caption.classList.remove("gomu-cal-caption--text");
      return;
    }
    formatters();
    clear(caption);
    caption.classList.add("gomu-cal-caption--text");
    // One month is named in the header, between the two arrows. Several name
    // themselves over their own grid — a header spanning them would say what
    // the columns already say.
    if (months === 1) {
      caption.append(h("span", { class: "gomu-cal-caption-text" }, fmtMonth.format(new Date(view))));
    }
  }

  function render(): void {
    const hadFocus = el.contains(document.activeElement);
    formatters();
    syncHeader();
    syncPresets();
    clear(monthsEl);
    for (let i = 0; i < months; i++) monthsEl.append(monthNode(i));
    if (hadFocus) focusDay();
    // The caption dropdowns become gomukit dropdowns once the grid is in the
    // document: their panels hang off the widget root, which a detached
    // element has none of.
    if (conf.monthDropdowns && !dropdownsEnhanced && el.isConnected) {
      dropdownsEnhanced = true;
      enhanceSelects(caption);
      syncHeader();
    }
  }

  function focusDay(): void {
    const btn = monthsEl.querySelector<HTMLButtonElement>(
      `[data-gomu-cal-day="${toISO(focus)}"]`,
    );
    btn?.focus({ preventScroll: true });
  }

  function focusGrid(): void {
    const at = parseISO(sel.start) ?? parseISO(todayISO());
    if (at !== null) {
      focus = isDisabled(at) ? nearest(at, 1) : at;
      reveal(focus);
    }
    render();
    focusDay();
  }

  /** Moves the keyboard by n days, over blocked days but not past the bounds. */
  function moveFocus(next: number): void {
    const lo = minMs();
    const hi = maxMs();
    if (lo !== null && next < lo) next = lo;
    if (hi !== null && next > hi) next = hi;
    focus = next;
    reveal(next);
    render();
    focusDay();
  }

  // --- Events ---

  el.addEventListener("click", (ev) => {
    const target = ev.target;
    if (!(target instanceof Element)) return;

    const day = target.closest<HTMLElement>("[data-gomu-cal-day]");
    if (day) {
      if (day.getAttribute("aria-disabled") === "true") return;
      pick(day.getAttribute("data-gomu-cal-day") ?? "");
      return;
    }
    const nav0 = target.closest<HTMLElement>("[data-gomu-cal-nav]");
    if (nav0) {
      setView(addMonths(view, nav0.getAttribute("data-gomu-cal-nav") === "prev" ? -1 : 1));
      render();
      return;
    }
    const preset = target.closest<HTMLElement>("[data-gomu-cal-preset]");
    if (preset) {
      usePreset(Number(preset.getAttribute("data-gomu-cal-preset")));
      return;
    }
    if (target.closest("[data-gomu-cal-clear]")) clearSelection();
  });

  el.addEventListener("pointerover", (ev) => {
    if (mode !== "range" || sel.start === "" || sel.end !== "") return;
    const target = ev.target;
    if (!(target instanceof Element)) return;
    const day = target.closest<HTMLElement>("[data-gomu-cal-day]");
    const iso = day?.getAttribute("data-gomu-cal-day") ?? "";
    if (iso === hover) return;
    hover = iso;
    paint();
  });

  el.addEventListener("pointerleave", () => {
    if (hover === "") return;
    hover = "";
    paint();
  });

  el.addEventListener("keydown", (ev) => {
    const target = ev.target;
    if (!(target instanceof Element) || !target.closest("[data-gomu-cal-day]")) return;
    let next: number | null = null;
    switch (ev.key) {
      case "ArrowLeft":
        next = addDays(focus, -1);
        break;
      case "ArrowRight":
        next = addDays(focus, 1);
        break;
      case "ArrowUp":
        next = addDays(focus, -7);
        break;
      case "ArrowDown":
        next = addDays(focus, 7);
        break;
      case "PageUp":
        next = addMonths(focus, ev.shiftKey ? -12 : -1);
        break;
      case "PageDown":
        next = addMonths(focus, ev.shiftKey ? 12 : 1);
        break;
      case "Home":
        next = addDays(focus, -((new Date(focus).getUTCDay() - firstDayOfWeek() + 7) % 7));
        break;
      case "End":
        next = addDays(focus, 6 - ((new Date(focus).getUTCDay() - firstDayOfWeek() + 7) % 7));
        break;
      default:
        return;
    }
    ev.preventDefault();
    moveFocus(next);
  });

  for (const select of [monthSelect, yearSelect]) {
    select.addEventListener("change", () => {
      const m = Number(monthSelect.value);
      const y = Number(yearSelect.value);
      if (!Number.isFinite(m) || !Number.isFinite(y)) return;
      const d = new Date(0);
      d.setUTCFullYear(y, m, 1);
      setView(d.getTime());
      render();
    });
  }

  const view0: CalendarView = {
    el,
    value,
    complete,
    render,
    focusGrid,
    setValue(v) {
      const start = parseISO(v.start ?? sel.start) === null ? "" : ((v.start ?? sel.start) as string);
      const end =
        mode === "range" && parseISO(v.end ?? sel.end) !== null ? ((v.end ?? sel.end) as string) : "";
      sel = { start, end: start === "" ? "" : end };
      anchor = "";
      hover = "";
      const at = parseISO(sel.start);
      if (at !== null) {
        focus = at;
        setView(at);
      }
      render();
    },
    patch(patch) {
      conf = { ...conf, ...patch };
      blocked = new Set(conf.disabled ?? []);
      setView(view);
      render();
    },
  };

  setView(view);
  // The single tab stop starts on today where today is on screen, and on the
  // first of the leading month where it is not: an arrow key has to begin
  // somewhere the reader recognizes.
  const today0 = parseISO(todayISO());
  if (today0 !== null && today0 >= view && today0 <= addMonths(view, months) - DAY_MS) {
    focus = today0;
  }
  return view0;
}

// --- Form fields --------------------------------------------------------

interface DateField {
  /** Re-reads the inputs after a programmatic write. */
  sync(): void;
}

const fields = new WeakMap<HTMLInputElement, DateField>();
let seq = 0;

/**
 * Upgrades a form's date fields into gomukit calendars: a trigger showing the
 * picked date (or window) in the host's locale, and the grid in a popover.
 *
 * The native inputs stay in the DOM as the single source of truth — value,
 * name, required, min/max — exactly as the dropdown keeps its <select>, so
 * mountForm goes on reading and writing them and nothing downstream can tell
 * this layer is here.
 *
 * cfgFor supplies the grid configuration by field name, since it is authored
 * config from the widget's config island rather than anything the markup could
 * carry.
 */
export function enhanceDateFields(
  scope: ParentNode,
  cfgFor: (name: string) => { calendar?: CalendarCfg; endName?: string; required?: boolean } | null,
): void {
  for (const wrap of scope.querySelectorAll<HTMLElement>("[data-gomu-daterange]")) {
    const name = wrap.getAttribute("data-gomu-daterange") ?? "";
    const startEl = wrap.querySelector<HTMLInputElement>(".gomu-daterange-start");
    const endEl = wrap.querySelector<HTMLInputElement>(".gomu-daterange-end");
    if (!startEl || !endEl) continue;
    const field = cfgFor(name);
    enhanceDateInput(startEl, endEl, wrap, field?.calendar ?? { mode: "range" }, !field?.required);
  }
  for (const input of scope.querySelectorAll<HTMLInputElement>("input[type=date]")) {
    if (input.closest("[data-gomu-daterange]")) continue;
    const field = cfgFor(input.name);
    if (!field) continue;
    enhanceDateInput(input, null, null, field.calendar ?? {}, !field.required);
  }
}

/** enhanceDateFields' counterpart to refreshDropdowns: re-reads every enhanced
 * date field under root after a prefill wrote its inputs in code. */
export function refreshDateFields(root: ParentNode): void {
  // A range's two inputs share one field; syncing it twice would only make the
  // grid read the same values again.
  const done = new Set<DateField>();
  for (const input of root.querySelectorAll<HTMLInputElement>("input[type=date]")) {
    const field = fields.get(input);
    if (!field || done.has(field)) continue;
    done.add(field);
    field.sync();
  }
}

function enhanceDateInput(
  startEl: HTMLInputElement,
  endEl: HTMLInputElement | null,
  wrapEl: HTMLElement | null,
  cfg: CalendarCfg,
  clearable: boolean,
): void {
  if (fields.has(startEl)) return;
  const parent = wrapEl ?? startEl.parentNode;
  if (!parent) return;
  const range = endEl !== null;
  const id = `gomu-cal-${++seq}`;

  // A range field is already wrapped (Go renders its two inputs in one block);
  // a single date input gets a wrapper here, the way the dropdown wraps a
  // <select>, so the trigger has something to be positioned inside.
  const wrap = wrapEl ?? h("div", { class: "gomu-dt" });
  if (!wrapEl) {
    (parent as Node & ParentNode).insertBefore(wrap, startEl);
    wrap.append(startEl);
  }
  wrap.classList.add("gomu-dt");
  startEl.classList.add("gomu-dt-native");
  startEl.tabIndex = -1;
  endEl?.classList.add("gomu-dt-native");
  if (endEl) endEl.tabIndex = -1;

  const trigger = h("button", {
    type: "button",
    class: "gomu-input gomu-dt-trigger",
    "aria-haspopup": "dialog",
    "aria-expanded": "false",
    "aria-controls": id,
  }) as HTMLButtonElement;
  // A field label addresses its control by id; the trigger is what clicking
  // that label should reach.
  if (startEl.id) {
    trigger.id = startEl.id;
    startEl.removeAttribute("id");
  }
  const label = startEl.getAttribute("aria-label");
  if (label !== null && !range) trigger.setAttribute("aria-label", label);

  trigger.append(icon("gomu-dt-icon", ...CALENDAR_ICON));
  const valueEl = h("span", { class: "gomu-dt-value" });
  trigger.append(valueEl);
  wrap.append(trigger);

  // Visibility lives on the overlay (see popup.ts); the panel itself is never
  // hidden, or [hidden]'s display:none would blank it inside a shown overlay.
  const panel = h("div", {
    class: "gomu-pop-panel gomu-cal-panel",
    id,
    role: "dialog",
    "aria-modal": "true",
  });
  if (label !== null) panel.setAttribute("aria-label", label);
  // The overlay hangs off the widget root rather than the field: the card
  // chrome clips its overflow, and a panel nested inside it would be cut off at
  // the card's edge (see popup.ts).
  const host = popupHost(startEl);
  const overlay = mountOverlay(host, panel);

  const cal = createCalendar(
    { ...cfg, mode: range ? "range" : "single" },
    {
      clearable,
      onChange: (v) => write(v),
      onDone: () => close(true),
    },
  );
  panel.append(cal.el);

  function placeholder(): string {
    return (
      startEl.getAttribute("placeholder") ?? (range ? "Pick a date range" : "Pick a date")
    );
  }

  /** Writes the calendar's selection back through the inputs, so a change
   * event fires exactly as if the reader had typed it. */
  function write(v: DateValue): void {
    let changed = startEl.value !== v.start;
    startEl.value = v.start;
    if (endEl) {
      changed = changed || endEl.value !== v.end;
      endEl.value = v.end;
    }
    if (!changed) return;
    syncTrigger();
    for (const el of [startEl, endEl]) {
      if (!el) continue;
      el.dispatchEvent(new Event("input", { bubbles: true }));
      el.dispatchEvent(new Event("change", { bubbles: true }));
    }
  }

  function syncTrigger(): void {
    const start = startEl.value;
    const end = endEl?.value ?? "";
    const text = range
      ? start === ""
        ? ""
        : formatDateRange(start, end)
      : formatDate(start);
    valueEl.textContent = text === "" ? placeholder() : text;
    valueEl.classList.toggle("gomu-dt-value--placeholder", text === "");

    trigger.disabled = startEl.disabled;
    const invalid = startEl.getAttribute("aria-invalid");
    if (invalid === null) trigger.removeAttribute("aria-invalid");
    else trigger.setAttribute("aria-invalid", invalid);
    if (startEl.disabled) close();
  }

  /** Re-reads the inputs into the grid and the trigger. */
  function sync(): void {
    cal.setValue({ start: startEl.value, end: endEl?.value ?? "" });
    syncTrigger();
  }

  function isOpen(): boolean {
    return !overlay.hidden;
  }

  function open(): void {
    if (isOpen() || startEl.disabled) return;
    sync();
    overlay.hidden = false;
    trigger.setAttribute("aria-expanded", "true");
    openPopup(popup);
    cal.focusGrid();
  }

  function close(focus = false): void {
    releasePopup(popup);
    if (!isOpen()) return;
    overlay.hidden = true;
    trigger.setAttribute("aria-expanded", "false");
    if (focus) trigger.focus();
  }

  trigger.addEventListener("click", () => {
    if (isOpen()) close(true);
    else open();
  });

  trigger.addEventListener("keydown", (ev) => {
    if (ev.key === "ArrowDown" || ev.key === "Enter" || ev.key === " " || ev.key === "Spacebar") {
      if (isOpen()) return;
      ev.preventDefault();
      open();
    }
  });

  panel.addEventListener("keydown", (ev) => {
    if (ev.key !== "Escape") return;
    ev.preventDefault();
    close(true);
  });

  // Anything that focuses the control itself (a label click, a behavior calling
  // focus()) lands on a hidden input; hand it to the trigger.
  startEl.addEventListener("focus", () => trigger.focus());

  // Behaviors set disabled and aria-invalid on the input directly, and both
  // reflect to attributes — so watching them keeps the trigger in step without
  // every call site having to remember this layer exists.
  if (typeof MutationObserver !== "undefined") {
    new MutationObserver(() => syncTrigger()).observe(startEl, {
      attributes: true,
      attributeFilter: ["disabled", "aria-invalid"],
    });
  }

  const popup: Popup = { anchor: wrap, panel, close };
  const field: DateField = { sync };
  fields.set(startEl, field);
  if (endEl) fields.set(endEl, field);
  sync();
}
