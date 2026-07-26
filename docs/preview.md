# Preview server

`examples/preview` is a runnable MCP server built for looking at gomukit
through a real MCP client. Where `examples/harness` fakes the host in one HTML
page, this one speaks the protocol: tools are registered tools, widgets are
`ui://` resources, and every button a widget renders fires an actual
`tools/call` that a server handler answers.

```sh
make preview                      # streamable HTTP on :8081
go run ./examples/preview -stdio  # for hosts that spawn the server
make inspect                      # the same, with the MCP Inspector connected
```

Then point an MCP Apps capable inspector at `http://localhost:8081/mcp`.

`make inspect` drives **this** server. The smaller `examples/demo` is a
different server on a different port — `make inspect-demo` puts the inspector in
front of that one instead, so its tools (`main_menu`, `list_users`,
`schedule_followup`, …) are the ones you see.

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
| `schedule_delivery` | DatePicker | A delivery date bounded by current availability, blocked days from state at call time |
| `reset_demo` | none | An ordinary text tool, to restore the seed data |

Everything the widgets fire back into (`delete_customer`, `save_customer`,
`ship_order`, and the rest) is registered app-only: `visibility: ["app"]`, so
it is callable from the UI and hidden from the model.

**The gallery** is a catalog: one tool per widget variant, each with its own
resource, covering the renderings the scenario has no room for. Start at
`preview_index`, a menu listing all of them, or call any `preview_*` tool
directly. It mirrors the story list in `examples/harness`, including the empty
states, the long list, the load-more strip, both menu styles, all three choice
layouts, the date and date-range pickers with presets and caption dropdowns, the runtime-data variants, a table with `RowsKey`/`RowID` moved off
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
| `-sandbox` | off | Give every MCP session its own scenario store |
| `-session-timeout` | `0` | Close sessions idle for this long (`0` never closes) |
| `-cors` | off | Answer CORS preflights and allow any origin |
| `-behind-proxy` | off | Accept a forwarded Host header on a loopback listener |

Tool calls are logged to stderr by default, including the ones widgets make.
That log is usually the fastest way to see which tool a button fired and with
what arguments.

The last four flags exist for hosting the server rather than running it
locally; see [Hosting it publicly](#hosting-it-publicly).

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
go build -o /tmp/gomukit-preview ./examples/preview
npx @modelcontextprotocol/inspector /tmp/gomukit-preview -stdio
```

Notes:

- The inspector sandboxes app iframes against an origin allowlist that assumes
  localhost. Serving the inspector from another host can make widgets refuse
  to load; run it locally.
- `theme.Transparent` (the `preview_theme_transparent` variant) only looks
  right when the host leaves the iframe element unpainted. Hosts that paint a
  background behind it will show that background instead of nothing.

## Screenshots

`make screenshots` rebuilds every image under `docs/assets` from the harness
stories, so the pictures in the README and the docs are never hand-captured:

```sh
make screenshots
make screenshots SHOT_FLAGS="--only table --themes light"
node scripts/screenshots.mjs --help
```

The script starts its own harness on a free port, drives an already installed
Chrome over the DevTools protocol (Node's built-in WebSocket speaks it — there
is no npm dependency and no browser download), and writes
`docs/assets/preview/<story>.png` plus a `-dark` variant per story, at 2x. The
five images the README embeds are rewritten from the same shots. Files whose
bytes did not change are left alone, so a re-run on an untouched tree leaves
the working copy clean.

Each story is loaded with a small MCP Apps host shim injected ahead of the
page's own scripts, so the widget completes the `ui/initialize` handshake and
renders with the theme, locale (`en-US`) and time zone (`UTC`) a host would
send — the same context `examples/harness` uses for its theme toggle. Stories
that ship without a snapshot on purpose (`confirm-runtime`, `choice-runtime`,
`datepicker-runtime`) get their push-panel payload delivered as a
`ui/notifications/tool-result` before the shot; the empty-state stories do not,
because the empty rendering is what they document.

Two knobs live at the top of `scripts/screenshots.mjs`: `WIDTHS`, the render
width per story or group — widgets lay themselves out against the room they are
given, so this is what decides how many columns a `Descriptions` keeps or
whether a `Choice` moves its description into the side panel — and
`README_SHOTS`, which story feeds which README image.

Rendering depends on the fonts installed on the machine (widgets ask for
`system-ui`), so a run on a different OS can rewrite every file even when
nothing in the library changed. Regenerate on the machine that produced the
committed set, or accept the whole-set diff.

## Other hosts

Nothing here is inspector-specific. The same endpoint works as a Claude custom
connector, and with MCPJam or any other MCP Apps host; `-stdio` covers hosts
that spawn servers. A host that does not implement the Apps extension still
sees ordinary tools returning text and structured content.

## Hosting it publicly

Put the endpoint on a URL and anyone with an MCP Apps capable chat client can
try the widgets without cloning anything. `examples/Dockerfile` builds
both the harness and this server into one scratch image, and
`examples/docker-compose.yml` runs them side by side — the harness on
8090, the preview MCP on 8081. Neither needs a checkout: the build context is
the repository URL.

```sh
curl -O https://raw.githubusercontent.com/Techthos/gomukit/main/examples/docker-compose.yml
docker compose up -d
```

Visitors then connect to `https://your-host/mcp`.

The scenario half writes to a store, which a shared deployment must not let
one visitor's edits leak out of. `-sandbox` builds a fresh server, and so a
fresh store, per MCP session: every writing tool works in full, and what a
visitor deletes or ships is theirs alone. That is the read-only property worth
having here — the deployment is read-only, the demo is not, so forms still
validate and confirmations still confirm. Nothing is persisted either way;
the process holds the only copy.

The other three flags cover the surroundings:

- `-session-timeout 30m` — a sandboxed store lives as long as its session, so
  idle ones need reaping.
- `-cors` — browser-based clients cannot reach the endpoint cross-origin
  without it. Server-side clients (Claude custom connectors among them) do not
  care.
- `-behind-proxy` — the SDK rejects a non-localhost `Host` on a loopback
  listener as a DNS rebinding attempt. Needed when a reverse proxy on the same
  host forwards over 127.0.0.1; not needed for a proxy on a Docker network.

Add `-quiet` unless you want every visitor's tool calls on stderr. Health
probes can hit `/healthz`, which the image has no shell to check from inside.

Two caveats for a public URL. The endpoint is unauthenticated, so anyone who
finds it can call the tools and hold a session; put it behind whatever your
proxy offers if that matters. And each live session holds its own fixture
data in memory, so session count is what to watch, with `-session-timeout`
bounding it.
