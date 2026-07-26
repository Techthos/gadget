package htmlx

import (
	"encoding/json"
	"fmt"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"
)

// Island element IDs the TypeScript runtime looks up on boot.
const (
	// ConfigIslandID holds the widget's static spec (columns, fields,
	// action bindings).
	ConfigIslandID = "gomu-config"
	// DataIslandID optionally holds a baked structuredContent snapshot for
	// instant first paint before any tool-result notification arrives.
	DataIslandID = "gomu-data"
)

// JSONIsland renders v as a <script type="application/json" id=...> data
// island. encoding/json's default HTML-safe encoding escapes '<', '>', '&'
// (to < etc.) and U+2028/U+2029, which prevents </script> breakout and
// JS line-terminator issues by construction.
func JSONIsland(id string, v any) (g.Node, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("htmlx: marshal island %q: %w", id, err)
	}
	return h.Script(h.Type("application/json"), h.ID(id), g.Raw(string(b))), nil
}
