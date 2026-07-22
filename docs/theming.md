# Theming

gadget uses a two-layer design-token system as CSS custom properties,
prefixed `--gadget-*`, scoped under `.gadget-root`.

## How a color is resolved

Every semantic token defaults to the **host-injected MCP Apps variable**
with a library fallback:

```css
--gadget-color-text: var(--color-text-primary, var(--gadget-p-gray-900));
```

MCP Apps hosts (Claude, ChatGPT, …) deliver `hostContext.styles.variables`
(`--color-background-primary`, `--font-sans`, `--border-radius-md`, semantic
danger/success colors, …) during `ui/initialize`; the runtime sets them as
inline style on the document root. Result: **widgets automatically match the
host's look** without configuration, and still render sensibly with no host.

## Dark mode

- The host's `theme: "light" | "dark"` sets `data-gadget-theme` on the root
  element; token overrides react to it.
- Without a host theme, `@media (prefers-color-scheme: dark)` provides the
  fallback.
- `color-scheme: light dark` lets host-provided `light-dark(...)` values
  self-adapt.
- Live `ui/notifications/host-context-changed` updates re-style the widget
  without reload.

## Overriding tokens: the Theme struct

```go
import "github.com/techthos/gadget/theme"

t := &theme.Theme{
    ColorPrimary: "#7c3aed",
    RadiusM:      "0.5rem",
    FontFamily:   `"Inter", sans-serif`,
    SpaceUnit:    "0.3rem", // roomier layout
    Extra: map[string]string{
        "--gadget-table-stripe": "rgb(0 0 0 / 4%)",
    },
}
table := &gadget.Table{ /* ... */, Theme: t}
```

`Theme.CSS()` emits a `.gadget-root { … }` block appended **after** the base
stylesheet: non-empty fields win over the defaults (including host values);
empty fields keep host-aware behavior. `Extra` keys must start with
`--gadget-`. Values are validated against CSS/HTML breakout
(`Theme.Validate()`).

## Token reference (semantic layer)

| Token | Purpose | Host variable consulted |
|---|---|---|
| `--gadget-color-bg` | widget background | `--color-background-primary` |
| `--gadget-color-surface` | headers, inputs, cards | `--color-background-secondary` |
| `--gadget-color-text` | primary text | `--color-text-primary` |
| `--gadget-color-text-muted` | secondary text | `--color-text-secondary` |
| `--gadget-color-border` | borders | `--color-border-primary` |
| `--gadget-color-primary` | accent / primary buttons | `--color-text-accent` |
| `--gadget-color-danger` | destructive actions, errors | `--color-text-danger` |
| `--gadget-color-success` | success states | `--color-text-success` |
| `--gadget-color-warning` | warnings | `--color-text-warning` |
| `--gadget-font` / `--gadget-font-mono` | typography | `--font-sans` / `--font-mono` |
| `--gadget-radius-s/m/l` | corner radii | `--border-radius-sm/md/lg` |
| `--gadget-space-unit` | base spacing unit (0.25rem) | — |
