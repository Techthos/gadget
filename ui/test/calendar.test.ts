import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	createCalendar,
	enhanceDateFields,
	formatDateRange,
	parseISO,
	refreshDateFields,
	resolvePreset,
	todayISO,
	toISO,
	type CalendarCfg,
	type DateValue,
} from "../src/calendar";
import { setLocale } from "../src/format";

// A fixed today, so "this month" and the today marker are assertable. It is a
// Thursday in the middle of a month that starts on a Saturday.
const TODAY = "2026-07-16";

function host(): HTMLElement {
	document.body.innerHTML = "";
	const root = document.createElement("div");
	root.className = "gomu-root";
	root.setAttribute("data-gomu-widget", "datepicker");
	document.body.append(root);
	return root;
}

function build(cfg: CalendarCfg, opts: Parameters<typeof createCalendar>[1] = {}) {
	const root = host();
	const cal = createCalendar(cfg, opts);
	root.append(cal.el);
	cal.render();
	return { root, cal };
}

function days(el: HTMLElement): HTMLButtonElement[] {
	return [...el.querySelectorAll<HTMLButtonElement>("[data-gomu-cal-day]")];
}

function day(el: HTMLElement, iso: string): HTMLButtonElement {
	const found = el.querySelector<HTMLButtonElement>(`[data-gomu-cal-day="${iso}"]`);
	if (!found) throw new Error(`no cell for ${iso}`);
	return found;
}

/** A press, as the popup layer hears one (see ui/src/popup.ts). */
function press(el: HTMLElement | Element): void {
	el.dispatchEvent(new MouseEvent("pointerdown", { bubbles: true }));
}

function key(el: HTMLElement, k: string, shift = false): void {
	el.dispatchEvent(
		new KeyboardEvent("keydown", { key: k, shiftKey: shift, bubbles: true, cancelable: true }),
	);
}

beforeEach(() => {
	vi.useFakeTimers();
	vi.setSystemTime(new Date(`${TODAY}T09:00:00Z`));
	setLocale("en-GB", "UTC");
	document.body.innerHTML = "";
});

afterEach(() => {
	vi.useRealTimers();
	setLocale(undefined, undefined);
});

describe("date arithmetic", () => {
	it("parses calendar dates and refuses everything else", () => {
		expect(toISO(parseISO("2026-07-16") as number)).toBe("2026-07-16");
		// Days, not instants: no zone can move one.
		expect(parseISO("2026-07-16")).toBe(Date.UTC(2026, 6, 16));
		for (const bad of ["2026-02-30", "2026-13-01", "26-07-16", "2026-7-16", "", "today", 42, null]) {
			expect(parseISO(bad)).toBeNull();
		}
	});

	it("reads today in the host's time zone, not the system's", () => {
		// 23:30 UTC on the 16th is already the 17th in Auckland.
		vi.setSystemTime(new Date("2026-07-16T23:30:00Z"));
		setLocale("en-GB", "Pacific/Auckland");
		expect(todayISO()).toBe("2026-07-17");
		setLocale("en-GB", "America/Los_Angeles");
		expect(todayISO()).toBe("2026-07-16");
	});
});

describe("presets", () => {
	const cases: Array<[string, DateValue]> = [
		["today", { start: TODAY, end: TODAY }],
		["yesterday", { start: "2026-07-15", end: "2026-07-15" }],
		["last-7-days", { start: "2026-07-10", end: TODAY }],
		["next-7-days", { start: TODAY, end: "2026-07-22" }],
		// 16 July 2026 is a Thursday; its Monday is the 13th.
		["this-week", { start: "2026-07-13", end: "2026-07-19" }],
		["last-week", { start: "2026-07-06", end: "2026-07-12" }],
		["this-month", { start: "2026-07-01", end: "2026-07-31" }],
		["last-month", { start: "2026-06-01", end: "2026-06-30" }],
		["this-year", { start: "2026-01-01", end: "2026-12-31" }],
		["year-to-date", { start: "2026-01-01", end: TODAY }],
	];
	for (const [span, want] of cases) {
		it(`resolves ${span} against the reader's today`, () => {
			expect(resolvePreset({ label: span, span }, "range")).toEqual(want);
		});
	}

	it("takes only the opening day in a single-date calendar", () => {
		expect(resolvePreset({ label: "Last 7 days", span: "last-7-days" }, "single")).toEqual({
			start: "2026-07-10",
			end: "",
		});
	});

	it("passes a fixed window through and rejects nonsense", () => {
		expect(
			resolvePreset({ label: "Fair", start: "2026-09-07", end: "2026-09-11" }, "range"),
		).toEqual({ start: "2026-09-07", end: "2026-09-11" });
		expect(resolvePreset({ label: "Bad", span: "last-fortnight" }, "range")).toBeNull();
		expect(resolvePreset({ label: "Bad", start: "nope" }, "range")).toBeNull();
	});
});

describe("calendar grid", () => {
	it("builds whole weeks from Monday, marking today and the neighbouring days", () => {
		const { cal } = build({ startOn: TODAY });
		const grid = cal.el.querySelector<HTMLElement>(".gomu-cal-grid")!;
		const heads = [...grid.querySelectorAll(".gomu-cal-weekday")].map((th) =>
			th.getAttribute("aria-label"),
		);
		expect(heads).toEqual([
			"Monday",
			"Tuesday",
			"Wednesday",
			"Thursday",
			"Friday",
			"Saturday",
			"Sunday",
		]);
		// July 2026 starts on a Wednesday and has 31 days: five whole weeks.
		expect(grid.querySelectorAll("tbody tr")).toHaveLength(5);
		expect(days(cal.el)).toHaveLength(31);
		expect(cal.el.querySelectorAll(".gomu-cal-outside")).toHaveLength(4);
		expect(day(cal.el, TODAY).getAttribute("aria-current")).toBe("date");
	});

	it("honours an explicit week start", () => {
		const { cal } = build({ startOn: TODAY, weekStart: "sunday" });
		const first = cal.el.querySelector(".gomu-cal-weekday")!;
		expect(first.getAttribute("aria-label")).toBe("Sunday");
	});

	it("shows two months for a range and travels together", () => {
		const { cal } = build({ mode: "range", months: 2, startOn: "2026-07-01" });
		expect(cal.el.querySelectorAll(".gomu-cal-month")).toHaveLength(2);
		expect(day(cal.el, "2026-08-15")).toBeTruthy();
		cal.el.querySelector<HTMLElement>('[data-gomu-cal-nav="next"]')!.click();
		expect(day(cal.el, "2026-09-15")).toBeTruthy();
		expect(cal.el.querySelector('[data-gomu-cal-day="2026-07-15"]')).toBeNull();
	});

	it("adds the ISO week column", () => {
		const { cal } = build({ startOn: "2026-01-15", weekNumbers: true });
		const weeks = [...cal.el.querySelectorAll("tbody .gomu-cal-weeknum")].map(
			(th) => th.textContent,
		);
		// 1 January 2026 is a Thursday, so its week is ISO week 1.
		expect(weeks[0]).toBe("1");
		expect(weeks[1]).toBe("2");
	});

	it("picks a single date and reports it once", () => {
		const changes: Array<[DateValue, boolean]> = [];
		let done = 0;
		const { cal } = build(
			{ startOn: TODAY },
			{ onChange: (v, complete) => changes.push([v, complete]), onDone: () => done++ },
		);
		day(cal.el, "2026-07-20").click();
		expect(cal.value()).toEqual({ start: "2026-07-20", end: "" });
		expect(cal.complete()).toBe(true);
		expect(changes).toEqual([[{ start: "2026-07-20", end: "" }, true]]);
		expect(done).toBe(1);
		expect(day(cal.el, "2026-07-20").classList.contains("gomu-cal-day--start")).toBe(true);
	});

	it("builds a range from two clicks, in either order", () => {
		const changes: boolean[] = [];
		const { cal } = build(
			{ mode: "range", months: 1, startOn: TODAY },
			{ onChange: (_v, complete) => changes.push(complete) },
		);
		day(cal.el, "2026-07-20").click();
		expect(cal.complete()).toBe(false);
		day(cal.el, "2026-07-24").click();
		expect(cal.value()).toEqual({ start: "2026-07-20", end: "2026-07-24" });
		expect(changes).toEqual([false, true]);
		expect(day(cal.el, "2026-07-22").classList.contains("gomu-cal-day--in-range")).toBe(true);

		// A click before the open end still makes a forwards range.
		day(cal.el, "2026-07-15").click();
		day(cal.el, "2026-07-10").click();
		expect(cal.value()).toEqual({ start: "2026-07-10", end: "2026-07-15" });
	});

	it("refuses days outside the window and days that are taken", () => {
		const { cal } = build({
			startOn: TODAY,
			min: "2026-07-10",
			max: "2026-07-25",
			disabled: ["2026-07-17"],
			disableWeekends: true,
		});
		for (const iso of ["2026-07-09", "2026-07-26", "2026-07-17", "2026-07-18"]) {
			const cell = day(cal.el, iso);
			expect(cell.getAttribute("aria-disabled")).toBe("true");
			cell.click();
		}
		expect(cal.value().start).toBe("");
		day(cal.el, "2026-07-16").click();
		expect(cal.value().start).toBe("2026-07-16");
	});

	it("will not let a range straddle a day that is taken", () => {
		const { cal } = build({
			mode: "range",
			months: 1,
			startOn: TODAY,
			disabled: ["2026-07-22"],
		});
		day(cal.el, "2026-07-20").click();
		day(cal.el, "2026-07-24").click();
		// The second click starts a new range instead of spanning the blocked day.
		expect(cal.value()).toEqual({ start: "2026-07-24", end: "" });
		day(cal.el, "2026-07-26").click();
		expect(cal.value()).toEqual({ start: "2026-07-24", end: "2026-07-26" });
	});

	it("stops the month arrows at the window's edge", () => {
		const { cal } = build({ startOn: TODAY, min: "2026-07-01", max: "2026-08-31" });
		const prev = cal.el.querySelector<HTMLButtonElement>('[data-gomu-cal-nav="prev"]')!;
		const next = cal.el.querySelector<HTMLButtonElement>('[data-gomu-cal-nav="next"]')!;
		expect(prev.disabled).toBe(true);
		expect(next.disabled).toBe(false);
		next.click();
		expect(cal.el.querySelector<HTMLButtonElement>('[data-gomu-cal-nav="next"]')!.disabled).toBe(
			true,
		);
	});

	it("applies a preset window", () => {
		const { cal } = build({
			mode: "range",
			months: 1,
			presets: [{ label: "This month", span: "this-month" }],
		});
		cal.el.querySelector<HTMLElement>("[data-gomu-cal-preset]")!.click();
		expect(cal.value()).toEqual({ start: "2026-07-01", end: "2026-07-31" });
	});

	it("trims a preset window to the bounds and switches off the ones left with nothing", () => {
		const { cal } = build({
			mode: "range",
			months: 1,
			min: "2026-07-10",
			max: "2026-07-20",
			presets: [
				// Trimmed at both ends to the window that may be picked.
				{ label: "This month", span: "this-month" },
				// Wholly before Min: nothing of it survives.
				{ label: "Last month", span: "last-month" },
			],
		});
		const shortcuts = [
			...cal.el.querySelectorAll<HTMLButtonElement>("[data-gomu-cal-preset]"),
		];
		expect(shortcuts.map((b) => b.disabled)).toEqual([false, true]);

		shortcuts[0]!.click();
		expect(cal.value()).toEqual({ start: "2026-07-10", end: "2026-07-20" });

		// A dead one leaves the selection alone.
		shortcuts[1]!.click();
		expect(cal.value()).toEqual({ start: "2026-07-10", end: "2026-07-20" });
	});

	it("switches off a preset whose window straddles a blocked day", () => {
		const { cal } = build({
			mode: "range",
			months: 1,
			disabled: ["2026-07-14"],
			presets: [
				{ label: "Fair", start: "2026-07-13", end: "2026-07-16" },
				{ label: "Show", start: "2026-07-15", end: "2026-07-16" },
			],
		});
		const shortcuts = [
			...cal.el.querySelectorAll<HTMLButtonElement>("[data-gomu-cal-preset]"),
		];
		expect(shortcuts.map((b) => b.disabled)).toEqual([true, false]);
	});

	it("re-reads the shortcuts when a tool result moves the bounds", () => {
		const { cal } = build({
			mode: "range",
			months: 1,
			min: "2026-08-01",
			presets: [{ label: "This month", span: "this-month" }],
		});
		const shortcut = cal.el.querySelector<HTMLButtonElement>("[data-gomu-cal-preset]")!;
		expect(shortcut.disabled).toBe(true);
		cal.patch({ min: "2026-07-01" });
		expect(shortcut.disabled).toBe(false);
	});

	it("switches off a single-date preset rather than moving the day it names", () => {
		const { cal } = build({
			months: 1,
			min: "2026-07-13",
			presets: [
				{ label: "Last 7 days", span: "last-7-days" },
				{ label: "Today", span: "today" },
			],
		});
		const shortcuts = [
			...cal.el.querySelectorAll<HTMLButtonElement>("[data-gomu-cal-preset]"),
		];
		expect(shortcuts.map((b) => b.disabled)).toEqual([true, false]);
		shortcuts[1]!.click();
		expect(cal.value()).toEqual({ start: TODAY, end: "" });
	});

	it("moves the keyboard over blocked days and travels by month", () => {
		const { cal } = build({ startOn: TODAY, disabled: ["2026-07-17"] });
		const start = day(cal.el, TODAY);
		expect(start.tabIndex).toBe(0);
		key(start, "ArrowRight");
		// The blocked day still takes focus: passing over it is how you get past it.
		expect(document.activeElement).toBe(day(cal.el, "2026-07-17"));
		key(document.activeElement as HTMLElement, "ArrowDown");
		expect(document.activeElement).toBe(day(cal.el, "2026-07-24"));
		key(document.activeElement as HTMLElement, "PageDown");
		expect(day(cal.el, "2026-08-24")).toBeTruthy();
		key(document.activeElement as HTMLElement, "PageUp", true);
		expect(day(cal.el, "2025-08-24")).toBeTruthy();
	});

	it("takes bounds and blocked days from a runtime patch", () => {
		const { cal } = build({ startOn: TODAY });
		expect(day(cal.el, "2026-07-20").getAttribute("aria-disabled")).toBeNull();
		cal.patch({ disabled: ["2026-07-20"], max: "2026-07-25" });
		expect(day(cal.el, "2026-07-20").getAttribute("aria-disabled")).toBe("true");
		expect(day(cal.el, "2026-07-26").getAttribute("aria-disabled")).toBe("true");
	});

	it("offers month and year dropdowns when asked", () => {
		const { cal } = build({
			startOn: "2026-07-16",
			monthDropdowns: true,
			fromYear: 2020,
			toYear: 2030,
		});
		const selects = cal.el.querySelectorAll("select");
		expect(selects).toHaveLength(2);
		const [month, year] = [...selects] as HTMLSelectElement[];
		expect(month!.options).toHaveLength(12);
		expect(year!.options).toHaveLength(11);
		expect(month!.value).toBe("6");
		expect(year!.value).toBe("2026");
		// The dropdowns are the gomukit dropdown, and the calendar is their parent
		// popup rather than their rival (see popup.ts).
		expect(cal.el.querySelectorAll(".gomu-dd-trigger")).toHaveLength(2);

		year!.value = "2028";
		year!.dispatchEvent(new Event("change", { bubbles: true }));
		expect(day(cal.el, "2028-07-16")).toBeTruthy();
	});
});

// --- The popover over a form's date fields ---

const DATE_FIELD = `
  <div class="gomu-field gomu-field--date">
    <label for="gomu-f-when">When</label>
    <input id="gomu-f-when" name="when" class="gomu-input" type="date" aria-label="When">
  </div>`;

const RANGE_FIELD = `
  <div class="gomu-field gomu-field--daterange">
    <label for="gomu-f-stay">Stay</label>
    <div class="gomu-daterange" data-gomu-daterange="stay">
      <input type="date" name="stay" class="gomu-input gomu-daterange-start" aria-label="Stay start date" id="gomu-f-stay">
      <input type="date" name="stay_until" class="gomu-input gomu-daterange-end" aria-label="Stay end date">
    </div>
  </div>`;

function field(
	markup: string,
	cfg: Record<string, { calendar?: CalendarCfg; endName?: string; required?: boolean }>,
) {
	const root = host();
	const form = document.createElement("form");
	form.innerHTML = markup;
	root.append(form);
	enhanceDateFields(form, (name) => cfg[name] ?? null);
	return {
		root,
		form,
		trigger: root.querySelector<HTMLButtonElement>(".gomu-dt-trigger")!,
		panel: root.querySelector<HTMLElement>(".gomu-cal-panel")!,
		// Visibility lives on the overlay that wraps the panel (see src/popup.ts).
		overlay: root.querySelector<HTMLElement>(".gomu-cal-panel")?.parentElement as HTMLElement,
		value: root.querySelector<HTMLElement>(".gomu-dt-value")!,
		start: form.querySelector<HTMLInputElement>('input[name="stay"], input[name="when"]')!,
		end: form.querySelector<HTMLInputElement>('input[name="stay_until"]'),
	};
}

describe("date fields", () => {
	it("upgrades a date input without removing it", () => {
		const f = field(DATE_FIELD, { when: { calendar: { startOn: TODAY } } });
		expect(f.start.isConnected).toBe(true);
		expect(f.start.type).toBe("date");
		expect(f.start.tabIndex).toBe(-1);
		expect(f.start.classList.contains("gomu-dt-native")).toBe(true);
		// The field label addresses the control by id, so the trigger takes it.
		expect(f.trigger.id).toBe("gomu-f-when");
		expect(f.start.id).toBe("");
		expect(f.trigger.getAttribute("aria-expanded")).toBe("false");
		expect(f.value.textContent).toBe("Pick a date");
		// The overlay escapes the card chrome that would clip it, and starts closed.
		expect(f.overlay.parentElement).toBe(f.root);
		expect(f.overlay.hidden).toBe(true);
	});

	it("writes the pick back through the native input", () => {
		const f = field(DATE_FIELD, { when: { calendar: { startOn: TODAY } } });
		const changes: string[] = [];
		f.start.addEventListener("change", () => changes.push(f.start.value));

		f.trigger.click();
		expect(f.overlay.hidden).toBe(false);
		// The panel must never carry hidden, or [hidden]'s display:none would
		// blank it inside a shown overlay.
		expect(f.panel.hasAttribute("hidden")).toBe(false);
		day(f.panel, "2026-07-20").click();

		expect(f.start.value).toBe("2026-07-20");
		expect(changes).toEqual(["2026-07-20"]);
		expect(f.value.textContent).toBe("20 Jul 2026");
		// A finished pick closes the popover.
		expect(f.overlay.hidden).toBe(true);
	});

	it("keeps the popover open until a range is finished", () => {
		const f = field(RANGE_FIELD, {
			stay: { calendar: { mode: "range", months: 1, startOn: TODAY }, endName: "stay_until" },
		});
		expect(f.value.textContent).toBe("Pick a date range");
		f.trigger.click();
		day(f.panel, "2026-07-20").click();
		expect(f.overlay.hidden).toBe(false);
		expect(f.start.value).toBe("2026-07-20");
		expect(f.end!.value).toBe("");

		day(f.panel, "2026-07-24").click();
		expect(f.overlay.hidden).toBe(true);
		expect(f.end!.value).toBe("2026-07-24");
		expect(f.value.textContent).toBe(formatDateRange("2026-07-20", "2026-07-24"));
	});

	it("reads a prefill written in code", () => {
		const f = field(RANGE_FIELD, {
			stay: { calendar: { mode: "range", months: 1 }, endName: "stay_until" },
		});
		f.start.value = "2026-09-07";
		f.end!.value = "2026-09-11";
		refreshDateFields(f.form);
		expect(f.value.textContent).toBe(formatDateRange("2026-09-07", "2026-09-11"));
		f.trigger.click();
		expect(day(f.panel, "2026-09-07").classList.contains("gomu-cal-day--start")).toBe(true);
	});

	it("offers a clear button only where the field is optional", () => {
		const optional = field(DATE_FIELD, { when: { calendar: { startOn: TODAY } } });
		expect(optional.panel.querySelector("[data-gomu-cal-clear]")).toBeTruthy();
		optional.start.value = "2026-07-20";
		refreshDateFields(optional.form);
		optional.panel.querySelector<HTMLElement>("[data-gomu-cal-clear]")!.click();
		expect(optional.start.value).toBe("");

		const required = field(DATE_FIELD, { when: { calendar: {}, required: true } });
		expect(required.panel.querySelector("[data-gomu-cal-clear]")).toBeNull();
	});

	it("follows the input's disabled and invalid state", () => {
		const f = field(DATE_FIELD, { when: {} });
		f.start.disabled = true;
		f.start.setAttribute("aria-invalid", "true");
		// Attribute changes are observed asynchronously.
		return Promise.resolve().then(() => {
			expect(f.trigger.disabled).toBe(true);
			expect(f.trigger.getAttribute("aria-invalid")).toBe("true");
		});
	});

	it("closes on Escape and re-focuses the trigger", () => {
		const f = field(DATE_FIELD, { when: { calendar: { startOn: TODAY } } });
		f.trigger.click();
		expect(f.overlay.hidden).toBe(false);
		key(f.panel, "Escape");
		expect(f.overlay.hidden).toBe(true);
		expect(document.activeElement).toBe(f.trigger);
	});

	it("keeps the popover open while its own month dropdown is used", () => {
		const f = field(DATE_FIELD, {
			when: { calendar: { startOn: TODAY, monthDropdowns: true, fromYear: 2020, toYear: 2030 } },
		});
		f.trigger.click();
		expect(f.overlay.hidden).toBe(false);

		// A popup opened from inside another must not close its own parent.
		const ddTrigger = f.panel.querySelector<HTMLButtonElement>(".gomu-dd-trigger")!;
		ddTrigger.click();
		const ddOverlay = f.root.querySelector<HTMLElement>(".gomu-dd-panel")!.parentElement as HTMLElement;
		expect(ddOverlay.hidden).toBe(false);
		expect(f.overlay.hidden).toBe(false);

		// A press inside the calendar is outside the dropdown: it peels one layer.
		press(f.panel);
		expect(ddOverlay.hidden).toBe(true);
		expect(f.overlay.hidden).toBe(false);

		press(document.body);
		expect(f.overlay.hidden).toBe(true);
	});

	it("leaves a date input the form does not declare alone", () => {
		const f = field(DATE_FIELD, {});
		expect(f.trigger).toBeNull();
		expect(f.start.classList.contains("gomu-dt-native")).toBe(false);
	});
});
