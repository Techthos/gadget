package gomukit

import (
	"encoding/json"
	"strings"
	"testing"
)

// inputItems is a details block that asks as well as states: the two go in
// one list, which is the point of the feature.
func inputItems() Descriptions {
	return Descriptions{Items: []DescriptionItem{
		{Label: "Booking", Key: "reference"},
		{Label: "Guests", Key: "guests", Input: &Input{
			Name:       "guests",
			Type:       InputNumber,
			Default:    2,
			Required:   true,
			Validation: &Validation{Min: f64(1), Max: f64(8), Step: f64(1), Message: "Between 1 and 8."},
		}},
		{Label: "Bed", Input: &Input{
			Name:        "bed",
			Type:        InputSelect,
			Placeholder: "Pick one",
			Options:     []Option{Opt("double"), Opt("twin")},
		}},
		{Label: "Late arrival", Input: &Input{Name: "late", Type: InputCheckbox, Default: true}},
		{Label: "Notes", Input: &Input{Name: "notes", Placeholder: "Anything else?"}},
	}}
}

func TestDescriptionInputValidation(t *testing.T) {
	if err := inputItems().validate("details", true); err != nil {
		t.Fatalf("a block of inputs must validate, got: %v", err)
	}

	cases := map[string]DescriptionItem{
		"no name":                {Label: "Guests", Input: &Input{Type: InputNumber}},
		"unknown type":           {Label: "Guests", Input: &Input{Name: "guests", Type: "range"}},
		"select without options": {Label: "Bed", Input: &Input{Name: "bed", Type: InputSelect}},
		"options without select": {Label: "Bed", Input: &Input{Name: "bed", Options: []Option{Opt("double")}}},
		"non-bool checkbox default": {
			Label: "Late", Input: &Input{Name: "late", Type: InputCheckbox, Default: "yes"},
		},
		"input with text":     {Label: "Guests", Text: "2", Input: &Input{Name: "guests"}},
		"typed input":         {Label: "Guests", Type: ColNumber, Input: &Input{Name: "guests"}},
		"formatted input":     {Label: "Guests", Format: "currency:EUR", Input: &Input{Name: "guests"}},
		"badge input":         {Label: "Guests", Badge: map[string]BadgeVariant{"x": BadgeInfo}, Input: &Input{Name: "guests"}},
		"link input":          {Label: "Guests", Link: &LinkSpec{HrefKey: "url"}, Input: &Input{Name: "guests"}},
		"input without label": {Input: &Input{Name: "guests"}},
	}
	for name, item := range cases {
		d := Descriptions{Items: []DescriptionItem{item}}
		if err := d.validate("details", true); err == nil {
			t.Errorf("%s: validate() = nil, want error", name)
		}
	}

	dup := Descriptions{Items: []DescriptionItem{
		{Label: "Guests", Input: &Input{Name: "guests"}},
		{Label: "Guests again", Input: &Input{Name: "guests"}},
	}}
	if err := dup.validate("details", true); err == nil {
		t.Error("duplicate input names: validate() = nil, want error")
	}

	// Two items may read the same field when one of them only prefills a
	// control from it — the rule against duplicates is about what is displayed.
	both := Descriptions{Items: []DescriptionItem{
		{Label: "Guests", Key: "guests"},
		{Label: "Change to", Key: "guests", Input: &Input{Name: "guests", Type: InputNumber}},
	}}
	if err := both.validate("details", true); err != nil {
		t.Errorf("a shown field prefilling a control must validate, got: %v", err)
	}
}

// A block that cannot carry what an input collects must refuse one outright,
// rather than render a control nothing reads.
func TestDescriptionInputsRejectedWhereReadOnly(t *testing.T) {
	d := Descriptions{Items: []DescriptionItem{{Label: "Guests", Input: &Input{Name: "guests"}}}}
	err := d.validate("details", false)
	if err == nil {
		t.Fatal("validate() = nil, want error")
	}
	if !strings.Contains(err.Error(), "read-only") {
		t.Errorf("error %q does not say why", err)
	}
}

func TestDescriptionInputConfig(t *testing.T) {
	items := inputItems().config()
	raw, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}

	// A stating item still serializes as the column it always was.
	if _, ok := got[0]["input"]; ok {
		t.Error("a value item must not carry an input")
	}
	if got[0]["type"] != "text" {
		t.Errorf("value item type = %v, want text", got[0]["type"])
	}

	guests, ok := got[1]["input"].(map[string]any)
	if !ok {
		t.Fatalf("item 1 carries no input: %v", got[1])
	}
	want := map[string]any{
		"name": "guests", "type": "number", "default": 2.0, "required": true,
		"min": 1.0, "max": 8.0, "step": 1.0, "message": "Between 1 and 8.",
	}
	for k, v := range want {
		if guests[k] != v {
			t.Errorf("guests input %s = %v, want %v", k, guests[k], v)
		}
	}
	// The prefill source travels as the item's key; nothing about the control
	// is typed or formatted like a value.
	if got[1]["key"] != "guests" {
		t.Errorf("prefill key = %v, want guests", got[1]["key"])
	}
	if _, ok := got[1]["type"]; ok {
		t.Error("an input item must not carry a column type")
	}

	bed, _ := got[2]["input"].(map[string]any)
	opts, ok := bed["options"].([]any)
	if !ok || len(opts) != 2 {
		t.Fatalf("bed options = %v, want two", bed["options"])
	}
	if bed["placeholder"] != "Pick one" {
		t.Errorf("bed placeholder = %v", bed["placeholder"])
	}

	late, _ := got[3]["input"].(map[string]any)
	if late["default"] != true {
		t.Errorf("late default = %v, want true", late["default"])
	}

	notes, _ := got[4]["input"].(map[string]any)
	if notes["type"] != "text" {
		t.Errorf("notes type = %v, want the text default", notes["type"])
	}
	if _, ok := notes["required"]; ok {
		t.Error("an optional input must not carry required")
	}
}

func TestDescriptionInputNames(t *testing.T) {
	got := inputItems().inputNames()
	want := []string{"guests", "bed", "late", "notes"}
	if len(got) != len(want) {
		t.Fatalf("inputNames() = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("inputNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}
