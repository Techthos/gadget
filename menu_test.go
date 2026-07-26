package gomukit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"

	"github.com/techthos/gomukit/theme"
)

const menuIconSVG = `<svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><rect x="2" y="3" width="12" height="10" rx="2"/></svg>`

// canonicalMenu exercises every menu item slot.
func canonicalMenu() *Menu {
	return &Menu{
		URI:   "ui://demo/menu",
		Title: "Demo app",
		Intro: "Pick where to start.",
		Items: []MenuItem{
			{
				Tool:         "list_users",
				Label:        "Users",
				Description:  "Browse the user directory.",
				IconSVG:      menuIconSVG,
				Badge:        "read",
				BadgeVariant: BadgeInfo,
			},
			{
				Tool:        "edit_user",
				Args:        map[string]any{"id": 1},
				Label:       "Edit Ada",
				Description: "Open the edit form for one user.",
			},
			// Neither a label nor decoration: the tool name carries the tile.
			{Tool: "archive_users"},
			// A prompt item navigates through the host's chat. Args are set
			// too, to pin that the island drops them on this path.
			{
				Tool:   "invite_user",
				Args:   map[string]any{"id": 9},
				Prompt: "Start an invite for a new teammate",
				Label:  "Invite",
			},
		},
		Brand: &Brand{Name: "Acme"},
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func TestMenuGolden(t *testing.T) {
	doc, err := canonicalMenu().Document()
	if err != nil {
		t.Fatal(err)
	}
	golden := filepath.Join("testdata", "golden", "menu.html")
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
		t.Error("document does not match golden file; run `go test -run TestMenuGolden -update ./...` and review the diff")
	}
	if _, err := xhtml.Parse(strings.NewReader(doc)); err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	for _, want := range []string{
		`data-gomu-widget="menu"`,
		`id="gomu-config"`,
		`class="gomu-menu-intro">Pick where to start.<`,
		`data-gomu-menu-item="0"`,
		`data-gomu-menu-item="1"`,
		`data-gomu-menu-item="2"`,
		`class="gomu-menu-label">Users<`,
		`class="gomu-menu-desc">Browse the user directory.<`,
		`gomu-badge--info">read<`,
		`<rect x="2" y="3" width="12" height="10" rx="2"/>`,
		`--gomu-color-primary:#7c3aed`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q", want)
		}
	}
	// A menu is authored, not fetched: no data island, no rows contract.
	if strings.Contains(doc, `id="gomu-data"`) {
		t.Error("menu document should carry no data island")
	}
}

// An item without a label falls back to the tool name so a tile is never blank.
func TestMenuItemLabelFallsBackToTool(t *testing.T) {
	doc, err := canonicalMenu().Document()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc, `class="gomu-menu-label">archive_users<`) {
		t.Error("unlabelled item should show its tool name")
	}
}

func TestMenuConfigIsland(t *testing.T) {
	b, err := json.Marshal(canonicalMenu().config())
	if err != nil {
		t.Fatal(err)
	}
	cfg := string(b)
	for _, want := range []string{
		`"widget":"menu"`,
		`{"tool":"list_users"}`,
		`{"args":{"id":1},"tool":"edit_user"}`,
		`{"tool":"archive_users"}`,
		`{"prompt":"Start an invite for a new teammate","tool":"invite_user"}`,
	} {
		if !strings.Contains(cfg, want) {
			t.Errorf("config island missing %s\nfull: %s", want, cfg)
		}
	}
	// A prompt item never calls the tool from the view, so its args are dead
	// weight and must not reach the island.
	if strings.Contains(cfg, `"id":9`) {
		t.Errorf("config island should drop args of a prompt item\nfull: %s", cfg)
	}
	// Labels, descriptions and icons are server-rendered; the runtime never
	// needs them, so they stay out of the island.
	for _, unwanted := range []string{"Users", "label", "icon", "description"} {
		if strings.Contains(cfg, unwanted) {
			t.Errorf("config island should not carry %q\nfull: %s", unwanted, cfg)
		}
	}
}

// Tile order and config order are matched positionally by the runtime.
func TestMenuConfigOrderMatchesTiles(t *testing.T) {
	m := canonicalMenu()
	doc, err := m.Document()
	if err != nil {
		t.Fatal(err)
	}
	prev := -1
	for i := range m.Items {
		at := strings.Index(doc, `data-gomu-menu-item="`+strconv.Itoa(i)+`"`)
		if at < 0 {
			t.Fatalf("tile %d missing", i)
		}
		if at <= prev {
			t.Errorf("tile %d is out of document order", i)
		}
		prev = at
	}
}

func TestMenuToolMetaAndDescriptor(t *testing.T) {
	m := canonicalMenu()
	meta, _ := json.Marshal(m.ToolMeta())
	if string(meta) != `{"ui":{"resourceUri":"ui://demo/menu"}}` {
		t.Errorf("ToolMeta = %s", meta)
	}
	d := m.Descriptor()
	if d.URI != "ui://demo/menu" || d.Name != "demo-menu" || d.MIMEType != "text/html;profile=mcp-app" {
		t.Errorf("Descriptor = %+v", d)
	}
}

func TestMenuValidate(t *testing.T) {
	cases := map[string]func(*Menu){
		"bad URI scheme": func(m *Menu) { m.URI = "https://x" },
		"no items":       func(m *Menu) { m.Items = nil },
		"item no tool":   func(m *Menu) { m.Items = []MenuItem{{Label: "Users"}} },
		"icon with script": func(m *Menu) {
			m.Items = []MenuItem{{Tool: "x", IconSVG: `<svg><script>alert(1)</script></svg>`}}
		},
		"icon with handler": func(m *Menu) {
			m.Items = []MenuItem{{Tool: "x", IconSVG: `<svg onload="alert(1)"><rect/></svg>`}}
		},
		"icon fragment": func(m *Menu) { m.Items = []MenuItem{{Tool: "x", IconSVG: `<rect/>`}} },
		"unknown badge variant": func(m *Menu) {
			m.Items = []MenuItem{{Tool: "x", Badge: "b", BadgeVariant: "chartreuse"}}
		},
		"unsafe brand": func(m *Menu) { m.Brand = &Brand{Name: "A", URL: "javascript:alert(1)"} },
		"unsafe theme": func(m *Menu) { m.Theme = &theme.Theme{ColorText: "red}</style>"} },
	}
	if err := canonicalMenu().Validate(); err != nil {
		t.Fatalf("canonical menu must validate, got: %v", err)
	}
	for name, mutate := range cases {
		m := canonicalMenu()
		mutate(m)
		if err := m.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		}
	}
}

func TestMenuInvalidFailsDocument(t *testing.T) {
	m := canonicalMenu()
	m.Items[0].IconSVG = `<svg onclick="x()"></svg>`
	if _, err := m.Document(); err == nil {
		t.Fatal("expected Document to reject an unsafe item icon")
	}
}
