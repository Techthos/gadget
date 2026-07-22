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
		CSS:      ".gadget-root{--gadget-color-bg:#fff}",
		ThemeCSS: ".gadget-root{--gadget-color-primary:#0f62fe}",
		Body: h.Div(h.Class("gadget-root"), Data("widget", "table"),
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
	doc, err := Document(DocConfig{Body: h.Div(h.Class("gadget-root"))})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"<!doctype html>", `<html lang="en">`, `charset="utf-8"`} {
		if !strings.Contains(doc, want) {
			t.Errorf("document missing %q:\n%s", want, doc)
		}
	}
	if strings.Contains(doc, "gadget-config") {
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
		Body:   h.Div(h.Class("gadget-root")),
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
