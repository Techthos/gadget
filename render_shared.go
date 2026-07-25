package gadget

import (
	"fmt"
	"sort"
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gadget/internal/htmlx"
)

// statusNode renders the shared, runtime-driven status/announcement region.
// It is the last element of every widget, and its bar keeps its height whether
// or not a message is showing, so the widget does not resize (and the host
// iframe does not jump) when work starts or finishes.
func statusNode() g.Node {
	return h.Div(h.Class("gadget-statusbar"),
		h.Div(h.Class("gadget-status"), htmlx.Data("status", ""), g.Attr("hidden"), h.Aria("live", "polite")),
	)
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
// enables it. When the widget offers a choice of page sizes, the chooser leads
// the bar; the runtime keeps the page size it reports in sync with it.
func paginationNode(sizes []int, current int) g.Node {
	nodes := []g.Node{h.Class("gadget-pagination"), htmlx.Data("pagination", ""), g.Attr("hidden")}
	if len(sizes) > 0 {
		nodes = append(nodes, pageSizeNode(sizes, current))
	}
	nodes = append(nodes,
		h.Button(h.Type("button"), h.Class("gadget-btn"), htmlx.Data("page", "prev"), g.Text("Previous")),
		h.Span(h.Class("gadget-page-info"), htmlx.Data("page-info", "")),
		h.Button(h.Type("button"), h.Class("gadget-btn"), htmlx.Data("page", "next"), g.Text("Next")),
	)
	return h.Div(nodes...)
}

// pageSizeNode renders the per-page chooser. The runtime upgrades the select
// into a gadget dropdown, so the caption is a span rather than a label: it
// names the control for the eye, while the select carries the accessible name.
func pageSizeNode(sizes []int, current int) g.Node {
	opts := []g.Node{
		h.Class("gadget-input gadget-page-size-select"),
		htmlx.Data("page-size", ""),
		h.Aria("label", "Items per page"),
	}
	for _, n := range sizes {
		value := strconv.Itoa(n)
		attrs := []g.Node{h.Value(value), g.Text(value)}
		if n == current {
			attrs = append(attrs, h.Selected())
		}
		opts = append(opts, h.Option(attrs...))
	}
	return h.Div(h.Class("gadget-page-size"),
		h.Span(g.Text("Per page")),
		h.Select(opts...),
	)
}

// pageSizeOptions is the set of page sizes a pagination bar offers: the
// configured alternatives plus the current page size, deduplicated and
// ascending. Empty when the widget does not paginate or names no
// alternatives, in which case no chooser is rendered at all.
func pageSizeOptions(pageSize int, sizes []int) []int {
	if pageSize <= 0 || len(sizes) == 0 {
		return nil
	}
	seen := map[int]bool{}
	var out []int
	for _, n := range append(append([]int{}, sizes...), pageSize) {
		if n > 0 && !seen[n] {
			seen[n] = true
			out = append(out, n)
		}
	}
	sort.Ints(out)
	return out
}

// validatePageSizes checks the PageSizes field shared by the paginated
// widgets. ctx is the caller's error prefix.
func validatePageSizes(ctx string, pageSize int, sizes []int) error {
	if len(sizes) == 0 {
		return nil
	}
	if pageSize <= 0 {
		return fmt.Errorf("%s: PageSizes needs PageSize > 0", ctx)
	}
	for _, n := range sizes {
		if n <= 0 {
			return fmt.Errorf("%s: PageSizes entries must be > 0, got %d", ctx, n)
		}
	}
	return nil
}
