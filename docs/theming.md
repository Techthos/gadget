# <img src="assets/gomukit-icon.svg" alt="" width="28" align="center"> Theming

gomukit uses a two-layer design-token system as CSS custom properties,
prefixed `--gomu-*`, scoped under `.gomu-root`.

## How a color is resolved

Every semantic token defaults to the **host-injected MCP Apps variable**
with a library fallback:

```css
--gomu-color-text: var(--color-text-primary, var(--gomu-p-gray-900));
```

MCP Apps hosts (Claude, ChatGPT, …) deliver `hostContext.styles.variables`
(`--color-background-primary`, `--font-sans`, `--border-radius-md`, semantic
danger/success colors, …) during `ui/initialize`; the runtime sets them as
inline style on the document root. Result: **widgets automatically match the
host's look** without configuration, and still render sensibly with no host.

## Dark mode

- The host's `theme: "light" | "dark"` sets `data-gomu-theme` on the root
  element; token overrides react to it.
- Without a host theme, `@media (prefers-color-scheme: dark)` provides the
  fallback.
- `:root { color-scheme }` starts at `light dark`, so host-provided
  `light-dark(...)` values self-adapt, and is then pinned to the host's theme
  once `hostContext` arrives. That pin is what keeps the iframe canvas
  transparent (see below), so do not override it to `normal`.
- Live `ui/notifications/host-context-changed` updates re-style the widget
  without reload.

## Embedding: hiding the iframe

A widget renders inside the host's iframe. By default it paints a page fill
plus an 8px gutter, so the frame reads as a panel of its own. `Transparent`
removes both, leaving only the widget's card on the host surface:

```go
table := &gomukit.Table{ /* ... */, Theme: &theme.Theme{Transparent: true}}
```

Card, tile, input and dropdown fills are untouched, so text contrast never
depends on what the host draws behind the frame. `ColorPage` and `PagePad` set
the same two knobs individually.

Three facts govern whether this actually looks embedded:

1. **The document canvas is transparent by default.** No attribute is needed;
   `allowtransparency` was an IE thing and is not in the spec. Transparency
   only breaks when something paints: the widget's page fill, or the host.
2. **The host must leave the `<iframe>` element unpainted.** The UA default
   border is `2px inset`, so the host needs `border: 0` and no `background`.
   gomukit cannot influence this from inside.
3. **The root color schemes must match.** Per css-color-adjust, when the
   embedded root element's used color scheme differs from the `<iframe>`
   element's, the UA replaces the transparent canvas with an opaque one in the
   embedded document's `Canvas` color. Chrome does this, Firefox does not, and
   **no author-level `background: transparent` can undo it** — the rule fires
   precisely because the canvas would otherwise be transparent. The runtime
   therefore pins `:root { color-scheme }` to `hostContext.theme` rather than
   leaving it at `light dark`, which would resolve from the OS preference and
   mismatch a host whose theme disagrees with it.

What transparency does **not** buy, because an iframe is still a nested
browsing context:

- Popovers cannot escape the frame. Dropdown panels, tooltips and focus rings
  are clipped by the iframe box; panels are anchored on `.gomu-root` and
  flipped to stay inside it.
- The frame rectangle keeps swallowing pointer events over transparent areas.
- Text selection cannot span host and widget, and `position: fixed` resolves
  against the iframe.
- Host webfonts do not load: the spec CSP has no `font-src`, so `--font-sans`
  resolves only against locally installed families.

`examples/harness` renders every story frameless by default and has a
**Frameless** toggle in the top bar to compare against the framed variant
(`/story/<id>?transparent=0`).

## Overriding tokens: the Theme struct

```go
import "github.com/techthos/gomukit/theme"

t := &theme.Theme{
    ColorPrimary: "#7c3aed",
    RadiusM:      "0.5rem",
    FontFamily:   `"Inter", sans-serif`,
    SpaceUnit:    "0.3rem", // roomier layout
    Extra: map[string]string{
        "--gomu-table-stripe": "rgb(0 0 0 / 4%)",
    },
}
table := &gomukit.Table{ /* ... */, Theme: t}
```

`Theme.CSS()` emits a `.gomu-root { … }` block (preceded by a `:root { … }`
block for the document-level page tokens) appended **after** the base
stylesheet: non-empty fields win over the defaults (including host values);
empty fields keep host-aware behavior. `Extra` keys must start with
`--gomu-`. Values are validated against CSS/HTML breakout
(`Theme.Validate()`).

## Token reference (semantic layer)

| Token | Purpose | Host variable consulted |
|---|---|---|
| `--gomu-color-bg` | cards, controls, overlays | `--color-background-primary` |
| `--gomu-color-page` | page fill behind the widget (defaults to `--gomu-color-bg`; `transparent` hides the frame) | — |
| `--gomu-color-surface` | headers, inputs, cards | `--color-background-secondary` |
| `--gomu-color-text` | primary text | `--color-text-primary` |
| `--gomu-color-text-muted` | secondary text | `--color-text-secondary` |
| `--gomu-color-border` | borders | `--color-border-primary` |
| `--gomu-color-primary` | accent / primary buttons | `--color-text-accent` |
| `--gomu-color-danger` | destructive actions, errors | `--color-text-danger` |
| `--gomu-color-success` | success states | `--color-text-success` |
| `--gomu-color-warning` | warnings | `--color-text-warning` |
| `--gomu-font` / `--gomu-font-mono` | typography | `--font-sans` / `--font-mono` |
| `--gomu-radius-s/m/l` | corner radii | `--border-radius-sm/md/lg` |
| `--gomu-space-unit` | base spacing unit (0.25rem) | — |
| `--gomu-page-pad` | gutter between widget and iframe edge (8px; set on `:root`) | — |
| `--gomu-card-width` | width of one card in the CardList carousel (17rem) | — |
