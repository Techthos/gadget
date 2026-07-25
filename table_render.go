package gadget

import (
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gadget/internal/assets"
	"github.com/techthos/gadget/internal/htmlx"
)

// Document implements Widget. The rendered shell contains the table chrome
// only; row content is rendered by the embedded runtime from tool-result
// data (and from the optional baked InitialData snapshot).
func (t *Table) Document() (string, error) {
	if err := t.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(t.InitialData) > 0 {
		data = t.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(t.Title, "Table"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  t.Theme.CSS(),
		Body:      t.shell(),
		Config:    t.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (t *Table) shell() g.Node {
	return h.Div(h.Class("gadget-root"), htmlx.Data("widget", "table"),
		h.Div(h.Class("gadget-card"),
			t.toolbar(),
			statusNode(),
			h.Div(h.Class("gadget-table-wrap"),
				h.Table(h.Class("gadget-table"),
					h.THead(h.Tr(t.headerCells()...)),
					h.TBody(htmlx.Data("rows", "")),
				),
			),
			emptyStateNode(t.Empty),
			paginationNode(),
		),
	)
}

func (t *Table) toolbar() g.Node {
	var items []g.Node
	if brand := brandNode(t.Brand); brand != nil {
		items = append(items, brand)
	}
	if t.Title != "" {
		items = append(items, h.H2(h.Class("gadget-title"), g.Text(t.Title)))
	}
	if t.Filterable {
		items = append(items, h.Input(
			h.Type("search"),
			h.Class("gadget-input gadget-filter"),
			htmlx.Data("filter", ""),
			h.Placeholder("Filter…"),
			h.Aria("label", "Filter rows"),
		))
	}
	if t.Selection != nil && len(t.Selection.Bulk) > 0 {
		bulk := []g.Node{
			h.Class("gadget-bulk"),
			htmlx.Data("bulk", ""),
			g.Attr("hidden"),
			h.Span(h.Class("gadget-bulk-count"), htmlx.Data("bulk-count", "")),
		}
		for i, a := range t.Selection.Bulk {
			bulk = append(bulk, actionButton(a, "bulk-action", i))
		}
		items = append(items, h.Div(bulk...))
	}
	if len(items) == 0 {
		return nil
	}
	return h.Div(append([]g.Node{h.Class("gadget-toolbar")}, items...)...)
}

func (t *Table) headerCells() []g.Node {
	var cells []g.Node
	if t.Selection != nil {
		cells = append(cells, h.Th(h.Class("gadget-th-select"),
			h.Input(h.Type("checkbox"), htmlx.Data("select-all", ""), h.Aria("label", "Select all rows")),
		))
	}
	for _, c := range t.Columns {
		var attrs []g.Node
		if c.Width != "" {
			attrs = append(attrs, h.Style("width:"+c.Width))
		}
		if c.Align != "" {
			attrs = append(attrs, h.Class("gadget-align-"+string(c.Align)))
		}
		if c.sortable() {
			attrs = append(attrs,
				h.Aria("sort", "none"),
				htmlx.Data("sort", c.Key),
				h.Button(h.Type("button"), h.Class("gadget-sort-btn"), g.Text(c.Label)),
			)
		} else {
			attrs = append(attrs, g.Text(c.Label))
		}
		cells = append(cells, h.Th(attrs...))
	}
	return cells
}

// actionButton renders a server-side action button (bulk actions; per-row
// buttons are runtime-rendered from the config island).
func actionButton(a Action, attr string, idx int) g.Node {
	class := "gadget-btn"
	if a.Variant != "" {
		class += " gadget-btn--" + string(a.Variant)
	}
	return h.Button(
		h.Type("button"),
		h.Class(class),
		htmlx.Data(attr, strconv.Itoa(idx)),
		g.Text(a.Label),
	)
}

func docTitle(title, fallback string) string {
	if title != "" {
		return title
	}
	return fallback
}
