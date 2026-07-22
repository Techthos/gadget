// Command demo is a runnable MCP server showcasing gadget widgets: a user
// table with row/bulk actions and an edit form with server-side validation.
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

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/techthos/gadget"
	"github.com/techthos/gadget/gosdk"
	"github.com/techthos/gadget/theme"
)

// --- in-memory data ---

type user struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Email   string  `json:"email"`
	Status  string  `json:"status"`
	Balance float64 `json:"balance"`
	Created string  `json:"createdAt"`
}

type db struct {
	sync.Mutex
	users  map[int]*user
	nextID int
}

func seed() *db {
	users := []*user{
		{1, "Ada Lovelace", "ada@example.com", "active", 1200.50, "2026-01-12T09:00:00Z"},
		{2, "Grace Hopper", "grace@example.com", "active", 815.00, "2026-02-03T10:30:00Z"},
		{3, "Alan Turing", "alan@example.com", "invited", 0, "2026-03-19T14:00:00Z"},
		{4, "Katherine Johnson", "katherine@example.com", "archived", 233.10, "2026-04-01T08:15:00Z"},
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
	rows, _ := gadget.RowsOf(list)
	return rows
}

// --- widgets ---

func usersTable() *gadget.Table {
	return &gadget.Table{
		URI:   "ui://demo/users",
		Title: "Users",
		Columns: []gadget.Column{
			gadget.Text("name", "Name"),
			gadget.Text("email", "Email"),
			gadget.Number("balance", "Balance", "currency:EUR"),
			gadget.Date("createdAt", "Created", "date"),
			gadget.Badge("status", "Status", map[string]gadget.BadgeVariant{
				"active":   gadget.BadgeSuccess,
				"invited":  gadget.BadgeInfo,
				"archived": gadget.BadgeNeutral,
			}),
			gadget.ActionsColumn(
				gadget.Action{
					Label: "Delete", Tool: "delete_user", Variant: gadget.VariantDanger,
					Confirm: "Really delete?",
					Args:    map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
				},
			),
		},
		PageSize:    10,
		DefaultSort: &gadget.SortSpec{Key: "name"},
		Filterable:  true,
		Selection: &gadget.SelectionConfig{Bulk: []gadget.Action{
			{
				Label: "Archive", Tool: "archive_users",
				Args: map[string]gadget.ArgSource{"ids": gadget.FromSelection("id")},
			},
		}},
		Empty: gadget.EmptyState{Title: "No users", Body: "Ask the assistant to create one."},
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func userForm() *gadget.Form {
	return &gadget.Form{
		URI:   "ui://demo/user-form",
		Title: "Edit user",
		Fields: []gadget.Field{
			{Name: "id", Type: gadget.FHidden},
			{Name: "name", Label: "Name", Required: true},
			{Name: "email", Label: "Email", Required: true,
				Validation: &gadget.Validation{Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email address."}},
			{Name: "status", Label: "Status", Type: gadget.FSelect, Required: true,
				Options: []gadget.Option{gadget.Opt("active"), gadget.Opt("invited"), gadget.Opt("archived")}},
			{Name: "balance", Label: "Balance (EUR)", Type: gadget.FNumber,
				Validation: &gadget.Validation{Min: ptr(0.0), Step: ptr(0.01)}},
		},
		Submit: gadget.SubmitSpec{Tool: "save_user", Label: "Save", SuccessMessage: "User saved."},
		Cancel: &gadget.CancelSpec{},
		Theme:  &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

func ptr[T any](v T) *T { return &v }

// --- server ---

func newServer(data *db) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "gadget-demo", Version: "0.1.0"},
		gosdk.EnableUI(nil),
	)
	table := usersTable()
	form := userForm()

	// Model-visible: list users, rendered by the table widget.
	type empty struct{}
	type rowsOut struct {
		Rows []map[string]any `json:"rows"`
	}
	must(gosdk.AddWidgetToolFor(server, table,
		&mcp.Tool{Name: "list_users", Description: "List all users in an interactive table."},
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
			values, err := gadget.RowsOf([]*user{u})
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
	log.Printf("gadget demo MCP server on http://localhost%s/mcp", *addr)
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
