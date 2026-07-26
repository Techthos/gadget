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

// canonicalTemplate exercises every card template slot.
func canonicalTemplate() CardTemplate {
	return CardTemplate{
		Header: CardHeader{
			TitleKey:       "name",
			DescriptionKey: "email",
			Badge: Badge("status", "Status", map[string]BadgeVariant{
				"active": BadgeSuccess,
				"banned": BadgeDanger,
			}),
		},
		Content: CardContent{
			TextKey: "bio",
			Items: Descriptions{Items: []DescriptionItem{
				{Label: "Balance", Key: "balance", Type: ColNumber, Format: "currency:EUR"},
				{Label: "Created", Key: "createdAt", Type: ColDate, Format: "date"},
				{Label: "Website", Key: "website", Type: ColLink, Link: &LinkSpec{HrefKey: "website"}},
				{Label: "Plan", Text: "Free"},
			}},
		},
		Footer: CardFooter{
			Text: "Balances update hourly.",
			Actions: []Action{
				{Label: "Edit", Tool: "edit_user", Args: map[string]ArgSource{"id": FromRow("id")}, Variant: VariantPrimary},
				{Label: "Delete", Tool: "delete_user", Confirm: "Really delete?", Args: map[string]ArgSource{"id": FromRow("id")}, Variant: VariantDanger},
			},
		},
	}
}

func canonicalCard() *Card {
	return &Card{
		URI:      "ui://demo/user",
		Title:    "User",
		Template: canonicalTemplate(),
		Empty:    EmptyState{Title: "No user", Body: "Nothing selected."},
		InitialData: map[string]any{
			"rows": []map[string]any{
				{"id": 1, "name": "Ada", "email": "ada@example.com", "balance": 12.5, "createdAt": "2026-01-01T00:00:00Z", "status": "active", "website": "https://example.com"},
			},
		},
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func canonicalCardList() *CardList {
	return &CardList{
		URI:         "ui://demo/users",
		Title:       "Users",
		Template:    canonicalTemplate(),
		PageSize:    10,
		PageSizes:   []int{25, 10, 50},
		DefaultSort: &SortSpec{Key: "balance", Desc: true},
		Filterable:  true,
		Selection: &SelectionConfig{Bulk: []Action{
			{Label: "Archive", Tool: "archive_users", Args: map[string]ArgSource{"ids": FromSelection("id")}},
		}},
		Empty: EmptyState{Title: "No users", Body: "Create a user to get started."},
		InitialData: map[string]any{
			"rows": []map[string]any{
				{"id": 1, "name": "Ada", "email": "ada@example.com", "balance": 12.5, "createdAt": "2026-01-01T00:00:00Z", "status": "active", "website": "https://example.com"},
			},
		},
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func TestCardGolden(t *testing.T) {
	doc, err := canonicalCard().Document()
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "card.html")
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
		t.Error("document does not match golden file; run `go test -run TestCardGolden -update ./...` and review the diff")
	}
	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, want := range []string{
		`data-gadget-widget="card"`,
		`id="gadget-config"`,
		`id="gadget-data"`,
		`data-gadget-card`,
		`--gadget-color-primary:#7c3aed`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q", want)
		}
	}
}

func TestCardListGolden(t *testing.T) {
	doc, err := canonicalCardList().Document()
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "cardlist.html")
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
		t.Error("document does not match golden file; run `go test -run TestCardListGolden -update ./...` and review the diff")
	}
	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, want := range []string{
		`data-gadget-widget="cardlist"`,
		`class="gadget-card-strip" data-gadget-cards`,
		`data-gadget-scroll="prev"`,
		`data-gadget-scroll="next"`,
		`data-gadget-sort-select`,
		`data-gadget-select-all`,
		`data-gadget-bulk-action="0"`,
		`data-gadget-page-size`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q", want)
		}
	}
}

func TestCardConfigIsland(t *testing.T) {
	b, err := json.Marshal(canonicalCard().config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{
		`"widget":"card"`,
		`"rowsKey":"rows"`,
		`"rowId":"id"`,
		`"header":{`,
		`"titleKey":"name"`,
		`"descriptionKey":"email"`,
		`"badge":{`,
		`"content":{`,
		`"textKey":"bio"`,
		`"link":{"hrefKey":"website"}`,
		`"text":"Free"`,
		`"footer":{`,
		`"text":"Balances update hourly."`,
		`"confirm":"Really delete?"`,
		`"empty":{"title":"No user","body":"Nothing selected."}`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
}

func TestCardListConfigIsland(t *testing.T) {
	b, err := json.Marshal(canonicalCardList().config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{
		`"widget":"cardlist"`,
		`"pageSize":10`,
		`"filterable":true`,
		`"defaultSort":{"key":"balance","desc":true}`,
		`"sort":[{"key":"balance","label":"Balance"},{"key":"createdAt","label":"Created"}]`,
		`"args":{"ids":{"selection":"id"}}`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
}

func TestCardListConfigLoadTool(t *testing.T) {
	l := canonicalCardList()
	l.LoadTool = "list_users"
	l.LoadArgs = map[string]any{"scope": "all"}
	b, _ := json.Marshal(l.config())
	cfg := string(b)
	for _, want := range []string{`"loadTool":"list_users"`, `"loadArgs":{"scope":"all"}`} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
	b2, _ := json.Marshal(canonicalCardList().config())
	if strings.Contains(string(b2), "loadTool") {
		t.Errorf("load keys present without LoadTool set: %s", b2)
	}
}

func TestCardToolMetaAndDescriptor(t *testing.T) {
	c := canonicalCard()
	meta, _ := json.Marshal(c.ToolMeta())
	if string(meta) != `{"ui":{"resourceUri":"ui://demo/user"}}` {
		t.Errorf("ToolMeta = %s", meta)
	}
	d := c.Descriptor()
	if d.URI != "ui://demo/user" || d.Name != "demo-user" || d.MIMEType != "text/html;profile=mcp-app" {
		t.Errorf("Descriptor = %+v", d)
	}
}

func TestCardValidate(t *testing.T) {
	cases := map[string]func(*Card){
		"bad URI scheme": func(c *Card) { c.URI = "https://x" },
		"no title key":   func(c *Card) { c.Template.Header.TitleKey = "" },
		"description key and text": func(c *Card) {
			c.Template.Header.Description = "Fixed"
		},
		"item no key":     func(c *Card) { c.Template.Content.Items.Items = []DescriptionItem{{Label: "X"}} },
		"item no label":   func(c *Card) { c.Template.Content.Items.Items = []DescriptionItem{{Key: "x"}} },
		"link no hrefKey": func(c *Card) { c.Template.Content.Items.Items = []DescriptionItem{{Label: "X", Type: ColLink}} },
		"non-badge badge": func(c *Card) { c.Template.Header.Badge = Text("status", "Status") },
		"badge and header action": func(c *Card) {
			c.Template.Header.Action = &Action{Label: "Open", Tool: "open_user"}
		},
		"duplicate item key": func(c *Card) {
			c.Template.Content.Items.Items = append(c.Template.Content.Items.Items,
				DescriptionItem{Label: "Balance again", Key: "balance", Type: ColNumber})
		},
		"content text key and text": func(c *Card) { c.Template.Content.Text = "Fixed" },
		"footer text key and text":  func(c *Card) { c.Template.Footer.TextKey = "note" },
		"selection arg in header action": func(c *Card) {
			c.Template.Header.Badge = Column{}
			c.Template.Header.Action = &Action{Label: "X", Tool: "x", Args: map[string]ArgSource{"ids": FromSelection("id")}}
		},
		"selection arg in footer action": func(c *Card) {
			c.Template.Footer.Actions = []Action{{Label: "X", Tool: "x", Args: map[string]ArgSource{"ids": FromSelection("id")}}}
		},
		"action no label": func(c *Card) { c.Template.Footer.Actions = []Action{{Tool: "x"}} },
		"unsafe theme":    func(c *Card) { c.Theme = &theme.Theme{ColorText: "red}</style>"} },
	}
	if err := canonicalCard().Validate(); err != nil {
		t.Fatalf("canonical card must validate, got: %v", err)
	}
	for name, mutate := range cases {
		c := canonicalCard()
		mutate(c)
		if err := c.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		}
	}
}

func TestCardListValidate(t *testing.T) {
	cases := map[string]func(*CardList){
		"bad URI scheme":        func(l *CardList) { l.URI = "https://x" },
		"no title key":          func(l *CardList) { l.Template.Header.TitleKey = "" },
		"negative page size":    func(l *CardList) { l.PageSize = -1 },
		"page size option zero": func(l *CardList) { l.PageSizes = []int{10, 0} },
		"page sizes unpaginated": func(l *CardList) {
			l.PageSize, l.PageSizes = 0, []int{10}
		},
		"load more unpaginated": func(l *CardList) {
			l.PageSize, l.PageSizes, l.LoadMore = 0, nil, true
		},
		"load more with page sizes": func(l *CardList) {
			l.PageSize, l.PageSizes, l.LoadMore = 5, []int{5, 10}, true
		},
		"default sort no key":  func(l *CardList) { l.DefaultSort = &SortSpec{} },
		"bulk action no label": func(l *CardList) { l.Selection = &SelectionConfig{Bulk: []Action{{Tool: "x"}}} },
	}
	if err := canonicalCardList().Validate(); err != nil {
		t.Fatalf("canonical card list must validate, got: %v", err)
	}
	for name, mutate := range cases {
		l := canonicalCardList()
		mutate(l)
		if err := l.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		}
	}
}
