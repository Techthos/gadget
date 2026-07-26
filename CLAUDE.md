# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`gomukit` is a Go library of prebuilt interactive HTML widgets (Table, Form) for MCP Apps (`io.modelcontextprotocol/ui`, spec `2026-01-26`). Widgets render as fully self-contained HTML documents (inline CSS/JS, satisfies the spec's locked-down CSP) served as `ui://` template resources from a Go MCP server. Pre-release; APIs unstable.

## Commands

```sh
make test         # go test ./... + vitest
make test-go      # Go tests only
make test-ui      # vitest only (npm --prefix ui run test)
make typecheck    # tsc --noEmit
make vet          # go vet ./...
make assets       # npm ci + rebuild the TS/CSS bundle into internal/assets/dist
make verify-dist  # rebuild assets, fail if committed dist drifted (CI does this)
make build        # build the example servers into ./bin (BIN_DIR=… to move it)
make clean        # remove ./bin
make inspect      # preview server + MCP Inspector, already connected
make inspect-demo # the same, in front of examples/demo on :8080
make screenshots  # rescreenshot every widget story into docs/assets
```

- Single Go test: `go test ./ -run TestName` (or `go test ./internal/htmlx -run TestName`, etc.)
- Single UI test file: `npm --prefix ui run test -- test/table.test.ts`
- Golden files: `go test ./ -update` and `go test ./internal/htmlx -update` regenerate `testdata/golden/`.

**After editing anything under `ui/` (src or css), run `make assets` and commit the resulting `internal/assets/dist/` changes** — the built bundle is committed and `go:embed`-ed so Go consumers never need Node; CI fails on drift.

## Architecture

Full details in `docs/architecture.md` (plus `docs/widgets.md`, `docs/theming.md`). The big picture:

### Split rendering model

The MCP Apps spec's template model means the HTML resource cannot contain per-call data — data arrives at runtime via `ui/notifications/tool-result`. So rendering is split:

- **Go renders structure** (registration time): widget shell, table chrome, form fields, plus a `#gomu-config` JSON island describing columns/fields/action bindings (and an optional `#gomu-data` snapshot).
- **TypeScript renders data** (runtime, in the host's sandboxed iframe): rows, prefill, errors — from the snapshot, then from every tool-result notification or widget-initiated `tools/call` response. All cell/formatting logic lives only in TS (`Intl` with host locale/timeZone).

### Layers

| Package | Role |
|---|---|
| `gomukit` (root) | Widget definitions (`Table`, `Form`, actions) + SSR shells via gomponents |
| `theme` | `Theme` struct → CSS token-override block |
| `uispec` | Spec constants and `_meta` types (no deps) |
| `gosdk` | The **only** package importing an MCP SDK (`modelcontextprotocol/go-sdk`); core works with any Go MCP implementation |
| `internal/htmlx` | Document assembler, JSON islands, raw-content guards |
| `internal/assets` | `go:embed` of the built JS/CSS bundle |
| `ui/` | Node workspace (esbuild/TypeScript/vitest, dev-only): bridge, state store + pure reducers, per-widget behaviors, CSS |

Keep this dependency discipline: core packages emit plain spec-shaped values; never import an MCP SDK outside `gosdk`.

### Shared spec constants

JSON-RPC method names live in `ui/src/spec-constants.json`, consumed by TS and mirrored in Go's `uispec` — cross-checked by tests. A spec rename is a one-line change; keep both sides in sync.

### Runtime (`ui/src`)

- `bridge.ts` — JSON-RPC 2.0 over `postMessage`: request correlation/timeouts, `ui/initialize` handshake, `tools/call`, size-changed reporting. Only host→view methods are accepted inbound.
- `host.ts` — applies `hostContext` (style vars, theme, fonts, locale/timeZone); `ResizeObserver` → size-changed notifications.
- `state.ts` — store + pure reducers (sort/filter/pagination/selection).
- `dropdown.ts` — upgrades every `<select>` into the gomukit dropdown; the
  select stays in the DOM as the value holder.
- `widgets/*.ts` — event-delegated behaviors on `data-gomu-*` attributes.

## Security invariants (by construction — do not break)

- Data reaches HTML **only** via gomponents text nodes (Go) or `textContent` (TS). Never build DOM from data with innerHTML.
- JSON islands rely on `encoding/json`'s HTML-safe escaping; `RawCSS`/`RawJS` refuse `</style`/`</script`/`<!--`; `Theme` values are validated against CSS breakout.
- Documents must stay fully self-contained: no external URLs, CDNs, or files on disk (must satisfy `default-src 'none'` + inline allowances).
- Confirmations use inline two-phase buttons — native `confirm()` doesn't work in sandboxed iframes.

## Examples / manual testing

- `examples/demo` — complete MCP server (streamable HTTP or `-stdio`) at `http://localhost:8080/mcp`.
- `examples/preview` — the widest MCP server, for driving from an MCP Apps capable inspector: `make preview`, endpoint `http://localhost:8081/mcp`. A scenario half (Acme Dispatch, mutable state, `scenario.go`) and a gallery half (one tool + resource per widget variant, `gallery.go`); `-mode` picks one or both, tool calls log to stderr. `examples/preview/preview_test.go` reads every resource and walks the app over an in-memory session, so a broken widget config fails `make test-go`. Details in `docs/preview.md`.
- `scripts/screenshots.mjs` — `make screenshots` starts its own harness, drives an installed Chrome over the DevTools protocol (no npm dependency, no download) and writes `docs/assets/preview/<story>[-dark].png` for every story plus the images the README embeds. Flags via `SHOT_FLAGS`, e.g. `make screenshots SHOT_FLAGS="--only table"`; unchanged bytes are not rewritten. Render widths per story live in the `WIDTHS` table at the top of the script.
- `examples/harness` — fake MCP Apps host in one HTML page with a story browser: `go run ./examples/harness`, open `http://localhost:8090`. Stories (widget variants) live in `examples/harness/stories.go` and are served at `/story/<id>`; the page renders the selected one in a sandboxed iframe, answers the handshake, logs traffic, follows size-changed, and simulates tool results/errors and theme changes. Stories render frameless (`theme.Transparent`) by default; the top bar's "Frameless" toggle switches to the framed variant (`?transparent=0`).
