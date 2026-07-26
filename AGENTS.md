# AGENTS.md — Working with `gadget`

This document is the complete reference an LLM (or any agent) needs to **use**
the `gadget` library. For repo-contribution guidance (build commands, asset
pipeline, invariants when editing this codebase) see `CLAUDE.md`; deeper design
docs live in `docs/architecture.md`, `docs/widgets.md`, `docs/theming.md`.

- **Module**: `github.com/techthos/gadget` (Go >= 1.25)
- **What it is**: prebuilt, parameterized, interactive HTML widgets (**Table**,
  **CardList**, **Card**, **Form**) for **MCP Apps** — the official MCP UI extension
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
   chrome, form fields with native validation attributes, card chrome) plus a
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
| `gadget` | `github.com/techthos/gadget` | Widget definitions (`Table`, `Form`, `Card`, `CardList`, `Menu`, `Confirm`, `Choice`, `Action`, columns, fields) + `RowsOf` |
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

`*Table`, `*Form`, `*Card`, `*CardList`, `*Menu`, `*Confirm`, and `*Choice`
implement:

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
    PageSizes   []int            // alternative page sizes offered in a dropdown on the pagination bar; entries > 0; needs PageSize > 0; PageSize is added if absent; empty renders no chooser
    DefaultSort *SortSpec        // pre-sort rows on load (Key required when set)
    Filterable  bool             // adds a client-side text filter box
    Selection   *SelectionConfig // enables row checkboxes + bulk actions
    Empty       EmptyState       // no-data message

    InitialData map[string]any        // optional structuredContent-shaped snapshot baked into the document
    LoadTool    string                 // read tool the runtime calls once on load to re-fetch rows (must return them under RowsKey), replacing the baked snapshot so a reloaded widget shows current data
    LoadArgs    map[string]any         // optional static args passed to LoadTool
    Brand       *Brand                 // application logo/name shown on the widget
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
    Actions  []Action                // ColActions: per-row actions, shown in a "⋯" menu
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
    LoadTool    string                 // read tool the runtime calls once on load to re-fetch prefill (must return it under PrefillKey), replacing the baked snapshot
    LoadArgs    map[string]any         // optional static args passed to LoadTool
    Brand       *Brand
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

`FSelect` and `FMultiSelect` render as the gadget dropdown: the runtime
upgrades the `<select>` into a styled trigger and popup listbox (keyboard
navigation, typeahead, check marks on the chosen entries) while the select
itself stays the value holder, so submitted value types are unchanged. Every
other select in the library — the CardList sort control, the pagination bar's
page-size chooser — is the same control.

If a `Placeholder` is set on a select field, it is the empty-state text of the
trigger.

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

An `Action` is a user-triggerable operation: a per-row action
(`ActionsColumn`), a bulk action over selected rows (`SelectionConfig.Bulk`),
or a link.

In a `Table`, both kinds are reached through a menu rather than a strip of
buttons: an actions column renders one "⋯" trigger per row, and the bulk bar
renders a single "Actions" trigger beside the selection count. `Confirm` is
asked inside that menu, on the item itself. `Card`/`CardList` actions are
still buttons.

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

### 3.9 `CardTemplate`

Shared by `Card` and `CardList`: describes how one record renders as a card,
in three sections rendered in this order. Only `Header` is required; a section
with nothing in it is not rendered at all.

```go
type CardTemplate struct {
    Header  CardHeader  // REQUIRED (it holds the title)
    Content CardContent // the card body
    Footer  CardFooter  // the bottom section
}

type CardHeader struct {
    TitleKey       string  // REQUIRED: row field shown as the card title
    DescriptionKey string  // row field shown under the title
    Description    string  // fixed text under the title, instead of DescriptionKey
    Badge          Column  // status badge for the header's end slot (build with gadget.Badge); present when its Key is set — must be a badge column
    Action         *Action // button for the header's end slot, instead of Badge
}

type CardContent struct {
    TextKey string       // row field rendered as a paragraph of body text
    Text    string       // fixed body prose, instead of TextKey
    Items   Descriptions // label/value detail rows (the shared Descriptions block)
}

type CardFooter struct {
    TextKey string   // row field shown as a footer note
    Text    string   // fixed footer note, instead of TextKey
    Actions []Action // footer buttons; FromSelection args are invalid here (bulk actions belong to CardList.Selection)
}
```

- The header has **one** end slot: set `Badge` or `Action`, never both.
- `Content.Items` is the same `Descriptions` block used by `Confirm`, so card
  fields carry the same types, `Format` strings, badge maps and links as a
  table column, and a value the record does not carry renders as `—`.
- Text slots are either/or: `DescriptionKey`/`Description`,
  `Content.TextKey`/`Content.Text` and `Footer.TextKey`/`Footer.Text` each
  reject being set together.
- Action buttons are indexed header-first, then footer — relevant only to the
  runtime, not to callers.

### 3.10 `Card`

Renders a **single record** — the first element of the rows array delivered
under `RowsKey` (same data contract as `Table`/`CardList`).

```go
type Card struct {
    URI      string       // REQUIRED. ui:// resource URI
    Title    string       // toolbar heading + document title
    Template CardTemplate // REQUIRED

    RowsKey string // structuredContent key holding the rows array; renders rows[0]. Default "rows"
    RowID   string // record field used for FromRow action args. Default "id"
    Empty   EmptyState // shown when no record is present

    InitialData map[string]any         // optional snapshot, e.g. {"rows": [{...}]}
    LoadTool    string                 // read tool called once on load to re-fetch the record (under RowsKey), replacing the snapshot
    LoadArgs    map[string]any         // optional static args passed to LoadTool
    Brand       *Brand
    Theme       *theme.Theme
    UI          *uispec.ResourceUIMeta
}
```

### 3.11 `CardList`

Renders a **collection** as cards in a horizontally scrolling strip (a
carousel), with the same client-side runtime as `Table` (filter, sort,
pagination, selection + bulk actions, per-card actions, load-time hydration) —
laid out as cards instead of table rows. The strip is the only layout: it fits
a narrow chat pane, where a table overflows and a card grid collapses into a
long vertical scroll.

```go
type CardList struct {
    URI      string       // REQUIRED. ui:// resource URI
    Title    string       // toolbar heading + document title
    Template CardTemplate // REQUIRED

    RowsKey string // structuredContent key holding the rows array. Default "rows"
    RowID   string // record field identifying a card (selection, FromRow/FromSelection). Default "id"

    PageSize    int              // > 0 enables client-side pagination; 0 disables; < 0 invalid
    PageSizes   []int            // alternative page sizes offered in a dropdown on the pagination bar; entries > 0; needs PageSize > 0; PageSize is added if absent; empty renders no chooser
    LoadMore    bool             // grow the strip instead of paging it: starts at PageSize records and appends PageSize more per "Load more" tile at the end of the strip, in place of the pagination bar; needs PageSize > 0; cannot be combined with PageSizes
    DefaultSort *SortSpec        // pre-sort records on load (Key required when set)
    Filterable  bool             // adds a client-side text filter box (matches title, description, body text, and content item values)
    Selection   *SelectionConfig // per-card checkboxes + bulk actions (FromSelection resolves across selected cards)
    Empty       EmptyState

    InitialData map[string]any
    LoadTool    string                 // read tool called once on load to re-fetch records (under RowsKey), replacing the snapshot
    LoadArgs    map[string]any
    Brand       *Brand
    Theme       *theme.Theme
    UI          *uispec.ResourceUIMeta
}
```

The sort control is a dropdown over the template's **sortable content items**
(text/number/date `Content.Items` reading a record field; badge/link items,
fixed-text items and the header slots are not offered) — no config needed.
`DefaultSort` sets the initial order and may reference any field key.

Carousel behavior is automatic: prev/next controls appear only when the cards
overflow the available width and disable at each end, the strip is draggable
with the mouse and swipeable on touch, and its scrollbar is hidden. `PageSize`
still applies and bounds how many cards are in the strip at once. Card width comes from the
`--gadget-card-width` token (default `17rem`), overridable per widget through
`theme.Theme.Extra`.

### 3.12 `Menu`

The app's front door: a responsive grid of tiles, one per tool the server
exposes with a UI. Choosing a tile calls that tool, and the host opens the
widget bound to it — a menu item is navigation, not an action with a result of
its own.

Unlike the data widgets, a `Menu` is **fully authored at registration time**:
the tiles are server-rendered from `Items`, the document carries no
`#gadget-data` island, and the menu reads nothing from `structuredContent`.
The config island holds only the tool name and static args behind each tile,
matched positionally to the rendered buttons.

```go
type Menu struct {
    URI   string      // ui:// resource URI (required)
    Title string      // toolbar + document title
    Intro string      // optional lead text above the tiles
    Items []MenuItem  // at least one required

    Brand *Brand                  // application logo/name
    Theme *theme.Theme            // design token overrides
    UI    *uispec.ResourceUIMeta  // resource _meta.ui override
}

type MenuItem struct {
    Tool         string         // MCP tool called when the item is chosen (required)
    Args         map[string]any // static arguments passed to Tool
    Label        string         // tile heading; defaults to Tool
    Description  string         // supporting line under the label
    IconSVG      string         // inline <svg> markup shown above the label
    Badge        string         // short marker in the tile's top right ("read", "beta")
    BadgeVariant BadgeVariant   // colors the badge; defaults to BadgeNeutral
}
```

```go
menu := &gadget.Menu{
    URI:   "ui://demo/menu",
    Title: "Acme users",
    Intro: "Pick where to start.",
    Items: []gadget.MenuItem{
        {Tool: "list_users", Label: "User table",
         Description: "Sortable, filterable directory.",
         Badge: "read", BadgeVariant: gadget.BadgeInfo},
        {Tool: "edit_user", Args: map[string]any{"id": 1}, Label: "Edit Ada"},
    },
}

// The tool that shows the menu returns no structured data of its own.
type empty struct{}
gosdk.AddWidgetToolFor(server, menu,
    &mcp.Tool{Name: "main_menu", Description: "Show the app menu."},
    func(context.Context, *mcp.CallToolRequest, empty) (*mcp.CallToolResult, empty, error) {
        return nil, empty{}, nil
    })
```

`MenuItem.Args` are fixed values, not row lookups: a menu tile has no record
behind it, so `Static`/`FromRow`/`FromSelection` do not apply here.

Runtime behavior: the whole grid goes inert while a tile's call is in flight
(a second tile would race the first one's view swap), a `loading` status reads
"Opening &lt;label&gt;…", and a tool result that comes back with `isError` is
shown in the status region with the menu left usable. Nothing else is rendered
from the result — the host is expected to take over the view. Tile width comes
from the `--gadget-menu-tile-min` token (default `11rem`), overridable per
widget through `theme.Theme.Extra`.

Documents are self-contained, so `IconSVG` is inline markup, never a URL — the
same trust level and the same checks as `Brand.LogoSVG`.

### 3.13 `Descriptions`

A label/value detail list. **Not a widget**: no URI, no `Document()`, not
registerable. It is a shared block embedded by value, used by `Confirm`, `Choice` (both
for the record and per option) and a card's content section.

```go
type Descriptions struct {
    Items []DescriptionItem
}

type DescriptionItem struct {
    Label  string                  // required
    Key    string                  // record field holding the value
    Text   string                  // fixed authored value, used instead of Key
    Type   ColumnType              // ColText (default), ColNumber, ColDate, ColBadge, ColLink
    Format string                  // same Intl format strings as Column.Format
    Badge  map[string]BadgeVariant // value -> variant (ColBadge)
    Link   *LinkSpec               // ColLink; URL comes from the record
    Align  Align
}
```

Exactly one of `Key` and `Text` per item. A `Key` value is read from the
record at runtime and typed/Intl-formatted exactly like a table cell; a `Text`
value is authored in Go and always plain text. `ColActions` is not a valid
item type.

There are no layout options by design: the list flows into as many columns as
the widget's own width allows and collapses to one in a narrow pane. The item
floor is the `--gadget-desc-min` token (default `12rem`), overridable through
`theme.Theme.Extra`. A data-bound value the record does not carry renders as an
em dash rather than vanishing.

### 3.14 `Confirm`

An approval widget: one question, the record it is about, the side effects of
answering yes, and exactly two outcomes. The long form of `Action.Confirm`.

```go
type Confirm struct {
    URI      string        // ui:// resource URI (required)
    Title    string        // toolbar + document title
    Prompt   string        // headline question (required)
    Body     string        // supporting prose
    Severity BadgeVariant  // BadgeInfo (default) | BadgeWarning | BadgeDanger

    Details Descriptions   // the record, bound to rows[0]
    Effects []Effect       // side effects; runtime EffectsKey replaces them

    Acknowledge   string   // checkbox label that must be ticked to enable accept
    TypeToConfirm string   // phrase that must be typed to enable accept

    Accept AcceptSpec      // required
    Reject *RejectSpec     // nil renders no declining button

    RowsKey    string      // default "rows"
    EffectsKey string      // default "effects"
    RowID      string      // default "id"

    InitialData map[string]any  // baked structuredContent snapshot
    LoadTool    string          // read tool called once on load
    LoadArgs    map[string]any

    Brand *Brand
    Theme *theme.Theme
    UI    *uispec.ResourceUIMeta
}

type Effect struct {
    Text     string        // the consequence (required)
    Detail   string        // secondary line
    Value    string        // magnitude at the row end ("128", "4 people")
    Severity BadgeVariant  // colors the row's dot; defaults to BadgeNeutral
}

type AcceptSpec struct {
    Tool           string               // MCP tool called on accept (required)
    Label          string               // defaults to "Confirm"
    Args           map[string]ArgSource // Static / FromRow only
    Variant        ActionVariant        // overrides the variant derived from Severity
    SuccessMessage string               // shown in place of the buttons on success
}

type RejectSpec struct {
    Label   string               // defaults to "Cancel"
    Tool    string               // optional; without it the server never hears the "no"
    Args    map[string]ArgSource // Static / FromRow only
    Message string               // terminal text; defaults to "Cancelled."
}
```

```go
confirm := &gadget.Confirm{
    URI:      "ui://demo/delete-user",
    Prompt:   "Delete Ada Lovelace?",
    Severity: gadget.BadgeDanger,
    Details: gadget.Descriptions{Items: []gadget.DescriptionItem{
        {Label: "User", Key: "name"},
        {Label: "Balance", Key: "balance", Type: gadget.ColNumber, Format: "currency:EUR"},
    }},
    Effects: []gadget.Effect{
        {Text: "Removes the account", Severity: gadget.BadgeDanger},
        {Text: "Deletes audit records", Value: "128", Severity: gadget.BadgeWarning},
    },
    TypeToConfirm: "ada@example.com",
    Accept: gadget.AcceptSpec{Tool: "delete_user", Label: "Delete user",
        Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
        SuccessMessage: "User deleted."},
    Reject: &gadget.RejectSpec{Label: "Keep user"},
}
```

Runtime behavior:

- **Severity** colors the icon and picks the accept button's variant: danger →
  `VariantDanger`, anything else → `VariantPrimary`, unless `Accept.Variant`
  says otherwise.
- **Effects** authored in Go are shown until a payload carries `EffectsKey`,
  which replaces the list wholesale. An effect severity that is not one of the
  `Badge*` values is ignored rather than styled.
- **Guards** gate the accept button: the acknowledgement must be ticked and the
  phrase typed exactly (trimmed). The button is rendered `disabled`
  server-side whenever a guard is configured, so the document is correct before
  the runtime mounts. Enter in the phrase field accepts.
- **Accepting** calls `Accept.Tool`; on success the buttons are replaced by
  `SuccessMessage` (or the result's text). A result with `isError` re-arms the
  widget so a transient failure can be retried.
- **Rejecting** calls `Reject.Tool` when set, then settles with `Message`.
- **The decision is terminal**: after accepting or declining the buttons stay
  gone, even if the host pushes further results (which still refresh the
  details and effects).

### 3.15 `Choice`

A deciding widget: a question, the options answering it, and the case for each
one. Picking is local — only the submit button calls a tool. Use it where
`Confirm` asks yes/no about one operation and the reader has to choose *which*
operation instead.

```go
type Choice struct {
    URI    string          // ui:// resource URI (required)
    Title  string          // toolbar + document title
    Prompt string          // headline question (required)
    Body   string          // supporting prose

    Layout   ChoiceLayout  // ChoiceAuto (default) | ChoiceSplit | ChoiceStacked
    Multiple bool          // checkboxes instead of radios
    Min      int           // fewest options a multiple choice accepts (default 1)
    Max      int           // most it accepts; 0 = no limit. Multiple only

    Options []ChoiceOption // may be empty when they arrive under OptionsKey
    Details Descriptions   // the record the question is about, bound to rows[0]

    Submit ChoiceSubmit    // required
    Cancel *RejectSpec     // nil renders no declining button

    RowsKey    string      // default "rows"
    OptionsKey string      // default "options"
    RowID      string      // default "id"

    InitialData map[string]any  // baked structuredContent snapshot
    LoadTool    string          // read tool called once on load
    LoadArgs    map[string]any

    Brand *Brand
    Theme *theme.Theme
    UI    *uispec.ResourceUIMeta
}

type ChoiceOption struct {
    Value   string        // sent to the tool (required, unique)
    Label   string        // list heading; defaults to Value
    Summary string        // one supporting line, always visible in the list

    Body    string        // prose in the description block
    Bullets []string      // short points under Body
    Details Descriptions  // label/value list; Key items read Data
    Data    map[string]any // the option's own record

    Badge        string        // short text beside the label
    BadgeVariant BadgeVariant  // defaults to BadgeNeutral; needs Badge

    Default  bool         // preselected (a single choice takes at most one)
    Disabled bool         // on offer, but not choosable now
}

type ChoiceSubmit struct {
    Tool           string               // MCP tool called on submit (required)
    Label          string               // defaults to "Continue"
    ValueArg       string               // argument carrying the decision; defaults to "choice"
    Args           map[string]ArgSource // Static / FromRow only
    Variant        ActionVariant        // defaults to VariantPrimary
    SuccessMessage string               // shown in place of the controls on success
}
```

`Cancel` reuses `RejectSpec` (§3.14): `Label` (default "Cancel"), an optional
`Tool` so the server hears the "no", its `Args`, and the terminal `Message`
(default "Cancelled.").

```go
choice := &gadget.Choice{
    URI:    "ui://demo/shipping",
    Prompt: "How should we ship order ORD-4471?",
    Details: gadget.Descriptions{Items: []gadget.DescriptionItem{
        {Label: "Order", Key: "reference"},
    }},
    Options: []gadget.ChoiceOption{
        {
            Value: "standard", Label: "Standard", Summary: "3-5 business days",
            Body:    "Handed to the postal service tonight.",
            Bullets: []string{"Tracked to the depot", "No signature"},
            Details: gadget.Descriptions{Items: []gadget.DescriptionItem{
                {Label: "Price", Key: "price", Type: gadget.ColNumber, Format: "currency:EUR"},
            }},
            Data:    map[string]any{"price": 4.9},
            Default: true,
        },
        {Value: "express", Label: "Express", Summary: "next business day",
         Badge: "fastest", BadgeVariant: gadget.BadgeSuccess},
    },
    Submit: gadget.ChoiceSubmit{Tool: "ship_order", Label: "Ship it", ValueArg: "method",
        Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
        SuccessMessage: "On its way."},
    Cancel: &gadget.RejectSpec{Label: "Decide later"},
}
```

The submit call is `Args` resolved as usual plus `ValueArg`: the chosen
`Value` (single) or the array of chosen values in option order (multiple). So
the example above calls `ship_order` with `{"id": 4471, "method": "express"}`.

Runtime behavior:

- **Layout** decides where an option's description block goes. `ChoiceSplit`
  puts it in a side panel that follows the option in hand; `ChoiceStacked`
  unfolds it inside the chosen option. `ChoiceAuto` measures the width the
  host gave the widget and picks split at or above 34rem, stacked below —
  re-measuring as the pane resizes, so one document reads in a wide canvas and
  a narrow chat column. The server-rendered class is `--auto`, which styles as
  stacked, so a document whose script never ran is still correct.
- **Options** authored in Go are shown until a payload carries `OptionsKey`,
  which replaces the list wholesale and re-applies the new options' own
  defaults. A tool-supplied option describes itself in plain values — its
  `details` entries are `{label, value}` pairs, already formatted server-side,
  where an authored `Details` item is a typed field config resolved against
  `Data`. A `badgeVariant` that is not a `Badge*` value is ignored rather than
  styled.
- **Selection**: a single choice is radios and swaps; a multiple choice is
  checkboxes. Submit stays disabled until `Min` (default 1) is met; at `Max`
  the unticked options disable rather than failing on click, and a hint under
  the list tracks the count. `Disabled` options are never chosen, but pointing
  at one still shows its description — why it cannot be taken is the thing
  worth reading.
- **Submitting** calls `Submit.Tool`; on success the controls are replaced by
  `SuccessMessage` (or the result's text). A result with `isError` re-arms the
  widget so a transient failure can be retried.
- **Cancelling** calls `Cancel.Tool` when set, then settles with `Message`.
- **The decision is terminal**: after submitting or cancelling the controls
  stay gone, even if the host pushes further results (which still refresh the
  details and options).

### 3.16 `Brand`

Identifies the application a widget belongs to. Available on `Table`, `Form`,
`Card`, `CardList` and `Menu` as the `Brand` field; one `*Brand` is typically
shared across every widget of a server. It always renders at the top left of the
widget chrome, as the first item of the toolbar, before the title.

```go
type Brand struct {
    Name        string // application name; required unless a logo is set
    URL         string // optional http(s) link, opened through the host (ui/openLink)
    LogoSVG     string // inline <svg> markup (recommended)
    LogoDataURI string // "data:image/...;base64,..." alternative to LogoSVG
    LogoAlt     string // alt text for LogoDataURI; defaults to Name
}
```

Documents are self-contained, so a logo is never a URL. Prefer `LogoSVG`: it is
plain markup and needs nothing from the host's CSP. `LogoDataURI` renders as an
`<img>` and therefore depends on the host allowing `img-src data:`, which the
spec does not guarantee. A `Brand` with `URL` renders as a button, not an
anchor — navigation is blocked in the host's sandboxed iframe, so the runtime
hands the URL to `ui/openLink`.

A brand makes the toolbar appear even when `Title` is empty.

### 3.17 Validation rules (what `Validate()` / `Document()` reject)

Table:
- `URI` must be a well-formed `ui://` URI with a non-empty path.
- At least one column; no duplicate column `Key`s.
- text/number/date/badge columns: `Key` required.
- link columns: `Link.HrefKey` required.
- actions columns: at least one action.
- `PageSize >= 0`; `PageSizes` entries `> 0` and only with `PageSize > 0`;
  `DefaultSort.Key` required when `DefaultSort` is set.
- Actions: `Label` required; `Tool` required for tool kind; `HrefKey` required
  for link kind; all `Args` built with the constructors; `FromSelection` only
  in bulk actions.
- `Theme` must pass `theme.Validate()`.

Form:
- `URI` as above; at least one field; `Submit.Tool` required.
- Field `Name` required and unique.
- `FSelect`/`FMultiSelect` require non-empty `Options`.
- `Theme` must pass `theme.Validate()`.

Card / CardList (via `CardTemplate`):
- `URI` as above; `Template.Header.TitleKey` required.
- `Header.Badge`, when present (`Key` set), must be a badge column, and cannot
  be combined with `Header.Action` — the header has one end slot.
- Each text slot rejects being filled twice: `Header.DescriptionKey` +
  `Header.Description`, `Content.TextKey` + `Content.Text`, `Footer.TextKey` +
  `Footer.Text`.
- `Content.Items` validated as `Descriptions` (§3.13): `Label` required;
  `Key` xor `Text`; text/number/date/badge need `Key`; link needs
  `Link.HrefKey`; no duplicate item `Key`s.
- `Header.Action` and `Footer.Actions`: validated like any action;
  `FromSelection` is rejected (per-card actions run on one record).
- CardList only: `PageSize >= 0`; `PageSizes` entries `> 0` and only with
  `PageSize > 0`; `LoadMore` needs `PageSize > 0` and rejects `PageSizes`;
  `DefaultSort.Key` required when set; bulk actions validated.
- `Theme` must pass `theme.Validate()`.

Menu:
- `URI` as above; at least one item.
- Item `Tool` required.
- Item `IconSVG`, when set, must pass the same `<svg>` checks as
  `Brand.LogoSVG` (below).
- Item `BadgeVariant`, when set, must be one of the `Badge*` constants.
- `Theme` must pass `theme.Validate()`.

Confirm:
- `URI` as above; `Prompt` required; `Accept.Tool` required.
- `Severity`, when set, must be one of the `Badge*` constants.
- `Accept.Args` / `Reject.Args`: built with `Static` or `FromRow`;
  `FromSelection` is rejected (a confirmation has no selection).
- `Reject.Args` require `Reject.Tool`.
- Effects: `Text` required; `Severity`, when set, must be a `Badge*` constant.
- `Details` validated as `Descriptions` (below).
- `Theme` must pass `theme.Validate()`.

Choice:
- `URI` as above; `Prompt` required; `Submit.Tool` required.
- `Layout`, when set, must be `ChoiceSplit` or `ChoiceStacked`.
- `Submit.Args` / `Cancel.Args`: built with `Static` or `FromRow`;
  `FromSelection` is rejected (a choice has no row selection).
- `Submit.Args` cannot contain `Submit.ValueArg` — the decision and a static
  argument cannot share a name.
- `Cancel.Args` require `Cancel.Tool`.
- `Min`/`Max` require `Multiple`, cannot be negative, and `Min <= Max` when
  `Max > 0`.
- Options: `Value` required and unique; `BadgeVariant`, when set, must be a
  `Badge*` constant and needs `Badge` text; no empty `Bullets` entry; a
  `Disabled` option cannot be `Default`; at most one `Default` in a single
  choice, and no more than `Max` in a multiple one.
- Option `Details` and widget `Details` validated as `Descriptions` (below).
- An empty `Options` list is legal: options may arrive at runtime.
- `Theme` must pass `theme.Validate()`.

Descriptions (wherever embedded):
- Item `Label` required.
- Exactly one of `Key` and `Text` per item; a `Text` item must be `ColText`.
- text/number/date/badge items need `Key`; link items need `Link.HrefKey`.
- No duplicate item `Key`s; `ColActions` is rejected.

Brand (all widgets, when set):
- `Name` or a logo is required.
- `LogoSVG` and `LogoDataURI` are mutually exclusive.
- `LogoSVG` must be a single `<svg>…</svg>` element and is rejected when it
  contains `<script>`, an `on*=` event handler, `<foreignObject>`, `<iframe>`,
  `<embed>`, `<object>`, `<use>`, `<animate>`, `<set>`, `javascript:`, an HTML
  comment, or `</style`.
- `LogoDataURI` must be `data:image/{png,jpeg,gif,webp,svg+xml};base64,` with a
  non-empty base64 payload.
- `URL`, when set, must be `http://` or `https://`.

`Document()` calls `Validate()` first and returns its error, so with `gosdk`
registration you get configuration errors at startup, not at render time.

---

## 4. The runtime data contract (structuredContent keys)

Widgets read all runtime data from the tool result's `structuredContent`:

| Widget | Key (configurable via) | Shape | Meaning |
|---|---|---|---|
| Table | `rows` (`RowsKey`) | `[]object` | rows to render |
| CardList | `rows` (`RowsKey`) | `[]object` | records to render as cards |
| Card | `rows` (`RowsKey`) | `[]object` | renders the first element (`rows[0]`) |
| Form | `values` (`PrefillKey`) | `{field: value}` | prefill (edit flows) |
| Form | `errors` (`ErrorsKey`) | `{field: "message"}` | server-side field errors, rendered inline; marks the submit failed |
| Confirm | `rows` (`RowsKey`) | `[]object` | the record the operation targets (`rows[0]`) |
| Confirm | `effects` (`EffectsKey`) | `[]object` | side effects: `{text, detail?, value?, severity?}`; replaces the authored list |
| Choice | `rows` (`RowsKey`) | `[]object` | the record the question is about (`rows[0]`) |
| Choice | `options` (`OptionsKey`) | `[]object` | what is on offer: `{value, label?, summary?, body?, bullets?, details?, badge?, badgeVariant?, default?, disabled?}` where `details` is `[{label, value}]`; replaces the authored list |
| Menu | — | — | reads nothing; tiles are authored and server-rendered |

`Card`/`CardList` share the `rows` contract with `Table`: an action or tool
result whose `structuredContent` contains `RowsKey` re-renders the widget
(`CardList` also clears the selection), so the same `list_users`/`delete_user`
tools drive a table or a card list interchangeably.

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

### Embedded per-call delivery (result-embedded widgets)

Besides the spec's template model (register the `ui://` resource once, the
host fetches it and pushes per-call data), a widget can be delivered
**embedded in the tool result**: build it per call with the data baked in
(`InitialData` — the runtime paints it before the handshake completes), give
it a **unique URI per render**, and append the rendered `Document()` to the
result's `content` as an embedded resource:

```
{type:"resource", resource:{uri, mimeType:"text/html;profile=mcp-app", text: doc}}
```

The mimeType MUST be `uispec.MIMEType` (`text/html;profile=mcp-app`) — that
profile is what tells the host to attach the MCP Apps bridge (handshake,
`tools/call`, size reporting) instead of rendering a dead static iframe.
There is no legacy mcp-ui interop: the runtime speaks only the MCP Apps
protocol, and actions in a host without it will fail with a request timeout.

Size reporting starts at first paint (not gated on the handshake) so the
host can grow the iframe immediately; the document resets
`body{margin:0;padding:8px}` so `body.scrollHeight` measures true content
height (margin sits outside it and would clip the bottom edge).

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

    Transparent bool   // drop the page fill and the gutter: the iframe rectangle disappears,
                       // leaving only the widget's card on the host surface. Card, control
                       // and overlay fills are untouched. == ColorPage "transparent" + PagePad "0"
    ColorPage   string // page fill alone (cards/controls/overlays keep ColorBackground); ignored when Transparent
    PagePad     string // gutter between widget and iframe edge (default 8px); ignored when Transparent

    Extra map[string]string // extra/override raw custom properties; keys MUST start with "--gadget-"
}

func (t *Theme) CSS() string      // ":root{...}" (page tokens) + ".gadget-root{...}", "" when nothing set
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

### Embedding without a visible frame

`Transparent: true` makes the widget document paint nothing of its own, so the
host page shows through the iframe and only the card (with its border radius)
reads as part of the host UI. Two things it depends on:

- The **host** must leave the `<iframe>` element unpainted: `border: 0`
  (the UA default is `2px inset`) and no `background`.
- The embedded document's **root color scheme must match the `<iframe>`
  element's**, or the UA paints an opaque canvas behind the whole document and
  no author `background: transparent` can undo it. The runtime handles this by
  pinning `:root { color-scheme }` to `hostContext.theme`.

Content still cannot escape the iframe box: dropdown panels, tooltips and
focus rings are clipped by the frame, and the frame rectangle keeps swallowing
pointer events over its transparent areas.

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

// Presence marker for a requested sandbox permission. Serializes to {} when
// set (non-nil) and is omitted when nil, per the MCP Apps spec.
type Permission struct{}

// Marker used to request a permission, e.g. Permissions{Camera: uispec.Grant}.
var Grant = &Permission{}

// Browser capabilities a UI resource requests. Serializes to the spec object
// shape, e.g. {"camera":{},"clipboardWrite":{}}.
type Permissions struct {
    Camera         *Permission `json:"camera,omitempty"`
    Microphone     *Permission `json:"microphone,omitempty"`
    Geolocation    *Permission `json:"geolocation,omitempty"`
    ClipboardWrite *Permission `json:"clipboardWrite,omitempty"`
}

// External origins a UI resource needs (hosts default to fully locked-down).
type CSP struct {
    ConnectDomains  []string `json:"connectDomains,omitempty"`
    ResourceDomains []string `json:"resourceDomains,omitempty"`
    FrameDomains    []string `json:"frameDomains,omitempty"`
    BaseURIDomains  []string `json:"baseUriDomains,omitempty"`
}

// _meta.ui on a ui:// resource (set via Table.UI / Form.UI).
type ResourceUIMeta struct {
    CSP           *CSP         `json:"csp,omitempty"`
    Permissions   *Permissions `json:"permissions,omitempty"`
    Domain        string       `json:"domain,omitempty"`
    PrefersBorder *bool        `json:"prefersBorder,omitempty"`
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
w := table // or form, card, cardlist; any gadget.Widget

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
3. **Mutating table/cardlist tools must return the updated row list** under
   `RowsKey`, or the UI keeps showing stale rows (a `Card` re-renders `rows[0]`).
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
  actions, the same users as a card carousel, + edit form with server-side
  validation, prefill, string-ID parsing). `go run ./examples/demo -addr :8080`
  (streamable HTTP at `/mcp`) or `go run ./examples/demo -stdio`. Point MCPJam,
  Claude custom connectors, or any MCP Apps host at `http://localhost:8080/mcp`.
- `examples/harness` — a fake MCP Apps host in one HTML page, with a story
  browser: a rail of widget variants (table, cardlist, card, form, menu, plus
  empty states and a long list) defined in `examples/harness/stories.go` and
  served one per route (`/story/<id>`, catalog at `/stories.json`). It renders
  the selected story in a sandboxed iframe at a chosen viewport width, answers
  the `ui/initialize` handshake, logs all JSON-RPC traffic (expandable
  entries), follows `ui/notifications/size-changed`, and simulates tool
  results/errors and theme changes (with a stateful in-memory backend so
  row/bulk actions update live). `go run ./examples/harness`, open
  `http://localhost:8090`. Use it to verify widget behavior without any real
  MCP client. Add a story by appending to `catalog()` and writing its builder.

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
