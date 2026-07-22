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

// Data returns a data-gadget-* attribute, the hook the runtime's event
// delegation dispatches on.
func Data(name, value string) g.Node {
	return g.Attr("data-gadget-"+name, value)
}
