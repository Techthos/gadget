// Command harness serves a minimal fake MCP Apps host for manually smoke-
// testing gadget widgets without a real host: it renders standalone widget
// documents (baked data) and embeds them in an iframe behind a JSON-RPC
// postMessage host that logs all traffic.
//
//	go run ./examples/harness
//	open http://localhost:8090
package main

import (
	_ "embed"
	"flag"
	"log"
	"net/http"

	"github.com/techthos/gadget"
	"github.com/techthos/gadget/theme"
)

//go:embed host.html
var hostPage []byte

func table() *gadget.Table {
	return &gadget.Table{
		URI:   "ui://harness/table",
		Title: "Users",
		Columns: []gadget.Column{
			gadget.Text("name", "Name"),
			gadget.Number("balance", "Balance", "currency:EUR"),
			gadget.Date("createdAt", "Created", "date"),
			gadget.Badge("status", "Status", map[string]gadget.BadgeVariant{
				"active": gadget.BadgeSuccess, "invited": gadget.BadgeInfo,
			}),
			gadget.ActionsColumn(
				gadget.Action{Label: "Delete", Tool: "delete_user", Variant: gadget.VariantDanger,
					Confirm: "Really?", Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
			),
		},
		PageSize:   3,
		Filterable: true,
		Selection: &gadget.SelectionConfig{Bulk: []gadget.Action{
			{Label: "Archive", Tool: "archive_users", Args: map[string]gadget.ArgSource{"ids": gadget.FromSelection("id")}},
		}},
		InitialData: map[string]any{"rows": []map[string]any{
			{"id": 1, "name": "Ada Lovelace", "balance": 1200.5, "createdAt": "2026-01-12T09:00:00Z", "status": "active"},
			{"id": 2, "name": "Grace Hopper", "balance": 815, "createdAt": "2026-02-03T10:30:00Z", "status": "active"},
			{"id": 3, "name": "Alan Turing", "balance": 0, "createdAt": "2026-03-19T14:00:00Z", "status": "invited"},
			{"id": 4, "name": "Katherine Johnson", "balance": 233.1, "createdAt": "2026-04-01T08:15:00Z", "status": "active"},
		}},
		Theme: &theme.Theme{ColorPrimary: "#7c3aed"},
	}
}

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
	}
}

func widgetHandler(w gadget.Widget) http.HandlerFunc {
	return func(rw http.ResponseWriter, _ *http.Request) {
		doc, err := w.Document()
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = rw.Write([]byte(doc))
	}
}

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = rw.Write(hostPage)
	})
	mux.HandleFunc("/widget/table", widgetHandler(table()))
	mux.HandleFunc("/widget/form", widgetHandler(form()))

	log.Printf("gadget harness on http://localhost%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
