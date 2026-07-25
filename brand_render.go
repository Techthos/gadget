package gadget

import (
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gadget/internal/htmlx"
)

// brandNode renders the brand mark, or nil when there is nothing to show.
// Validation has already run by the time a document is assembled, so an
// unsafe logo is dropped here rather than half-rendered.
func brandNode(b *Brand) g.Node {
	if b == nil {
		return nil
	}
	var parts []g.Node
	if b.LogoSVG != "" {
		if svg, err := htmlx.RawSVG(b.LogoSVG); err == nil {
			parts = append(parts, h.Span(h.Class("gadget-brand-logo"), svg))
		}
	} else if b.LogoDataURI != "" && validateImageDataURI(b.LogoDataURI) == nil {
		alt := b.LogoAlt
		if alt == "" {
			alt = b.Name
		}
		parts = append(parts, h.Img(h.Class("gadget-brand-logo"), h.Src(b.LogoDataURI), h.Alt(alt)))
	}
	if b.Name != "" {
		parts = append(parts, h.Span(h.Class("gadget-brand-name"), g.Text(b.Name)))
	}
	if len(parts) == 0 {
		return nil
	}

	const class = "gadget-brand"

	// A link is a button, not an anchor: navigation is blocked in the host's
	// sandboxed iframe, so the runtime hands the URL to ui/openLink instead.
	if b.URL != "" {
		attrs := []g.Node{
			h.Type("button"),
			h.Class(class + " gadget-brand--link"),
			htmlx.Data("brand", b.URL),
			h.Title(b.Name),
		}
		return h.Button(append(attrs, parts...)...)
	}
	return h.Div(append([]g.Node{h.Class(class)}, parts...)...)
}
