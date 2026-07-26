package htmlx

import (
	"fmt"
	"strings"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// RawCSS wraps trusted CSS (the built stylesheet, a validated theme block)
// in a <style> element. It refuses content that could close the element or
// open an HTML comment, as a backstop against injection through upstream
// bugs — legitimate stylesheet content never contains these sequences.
func RawCSS(css string) (g.Node, error) {
	if err := checkRaw(css, "</style"); err != nil {
		return nil, fmt.Errorf("htmlx: unsafe CSS: %w", err)
	}
	return h.StyleEl(g.Raw(css)), nil
}

// RawJS wraps the trusted runtime bundle in a <script> element, with the
// same backstop rules as RawCSS.
func RawJS(js string) (g.Node, error) {
	if err := checkRaw(js, "</script"); err != nil {
		return nil, fmt.Errorf("htmlx: unsafe JS: %w", err)
	}
	return h.Script(g.Raw(js)), nil
}

// svgDenied are constructs an inline logo has no legitimate use for, but
// which would introduce script execution, network access, or an escape from
// the SVG subtree. Checked case-insensitively against the whole document.
var svgDenied = []string{
	"<script", "<foreignobject", "<iframe", "<embed", "<object", "<use",
	"<animate", "<set", "<handler", "javascript:", "<!--", "</style",
}

// RawSVG wraps a trusted inline SVG logo as markup. The caller supplies it at
// registration time (it is author input, the same trust level as RawCSS and
// RawJS), so this is a backstop, not a sanitizer: it requires a single <svg>
// root and refuses script-bearing or resource-loading constructs.
func RawSVG(svg string) (g.Node, error) {
	s := strings.TrimSpace(svg)
	lower := strings.ToLower(s)
	if !strings.HasPrefix(lower, "<svg") || !strings.HasSuffix(lower, "</svg>") {
		return nil, fmt.Errorf("htmlx: unsafe SVG: content must be a single <svg> element")
	}
	for _, bad := range svgDenied {
		if strings.Contains(lower, bad) {
			return nil, fmt.Errorf("htmlx: unsafe SVG: content contains %q", bad)
		}
	}
	if name := eventHandlerAttr(lower); name != "" {
		return nil, fmt.Errorf("htmlx: unsafe SVG: content contains event handler %q", name)
	}
	return g.Raw(s), nil
}

// eventHandlerAttr reports the first on*= attribute found in an already
// lowercased document, or "" when there is none. An attribute name is
// preceded by whitespace and followed (allowing spaces) by "=".
func eventHandlerAttr(lower string) string {
	for i := 0; i+2 < len(lower); i++ {
		if lower[i] != 'o' || lower[i+1] != 'n' {
			continue
		}
		if i > 0 && !isSpace(lower[i-1]) {
			continue
		}
		j := i + 2
		for j < len(lower) && lower[j] >= 'a' && lower[j] <= 'z' {
			j++
		}
		if j == i+2 {
			continue
		}
		k := j
		for k < len(lower) && isSpace(lower[k]) {
			k++
		}
		if k < len(lower) && lower[k] == '=' {
			return lower[i:j]
		}
	}
	return ""
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '\f'
}

func checkRaw(s, closer string) error {
	lower := strings.ToLower(s)
	if strings.Contains(lower, closer) {
		return fmt.Errorf("content contains %q", closer)
	}
	if strings.Contains(lower, "<!--") {
		return fmt.Errorf("content contains %q", "<!--")
	}
	return nil
}

// Data returns a data-gomu-* attribute, the hook the runtime's event
// delegation dispatches on.
func Data(name, value string) g.Node {
	return g.Attr("data-gomu-"+name, value)
}
