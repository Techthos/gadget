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

// canonicalChoice exercises every slot: a context record, options with prose,
// bullets, typed details and a badge, a default, a disabled option, and both
// outcomes.
func canonicalChoice() *Choice {
	return &Choice{
		URI:    "ui://demo/shipping",
		Title:  "Shipping",
		Prompt: "How should we ship order #4471?",
		Body:   "The parcel is packed and leaves the warehouse today either way.",
		Details: Descriptions{Items: []DescriptionItem{
			{Label: "Order", Key: "reference"},
			{Label: "Destination", Text: "Berlin, DE"},
		}},
		Options: []ChoiceOption{
			{
				Value:   "standard",
				Label:   "Standard",
				Summary: "3-5 business days",
				Body:    "Handed to the postal service tonight.",
				Bullets: []string{"Tracked to the depot", "No signature"},
				Details: Descriptions{Items: []DescriptionItem{
					{Label: "Price", Key: "price", Type: ColNumber, Format: "currency:EUR"},
					{Label: "Arrives", Key: "eta", Type: ColDate},
				}},
				Data:    map[string]any{"price": 4.9, "eta": "2026-08-03"},
				Default: true,
			},
			{
				Value:        "express",
				Label:        "Express",
				Summary:      "next business day",
				Body:         "Arrives before 12:00 tomorrow.",
				Bullets:      []string{"Tracked end to end", "Signature required"},
				Badge:        "fastest",
				BadgeVariant: BadgeSuccess,
				Details: Descriptions{Items: []DescriptionItem{
					{Label: "Price", Key: "price", Type: ColNumber, Format: "currency:EUR"},
				}},
				Data: map[string]any{"price": 14.9},
			},
			{
				Value:    "pickup",
				Label:    "Depot pickup",
				Summary:  "not available for this address",
				Disabled: true,
			},
		},
		Submit: ChoiceSubmit{
			Tool:           "ship_order",
			Label:          "Ship it",
			ValueArg:       "method",
			Args:           map[string]ArgSource{"order": FromRow("id"), "notify": Static(true)},
			SuccessMessage: "On its way.",
		},
		Cancel: &RejectSpec{Label: "Decide later", Tool: "postpone", Args: map[string]ArgSource{"order": FromRow("id")}},
		InitialData: map[string]any{
			"rows": []map[string]any{{"id": 4471, "reference": "ORD-4471"}},
		},
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func TestChoiceGolden(t *testing.T) {
	doc, err := canonicalChoice().Document()
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "choice.html")
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
		t.Error("document does not match golden file; run `go test -run TestChoiceGolden -update ./...` and review the diff")
	}
	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, want := range []string{
		`data-gomu-widget="choice"`,
		`gomu-choice--auto`,
		`How should we ship order #4471?`,
		`data-gomu-descriptions`,
		`data-gomu-options`,
		`data-gomu-panel`,
		`data-gomu-hint`,
		`data-gomu-cancel`,
		`data-gomu-submit`,
		`data-gomu-outcome`,
		`>Ship it</button>`,
		`>Decide later</button>`,
		`role="radiogroup"`,
		`--gomu-color-primary:#7c3aed`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q", want)
		}
	}
	// Nothing can be submitted before the runtime knows what is chosen.
	if !strings.Contains(doc, `data-gomu-submit="" disabled`) {
		t.Error("submit button must render disabled")
	}
	// Options are runtime state: an option list can be replaced wholesale by a
	// tool result, so it travels in the config island, not the markup.
	if strings.Contains(doc, `>Express<`) {
		t.Error("option labels must not be server-rendered")
	}
}

// choiceShell renders the widget markup alone. Negative assertions cannot run
// against the whole document: the inlined stylesheet and runtime mention every
// class and data attribute the library knows.
func choiceShell(t *testing.T, c *Choice) string {
	t.Helper()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := c.shell().Render(&b); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func TestChoiceMinimalShell(t *testing.T) {
	c := &Choice{
		URI:     "ui://demo/pick",
		Prompt:  "Which one?",
		Options: []ChoiceOption{{Value: "a"}, {Value: "b"}},
		Submit:  ChoiceSubmit{Tool: "pick"},
	}
	shell := choiceShell(t, c)
	doc, err := c.Document()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, unwanted := range []string{
		"data-gomu-descriptions", "data-gomu-cancel", "gomu-toolbar", "gomu-choice-lede",
	} {
		if strings.Contains(shell, unwanted) {
			t.Errorf("minimal shell should not contain %q:\n%s", unwanted, shell)
		}
	}
	for _, want := range []string{
		`gomu-choice--auto`,
		`class="gomu-btn gomu-btn--primary" data-gomu-submit`,
		`>Continue</button>`,
	} {
		if !strings.Contains(shell, want) {
			t.Errorf("minimal shell missing %q:\n%s", want, shell)
		}
	}
}

func TestChoiceMultipleShell(t *testing.T) {
	c := canonicalChoice()
	c.Multiple = true
	c.Min = 2
	c.Max = 3
	c.Options[0].Default = false
	shell := choiceShell(t, c)
	if !strings.Contains(shell, `role="group"`) {
		t.Errorf("a multiple choice is not a radiogroup:\n%s", shell)
	}
}

func TestChoiceLayoutClass(t *testing.T) {
	for layout, want := range map[ChoiceLayout]string{
		ChoiceAuto:    "gomu-choice--auto",
		ChoiceSplit:   "gomu-choice--split",
		ChoiceStacked: "gomu-choice--stacked",
	} {
		c := canonicalChoice()
		c.Layout = layout
		if shell := choiceShell(t, c); !strings.Contains(shell, want) {
			t.Errorf("layout %q: shell missing %q", layout, want)
		}
	}
}

func TestChoiceConfigIsland(t *testing.T) {
	b, err := json.Marshal(canonicalChoice().config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{
		`"widget":"choice"`,
		`"layout":"auto"`,
		`"rowsKey":"rows"`,
		`"optionsKey":"options"`,
		`"rowId":"id"`,
		`"min":1`,
		`"submit":{"args":{"notify":{"static":true},"order":{"row":"id"}},"successMessage":"On its way.","tool":"ship_order","valueArg":"method"}`,
		`"cancel":{"args":{"order":{"row":"id"}},"message":"Cancelled.","tool":"postpone"}`,
		`{"key":"reference","label":"Order","type":"text"}`,
		`"data":{"eta":"2026-08-03","price":4.9}`,
		`"bullets":["Tracked end to end","Signature required"]`,
		`"badge":"fastest","badgeVariant":"success"`,
		`"default":true`,
		`"disabled":true`,
		`{"format":"currency:EUR","key":"price","label":"Price","type":"number"}`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
	// A single choice has no bounds to enforce beyond "one".
	for _, unwanted := range []string{`"multiple"`, `"max"`} {
		if strings.Contains(cfg, unwanted) {
			t.Errorf("config island should not carry %s: %s", unwanted, cfg)
		}
	}
	// Sorting is meaningless for a detail list.
	if strings.Contains(cfg, `"sortable"`) {
		t.Errorf("config island should not carry sortable: %s", cfg)
	}
}

func TestChoiceConfigDefaults(t *testing.T) {
	c := &Choice{
		URI:     "ui://demo/pick",
		Prompt:  "Which one?",
		Options: []ChoiceOption{{Value: "a"}},
		Submit:  ChoiceSubmit{Tool: "pick"},
		Cancel:  &RejectSpec{},
	}
	b, _ := json.Marshal(c.config())
	cfg := string(b)
	for _, want := range []string{
		`"submit":{"tool":"pick","valueArg":"choice"}`,
		`"cancel":{"message":"Cancelled."}`,
		// An option with no label answers with its value.
		`"options":[{"label":"a","value":"a"}]`,
		`"min":1`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
	for _, unwanted := range []string{"details", "loadTool", "multiple"} {
		if strings.Contains(cfg, unwanted) {
			t.Errorf("config island should not carry %q: %s", unwanted, cfg)
		}
	}
}

func TestChoiceConfigMultiple(t *testing.T) {
	c := canonicalChoice()
	c.Multiple = true
	c.Min = 2
	c.Max = 3
	c.LoadTool = "get_shipping_options"
	c.LoadArgs = map[string]any{"order": 4471}
	b, _ := json.Marshal(c.config())
	cfg := string(b)
	for _, want := range []string{
		`"multiple":true`,
		`"min":2`,
		`"max":3`,
		`"loadTool":"get_shipping_options"`,
		`"loadArgs":{"order":4471}`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
}

func TestChoiceToolMetaAndDescriptor(t *testing.T) {
	c := canonicalChoice()
	meta, _ := json.Marshal(c.ToolMeta())
	if string(meta) != `{"ui":{"resourceUri":"ui://demo/shipping"}}` {
		t.Errorf("ToolMeta = %s", meta)
	}
	d := c.Descriptor()
	if d.URI != "ui://demo/shipping" || d.Name != "demo-shipping" || d.MIMEType != "text/html;profile=mcp-app" {
		t.Errorf("Descriptor = %+v", d)
	}
}

func TestChoiceValidate(t *testing.T) {
	cases := map[string]func(*Choice){
		"bad URI scheme": func(c *Choice) { c.URI = "https://x" },
		"no prompt":      func(c *Choice) { c.Prompt = "" },
		"no submit tool": func(c *Choice) { c.Submit.Tool = "" },
		"unknown layout": func(c *Choice) { c.Layout = "columns" },
		"selection arg in submit": func(c *Choice) {
			c.Submit.Args = map[string]ArgSource{"ids": FromSelection("id")}
		},
		"empty arg source": func(c *Choice) { c.Submit.Args = map[string]ArgSource{"order": {}} },
		"value arg collides with a static arg": func(c *Choice) {
			c.Submit.Args = map[string]ArgSource{"method": Static("express")}
		},
		"cancel args without tool": func(c *Choice) {
			c.Cancel = &RejectSpec{Args: map[string]ArgSource{"order": FromRow("id")}}
		},
		"selection arg in cancel": func(c *Choice) {
			c.Cancel = &RejectSpec{Tool: "x", Args: map[string]ArgSource{"ids": FromSelection("id")}}
		},
		"bounds without multiple": func(c *Choice) { c.Max = 2 },
		"negative bound":          func(c *Choice) { c.Multiple, c.Min = true, -1 },
		"min above max":           func(c *Choice) { c.Multiple, c.Min, c.Max = true, 3, 2 },
		"option without value":    func(c *Choice) { c.Options[1].Value = "" },
		"duplicate option value":  func(c *Choice) { c.Options[1].Value = "standard" },
		"unknown badge variant":   func(c *Choice) { c.Options[1].BadgeVariant = "critical" },
		"badge variant without text": func(c *Choice) {
			c.Options[1].Badge, c.Options[1].BadgeVariant = "", BadgeInfo
		},
		"empty bullet":                    func(c *Choice) { c.Options[1].Bullets = []string{"ok", ""} },
		"disabled default":                func(c *Choice) { c.Options[2].Default = true },
		"two defaults in a single choice": func(c *Choice) { c.Options[1].Default = true },
		"defaults above max": func(c *Choice) {
			c.Multiple, c.Max = true, 1
			c.Options[1].Default = true
		},
		"option item without label": func(c *Choice) {
			c.Options[1].Details = Descriptions{Items: []DescriptionItem{{Key: "price"}}}
		},
		"widget item with key and text": func(c *Choice) {
			c.Details = Descriptions{Items: []DescriptionItem{{Label: "Order", Key: "reference", Text: "x"}}}
		},
		"unsafe theme": func(c *Choice) { c.Theme = &theme.Theme{ColorText: "red}</style>"} },
	}
	if err := canonicalChoice().Validate(); err != nil {
		t.Fatalf("canonical choice must validate, got: %v", err)
	}
	for name, mutate := range cases {
		c := canonicalChoice()
		mutate(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		} else if _, derr := c.Document(); derr == nil {
			t.Errorf("%s: Document() = nil error, want the validation error", name)
		}
	}
}

// An option list may arrive entirely at runtime, so an authored-empty widget
// is legal — it is the one case where the document renders nothing to pick.
func TestChoiceAllowsRuntimeOnlyOptions(t *testing.T) {
	c := &Choice{
		URI:        "ui://demo/pick",
		Prompt:     "Which one?",
		Submit:     ChoiceSubmit{Tool: "pick"},
		OptionsKey: "choices",
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("Validate() = %v, want nil", err)
	}
	b, _ := json.Marshal(c.config())
	if !strings.Contains(string(b), `"optionsKey":"choices"`) {
		t.Errorf("config island missing the options key: %s", b)
	}
}

func TestChoiceConfigChatPrompt(t *testing.T) {
	c := canonicalChoice()
	c.Submit.ChatPrompt = "Ship order ORD-4471"
	c.Submit.Args = map[string]ArgSource{"order": FromRow("id")}

	// Inspect the submit block alone: the cancel side keeps its own args, so a
	// search over the whole island would find those instead.
	submit, _ := json.Marshal(c.config()["submit"])
	got := string(submit)

	if !strings.Contains(got, `"chatPrompt":"Ship order ORD-4471"`) {
		t.Errorf("submit config missing chatPrompt\nfull: %s", got)
	}
	// The chat path never calls the tool from the view, so its args are dead
	// weight and must not reach the island.
	if strings.Contains(got, `"args"`) {
		t.Errorf("submit config should drop args when chatPrompt is set\nfull: %s", got)
	}
}
