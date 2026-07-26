# Preview server

`examples/preview` is a runnable MCP server built for looking at gadget
through a real MCP client. Where `examples/harness` fakes the host in one HTML
page, this one speaks the protocol: tools are registered tools, widgets are
`ui://` resources, and every button a widget renders fires an actual
`tools/call` that a server handler answers.

```sh
make preview                      # streamable HTTP on :8081
go run ./examples/preview -stdio  # for hosts that spawn the server
```

Then point an MCP Apps capable inspector at `http://localhost:8081/mcp`.

## What it contains

Two halves, both registered by default.

**The scenario** is a small application, Acme Dispatch, with mutable state:
customers and the orders waiting to leave the warehouse. It is the half that
shows how the pieces fit together.

| Tool | Widget | What it demonstrates |
|---|---|---|
| `main_menu` | Menu | The launcher: tiles with icons, badges and static args |
| `list_customers` | Table | Sort, filter, pagination with a page-size chooser, row actions, bulk actions, `LoadTool` hydration |
| `browse_customers` | CardList | The same records as a paged card strip |
| `show_customer` | Card | Header action, body prose, the full typed detail list |
| `list_orders` | Table | A link column, `decimal:1` and `int` number formats, actions that open other widgets |
| `edit_customer` | Form | Runtime prefill, server-side validation answering with field errors |
| `new_customer` | Form | Every field type, plus `StaticArgs` merged into the submit |
| `confirm_delete_customer` | Confirm | Consequences counted from current state at call time, acknowledgement box, a reject tool the server hears about |
| `choose_shipping` | Choice | Options priced per call, typed option details formatted in the host locale |
| `choose_extras` | Choice | Authored options, several picks, bounded by `Min`/`Max` |
| `reset_demo` | none | An ordinary text tool, to restore the seed data |

Everything the widgets fire back into (`delete_customer`, `save_customer`,
`ship_order`, and the rest) is registered app-only: `visibility: ["app"]`, so
it is callable from the UI and hidden from the model.

**The gallery** is a catalog: one tool per widget variant, each with its own
resource, covering the renderings the scenario has no room for. Start at
`preview_index`, a menu listing all of them, or call any `preview_*` tool
directly. It mirrors the story list in `examples/harness`, including the empty
states, the long list, the load-more strip, both menu styles, all three choice
layouts, the runtime-data variants, a table with `RowsKey`/`RowID` moved off
their defaults, a full theme-token override, the frameless theme, and the
data-URI brand path.

Gallery data is canned and stateless. Actions fired from a gallery widget land
on the `sandbox_*` tools, which compute an answer from the same fixture, so
the gallery never disturbs the scenario store. The gallery forms validate for
real: submit `taken@example.com` to see a field error come back.

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `-addr` | `:8081` | HTTP listen address |
| `-stdio` | off | Serve over stdio instead of HTTP |
| `-mode` | `all` | Register `all`, `scenario` or `gallery` tools |
| `-quiet` | off | Do not log tool calls to stderr |

Tool calls are logged to stderr by default, including the ones widgets make.
That log is usually the fastest way to see which tool a button fired and with
what arguments.

## Running it under the MCP Inspector

The official inspector renders MCP Apps in its **Apps** tab (verified with
v1.0.0).

```sh
make preview                          # terminal 1
npx @modelcontextprotocol/inspector   # terminal 2
```

In the inspector: **Transport** `Streamable HTTP`, **URL**
`http://localhost:8081/mcp`, then Connect. Both fields can be prefilled from
the address bar:

```
http://localhost:6274/?transport=streamable-http&serverUrl=http://localhost:8081/mcp
```

Open the **Apps** tab, pick `main_menu` or `preview_index`, and the widget
renders in the pane on the right; the ⤢ button widens it, which is worth doing
for the tables, since the narrow pane collapses them to the stacked layout.
Tiles and buttons inside a widget call back into the server over the same
session: the `tools/call` lines on the preview server's stderr are the proof,
and they name the tool and its arguments.

The CLI mode is useful for checking the wire without the UI:

```sh
npx @modelcontextprotocol/inspector --cli http://localhost:8081/mcp \
  --transport http --method tools/list

npx @modelcontextprotocol/inspector --cli http://localhost:8081/mcp \
  --transport http --method tools/call --tool-name choose_shipping \
  --tool-arg id=4471
```

To let the inspector spawn the server itself, build a binary first and hand it
that. (`go run` does not work here: the inspector needs a command it can start
from any directory, and `go run` needs the module.)

```sh
go build -o /tmp/gadget-preview ./examples/preview
npx @modelcontextprotocol/inspector /tmp/gadget-preview -stdio
```

Notes:

- The inspector sandboxes app iframes against an origin allowlist that assumes
  localhost. Serving the inspector from another host can make widgets refuse
  to load; run it locally.
- `theme.Transparent` (the `preview_theme_transparent` variant) only looks
  right when the host leaves the iframe element unpainted. Hosts that paint a
  background behind it will show that background instead of nothing.

## Other hosts

Nothing here is inspector-specific. The same endpoint works as a Claude custom
connector, and with MCPJam or any other MCP Apps host; `-stdio` covers hosts
that spawn servers. A host that does not implement the Apps extension still
sees ordinary tools returning text and structured content.
