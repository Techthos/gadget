// Command harness serves a fake MCP Apps host for manually smoke-testing
// gadget widgets without a real host. It renders a catalog of widget stories
// (one route per story, baked data) and embeds them in an iframe behind a
// JSON-RPC postMessage host that answers the handshake, replies to tool
// calls and logs all traffic.
//
//	go run ./examples/harness
//	open http://localhost:8090
package main

import (
	_ "embed"
	"encoding/json"
	"flag"
	"log"
	"net/http"

	"github.com/techthos/gadget"
)

//go:embed host.html
var hostPage []byte

func widgetHandler(w gadget.Widget) http.HandlerFunc {
	return func(rw http.ResponseWriter, _ *http.Request) {
		doc, err := w.Document()
		if err != nil {
			http.Error(rw, err.Error(), http.StatusInternalServerError)
			return
		}
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.Header().Set("Cache-Control", "no-store")
		_, _ = rw.Write([]byte(doc))
	}
}

func main() {
	addr := flag.String("addr", ":8090", "listen address")
	flag.Parse()

	list := stories()
	byID := make(map[string]story, len(list))
	for _, s := range list {
		byID[s.ID] = s
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
		rw.Header().Set("Cache-Control", "no-store")
		_, _ = rw.Write(hostPage)
	})
	mux.HandleFunc("GET /stories.json", func(rw http.ResponseWriter, _ *http.Request) {
		rw.Header().Set("Content-Type", "application/json; charset=utf-8")
		rw.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(rw).Encode(list)
	})
	mux.HandleFunc("GET /story/{id}", func(rw http.ResponseWriter, r *http.Request) {
		s, ok := byID[r.PathValue("id")]
		if !ok {
			http.NotFound(rw, r)
			return
		}
		widgetHandler(s.build())(rw, r)
	})

	log.Printf("gadget harness on http://localhost%s (%d stories)", *addr, len(list))
	log.Fatal(http.ListenAndServe(*addr, mux))
}
