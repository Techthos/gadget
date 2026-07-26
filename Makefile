.PHONY: assets build clean harness preview inspect inspect-demo test test-go test-ui typecheck verify-dist vet

# Where make build puts the example binaries.
BIN_DIR ?= bin

# Preview/inspector knobs. Override on the command line, e.g.
#   make inspect PREVIEW_PORT=9000 INSPECTOR_PORT=7000
PREVIEW_PORT ?= 8081
PREVIEW_MODE ?= all
PREVIEW_URL ?= http://localhost:$(PREVIEW_PORT)/mcp
# Which server make inspect puts the inspector in front of. The demo speaks the
# same protocol with none of the preview's flags, so both parts are overridable:
#   make inspect INSPECT_PKG=./examples/demo INSPECT_FLAGS= PREVIEW_PORT=8080
# which is what make inspect-demo does.
INSPECT_PKG ?= ./examples/preview
INSPECT_FLAGS ?= -mode $(PREVIEW_MODE)
INSPECTOR_PKG ?= @modelcontextprotocol/inspector@latest
INSPECTOR_PORT ?= 6274
INSPECTOR_PROXY_PORT ?= 6277
# Set OPEN=0 to keep make inspect from opening a browser.
OPEN ?= 1

# Build the example servers into $(BIN_DIR). Go only: the TS/CSS bundle they
# embed is committed, so nothing here needs Node. Run make assets first if you
# changed anything under ui/.
build:
	go build -trimpath -o $(BIN_DIR)/ ./examples/...
	@ls -1 $(BIN_DIR)

clean:
	rm -rf $(BIN_DIR)

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
	echo "building $(INSPECT_PKG)..."; \
	go build -o $$dir/server $(INSPECT_PKG); \
	$$dir/server -addr :$(PREVIEW_PORT) $(INSPECT_FLAGS) & \
	preview=$$!; \
	tries=0; \
	until curl -s -o /dev/null --max-time 1 $(PREVIEW_URL); do \
		tries=$$((tries+1)); \
		if [ $$tries -gt 50 ]; then echo "server did not come up on :$(PREVIEW_PORT)"; exit 1; fi; \
		kill -0 $$preview 2>/dev/null || { echo "server exited"; exit 1; }; \
		sleep 0.2; \
	done; \
	token=$$(openssl rand -hex 16 2>/dev/null || echo gadget-preview); \
	url="http://localhost:$(INSPECTOR_PORT)/?transport=streamable-http&serverUrl=$(PREVIEW_URL)&MCP_PROXY_AUTH_TOKEN=$$token"; \
	echo; \
	echo "inspector:  $$url"; \
	echo "connect, then open the Apps tab and pick main_menu (or preview_index)."; \
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

# The same, in front of the small demo server (examples/demo) instead.
inspect-demo:
	@$(MAKE) inspect INSPECT_PKG=./examples/demo INSPECT_FLAGS= PREVIEW_PORT=8080

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
