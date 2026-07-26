# Extensibility: custom components

Status: design proposal, nothing implemented. This describes what it would
take for a third party to ship a widget that is not built into gomukit, what
public API that requires, and what it costs in stability and security.

## Where we are today

`Widget` (`widget.go`) is a public interface, so anyone can already write a
type with `Document`, `Descriptor`, `ToolMeta` and `Validate` and register it
through `gosdk.AddWidget`. In practice that is useless, because everything
needed to produce a document is out of reach:

| Capability | Lives in | Reachable externally |
|---|---|---|
| Document assembly, JSON islands, CSP guards | `internal/htmlx` | no |
| Stylesheet and runtime bundle | `internal/assets` | no |
| Shared chrome (status, empty state, pagination, brand) | unexported funcs in root | no |
| Behavior registration | `registerBehavior` in `ui/src/index.ts`, bundled as a closed IIFE | no |
| Theme, spec constants and `_meta` types | `theme`, `uispec` | yes |

Three further facts shape the design:

1. The runtime bundle is built with `format: "iife"` and no `globalName`
   (`ui/build.mjs`), so nothing it exports is observable from another script
   in the same document.
2. `boot()` mounts exactly one behavior, found via a single
   `document.querySelector("[data-gomu-widget]")`, using one global
   `#gomu-config` island.
3. An unknown widget kind is not an error. `behaviors.get(kind)?.(...)` is a
   no-op, so a document with a custom kind still gets the `ui/initialize`
   handshake, host context (theme, fonts, locale, style variables), `<select>`
   upgrades, brand link handling and size reporting. Only data rendering and
   interaction are missing.

That last point is what makes a staged rollout possible.

## Level 1: custom Go widget, no custom JavaScript

Goal: a third party renders arbitrary server side markup that looks and
behaves like a gomukit widget (tokens, theme, host context, sizing), with no
runtime behavior of its own. Suitable for read only or link only components.

### Proposed surface

A new public package, `gomukit/compose`, mirroring `internal/htmlx` plus the
embedded assets:

```go
package compose

// DocConfig describes one widget document.
type DocConfig struct {
	Title, Lang       string
	CSS, ThemeCSS     string
	ExtraCSS          string // author stylesheet, emitted after ThemeCSS
	Body              g.Node // rooted at .gomu-root with data-gomu-widget
	Config, Data      any    // #gomu-config, #gomu-data
	RuntimeJS         string
}

func Document(c DocConfig) (string, error)
func JSONIsland(id string, v any) (g.Node, error)
func RawCSS(css string) (g.Node, error)
func RawJS(js string) (g.Node, error)
func RawSVG(svg string) (g.Node, error)
func Data(name, value string) g.Node // data-gomu-<name>

var StylesCSS string // internal/assets.StylesCSS
var RuntimeJS string // internal/assets.RuntimeJS
```

Optionally, the shared chrome currently unexported in `render_shared.go` and
`brand_render.go`, so custom widgets do not reimplement it:
`compose.Status()`, `compose.Empty(gomukit.EmptyState)`,
`compose.Pagination(sizes []int, current int)`, `compose.Brand(gomukit.Brand)`.

`internal/htmlx` stays where it is and `compose` delegates to it, so the
public package is a curated surface rather than a rename.

### Consumer shape

```go
type Gauge struct {
	URI, Title string
	Theme      theme.Theme
	Max        float64
}

func (g *Gauge) Document() (string, error) {
	return compose.Document(compose.DocConfig{
		Title:     g.Title,
		CSS:       compose.StylesCSS,
		ThemeCSS:  g.Theme.CSS(),
		ExtraCSS:  gaugeCSS,
		Body:      g.shell(),           // .gomu-root + data-gomu-widget="gauge"
		Config:    map[string]any{"max": g.Max},
		RuntimeJS: compose.RuntimeJS,
	})
}
```

### Cost

gomponents (`maragu.dev/gomponents`) becomes part of gomukit's public API,
where today it is an implementation detail. The alternative is a string based
`Body` guarded like `RawSVG`, which avoids the dependency but gives up the
by-construction escaping that the security invariants rest on. Recommendation:
accept the gomponents dependency; a raw HTML string body would make "data
reaches HTML only through text nodes" unenforceable.

## Level 2: custom runtime behavior

Goal: the custom component renders data from tool results, calls tools, and
reports status, exactly like `table` does.

### Two mechanisms

**A. Expose the runtime.** Build with `globalName: "gomukit"` and give the
bundle a curated entry surface (a new `ui/src/public.ts` re-exported from
`index.ts`) rather than exporting whatever happens to be internal:

| Export | Purpose |
|---|---|
| `registerBehavior(kind, fn)` | register the behavior for a custom kind |
| `MountContext` | `{ root, config, initialData, bridge, ready }` |
| `Bridge` methods: `request`, `notify`, `on`, `callTool`, `openLink`, `hasHost` | host protocol |
| `M` (method names), `HOST_CONTEXT_EVENT` | protocol constants and the re-render signal |
| `h`, `delegate`, `checkbox` | DOM helpers that keep the no-innerHTML invariant |
| `formatCell`, `getLocale` | locale aware formatting fed by host context |
| `version` | runtime API version, for compatibility checks |

**B. Emit the author's script.** Add `ExtraJS string` to `DocConfig`, emitted
through `RawJS` as a second inline `<script>` immediately after the bundle.

Timing works without changes to `boot()`: both scripts are inline and in
`<body>`, so `document.readyState` is `"loading"` while they run, `boot()`
defers to `DOMContentLoaded`, and the author's `registerBehavior` call lands
before the registry is read.

### Consumer shape

```ts
gomukit.registerBehavior("gauge", ({ root, config, initialData, bridge, ready }) => {
  const host = root.querySelector("[data-gomu-gauge]")!;
  const paint = (data) => { host.textContent = String(data?.value ?? ""); };
  if (initialData) paint(initialData);
  bridge.on(gomukit.M.toolResult, (p) => paint(p.structuredContent));
});
```

The author bundles that to a single string themselves (esbuild, tsc, or hand
written ES2020) and hands it to Go as `ExtraJS`. gomukit embeds a string and
does not become a build tool. Shipping a `.d.ts` alongside the published
package would make this pleasant; see open questions.

### A convenience wrapper

Most authors will not want to implement `Widget` from scratch. A root level
`gomukit.Custom` covers the common case:

```go
type Custom struct {
	URI, Name, Title, Description string
	Kind        string   // data-gomu-widget value, and the registerBehavior key
	Body        g.Node   // markup inside .gomu-root
	CSS, JS     string   // author stylesheet and behavior script
	Config      any      // #gomu-config payload
	InitialData map[string]any
	Theme       theme.Theme
	Brand       Brand
	Empty       EmptyState
	UI          *uispec.ResourceUIMeta
}
```

`Validate()` enforces a non empty `Kind` matching `[a-z][a-z0-9-]*`, a `Kind`
that does not collide with a built in, and the existing raw content guards.
`Document()` wraps `Body` in the standard root, toolbar, empty state and
status chrome.

## Level 3: several components in one document

Independent of levels 1 and 2 and considerably larger. Requires per root
config islands (`data-gomu-config="<id>"` resolved against
`#gomu-config-<id>`), `boot()` switching to `querySelectorAll` with a mount
loop, one shared `Bridge` fanning tool results out to every mounted behavior,
and a single size observer on the document rather than per widget. Defer until
levels 1 and 2 are in use.

## Security

The existing invariants hold or move, none are silently lost:

- `RawCSS`/`RawJS`/`RawSVG` guards still apply to author supplied content, so
  `</script`, `</style` and `<!--` remain refused.
- JSON islands still go through `encoding/json` HTML safe escaping.
- Documents stay self contained. Author CSS and JS are inline strings; there
  is no mechanism to reference an external URL, and the host CSP would block
  it anyway. Custom code that needs network access must declare
  `uispec.CSP.ConnectDomains` on the resource descriptor.
- "Data reaches the DOM only via text nodes" changes from a by-construction
  property to a documented contract for author JavaScript. `h` and
  `textContent` are provided precisely so the safe path is the easy one. This
  is the single real reduction in guarantees and should be stated plainly in
  the public docs.
- `Theme` values remain validated; nothing about theming changes.

## Stability

Levels 1 and 2 promote three things to public contract that are currently
free to change: the document structure (island IDs, `gomu-root`,
`data-gomu-widget`), the CSS class and token names, and the runtime
`MountContext`/`Bridge` shapes. Mitigations:

- Version the runtime surface explicitly (`gomukit.RuntimeAPIVersion` in Go,
  `gomukit.version` in JS) and have `Custom.Validate()` or a boot time check
  warn on mismatch.
- Treat `docs/extensibility.md` and `AGENTS.md` as the definition of what is
  supported; class names not listed there stay internal.
- Per `.claude/rules/agents-md-sync.md`, every item above lands in `AGENTS.md`
  in the same change set.

## Phasing and open questions

`docs/extensibility-plan.md` carries the step by step implementation plan for
all three levels, with the files each step touches, its tests, and a risk
table. It also records the decisions taken on the questions this document
originally left open: `gomukit/compose` as the package, committed
`ui/types/gomukit.d.ts` rather than an npm publish, `g.Node` (not raw HTML
strings) for custom bodies, a reserved kind list rather than forced
namespacing, and `ExtraJS` as an ordered slice.

The one question still genuinely open is level 3 tool result routing: with
several components mounted, which of them a `ui/notifications/tool-result`
should paint. The plan proposes broadcast by default with an optional
`source` key, and flags that anything stronger may need a spec level answer.
