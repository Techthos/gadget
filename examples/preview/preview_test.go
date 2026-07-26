package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/techthos/gadget/uispec"
)

// connect spins up the preview server against an in-memory client that
// declares the MCP Apps extension, which is what a UI-capable inspector does.
func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	return connectTo(t, newServer(true, true, false))
}

// connectTo is connect against a server the caller built, which is how the
// -sandbox arrangement is tested: one server per session.
func connectTo(t *testing.T, server *mcp.Server) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	caps := &mcp.ClientCapabilities{}
	caps.AddExtension(uispec.ExtensionID, map[string]any{"mimeTypes": []string{uispec.MIMEType}})
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "preview-test"}, &mcp.ClientOptions{Capabilities: caps}).
		Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return cs
}

// TestResourcesRender reads every registered ui:// resource and checks it is a
// self-contained document. A widget that fails validation never gets this far:
// registration renders it.
func TestResourcesRender(t *testing.T) {
	ctx := context.Background()
	cs := connect(t)

	res, err := cs.ListResources(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Resources) < 30 {
		t.Fatalf("expected the full widget set, got %d resources", len(res.Resources))
	}
	for _, r := range res.Resources {
		if !strings.HasPrefix(r.URI, "ui://preview/") {
			t.Errorf("unexpected resource URI %q", r.URI)
		}
		if r.MIMEType != uispec.MIMEType {
			t.Errorf("%s: mime type %q, want %q", r.URI, r.MIMEType, uispec.MIMEType)
		}
		read, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: r.URI})
		if err != nil {
			t.Fatalf("%s: %v", r.URI, err)
		}
		doc := read.Contents[0].Text
		if !strings.HasPrefix(doc, "<!doctype html>") {
			t.Errorf("%s: not an HTML document", r.URI)
		}
		if strings.Contains(doc, "src=\"http") || strings.Contains(doc, "href=\"http://cdn") {
			t.Errorf("%s: document references an external resource", r.URI)
		}
	}
}

// TestToolsAreLinked checks every tool points at a widget resource, and that
// the tools only widgets fire are hidden from the model.
func TestToolsAreLinked(t *testing.T) {
	ctx := context.Background()
	cs := connect(t)

	res, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	uris := map[string]bool{}
	for _, r := range mustResources(t, cs) {
		uris[r.URI] = true
	}

	appOnly := 0
	for _, tool := range res.Tools {
		if tool.Name == "reset_demo" { // the one tool with no widget
			continue
		}
		meta, ok := tool.Meta[uispec.MetaKey].(map[string]any)
		if !ok {
			t.Errorf("%s: no ui meta", tool.Name)
			continue
		}
		uri, _ := meta["resourceUri"].(string)
		if !uris[uri] {
			t.Errorf("%s: resourceUri %q is not a registered resource", tool.Name, uri)
		}
		if vis, ok := meta["visibility"].([]any); ok && len(vis) == 1 && vis[0] == uispec.VisibilityApp {
			appOnly++
		}
	}
	if appOnly < 10 {
		t.Errorf("expected the widget-only tools to be marked app-only, got %d", appOnly)
	}
}

// TestGalleryToolsAnswer calls every gallery tool and checks the ones that
// carry runtime data answer with the key their widget reads.
func TestGalleryToolsAnswer(t *testing.T) {
	cs := connect(t)

	for _, p := range galleryCatalog() {
		out := call(t, cs, p.Tool, nil)
		if p.Data == nil {
			continue
		}
		want := p.Data()
		switch {
		case want.Records != nil:
			assertKey(t, p.Tool, out, "records")
		case want.Options != nil:
			assertKey(t, p.Tool, out, "options")
			assertKey(t, p.Tool, out, "rows")
		case want.Effects != nil:
			assertKey(t, p.Tool, out, "effects")
		case want.Values != nil:
			assertKey(t, p.Tool, out, "values")
		default:
			assertKey(t, p.Tool, out, "rows")
			if len(*want.Rows) == 0 {
				// An empty-state variant has to say "rows": [] rather than
				// leave the key out, or the widget keeps what it had.
				if rows, _ := out["rows"].([]any); len(rows) != 0 {
					t.Errorf("%s: expected an explicit empty rows array, got %v", p.Tool, out["rows"])
				}
			}
		}
	}
}

// TestScenarioMutates walks the app the way a user would: list, delete, and
// confirm the next listing reflects it.
func TestScenarioMutates(t *testing.T) {
	cs := connect(t)

	before := rowCount(t, call(t, cs, "list_customers", nil))
	call(t, cs, "delete_customer", map[string]any{"id": 1})
	if after := rowCount(t, call(t, cs, "list_customers", nil)); after != before-1 {
		t.Fatalf("after deleting: %d rows, want %d", after, before-1)
	}

	// The form's submit tool validates server side; a duplicate email comes
	// back as a field error rather than a write.
	out := call(t, cs, "save_customer", map[string]any{
		"id": "2", "name": "Grace Hopper", "email": "katherine@example.com",
		"plan": "team", "status": "active", "seats": 12,
	})
	errs, ok := out["errors"].(map[string]any)
	if !ok || errs["email"] == nil {
		t.Fatalf("expected an email error, got %v", out)
	}

	// The confirmation counts its consequences from current state.
	out = call(t, cs, "confirm_delete_customer", map[string]any{"id": 2})
	effects, _ := out["effects"].([]any)
	if len(effects) < 2 {
		t.Fatalf("expected computed effects, got %v", out["effects"])
	}

	// Shipping options are priced per order, so a heavier parcel gets a
	// different offer than a light one.
	out = call(t, cs, "choose_shipping", map[string]any{"id": 4474})
	options, _ := out["options"].([]any)
	if len(options) != 3 {
		t.Fatalf("expected three shipping options, got %d", len(options))
	}
	drone, _ := options[2].(map[string]any)
	if drone["disabled"] != true {
		t.Errorf("the 5.1 kg parcel should not be flyable: %v", drone)
	}

	// The delivery window is computed against the day the tool runs, so it
	// bounds the grid at call time rather than at registration time.
	out = call(t, cs, "schedule_delivery", map[string]any{"id": 4474})
	window, ok := out["value"].(map[string]any)
	if !ok {
		t.Fatalf("expected a delivery window, got %v", out)
	}
	min, _ := window["min"].(string)
	max, _ := window["max"].(string)
	if min == "" || max == "" || min >= max {
		t.Fatalf("expected a forward window, got %v", window)
	}
	if blocked, _ := window["disabled"].([]any); len(blocked) != 2 {
		t.Fatalf("expected the two stocktaking days, got %v", window["disabled"])
	}
	if _, seeded := window["start"]; seeded {
		t.Errorf("an order with no delivery date must not preselect one: %v", window)
	}
	// Booking one records it, and the next call opens on what was booked.
	if res := callText(t, cs, "set_delivery_date", map[string]any{"id": 4474, "date": min}); !strings.Contains(res, min) {
		t.Errorf("set_delivery_date = %q, want the booked day", res)
	}
	out = call(t, cs, "schedule_delivery", map[string]any{"id": 4474})
	window, _ = out["value"].(map[string]any)
	if window["start"] != min {
		t.Errorf("expected the booked day back, got %v", window)
	}

	call(t, cs, "reset_demo", nil)
	if after := rowCount(t, call(t, cs, "list_customers", nil)); after != before {
		t.Fatalf("after reset: %d rows, want %d", after, before)
	}
}

// TestSandboxIsolatesSessions covers what -sandbox promises a hosted preview:
// a server built per session carries its own store, so one visitor's delete
// is invisible to the next.
func TestSandboxIsolatesSessions(t *testing.T) {
	a := connectTo(t, newServer(true, true, false))
	b := connectTo(t, newServer(true, true, false))

	before := rowCount(t, call(t, b, "list_customers", nil))
	call(t, a, "delete_customer", map[string]any{"id": 1})

	if after := rowCount(t, call(t, a, "list_customers", nil)); after != before-1 {
		t.Fatalf("session a: %d rows after its own delete, want %d", after, before-1)
	}
	if after := rowCount(t, call(t, b, "list_customers", nil)); after != before {
		t.Fatalf("session b saw session a's delete: %d rows, want %d", after, before)
	}
}

// --- helpers ---

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) map[string]any {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s: tool reported an error: %v", name, res.Content)
	}
	if res.StructuredContent == nil {
		return map[string]any{}
	}
	var out map[string]any
	raw, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out
}

// callText is call for the tools that answer in prose rather than in
// structuredContent.
func callText(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) string {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	if res.IsError {
		t.Fatalf("%s: tool reported an error: %v", name, res.Content)
	}
	var b strings.Builder
	for _, c := range res.Content {
		if text, ok := c.(*mcp.TextContent); ok {
			b.WriteString(text.Text)
		}
	}
	return b.String()
}

func mustResources(t *testing.T, cs *mcp.ClientSession) []*mcp.Resource {
	t.Helper()
	res, err := cs.ListResources(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return res.Resources
}

func assertKey(t *testing.T, tool string, out map[string]any, key string) {
	t.Helper()
	if _, ok := out[key]; !ok {
		t.Errorf("%s: result carries no %q key: %v", tool, key, out)
	}
}

func rowCount(t *testing.T, out map[string]any) int {
	t.Helper()
	rows, ok := out["rows"].([]any)
	if !ok {
		t.Fatalf("result carries no rows: %v", out)
	}
	return len(rows)
}
