package gomukit

import (
	"fmt"

	"github.com/techthos/gomukit/theme"
	"github.com/techthos/gomukit/uispec"
)

// CardTemplate describes how one record renders as a card. It is shared by
// the single-record Card widget and the CardList collection widget, and is
// built from three sections that always render in the same order:
//
//	Header  — what the record is: title, description, and one slot at the
//	          end of the line for a status badge or a button.
//	Content — the record itself: a paragraph of prose and/or a label/value
//	          detail list (typed and Intl-formatted like table cells).
//	Footer  — what can be done with it: a note and a row of action buttons.
//
// Only Header is required; a section with nothing in it is not rendered at
// all, so a bare title-and-actions card carries no empty chrome.
type CardTemplate struct {
	// Header is the top section (required: it holds the title).
	Header CardHeader
	// Content is the card body.
	Content CardContent
	// Footer is the bottom section.
	Footer CardFooter
}

// CardHeader is the card's top section: the title, an optional description
// under it, and a single action slot at the end of the header line holding
// either a status badge or a button — not both, the way one slot works.
type CardHeader struct {
	// TitleKey is the row field shown as the card title (required).
	TitleKey string
	// DescriptionKey is a row field shown under the title.
	DescriptionKey string
	// Description is fixed text shown under the title, used instead of
	// DescriptionKey.
	Description string
	// Badge is a status badge for the header's action slot. Construct it with
	// the Badge column constructor; it is present when its Key is set, and
	// must be a badge column.
	Badge Column
	// Action is a button for the header's action slot, used instead of Badge.
	// FromSelection args are invalid here (they belong to CardList bulk
	// actions).
	Action *Action
}

func (h CardHeader) hasBadge() bool { return h.Badge.Key != "" }

func (h CardHeader) validate(ctx string) error {
	if h.TitleKey == "" {
		return fmt.Errorf("%s: TitleKey is required", ctx)
	}
	if err := validateTextSlot(ctx+": description", h.DescriptionKey, h.Description); err != nil {
		return err
	}
	if h.hasBadge() {
		if h.Badge.columnType() != ColBadge {
			return fmt.Errorf("%s: Badge must be a badge column", ctx)
		}
		if h.Action != nil {
			return fmt.Errorf("%s: Badge and Action share one header slot; set only one", ctx)
		}
	}
	if h.Action != nil {
		if err := validateRecordAction(ctx, *h.Action); err != nil {
			return err
		}
	}
	return nil
}

func (h CardHeader) config() map[string]any {
	m := map[string]any{"titleKey": h.TitleKey}
	if h.DescriptionKey != "" {
		m["descriptionKey"] = h.DescriptionKey
	}
	if h.Description != "" {
		m["description"] = h.Description
	}
	if h.hasBadge() {
		m["badge"] = columnConfig(h.Badge)
	}
	if h.Action != nil {
		m["action"] = h.Action.config()
	}
	return m
}

// CardContent is the card's body section: prose, a label/value detail list,
// or both. The list is the shared Descriptions block, so a card's fields
// render and format exactly like a confirmation's details.
type CardContent struct {
	// TextKey is a row field rendered as a paragraph of body text.
	TextKey string
	// Text is fixed body prose, used instead of TextKey.
	Text string
	// Items are the label/value detail rows.
	Items Descriptions
}

func (c CardContent) empty() bool {
	return c.TextKey == "" && c.Text == "" && c.Items.empty()
}

func (c CardContent) validate(ctx string) error {
	if err := validateTextSlot(ctx+": text", c.TextKey, c.Text); err != nil {
		return err
	}
	return c.Items.validate(ctx, true)
}

func (c CardContent) config() map[string]any {
	m := map[string]any{}
	if c.TextKey != "" {
		m["textKey"] = c.TextKey
	}
	if c.Text != "" {
		m["text"] = c.Text
	}
	if !c.Items.empty() {
		m["items"] = c.Items.config()
	}
	return m
}

// CardFooter is the card's bottom section: a note, and the buttons that act
// on the record.
type CardFooter struct {
	// TextKey is a row field shown as a footer note.
	TextKey string
	// Text is a fixed footer note, used instead of TextKey.
	Text string
	// Actions renders the footer buttons. FromSelection args are invalid here
	// (they belong to CardList bulk actions).
	Actions []Action
}

func (f CardFooter) empty() bool {
	return f.TextKey == "" && f.Text == "" && len(f.Actions) == 0
}

func (f CardFooter) validate(ctx string) error {
	if err := validateTextSlot(ctx+": text", f.TextKey, f.Text); err != nil {
		return err
	}
	for _, a := range f.Actions {
		if err := validateRecordAction(ctx, a); err != nil {
			return err
		}
	}
	return nil
}

func (f CardFooter) config() map[string]any {
	m := map[string]any{}
	if f.TextKey != "" {
		m["textKey"] = f.TextKey
	}
	if f.Text != "" {
		m["text"] = f.Text
	}
	if len(f.Actions) > 0 {
		m["actions"] = actionConfigs(f.Actions)
	}
	return m
}

// validateTextSlot rejects a text slot that is filled twice: a slot reads the
// record or states fixed words, never both.
func validateTextSlot(ctx, key, text string) error {
	if key != "" && text != "" {
		return fmt.Errorf("%s: Key and Text are mutually exclusive", ctx)
	}
	return nil
}

// validateRecordAction checks an action that fires on one record: valid in
// itself, and free of FromSelection args (those resolve across a selection,
// which a single record has no part in).
func validateRecordAction(ctx string, a Action) error {
	if err := a.validate(ctx); err != nil {
		return err
	}
	for name, src := range a.Args {
		if src.selection != "" {
			return fmt.Errorf("%s: action %q: argument %q: FromSelection is only valid in bulk actions", ctx, a.Label, name)
		}
	}
	return nil
}

// validate checks the template section by section.
func (t CardTemplate) validate(ctx string) error {
	if err := t.Header.validate(ctx + ": header"); err != nil {
		return err
	}
	if err := t.Content.validate(ctx + ": content"); err != nil {
		return err
	}
	if err := t.Footer.validate(ctx + ": footer"); err != nil {
		return err
	}
	return t.validateInputs(ctx)
}

// validateInputs checks the content's controls against the buttons that will
// carry what they collect: an action fires with its own arguments merged with
// the card's input values, so a shared name would send one where the other is
// expected.
func (t CardTemplate) validateInputs(ctx string) error {
	names := t.Content.Items.inputNames()
	if len(names) == 0 {
		return nil
	}
	actions := append([]Action{}, t.Footer.Actions...)
	if t.Header.Action != nil {
		actions = append(actions, *t.Header.Action)
	}
	for _, a := range actions {
		for _, name := range names {
			if _, clash := a.Args[name]; clash {
				return fmt.Errorf("%s: content: input %q is also an argument of action %q", ctx, name, a.Label)
			}
		}
	}
	return nil
}

// config serializes the template into the "card" object of the config island.
// Empty sections are left out, so the runtime renders only what was authored.
func (t CardTemplate) config() map[string]any {
	card := map[string]any{"header": t.Header.config()}
	if !t.Content.empty() {
		card["content"] = t.Content.config()
	}
	if !t.Footer.empty() {
		card["footer"] = t.Footer.config()
	}
	return card
}

// sortOptions derives the sortable content items for a CardList sort control:
// the ones reading a record field with an orderable type. Fixed-text items
// name no field to sort by.
func (t CardTemplate) sortOptions() []map[string]any {
	var opts []map[string]any
	for _, item := range t.Content.Items.Items {
		// An input item's Key is a prefill source, not a value on display;
		// sorting a list by what its controls hold would order it by nothing
		// the reader can see.
		if item.Input != nil || item.Key == "" || !item.column().sortable() {
			continue
		}
		label := item.Label
		if label == "" {
			label = item.Key
		}
		opts = append(opts, map[string]any{"key": item.Key, "label": label})
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
	// Theme overrides gomukit design tokens for this widget.
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
		return fmt.Errorf("gomukit: card: %w", err)
	}
	if err := c.Template.validate(fmt.Sprintf("gomukit: card %s", c.URI)); err != nil {
		return err
	}
	if err := c.Brand.Validate(); err != nil {
		return fmt.Errorf("gomukit: card %s: %w", c.URI, err)
	}
	if err := c.Theme.Validate(); err != nil {
		return fmt.Errorf("gomukit: card %s: %w", c.URI, err)
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
	// LoadMore turns the carousel into a growing strip instead of a paged one:
	// it starts at PageSize records and appends the next PageSize each time
	// the reader activates the "Load more" tile at the end of the strip, which
	// replaces the prev/next pagination bar. Records already scrolled past stay
	// where they are, so the strip reads as one continuous run.
	//
	// Requires PageSize > 0, and cannot be combined with PageSizes: the page
	// size chooser lives on the bar this removes.
	LoadMore bool
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
	// Theme overrides gomukit design tokens for this widget.
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
		return fmt.Errorf("gomukit: cardlist: %w", err)
	}
	if err := l.Template.validate(fmt.Sprintf("gomukit: cardlist %s", l.URI)); err != nil {
		return err
	}
	if l.PageSize < 0 {
		return fmt.Errorf("gomukit: cardlist %s: PageSize must be >= 0", l.URI)
	}
	if err := validatePageSizes(fmt.Sprintf("gomukit: cardlist %s", l.URI), l.PageSize, l.PageSizes); err != nil {
		return err
	}
	if l.LoadMore {
		if l.PageSize <= 0 {
			return fmt.Errorf("gomukit: cardlist %s: LoadMore needs PageSize > 0", l.URI)
		}
		if len(l.PageSizes) > 0 {
			return fmt.Errorf("gomukit: cardlist %s: LoadMore and PageSizes are mutually exclusive", l.URI)
		}
	}
	if l.DefaultSort != nil && l.DefaultSort.Key == "" {
		return fmt.Errorf("gomukit: cardlist %s: DefaultSort.Key is required", l.URI)
	}
	if l.Selection != nil {
		for _, a := range l.Selection.Bulk {
			if err := validateBulkAction(fmt.Sprintf("gomukit: cardlist %s: bulk", l.URI), a); err != nil {
				return err
			}
		}
	}
	if err := l.Brand.Validate(); err != nil {
		return fmt.Errorf("gomukit: cardlist %s: %w", l.URI, err)
	}
	if err := l.Theme.Validate(); err != nil {
		return fmt.Errorf("gomukit: cardlist %s: %w", l.URI, err)
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
	if l.LoadMore {
		cfg["loadMore"] = true
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
