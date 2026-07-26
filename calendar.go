package gadget

import (
	"fmt"
	"time"
)

// isoDate is the layout every date in a calendar configuration is written in.
// Calendar dates are days, not instants: no time, no zone, no offset.
const isoDate = "2006-01-02"

// DateMode selects what a calendar picks: one date, or the span between two.
type DateMode string

const (
	// DateSingle picks one date. The zero value.
	DateSingle DateMode = ""
	// DateRange picks a start and an end date, and treats the days between
	// them as selected.
	DateRange DateMode = "range"
)

var dateModes = map[DateMode]bool{DateSingle: true, DateRange: true}

// WeekStart is the day a calendar's week begins on.
type WeekStart string

const (
	// WeekStartLocale takes the first day from the host's locale — Monday in
	// most of the world, Sunday in the US, Saturday in much of the Middle
	// East. It is the default, and the right answer unless a domain says
	// otherwise (a Monday-to-Sunday shift roster, say).
	WeekStartLocale WeekStart = ""
	WeekStartMonday WeekStart = "monday"
	WeekStartSunday WeekStart = "sunday"
	// WeekStartSaturday is the first day in much of the Middle East.
	WeekStartSaturday WeekStart = "saturday"
)

var weekStarts = map[WeekStart]bool{
	WeekStartLocale:   true,
	WeekStartMonday:   true,
	WeekStartSunday:   true,
	WeekStartSaturday: true,
}

// DateSpan names a window relative to the reader's today. A server cannot
// name those dates at registration time — "the last 7 days" is a different
// week by the time the widget is read, and "today" depends on the reader's
// time zone rather than the server's — so a span travels as the name of a
// rule and the runtime resolves it against the host's clock.
type DateSpan string

const (
	SpanToday      DateSpan = "today"
	SpanYesterday  DateSpan = "yesterday"
	SpanTomorrow   DateSpan = "tomorrow"
	SpanLast7Days  DateSpan = "last-7-days"
	SpanLast30Days DateSpan = "last-30-days"
	SpanLast90Days DateSpan = "last-90-days"
	SpanNext7Days  DateSpan = "next-7-days"
	SpanNext30Days DateSpan = "next-30-days"
	SpanThisWeek   DateSpan = "this-week"
	SpanLastWeek   DateSpan = "last-week"
	SpanThisMonth  DateSpan = "this-month"
	SpanLastMonth  DateSpan = "last-month"
	SpanThisYear   DateSpan = "this-year"
	// SpanYearToDate runs from 1 January to today, where SpanThisYear runs to
	// 31 December.
	SpanYearToDate DateSpan = "year-to-date"
)

var dateSpans = map[DateSpan]bool{
	SpanToday: true, SpanYesterday: true, SpanTomorrow: true,
	SpanLast7Days: true, SpanLast30Days: true, SpanLast90Days: true,
	SpanNext7Days: true, SpanNext30Days: true,
	SpanThisWeek: true, SpanLastWeek: true,
	SpanThisMonth: true, SpanLastMonth: true,
	SpanThisYear: true, SpanYearToDate: true,
}

// DatePreset is a named shortcut listed beside the grid: one press picks a
// whole window instead of two dates. Exactly one of Span and Start/End says
// which window.
type DatePreset struct {
	// Label names the shortcut, e.g. "Last 7 days" (required).
	Label string
	// Span is a window relative to the reader's today, resolved at runtime in
	// the host's time zone.
	Span DateSpan
	// Start and End are a fixed window, as "YYYY-MM-DD" — a quarter that has
	// closed, a campaign that ran. In a single-date calendar only Start is
	// used; End is not allowed.
	Start, End string
}

func (p DatePreset) validate(ctx string, mode DateMode) error {
	if p.Label == "" {
		return fmt.Errorf("%s: Label is required", ctx)
	}
	fixed := p.Start != "" || p.End != ""
	if (p.Span == "") == !fixed {
		return fmt.Errorf("%s: set exactly one of Span or Start/End", ctx)
	}
	if p.Span != "" && !dateSpans[p.Span] {
		return fmt.Errorf("%s: unknown span %q", ctx, p.Span)
	}
	if fixed && p.Start == "" {
		return fmt.Errorf("%s: End needs Start", ctx)
	}
	if p.End != "" && mode != DateRange {
		return fmt.Errorf("%s: End needs a range calendar", ctx)
	}
	start, err := parseISODate(ctx+": Start", p.Start)
	if err != nil {
		return err
	}
	end, err := parseISODate(ctx+": End", p.End)
	if err != nil {
		return err
	}
	if p.Start != "" && p.End != "" && end.Before(start) {
		return fmt.Errorf("%s: End %s is before Start %s", ctx, p.End, p.Start)
	}
	return nil
}

func (p DatePreset) config() map[string]any {
	m := map[string]any{"label": p.Label}
	if p.Span != "" {
		m["span"] = string(p.Span)
	}
	if p.Start != "" {
		m["start"] = p.Start
	}
	if p.End != "" {
		m["end"] = p.End
	}
	return m
}

// Calendar configures the date grid: which days may be picked, how many
// months are on show, and how the reader travels between them. It is a shared
// building block rather than a widget — the DatePicker widget renders one
// inline, and a Form's FDate and FDateRange fields render one in a popover —
// so the same configuration means the same grid in both places.
//
// The zero value is a one-month grid with every day selectable, which is what
// most fields want.
type Calendar struct {
	// Min is the earliest selectable date, as "YYYY-MM-DD". Days before it
	// are shown but cannot be picked, and the grid will not travel past the
	// month holding it.
	Min string
	// Max is the latest selectable date, as "YYYY-MM-DD".
	Max string
	// Disabled lists individual days that cannot be picked ("YYYY-MM-DD") —
	// holidays, sold-out days, days already booked. In a range calendar a
	// span may not straddle one.
	Disabled []string
	// DisableWeekends blocks every Saturday and Sunday. It is about the days
	// themselves, not about where the week starts: WeekStartSaturday still
	// blocks the same two days.
	DisableWeekends bool

	// Months is how many months are shown at once. Defaults to 1 for a single
	// date and 2 for a range, which is what makes a span across a month
	// boundary one gesture rather than two. Maximum 4. They sit side by side
	// where the widget has room and wrap under each other where it has not —
	// every month asked for stays on screen, narrow chat pane or not.
	Months int
	// WeekNumbers adds a leading column of ISO 8601 week numbers.
	WeekNumbers bool
	// MonthDropdowns replaces the month caption with month and year
	// dropdowns, so a date years away is two presses rather than a hundred.
	// Use it for dates of birth and anything else far from today.
	MonthDropdowns bool
	// FromYear and ToYear bound the year dropdown. They default to the years
	// of Min and Max, and — where those are unset — to 100 years before and
	// 10 years after the year the document is rendered in.
	FromYear, ToYear int
	// WeekStart overrides the first day of the week. Defaults to the host
	// locale's own first day.
	WeekStart WeekStart

	// StartOn is the month the grid opens on while nothing is selected, as
	// "YYYY-MM-DD" (the day itself is ignored). Defaults to the month holding
	// the selection, or the reader's current month clamped into Min/Max.
	StartOn string

	// Presets are named shortcuts listed beside the grid.
	Presets []DatePreset
}

// months is how many months the grid shows: what the author asked for, or the
// default for the mode — one date needs one month, a span usually crosses a
// boundary and reads better with the next one already on screen.
func (c *Calendar) months(mode DateMode) int {
	if c != nil && c.Months > 0 {
		return c.Months
	}
	if mode == DateRange {
		return 2
	}
	return 1
}

// validate checks the grid against the mode it will be rendered in. A nil
// Calendar is the zero value and always valid.
func (c *Calendar) validate(ctx string, mode DateMode) error {
	if c == nil {
		return nil
	}
	min, err := parseISODate(ctx+": Min", c.Min)
	if err != nil {
		return err
	}
	max, err := parseISODate(ctx+": Max", c.Max)
	if err != nil {
		return err
	}
	if c.Min != "" && c.Max != "" && max.Before(min) {
		return fmt.Errorf("%s: Max %s is before Min %s", ctx, c.Max, c.Min)
	}
	for n, d := range c.Disabled {
		if d == "" {
			return fmt.Errorf("%s: Disabled[%d] is empty", ctx, n)
		}
		day, err := parseISODate(fmt.Sprintf("%s: Disabled[%d]", ctx, n), d)
		if err != nil {
			return err
		}
		// A blocked day outside the selectable window is already unreachable;
		// listing it is a sign the two disagree about what is on offer.
		if (c.Min != "" && day.Before(min)) || (c.Max != "" && day.After(max)) {
			return fmt.Errorf("%s: Disabled[%d] %s is outside Min/Max", ctx, n, d)
		}
	}
	if c.Months < 0 || c.Months > 4 {
		return fmt.Errorf("%s: Months must be between 1 and 4, got %d", ctx, c.Months)
	}
	if !weekStarts[c.WeekStart] {
		return fmt.Errorf("%s: unknown week start %q", ctx, c.WeekStart)
	}
	if err := validateYear(ctx+": FromYear", c.FromYear); err != nil {
		return err
	}
	if err := validateYear(ctx+": ToYear", c.ToYear); err != nil {
		return err
	}
	if c.FromYear != 0 && c.ToYear != 0 && c.ToYear < c.FromYear {
		return fmt.Errorf("%s: ToYear %d is before FromYear %d", ctx, c.ToYear, c.FromYear)
	}
	if _, err := parseISODate(ctx+": StartOn", c.StartOn); err != nil {
		return err
	}
	for n, p := range c.Presets {
		if err := p.validate(fmt.Sprintf("%s: preset %d (%s)", ctx, n, p.Label), mode); err != nil {
			return err
		}
	}
	return nil
}

// config serializes the grid for the runtime. Mode and month count are always
// emitted — they decide the shape of what is built — and everything else only
// when it is set, so a plain field carries a two-key object.
func (c *Calendar) config(mode DateMode) map[string]any {
	cfg := map[string]any{"months": c.months(mode)}
	if mode == DateRange {
		cfg["mode"] = string(DateRange)
	} else {
		cfg["mode"] = "single"
	}
	if c == nil {
		return cfg
	}
	if c.Min != "" {
		cfg["min"] = c.Min
	}
	if c.Max != "" {
		cfg["max"] = c.Max
	}
	if len(c.Disabled) > 0 {
		cfg["disabled"] = c.Disabled
	}
	if c.DisableWeekends {
		cfg["disableWeekends"] = true
	}
	if c.WeekNumbers {
		cfg["weekNumbers"] = true
	}
	if c.MonthDropdowns {
		cfg["monthDropdowns"] = true
		from, to := c.yearRange()
		cfg["fromYear"] = from
		cfg["toYear"] = to
	}
	if c.WeekStart != WeekStartLocale {
		cfg["weekStart"] = string(c.WeekStart)
	}
	if c.StartOn != "" {
		cfg["startOn"] = c.StartOn
	}
	if len(c.Presets) > 0 {
		presets := make([]map[string]any, len(c.Presets))
		for n, p := range c.Presets {
			presets[n] = p.config()
		}
		cfg["presets"] = presets
	}
	return cfg
}

// yearRange bounds the year dropdown. The window the author bounded wins;
// failing that the years around the render, wide enough back for a date of
// birth and far enough forward for a plan.
//
// This is the one place a calendar reads the server's clock, and it is only a
// listing: which years the dropdown offers, never which days may be picked.
func (c *Calendar) yearRange() (int, int) {
	from, to := c.FromYear, c.ToYear
	if from == 0 {
		if c.Min != "" {
			from = year(c.Min)
		} else {
			from = time.Now().Year() - 100
		}
	}
	if to == 0 {
		if c.Max != "" {
			to = year(c.Max)
		} else {
			to = time.Now().Year() + 10
		}
	}
	if to < from {
		to = from
	}
	return from, to
}

// parseISODate accepts a "YYYY-MM-DD" date or the empty string, which every
// date in a calendar configuration treats as "unset".
func parseISODate(ctx, s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(isoDate, s)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s: %q is not a YYYY-MM-DD date", ctx, s)
	}
	return t, nil
}

// year is the year of a date already known to parse.
func year(s string) int {
	t, err := time.Parse(isoDate, s)
	if err != nil {
		return 0
	}
	return t.Year()
}

func validateYear(ctx string, y int) error {
	if y != 0 && (y < 1 || y > 9999) {
		return fmt.Errorf("%s: %d is not a four-digit year", ctx, y)
	}
	return nil
}
