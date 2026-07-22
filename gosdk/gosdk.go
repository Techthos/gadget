// Package gosdk adapts gadget widgets to the official Go MCP SDK
// (github.com/modelcontextprotocol/go-sdk). It is the only gadget package
// that imports the SDK; the core stays SDK-agnostic.
//
// Typical wiring:
//
//	table := &gadget.Table{URI: "ui://demo/users", Columns: ...}
//	server := mcp.NewServer(&mcp.Implementation{Name: "demo"}, gosdk.EnableUI(nil))
//	gosdk.AddWidgetToolFor(server, table, &mcp.Tool{Name: "list_users"}, listUsers)
package gosdk

import (
	"context"
	"fmt"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/techthos/gadget"
	"github.com/techthos/gadget/uispec"
)

// EnableUI declares the MCP Apps extension (io.modelcontextprotocol/ui) in
// the server capabilities. It mutates and returns opts so it composes with
// mcp.NewServer; passing nil allocates fresh options.
//
// Note: explicitly setting Capabilities disables the SDK's historical
// default of advertising {"logging":{}}; tool/resource capabilities are
// still inferred from registered features.
func EnableUI(opts *mcp.ServerOptions) *mcp.ServerOptions {
	if opts == nil {
		opts = &mcp.ServerOptions{}
	}
	if opts.Capabilities == nil {
		opts.Capabilities = &mcp.ServerCapabilities{}
	}
	opts.Capabilities.AddExtension(uispec.ExtensionID, map[string]any{
		"mimeTypes": []string{uispec.MIMEType},
	})
	return opts
}

var (
	mu sync.Mutex
	// registered tracks widget URIs per server for idempotent AddWidget.
	// Entries live as long as the server; fine for the usual
	// servers-live-for-the-process-lifetime deployment.
	registered = map[*mcp.Server]map[string]bool{}
)

// AddWidget registers w's template as a ui:// resource on s. The document
// is rendered once and served from memory. Registering the same URI on the
// same server again is a no-op.
func AddWidget(s *mcp.Server, w gadget.Widget) error {
	d := w.Descriptor()

	mu.Lock()
	uris := registered[s]
	if uris == nil {
		uris = map[string]bool{}
		registered[s] = uris
	}
	if uris[d.URI] {
		mu.Unlock()
		return nil
	}
	mu.Unlock()

	doc, err := w.Document() // validates the widget
	if err != nil {
		return fmt.Errorf("gosdk: render widget %s: %w", d.URI, err)
	}

	res := &mcp.Resource{
		URI:         d.URI,
		Name:        d.Name,
		Title:       d.Title,
		Description: d.Description,
		MIMEType:    d.MIMEType,
	}
	if m := d.MetaMap(); m != nil {
		res.Meta = m
	}
	handler := func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      d.URI,
				MIMEType: d.MIMEType,
				Text:     doc,
			}},
		}, nil
	}

	mu.Lock()
	defer mu.Unlock()
	if uris[d.URI] { // lost a race; the other registration won
		return nil
	}
	s.AddResource(res, handler)
	uris[d.URI] = true
	return nil
}

// AddWidgetTool registers t with its _meta linked to w (registering w's
// resource first if needed) and installs the raw handler h.
func AddWidgetTool(s *mcp.Server, w gadget.Widget, t *mcp.Tool, h mcp.ToolHandler) error {
	if err := AddWidget(s, w); err != nil {
		return err
	}
	linkTool(t, w)
	s.AddTool(t, h)
	return nil
}

// AddWidgetToolFor is AddWidgetTool with the SDK's typed-handler variant:
// input/output schemas are inferred from In and Out.
func AddWidgetToolFor[In, Out any](s *mcp.Server, w gadget.Widget, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) error {
	if err := AddWidget(s, w); err != nil {
		return err
	}
	linkTool(t, w)
	mcp.AddTool(s, t, h)
	return nil
}

func linkTool(t *mcp.Tool, w gadget.Widget) {
	t.Meta = uispec.MergeMeta(t.Meta, w.ToolMeta())
}

// AppOnly links t to w and marks it app-only (visibility: ["app"]): callable
// from the widget UI, hidden from the model. Use it for row-action and
// submit tools the model should not invoke directly. Register the tool
// separately (e.g. via AddWidgetTool, which keeps the merged visibility).
func AppOnly(t *mcp.Tool, w gadget.Widget) {
	meta := uispec.ToolUIMeta{
		ResourceURI: w.Descriptor().URI,
		Visibility:  []string{uispec.VisibilityApp},
	}.MetaMap()
	t.Meta = uispec.MergeMeta(t.Meta, meta)
}

// WithAppData merges data into the result's _meta. Result _meta is
// delivered to the widget but hidden from the model, per the MCP Apps spec.
func WithAppData(res *mcp.CallToolResult, data map[string]any) {
	res.Meta = uispec.MergeMeta(res.Meta, data)
}

// ClientSupportsUI reports whether the session's client declared the MCP
// Apps extension. Servers MAY branch on it (e.g. return richer text for
// non-UI clients); attaching _meta.ui unconditionally is also spec-legal —
// hosts ignore unknown metadata.
func ClientSupportsUI(ss *mcp.ServerSession) bool {
	p := ss.InitializeParams()
	if p == nil || p.Capabilities == nil {
		return false
	}
	_, ok := p.Capabilities.Extensions[uispec.ExtensionID]
	return ok
}
