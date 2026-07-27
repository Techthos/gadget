# <img src="assets/gomukit-icon.svg" alt="" width="28" align="center"> Extensibility implementation plan

Companion to `docs/extensibility.md`, which argues the design. This is the
work: ordered steps, files touched, tests, and the decisions each step
assumes. Levels are independently shippable; level 3 is much larger than the
other two combined and is written up in full here so its cost is visible
before anyone starts level 1.

## Decisions this plan assumes

Resolving the design doc's open questions. All are reversible before the first
release that ships them; none are reversible after.

| # | Question | Decision | Reason |
|---|---|---|---|
| 1 | Package placement | New `gomukit/compose` | Keeps the root package about widgets. The root API is already large (Table, Form, Card, CardList, Menu, Confirm, Descriptions, Brand, Action). |
| 2 | TypeScript types | Emit `ui/types/gomukit.d.ts` with `tsc --emitDeclarationOnly` from `ui/src/public.ts`, commit it, document copying it. `ui/package.json` stays private | No npm publishing pipeline, no registry account, deterministic and verifiable in CI like `dist/`. |
| 3 | `Custom.Body` type | `g.Node` only | A guarded raw HTML string cannot preserve "data reaches HTML only through text nodes". Authors who want strings can call `gomponents.Raw` themselves and own the consequence. |
| 4 | Kind namespacing | No forced prefix. Kind must match `^[a-z][a-z0-9-]*$` and not be one of the reserved built in names | A prefix is ceremony; a reserved list plus a validation error is enough, and reserved names are cheap to extend. |
| 5 | `ExtraJS` cardinality | `[]string`, emitted in order, one `<script>` each | Level 3 mounts several components in one document, each possibly bringing its own script. Deduplication is the author's problem until proven otherwise. |

Additional standing decision: `internal/htmlx` and `internal/assets` stay
internal. `compose` is a curated public facade over them, not a rename, so the
public surface is a deliberate subset.

## Level 1: custom Go widgets

Outcome: a third party can render a self contained document with gomukit CSS,
theme, host context, brand chrome and sizing, without custom JavaScript.

### L1.1 Extract shared chrome into `internal/chrome`

Prerequisite for exposing chrome publicly without an import cycle: `compose`
must import the root package for `gomukit.EmptyState` and `gomukit.Brand`, so
the root package cannot import `compose`.

- New `internal/chrome/chrome.go` holding what `render_shared.go`,
  `brand_render.go` and `check_render.go` do today, parameterised by plain
  values rather than root types: `Status()`, `Empty(title, body string)`,
  `Pagination(sizes []int, current int)`, `PageSize(sizes []int, current int)`,
  `Brand(spec BrandSpec)`, `Checkbox(attrs)`.
- `BrandSpec` is a plain struct (`Name, URL, LogoSVG, LogoDataURI, LogoAlt`).
- Root package keeps `statusNode`, `emptyStateNode`, `paginationNode`,
  `brandNode` as one line adapters, so no built in widget changes shape.
- Validation (`Brand.Validate`, `validateImageDataURI`, `validatePageSizes`)
  stays in the root package. `internal/chrome` renders, it does not validate.

Tests: existing golden files must be byte identical after this step. That is
the whole acceptance criterion. Run `make test-go` and confirm zero diffs in
`testdata/golden/`.

### L1.2 `compose` package core

New files:

- `compose/doc.go`: package comment, stability statement, the security
  contract authors inherit.
- `compose/document.go`: `DocConfig` and `Document`, delegating to
  `internal/htmlx`. Fields: `Title`, `Lang`, `CSS`, `ThemeCSS`, `ExtraCSS`,
  `Body`, `Config`, `Data`, `ExtraJS []string`, `RuntimeJS`.
- `compose/raw.go`: `RawCSS`, `RawJS`, `RawSVG`, `JSONIsland`, `Data`,
  re-exported from `internal/htmlx`.
- `compose/assets.go`: `StylesCSS`, `RuntimeJS` vars from `internal/assets`,
  plus `ConfigIslandID`, `DataIslandID` constants.
- `compose/chrome.go`: `Status()`, `Empty(gomukit.EmptyState)`,
  `Pagination(sizes []int, current int)`, `Brand(*gomukit.Brand)`,
  `Root(kind string, children ...g.Node)` which emits
  `<div class="gomu-root" data-gomu-widget="kind">`.

`internal/htmlx.DocConfig` gains `ExtraCSS string` and `ExtraJS []string`;
`Document` emits `ExtraCSS` after `ThemeCSS` and each `ExtraJS` entry after
`RuntimeJS`, every one through the existing raw guards.

### L1.3 Tests

- `compose/compose_test.go`: document contains both islands when configured
  and neither when not; `ExtraCSS`/`ExtraJS` land in the right order;
  guards refuse `</script`, `</style`, `<!--`; output contains no `http://`,
  `https://` or `src=` outside data URIs (self containment).
- `compose/example_test.go`: a `gauge` widget implementing `gomukit.Widget`
  end to end, doubling as the doc example.
- Golden: `testdata/golden/custom.html` from that gauge, wired like the other
  golden tests (`-update` flag, same failure message shape).
- `gosdk/gosdk_test.go`: register the gauge through `AddWidget` and assert the
  resource is served. This proves a non built in widget travels the whole path.

### L1.4 Docs

- `AGENTS.md`: new section "Package `compose`", documenting every exported
  symbol at the same depth as the existing package sections. Required by
  `.claude/rules/agents-md-sync.md`.
- `docs/architecture.md`: add `gomukit/compose` to the layer table, note that
  `internal/chrome` is shared by root and compose.
- `docs/extensibility.md`: replace the level 1 proposal with the shipped API.

Level 1 verification: `make test`, `make vet`, golden files reviewed.
No `ui/` changes, so no `make assets`.

## Level 2: custom runtime behaviors

Outcome: a custom component renders tool result data, calls tools and reports
status like a built in.

### L2.1 Runtime public surface

- New `ui/src/public.ts`, the only module the global exposes. It re-exports,
  explicitly and by name, never `export *` from an internal module:
  `registerBehavior`, type `MountContext`, type `Behavior`, `Bridge`,
  `BridgeError`, `M`, `HOST_CONTEXT_EVENT`, `h`, `delegate`, `clear`,
  `checkbox`, `readIsland`, `rowsFrom`, type `Row`, `formatCell`, `getLocale`,
  `RUNTIME_API_VERSION`.
- `ui/src/index.ts` adds `export * from "./public"` and keeps its side effects
  (auto boot) unchanged.
- `ui/build.mjs`: add `globalName: "gomukit"` to the JS build only.

Note on boot ordering, which needs no code change: the bundle and the author
script are both inline in `<body>`, so `document.readyState` is `"loading"`
while they execute, `boot()` defers to `DOMContentLoaded`, and a
`registerBehavior` call in the author script lands before the registry is
read. `ui/test/public.test.ts` must assert exactly this, because it is the
only thing keeping the mechanism working.

### L2.2 Runtime API version

- `ui/src/spec-constants.json` gains `"runtimeApiVersion": 1`.
- `ui/src/protocol.ts` exports `RUNTIME_API_VERSION` from it.
- Go mirrors it as `compose.RuntimeAPIVersion`, cross checked by the existing
  constants test in `uispec` (extend it rather than adding a new mechanism).
- Bump the integer on any breaking change to `MountContext` or the exported
  runtime helpers. Additive changes do not bump.

### L2.3 `gomukit.Custom`

New `custom.go` and `custom_render.go` in the root package:

```go
type Custom struct {
	URI, Name, Title, Description string
	Kind        string
	Body        g.Node
	CSS         string
	JS          string
	Config      any
	InitialData map[string]any
	Theme       *theme.Theme
	Brand       *Brand
	Empty       EmptyState
	UI          *uispec.ResourceUIMeta
}
```

- `Validate()`: URI present and `ui://`; `Kind` matches the pattern and is not
  reserved; `Body` non nil; `CSS`/`JS` pass the raw guards; `Brand.Validate()`.
- `Document()`: standard root, optional toolbar with brand and title, `Body`,
  empty state, status bar, `#gomu-config` (author `Config`),
  `#gomu-data` (`InitialData`), bundle, then author `JS`.
- `Descriptor()`/`ToolMeta()`: identical to `Menu`'s.
- `reservedKinds`: `table`, `form`, `card`, `cardlist`, `menu`, `confirm`,
  plus `page` reserved ahead of level 3.

### L2.4 Types for authors

- `make types`: `tsc --emitDeclarationOnly --declaration` over `public.ts`
  into `ui/types/gomukit.d.ts`, committed.
- Extend `make verify-dist` to also fail on `ui/types/` drift, so the shipped
  types cannot silently diverge from the bundle.

### L2.5 Tests

- `ui/test/public.test.ts`: registration before boot mounts the behavior;
  unknown kind is a silent no op; `MountContext` carries root, config,
  initialData, bridge and ready; the global is present on the built bundle.
- `custom_test.go`: validation table (bad kind, reserved kind, missing body,
  unsafe CSS/JS), document shape, golden `testdata/golden/custom.html`
  (supersedes the L1.3 golden, which becomes the compose level fixture).
- Harness: a `customGauge()` story in `examples/harness/stories.go` with a
  small inline behavior and a push payload, so the mechanism is exercised
  against the fake host and visible during review.

### L2.6 Docs

`AGENTS.md` gains "Custom widgets" covering `Custom`, the runtime global, the
`MountContext` contract and the author's innerHTML obligation. `CLAUDE.md`
notes that `globalName` makes the bundle's public exports load bearing.
`docs/extensibility.md` gains a worked end to end example.

Level 2 verification: `make assets` and commit `internal/assets/dist/`,
`make verify-dist`, `make typecheck`, `make test`.

## Level 3: several components in one document

Outcome: one `ui://` resource renders a page composed of multiple widgets.
Largest of the three and the only one that changes the shape of the existing
public API. Do not start it before levels 1 and 2 have real users.

### L3.1 Per root config islands

- `internal/htmlx`: `DocConfig.Config any` becomes
  `Configs []IslandConfig{ID string, Value any}`. One entry keeps emitting
  `#gomu-config` verbatim, so every existing golden file is unchanged;
  several entries emit `#gomu-config-<id>` and the matching root carries
  `data-gomu-config="<id>"`.
- `ui/src/data.ts`: `configFor(root)` resolves the attribute to an id and
  falls back to the default island.

### L3.2 Boot loop

`ui/src/index.ts`: `querySelectorAll("[data-gomu-widget]")`, one shared
`Bridge`, one shared `ready`, mount each root with its own config and the
shared bridge, `enhanceSelects` per root (already root scoped), and exactly
one `watchSize(bridge)` on `document.body`. Brand delegation moves from the
single root to `document`.

### L3.3 Tool result routing

The open problem, and the reason this level is expensive. Today each behavior
subscribes to `M.toolResult` and paints from `structuredContent`. With several
mounted behaviors every one of them would paint from every result.

Plan: default to broadcast, which is correct for several views of the same
payload, and add an optional `source` key in a component's config selecting a
nested object (`structuredContent.users` rather than the root). Components
without `source` keep today's behavior. Anything more (per component tool
correlation, request scoped results) needs a spec level answer about what a
host sends and is out of scope.

### L3.4 The `Component` split

Built in widgets today keep `shell()` and `config()` unexported, and `Widget`
means "renders a whole document". A page needs children that render a fragment
plus a config value:

```go
type Component interface {
	Kind() string
	Shell() g.Node
	Config() any
}
```

Every built in gains exported `Kind`/`Shell`/`Config` implementing it, their
`Document()` becomes a thin wrapper over `compose.Document`, and a new
`gomukit.Page` (`URI`, `Title`, `Theme`, `Brand`, `Components []Component`)
assembles the document, allocating island ids and validating uniqueness.

This is the step that grows the public API sharply: six widgets times three
methods, plus `Page`, plus `Component`. It also makes the internal shell
structure a compatibility promise. Budget the `AGENTS.md` update accordingly.

### L3.5 CSS and layout

`ui/css/base.css` scopes everything under `.gomu-root` and resets inside it,
so several roots coexist without change. What does need work: vertical rhythm
between stacked components, whether `Page` provides a grid or leaves layout to
author CSS (recommend: a single column stack plus documented tokens, no grid
API in the first cut), and `ui/src/popup.ts` anchoring dropdowns to the
nearest `.gomu-root`, which stays correct per component.

### L3.6 Tests

Go: `page_test.go` (island ids, uniqueness validation, kind collisions) plus
golden `testdata/golden/page.html`. TS: multi root boot mounts every behavior,
size is reported once, a scoped component ignores results outside its
`source`. Harness: a story with a table and a form in one document.

## Sequencing and risk

| Step | Touches | Risk | Notes |
|---|---|---|---|
| L1.1 chrome extraction | root, `internal/chrome` | low | Golden files must not move. Pure refactor. |
| L1.2 to L1.4 compose | new package, docs | low | Additive. gomponents becomes public API. |
| L2.1 to L2.2 runtime surface | `ui/`, build, dist | medium | Bundle global and boot ordering. Dist must be rebuilt and committed. |
| L2.3 `Custom` | root, goldens | low | Additive. |
| L2.4 types | Makefile, `ui/types` | low | New CI drift check. |
| L2.5 to L2.6 | tests, harness, docs | low | |
| L3.1 to L3.2 | `internal/htmlx`, `ui/src` | medium | Back compat rests on the single config case staying byte identical. |
| L3.3 routing | `ui/src/widgets/*` | high | Semantics are genuinely undecided; may need spec clarification. |
| L3.4 `Component` | every widget, `AGENTS.md` | high | Largest permanent public API growth in the plan. |
| L3.5 to L3.6 | CSS, tests, harness | medium | |

Rough shape: level 1 is a day, level 2 is two to three days including the
harness story and docs, level 3 is a week or more and should be re-planned
once levels 1 and 2 have been used in anger.

## Per phase verification checklist

1. `make vet && make test` green.
2. `make typecheck` green when `ui/` changed.
3. `make assets` run and `internal/assets/dist/` committed when `ui/src` or
   `ui/css` changed; `make verify-dist` clean.
4. Golden diffs reviewed by eye, never blanket regenerated.
5. `AGENTS.md` updated in the same change set for any exported surface,
   default, validation rule or data contract change.
6. Harness story added or updated for anything user visible, checked at
   `http://localhost:8090`.
