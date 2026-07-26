// Command demo is a runnable MCP server showcasing gomukit widgets: a user
// table with row/bulk actions, the same users as a card grid, an edit form
// with server-side validation, a confirmation, and a date picker whose
// selectable window is computed per call.
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
}

type db struct {
	sync.Mutex
	users  map[int]*user
	nextID int
}

func seed() *db {
	users := []*user{
		{1, "Ada Lovelace", "ada@example.com", "active", 1200.50, "2026-01-12T09:00:00Z", ""},
		{2, "Grace Hopper", "grace@example.com", "active", 815.00, "2026-02-03T10:30:00Z", ""},
		{3, "Alan Turing", "alan@example.com", "invited", 0, "2026-03-19T14:00:00Z", ""},
		{4, "Katherine Johnson", "katherine@example.com", "archived", 233.10, "2026-04-01T08:15:00Z", ""},
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
