# AGENTS.md — Working with `gadget`

This document is the complete reference an LLM (or any agent) needs to **use**
the `gadget` library. For repo-contribution guidance (build commands, asset
pipeline, invariants when editing this codebase) see `CLAUDE.md`; deeper design
docs live in `docs/architecture.md`, `docs/widgets.md`, `docs/theming.md`.

- **Module**: `github.com/techthos/gadget` (Go >= 1.25)
- **What it is**: prebuilt, parameterized, interactive HTML widgets (**Table**,
  **Form**) for **MCP Apps** — the official MCP UI extension
  (`io.modelcontextprotocol/ui`, spec `2026-01-26`). Widgets render as fully
  self-contained HTML documents (inline CSS + JS, no external references)
  served as `ui://` template resources from a Go MCP server. Hosts (Claude,
  ChatGPT, VS Code, Cursor, Goose, Postman, …) render them in a sandboxed
  iframe inside the chat.
- **Status**: pre-release; APIs are not stable yet.
- **License**: MIT

---

## 1. Mental model (read this first)

The MCP Apps spec uses a **template model**: the HTML resource is registered
once and cannot contain per-call data. Data arrives at runtime. `gadget`
therefore splits rendering:

1. **Go renders structure** (registration time): the widget shell (table
   chrome, form fields with native validation attributes) plus a
   `#gadget-config` JSON island describing columns/fields/action bindings, and
   an optional `#gadget-data` snapshot (`InitialData`).
2. **The embedded TypeScript runtime renders data** (runtime, inside the
   host's sandboxed iframe): rows, prefill values, errors — first from the
   snapshot, then from every `ui/notifications/tool-result` notification and
   every widget-initiated `tools/call` response. All formatting uses `Intl`
   with the host's locale/time zone.

Consequences for you as a library user:

- You never write HTML, CSS, or JavaScript. You declare a widget struct, link
  tools to it, and return **`structuredContent`-shaped data** from tool
  handlers.
- The widget ↔ server contract is entirely about **which keys appear in the
  tool result's `structuredContent`** (see section 4).
- Sorting, filtering, pagination, and selection are **client-side** over the
  rows delivered — there is no server round-trip for them.

### Package map

| Package | Import path | Role |
|---|---|---|
| `gadget` | `github.com/techthos/gadget` | Widget definitions (`Table`, `Form`, `Action`, columns, fields) + `RowsOf` |
| `theme` | `github.com/techthos/gadget/theme` | `Theme` struct → CSS design-token overrides |
| `uispec` | `github.com/techthos/gadget/uispec` | MCP Apps spec constants and `_meta` types (zero deps) |
| `gosdk` | `github.com/techthos/gadget/gosdk` | Adapter for the official `github.com/modelcontextprotocol/go-sdk` — the **only** package importing an MCP SDK |

The core is SDK-agnostic: with any other Go MCP implementation, wire widgets
manually via the `Widget` interface (section 8).

---

## 2. Quickstart (official go-sdk)

```go
package main

import (
    "context"
    "net/http"

    "github.com/modelcontextprotocol/go-sdk/mcp"
    "github.com/techthos/gadget"
    "github.com/techthos/gadget/gosdk"
)

func main() {
    table := &gadget.Table{
        URI:   "ui://myapp/users",
        Title: "Users",
        Columns: []gadget.Column{
            gadget.Text("name", "Name"),
            gadget.Number("balance", "Balance", "currency:EUR"),
            gadget.Badge("status", "Status", map[string]gadget.BadgeVariant{
                "active": gadget.BadgeSuccess,
            }),
        },
        Filterable: true,
        PageSize:   10,
    }

    // EnableUI declares the MCP Apps extension capability.
    server := mcp.NewServer(&mcp.Implementation{Name: "myapp"}, gosdk.EnableUI(nil))

    type in struct{}
    type out struct {
        Rows []map[string]any `json:"rows"` // key must match Table.RowsKey (default "rows")
    }
    gosdk.AddWidgetToolFor(server, table,
        &mcp.Tool{Name: "list_users", Description: "List users in a table."},
        func(context.Context, *mcp.CallToolRequest, in) (*mcp.CallToolResult, out, error) {
            rows, _ := gadget.RowsOf(loadUsers())
            return nil, out{Rows: rows}, nil
        })

    h := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, nil)
    http.ListenAndServe(":8080", h)
}
```

The typed handler's `out` struct is serialized by the SDK into the tool
result's `structuredContent`; the widget reads its data from there.

---

## 3. Package `gadget` — full API reference

### 3.1 The `Widget` interface

Both `*Table` and `*Form` implement:

```go
type Widget interface {
    Document() (string, error)              // complete self-contained HTML document (calls Validate first)
    Descriptor() uispec.ResourceDescriptor  // registration data for the ui:// template resource
    ToolMeta() map[string]any               // tool _meta linking the tool to this widget: {"ui": {"resourceUri": ...}}
    Validate() error                        // checks the widget configuration
}
```

With `gosdk` you rarely call these yourself; they exist for manual wiring.

### 3.2 Shared types

```go
type Align string        // AlignStart | AlignCenter | AlignEnd

type SortSpec struct {   // default sort order for a table
    Key  string `json:"key"`
    Desc bool   `json:"desc,omitempty"`
}

type EmptyState struct { // no-data message (Table.Empty)
    Title string `json:"title,omitempty"` // defaults to "No data" when rendered
    Body  string `json:"body,omitempty"`
}
```

### 3.3 `Table`

```go
type Table struct {
    URI     string      // REQUIRED. ui:// resource URI, e.g. "ui://myapp/users"
    Title   string      // toolbar heading + document title
    Columns []Column    // REQUIRED, non-empty

    RowsKey string      // structuredContent key holding the rows array. Default "rows"
    RowID   string      // row field uniquely identifying a row (selection, FromRow/FromSelection). Default "id"

    PageSize    int              // > 0 enables client-side pagination (page size); 0 disables; < 0 is invalid
    DefaultSort *SortSpec        // pre-sort rows on load (Key required when set)
    Filterable  bool             // adds a client-side text filter box
    Selection   *SelectionConfig // enables row checkboxes + bulk actions
    Empty       EmptyState       // no-data message

    InitialData map[string]any        // optional structuredContent-shaped snapshot baked into the document
    Theme       *theme.Theme          // design-token overrides for this widget
    UI          *uispec.ResourceUIMeta // overrides resource _meta.ui (CSP, permissions, prefersBorder)
}

type SelectionConfig struct {
    Bulk []Action // toolbar actions shown while rows are selected; FromSelection args resolve across all selected rows
}
```

Rows are string-keyed JSON objects (`[]map[string]any` /
`[]object`) delivered at runtime under `RowsKey`.

### 3.4 `Column`

```go
type Column struct {
    Key      string                  // row field this column displays (REQUIRED except for ColActions)
    Label    string
    Type     ColumnType              // defaults to ColText
    Sortable *bool                   // overrides default sortability
    Align    Align
    Format   string                  // see formats below
    Badge    map[string]BadgeVariant // ColBadge: cell value -> variant
    Link     *LinkSpec               // ColLink config
    Actions  []Action                // ColActions: per-row buttons
    Width    string                  // CSS width, e.g. "12rem", "20%"
}
```

**Column types** (`ColumnType`): `ColText` (`"text"`, zero-value default),
`ColNumber`, `ColDate`, `ColBadge`, `ColLink`, `ColActions`.

**Formats** (interpreted by the runtime via `Intl`, host locale/time zone):

| Type | Format values |
|---|---|
| number | `"int"`, `"decimal:<digits>"`, `"percent"`, `"currency:<code>"` (e.g. `"currency:EUR"`) |
| date | `"date"`, `"datetime"`, `"time"`, `"relative"` |

**Sortability defaults**: text/number/date columns are sortable; badge, link,
and actions columns are not. Set `Sortable: &b` to override either way.

**Badge variants** (`BadgeVariant`): `BadgeNeutral`, `BadgeInfo`,
`BadgeSuccess`, `BadgeWarning`, `BadgeDanger`.

**Link columns**:

```go
type LinkSpec struct {
    HrefKey string `json:"hrefKey"`           // REQUIRED: row field holding the URL
    TextKey string `json:"textKey,omitempty"` // row field holding the link text …
    Text    string `json:"text,omitempty"`    // … or a fixed text; else the URL itself is shown
}
```

**Column constructors** (sugar — plain struct literals work too):

```go
gadget.Text(key, label)                    // text column
gadget.Number(key, label, format...)       // number column, right-aligned (Align: AlignEnd)
gadget.Date(key, label, format...)         // date column
gadget.Badge(key, label, variants)         // badge column
gadget.Link(hrefKey, label)                // link column (Key and Link.HrefKey both set to hrefKey)
gadget.ActionsColumn(actions...)           // per-row actions column (empty Label)
```

### 3.5 `Form`

```go
type Form struct {
    URI    string      // REQUIRED. ui:// resource URI
    Title  string      // heading + document title
    Fields []Field     // REQUIRED, non-empty
    Submit SubmitSpec  // REQUIRED (Submit.Tool must be set)
    Cancel *CancelSpec // when set, adds a reset button

    PrefillKey string  // structuredContent key with {"field": value} prefill. Default "values"
    ErrorsKey  string  // structuredContent key with {"field": "message"} errors. Default "errors"

    InitialData map[string]any         // optional snapshot, e.g. {"values": {...}} for a pre-filled edit form
    Theme       *theme.Theme
    UI          *uispec.ResourceUIMeta
}

type SubmitSpec struct {
    Tool           string         // REQUIRED: MCP tool called with {field: value, ...} merged over StaticArgs
    Label          string         // button label, default "Submit"
    StaticArgs     map[string]any // fixed args merged UNDER the field values (field values win)
    SuccessMessage string         // shown after a successful submit
}

type CancelSpec struct {
    Label string // default "Cancel"
}
```

### 3.6 `Field`

```go
type Field struct {
    Name        string      // REQUIRED, unique: the tool-call argument name
    Label       string      // defaults to Name when rendered
    Description string      // help text under the control
    Placeholder string
    Type        FieldType   // defaults to FText
    Required    bool
    Default     any         // initial value: string-like for most; bool for FCheckbox; []string (or string) for FMultiSelect
    Options     []Option    // REQUIRED for FSelect / FMultiSelect
    Validation  *Validation // client-side constraints
    Rows        int         // textarea height (FTextarea), default 3
}
```

**Field types** (`FieldType`): `FText` (`"text"`, zero-value default),
`FTextarea`, `FNumber`, `FCheckbox`, `FSelect`, `FMultiSelect`, `FDate`,
`FTime`, `FHidden`, `FReadonly`.

```go
type Option struct {
    Value string `json:"value"`
    Label string `json:"label"`
}
gadget.Opt("admin") // Option{Value: "admin", Label: "admin"}
```

**Client-side validation** — rendered as native HTML attributes and enforced
by the runtime before submit (inline error messages; `Message` overrides the
browser's text):

```go
type Validation struct {
    Pattern string   // HTML pattern-attribute regex
    Min     *float64 // number/date/time constraints
    Max     *float64
    Step    *float64
    MinLen  *int     // text length constraints
    MaxLen  *int
    Message string   // overrides the browser's validation message
}
```

**Submitted value types** (what your submit tool receives as arguments):

| Field type | Submitted as |
|---|---|
| `FCheckbox` | `bool` |
| `FNumber` | number (**omitted entirely when empty**) |
| `FMultiSelect` | `[]string` |
| everything else — including `FHidden` and `FReadonly` | `string` (parse server-side, e.g. hidden numeric IDs arrive as `"3"`) |

### 3.7 `Action`

An `Action` is a user-triggerable operation: a per-row button
(`ActionsColumn`), a bulk action over selected rows (`SelectionConfig.Bulk`),
or a link.

```go
type Action struct {
    Label   string               // REQUIRED
    Kind    ActionKind           // ActionTool (default) | ActionLink
    Tool    string               // MCP tool name (REQUIRED for ActionTool)
    Args    map[string]ArgSource // tool argument name -> value source
    HrefKey string               // row field holding the URL (REQUIRED for ActionLink; opens via ui/open-link)
    Confirm string               // when set: inline two-phase confirmation with this text before firing
    Variant ActionVariant        // VariantDefault ("") | VariantPrimary | VariantDanger
}
```

**Argument sources** (`ArgSource` is opaque — construct ONLY with these):

```go
gadget.Static(v)             // fixed value
gadget.FromRow("field")      // value of the field on the row the action was triggered on
gadget.FromSelection("field") // values of the field across ALL selected rows — bulk actions ONLY
```

`FromSelection` in a per-row (column) action is a validation error. An
`ArgSource` built any other way (zero value, multiple sources) fails
validation/marshaling.

**Behavior contract**:

- `Confirm` renders an inline two-phase button (click → confirm text → click
  again). Native `confirm()` dialogs are silently disabled in sandboxed MCP
  Apps iframes — never rely on them.
- **If a tool called by a table action returns a result whose
  `structuredContent` contains `RowsKey`, the table re-renders with the
  returned rows and clears the selection.** Therefore: mutating tools
  (delete/archive/…) should return the updated full row list.

### 3.8 `RowsOf`

```go
func RowsOf(slice any) ([]map[string]any, error)
```

Converts a typed slice (e.g. `[]User` or `[]*User`) into row maps via
`encoding/json`, honoring `json` struct tags. Use it to feed typed data into
`Table.InitialData` or a tool result:

```go
rows, err := gadget.RowsOf(users)
table.InitialData = map[string]any{"rows": rows}
```

Errors if the value doesn't marshal to a JSON array of objects.

### 3.9 Validation rules (what `Validate()` / `Document()` reject)

Table:
- `URI` must be a well-formed `ui://` URI with a non-empty path.
- At least one column; no duplicate column `Key`s.
- text/number/date/badge columns: `Key` required.
- link columns: `Link.HrefKey` required.
- actions columns: at least one action.
- `PageSize >= 0`; `DefaultSort.Key` required when `DefaultSort` is set.
- Actions: `Label` required; `Tool` required for tool kind; `HrefKey` required
  for link kind; all `Args` built with the constructors; `FromSelection` only
  in bulk actions.
- `Theme` must pass `theme.Validate()`.

Form:
- `URI` as above; at least one field; `Submit.Tool` required.
- Field `Name` required and unique.
- `FSelect`/`FMultiSelect` require non-empty `Options`.
- `Theme` must pass `theme.Validate()`.

`Document()` calls `Validate()` first and returns its error, so with `gosdk`
registration you get configuration errors at startup, not at render time.

---

## 4. The runtime data contract (structuredContent keys)

Widgets read all runtime data from the tool result's `structuredContent`:

| Widget | Key (configurable via) | Shape | Meaning |
|---|---|---|---|
| Table | `rows` (`RowsKey`) | `[]object` | rows to render |
| Form | `values` (`PrefillKey`) | `{field: value}` | prefill (edit flows) |
| Form | `errors` (`ErrorsKey`) | `{field: "message"}` | server-side field errors, rendered inline; marks the submit failed |

With the go-sdk typed handlers, your `Out` struct's JSON form becomes
`structuredContent` — so match the JSON tags to these keys:

```go
type rowsOut struct { Rows []map[string]any `json:"rows"` }
type editOut struct { Values map[string]any `json:"values"` }
type saveOut struct { Errors map[string]string `json:"errors,omitempty"` }
```

Flows:

- **List**: model calls e.g. `list_users` → result `{"rows": [...]}` → table
  renders.
- **Row/bulk mutation**: widget calls e.g. `delete_user` (app-only) → return
  `{"rows": [...updated list...]}` → table re-renders, selection cleared.
- **Edit form**: model calls e.g. `edit_user` (linked to the Form) → result
  `{"values": {"id": 3, "name": "Ada", ...}}` → form prefills.
- **Submit**: widget calls `Submit.Tool` with `{field: value, ...}` merged
  over `StaticArgs` → return `{"errors": {"email": "taken"}}` to fail with
  inline errors, or an errors-free result to succeed (shows
  `SuccessMessage` if set).

### Legacy mcp-ui host interop (embedded per-call widgets)

Hosts that render the community **mcp-ui** standard but not MCP Apps (e.g.
LibreChat v0.8.x) never fetch `ui://` template resources or push tool
results. Widgets still work there via a different consumption pattern:

- Build the widget **per call** with the data baked in (`InitialData` — the
  runtime paints it before, and without, any host handshake) and a **unique
  URI per render**, and append the rendered `Document()` to the tool result's
  `content` as an embedded resource
  (`{type:"resource", resource:{uri, mimeType:"text/html", text: doc}}`).
- **Actions fall back automatically**: until an MCP Apps host is confirmed
  (`ui/initialize` answered, or any host→view method seen), `callTool` posts
  the legacy mcp-ui action message
  `{type:"tool", messageId, payload:{toolName, params}}` to the parent and
  `openLink` posts `{type:"link", payload:{url}}`. If the host replies with a
  `ui-message-response` for that `messageId`, it is used as the tool result
  (`payload.error` rejects); otherwise the call resolves fire-and-forget
  after `uiResponseTimeoutMs` (default 3000 ms) with
  `{dispatched: true, content:[{type:"text", text:"Action sent to the host."}]}` —
  the table shows that text as a transient status, the form shows it instead
  of `SuccessMessage` (the host, not the widget, completes the action — e.g.
  LibreChat turns it into a conversation turn where the model runs the tool).
- **The iframe auto-resizes**: size reporting starts at first paint (not
  gated on the handshake) and, until a host is confirmed, also posts the
  mcp-ui `{type:"ui-size-change", payload:{height}}` message — hosts with
  auto-resize grow the iframe so the widget is always fully visible, never
  internally scrolled. `width` is deliberately omitted so the iframe keeps
  its responsive CSS width. The document resets `body{margin:0;padding:8px}`
  (margin sits outside `body.scrollHeight` and would clip the bottom).
- Consequence for such hosts: point actions/submits at **model-visible**
  tools (the host routes them through the model; `_meta.ui.visibility` is
  not understood there), and there is no rows-refresh round-trip — attach a
  freshly rendered widget to the mutating tool's result instead.

---

## 5. Package `gosdk` — official go-sdk adapter

```go
import "github.com/techthos/gadget/gosdk"
```

```go
// Declares the MCP Apps extension in server capabilities. Mutates and
// returns opts (nil allocates fresh options) so it composes with
// mcp.NewServer. NOTE: explicitly setting Capabilities disables the SDK's
// historical default of advertising {"logging":{}}.
func EnableUI(opts *mcp.ServerOptions) *mcp.ServerOptions

// Registers w's template as a ui:// resource on s. The document is rendered
// ONCE and served from memory. Idempotent per (server, URI): re-registering
// the same URI is a no-op. Returns render/validation errors.
func AddWidget(s *mcp.Server, w gadget.Widget) error

// Registers tool t linked to w via _meta (registers w's resource first if
// needed); raw-handler variant.
func AddWidgetTool(s *mcp.Server, w gadget.Widget, t *mcp.Tool, h mcp.ToolHandler) error

// Same, with the SDK's typed handler: input/output JSON schemas inferred
// from In and Out. Out's JSON form becomes structuredContent.
func AddWidgetToolFor[In, Out any](s *mcp.Server, w gadget.Widget, t *mcp.Tool, h mcp.ToolHandlerFor[In, Out]) error

// Marks t app-only (_meta.ui.visibility: ["app"]): callable from the widget
// UI, hidden from the model. Call BEFORE registering the tool (registration
// merges, so the visibility is kept). Use for row-action and submit tools.
func AppOnly(t *mcp.Tool, w gadget.Widget)

// Merges data into the result's _meta — delivered to the widget but hidden
// from the model (per spec).
func WithAppData(res *mcp.CallToolResult, data map[string]any)

// Reports whether the session's client declared the MCP Apps extension.
// Branching on it is optional — attaching _meta.ui unconditionally is
// spec-legal (hosts ignore unknown metadata).
func ClientSupportsUI(ss *mcp.ServerSession) bool
```

**Canonical wiring pattern** (from `examples/demo`):

```go
server := mcp.NewServer(&mcp.Implementation{Name: "myapp"}, gosdk.EnableUI(nil))

// Model-visible tool rendered by the table:
gosdk.AddWidgetToolFor(server, table,
    &mcp.Tool{Name: "list_users", Description: "List users."}, listUsers)

// App-only tool (fired by a row action, hidden from the model):
del := &mcp.Tool{Name: "delete_user", Description: "Delete a user."}
gosdk.AppOnly(del, table)
gosdk.AddWidgetToolFor(server, table, del, deleteUser)
```

Multiple tools may link to the same widget; `AddWidget` runs implicitly and
is idempotent. Serve via `mcp.NewStreamableHTTPHandler` (HTTP) or
`server.Run(ctx, &mcp.StdioTransport{})` (stdio).

---

## 6. Package `theme` — styling overrides

Widgets ship a `--gadget-*` design-token system scoped under `.gadget-root`.
Every semantic token defaults to the **host-injected MCP Apps CSS variable**
(delivered via `hostContext.styles.variables` during `ui/initialize`) with a
built-in fallback — so widgets automatically match Claude/ChatGPT theming with
zero configuration, and dark mode follows the host theme (with a
`prefers-color-scheme` fallback). Only use `Theme` to deliberately override.

```go
type Theme struct {
    ColorBackground  string // page/widget background
    ColorSurface     string // cards, table header, inputs
    ColorText        string
    ColorTextMuted   string
    ColorBorder      string
    ColorPrimary     string // accent: primary buttons, focused controls, links
    ColorPrimaryText string // text on primary background
    ColorDanger      string
    ColorSuccess     string
    ColorWarning     string

    FontFamily     string
    FontFamilyMono string

    RadiusS string
    RadiusM string
    RadiusL string

    SpaceUnit string // base spacing unit (default 0.25rem); all gaps/paddings derive from it

    Extra map[string]string // extra/override raw custom properties; keys MUST start with "--gadget-"
}

func (t *Theme) CSS() string      // ".gadget-root{...}" block, "" when nothing set; skips invalid entries
func (t *Theme) Validate() error  // surfaces what CSS() would silently skip
```

- The zero value (and a nil `*Theme`) overrides nothing. Fields hold raw CSS
  values (`"#0f62fe"`, `"0.5rem"`, `"Inter, sans-serif"`). Non-empty fields
  win the cascade over host values; empty fields keep host-aware defaults.
- **Value safety**: values must not contain `{`, `}`, `;`, `</`, or `<!--`
  (CSS/HTML breakout guard). `Extra` keys must start with `--gadget-` and use
  only `[A-Za-z0-9_-]`. Widget `Validate()` calls `Theme.Validate()`.

Token → host variable mapping (for reference): `--gadget-color-bg` ←
`--color-background-primary`, `--gadget-color-surface` ←
`--color-background-secondary`, `--gadget-color-text` ←
`--color-text-primary`, `--gadget-color-text-muted` ←
`--color-text-secondary`, `--gadget-color-border` ←
`--color-border-primary`, `--gadget-color-primary` ← `--color-text-accent`,
danger/success/warning ← `--color-text-danger/success/warning`,
`--gadget-font`/`--gadget-font-mono` ← `--font-sans`/`--font-mono`,
`--gadget-radius-s/m/l` ← `--border-radius-sm/md/lg`.

---

## 7. Package `uispec` — spec constants and `_meta` types

Dependency-free; use it for manual wiring or advanced `_meta` control.

```go
const (
    ExtensionID = "io.modelcontextprotocol/ui" // capability-negotiation extension id
    SpecVersion = "2026-01-26"                 // targeted MCP Apps spec version
    MIMEType    = "text/html;profile=mcp-app"  // media type of UI template resources
    MetaKey     = "ui"                         // _meta key for UI metadata
    URIScheme   = "ui"                         // ui:// scheme
)

const (
    VisibilityModel = "model" // tool callable by the model
    VisibilityApp   = "app"   // tool callable from the app UI only
)

const ( // ResourceUIMeta.Permissions values
    PermissionCamera         = "camera"
    PermissionMicrophone     = "microphone"
    PermissionGeolocation    = "geolocation"
    PermissionClipboardWrite = "clipboardWrite"
)

// External origins a UI resource needs (hosts default to fully locked-down).
type CSP struct {
    ConnectDomains  []string `json:"connectDomains,omitempty"`
    ResourceDomains []string `json:"resourceDomains,omitempty"`
    FrameDomains    []string `json:"frameDomains,omitempty"`
    BaseURIDomains  []string `json:"baseUriDomains,omitempty"`
}

// _meta.ui on a ui:// resource (set via Table.UI / Form.UI).
type ResourceUIMeta struct {
    CSP           *CSP     `json:"csp,omitempty"`
    Permissions   []string `json:"permissions,omitempty"`
    Domain        string   `json:"domain,omitempty"`
    PrefersBorder *bool    `json:"prefersBorder,omitempty"`
}

// _meta.ui on a tool, linking it to its template resource.
type ToolUIMeta struct {
    ResourceURI string   `json:"resourceUri"`
    Visibility  []string `json:"visibility,omitempty"`
}

// Everything needed to register a widget's template resource, SDK-agnostic.
type ResourceDescriptor struct {
    URI, Name, Title, Description string
    MIMEType                      string // always uispec.MIMEType for gadget widgets
    UI                            *ResourceUIMeta
}

func (m ResourceUIMeta) MetaMap() map[string]any     // {"ui": {...}}
func (m ToolUIMeta) MetaMap() map[string]any         // {"ui": {"resourceUri": ..., ...}}
func (d ResourceDescriptor) MetaMap() map[string]any // nil when d.UI == nil

// Recursive merge (maps merge, everything else overwrites); nil dst allocated.
func MergeMeta(dst, src map[string]any) map[string]any

// Checks uri is a well-formed ui:// URI (prefix + non-empty path).
func ValidateURI(uri string) error
```

Note: gadget widgets don't need any `CSP` declarations — documents are fully
self-contained and satisfy the spec's default locked-down policy. Only set
`UI.CSP` if you know a host-specific reason to.

---

## 8. Using gadget WITHOUT the official go-sdk

The core emits plain spec-shaped values; adapt to any Go MCP implementation:

```go
w := table // or form; any gadget.Widget

doc, err := w.Document() // render once (validates); serve from memory
d := w.Descriptor()      // d.URI, d.Name (derived: "ui://demo/users" -> "demo-users"),
                         // d.Title, d.MIMEType ("text/html;profile=mcp-app"), d.MetaMap()

// 1. Advertise the extension in server capabilities:
//    capabilities.extensions["io.modelcontextprotocol/ui"] = {"mimeTypes": ["text/html;profile=mcp-app"]}
// 2. Register a resource at d.URI with d.MIMEType (+ d.MetaMap() as _meta if non-nil);
//    resources/read returns doc as text.
// 3. On each linked tool, merge w.ToolMeta() into the tool's _meta.
// 4. Tool results carry widget data in structuredContent (section 4).
```

---

## 9. Constraints, gotchas, and rules for generated code

1. **Match keys exactly.** The most common wiring bug: the handler's output
   JSON key doesn't match `RowsKey`/`PrefillKey`/`ErrorsKey` (defaults:
   `"rows"`, `"values"`, `"errors"`). Nothing renders and nothing errors.
2. **Row identity**: every row should carry the `RowID` field (default
   `"id"`); selection and `FromRow`/`FromSelection` depend on it.
3. **Mutating table tools must return the updated row list** under `RowsKey`,
   or the UI keeps showing stale rows.
4. **Hidden/readonly/text form fields submit strings.** A hidden numeric ID
   arrives as `"3"` — parse server-side (`strconv.Atoi`). Empty `FNumber`
   fields are omitted from the arguments entirely.
5. **Mark widget-only tools app-only** (`gosdk.AppOnly`) — submit targets and
   row-action tools the model shouldn't invoke directly. Call `AppOnly`
   before registering the tool.
6. **`FromSelection` only in bulk actions**; `FromRow` in per-row actions.
7. **No native dialogs**: `confirm()`/`alert()` don't work in sandboxed MCP
   Apps iframes. Use `Action.Confirm` for destructive confirmation.
8. **Documents must stay self-contained**: no external URLs, CDNs, fonts, or
   images from the network. This is by construction — don't try to inject
   any via `Theme` (values are validated against breakout anyway).
9. **Widgets are registered once, immutably**: `AddWidget` renders the
   document a single time and serves it from memory. Don't mutate a widget
   struct after registration and expect changes; per-call variation belongs
   in tool-result data, not the template.
10. **Use `gadget.RowsOf`** to convert typed slices; it honors `json` tags,
    so column `Key`s must match the JSON tag names, not Go field names.
11. **`InitialData`** is optional and only an instant-first-paint snapshot; it
    is shaped like `structuredContent` (e.g. `{"rows": [...]}`), and is
    superseded by runtime tool results.
12. **Sort/filter/pagination are client-side** over delivered rows. For big
    datasets, page/filter server-side in the tool and deliver a bounded list.
13. **Theming**: prefer no `Theme` (host-matched look). When overriding,
    values are raw CSS; `Extra` keys must start with `--gadget-`.
14. **Errors surface at startup**: `AddWidget*` returns validation errors
    (bad URI, missing keys, duplicate columns/fields, unsafe theme values) —
    check them (`log.Fatal`/`must`).

---

## 10. Examples and manual testing

- `examples/demo` — complete runnable MCP server (users table with row/bulk
  actions + edit form with server-side validation, prefill, string-ID
  parsing). `go run ./examples/demo -addr :8080` (streamable HTTP at
  `/mcp`) or `go run ./examples/demo -stdio`. Point MCPJam, Claude custom
  connectors, or any MCP Apps host at `http://localhost:8080/mcp`.
- `examples/harness` — a fake MCP Apps host in one HTML page: renders widgets
  in a sandboxed iframe, answers the `ui/initialize` handshake, logs all
  JSON-RPC traffic, simulates tool results/errors and theme changes.
  `go run ./examples/harness`, open `http://localhost:8090`. Use it to verify
  widget behavior without any real MCP client.

## 11. Development commands (when modifying this repo)

```sh
make test         # go test ./... + vitest
make test-go      # Go tests only
make test-ui      # vitest only
make typecheck    # tsc --noEmit
make vet          # go vet ./...
make assets       # npm ci + rebuild the TS/CSS bundle into internal/assets/dist
make verify-dist  # rebuild assets, fail if committed dist drifted (CI does this)
```

After editing anything under `ui/` (src or css), run `make assets` and commit
the resulting `internal/assets/dist/` changes — the bundle is committed and
`go:embed`-ed so Go consumers never need Node; CI fails on drift. Golden
files: `go test ./ -update` regenerates `testdata/golden/`. Full contributor
rules: `CLAUDE.md`.
