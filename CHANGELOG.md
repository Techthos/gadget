# Changelog

## Unreleased

- Legacy mcp-ui fallback now speaks the **UI Interaction Protocol v1**: when
  `callTool` receives the new optional `UIEventMeta` (`{label, kind}`), the
  dispatch is a prompt-type action `{type:"prompt", messageId,
  payload:{prompt}}` whose text is the `\uievent` envelope — a single-line
  JSON header (`v:1`, `label` ≤ 80 chars, `kind` `click`|`submit`|`select`)
  followed by an instruction naming the tool and its JSON arguments.
  Protocol-aware hosts (LibreChat with the uievent-chip patch) render the
  interaction as an event chip instead of a fake user message. The built-in
  widgets pass meta on every action: table row actions (`"<label> <row id>"`,
  `click`), bulk actions (`"<label> (<n> selected)"`, `select`), form submit
  (form title or submit label, `submit`). Calls without meta keep posting the
  plain tool-type action.
- Legacy **mcp-ui host interop**: until an MCP Apps host is confirmed
  (`ui/initialize` answered or any host→view method seen), the bridge
  dispatches actions via the community mcp-ui postMessage protocol —
  `callTool` posts `{type:"tool", messageId, payload:{toolName, params}}`,
  `openLink` posts `{type:"link", payload:{url}}`. A matching
  `ui-message-response` is used as the tool result; otherwise the call
  resolves fire-and-forget (`CallToolResult.dispatched: true`, new
  `BridgeOptions.uiResponseTimeoutMs`, default 3000 ms). Enables per-call
  embedded widgets (`InitialData` + unique URI) in hosts like LibreChat.
- Iframe auto-resize for mcp-ui hosts: size watching starts at first paint
  (no longer gated on `ui/initialize`), and until a host is confirmed
  `sizeChanged` also posts `{type:"ui-size-change", payload:{height}}`
  (height only — the responsive CSS width must win). Document CSS now resets
  `body{margin:0;padding:8px}` so `body.scrollHeight` measures the true
  content height (margins clipped the bottom edge).

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
