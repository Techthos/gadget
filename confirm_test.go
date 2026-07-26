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

// canonicalConfirm exercises every slot: both guards, a mixed detail list,
// authored effects, and both outcomes.
func canonicalConfirm() *Confirm {
	return &Confirm{
		URI:      "ui://demo/delete-user",
		Title:    "Delete user",
		Prompt:   "Delete Ada Lovelace?",
		Body:     "The account and everything attached to it goes away.",
		Severity: BadgeDanger,
		Details: Descriptions{Items: []DescriptionItem{
			{Label: "User", Key: "name"},
			{Label: "Balance", Key: "balance", Type: ColNumber, Format: "currency:EUR"},
			{Label: "Status", Key: "status", Type: ColBadge, Badge: map[string]BadgeVariant{
				"active": BadgeSuccess,
				"banned": BadgeDanger,
			}},
			{Label: "Website", Type: ColLink, Link: &LinkSpec{HrefKey: "website"}},
			{Label: "Region", Text: "eu-central-1"},
		}},
		Effects: []Effect{
			{Text: "Removes the account", Severity: BadgeDanger},
			{Text: "Deletes audit records", Detail: "Not recoverable.", Value: "12", Severity: BadgeWarning},
			{Text: "Notifies the team", Severity: BadgeInfo},
		},
		Acknowledge:   "I understand this cannot be undone.",
		TypeToConfirm: "ada@example.com",
		Accept: AcceptSpec{
			Tool:           "delete_user",
			Label:          "Delete user",
			Args:           map[string]ArgSource{"id": FromRow("id"), "hard": Static(true)},
			SuccessMessage: "User deleted.",
		},
		Reject: &RejectSpec{Label: "Keep user", Tool: "cancel_deletion", Args: map[string]ArgSource{"id": FromRow("id")}},
		InitialData: map[string]any{
			"rows": []map[string]any{
				{"id": 1, "name": "Ada", "balance": 12.5, "status": "active", "website": "https://example.com"},
			},
		},
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func TestConfirmGolden(t *testing.T) {
	doc, err := canonicalConfirm().Document()
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "confirm.html")
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
		t.Error("document does not match golden file; run `go test -run TestConfirmGolden -update ./...` and review the diff")
	}
	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, want := range []string{
		`data-gadget-widget="confirm"`,
		`gadget-confirm--danger`,
		`Delete Ada Lovelace?`,
		`data-gadget-descriptions`,
		`data-gadget-effects`,
		`data-gadget-ack`,
		`data-gadget-phrase`,
		`<code>ada@example.com</code>`,
		`data-gadget-reject`,
		`data-gadget-accept`,
		`data-gadget-outcome`,
		`--gadget-color-primary:#7c3aed`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q", want)
		}
	}
	// The guards are enforced before the runtime mounts.
	if !strings.Contains(doc, `data-gadget-accept="" disabled`) {
		t.Error("guarded accept button must render disabled")
	}
	// Detail values are runtime state; only the labels are authored, and they
	// travel in the config island rather than the markup.
	if strings.Contains(doc, `>eu-central-1<`) {
		t.Error("detail values must not be server-rendered")
	}
}

// confirmShell renders the widget markup alone. Negative assertions cannot
// run against the whole document: the inlined stylesheet and runtime mention
// every class and data attribute the library knows.
func confirmShell(t *testing.T, c *Confirm) string {
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

func TestConfirmMinimalShell(t *testing.T) {
	c := &Confirm{URI: "ui://demo/ok", Prompt: "Proceed?", Accept: AcceptSpec{Tool: "go"}}
	shell := confirmShell(t, c)
	doc, err := c.Document()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	// Every optional block is absent, including the toolbar: an untitled,
	// unbranded confirmation is the question and nothing above it.
	for _, unwanted := range []string{
		"data-gadget-descriptions", "data-gadget-ack", "data-gadget-phrase",
		"data-gadget-reject", "gadget-toolbar", "gadget-confirm-lede",
	} {
		if strings.Contains(shell, unwanted) {
			t.Errorf("minimal shell should not contain %q:\n%s", unwanted, shell)
		}
	}
	for _, want := range []string{
		`gadget-confirm--info`,
		`class="gadget-btn gadget-btn--primary" data-gadget-accept`,
		`>Confirm</button>`,
	} {
		if !strings.Contains(shell, want) {
			t.Errorf("minimal shell missing %q:\n%s", want, shell)
		}
	}
	// Nothing is disabled when there is no guard to satisfy.
	if strings.Contains(shell, "disabled") {
		t.Errorf("unguarded accept button must not render disabled:\n%s", shell)
	}
}

func TestConfirmConfigIsland(t *testing.T) {
	b, err := json.Marshal(canonicalConfirm().config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{
		`"widget":"confirm"`,
		`"rowsKey":"rows"`,
		`"effectsKey":"effects"`,
		`"rowId":"id"`,
		`"accept":{"args":{"hard":{"static":true},"id":{"row":"id"}},"successMessage":"User deleted.","tool":"delete_user"}`,
		`"reject":{"args":{"id":{"row":"id"}},"message":"Cancelled.","tool":"cancel_deletion"}`,
		`"acknowledge":true`,
		`"typeToConfirm":"ada@example.com"`,
		`{"key":"name","label":"User","type":"text"}`,
		`{"format":"currency:EUR","key":"balance","label":"Balance","type":"number"}`,
		`{"key":"","label":"Website","link":{"hrefKey":"website"},"type":"link"}`,
		`{"key":"","label":"Region","text":"eu-central-1","type":"text"}`,
		`{"detail":"Not recoverable.","severity":"warning","text":"Deletes audit records","value":"12"}`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
	// Sorting is meaningless for a detail list.
	if strings.Contains(cfg, `"sortable"`) {
		t.Errorf("config island should not carry sortable: %s", cfg)
	}
}

func TestConfirmConfigDefaults(t *testing.T) {
	c := &Confirm{URI: "ui://demo/ok", Prompt: "Proceed?", Accept: AcceptSpec{Tool: "go"}, Reject: &RejectSpec{}}
	b, _ := json.Marshal(c.config())
	cfg := string(b)
	for _, want := range []string{`"reject":{"message":"Cancelled."}`, `"accept":{"tool":"go"}`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
	for _, unwanted := range []string{"details", "effects\":[", "acknowledge", "typeToConfirm", "loadTool"} {
		if strings.Contains(cfg, unwanted) {
			t.Errorf("config island should not carry %q: %s", unwanted, cfg)
		}
	}
}

func TestConfirmConfigLoadTool(t *testing.T) {
	c := canonicalConfirm()
	c.LoadTool = "get_user"
	c.LoadArgs = map[string]any{"id": 1}
	b, _ := json.Marshal(c.config())
	cfg := string(b)
	for _, want := range []string{`"loadTool":"get_user"`, `"loadArgs":{"id":1}`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
}

func TestConfirmToolMetaAndDescriptor(t *testing.T) {
	c := canonicalConfirm()
	meta, _ := json.Marshal(c.ToolMeta())
	if string(meta) != `{"ui":{"resourceUri":"ui://demo/delete-user"}}` {
		t.Errorf("ToolMeta = %s", meta)
	}
	d := c.Descriptor()
	if d.URI != "ui://demo/delete-user" || d.Name != "demo-delete-user" || d.MIMEType != "text/html;profile=mcp-app" {
		t.Errorf("Descriptor = %+v", d)
	}
}

func TestConfirmAcceptVariant(t *testing.T) {
	cases := map[string]struct {
		mutate func(*Confirm)
		want   ActionVariant
	}{
		"danger severity":  {func(c *Confirm) { c.Severity = BadgeDanger }, VariantDanger},
		"warning severity": {func(c *Confirm) { c.Severity = BadgeWarning }, VariantPrimary},
		"default severity": {func(c *Confirm) { c.Severity = "" }, VariantPrimary},
		"explicit variant": {func(c *Confirm) { c.Accept.Variant = VariantPrimary }, VariantPrimary},
	}
	for name, tc := range cases {
		c := canonicalConfirm()
		tc.mutate(c)
		if got := c.acceptVariant(); got != tc.want {
			t.Errorf("%s: acceptVariant() = %q, want %q", name, got, tc.want)
		}
	}
}

func TestConfirmValidate(t *testing.T) {
	cases := map[string]func(*Confirm){
		"bad URI scheme":   func(c *Confirm) { c.URI = "https://x" },
		"no prompt":        func(c *Confirm) { c.Prompt = "" },
		"no accept tool":   func(c *Confirm) { c.Accept.Tool = "" },
		"unknown severity": func(c *Confirm) { c.Severity = "critical" },
		"selection arg in accept": func(c *Confirm) {
			c.Accept.Args = map[string]ArgSource{"ids": FromSelection("id")}
		},
		"selection arg in reject": func(c *Confirm) {
			c.Reject = &RejectSpec{Tool: "x", Args: map[string]ArgSource{"ids": FromSelection("id")}}
		},
		"empty arg source": func(c *Confirm) { c.Accept.Args = map[string]ArgSource{"id": {}} },
		"reject args without tool": func(c *Confirm) {
			c.Reject = &RejectSpec{Args: map[string]ArgSource{"id": FromRow("id")}}
		},
		"item without label": func(c *Confirm) {
			c.Details = Descriptions{Items: []DescriptionItem{{Key: "name"}}}
		},
		"item without value": func(c *Confirm) {
			c.Details = Descriptions{Items: []DescriptionItem{{Label: "User"}}}
		},
		"item with key and text": func(c *Confirm) {
			c.Details = Descriptions{Items: []DescriptionItem{{Label: "User", Key: "name", Text: "Ada"}}}
		},
		"typed text item": func(c *Confirm) {
			c.Details = Descriptions{Items: []DescriptionItem{{Label: "Balance", Text: "12", Type: ColNumber}}}
		},
		"link item without href": func(c *Confirm) {
			c.Details = Descriptions{Items: []DescriptionItem{{Label: "Site", Key: "website", Type: ColLink}}}
		},
		"actions item": func(c *Confirm) {
			c.Details = Descriptions{Items: []DescriptionItem{{Label: "Do", Key: "x", Type: ColActions}}}
		},
		"duplicate item key": func(c *Confirm) {
			c.Details.Items = append(c.Details.Items, DescriptionItem{Label: "Name again", Key: "name"})
		},
		"effect without text":     func(c *Confirm) { c.Effects = []Effect{{Detail: "orphan"}} },
		"unknown effect severity": func(c *Confirm) { c.Effects = []Effect{{Text: "x", Severity: "fatal"}} },
		"unsafe theme":            func(c *Confirm) { c.Theme = &theme.Theme{ColorText: "red}</style>"} },
	}
	if err := canonicalConfirm().Validate(); err != nil {
		t.Fatalf("canonical confirm must validate, got: %v", err)
	}
	for name, mutate := range cases {
		c := canonicalConfirm()
		mutate(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		} else if _, derr := c.Document(); derr == nil {
			t.Errorf("%s: Document() = nil error, want the validation error", name)
		}
	}
}

func TestConfirmConfigChatPrompt(t *testing.T) {
	c := &Confirm{
		URI:    "ui://demo/ok",
		Prompt: "Proceed?",
		Accept: AcceptSpec{
			Tool:       "delete_user",
			ChatPrompt: "Delete the account for Ada",
			Args:       map[string]ArgSource{"id": FromRow("id")},
		},
	}
	b, _ := json.Marshal(c.config())
	cfg := string(b)

	if !strings.Contains(cfg, `"chatPrompt":"Delete the account for Ada"`) {
		t.Errorf("config island missing chatPrompt\nfull: %s", cfg)
	}
	// The chat path never calls the tool from the view, so its args are dead
	// weight and must not reach the island.
	if strings.Contains(cfg, `"args"`) {
		t.Errorf("config island should drop args when chatPrompt is set\nfull: %s", cfg)
	}
}
