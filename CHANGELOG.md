# Changelog

## v0.7.1 - 2026-07-27

- **`Descriptions` items can ask, not only state.** A `DescriptionItem` with an
  `Input` renders a control in its value cell — `InputText` (default),
  `InputNumber`, `InputSelect` (a gomukit dropdown over `Options`) or
  `InputCheckbox` — with `Placeholder`, `Required`, `Default` and the same
  `Validation` a form `Field` takes. What the reader puts in it is collected at
  call time and merged into the widget's own call: `DatePicker.Details` sends it
  with the picked date(s) (or appends it to the turn when `ChatPrompt` is set),
  and a card's `Content.Items` send it with every action button of that card —
  per record, so a `CardList` keeps each card's answers to itself and bulk
  actions get none. `Confirm.Details` and a `Choice`'s details stay read-only
  and reject an `Input` at validation time. Prefill order is the reader's own
  answer, then the record field named by `Key`, then `Default`; answers survive
  the re-render a tool result causes. Native constraint validation runs before
  the call and shows the message under the offending control, and an input name
  colliding with an argument the call already builds is a validation error.

## v0.6.0 - 2026-07-26

- **New widget `DatePicker`**, and a calendar for forms. One grid, two
  mountings: `DatePicker` renders it inline as a view of its own (a question, an
  inline calendar, a line stating what is picked, submit and cancel — the same
  frame as `Choice`, with the same terminal decision and re-arm-on-error
  behaviour), while a `Form`'s `FDate` and the new `FDateRange` open it in a
  popover over their native date inputs. The inputs survive the upgrade as the
  value holders, exactly as a `<select>` does under the gomukit dropdown, so
  validation and submitted values are unchanged and a document whose script
  never runs still has a working control.
  Dates travel as `"YYYY-MM-DD"` throughout: `Submit.ValueArg` (default
  `"date"`, or `"start"` in a range) and `Submit.EndArg` (default `"end"`) are
  two flat string arguments, and a range field sends `Name` plus `EndName`
  (default `Name + "_end"`). The host's time zone decides one thing only: which
  day is today.
- **New shared block `Calendar`**, configuring that grid for both: `Min`/`Max`
  bounds, `Disabled` days (a range may not straddle one), `DisableWeekends`,
  `Months` shown at once (1 for a date, 2 for a range; side by side where
  there is room, wrapped under each other where there is not),
  `WeekNumbers` (ISO 8601), `MonthDropdowns` with `FromYear`/`ToYear`,
  `WeekStart`, `StartOn`, and `Presets`. A `DatePreset` names either a fixed
  window or a `DateSpan` — `SpanToday`, `SpanLast7Days`, `SpanThisMonth`,
  `SpanYearToDate` and the rest — resolved at runtime against the reader's
  today, since a server cannot name those dates at registration time. Month and
  weekday names, the first day of the week and the formatted value all come from
  the host locale, so the grid is built at runtime and rebuilt on a host-context
  change. Full keyboard control: one tab stop, arrows by day and week,
  PageUp/PageDown by month (with Shift, by year), Home/End across the week.
- A `DatePicker` reads its selection and its limits from one runtime key,
  `ValueKey` (default `"value"`): `"YYYY-MM-DD"`, or
  `{start, end, min, max, disabled}` — which days are still free is exactly what
  changes between registration and the question, so `LoadTool` fetches it fresh.
- **Widget actions can go through the chat.** `Action.Prompt` (row, bulk and
  card actions), `MenuItem.Prompt`, and `ChatPrompt` on `Confirm`'s accept,
  `Choice`'s submit and `DatePicker`'s submit ask the host to post a user
  message (`ui/message`) instead of calling the tool from the view. It is for
  hosts that answer a view-initiated `tools/call` out of band: they run the
  call without opening the widget bound to it, so an action whose whole point
  is opening another view looks inert. On the chat path the model makes the
  call, so `Args` are ignored and left out of the config island — the text
  should read as the request a user would type. A `Choice` appends what was
  picked and a `DatePicker` the date, since a chat turn has no argument to
  carry them. Link actions cannot set it.
- Popups nest: a panel opened from inside another (the calendar's month and year
  dropdowns) no longer closes its parent, and an outside press peels the stack
  from the top.


## v0.5.0 - 2026-07-26

- **`CardTemplate` is now three sections** (breaking): `Header`
  (`TitleKey`, `DescriptionKey`/`Description`, and one end slot holding either
  `Badge` or `Action`), `Content` (`TextKey`/`Text` body prose plus `Items`, a
  `Descriptions` block) and `Footer` (`TextKey`/`Text` note plus `Actions`).
  Replaces the flat `TitleKey`/`SubtitleKey`/`Badge`/`Fields`/`Actions` shape;
  body fields move from `[]Column` to `Descriptions`, so cards, confirmations
  and any future detail view format identically and a missing value renders as
  an em dash. A section with nothing in it is not rendered. The card's spacing
  is one rhythm, `--gomu-card-spacing`, used as both the gap between sections
  and their inset; a footer that follows content is separated by a rule.
  `CardList` derives its sort options from the sortable `Content.Items` and
  filters across title, description, body text and item values.

- **New widget `Confirm`**: an approval view for operations that need weighing
  before they run. It states the question (`Prompt`, `Body`, `Severity`), shows
  the record it targets (`Details`, a `Descriptions` block bound to `rows[0]`)
  and the consequences (`Effects` — text, detail, magnitude, severity), and
  offers exactly two outcomes: `Accept` calls a tool, `Reject` optionally calls
  one. Optional guards gate the accept button — `Acknowledge` (a checkbox) and
  `TypeToConfirm` (a phrase typed exactly); the button renders `disabled`
  server-side whenever one is configured. The decision is terminal: on success
  the buttons are replaced by the outcome, while an `isError` result re-arms the
  widget for a retry. Effects authored in Go are replaced wholesale by any
  delivered under `EffectsKey` (default `"effects"`), so a server can report
  what this particular call will cost.
- **New widget `Choice`**: a deciding view for questions with more than one
  answer. It states the question (`Prompt`, `Body`), optionally the record it
  is about (`Details`, bound to `rows[0]`), and the options answering it —
  each a `ChoiceOption` with a `Label`/`Summary` for the list and `Body`,
  `Bullets` and its own `Details` (resolved against the option's `Data`) for
  the description block. Picking is local; only `Submit` calls a tool, with the
  chosen `Value` — or the array of them — under `Submit.ValueArg` (default
  `"choice"`). `Multiple` turns the radios into checkboxes bounded by
  `Min`/`Max`, where the unticked options disable at the cap. `Layout` places
  the description block: `ChoiceSplit` in a side panel, `ChoiceStacked` under
  its option, and the default `ChoiceAuto` measures the width the host gave the
  widget and switches between the two as the pane resizes. Options authored in
  Go are replaced wholesale by any delivered under `OptionsKey` (default
  `"options"`), so a server can offer what is actually available at call time.
  The decision is terminal, with the same re-arm-on-error behavior as
  `Confirm`.
- **New shared block `Descriptions`**: a label/value detail list
  (`DescriptionItem` — `Label` plus either a record `Key`, typed and
  Intl-formatted like a table cell, or authored `Text`). No layout options: it
  takes as many columns as the widget's width allows and collapses to one in a
  narrow pane, with the item floor in the new `--gomu-desc-min` token
  (default `12rem`). Missing data-bound values render as an em dash.

- **Table actions are menus**: an actions column now renders one `⋯` trigger
  per row and the bulk bar a single "Actions" trigger, each opening a menu of
  the authored labels, where a `Confirm` is also asked (the item swaps to the
  confirmation text and waits for a second choice). The column costs the same
  width whether it holds one action or five, so the row's data keeps the space
  the buttons used to take. No API change: `ActionsColumn` and
  `SelectionConfig.Bulk` are authored exactly as before. `Card`/`CardList`
  actions are unchanged. Internally the menu shares one open-popup slot and
  placement with the `<select>` dropdown (`ui/src/popup.ts`), so a widget can
  never show two panels at once.

- **New example `examples/preview`**: an MCP server built for inspecting the
  library through a real client rather than the fake host in
  `examples/harness`. Two halves: a small application with mutable state
  (customers and orders, listed, edited, confirmed and shipped through the
  widgets) and a gallery registering every widget variant as its own tool and
  `ui://` resource. `make preview` serves it at `http://localhost:8081/mcp`;
  `-stdio`, `-mode` and `-quiet` cover the other ways to run it. See
  `docs/preview.md`. No library change.

## v0.4.3 - 2026-07-24

- **Fix iframe width collapse (`size-changed`)**: `watchSize` now reports
  **height only** and omits `width` from `ui/notifications/size-changed`. Hosts
  that pin `iframe.style.width` to a reported value (e.g. the MCP Inspector)
  coupled the frame to the view's own inner measurement — the vertical scrollbar
  and a wrapping toolbar shaved pixels off every read, so a wide/tall widget
  (e.g. a many-column table) ratcheted the frame to zero width. The view now
  lets the host own width (iframe fills available space); only height is
  content-driven. Supersedes the v0.4.2 `clientWidth` change, which reported a
  different value on the same axis and still ratcheted. `Bridge.sizeChanged`
  signature is now `(height, width?)` with width optional and omitted by default.

## v0.2.0 - 2026-07-23

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

## v0.1.0 - 2026-07-23

Initial scaffold:

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
- Theming: `--gomu-*` token system defaulting to host-injected variables;
  `theme.Theme` override struct; dark mode via host theme +
  `prefers-color-scheme` fallback.
- `gosdk` adapter for `modelcontextprotocol/go-sdk` (extension capability,
  widget/tool registration, app-only visibility, app-only result data).
- Examples: `demo` MCP server (streamable HTTP/stdio), `harness` fake host.

Before v0.1.0: re-verify spec wording against the MCP core 2026-07-28
release.
