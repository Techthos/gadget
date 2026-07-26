.PHONY: assets harness preview inspect test test-go test-ui typecheck verify-dist vet

# Preview/inspector knobs. Override on the command line, e.g.
#   make inspect PREVIEW_PORT=9000 INSPECTOR_PORT=7000
PREVIEW_PORT ?= 8081
PREVIEW_MODE ?= all
PREVIEW_URL ?= http://localhost:$(PREVIEW_PORT)/mcp
INSPECTOR_PKG ?= @modelcontextprotocol/inspector@latest
INSPECTOR_PORT ?= 6274
INSPECTOR_PROXY_PORT ?= 6277
# Set OPEN=0 to keep make inspect from opening a browser.
OPEN ?= 1

# Build the TS/CSS bundle into internal/assets/dist (committed, go:embed'd).
assets:
	npm --prefix ui ci
	npm --prefix ui run build

# Serve the fake MCP Apps host for manual widget preview at localhost:8090.
harness:
	go run ./examples/harness

# Serve the preview MCP server on its own. See docs/preview.md.
preview:
	go run ./examples/preview -addr :$(PREVIEW_PORT) -mode $(PREVIEW_MODE)

# Serve the preview MCP server with the MCP Inspector in front of it, already
# pointed at the endpoint. Ctrl-C stops both.
#
# The server runs as a built binary rather than `go run`, so stopping the make
# recipe actually stops the listener instead of orphaning the child process.
inspect:
	@command -v npx >/dev/null 2>&1 || { echo "make inspect needs npx (Node 18+)"; exit 1; }
	@set -e; \
	dir=$$(mktemp -d); \
	trap 'kill $$preview 2>/dev/null; rm -rf $$dir' EXIT INT TERM; \
	echo "building the preview server..."; \
	go build -o $$dir/preview ./examples/preview; \
	$$dir/preview -addr :$(PREVIEW_PORT) -mode $(PREVIEW_MODE) & \
	preview=$$!; \
	tries=0; \
	until curl -s -o /dev/null --max-time 1 $(PREVIEW_URL); do \
		tries=$$((tries+1)); \
		if [ $$tries -gt 50 ]; then echo "preview server did not come up on :$(PREVIEW_PORT)"; exit 1; fi; \
		kill -0 $$preview 2>/dev/null || { echo "preview server exited"; exit 1; }; \
		sleep 0.2; \
	done; \
	token=$$(openssl rand -hex 16 2>/dev/null || echo gadget-preview); \
	url="http://localhost:$(INSPECTOR_PORT)/?transport=streamable-http&serverUrl=$(PREVIEW_URL)&MCP_PROXY_AUTH_TOKEN=$$token"; \
	echo; \
	echo "inspector:  $$url"; \
	echo "connect, then open the Apps tab and pick main_menu or preview_index."; \
	echo; \
	if [ "$(OPEN)" = "1" ] && command -v xdg-open >/dev/null 2>&1; then \
		( tries=0; \
		  until curl -s -o /dev/null --max-time 1 http://localhost:$(INSPECTOR_PORT); do \
			tries=$$((tries+1)); [ $$tries -gt 100 ] && exit 0; sleep 0.3; \
		  done; \
		  xdg-open "$$url" >/dev/null 2>&1 ) & \
	fi; \
	MCP_PROXY_AUTH_TOKEN=$$token \
	CLIENT_PORT=$(INSPECTOR_PORT) \
	SERVER_PORT=$(INSPECTOR_PROXY_PORT) \
	MCP_AUTO_OPEN_ENABLED=false \
	npx -y $(INSPECTOR_PKG)

test: test-go test-ui

test-go:
	go test ./...

test-ui:
	npm --prefix ui run test

typecheck:
	npm --prefix ui run typecheck

vet:
	go vet ./...

# Fails if committed dist does not match a fresh build of ui/ sources.
verify-dist: assets
	git diff --exit-code internal/assets/dist
