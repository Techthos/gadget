package gadget

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"

	"github.com/techthos/gadget/theme"
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
			{Name: "birthday", Label: "Birthday", Type: FDate},
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
		`data-gadget-widget="form"`,
		`<form class="gadget-form" data-gadget-form`,
		`name="name" class="gadget-input" placeholder="Full name" required minlength="2" maxlength="80" type="text"`,
		`<textarea id="gadget-f-bio"`,
		`min="0" max="150" step="1" type="number"`,
		`type="checkbox" checked`,
		`<select id="gadget-f-role"`,
		`<option value="admin"`,
		`multiple`,
		`type="date"`,
		`type="time"`,
		`type="hidden" name="id" value="42"`,
		`readonly`,
		`data-gadget-error-for="name"`,
		`data-gadget-cancel`,
		`type="submit"`,
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
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
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
