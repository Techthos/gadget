package main

import (
	"fmt"

	"github.com/techthos/gadget"
	"github.com/techthos/gadget/theme"
)

// story is one entry in the harness catalog: a named widget configuration the
// host page loads into its iframe. The page reads the catalog from
// /stories.json and renders it as a rail grouped by Group.
type story struct {
	ID    string `json:"id"`
	Group string `json:"group"`
	Label string `json:"label"`
	Desc  string `json:"desc"`
	Src   string `json:"src"`
	// Payload seeds the push panel with a structuredContent example that
	// makes sense for this story.
	Payload string `json:"payload"`

	build func() gadget.Widget
}

// Push-panel presets. Kept as literal JSON so the textarea shows it the way a
// tool author would write it.
const (
	pushRows = `{
  "rows": [
    {
      "id": 9, "name": "Pushed Row", "email": "pushed@example.com",
      "balance": 42.5, "createdAt": "2026-07-23T00:00:00Z",
      "status": "invited", "website": "https://example.com/pushed"
    }
  ]
}`
	pushEmptyRows = `{ "rows": [] }`
	pushErrors    = `{
  "errors": { "email": "This email is already taken." }
}`
	pushValues = `{
  "values": {
    "name": "Grace Hopper", "email": "grace@example.com",
    "role": "admin", "notify": false
  }
}`
)

// catalog returns every story, in rail order. Src is filled in by stories().
func catalog() []story {
	return []story{
		{
			ID: "table-default", Group: "Table", Label: "Full featured",
			Desc:    "Filter, pagination, selection with a bulk action, and a per-row delete behind a confirm.",
			Payload: pushRows, build: func() gadget.Widget { return table() },
		},
		{
			ID: "table-plain", Group: "Table", Label: "Read only",
			Desc:    "No filter, pagination, selection or actions — just typed columns.",
			Payload: pushRows, build: func() gadget.Widget { return tablePlain() },
		},
		{
			ID: "table-long", Group: "Table", Label: "Long list",
			Desc:    "24 records, page size 8, pre-sorted by balance.",
			Payload: pushRows, build: func() gadget.Widget { return tableLong() },
		},
		{
			ID: "table-empty", Group: "Table", Label: "Empty state",
			Desc:    "Ships with no snapshot; push a result to fill it.",
			Payload: pushRows, build: func() gadget.Widget { return tableEmpty() },
		},
		{
			ID: "cards-default", Group: "CardList", Label: "Carousel",
			Desc:    "Paged card carousel with filter, sort and bulk archive.",
			Payload: pushRows, build: func() gadget.Widget { return cardList() },
		},
		{
			ID: "cards-empty", Group: "CardList", Label: "Empty state",
			Desc:    "No records baked in — exercises the empty message.",
			Payload: pushRows, build: func() gadget.Widget { return cardListEmpty() },
		},
		{
			ID: "card-default", Group: "Card", Label: "Single record",
			Desc:    "One record rendered from rows[0], with a footer action.",
			Payload: pushRows, build: func() gadget.Widget { return card() },
		},
		{
			ID: "card-empty", Group: "Card", Label: "Empty state",
			Desc:    "Waiting for a record; push a result to load one.",
			Payload: pushRows, build: func() gadget.Widget { return cardEmpty() },
		},
		{
			ID: "form-edit", Group: "Form", Label: "Edit record",
			Desc:    "Prefilled from a baked snapshot. Tick the field-error switch, then submit.",
			Payload: pushValues, build: func() gadget.Widget { return form() },
		},
		{
			ID: "form-create", Group: "Form", Label: "All field types",
			Desc:    "Text, textarea, number, date, time, select, multiselect, checkbox, readonly.",
			Payload: pushErrors, build: func() gadget.Widget { return formCreate() },
		},
		{
			ID: "menu-default", Group: "Menu", Label: "Launcher",
			Desc:    "Tiles with icons and badges; each tile fires a tools/call.",
			Payload: pushEmptyRows, build: func() gadget.Widget { return menu() },
		},
		{
			ID: "menu-plain", Group: "Menu", Label: "Plain tiles",
			Desc:    "No icons, badges or descriptions — the minimum a Menu needs.",
			Payload: pushEmptyRows, build: func() gadget.Widget { return menuPlain() },
		},
	}
}

// stories returns the catalog with Src derived from each ID.
func stories() []story {
	list := catalog()
	for i := range list {
		list[i].Src = "/story/" + list[i].ID
	}
	return list
}

// --- shared building blocks ---

// harnessRows is a small record set with the extra fields (email, website)
// the card widgets display; reused across stories.
func harnessRows() []map[string]any {
	return []map[string]any{
		{"id": 1, "name": "Ada Lovelace", "email": "ada@example.com", "balance": 1200.5, "createdAt": "2026-01-12T09:00:00Z", "status": "active", "website": "https://example.com/ada"},
		{"id": 2, "name": "Grace Hopper", "email": "grace@example.com", "balance": 815, "createdAt": "2026-02-03T10:30:00Z", "status": "active", "website": "https://example.com/grace"},
		{"id": 3, "name": "Alan Turing", "email": "alan@example.com", "balance": 0, "createdAt": "2026-03-19T14:00:00Z", "status": "invited", "website": ""},
		{"id": 4, "name": "Katherine Johnson", "email": "katherine@example.com", "balance": 233.1, "createdAt": "2026-04-01T08:15:00Z", "status": "active", "website": "https://example.com/katherine"},
	}
}

// manyRows synthesizes n records for the long-list story.
func manyRows(n int) []map[string]any {
	given := []string{"Ada", "Grace", "Alan", "Katherine", "Barbara", "Edsger", "Margaret", "Donald"}
	family := []string{"Lovelace", "Hopper", "Turing", "Johnson", "Liskov", "Dijkstra", "Hamilton", "Knuth"}
	status := []string{"active", "invited", "archived"}

	rows := make([]map[string]any, 0, n)
	for i := range n {
		name := fmt.Sprintf("%s %s", given[i%len(given)], family[(i/len(given)+i)%len(family)])
		rows = append(rows, map[string]any{
			"id":        i + 1,
			"name":      name,
			"email":     fmt.Sprintf("user%02d@example.com", i+1),
			"balance":   float64((i*317)%2400) + 0.5,
			"createdAt": fmt.Sprintf("2026-%02d-%02dT09:00:00Z", i%12+1, i%27+1),
			"status":    status[i%len(status)],
			"website":   fmt.Sprintf("https://example.com/user%02d", i+1),
		})
	}
	return rows
}

func statusBadge() gadget.Column {
	return gadget.Badge("status", "Status", map[string]gadget.BadgeVariant{
		"active": gadget.BadgeSuccess, "invited": gadget.BadgeInfo, "archived": gadget.BadgeNeutral,
	})
}

func deleteAction() gadget.Action {
	return gadget.Action{Label: "Delete", Tool: "delete_user", Variant: gadget.VariantDanger,
		Confirm: "Really?", Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}}
}

func archiveBulk() *gadget.SelectionConfig {
	return &gadget.SelectionConfig{Bulk: []gadget.Action{
		{Label: "Archive", Tool: "archive_users", Args: map[string]gadget.ArgSource{"ids": gadget.FromSelection("id")}},
	}}
}

// demoBrand exercises the inline-SVG logo path.
func demoBrand() *gadget.Brand {
	return &gadget.Brand{
		Name:    "Acme",
		URL:     "https://example.com",
		LogoSVG: `<svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><circle cx="8" cy="8" r="7"/></svg>`,
	}
}

func demoTheme() *theme.Theme { return &theme.Theme{ColorPrimary: "#7c3aed"} }

func num(v float64) *float64 { return &v }

// --- Table stories ---

func table() *gadget.Table {
	return &gadget.Table{
		URI:   "ui://harness/table",
		Title: "Users",
		Columns: []gadget.Column{
			gadget.Text("name", "Name"),
			gadget.Number("balance", "Balance", "currency:EUR"),
			gadget.Date("createdAt", "Created", "date"),
			statusBadge(),
			gadget.ActionsColumn(deleteAction()),
		},
		PageSize:    3,
		PageSizes:   []int{3, 5, 10},
		Filterable:  true,
		Selection:   archiveBulk(),
		InitialData: map[string]any{"rows": harnessRows()},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

func tablePlain() *gadget.Table {
	return &gadget.Table{
		URI:   "ui://harness/table-plain",
		Title: "Users",
		Columns: []gadget.Column{
			gadget.Text("name", "Name"),
			gadget.Text("email", "Email"),
			gadget.Number("balance", "Balance", "currency:EUR"),
			gadget.Date("createdAt", "Created", "date"),
		},
		InitialData: map[string]any{"rows": harnessRows()},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

func tableLong() *gadget.Table {
	return &gadget.Table{
		URI:   "ui://harness/table-long",
		Title: "Directory",
		Columns: []gadget.Column{
			gadget.Text("name", "Name"),
			gadget.Text("email", "Email"),
			gadget.Number("balance", "Balance", "currency:EUR"),
			gadget.Date("createdAt", "Created", "date"),
			statusBadge(),
			gadget.ActionsColumn(deleteAction()),
		},
		PageSize:    8,
		PageSizes:   []int{8, 16, 24},
		DefaultSort: &gadget.SortSpec{Key: "balance", Desc: true},
		Filterable:  true,
		Selection:   archiveBulk(),
		InitialData: map[string]any{"rows": manyRows(24)},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

func tableEmpty() *gadget.Table {
	return &gadget.Table{
		URI:   "ui://harness/table-empty",
		Title: "Users",
		Columns: []gadget.Column{
			gadget.Text("name", "Name"),
			gadget.Number("balance", "Balance", "currency:EUR"),
			gadget.Date("createdAt", "Created", "date"),
			statusBadge(),
		},
		Filterable:  true,
		Empty:       gadget.EmptyState{Title: "No users yet", Body: "Push a tool-result from the panel to fill the table."},
		InitialData: map[string]any{"rows": []map[string]any{}},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// --- Card stories ---

// cardTemplate is shared by the single Card and the CardList.
func cardTemplate() gadget.CardTemplate {
	return gadget.CardTemplate{
		TitleKey:    "name",
		SubtitleKey: "email",
		Badge:       statusBadge(),
		Fields: []gadget.Column{
			gadget.Number("balance", "Balance", "currency:EUR"),
			gadget.Date("createdAt", "Joined", "relative"),
			gadget.Link("website", "Website"),
		},
		Actions: []gadget.Action{deleteAction()},
	}
}

func cardList() *gadget.CardList {
	return &gadget.CardList{
		URI:         "ui://harness/cards",
		Title:       "Users",
		Template:    cardTemplate(),
		PageSize:    3,
		PageSizes:   []int{3, 6, 12},
		DefaultSort: &gadget.SortSpec{Key: "balance", Desc: true},
		Filterable:  true,
		Selection:   archiveBulk(),
		InitialData: map[string]any{"rows": harnessRows()},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

func cardListEmpty() *gadget.CardList {
	l := cardList()
	l.URI = "ui://harness/cards-empty"
	l.Empty = gadget.EmptyState{Title: "Nothing to show", Body: "Push a tool-result with rows to populate the list."}
	l.InitialData = map[string]any{"rows": []map[string]any{}}
	return l
}

func card() *gadget.Card {
	return &gadget.Card{
		URI:         "ui://harness/card",
		Title:       "User",
		Template:    cardTemplate(),
		Empty:       gadget.EmptyState{Title: "No user", Body: "Push a tool-result to load one."},
		InitialData: map[string]any{"rows": harnessRows()[:1]},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

func cardEmpty() *gadget.Card {
	c := card()
	c.URI = "ui://harness/card-empty"
	c.InitialData = map[string]any{"rows": []map[string]any{}}
	return c
}

// --- Form stories ---

func form() *gadget.Form {
	return &gadget.Form{
		URI:   "ui://harness/form",
		Title: "Edit user",
		Fields: []gadget.Field{
			{Name: "id", Type: gadget.FHidden, Default: "1"},
			{Name: "name", Label: "Name", Required: true},
			{Name: "email", Label: "Email", Required: true,
				Validation: &gadget.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
			{Name: "role", Label: "Role", Type: gadget.FSelect, Options: []gadget.Option{gadget.Opt("user"), gadget.Opt("admin")}},
			{Name: "notify", Label: "Send notifications", Type: gadget.FCheckbox, Default: true},
		},
		Submit:      gadget.SubmitSpec{Tool: "save_user", SuccessMessage: "Saved."},
		Cancel:      &gadget.CancelSpec{},
		InitialData: map[string]any{"values": map[string]any{"name": "Ada Lovelace", "email": "ada@example.com"}},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

func formCreate() *gadget.Form {
	return &gadget.Form{
		URI:   "ui://harness/form-create",
		Title: "New user",
		Fields: []gadget.Field{
			{Name: "account", Label: "Account", Type: gadget.FReadonly, Default: "acme-eu"},
			{Name: "name", Label: "Name", Required: true, Placeholder: "Ada Lovelace",
				Description: "Shown everywhere the record appears."},
			{Name: "email", Label: "Email", Required: true, Placeholder: "ada@example.com",
				Validation: &gadget.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
			{Name: "role", Label: "Role", Type: gadget.FSelect, Default: "user",
				Options: []gadget.Option{gadget.Opt("user"), gadget.Opt("admin"), gadget.Opt("auditor")}},
			{Name: "scopes", Label: "Scopes", Type: gadget.FMultiSelect,
				Options: []gadget.Option{gadget.Opt("read"), gadget.Opt("write"), gadget.Opt("billing")},
				Default: []string{"read"}},
			{Name: "seats", Label: "Seats", Type: gadget.FNumber, Default: "3",
				Validation: &gadget.Validation{Min: num(1), Max: num(50), Step: num(1)}},
			{Name: "startsOn", Label: "Starts on", Type: gadget.FDate, Default: "2026-08-01"},
			{Name: "digestAt", Label: "Daily digest", Type: gadget.FTime, Default: "09:00"},
			{Name: "notes", Label: "Notes", Type: gadget.FTextarea, Rows: 3,
				Placeholder: "Anything the team should know"},
			{Name: "notify", Label: "Send notifications", Type: gadget.FCheckbox, Default: true},
		},
		Submit:      gadget.SubmitSpec{Tool: "save_user", Label: "Create user", SuccessMessage: "User created."},
		Cancel:      &gadget.CancelSpec{},
		InitialData: map[string]any{},
		Brand:       demoBrand(),
		Theme:       demoTheme(),
	}
}

// --- Menu stories ---

func menu() *gadget.Menu {
	icon := `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 10h18M9 10v10"/></svg>`
	pen := `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M12 20h9"/><path d="M16.5 3.5a2.1 2.1 0 0 1 3 3L7 19l-4 1 1-4Z"/></svg>`
	box := `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M21 8v13H3V8"/><path d="M1 3h22v5H1zM10 12h4"/></svg>`
	return &gadget.Menu{
		URI:   "ui://harness/menu",
		Title: "Harness app",
		Intro: "Each tile fires a tools/call; watch the traffic pane.",
		Items: []gadget.MenuItem{
			{
				Tool: "list_users", Label: "User table",
				Description:  "Sortable, filterable directory.",
				IconSVG:      icon,
				Badge:        "read",
				BadgeVariant: gadget.BadgeInfo,
			},
			{
				Tool: "edit_user", Args: map[string]any{"id": 1},
				Label:        "Edit Ada",
				Description:  "Open the edit form for one user.",
				IconSVG:      pen,
				Badge:        "write",
				BadgeVariant: gadget.BadgeWarning,
			},
			{
				Tool: "archive_users", Label: "Archive all",
				Description:  "Bulk-archive every user.",
				IconSVG:      box,
				Badge:        "danger",
				BadgeVariant: gadget.BadgeDanger,
			},
		},
		Brand: demoBrand(),
		Theme: demoTheme(),
	}
}

func menuPlain() *gadget.Menu {
	return &gadget.Menu{
		URI:   "ui://harness/menu-plain",
		Title: "Harness app",
		Items: []gadget.MenuItem{
			{Tool: "list_users", Label: "Users"},
			{Tool: "save_user", Label: "New user"},
			// No label: the tile falls back to the tool name.
			{Tool: "archive_users"},
		},
		Brand: demoBrand(),
	}
}
