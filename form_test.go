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

func f64(v float64) *float64 { return &v }
func iptr(v int) *int        { return &v }

// canonicalForm exercises every field type.
func canonicalForm() *Form {
	return &Form{
		URI:   "ui://demo/user-form",
		Title: "Edit user",
		Fields: []Field{
			{Name: "name", Label: "Name", Required: true, Placeholder: "Full name",
				Validation: &Validation{MinLen: iptr(2), MaxLen: iptr(80), Message: "Enter the full name."}},
			{Name: "bio", Label: "Bio", Type: FTextarea, Rows: 4, Description: "Shown on the profile."},
			{Name: "age", Label: "Age", Type: FNumber, Validation: &Validation{Min: f64(0), Max: f64(150), Step: f64(1)}},
			{Name: "active", Label: "Active", Type: FCheckbox, Default: true},
			{Name: "role", Label: "Role", Type: FSelect, Required: true, Default: "user",
				Options: []Option{Opt("user"), {Value: "admin", Label: "Administrator"}}},
			{Name: "tags", Label: "Tags", Type: FMultiSelect, Default: []string{"a"},
				Options: []Option{Opt("a"), Opt("b"), Opt("c")}},
			{Name: "birthday", Label: "Birthday", Type: FDate,
				Calendar: &Calendar{Max: "2026-01-01", MonthDropdowns: true, FromYear: 1920, ToYear: 2026}},
			{Name: "stay", Label: "Stay", Type: FDateRange, EndName: "stay_until", Required: true,
				Default:  []string{"2026-08-20", "2026-08-23"},
				Calendar: &Calendar{Min: "2026-08-01", Presets: []DatePreset{{Label: "This week", Span: SpanThisWeek}}}},
			{Name: "alarm", Label: "Alarm", Type: FTime},
			{Name: "id", Type: FHidden, Default: "42"},
			{Name: "createdAt", Label: "Created", Type: FReadonly, Default: "2026-01-01"},
		},
		Submit: SubmitSpec{Tool: "save_user", Label: "Save",
			StaticArgs: map[string]any{"source": "widget"}, SuccessMessage: "User saved."},
		Cancel: &CancelSpec{},
		Theme:  &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func TestFormGolden(t *testing.T) {
	doc, err := canonicalForm().Document()
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "golden", "form.html")
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
		t.Error("document does not match golden file; run `go test -run TestFormGolden -update ./...` and review the diff")
	}

	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, want := range []string{
		`data-gomu-widget="form"`,
		`<form class="gomu-form" data-gomu-form`,
		`name="name" class="gomu-input" placeholder="Full name" required minlength="2" maxlength="80" type="text"`,
		`<textarea id="gomu-f-bio"`,
		`min="0" max="150" step="1" type="number"`,
		`<span class="gomu-check"><input type="checkbox" id="gomu-f-active" name="active" checked>`,
		`<path class="gomu-check-tick"`,
		`<select id="gomu-f-role"`,
		`<option value="admin"`,
		`multiple`,
		`type="date"`,
		`type="time"`,
		// A date field's window is a native constraint too, so the fallback
		// control and checkValidity() agree with the grid.
		`max="2026-01-01" type="date"`,
		// A range is its two value holders, named after the two tool arguments.
		`<div class="gomu-daterange" data-gomu-daterange="stay">`,
		`name="stay" class="gomu-input gomu-daterange-start" aria-label="Stay start date"`,
		`name="stay_until" class="gomu-input gomu-daterange-end" aria-label="Stay end date"`,
		`value="2026-08-20"`,
		`value="2026-08-23"`,
		`type="hidden" name="id" value="42"`,
		`readonly`,
		`data-gomu-error-for="name"`,
		`data-gomu-cancel`,
		// Not type="submit": sandboxed hosts block native form submission.
		`data-gomu-submit`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q", want)
		}
	}
}

func TestFormConfigIsland(t *testing.T) {
	b, err := json.Marshal(canonicalForm().config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{
		`"widget":"form"`,
		`"prefillKey":"values"`,
		`"errorsKey":"errors"`,
		`"submit":{"staticArgs":{"source":"widget"},"successMessage":"User saved.","tool":"save_user"}`,
		`{"message":"Enter the full name.","name":"name","type":"text"}`,
		`{"name":"tags","type":"multiselect"}`,
		// A date field carries its grid: the runtime builds it, so nothing about
		// it is in the markup.
		`{"calendar":{"fromYear":1920,"max":"2026-01-01","mode":"single","monthDropdowns":true,"months":1,"toYear":2026},"name":"birthday","type":"date"}`,
		`"endName":"stay_until"`,
		`"presets":[{"label":"This week","span":"this-week"}]`,
		`"mode":"range","months":2`,
		`"required":true`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
	// Only date fields carry a grid.
	if strings.Contains(cfg, `{"calendar":{"mode":"single","months":1},"name":"alarm"`) {
		t.Errorf("a time field must not carry a calendar: %s", cfg)
	}
}

func TestFormConfigLoadTool(t *testing.T) {
	f := canonicalForm()
	f.LoadTool = "get_user"
	f.LoadArgs = map[string]any{"id": "42"}
	b, err := json.Marshal(f.config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{`"loadTool":"get_user"`, `"loadArgs":{"id":"42"}`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}

	// Both keys are omitted when LoadTool is unset.
	b2, err := json.Marshal(canonicalForm().config())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b2), "loadTool") || strings.Contains(string(b2), "loadArgs") {
		t.Errorf("load keys present without LoadTool set: %s", b2)
	}
}

func TestFormValidate(t *testing.T) {
	cases := map[string]func(*Form){
		"bad URI":              func(f *Form) { f.URI = "nope" },
		"no fields":            func(f *Form) { f.Fields = nil },
		"no submit tool":       func(f *Form) { f.Submit.Tool = "" },
		"unnamed field":        func(f *Form) { f.Fields = []Field{{Label: "X"}} },
		"duplicate field name": func(f *Form) { f.Fields = append(f.Fields, Field{Name: "name"}) },
		"select without options": func(f *Form) {
			f.Fields = []Field{{Name: "x", Type: FSelect}}
		},
		"unknown type": func(f *Form) {
			f.Fields = []Field{{Name: "x", Type: "wizard"}}
		},
		"unsafe theme": func(f *Form) { f.Theme = &theme.Theme{ColorText: "x}</style>"} },
		// A range's end argument shares the field namespace: two fields writing
		// the same tool argument would send one value where the other is meant.
		"end name collides with a field": func(f *Form) {
			f.Fields = append(f.Fields, Field{Name: "stay_until"})
		},
		"end name collides with itself": func(f *Form) {
			f.Fields = []Field{{Name: "x", Type: FDateRange, EndName: "x"}}
		},
		"end name without a range": func(f *Form) {
			f.Fields = []Field{{Name: "x", Type: FDate, EndName: "y"}}
		},
		"calendar on a text field": func(f *Form) {
			f.Fields = []Field{{Name: "x", Calendar: &Calendar{}}}
		},
		"malformed date default": func(f *Form) {
			f.Fields = []Field{{Name: "x", Type: FDate, Default: "01.02.2026"}}
		},
		"backwards range default": func(f *Form) {
			f.Fields = []Field{{Name: "x", Type: FDateRange, Default: []string{"2026-08-23", "2026-08-20"}}}
		},
		"range default outside the window": func(f *Form) {
			f.Fields = []Field{{Name: "x", Type: FDateRange, Default: []string{"2026-07-01", "2026-08-20"},
				Calendar: &Calendar{Min: "2026-08-01"}}}
		},
		"invalid calendar": func(f *Form) {
			f.Fields = []Field{{Name: "x", Type: FDate, Calendar: &Calendar{Min: "yesterday"}}}
		},
		"range preset in a single-date field": func(f *Form) {
			f.Fields = []Field{{Name: "x", Type: FDate, Calendar: &Calendar{
				Presets: []DatePreset{{Label: "Fair", Start: "2026-09-07", End: "2026-09-11"}},
			}}}
		},
	}

	if err := canonicalForm().Validate(); err != nil {
		t.Fatalf("canonical form must validate, got: %v", err)
	}
	for name, mutate := range cases {
		form := canonicalForm()
		mutate(form)
		if err := form.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		}
	}
}
