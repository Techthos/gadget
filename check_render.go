package gadget

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// The two marks a checkbox can show, drawn in a 16x16 viewBox. Mirrored in
// ui/src/dom.ts, which builds the same node at runtime.
const (
	checkTickPath = "M4.25 8.5 6.75 11l5-6"
	checkDashPath = "M4.5 8h7"
)

// checkboxNode renders a checkbox as a token-styled box with an inline <svg>
// mark (see ui/css/check.css). attrs go on the <input>, which keeps every
// native behaviour — the CSS only strips its appearance.
func checkboxNode(attrs ...g.Node) g.Node {
	input := append([]g.Node{h.Type("checkbox")}, attrs...)
	return h.Span(h.Class("gadget-check"),
		h.Input(input...),
		checkIconNode(),
	)
}

// checkIconNode is the inline mark. It is markup rather than a background
// image because a data: URI would depend on the host allowing img-src data:,
// which the MCP Apps CSP does not guarantee.
func checkIconNode() g.Node {
	return g.El("svg",
		g.Attr("class", "gadget-check-icon"),
		g.Attr("viewBox", "0 0 16 16"),
		g.Attr("fill", "none"),
		g.Attr("stroke", "currentColor"),
		g.Attr("stroke-width", "2"),
		g.Attr("stroke-linecap", "round"),
		g.Attr("stroke-linejoin", "round"),
		g.Attr("aria-hidden", "true"),
		g.El("path", g.Attr("class", "gadget-check-tick"), g.Attr("d", checkTickPath)),
		g.El("path", g.Attr("class", "gadget-check-dash"), g.Attr("d", checkDashPath)),
	)
}
