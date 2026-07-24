package gadget

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gadget/internal/assets"
	"github.com/techthos/gadget/internal/htmlx"
)

// --- Card (single record) ---

// Document implements Widget. The shell contains the card chrome only; the
// record's title/subtitle/fields are rendered by the embedded runtime from
// tool-result data (and the optional baked InitialData snapshot).
func (c *Card) Document() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(c.InitialData) > 0 {
		data = c.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(c.Title, "Card"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  c.Theme.CSS(),
		Body:      c.shell(),
		Config:    c.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (c *Card) shell() g.Node {
	var chrome []g.Node
	chrome = append(chrome, h.Class("gadget-card"))
	if c.Title != "" {
		chrome = append(chrome, h.Div(h.Class("gadget-toolbar"),
			h.H2(h.Class("gadget-title"), g.Text(c.Title)),
		))
	}
	chrome = append(chrome,
		statusNode(),
		// The runtime renders one card element into this host.
		h.Div(h.Class("gadget-card-host"), htmlx.Data("card", "")),
		emptyStateNode(c.Empty),
	)
	return h.Div(h.Class("gadget-root"), htmlx.Data("widget", "card"),
		h.Div(chrome...),
	)
}

// --- CardList (collection) ---

// Document implements Widget. The shell contains the list chrome (toolbar,
// filter, sort, selection, pagination); the cards themselves are rendered by
// the embedded runtime from tool-result data and the optional snapshot.
func (l *CardList) Document() (string, error) {
	if err := l.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(l.InitialData) > 0 {
		data = l.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(l.Title, "Cards"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  l.Theme.CSS(),
		Body:      l.shell(),
		Config:    l.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (l *CardList) shell() g.Node {
	return h.Div(h.Class("gadget-root"), htmlx.Data("widget", "cardlist"),
		h.Div(h.Class("gadget-card"),
			l.toolbar(),
			statusNode(),
			h.Div(h.Class("gadget-card-grid"), htmlx.Data("cards", "")),
			emptyStateNode(l.Empty),
			paginationNode(),
		),
	)
}

func (l *CardList) toolbar() g.Node {
	var items []g.Node
	if l.Title != "" {
		items = append(items, h.H2(h.Class("gadget-title"), g.Text(l.Title)))
	}
	if l.Selection != nil {
		items = append(items, h.Label(h.Class("gadget-cards-selectall"),
			h.Input(h.Type("checkbox"), htmlx.Data("select-all", ""), h.Aria("label", "Select all cards")),
			g.Text("Select all"),
		))
	}
	if l.Filterable {
		items = append(items, h.Input(
			h.Type("search"),
			h.Class("gadget-input gadget-filter"),
			htmlx.Data("filter", ""),
			h.Placeholder("Filter…"),
			h.Aria("label", "Filter cards"),
		))
	}
	if sort := l.sortControl(); sort != nil {
		items = append(items, sort)
	}
	if l.Selection != nil && len(l.Selection.Bulk) > 0 {
		bulk := []g.Node{
			h.Class("gadget-bulk"),
			htmlx.Data("bulk", ""),
			g.Attr("hidden"),
			h.Span(h.Class("gadget-bulk-count"), htmlx.Data("bulk-count", "")),
		}
		for i, a := range l.Selection.Bulk {
			bulk = append(bulk, actionButton(a, "bulk-action", i))
		}
		items = append(items, h.Div(bulk...))
	}
	if len(items) == 0 {
		return nil
	}
	return h.Div(append([]g.Node{h.Class("gadget-toolbar")}, items...)...)
}

// sortControl renders a select over the template's sortable fields; the
// runtime keeps its value in sync with the current sort and reads changes.
// Option values are "<key>|asc" / "<key>|desc".
func (l *CardList) sortControl() g.Node {
	opts := l.Template.sortOptions()
	if len(opts) == 0 {
		return nil
	}
	children := []g.Node{
		h.Class("gadget-input gadget-sort-select"),
		htmlx.Data("sort-select", ""),
		h.Aria("label", "Sort cards"),
		h.Option(h.Value(""), g.Text("Sort…")),
	}
	for _, o := range opts {
		key, _ := o["key"].(string)
		label, _ := o["label"].(string)
		children = append(children,
			h.Option(h.Value(key+"|asc"), g.Text(label+" ↑")),
			h.Option(h.Value(key+"|desc"), g.Text(label+" ↓")),
		)
	}
	return h.Select(children...)
}
