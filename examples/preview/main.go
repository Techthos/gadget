// Command preview is a runnable MCP server built for inspecting gadget: it
// exposes every widget the library renders as real MCP tools and ui://
// resources, so an MCP Apps capable inspector drives them over the wire
// instead of through a fake host page.
//
// It has two halves. The scenario is a small application, Acme Dispatch, with
// mutable state: customers and orders, listed, edited, confirmed and shipped
// through the widgets, where an action taken in one view shows up in the
// next. The gallery is a catalog: one tool per widget variant, each with its
// own resource, covering the renderings the scenario has no room for.
//
// Run with streamable HTTP (default), then point the inspector at
// http://localhost:8081/mcp:
//
//	go run ./examples/preview
//
// or over stdio, for hosts that spawn the server themselves:
//
//	go run ./examples/preview -stdio
//
// Exposed publicly it should run with -sandbox, which gives every MCP session
// its own store: visitors still exercise the writing tools in full, but no
// one's edits reach anyone else's widgets.
//
// Flags:
//
//	-addr             HTTP listen address (default :8081)
//	-mode             which tools to register: all, scenario or gallery (default all)
//	-quiet            do not log tool calls to stderr
//	-sandbox          give every session its own scenario store
//	-session-timeout  close idle sessions after this long (0 = never)
//	-cors             answer preflights and allow any origin
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/techthos/gadget/gosdk"
	"github.com/techthos/gadget/uispec"
)

func main() {
	addr := flag.String("addr", ":8081", "HTTP listen address")
	stdio := flag.Bool("stdio", false, "serve over stdio instead of HTTP")
	mode := flag.String("mode", "all", "which tools to register: all, scenario or gallery")
	quiet := flag.Bool("quiet", false, "do not log tool calls to stderr")
	sandbox := flag.Bool("sandbox", false, "give every session its own scenario store")
	sessionTimeout := flag.Duration("session-timeout", 0, "close sessions idle for this long (0 = never)")
	cors := flag.Bool("cors", false, "answer CORS preflights and allow any origin")
	behindProxy := flag.Bool("behind-proxy", false, "accept non-localhost Host headers on a loopback listener (needed behind a reverse proxy on the same host)")
	flag.Parse()

	scenario := *mode == "all" || *mode == "scenario"
	gallery := *mode == "all" || *mode == "gallery"
	if !scenario && !gallery {
		log.Fatalf("preview: unknown -mode %q (want all, scenario or gallery)", *mode)
	}

	if *stdio {
		// Logging goes to stderr, which stdio hosts keep clear of the
		// protocol stream on stdout.
		if err := newServer(scenario, gallery, !*quiet).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
		return
	}

	// getServer runs once per new session. Sharing one server (and so one
	// store) is what you want locally; -sandbox builds a fresh one instead,
	// so a public deployment hands every visitor their own copy of the
	// scenario data to mutate.
	getServer := func(*http.Request) *mcp.Server { return newServer(scenario, gallery, !*quiet) }
	if !*sandbox {
		shared := newServer(scenario, gallery, !*quiet)
		getServer = func(*http.Request) *mcp.Server { return shared }
	}

	handler := mcp.NewStreamableHTTPHandler(getServer, &mcp.StreamableHTTPOptions{
		// Sandboxed stores live for as long as their session does, so a
		// public deployment needs idle ones reaped.
		SessionTimeout: *sessionTimeout,
		// The SDK rejects a non-localhost Host on a loopback listener as a
		// DNS rebinding attempt. That is the right default for a server a
		// user runs locally, and wrong for one a reverse proxy on the same
		// host forwards to.
		DisableLocalhostProtection: *behindProxy,
	})
	mux := http.NewServeMux()
	mux.Handle("/mcp", withCORS(handler, *cors))
	// Something for a proxy or uptime probe to hit: the image is scratch, so
	// there is no shell to run a container-side healthcheck in.
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok\n"))
	})

	log.Printf("gadget preview server on http://localhost%s/mcp (mode %s, spec %s, sandbox %t)", *addr, *mode, uispec.SpecVersion, *sandbox)
	log.Printf("inspector: npx @modelcontextprotocol/inspector, then connect over Streamable HTTP to that URL")
	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(srv.ListenAndServe())
}

// withCORS lets browser-based MCP clients reach the endpoint from another
// origin. Off unless asked for: it only matters when the server is published,
// and everything it exposes is a demo fixture with no credentials attached.
func withCORS(next http.Handler, on bool) http.Handler {
	if !on {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		h.Set("Access-Control-Allow-Headers", "Content-Type, Accept, Authorization, Last-Event-ID, Mcp-Session-Id, MCP-Protocol-Version")
		// Clients read the session id off the initialize response, and
		// cross-origin JS cannot see a header that is not exposed.
		h.Set("Access-Control-Expose-Headers", "Mcp-Session-Id")
		h.Set("Access-Control-Max-Age", "86400")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func newServer(scenario, gallery, logCalls bool) *mcp.Server {
	server := mcp.NewServer(
		&mcp.Implementation{Name: "gadget-preview", Version: "0.1.0"},
		gosdk.EnableUI(nil),
	)
	if logCalls {
		server.AddReceivingMiddleware(logMiddleware)
	}
	if scenario {
		registerScenario(server, newStore(), gallery)
	}
	if gallery {
		registerGallery(server)
	}
	return server
}

// logMiddleware prints every inbound request to stderr. Widget-initiated
// calls arrive here too, so the log shows which tool a button fired and with
// what arguments, which is the part an inspector's own log usually omits.
func logMiddleware(next mcp.MethodHandler) mcp.MethodHandler {
	return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
		detail := ""
		switch r := req.(type) {
		case *mcp.CallToolRequest:
			detail = " " + r.Params.Name + " " + compact(string(r.Params.Arguments))
		case *mcp.ReadResourceRequest:
			detail = " " + r.Params.URI
		}
		res, err := next(ctx, method, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s%s: %v\n", method, detail, err)
		} else {
			fmt.Fprintf(os.Stderr, "%s%s\n", method, detail)
		}
		return res, err
	}
}

// compact keeps the argument line to one readable row.
func compact(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 160 {
		return s[:157] + "..."
	}
	return s
}
