package gadget

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gadget/internal/htmlx"
)

// statusNode renders the shared, runtime-driven status/announcement region.
func statusNode() g.Node {
	return h.Div(h.Class("gadget-status"), htmlx.Data("status", ""), g.Attr("hidden"), h.Aria("live", "polite"))
}

// emptyStateNode renders the shared no-data region, hidden until the runtime
// determines there is nothing to show. Title defaults to "No data".
func emptyStateNode(e EmptyState) g.Node {
	title := e.Title
	if title == "" {
		title = "No data"
	}
	nodes := []g.Node{
		h.Class("gadget-empty"),
		htmlx.Data("empty", ""),
		g.Attr("hidden"),
		h.H3(g.Text(title)),
	}
	if e.Body != "" {
		nodes = append(nodes, h.P(g.Text(e.Body)))
	}
	return h.Div(nodes...)
}

// paginationNode renders the shared prev/next pager, hidden until the runtime
// enables it.
func paginationNode() g.Node {
	return h.Div(h.Class("gadget-pagination"), htmlx.Data("pagination", ""), g.Attr("hidden"),
		h.Button(h.Type("button"), h.Class("gadget-btn"), htmlx.Data("page", "prev"), g.Text("Previous")),
		h.Span(h.Class("gadget-page-info"), htmlx.Data("page-info", "")),
		h.Button(h.Type("button"), h.Class("gadget-btn"), htmlx.Data("page", "next"), g.Text("Next")),
	)
}
