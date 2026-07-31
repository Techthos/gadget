// Command demo is a runnable MCP server showcasing gomukit widgets: a user
// table with row/bulk actions, the same users as a card grid, an edit form
// with server-side validation, the whole record as a grouped profile form
// laid out in columns, a confirmation, and a date picker whose selectable
// window is computed per call.
//
// Run with streamable HTTP (default, for MCPJam / Claude custom connectors):
//
//	go run ./examples/demo -addr :8080
//
// or over stdio (for hosts that spawn the server):
//
//	go run ./examples/demo -stdio
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/techthos/gomukit"
	"github.com/techthos/gomukit/gosdk"
	"github.com/techthos/gomukit/theme"
)

// --- in-memory data ---

type user struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Status  string  `json:"status"`
	Balance float64 `json:"balance"`
	Created string  `json:"createdAt"`
	// FollowUp is the day the account manager calls back, "YYYY-MM-DD", set
	// by the date picker. Empty until someone picks one.
	FollowUp string `json:"followUpAt"`

	// The rest of the account, edited through the grouped profile form. The
	// short edit form never touches these, and the table does not show them —
	// they are what a form with field sets is for: more of a record than one
	// column of controls can hold.
	Phone    string `json:"phone"`
	Company  string `json:"company"`
	Plan     string `json:"plan"`
	Seats    int    `json:"seats"`
	StartsOn string `json:"startsOn"` // "YYYY-MM-DD", contract start
	EndsOn   string `json:"endsOn"`   // "YYYY-MM-DD", empty = permanent
	Notes    string `json:"notes"`
	Announce bool   `json:"announce"` // tell the team channel about changes
}

type db struct {
	sync.Mutex
	users  map[int]*user
	nextID int
}

func seed() *db {
	users := []*user{
		{ID: 1, Name: "Ada Lovelace", Email: "ada@example.com", Status: "active", Balance: 1200.50,
			Created: "2026-01-12T09:00:00Z", Phone: "+30 210 0000001", Company: "Analytical Engines",
			Plan: "enterprise", Seats: 24, StartsOn: "2026-01-12", Announce: true},
		{ID: 2, Name: "Grace Hopper", Email: "grace@example.com", Status: "active", Balance: 815.00,
			Created: "2026-02-03T10:30:00Z", Phone: "+30 210 0000002", Company: "Compiler Works",
			Plan: "team", Seats: 8, StartsOn: "2026-02-03", EndsOn: "2027-02-02"},
		{ID: 3, Name: "Alan Turing", Email: "alan@example.com", Status: "invited",
			Created: "2026-03-19T14:00:00Z", Company: "Bletchley Ltd", Plan: "starter", Seats: 1},
		{ID: 4, Name: "Katherine Johnson", Email: "katherine@example.com", Status: "archived", Balance: 233.10,
			Created: "2026-04-01T08:15:00Z", Company: "Orbital Maths", Plan: "team", Seats: 5,
			StartsOn: "2026-04-01", EndsOn: "2026-07-01", Notes: "Archived after the pilot."},
	}
	m := map[int]*user{}
	for _, u := range users {
		m[u.ID] = u
	}
	return &db{users: m, nextID: 5}
}

func (d *db) rows() []map[string]any {
	d.Lock()
	defer d.Unlock()
	list := make([]*user, 0, len(d.users))
	for _, u := range d.users {
		list = append(list, u)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	rows, _ := gomukit.RowsOf(list)
	return rows
}

// bookedDays are the days that already hold a call, in the "YYYY-MM-DD" form
// the calendar blocks by. Call with the lock held.
func bookedDays(d *db) []string {
	var days []string
	for _, u := range d.users {
		if u.FollowUp != "" {
			days = append(days, u.FollowUp)
		}
	}
	sort.Strings(days)
	return days
}

// userRow is one user in the row shape every widget reads (honors json tags).
func userRow(u *user) map[string]any {
	rows, _ := gomukit.RowsOf([]*user{u})
	if len(rows) == 0 {
		return map[string]any{}
	}
	return rows[0]
}

// --- widgets ---

func usersTable() *gomukit.Table {
	return &gomukit.Table{
		URI:   "ui://demo/users",
		Title: "Users",
		Columns: []gomukit.Column{
			gomukit.Text("name", "Name"),
			gomukit.Text("email", "Email"),
			gomukit.Number("balance", "Balance", "currency:EUR"),
			gomukit.Date("createdAt", "Created", "date"),
			// Written by the date picker, so the two widgets are visibly the
			// same record.
			gomukit.Date("followUpAt", "Follow-up", "date"),
			gomukit.Badge("status", "Status", map[string]gomukit.BadgeVariant{
				"active":   gomukit.BadgeSuccess,
				"invited":  gomukit.BadgeInfo,
				"archived": gomukit.BadgeNeutral,
			}),
			gomukit.ActionsColumn(
				gomukit.Action{
					Label: "Delete", Tool: "delete_user", Variant: gomukit.VariantDanger,
					Confirm: "Really delete?",
					Args:    map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
				},
			),
		},
		PageSize:    10,
		PageSizes:   []int{10, 25, 50},
		DefaultSort: &gomukit.SortSpec{Key: "name"},
		Filterable:  true,
		Selection: &gomukit.SelectionConfig{Bulk: []gomukit.Action{
			{
				Label: "Archive", Tool: "archive_users",
				Args: map[string]gomukit.ArgSource{"ids": gomukit.FromSelection("id")},
			},
		}},
		Empty: gomukit.EmptyState{Title: "No users", Body: "Ask the assistant to create one."},
		Brand: demoBrand(),
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func usersCards() *gomukit.CardList {
	return &gomukit.CardList{
		URI:   "ui://demo/user-cards",
		Title: "Users",
		Template: gomukit.CardTemplate{
			Header: gomukit.CardHeader{
				TitleKey:       "name",
				DescriptionKey: "email",
				Badge: gomukit.Badge("status", "Status", map[string]gomukit.BadgeVariant{
					"active":   gomukit.BadgeSuccess,
					"invited":  gomukit.BadgeInfo,
					"archived": gomukit.BadgeNeutral,
				}),
			},
			Content: gomukit.CardContent{
				Items: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
					{Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
					{Label: "Joined", Key: "createdAt", Type: gomukit.ColDate, Format: "relative"},
				}},
			},
			Footer: gomukit.CardFooter{Actions: []gomukit.Action{
				{
					Label: "Delete", Tool: "delete_user", Variant: gomukit.VariantDanger,
					Confirm: "Really delete?",
					Args:    map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
				},
			}},
		},
		PageSize:    12,
		PageSizes:   []int{12, 24, 48},
		DefaultSort: &gomukit.SortSpec{Key: "balance", Desc: true},
		Filterable:  true,
		Selection: &gomukit.SelectionConfig{Bulk: []gomukit.Action{
			{
				Label: "Archive", Tool: "archive_users",
				Args: map[string]gomukit.ArgSource{"ids": gomukit.FromSelection("id")},
			},
		}},
		Empty: gomukit.EmptyState{Title: "No users", Body: "Ask the assistant to create one."},
		Brand: demoBrand(),
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func userForm() *gomukit.Form {
	return &gomukit.Form{
		URI:   "ui://demo/user-form",
		Title: "Edit user",
		Fields: []gomukit.Field{
			{Name: "id", Type: gomukit.FHidden},
			{Name: "name", Label: "Name", Required: true},
			{Name: "email", Label: "Email", Required: true,
				Validation: &gomukit.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
			{Name: "status", Label: "Status", Type: gomukit.FSelect, Required: true,
				Options: []gomukit.Option{gomukit.Opt("active"), gomukit.Opt("invited"), gomukit.Opt("archived")}},
			{Name: "balance", Label: "Balance (EUR)", Type: gomukit.FNumber,
				Validation: &gomukit.Validation{Min: ptr(0.0), Step: ptr(0.01)}},
		},
		Submit: gomukit.SubmitSpec{Tool: "save_user", Label: "Save", SuccessMessage: "User saved."},
		Cancel: &gomukit.CancelSpec{},
		Brand:  demoBrand(),
		Theme:  &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

// profileForm is the whole account rather than the five fields the edit form
// touches: too much for one column of controls, so it is read as four groups
// laid out two fields to a row. Everything else — validation, prefill,
// submission — works exactly as it does in userForm; grouping is layout.
func profileForm() *gomukit.Form {
	return &gomukit.Form{
		URI:     "ui://demo/user-profile",
		Title:   "Customer profile",
		Columns: 2,
		Fields: []gomukit.Field{
			{Name: "id", Type: gomukit.FHidden},
			// Ungrouped fields sit above the first group. Span 2: the account
			// this record belongs to is a statement about all four groups.
			{Name: "company", Label: "Company", Required: true, Span: 2,
				Description: "The account every seat below is billed to."},
		},
		FieldSets: []gomukit.FieldSet{
			{
				Title:       "Contact",
				Description: "Who we talk to about this account.",
				Fields: []gomukit.Field{
					{Name: "name", Label: "Name", Required: true},
					{Name: "phone", Label: "Phone", Placeholder: "+30 …"},
					{Name: "email", Label: "Email", Required: true, Span: 2,
						Validation: &gomukit.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
				},
			},
			{
				Title:       "Subscription",
				Description: "The plan decides the seat floor; the server checks it.",
				Boxed:       true,
				Fields: []gomukit.Field{
					{Name: "plan", Label: "Plan", Type: gomukit.FSelect, Required: true,
						Options: []gomukit.Option{gomukit.Opt("starter"), gomukit.Opt("team"), gomukit.Opt("enterprise")}},
					{Name: "seats", Label: "Seats", Type: gomukit.FNumber, Required: true,
						Validation: &gomukit.Validation{Min: ptr(1.0), Max: ptr(500.0), Step: ptr(1.0)}},
					{Name: "status", Label: "Status", Type: gomukit.FSelect, Required: true,
						Options: []gomukit.Option{gomukit.Opt("active"), gomukit.Opt("invited"), gomukit.Opt("archived")}},
					{Name: "balance", Label: "Balance (EUR)", Type: gomukit.FNumber,
						Validation: &gomukit.Validation{Min: ptr(0.0), Step: ptr(0.01)}},
					{Name: "startsOn", Label: "Contract period", Type: gomukit.FDateRange, EndName: "endsOn", Span: 2,
						Description: "Leave the end open for a rolling contract.",
						Calendar: &gomukit.Calendar{
							Presets: []gomukit.DatePreset{
								{Label: "Next 30 days", Span: gomukit.SpanNext30Days},
								{Label: "This month", Span: gomukit.SpanThisMonth},
							},
						}},
				},
			},
			{
				// One column, whatever the form says: neither of these two reads
				// well beside something else.
				Title:   "Internal",
				Columns: 1,
				Fields: []gomukit.Field{
					{Name: "notes", Label: "Account notes", Type: gomukit.FTextarea, Rows: 3,
						Placeholder: "Anything the next person on this account should know"},
					{Name: "announce", Label: "Announce changes in the team channel", Type: gomukit.FCheckbox},
				},
			},
		},
		Submit: gomukit.SubmitSpec{Tool: "save_profile", Label: "Save profile", SuccessMessage: "Profile saved."},
		Cancel: &gomukit.CancelSpec{},
		Brand:  demoBrand(),
		Theme:  &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

// deleteConfirm asks before a deletion runs. The record and the consequences
// are per call, so both arrive from confirm_delete_user's result: the user
// under "rows", what removing them costs under "effects".
func deleteConfirm() *gomukit.Confirm {
	return &gomukit.Confirm{
		URI:      "ui://demo/confirm-delete",
		Title:    "Delete user",
		Prompt:   "Delete this user?",
		Body:     "The account and everything attached to it is removed for good.",
		Severity: gomukit.BadgeDanger,
		Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
			{Label: "User", Key: "name"},
			{Label: "Email", Key: "email"},
			{Label: "Balance", Key: "balance", Type: gomukit.ColNumber, Format: "currency:EUR"},
			{Label: "Status", Key: "status", Type: gomukit.ColBadge, Badge: map[string]gomukit.BadgeVariant{
				"active":   gomukit.BadgeSuccess,
				"invited":  gomukit.BadgeInfo,
				"archived": gomukit.BadgeNeutral,
			}},
			{Label: "Region", Text: "eu-central-1"},
		}},
		Acknowledge: "I understand this cannot be undone.",
		Accept: gomukit.AcceptSpec{
			Tool:           "apply_delete_user",
			Label:          "Delete user",
			Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
			SuccessMessage: "User deleted.",
		},
		Reject: &gomukit.RejectSpec{Label: "Keep user", Message: "Nothing was deleted."},
		Brand:  demoBrand(),
		Theme:  &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

// followUpPicker asks for one date. The question and the shortcuts are
// authored here; the window it may be answered in is not — which days are
// still open changes between registration and the question, so
// schedule_followup computes min/max and the days already taken at call time
// and delivers them under "value" alongside the record.
func followUpPicker() *gomukit.DatePicker {
	return &gomukit.DatePicker{
		URI:    "ui://demo/followup",
		Title:  "Follow-up",
		Prompt: "When should we call this user back?",
		Body:   "Weekends are not working days, and the diary is already full on some.",
		Calendar: &gomukit.Calendar{
			DisableWeekends: true,
			Presets: []gomukit.DatePreset{
				{Label: "Today", Span: gomukit.SpanToday},
				{Label: "Tomorrow", Span: gomukit.SpanTomorrow},
			},
		},
		Details: gomukit.Descriptions{Items: []gomukit.DescriptionItem{
			{Label: "User", Key: "name"},
			{Label: "Email", Key: "email"},
			{Label: "Status", Key: "status", Type: gomukit.ColBadge, Badge: map[string]gomukit.BadgeVariant{
				"active":   gomukit.BadgeSuccess,
				"invited":  gomukit.BadgeInfo,
				"archived": gomukit.BadgeNeutral,
			}},
		}},
		Submit: gomukit.DateSubmit{
			Tool:           "set_followup_date",
			Label:          "Book the call",
			Args:           map[string]gomukit.ArgSource{"id": gomukit.FromRow("id")},
			SuccessMessage: "Call booked.",
		},
		Cancel: &gomukit.RejectSpec{Label: "Not now", Message: "No call was booked."},
		Brand:  demoBrand(),
		Theme:  &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

// demoMenu is the app's front door: one tile per UI-backed tool. Choosing a
// tile calls that tool, and the host opens the widget bound to it.
func demoMenu() *gomukit.Menu {
	return &gomukit.Menu{
		URI:   "ui://demo/menu",
		Title: "Acme users",
		Intro: "Pick where to start.",
		Items: []gomukit.MenuItem{
			{
				Tool:         "list_users",
				Label:        "User table",
				Description:  "Sortable, filterable directory with bulk actions.",
				IconSVG:      iconTable,
				Badge:        "read",
				BadgeVariant: gomukit.BadgeInfo,
			},
			{
				Tool:         "list_user_cards",
				Label:        "User cards",
				Description:  "The same users as a swipeable card strip.",
				IconSVG:      iconCards,
				Badge:        "read",
				BadgeVariant: gomukit.BadgeInfo,
			},
			{
				Tool:         "edit_user",
				Args:         map[string]any{"id": 1},
				Label:        "Edit Ada",
				Description:  "Open the edit form for the first user.",
				IconSVG:      iconPencil,
				Badge:        "write",
				BadgeVariant: gomukit.BadgeWarning,
			},
			{
				Tool:         "edit_profile",
				Args:         map[string]any{"id": 1},
				Label:        "Ada's full profile",
				Description:  "The whole account, in grouped fields two to a row.",
				IconSVG:      iconProfile,
				Badge:        "write",
				BadgeVariant: gomukit.BadgeWarning,
			},
			{
				Tool:         "schedule_followup",
				Args:         map[string]any{"id": 1},
				Label:        "Call Ada back",
				Description:  "Pick the day, from the ones still open.",
				IconSVG:      iconCalendar,
				Badge:        "write",
				BadgeVariant: gomukit.BadgeWarning,
			},
		},
		Brand: demoBrand(),
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

// Menu icons are inline SVG: a widget document references nothing external.
const (
	iconTable    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M3 10h18M9 10v10"/></svg>`
	iconCards    = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="5" width="8" height="14" rx="2"/><rect x="13" y="5" width="8" height="14" rx="2"/></svg>`
	iconPencil   = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><path d="M4 20h4L20 8l-4-4L4 16v4z"/></svg>`
	iconCalendar = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="5" width="18" height="16" rx="2"/><path d="M8 3v4M16 3v4M3 11h18"/></svg>`
	iconProfile  = `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" aria-hidden="true"><rect x="3" y="4" width="18" height="16" rx="2"/><circle cx="9" cy="10" r="2"/><path d="M6 16c.7-1.6 1.9-2.4 3-2.4s2.3.8 3 2.4M15 9h3M15 13h3"/></svg>`
)

// demoBrand is the application mark shown at the top left of every widget.
func demoBrand() *gomukit.Brand {
	return &gomukit.Brand{
		Name:    "Acme",
		URL:     "https://example.com",
		LogoSVG: `<svg viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><circle cx="8" cy="8" r="7"/></svg>`,
	}
}

func ptr[T any](v T) *T { return &v }

// --- server ---

func newServer(data *db) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "gomukit-demo", Version: "0.1.0"},
		gosdk.EnableUI(nil),
	)
	table := usersTable()
	cards := usersCards()
	form := userForm()
	profile := profileForm()
	menu := demoMenu()
	confirm := deleteConfirm()
	picker := followUpPicker()

	// Model-visible: list users, rendered by the table widget.
	type empty struct{}
	type rowsOut struct {
		Rows []map[string]any `json:"rows"`
	}

	// Model-visible: the app's main menu. It carries no data of its own — the
	// tiles are baked into the document — so the result is text only.
	must(gosdk.AddWidgetToolFor(server, menu,
		&mcp.Tool{Name: "main_menu", Description: "Show the Acme users app menu."},
		func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, empty, error) {
			return textResult("Showing the app menu."), empty{}, nil
		}))
	must(gosdk.AddWidgetToolFor(server, table,
		&mcp.Tool{Name: "list_users", Description: "List all users in an interactive table."},
		func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, rowsOut, error) {
			return nil, rowsOut{Rows: data.rows()}, nil
		}))

	// Model-visible: the same users rendered as a card grid.
	must(gosdk.AddWidgetToolFor(server, cards,
		&mcp.Tool{Name: "list_user_cards", Description: "List all users as a grid of cards."},
		func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, rowsOut, error) {
			return nil, rowsOut{Rows: data.rows()}, nil
		}))

	// App-only: fired by the table's row action.
	type deleteIn struct {
		ID int `json:"id"`
	}
	deleteTool := &mcp.Tool{Name: "delete_user", Description: "Delete a user by id."}
	gosdk.AppOnly(deleteTool, table)
	must(gosdk.AddWidgetToolFor(server, table, deleteTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in deleteIn) (*mcp.CallToolResult, rowsOut, error) {
			data.Lock()
			delete(data.users, in.ID)
			data.Unlock()
			return textResult("User deleted."), rowsOut{Rows: data.rows()}, nil
		}))

	// App-only: fired by the table's bulk action.
	type archiveIn struct {
		IDs []int `json:"ids"`
	}
	archiveTool := &mcp.Tool{Name: "archive_users", Description: "Archive users by id."}
	gosdk.AppOnly(archiveTool, table)
	must(gosdk.AddWidgetToolFor(server, table, archiveTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in archiveIn) (*mcp.CallToolResult, rowsOut, error) {
			data.Lock()
			for _, id := range in.IDs {
				if u := data.users[id]; u != nil {
					u.Status = "archived"
				}
			}
			data.Unlock()
			return textResult(fmt.Sprintf("Archived %d users.", len(in.IDs))), rowsOut{Rows: data.rows()}, nil
		}))

	// Model-visible: ask before deleting. The table's own row action deletes
	// behind a two-phase button; this is the other style — a view of its own
	// that spells out what the deletion costs before it runs.
	type confirmOut struct {
		Rows    []map[string]any `json:"rows"`
		Effects []map[string]any `json:"effects"`
	}
	must(gosdk.AddWidgetToolFor(server, confirm,
		&mcp.Tool{Name: "confirm_delete_user", Description: "Ask the user to confirm deleting a user by id."},
		func(_ context.Context, _ *mcp.CallToolRequest, in deleteIn) (*mcp.CallToolResult, confirmOut, error) {
			data.Lock()
			u := data.users[in.ID]
			data.Unlock()
			if u == nil {
				return textResult("No such user."), confirmOut{}, nil
			}
			// Effects are computed per call: this is what makes them worth
			// showing rather than authoring once at registration time.
			effects := []map[string]any{
				{"text": "Removes the account", "detail": "Sign-in stops working immediately.", "severity": "danger"},
				{"text": "Releases the balance", "value": fmt.Sprintf("%.2f EUR", u.Balance), "severity": "warning"},
			}
			if u.Status == "active" {
				effects = append(effects, map[string]any{"text": "Ends an active session", "severity": "warning"})
			}
			return nil, confirmOut{Rows: []map[string]any{userRow(u)}, Effects: effects}, nil
		}))

	// App-only: fired by the confirmation's accept button.
	applyDelete := &mcp.Tool{Name: "apply_delete_user", Description: "Delete a user by id, after confirmation."}
	gosdk.AppOnly(applyDelete, confirm)
	must(gosdk.AddWidgetToolFor(server, confirm, applyDelete,
		func(_ context.Context, _ *mcp.CallToolRequest, in deleteIn) (*mcp.CallToolResult, empty, error) {
			data.Lock()
			delete(data.users, in.ID)
			data.Unlock()
			return textResult("User deleted."), empty{}, nil
		}))

	// Model-visible: ask which day to call a user back on. The picker reads
	// the record under "rows" and its own limits under "value" — the window,
	// and the days the diary has already taken — both computed here rather
	// than authored into the widget, since neither is true for long.
	type dateOut struct {
		Rows  []map[string]any `json:"rows"`
		Value map[string]any   `json:"value"`
	}
	must(gosdk.AddWidgetToolFor(server, picker,
		&mcp.Tool{Name: "schedule_followup", Description: "Ask which day to call a user back on."},
		func(_ context.Context, _ *mcp.CallToolRequest, in deleteIn) (*mcp.CallToolResult, dateOut, error) {
			data.Lock()
			u := data.users[in.ID]
			taken := bookedDays(data)
			data.Unlock()
			if u == nil {
				return textResult("No such user."), dateOut{}, nil
			}
			today := time.Now()
			value := map[string]any{
				"min": today.Format(time.DateOnly),
				"max": today.AddDate(0, 0, 42).Format(time.DateOnly),
			}
			if len(taken) > 0 {
				value["disabled"] = taken
			}
			// A user who already has a call keeps it selected, so opening the
			// picker again shows what was booked rather than nothing.
			if u.FollowUp != "" {
				value["start"] = u.FollowUp
			}
			return nil, dateOut{Rows: []map[string]any{userRow(u)}, Value: value}, nil
		}))

	// App-only: fired by the picker's submit button.
	type followUpIn struct {
		ID   int    `json:"id"`
		Date string `json:"date"`
	}
	followUpTool := &mcp.Tool{Name: "set_followup_date", Description: "Record the day a user is called back."}
	gosdk.AppOnly(followUpTool, picker)
	must(gosdk.AddWidgetToolFor(server, picker, followUpTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in followUpIn) (*mcp.CallToolResult, rowsOut, error) {
			data.Lock()
			u := data.users[in.ID]
			if u != nil {
				u.FollowUp = in.Date
			}
			data.Unlock()
			if u == nil {
				return textResult("No such user."), rowsOut{}, nil
			}
			// The rows go back too: a table open beside the picker repaints
			// from the server's truth rather than guessing locally.
			return textResult(fmt.Sprintf("%s will be called back on %s.", u.Name, in.Date)),
				rowsOut{Rows: data.rows()}, nil
		}))

	// Model-visible: open the edit form prefilled for one user.
	type editIn struct {
		ID int `json:"id"`
	}
	type editOut struct {
		Values map[string]any `json:"values"`
	}
	must(gosdk.AddWidgetToolFor(server, form,
		&mcp.Tool{Name: "edit_user", Description: "Open an edit form for the given user id."},
		func(_ context.Context, _ *mcp.CallToolRequest, in editIn) (*mcp.CallToolResult, editOut, error) {
			data.Lock()
			u := data.users[in.ID]
			data.Unlock()
			if u == nil {
				return textResult(fmt.Sprintf("User %d not found.", in.ID)), editOut{}, nil
			}
			values, err := gomukit.RowsOf([]*user{u})
			if err != nil {
				return nil, editOut{}, err
			}
			return nil, editOut{Values: values[0]}, nil
		}))

	// App-only: the form's submit target, with server-side validation.
	type saveIn struct {
		ID      string  `json:"id"` // hidden form fields submit strings
		Name    string  `json:"name"`
		Email   string  `json:"email"`
		Status  string  `json:"status"`
		Balance float64 `json:"balance"`
	}
	type saveOut struct {
		Errors map[string]string `json:"errors,omitempty"`
	}
	saveTool := &mcp.Tool{Name: "save_user", Description: "Save a user."}
	gosdk.AppOnly(saveTool, form)
	must(gosdk.AddWidgetToolFor(server, form, saveTool,
		func(_ context.Context, _ *mcp.CallToolRequest, in saveIn) (*mcp.CallToolResult, saveOut, error) {
			userID, _ := strconv.Atoi(in.ID) // 0 (no/invalid id) creates a new user
			errs := map[string]string{}
			if strings.TrimSpace(in.Name) == "" {
				errs["name"] = "Name must not be empty."
			}
			data.Lock()
			for id, u := range data.users {
				if id != userID && strings.EqualFold(u.Email, in.Email) {
					errs["email"] = "This email is already taken."
				}
			}
			if len(errs) > 0 {
				data.Unlock()
				return nil, saveOut{Errors: errs}, nil
			}
			u := data.users[userID]
			if u == nil {
				u = &user{ID: data.nextID, Created: "2026-07-23T00:00:00Z"}
				data.nextID++
				data.users[u.ID] = u
			}
			u.Name, u.Email, u.Status, u.Balance = in.Name, in.Email, in.Status, in.Balance
			data.Unlock()
			return textResult("User saved."), saveOut{}, nil
		}))

	// Model-visible: the same record in the grouped profile form. Prefill is
	// one flat {field: value} map whatever the grouping is — RowsOf over the
	// record covers every field of every group, because a field set changes
	// where a field is drawn and nothing else.
	must(gosdk.AddWidgetToolFor(server, profile,
		&mcp.Tool{Name: "edit_profile", Description: "Open the full customer profile form for the given user id."},
		func(_ context.Context, _ *mcp.CallToolRequest, in editIn) (*mcp.CallToolResult, editOut, error) {
			data.Lock()
			u := data.users[in.ID]
			data.Unlock()
			if u == nil {
				return textResult(fmt.Sprintf("User %d not found.", in.ID)), editOut{}, nil
			}
			values, err := gomukit.RowsOf([]*user{u})
			if err != nil {
				return nil, editOut{}, err
			}
			return nil, editOut{Values: values[0]}, nil
		}))

	// App-only: the profile form's submit target. A grouped form submits one
	// flat argument map, so this reads exactly like save_user — the date range
	// is the only shape worth noting, and it is two plain string arguments.
	type saveProfileIn struct {
		ID       string  `json:"id"` // hidden fields submit strings
		Company  string  `json:"company"`
		Name     string  `json:"name"`
		Phone    string  `json:"phone"`
		Email    string  `json:"email"`
		Plan     string  `json:"plan"`
		Seats    int     `json:"seats"`
		Status   string  `json:"status"`
		Balance  float64 `json:"balance"`
		StartsOn string  `json:"startsOn"`
		EndsOn   string  `json:"endsOn"`
		Notes    string  `json:"notes"`
		Announce bool    `json:"announce"`
	}
	// Seat floors per plan: a rule the browser cannot check, so it comes back
	// as a field error on the control it belongs to.
	seatFloor := map[string]int{"starter": 1, "team": 3, "enterprise": 10}
	saveProfile := &mcp.Tool{Name: "save_profile", Description: "Save the full customer profile."}
	gosdk.AppOnly(saveProfile, profile)
	must(gosdk.AddWidgetToolFor(server, profile, saveProfile,
		func(_ context.Context, _ *mcp.CallToolRequest, in saveProfileIn) (*mcp.CallToolResult, saveOut, error) {
			userID, _ := strconv.Atoi(in.ID) // 0 (no/invalid id) creates a new user
			errs := map[string]string{}
			if strings.TrimSpace(in.Company) == "" {
				errs["company"] = "Every seat is billed to a company."
			}
			if strings.TrimSpace(in.Name) == "" {
				errs["name"] = "Name must not be empty."
			}
			if floor := seatFloor[in.Plan]; in.Seats < floor {
				errs["seats"] = fmt.Sprintf("The %s plan starts at %d seats.", in.Plan, floor)
			}
			if in.EndsOn != "" && in.StartsOn == "" {
				errs["startsOn"] = "A contract that ends has to start."
			}
			data.Lock()
			for id, u := range data.users {
				if id != userID && strings.EqualFold(u.Email, in.Email) {
					errs["email"] = "This email is already taken."
				}
			}
			if len(errs) > 0 {
				data.Unlock()
				return nil, saveOut{Errors: errs}, nil
			}
			u := data.users[userID]
			if u == nil {
				u = &user{ID: data.nextID, Created: "2026-07-23T00:00:00Z"}
				data.nextID++
				data.users[u.ID] = u
			}
			u.Company, u.Name, u.Phone, u.Email = in.Company, in.Name, in.Phone, in.Email
			u.Plan, u.Seats, u.Status, u.Balance = in.Plan, in.Seats, in.Status, in.Balance
			u.StartsOn, u.EndsOn, u.Notes, u.Announce = in.StartsOn, in.EndsOn, in.Notes, in.Announce
			data.Unlock()
			return textResult(fmt.Sprintf("Saved the profile for %s.", u.Name)), saveOut{}, nil
		}))

	return server
}

func textResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	stdio := flag.Bool("stdio", false, "serve over stdio instead of HTTP")
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	server := newServer(seed())

	if *stdio {
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
		return
	}

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	log.Printf("gomukit demo MCP server on http://localhost%s/mcp", *addr)
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
