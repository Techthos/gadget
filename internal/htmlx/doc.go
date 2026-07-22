// Package htmlx assembles complete, self-contained MCP Apps HTML documents:
// inline stylesheet, widget shell markup, JSON data islands, and the inline
// runtime bundle. The output satisfies the spec's default locked-down CSP
// (default-src 'none' with inline script/style allowances) — it references
// nothing external.
package htmlx

import (
	"fmt"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// DocConfig describes one widget document.
type DocConfig struct {
	Title string
	Lang  string // defaults to "en"

	CSS      string // base stylesheet (assets.StylesCSS)
	ThemeCSS string // optional theme override block, emitted after CSS

	// Body is the SSR'd widget shell, rooted at an element carrying
	// class="gadget-root" and data-gadget-widget="<kind>".
	Body g.Node

	// Config is serialized into the #gadget-config island (required by the
	// runtime to mount a behavior).
	Config any

	// Data, when non-nil, is serialized into the optional #gadget-data
	// island as a baked structuredContent snapshot.
	Data any

	RuntimeJS string // runtime bundle (assets.RuntimeJS)
}

// Document renders the complete HTML document.
func Document(c DocConfig) (string, error) {
	lang := c.Lang
	if lang == "" {
		lang = "en"
	}

	head := []g.Node{
		h.Meta(h.Charset("utf-8")),
		h.Meta(h.Name("viewport"), h.Content("width=device-width, initial-scale=1")),
		h.TitleEl(g.Text(c.Title)),
	}
	if c.CSS != "" {
		style, err := RawCSS(c.CSS)
		if err != nil {
			return "", fmt.Errorf("stylesheet: %w", err)
		}
		head = append(head, style)
	}
	if c.ThemeCSS != "" {
		style, err := RawCSS(c.ThemeCSS)
		if err != nil {
			return "", fmt.Errorf("theme: %w", err)
		}
		head = append(head, style)
	}

	body := []g.Node{c.Body}
	if c.Config != nil {
		island, err := JSONIsland(ConfigIslandID, c.Config)
		if err != nil {
			return "", err
		}
		body = append(body, island)
	}
	if c.Data != nil {
		island, err := JSONIsland(DataIslandID, c.Data)
		if err != nil {
			return "", err
		}
		body = append(body, island)
	}
	if c.RuntimeJS != "" {
		script, err := RawJS(c.RuntimeJS)
		if err != nil {
			return "", fmt.Errorf("runtime: %w", err)
		}
		body = append(body, script)
	}

	doc := h.Doctype(
		h.HTML(h.Lang(lang),
			h.Head(head...),
			h.Body(body...),
		),
	)

	var sb strings.Builder
	if err := doc.Render(&sb); err != nil {
		return "", fmt.Errorf("htmlx: render document: %w", err)
	}
	return sb.String(), nil
}
