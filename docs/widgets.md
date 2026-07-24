# Widget reference

Every widget implements `gadget.Widget`: it renders one self-contained HTML
document (`Document()`), registers as one `ui://` resource
(`Descriptor()`), and links to tools via `ToolMeta()`. With the go-sdk
adapter you rarely call these yourself — see `gadget/gosdk`.

## Data contract

Widgets read runtime data from the tool result's `structuredContent`:

| Widget | Key (default) | Shape |
|---|---|---|
| Table | `rows` (`RowsKey`) | `[]object` — one JSON object per row |
| CardList | `rows` (`RowsKey`) | `[]object` — one JSON object per card |
| Card | `rows` (`RowsKey`) | `[]object` — renders the first element (`rows[0]`) |
| Form | `values` (`PrefillKey`) | `{field: value}` prefill |
| Form | `errors` (`ErrorsKey`) | `{field: "message"}` server-side errors |

`gadget.RowsOf(slice)` converts typed Go slices to row maps (honors json
tags).

## Table

```go
table := &gadget.Table{
    URI:   "ui://myapp/users",
    Title: "Users",
    Columns: []gadget.Column{
        gadget.Text("name", "Name"),
        gadget.Number("balance", "Balance", "currency:EUR"),
        gadget.Date("createdAt", "Created", "date"),
        gadget.Badge("status", "Status", map[string]gadget.BadgeVariant{
            "active": gadget.BadgeSuccess,
            "banned": gadget.BadgeDanger,
        }),
        gadget.Link("website", "Website"),
        gadget.ActionsColumn(
            gadget.Action{
                Label: "Delete", Tool: "delete_user",
                Variant: gadget.VariantDanger, Confirm: "Really delete?",
                Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")},
            },
        ),
    },
    PageSize:    10,
    DefaultSort: &gadget.SortSpec{Key: "name"},
    Filterable:  true,
    Selection: &gadget.SelectionConfig{Bulk: []gadget.Action{{
        Label: "Archive", Tool: "archive_users",
        Args: map[string]gadget.ArgSource{"ids": gadget.FromSelection("id")},
    }}},
    Empty: gadget.EmptyState{Title: "No users"},
}
```

- **Column types**: `text`, `number`, `date`, `badge`, `link`, `actions`.
  Number formats: `int`, `decimal:<digits>`, `percent`, `currency:<code>`;
  date formats: `date`, `datetime`, `time`, `relative` — all rendered via
  `Intl` in the host's locale/time zone.
- **Sorting/filtering/pagination** are client-side over the delivered rows.
  Text/number/date columns sort by default (`Sortable` overrides).
- **RowID** (default `"id"`) identifies rows for selection and args.
- **Actions**: `Kind` tool (default) calls an MCP tool; `Kind` link opens
  `HrefKey` via `ui/open-link`. Arg sources: `Static(v)`, `FromRow(field)`,
  `FromSelection(field)` (bulk only). `Confirm` renders an inline two-phase
  confirmation (native `confirm()` doesn't work in sandboxed iframes).
- If an action's result contains `RowsKey`, the table re-renders with the
  returned rows and clears the selection — return the updated list from
  mutating tools.

## Form

```go
form := &gadget.Form{
    URI:   "ui://myapp/user-form",
    Title: "Edit user",
    Fields: []gadget.Field{
        {Name: "id", Type: gadget.FHidden},
        {Name: "name", Label: "Name", Required: true},
        {Name: "email", Label: "Email", Required: true,
            Validation: &gadget.Validation{
                Pattern: `[^@\s]+@[^@\s]+`, Message: "Enter a valid email.",
            }},
        {Name: "role", Label: "Role", Type: gadget.FSelect, Required: true,
            Options: []gadget.Option{gadget.Opt("user"), gadget.Opt("admin")}},
        {Name: "active", Label: "Active", Type: gadget.FCheckbox, Default: true},
    },
    Submit: gadget.SubmitSpec{Tool: "save_user", SuccessMessage: "Saved."},
    Cancel: &gadget.CancelSpec{},
}
```

- **Field types**: `text`, `textarea`, `number`, `checkbox`, `select`,
  `multiselect`, `date`, `time`, `hidden`, `readonly`.
- **Client validation** renders as native HTML attributes (`required`,
  `pattern`, `min`/`max`/`step`, `minlength`/`maxlength`) and is enforced
  before submit, with inline error messages (`Validation.Message` overrides
  the browser text).
- **Submit** calls `Submit.Tool` with `{field: value}` merged over
  `StaticArgs`. Value types: checkbox → bool, number → number (omitted when
  empty), multiselect → []string, everything else → string (hidden fields
  included — parse server-side).
- **Server-side errors**: return `{ErrorsKey: {"field": "message"}}` in
  `structuredContent`; they render inline and mark the form failed.
- **Edit mode**: a model-invoked tool linked to the form (e.g. `edit_user`)
  returns `{PrefillKey: {...}}`; the form prefills from the tool result.

## Card and CardList

`Card` renders one record; `CardList` renders many records as cards in a
responsive grid with the same client-side machinery as `Table` (filter, sort,
pagination, selection + bulk actions, per-card actions). Both share a
`CardTemplate` and read the same `rows` contract as `Table` — so the same
list/mutation tools drive either.

```go
tmpl := gadget.CardTemplate{
    TitleKey:    "name",
    SubtitleKey: "email",
    Badge: gadget.Badge("status", "Status", map[string]gadget.BadgeVariant{
        "active": gadget.BadgeSuccess,
        "banned": gadget.BadgeDanger,
    }),
    Fields: []gadget.Column{
        gadget.Number("balance", "Balance", "currency:EUR"),
        gadget.Date("createdAt", "Joined", "relative"),
        gadget.Link("website", "Website"),
    },
    Actions: []gadget.Action{
        {Label: "Edit", Tool: "edit_user", Variant: gadget.VariantPrimary,
            Args: map[string]gadget.ArgSource{"id": gadget.FromRow("id")}},
    },
}

cards := &gadget.CardList{
    URI:         "ui://myapp/users",
    Title:       "Users",
    Template:    tmpl,
    PageSize:    12,
    DefaultSort: &gadget.SortSpec{Key: "balance", Desc: true},
    Filterable:  true,
    Selection: &gadget.SelectionConfig{Bulk: []gadget.Action{{
        Label: "Archive", Tool: "archive_users",
        Args: map[string]gadget.ArgSource{"ids": gadget.FromSelection("id")},
    }}},
    Empty: gadget.EmptyState{Title: "No users"},
}

card := &gadget.Card{URI: "ui://myapp/user", Title: "User", Template: tmpl}
```

- **Template**: `TitleKey` (required) and optional `SubtitleKey` pull the card
  heading from row fields; `Badge` (a badge column) shows a status pill;
  `Fields` are typed label/value rows (same `Column` types and formats as a
  table, minus `actions`); `Actions` render as footer buttons.
- **CardList** reuses `RowsKey`/`RowID`, `PageSize`, `DefaultSort`,
  `Filterable`, and `Selection` exactly as `Table`. The sort control is a
  select auto-derived from the sortable body fields; filtering matches title,
  subtitle, and field values.
- **Card** renders `rows[0]`; use it as a detail view. Both support
  `InitialData` and `LoadTool`/`LoadArgs` load-time hydration, and re-render
  when an action or tool result returns `RowsKey`.
- **Actions** behave exactly as in tables (`Static`/`FromRow` args, inline
  `Confirm`, `Variant`); `FromSelection` is bulk-only via
  `CardList.Selection`.

## Wiring with the official Go SDK

```go
server := mcp.NewServer(&mcp.Implementation{Name: "myapp"}, gosdk.EnableUI(nil))

// Model-visible tool rendered by the table:
gosdk.AddWidgetToolFor(server, table,
    &mcp.Tool{Name: "list_users", Description: "List users."}, listUsers)

// App-only tool (fired by row actions, hidden from the model):
del := &mcp.Tool{Name: "delete_user", Description: "Delete a user."}
gosdk.AppOnly(del, table)
gosdk.AddWidgetToolFor(server, table, del, deleteUser)
```

`examples/demo` is a complete runnable server; `examples/harness` is a fake
host for manual verification without any MCP client.
