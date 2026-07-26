package gadget

import "fmt"

// Descriptions is a label/value detail list — the block a widget uses to spell
// out what a record is before the reader acts on it. It is a shared building
// block, not a widget: it has no URI and no Document, and is embedded by value
// the way CardTemplate is.
//
// There are no layout controls. The items lay themselves out in as many
// columns as the widget's own width allows and collapse to one in a narrow
// chat pane, so the same document reads in either.
type Descriptions struct {
	// Items are the entries, in reading order (at least one to render).
	Items []DescriptionItem
}

func (d Descriptions) empty() bool { return len(d.Items) == 0 }

// DescriptionItem is one label/value pair. The value is either read from the
// record at runtime (Key) or authored here as fixed text (Text) — exactly one
// of the two. A Key value is typed and Intl-formatted by the runtime, using
// the same types, formats and host locale as a table cell.
type DescriptionItem struct {
	// Label names the value (required).
	Label string
	// Key is the record field holding the value.
	Key string
	// Text is a fixed value, used instead of Key.
	Text string
	// Type selects rendering: ColText (default), ColNumber, ColDate, ColBadge
	// or ColLink. Fixed-text items are always text.
	Type ColumnType
	// Format refines number and date rendering, exactly as Column.Format.
	Format string
	// Badge maps values to badge variants (ColBadge).
	Badge map[string]BadgeVariant
	// Link configures a ColLink item; its URL comes from the record.
	Link *LinkSpec
	// Align positions the value within its cell.
	Align Align
}

func (i DescriptionItem) itemType() ColumnType {
	if i.Type == "" {
		return ColText
	}
	return i.Type
}

// column expresses the item as a Column so it serializes through the same
// path as a table column and formats identically at runtime.
func (i DescriptionItem) column() Column {
	return Column{
		Key:    i.Key,
		Label:  i.Label,
		Type:   i.Type,
		Align:  i.Align,
		Format: i.Format,
		Badge:  i.Badge,
		Link:   i.Link,
	}
}

func (i DescriptionItem) config() map[string]any {
	m := columnConfig(i.column())
	// A description item is never a sort key; columnConfig emits the flag for
	// tables and card lists, which is noise in this payload.
	delete(m, "sortable")
	if i.Text != "" {
		m["text"] = i.Text
	}
	return m
}

func (i DescriptionItem) validate(ctx string) error {
	if i.Label == "" {
		return fmt.Errorf("%s: Label is required", ctx)
	}
	if i.Text != "" {
		// An authored value is rendered as written: there is no record field
		// to type, format or map to a badge variant.
		if i.Key != "" {
			return fmt.Errorf("%s: Key and Text are mutually exclusive", ctx)
		}
		if i.itemType() != ColText {
			return fmt.Errorf("%s: a Text value cannot be typed %q", ctx, i.itemType())
		}
		return nil
	}
	switch i.itemType() {
	case ColText, ColNumber, ColDate, ColBadge:
		if i.Key == "" {
			return fmt.Errorf("%s: needs a Key or a Text value", ctx)
		}
	case ColLink:
		if i.Link == nil || i.Link.HrefKey == "" {
			return fmt.Errorf("%s: link items need Link.HrefKey", ctx)
		}
	default:
		return fmt.Errorf("%s: unknown item type %q", ctx, i.Type)
	}
	return nil
}

// validate checks every item and rejects two items reading the same field.
func (d Descriptions) validate(ctx string) error {
	seen := map[string]bool{}
	for n, item := range d.Items {
		ictx := fmt.Sprintf("%s: item %d (%s)", ctx, n, item.Label)
		if err := item.validate(ictx); err != nil {
			return err
		}
		if item.Key != "" {
			if seen[item.Key] {
				return fmt.Errorf("%s: duplicate item key %q", ictx, item.Key)
			}
			seen[item.Key] = true
		}
	}
	return nil
}

// config serializes the items for the config island. The runtime renders the
// list, so labels travel with it rather than being server-rendered.
func (d Descriptions) config() []map[string]any {
	items := make([]map[string]any, len(d.Items))
	for n, item := range d.Items {
		items[n] = item.config()
	}
	return items
}
