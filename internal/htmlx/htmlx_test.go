package htmlx

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	xhtml "golang.org/x/net/html"
	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

var update = flag.Bool("update", false, "update golden files")

func TestDocumentGolden(t *testing.T) {
	doc, err := Document(DocConfig{
		Title:    "Users",
		CSS:      ".gomu-root{--gomu-color-bg:#fff}",
		ThemeCSS: ".gomu-root{--gomu-color-primary:#0f62fe}",
		Body: h.Div(h.Class("gomu-root"), Data("widget", "table"),
			h.H1(g.Text("Users")),
		),
		Config:    map[string]any{"widget": "table", "rowsKey": "rows"},
		Data:      map[string]any{"rows": []map[string]any{{"id": 1, "name": "Ada"}}},
		RuntimeJS: `document.addEventListener("DOMContentLoaded",()=>{});`,
	})
	if err != nil {
		t.Fatal(err)
	}

	golden := filepath.Join("testdata", "golden", "doc_basic.html")
	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(golden, []byte(doc), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if doc != string(want) {
		t.Errorf("document does not match golden file %s\ngot:\n%s", golden, doc)
	}

	mustParse(t, doc)
}

func TestDocumentDefaults(t *testing.T) {
	doc, err := Document(DocConfig{Body: h.Div(h.Class("gomu-root"))})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<!doctype html>", `<html lang="en">`, `charset="utf-8"`} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "gomu-config") {
		t.Error("nil Config should not emit a config island")
	}
	mustParse(t, doc)
}

func TestJSONIslandHostilePayloads(t *testing.T) {
	hostile := map[string]any{
		"breakout": `</script><script>alert(1)</script>`,
		"comment":  `<!--<script>`,
		"lineseps": "a\u2028b\u2029c",
		"amp":      `a&b<c>d`,
	}
	doc, err := Document(DocConfig{
		Body:   h.Div(h.Class("gomu-root")),
		Config: map[string]any{"widget": "table"},
		Data:   hostile,
	})
	if err != nil {
		t.Fatal(err)
	}

	// No literal breakout sequences may survive serialization.
	lower := strings.ToLower(doc)
	if strings.Count(lower, "</script") != 2 { // exactly the two island closers
		t.Errorf("unexpected </script occurrences in:\n%s", doc)
	}
	if strings.Contains(lower, "<!--") {
		t.Errorf("comment-open leaked into document:\n%s", doc)
	}
	if strings.Contains(doc, "\u2028") || strings.Contains(doc, "\u2029") {
		t.Error("unescaped U+2028/U+2029 in document")
	}

	// The island must round-trip through an HTML parser + JSON decode.
	root := mustParse(t, doc)
	raw := textOfByID(root, DataIslandID)
	if raw == "" {
		t.Fatal("data island not found after HTML parse")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("island JSON does not decode: %v\nraw: %s", err, raw)
	}
	if !reflect.DeepEqual(got, hostile) {
		t.Errorf("island round-trip mismatch:\n got %#v\nwant %#v", got, hostile)
	}
}

func TestRawGuards(t *testing.T) {
	if _, err := RawCSS("body{}</style><script>x</script>"); err == nil {
		t.Error("RawCSS accepted </style> breakout")
	}
	if _, err := RawCSS("/* <!-- */"); err == nil {
		t.Error("RawCSS accepted comment-open")
	}
	if _, err := RawJS(`let s = "</SCRIPT>";`); err == nil {
		t.Error("RawJS accepted case-variant </script> breakout")
	}
	if _, err := RawJS("let x = 1;"); err != nil {
		t.Errorf("RawJS rejected safe content: %v", err)
	}
	if _, err := RawCSS(".a{color:red}"); err != nil {
		t.Errorf("RawCSS rejected safe content: %v", err)
	}
}

func TestRawSVGGuards(t *testing.T) {
	safe := map[string]string{
		"minimal":    `<svg viewBox="0 0 8 8"><circle cx="4" cy="4" r="3"/></svg>`,
		"whitespace": "\n  <svg><path d=\"M0 0h8v8H0z\"/></svg>\n",
		// "on" inside an attribute value or element name is not a handler.
		"on prefix":   `<svg><path id="onward" d="M0 0"/></svg>`,
		"uppercase":   `<SVG><CIRCLE r="1"/></SVG>`,
		"gradient":    `<svg><defs><linearGradient id="g"/></defs><rect fill="url(#g)"/></svg>`,
		"title child": `<svg><title>Acme</title><rect/></svg>`,
	}
	for name, svg := range safe {
		if _, err := RawSVG(svg); err != nil {
			t.Errorf("RawSVG rejected safe content %s: %v", name, err)
		}
	}

	unsafe := map[string]string{
		"script":            `<svg><script>alert(1)</script></svg>`,
		"handler":           `<svg onload="alert(1)"><rect/></svg>`,
		"spaced handler":    `<svg onclick = "alert(1)"><rect/></svg>`,
		"uppercase handler": `<svg ONLOAD="alert(1)"><rect/></svg>`,
		"foreign object":    `<svg><foreignObject><body/></foreignObject></svg>`,
		"use":               `<svg><use href="#x"/></svg>`,
		"javascript url":    `<svg><a href="javascript:alert(1)"><rect/></a></svg>`,
		"animate":           `<svg><rect><animate attributeName="x"/></rect></svg>`,
		"comment":           `<svg><!-- hi --><rect/></svg>`,
		"fragment":          `<rect/>`,
		"trailing markup":   `<svg><rect/></svg><img src=x>`,
		"leading markup":    `<img src=x><svg><rect/></svg>`,
		"empty":             ``,
	}
	for name, svg := range unsafe {
		if _, err := RawSVG(svg); err == nil {
			t.Errorf("RawSVG accepted unsafe content %s", name)
		}
	}
}

// mustParse parses doc with the standards-compliant HTML parser.
func mustParse(t *testing.T, doc string) *xhtml.Node {
	t.Helper()
	root, err := xhtml.Parse(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("document does not parse: %v", err)
	}
	return root
}

// textOfByID returns the concatenated text content of the element with the
// given id, or "".
func textOfByID(n *xhtml.Node, id string) string {
	if n.Type == xhtml.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "id" && a.Val == id {
				var sb strings.Builder
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == xhtml.TextNode {
						sb.WriteString(c.Data)
					}
				}
				return sb.String()
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if s := textOfByID(c, id); s != "" {
			return s
		}
	}
	return ""
}
