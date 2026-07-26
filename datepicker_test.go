package gomukit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"

	"github.com/techthos/gomukit/theme"
)

// canonicalDatePicker exercises every slot: a range, a context record, a bound
// and blocked grid with week numbers and presets, and both outcomes.
func canonicalDatePicker() *DatePicker {
	return &DatePicker{
		URI:    "ui://demo/window",
		Title:  "Delivery window",
		Prompt: "Which nights should we hold the room?",
		Body:   "Checkout is at 11:00 on the last day.",
		Mode:   DateRange,
		Calendar: &Calendar{
			Min:         "2026-08-01",
			Max:         "2026-10-31",
			Disabled:    []string{"2026-08-14", "2026-08-15"},
			WeekNumbers: true,
			Presets: []DatePreset{
				{Label: "This week", Span: SpanThisWeek},
				{Label: "Next 7 days", Span: SpanNext7Days},
				{Label: "Trade fair", Start: "2026-09-07", End: "2026-09-11"},
			},
		},
		Default:    "2026-08-20",
		DefaultEnd: "2026-08-23",
		Details: Descriptions{Items: []DescriptionItem{
			{Label: "Booking", Key: "reference"},
			{Label: "Room", Text: "Suite 4"},
		}},
		Submit: DateSubmit{
			Tool:           "hold_room",
			Label:          "Hold it",
			ValueArg:       "from",
			EndArg:         "until",
			Args:           map[string]ArgSource{"booking": FromRow("id"), "notify": Static(true)},
			SuccessMessage: "Held.",
		},
		Cancel: &RejectSpec{Label: "Decide later", Tool: "postpone", Args: map[string]ArgSource{"booking": FromRow("id")}},
		InitialData: map[string]any{
			"rows": []map[string]any{{"id": 4471, "reference": "BKG-4471"}},
		},
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func TestDatePickerGolden(t *testing.T) {
	doc, err := canonicalDatePicker().Document()
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "datepicker.html")
	if *update {
		if err := os.WriteFile(golden, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if doc != string(want) {
		t.Error("document does not match golden file; run `go test -run TestDatePickerGolden -update ./...` and review the diff")
	}
	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, want := range []string{
		`data-gomu-widget="datepicker"`,
		`gomu-datepicker--range`,
		`Which nights should we hold the room?`,
		`data-gomu-descriptions`,
		`data-gomu-calendar`,
		`data-gomu-summary`,
		`data-gomu-cancel`,
		`data-gomu-submit`,
		`data-gomu-outcome`,
		`>Hold it</button>`,
		`>Decide later</button>`,
		`--gomu-color-primary:#7c3aed`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q", want)
		}
	}
	// Nothing can be submitted before the runtime knows what is picked.
	if !strings.Contains(doc, `data-gomu-submit="" disabled`) {
		t.Error("submit button must render disabled")
	}
}

// datePickerShell renders the widget markup alone. Negative assertions cannot
// run against the whole document: the inlined stylesheet and runtime mention
// every class and data attribute the library knows.
func datePickerShell(t *testing.T, d *DatePicker) string {
	t.Helper()
	if err := d.Validate(); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := d.shell().Render(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestDatePickerMinimalShell(t *testing.T) {
	d := &DatePicker{
		URI:    "ui://demo/when",
		Prompt: "When?",
		Submit: DateSubmit{Tool: "schedule"},
	}
	shell := datePickerShell(t, d)
	doc, err := d.Document()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, unwanted := range []string{
		"data-gomu-descriptions", "data-gomu-cancel", "gomu-toolbar", "gomu-datepicker-lede",
	} {
		if strings.Contains(shell, unwanted) {
			t.Errorf("minimal shell should not contain %q:\n%s", unwanted, shell)
		}
	}
	for _, want := range []string{
		`gomu-datepicker--single`,
		`class="gomu-btn gomu-btn--primary" data-gomu-submit`,
		`>Continue</button>`,
	} {
		if !strings.Contains(shell, want) {
			t.Errorf("minimal shell missing %q:\n%s", want, shell)
		}
	}
	// The grid is runtime work: no day, month or weekday name is server-rendered.
	if strings.Contains(shell, "gomu-cal-day") {
		t.Errorf("the grid must not be server-rendered:\n%s", shell)
	}
}

func TestDatePickerConfigIsland(t *testing.T) {
	b, err := json.Marshal(canonicalDatePicker().config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{
		`"widget":"datepicker"`,
		`"valueKey":"value"`,
		`"rowsKey":"rows"`,
		`"rowId":"id"`,
		`"default":"2026-08-20"`,
		`"defaultEnd":"2026-08-23"`,
		`"mode":"range"`,
		`"months":2`,
		`"min":"2026-08-01"`,
		`"max":"2026-10-31"`,
		`"disabled":["2026-08-14","2026-08-15"]`,
		`"weekNumbers":true`,
		`{"label":"This week","span":"this-week"}`,
		`{"end":"2026-09-11","label":"Trade fair","start":"2026-09-07"}`,
		`"submit":{"args":{"booking":{"row":"id"},"notify":{"static":true}},"endArg":"until","successMessage":"Held.","tool":"hold_room","valueArg":"from"}`,
		`"cancel":{"args":{"booking":{"row":"id"}},"message":"Cancelled.","tool":"postpone"}`,
		`{"key":"reference","label":"Booking","type":"text"}`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
	// Nothing was asked for, so nothing is claimed.
	for _, unwanted := range []string{`"monthDropdowns"`, `"disableWeekends"`, `"weekStart"`, `"loadTool"`} {
		if strings.Contains(cfg, unwanted) {
			t.Errorf("config island should not carry %s: %s", unwanted, cfg)
		}
	}
}

func TestDatePickerConfigDefaults(t *testing.T) {
	d := &DatePicker{
		URI:    "ui://demo/when",
		Prompt: "When?",
		Submit: DateSubmit{Tool: "schedule"},
		Cancel: &RejectSpec{},
	}
	b, _ := json.Marshal(d.config())
	cfg := string(b)
	for _, want := range []string{
		// A lone date is just the date, and one month is enough to pick it in.
		`"submit":{"tool":"schedule","valueArg":"date"}`,
		`"calendar":{"mode":"single","months":1}`,
		`"cancel":{"message":"Cancelled."}`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
	for _, unwanted := range []string{"details", "default", "endArg"} {
		if strings.Contains(cfg, unwanted) {
			t.Errorf("config island should not carry %q: %s", unwanted, cfg)
		}
	}
}

// A range names its two ends start and end unless the author says otherwise,
// and asks for two months so a span across a boundary is one gesture.
func TestDatePickerRangeDefaults(t *testing.T) {
	d := &DatePicker{
		URI:    "ui://demo/when",
		Prompt: "When?",
		Mode:   DateRange,
		Submit: DateSubmit{Tool: "book"},
	}
	b, _ := json.Marshal(d.config())
	cfg := string(b)
	for _, want := range []string{
		`"submit":{"endArg":"end","tool":"book","valueArg":"start"}`,
		`"mode":"range","months":2`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
}

// The year dropdown is bounded by the calendar's own window where it has one,
// so a date of birth does not offer years nobody may pick.
func TestDatePickerYearRangeFromBounds(t *testing.T) {
	d := &DatePicker{
		URI:      "ui://demo/dob",
		Prompt:   "Date of birth?",
		Calendar: &Calendar{Min: "1920-01-01", Max: "2010-12-31", MonthDropdowns: true},
		Submit:   DateSubmit{Tool: "save"},
	}
	b, _ := json.Marshal(d.config())
	cfg := string(b)
	for _, want := range []string{`"monthDropdowns":true`, `"fromYear":1920`, `"toYear":2010`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
}

func TestDatePickerToolMetaAndDescriptor(t *testing.T) {
	d := canonicalDatePicker()
	meta, _ := json.Marshal(d.ToolMeta())
	if string(meta) != `{"ui":{"resourceUri":"ui://demo/window"}}` {
		t.Errorf("ToolMeta = %s", meta)
	}
	desc := d.Descriptor()
	if desc.URI != "ui://demo/window" || desc.Name != "demo-window" || desc.MIMEType != "text/html;profile=mcp-app" {
		t.Errorf("Descriptor = %+v", desc)
	}
}

func TestDatePickerValidate(t *testing.T) {
	cases := map[string]func(*DatePicker){
		"bad URI scheme": func(d *DatePicker) { d.URI = "https://x" },
		"no prompt":      func(d *DatePicker) { d.Prompt = "" },
		"no submit tool": func(d *DatePicker) { d.Submit.Tool = "" },
		"unknown mode":   func(d *DatePicker) { d.Mode = "week" },
		"selection arg in submit": func(d *DatePicker) {
			d.Submit.Args = map[string]ArgSource{"ids": FromSelection("id")}
		},
		"empty arg source": func(d *DatePicker) { d.Submit.Args = map[string]ArgSource{"booking": {}} },
		"value arg collides with a static arg": func(d *DatePicker) {
			d.Submit.Args = map[string]ArgSource{"from": Static("2026-08-01")}
		},
		"end arg collides with a static arg": func(d *DatePicker) {
			d.Submit.Args = map[string]ArgSource{"until": Static("2026-08-02")}
		},
		"both ends in one argument": func(d *DatePicker) { d.Submit.EndArg = "from" },
		"end arg without a range": func(d *DatePicker) {
			d.Mode, d.DefaultEnd = DateSingle, ""
			d.Calendar.Presets[2].End = ""
		},
		"default end without a range": func(d *DatePicker) {
			d.Mode, d.Submit.EndArg = DateSingle, ""
			d.Calendar.Presets[2].End = ""
		},
		"cancel args without tool": func(d *DatePicker) {
			d.Cancel = &RejectSpec{Args: map[string]ArgSource{"booking": FromRow("id")}}
		},
		"malformed default":         func(d *DatePicker) { d.Default = "20/08/2026" },
		"impossible default":        func(d *DatePicker) { d.Default, d.DefaultEnd = "2026-02-30", "" },
		"backwards default range":   func(d *DatePicker) { d.Default, d.DefaultEnd = "2026-08-23", "2026-08-20" },
		"default end without start": func(d *DatePicker) { d.Default = "" },
		"default before min":        func(d *DatePicker) { d.Default = "2026-07-30" },
		"default after max":         func(d *DatePicker) { d.Default, d.DefaultEnd = "2026-11-01", "2026-11-02" },
		"default on a blocked day":  func(d *DatePicker) { d.Default, d.DefaultEnd = "2026-08-14", "" },
		"malformed min":             func(d *DatePicker) { d.Calendar.Min = "2026-8-1" },
		"max before min":            func(d *DatePicker) { d.Calendar.Max = "2026-07-01" },
		"blocked day out of bounds": func(d *DatePicker) { d.Calendar.Disabled = []string{"2027-01-01"} },
		"empty blocked day":         func(d *DatePicker) { d.Calendar.Disabled = []string{""} },
		"too many months":           func(d *DatePicker) { d.Calendar.Months = 5 },
		"unknown week start":        func(d *DatePicker) { d.Calendar.WeekStart = "wednesday" },
		"year out of range":         func(d *DatePicker) { d.Calendar.FromYear = 12345 },
		"years the wrong way round": func(d *DatePicker) { d.Calendar.FromYear, d.Calendar.ToYear = 2020, 2010 },
		"malformed start month":     func(d *DatePicker) { d.Calendar.StartOn = "2026-08" },
		"preset without a label":    func(d *DatePicker) { d.Calendar.Presets[0].Label = "" },
		"preset with both kinds":    func(d *DatePicker) { d.Calendar.Presets[0].Start = "2026-08-01" },
		"preset with neither":       func(d *DatePicker) { d.Calendar.Presets[0].Span = "" },
		"unknown span":              func(d *DatePicker) { d.Calendar.Presets[0].Span = "last-fortnight" },
		"preset end without start":  func(d *DatePicker) { d.Calendar.Presets[2].Start = "" },
		"backwards preset":          func(d *DatePicker) { d.Calendar.Presets[2].End = "2026-09-01" },
		"widget item with key and text": func(d *DatePicker) {
			d.Details = Descriptions{Items: []DescriptionItem{{Label: "Booking", Key: "reference", Text: "x"}}}
		},
		"unsafe theme": func(d *DatePicker) { d.Theme = &theme.Theme{ColorText: "red}</style>"} },
	}
	if err := canonicalDatePicker().Validate(); err != nil {
		t.Fatalf("canonical date picker must validate, got: %v", err)
	}
	for name, mutate := range cases {
		d := canonicalDatePicker()
		mutate(d)
		if err := d.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		} else if _, derr := d.Document(); derr == nil {
			t.Errorf("%s: Document() = nil error, want the validation error", name)
		}
	}
}

// A single-date picker is the zero value of Mode, and the plainest thing the
// widget can be.
func TestDatePickerSingleMinimal(t *testing.T) {
	d := &DatePicker{
		URI:    "ui://demo/when",
		Prompt: "When?",
		Submit: DateSubmit{Tool: "schedule"},
	}
	if err := d.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	if d.valueArg() != "date" || d.endArg() != "" {
		t.Errorf("args = %q/%q, want date and none", d.valueArg(), d.endArg())
	}
}

func TestCalendarMonthsDefault(t *testing.T) {
	var none *Calendar
	if got := none.months(DateSingle); got != 1 {
		t.Errorf("single months = %d, want 1", got)
	}
	if got := none.months(DateRange); got != 2 {
		t.Errorf("range months = %d, want 2", got)
	}
	if got := (&Calendar{Months: 3}).months(DateRange); got != 3 {
		t.Errorf("explicit months = %d, want 3", got)
	}
}

// A nil Calendar is the zero value: valid, and serialized as the shape the
// runtime needs rather than as nothing at all.
func TestCalendarNilConfig(t *testing.T) {
	var none *Calendar
	if err := none.validate("ctx", DateRange); err != nil {
		t.Fatalf("validate() = %v, want nil", err)
	}
	b, _ := json.Marshal(none.config(DateSingle))
	if string(b) != `{"mode":"single","months":1}` {
		t.Errorf("config = %s", b)
	}
}

func TestDatePickerConfigChatPrompt(t *testing.T) {
	d := canonicalDatePicker()
	d.Submit.ChatPrompt = "Book the room"
	d.Submit.Args = map[string]ArgSource{"booking": FromRow("id")}

	// Inspect the submit block alone: the cancel side keeps its own args, so a
	// search over the whole island would find those instead.
	submit, _ := json.Marshal(d.config()["submit"])
	got := string(submit)

	if !strings.Contains(got, `"chatPrompt":"Book the room"`) {
		t.Errorf("submit config missing chatPrompt\nfull: %s", got)
	}
	// The chat path never calls the tool from the view, so its args are dead
	// weight and must not reach the island.
	if strings.Contains(got, `"args"`) {
		t.Errorf("submit config should drop args when chatPrompt is set\nfull: %s", got)
	}
}
