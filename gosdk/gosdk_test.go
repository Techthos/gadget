package gosdk

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/techthos/gomukit"
	"github.com/techthos/gomukit/uispec"
)

type listInput struct{}

type listOutput struct {
	Rows []map[string]any `json:"rows"`
}

func testTable() *gomukit.Table {
	return &gomukit.Table{
		URI:     "ui://demo/users",
		Title:   "Users",
		Columns: []gomukit.Column{gomukit.Text("name", "Name")},
	}
}

func listUsers(context.Context, *mcp.CallToolRequest, listInput) (*mcp.CallToolResult, listOutput, error) {
	return nil, listOutput{Rows: []map[string]any{{"id": 1, "name": "Ada"}}}, nil
}

// connect spins up an in-memory client/server pair.
func connect(t *testing.T, server *mcp.Server, clientOpts *mcp.ClientOptions) (*mcp.ServerSession, *mcp.ClientSession) {
	t.Helper()
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	ss, err := server.Connect(ctx, st, nil)
	if err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, clientOpts)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return ss, cs
}

func uiClientOpts() *mcp.ClientOptions {
	caps := &mcp.ClientCapabilities{}
	caps.AddExtension(uispec.ExtensionID, map[string]any{
		"mimeTypes": []string{uispec.MIMEType},
	})
	return &mcp.ClientOptions{Capabilities: caps}
}

func TestEndToEnd(t *testing.T) {
	ctx := context.Background()
	tbl := testTable()
	server := mcp.NewServer(&mcp.Implementation{Name: "demo", Version: "0.0.1"}, EnableUI(nil))

	tool := &mcp.Tool{Name: "list_users", Description: "List users"}
	if err := AddWidgetToolFor(server, tbl, tool, listUsers); err != nil {
		t.Fatal(err)
	}

	ss, cs := connect(t, server, uiClientOpts())

	// Server capabilities advertise the extension.
	initRes := cs.InitializeResult()
	if initRes == nil || initRes.Capabilities == nil {
		t.Fatal("no initialize result / capabilities")
	}
	if _, ok := initRes.Capabilities.Extensions[uispec.ExtensionID]; !ok {
		t.Errorf("server capabilities missing extension %s: %+v", uispec.ExtensionID, initRes.Capabilities.Extensions)
	}

	// Session sees the client's extension declaration.
	if !ClientSupportsUI(ss) {
		t.Error("ClientSupportsUI = false for a UI-declaring client")
	}

	// tools/list exposes the merged _meta.ui.resourceUri.
	tools, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, tl := range tools.Tools {
		if tl.Name != "list_users" {
			continue
		}
		found = true
		ui, ok := tl.Meta[uispec.MetaKey].(map[string]any)
		if !ok {
			t.Fatalf("tool _meta.ui missing: %v", tl.Meta)
		}
		if ui["resourceUri"] != "ui://demo/users" {
			t.Errorf("resourceUri = %v", ui["resourceUri"])
		}
	}
	if !found {
		t.Fatal("list_users not in tools/list")
	}

	// resources/read returns the template document.
	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "ui://demo/users"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("contents = %d", len(res.Contents))
	}
	c := res.Contents[0]
	if c.MIMEType != uispec.MIMEType {
		t.Errorf("mimeType = %q", c.MIMEType)
	}
	for _, want := range []string{"<!doctype html>", `id="gomu-config"`, `data-gomu-widget="table"`} {
		if !strings.Contains(c.Text, want) {
			t.Errorf("document missing %q", want)
		}
	}

	// Calling the tool returns structuredContent rows.
	out, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_users", Arguments: map[string]any{}})
	if err != nil {
		t.Fatal(err)
	}
	sc, ok := out.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structuredContent = %T", out.StructuredContent)
	}
	if _, ok := sc["rows"]; !ok {
		t.Errorf("structuredContent missing rows: %v", sc)
	}
}

func TestClientWithoutUI(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "demo", Version: "0.0.1"}, EnableUI(nil))
	if err := AddWidget(server, testTable()); err != nil {
		t.Fatal(err)
	}
	ss, _ := connect(t, server, nil)
	if ClientSupportsUI(ss) {
		t.Error("ClientSupportsUI = true for a plain client")
	}
}

func TestAddWidgetIdempotent(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "demo", Version: "0.0.1"}, EnableUI(nil))
	tbl := testTable()
	if err := AddWidget(server, tbl); err != nil {
		t.Fatal(err)
	}
	// A second registration (e.g. two tools sharing one widget) must not panic.
	if err := AddWidget(server, tbl); err != nil {
		t.Fatal(err)
	}
}

func TestAppOnlyVisibility(t *testing.T) {
	tool := &mcp.Tool{Name: "delete_user"}
	AppOnly(tool, testTable())
	ui, ok := tool.Meta[uispec.MetaKey].(map[string]any)
	if !ok {
		t.Fatalf("meta = %v", tool.Meta)
	}
	vis, ok := ui["visibility"].([]any)
	if !ok || len(vis) != 1 || vis[0] != "app" {
		t.Errorf("visibility = %v", ui["visibility"])
	}
	if ui["resourceUri"] != "ui://demo/users" {
		t.Errorf("resourceUri = %v", ui["resourceUri"])
	}
}

func TestWithAppData(t *testing.T) {
	res := &mcp.CallToolResult{}
	WithAppData(res, map[string]any{"secret": "app-only"})
	if res.Meta["secret"] != "app-only" {
		t.Errorf("meta = %v", res.Meta)
	}
}

func TestAddWidgetInvalidWidget(t *testing.T) {
	server := mcp.NewServer(&mcp.Implementation{Name: "demo", Version: "0.0.1"}, nil)
	bad := &gomukit.Table{URI: "https://wrong"}
	if err := AddWidget(server, bad); err == nil {
		t.Error("AddWidget with invalid widget must error")
	}
}
