# Changelog

## Unreleased

- **MCP Apps native only**: removed the legacy mcp-ui postMessage interop
  (the `{type:"tool"|"prompt"|"link"}` action dispatch, `ui-message-response`
  handling, the `\uievent` envelope/`UIEventMeta`, the `ui-size-change`
  message, `CallToolResult.dispatched`, and `BridgeOptions.uiResponseTimeoutMs`).
  The runtime speaks only the MCP Apps protocol (`ui/initialize`,
  `tools/call`, `ui/notifications/*`); hosts must attach the standard bridge.
  Result-embedded per-call widgets remain supported but MUST be declared with
  `uispec.MIMEType` (`text/html;profile=mcp-app`) so the host classifies them
  as apps.
- Iframe auto-resize: size watching starts at first paint (no longer gated
  on `ui/initialize`) so hosts can size the frame during the handshake.
  Document CSS resets `body{margin:0;padding:8px}` so `body.scrollHeight`
  measures the true content height (margins clipped the bottom edge).

Initial scaffold (targeting v0.1.0):

- MCP Apps (`io.modelcontextprotocol/ui`, spec `2026-01-26`) support:
  `uispec` constants/meta types, self-contained HTML documents satisfying
  the default locked-down CSP.
- Widgets: **Table** (typed columns, client sort/filter/pagination,
  selection + bulk actions, row actions with inline confirm) and **Form**
  (10 field types, native + inline validation, server-side field errors,
  prefill, submit → tool call).
- Embedded TypeScript runtime: JSON-RPC postMessage bridge
  (`ui/initialize`, `tools/call`, size-changed reporting, host-context
  handling), event-delegated behaviors, Intl formatting.
- Theming: `--gadget-*` token system defaulting to host-injected variables;
  `theme.Theme` override struct; dark mode via host theme +
  `prefers-color-scheme` fallback.
- `gosdk` adapter for `modelcontextprotocol/go-sdk` (extension capability,
  widget/tool registration, app-only visibility, app-only result data).
- Examples: `demo` MCP server (streamable HTTP/stdio), `harness` fake host.

Before v0.1.0: re-verify spec wording against the MCP core 2026-07-28
release.
