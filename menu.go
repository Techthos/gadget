package gadget

import (
	"fmt"

	"github.com/techthos/gadget/internal/htmlx"
	"github.com/techthos/gadget/theme"
	"github.com/techthos/gadget/uispec"
)

// MenuItem is one entry in a Menu: a tool the user can start from the menu
// tile grid. Choosing it calls Tool with Args; the host answers by opening
// that tool's own widget, so a menu item is navigation rather than an action
// with a result of its own.
type MenuItem struct {
	// Tool is the MCP tool called when the item is chosen (required).
	Tool string
	// Args are static arguments passed to Tool. Menu items carry no record,
	// so unlike Action args these are fixed values, not row lookups.
	Args map[string]any
	// Label is the tile heading. Defaults to Tool.
	Label string
	// Description is the supporting line under the label.
	Description string
	// IconSVG is inline <svg> markup shown above the label. Documents stay
	// self-contained, so an icon is never a URL. It is author-trusted input,
	// checked only against script-bearing and resource-loading constructs.
	IconSVG string
	// Badge is optional short text shown in the tile's top right — a category
	// or state marker such as "read" or "beta".
	Badge string
	// BadgeVariant colors the badge. Defaults to BadgeNeutral.
	BadgeVariant BadgeVariant
}

func (i MenuItem) label() string {
	if i.Label != "" {
		return i.Label
	}
	return i.Tool
}

// badgeVariants are the values BadgeVariant may take. A menu badge's variant
// becomes a class name in server-rendered markup, so it is checked up front
// rather than silently producing a class no stylesheet rule matches.
var badgeVariants = map[BadgeVariant]bool{
	BadgeNeutral: true,
	BadgeInfo:    true,
	BadgeSuccess: true,
	BadgeWarning: true,
	BadgeDanger:  true,
}

func (i MenuItem) validate(ctx string) error {
	if i.Tool == "" {
		return fmt.Errorf("%s: Tool is required", ctx)
	}
	if i.IconSVG != "" {
		if _, err := htmlx.RawSVG(i.IconSVG); err != nil {
			return fmt.Errorf("%s: IconSVG: %w", ctx, err)
		}
	}
	if i.BadgeVariant != "" && !badgeVariants[i.BadgeVariant] {
		return fmt.Errorf("%s: unknown badge variant %q", ctx, i.BadgeVariant)
	}
	return nil
}

// Menu renders a launcher: a grid of tiles, one per MCP tool the application
// exposes with a UI. Choosing a tile calls that tool, and the host opens the
// widget bound to it — so a Menu is the entry point an app hands the user
// before any particular record is in view.
//
// Unlike the data widgets, a Menu is fully authored at registration time: the
// tiles are server-rendered from Items and the document needs nothing from
// the runtime data contract.
type Menu struct {
	// URI is the widget's ui:// resource URI (required).
	URI string
	// Title is shown in the toolbar and the document title.
	Title string
	// Intro is optional lead text shown above the tiles.
	Intro string
	// Items are the menu entries (at least one is required).
	Items []MenuItem

	// Brand renders the application logo/name on the widget.
	Brand *Brand
	// Theme overrides gadget design tokens for this widget.
	Theme *theme.Theme
	// UI overrides resource _meta.ui (CSP, permissions, prefersBorder).
	UI *uispec.ResourceUIMeta
}

// Validate implements Widget.
func (m *Menu) Validate() error {
	if err := uispec.ValidateURI(m.URI); err != nil {
		return fmt.Errorf("gadget: menu: %w", err)
	}
	if len(m.Items) == 0 {
		return fmt.Errorf("gadget: menu %s: at least one item is required", m.URI)
	}
	for i, item := range m.Items {
		ctx := fmt.Sprintf("gadget: menu %s: item %d (%s)", m.URI, i, item.label())
		if err := item.validate(ctx); err != nil {
			return err
		}
	}
	if err := m.Brand.Validate(); err != nil {
		return fmt.Errorf("gadget: menu %s: %w", m.URI, err)
	}
	if err := m.Theme.Validate(); err != nil {
		return fmt.Errorf("gadget: menu %s: %w", m.URI, err)
	}
	return nil
}

// Descriptor implements Widget.
func (m *Menu) Descriptor() uispec.ResourceDescriptor {
	return uispec.ResourceDescriptor{
		URI:      m.URI,
		Name:     resourceName(m.URI),
		Title:    m.Title,
		MIMEType: uispec.MIMEType,
		UI:       m.UI,
	}
}

// ToolMeta implements Widget.
func (m *Menu) ToolMeta() map[string]any {
	return uispec.ToolUIMeta{ResourceURI: m.URI}.MetaMap()
}

// config serializes only what the runtime needs: the tool call behind each
// tile, positionally matched to the server-rendered buttons. Labels, icons
// and descriptions are already in the markup.
func (m *Menu) config() map[string]any {
	items := make([]map[string]any, len(m.Items))
	for i, item := range m.Items {
		entry := map[string]any{"tool": item.Tool}
		if len(item.Args) > 0 {
			entry["args"] = item.Args
		}
		items[i] = entry
	}
	return map[string]any{
		"widget": "menu",
		"items":  items,
	}
}
