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
// Flags:
//
//	-addr   HTTP listen address (default :8081)
//	-mode   which tools to register: all, scenario or gallery (default all)
//	-quiet  do not log tool calls to stderr
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/techthos/gadget/gosdk"
	"github.com/techthos/gadget/uispec"
)

func main() {
	addr := flag.String("addr", ":8081", "HTTP listen address")
	stdio := flag.Bool("stdio", false, "serve over stdio instead of HTTP")
	mode := flag.String("mode", "all", "which tools to register: all, scenario or gallery")
	quiet := flag.Bool("quiet", false, "do not log tool calls to stderr")
	flag.Parse()

	scenario := *mode == "all" || *mode == "scenario"
	gallery := *mode == "all" || *mode == "gallery"
	if !scenario && !gallery {
		log.Fatalf("preview: unknown -mode %q (want all, scenario or gallery)", *mode)
	}

	server := newServer(scenario, gallery, !*quiet)

	if *stdio {
		// Logging goes to stderr, which stdio hosts keep clear of the
		// protocol stream on stdout.
		if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatal(err)
		}
		return
	}

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	log.Printf("gadget preview server on http://localhost%s/mcp (mode %s, spec %s)", *addr, *mode, uispec.SpecVersion)
	log.Printf("inspector: npx @modelcontextprotocol/inspector, then connect over Streamable HTTP to that URL")
	log.Fatal(http.ListenAndServe(*addr, mux))
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
