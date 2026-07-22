# Architecture

gadget targets the **MCP Apps extension** (`io.modelcontextprotocol/ui`,
spec `2026-01-26`): a server predeclares HTML template resources at `ui://`
URIs (`text/html;profile=mcp-app`), tools link to them via
`_meta.ui.resourceUri`, and hosts render the template in a sandboxed iframe,
delivering per-call data at runtime.

## Layers

| Package | Role | Depends on |
|---|---|---|
| `gadget` | Widget definitions (`Table`, `Form`, actions) + SSR shells | gomponents, internal/* |
| `gadget/theme` | Design-token overrides (`Theme` → CSS block) | — |
| `gadget/uispec` | Spec constants and `_meta` types | — |
| `gadget/gosdk` | Adapter for `modelcontextprotocol/go-sdk` | the SDK |
| `internal/htmlx` | Document assembler, JSON islands, raw-content guards | gomponents |
| `internal/assets` | `go:embed` of the built JS/CSS bundle | — |
| `ui/` (TS/CSS workspace) | Runtime bundle: bridge, behaviors, stylesheet | — (built with esbuild) |

The core emits plain spec-shaped values; only `gosdk` imports an MCP SDK, so
gadget works with any Go MCP implementation.

## Rendering model: template shell + runtime data render

The spec's template model means the resource document cannot contain
per-call data — data arrives via `ui/notifications/tool-result`
(`structuredContent`). gadget therefore splits rendering:

- **Go (build time / registration time)** renders *structure*: table chrome,
  toolbar, headers, form fields with native validation attributes, empty and
  status regions — plus a `#gadget-config` JSON island describing columns,
  fields, and action bindings.
- **TypeScript (runtime)** renders *data*: rows, prefill values, errors —
  from the optional baked `#gadget-data` snapshot (`InitialData`), then from
  every tool-result notification or widget-initiated `tools/call` response.
  Formatting uses `Intl` with the host's locale and time zone.

One data-rendering path means no duplicated cell/formatting logic between
languages.

## Document anatomy

```
<!doctype html>
<head>
  <style>   bundled tokens + widget CSS (internal/assets/dist/gadget.css)
  <style>   optional Theme override block
<body>
  <div class="gadget-root" data-gadget-widget="table">…shell…</div>
  <script type="application/json" id="gadget-config">…widget spec…</script>
  <script type="application/json" id="gadget-data">…optional snapshot…</script>
  <script>  runtime IIFE (internal/assets/dist/gadget.js)
```

Everything is inline; the document satisfies the spec's default CSP
(`default-src 'none'` with inline script/style allowances) with no
declarations needed.

## Runtime (ui/src)

- `bridge.ts` — hand-rolled JSON-RPC 2.0 over `postMessage`: request
  correlation with timeouts, notification handlers, the `ui/initialize`
  handshake, `tools/call` / `ui/open-link` helpers, size-changed reporting.
  Direction discipline (only host→view methods are accepted inbound) makes
  it robust even when host and view share one window (tests).
- `host.ts` — applies `hostContext`: style variables onto the root element,
  `data-gadget-theme`, host fonts, locale/timeZone; watches content size via
  `ResizeObserver` → `ui/notifications/size-changed`.
- `state.ts` — store + pure reducers (sort/filter/pagination/selection).
- `widgets/*.ts` — per-widget behaviors, event-delegated on
  `data-gadget-*` attributes; DOM built via `textContent` only (no
  innerHTML with data — XSS-safe by construction).
- Method names come from `spec-constants.json`, shared with Go's `uispec`
  (cross-checked by tests) so spec renames are one-line changes.

## Asset pipeline

`ui/` is a Node workspace (esbuild, TypeScript, vitest — dev-time only).
`npm run build` bundles `src/index.ts` → `internal/assets/dist/gadget.js`
and `css/index.css` → `dist/gadget.css` (minified IIFE, es2020). The dist
files are **committed** and embedded via `go:embed`, so `go get` consumers
never need Node. CI rebuilds and fails on drift (`make verify-dist`).

## Security notes

- Data → HTML only via gomponents text nodes (Go) or `textContent` (TS).
- JSON islands rely on `encoding/json`'s HTML-safe escaping (`<`, `>`, `&`,
  U+2028/9) — `</script>` breakout is impossible.
- `RawCSS`/`RawJS` refuse `</style`/`</script`/`<!--` as a backstop for the
  trusted bundle; `Theme` values are validated against CSS breakout.
- Confirmations are inline two-phase buttons — native `confirm()` is
  silently disabled in sandboxed iframes without `allow-modals`.

## Known deviations / future work

- Widget-initiated `tools/call` responses are applied directly; hosts may
  additionally echo a `tool-result` notification (both paths are handled,
  idempotently for identical rows).
- `ui/resource-teardown` is acknowledged but does not yet run teardown
  callbacks.
- Spec `2026-01-26` is pinned; re-verify `_meta` key wording after the MCP
  core `2026-07-28` release before tagging v0.1.0.
