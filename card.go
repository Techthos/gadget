package gadget

import (
	"fmt"

	"github.com/techthos/gadget/theme"
	"github.com/techthos/gadget/uispec"
)

// CardTemplate describes how one record renders as a card. It is shared by
// the single-record Card widget and the CardList collection widget: a title
// and optional subtitle pulled from row fields, an optional status badge, a
// list of label/value body fields (typed and Intl-formatted like table
// cells), and a footer row of actions.
type CardTemplate struct {
	// TitleKey is the row field shown as the card title (required).
	TitleKey string
	// SubtitleKey is an optional row field shown under the title.
	SubtitleKey string
	// Badge is an optional status badge shown in the card header. Construct
	// it with the Badge column constructor; it is present when its Key is
	// set, and must be a badge column.
	Badge Column
	// Fields are the label/value body rows (text/number/date/badge/link
	// columns — not actions).
	Fields []Column
	// Actions renders footer buttons. FromSelection args are invalid here
	// (they belong to CardList bulk actions).
	Actions []Action
}

func (t CardTemplate) hasBadge() bool { return t.Badge.Key != "" }

// validate checks the template. bulk reports whether the enclosing widget is
// a collection (only then may per-record actions coexist with selection);
// FromSelection remains invalid on the template's own Actions in both cases.
func (t CardTemplate) validate(ctx string) error {
	if t.TitleKey == "" {
		return fmt.Errorf("%s: TitleKey is required", ctx)
	}
	if t.hasBadge() && t.Badge.columnType() != ColBadge {
		return fmt.Errorf("%s: Badge must be a badge column", ctx)
	}
	seen := map[string]bool{}
	for i, f := range t.Fields {
		fctx := fmt.Sprintf("%s: field %d (%s)", ctx, i, f.Label)
		switch f.columnType() {
		case ColText, ColNumber, ColDate, ColBadge:
			if f.Key == "" {
				return fmt.Errorf("%s: key is required", fctx)
			}
		case ColLink:
			if f.Link == nil || f.Link.HrefKey == "" {
				return fmt.Errorf("%s: link fields need Link.HrefKey", fctx)
			}
		case ColActions:
			return fmt.Errorf("%s: actions belong in CardTemplate.Actions, not Fields", fctx)
		default:
			return fmt.Errorf("%s: unknown field type %q", fctx, f.Type)
		}
		if f.Key != "" {
			if seen[f.Key] {
				return fmt.Errorf("%s: duplicate field key %q", fctx, f.Key)
			}
			seen[f.Key] = true
		}
	}
	for _, a := range t.Actions {
		if err := a.validate(ctx); err != nil {
			return err
		}
		for name, src := range a.Args {
			if src.selection != "" {
				return fmt.Errorf("%s: action %q: argument %q: FromSelection is only valid in bulk actions", ctx, a.Label, name)
			}
		}
	}
	return nil
}

// config serializes the template into the "card" object of the config island.
func (t CardTemplate) config() map[string]any {
	fields := make([]map[string]any, len(t.Fields))
	for i, f := range t.Fields {
		fields[i] = columnConfig(f)
	}
	card := map[string]any{
		"titleKey": t.TitleKey,
		"fields":   fields,
	}
	if t.SubtitleKey != "" {
		card["subtitleKey"] = t.SubtitleKey
	}
	if t.hasBadge() {
		card["badge"] = columnConfig(t.Badge)
	}
	if len(t.Actions) > 0 {
		card["actions"] = actionConfigs(t.Actions)
	}
	return card
}

// sortOptions derives the sortable body fields for a CardList sort control.
func (t CardTemplate) sortOptions() []map[string]any {
	var opts []map[string]any
	for _, f := range t.Fields {
		if f.sortable() && f.Key != "" {
			label := f.Label
			if label == "" {
				label = f.Key
			}
			opts = append(opts, map[string]any{"key": f.Key, "label": label})
		}
	}
	return opts
}

// --- Card (single record) ---

// Card renders a single record as a card. The record is the first element of
// the rows array delivered at runtime under RowsKey (the same contract as
// Table and CardList), or baked into the document via InitialData.
type Card struct {
	// URI is the widget's ui:// resource URI (required).
	URI string
	// Title is shown in the toolbar and the document title.
	Title string
	// Template describes the card content (required).
	Template CardTemplate

	// RowsKey is the structuredContent key holding the rows array; the card
	// renders rows[0]. Defaults to "rows".
	RowsKey string
	// RowID is the record field used for FromRow action args. Defaults to
	// "id".
	RowID string
	// Empty configures the message shown when no record is present.
	Empty EmptyState

	// InitialData is an optional structuredContent-shaped snapshot baked into
	// the document as a JSON island.
	InitialData map[string]any

	// LoadTool, when set, names a read tool the runtime calls once on load to
	// hydrate the card from fresh data, replacing InitialData. It must return
	// the record under RowsKey.
	LoadTool string
	// LoadArgs are optional static arguments passed to LoadTool.
	LoadArgs map[string]any

	// Brand renders the application logo/name on the widget.
	Brand *Brand
	// Theme overrides gadget design tokens for this widget.
	Theme *theme.Theme
	// UI overrides resource _meta.ui (CSP, permissions, prefersBorder).
	UI *uispec.ResourceUIMeta
}

func (c *Card) rowsKey() string {
	if c.RowsKey == "" {
		return "rows"
	}
	return c.RowsKey
}

func (c *Card) rowID() string {
	if c.RowID == "" {
		return "id"
	}
	return c.RowID
}

// Validate implements Widget.
func (c *Card) Validate() error {
	if err := uispec.ValidateURI(c.URI); err != nil {
		return fmt.Errorf("gadget: card: %w", err)
	}
	if err := c.Template.validate(fmt.Sprintf("gadget: card %s", c.URI)); err != nil {
		return err
	}
	if err := c.Brand.Validate(); err != nil {
		return fmt.Errorf("gadget: card %s: %w", c.URI, err)
	}
	if err := c.Theme.Validate(); err != nil {
		return fmt.Errorf("gadget: card %s: %w", c.URI, err)
	}
	return nil
}

// Descriptor implements Widget.
func (c *Card) Descriptor() uispec.ResourceDescriptor {
	return uispec.ResourceDescriptor{
		URI:      c.URI,
		Name:     resourceName(c.URI),
		Title:    c.Title,
		MIMEType: uispec.MIMEType,
		UI:       c.UI,
	}
}

// ToolMeta implements Widget.
func (c *Card) ToolMeta() map[string]any {
	return uispec.ToolUIMeta{ResourceURI: c.URI}.MetaMap()
}

func (c *Card) config() map[string]any {
	cfg := map[string]any{
		"widget":  "card",
		"rowsKey": c.rowsKey(),
		"rowId":   c.rowID(),
		"card":    c.Template.config(),
	}
	if c.Empty != (EmptyState{}) {
		cfg["empty"] = c.Empty
	}
	if c.LoadTool != "" {
		cfg["loadTool"] = c.LoadTool
		if len(c.LoadArgs) > 0 {
			cfg["loadArgs"] = c.LoadArgs
		}
	}
	return cfg
}

// --- CardList (collection) ---

// CardList renders a collection of records as cards in a responsive grid,
// with client-side filter, sort, pagination, selection with bulk actions,
// and per-card actions — the same runtime machinery as Table, laid out as
// cards instead of table rows.
type CardList struct {
	// URI is the widget's ui:// resource URI (required).
	URI string
	// Title is shown in the toolbar and the document title.
	Title string
	// Template describes how each record renders as a card (required).
	Template CardTemplate

	// RowsKey is the structuredContent key holding the rows array. Defaults
	// to "rows".
	RowsKey string
	// RowID uniquely identifies a record, used for selection and
	// FromRow/FromSelection args. Defaults to "id".
	RowID string

	// PageSize enables client-side pagination when > 0.
	PageSize int
	// PageSizes offers alternative page sizes in a dropdown on the pagination
	// bar. Entries must be > 0 and PageSize must be set; PageSize is added to
	// the list if it is not among them. Empty renders no chooser.
	PageSizes []int
	// DefaultSort pre-sorts records on load.
	DefaultSort *SortSpec
	// Filterable adds a client-side text filter box.
	Filterable bool
	// Selection enables per-card checkboxes and bulk actions.
	Selection *SelectionConfig
	// Empty configures the no-data message.
	Empty EmptyState

	// InitialData is an optional structuredContent-shaped snapshot baked into
	// the document as a JSON island.
	InitialData map[string]any

	// LoadTool, when set, names a read tool the runtime calls once on load to
	// hydrate the list from fresh data, replacing InitialData. It must return
	// the records under RowsKey.
	LoadTool string
	// LoadArgs are optional static arguments passed to LoadTool.
	LoadArgs map[string]any

	// Brand renders the application logo/name on the widget.
	Brand *Brand
	// Theme overrides gadget design tokens for this widget.
	Theme *theme.Theme
	// UI overrides resource _meta.ui (CSP, permissions, prefersBorder).
	UI *uispec.ResourceUIMeta
}

func (l *CardList) rowsKey() string {
	if l.RowsKey == "" {
		return "rows"
	}
	return l.RowsKey
}

func (l *CardList) rowID() string {
	if l.RowID == "" {
		return "id"
	}
	return l.RowID
}

// Validate implements Widget.
func (l *CardList) Validate() error {
	if err := uispec.ValidateURI(l.URI); err != nil {
		return fmt.Errorf("gadget: cardlist: %w", err)
	}
	if err := l.Template.validate(fmt.Sprintf("gadget: cardlist %s", l.URI)); err != nil {
		return err
	}
	if l.PageSize < 0 {
		return fmt.Errorf("gadget: cardlist %s: PageSize must be >= 0", l.URI)
	}
	if err := validatePageSizes(fmt.Sprintf("gadget: cardlist %s", l.URI), l.PageSize, l.PageSizes); err != nil {
		return err
	}
	if l.DefaultSort != nil && l.DefaultSort.Key == "" {
		return fmt.Errorf("gadget: cardlist %s: DefaultSort.Key is required", l.URI)
	}
	if l.Selection != nil {
		for _, a := range l.Selection.Bulk {
			if err := a.validate(fmt.Sprintf("gadget: cardlist %s: bulk", l.URI)); err != nil {
				return err
			}
		}
	}
	if err := l.Brand.Validate(); err != nil {
		return fmt.Errorf("gadget: cardlist %s: %w", l.URI, err)
	}
	if err := l.Theme.Validate(); err != nil {
		return fmt.Errorf("gadget: cardlist %s: %w", l.URI, err)
	}
	return nil
}

// Descriptor implements Widget.
func (l *CardList) Descriptor() uispec.ResourceDescriptor {
	return uispec.ResourceDescriptor{
		URI:      l.URI,
		Name:     resourceName(l.URI),
		Title:    l.Title,
		MIMEType: uispec.MIMEType,
		UI:       l.UI,
	}
}

// ToolMeta implements Widget.
func (l *CardList) ToolMeta() map[string]any {
	return uispec.ToolUIMeta{ResourceURI: l.URI}.MetaMap()
}

func (l *CardList) config() map[string]any {
	cfg := map[string]any{
		"widget":     "cardlist",
		"rowsKey":    l.rowsKey(),
		"rowId":      l.rowID(),
		"pageSize":   l.PageSize,
		"filterable": l.Filterable,
		"card":       l.Template.config(),
	}
	if opts := l.Template.sortOptions(); len(opts) > 0 {
		cfg["sort"] = opts
	}
	if l.DefaultSort != nil {
		cfg["defaultSort"] = l.DefaultSort
	}
	if l.Selection != nil {
		cfg["selection"] = map[string]any{"bulk": actionConfigs(l.Selection.Bulk)}
	}
	if l.Empty != (EmptyState{}) {
		cfg["empty"] = l.Empty
	}
	if l.LoadTool != "" {
		cfg["loadTool"] = l.LoadTool
		if len(l.LoadArgs) > 0 {
			cfg["loadArgs"] = l.LoadArgs
		}
	}
	return cfg
}
