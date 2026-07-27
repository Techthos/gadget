package gomukit

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
//
// An item can also ask rather than state: set Input and the value cell holds a
// control the reader fills in, whose value travels with the widget's own call.
// See Input.
type DescriptionItem struct {
	// Label names the value (required).
	Label string
	// Key is the record field holding the value. On an Input item it is the
	// prefill source instead: the control opens on that field's value.
	Key string
	// Text is a fixed value, used instead of Key.
	Text string
	// Input makes this item a question: the value cell renders a control, and
	// what the reader puts in it travels with the widget's call. Only the
	// widgets that own a call accept one — DatePicker.Details and a card's
	// Content.Items; a Confirm's or a Choice's details are read-only.
	Input *Input
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

// InputType selects the control an editable DescriptionItem renders.
type InputType string

const (
	InputText     InputType = "text"   // the zero-value default
	InputNumber   InputType = "number" // a number input; the value travels as a number
	InputSelect   InputType = "select" // a dropdown over Options
	InputCheckbox InputType = "checkbox"
)

var inputTypes = map[InputType]bool{
	InputText: true, InputNumber: true, InputSelect: true, InputCheckbox: true,
}

// Input is the control an editable DescriptionItem renders in place of a
// value. What the reader puts in it is collected at call time and merged into
// the arguments of the widget's own call: a DatePicker's submit, alongside the
// picked date, or the action buttons of the card the item sits in.
//
// It is the short form of a Form field, for the question a widget asks in
// passing — "how many?", "which warehouse?" — rather than a form of its own.
// A whole set of fields with their own submit button is still a Form.
type Input struct {
	// Name is the tool argument the value travels in (required, unique within
	// the block, and distinct from every other argument the widget's call
	// builds).
	Name string
	// Type defaults to InputText.
	Type InputType
	// Placeholder is the empty-state text. On an InputSelect it is what the
	// dropdown reads before a choice is made.
	Placeholder string
	// Required blocks the widget's call until the control is filled in.
	Required bool
	// Default is the value the control opens on: string-like for text, number
	// and select, bool for InputCheckbox. The item's Key wins over it when the
	// record carries that field.
	Default any
	// Options are the choices of an InputSelect (required for it, invalid
	// otherwise).
	Options []Option
	// Validation adds client-side constraints, exactly as on a form Field.
	// Only Message applies to a select or a checkbox; the rest constrain text
	// length, number bounds and step.
	Validation *Validation
}

func (in *Input) inputType() InputType {
	if in == nil || in.Type == "" {
		return InputText
	}
	return in.Type
}

func (in *Input) validate(ctx string) error {
	if in.Name == "" {
		return fmt.Errorf("%s: Input.Name is required", ctx)
	}
	if !inputTypes[in.inputType()] {
		return fmt.Errorf("%s: unknown input type %q", ctx, in.Type)
	}
	if in.inputType() == InputSelect {
		if len(in.Options) == 0 {
			return fmt.Errorf("%s: a select input needs Options", ctx)
		}
	} else if len(in.Options) > 0 {
		return fmt.Errorf("%s: Options need input type InputSelect", ctx)
	}
	if in.inputType() == InputCheckbox && in.Default != nil {
		if _, ok := in.Default.(bool); !ok {
			return fmt.Errorf("%s: a checkbox Default must be a bool", ctx)
		}
	}
	return nil
}

// config serializes the control for the runtime, which builds it: the whole
// list is rendered client-side, so the control travels as configuration rather
// than as markup.
func (in *Input) config() map[string]any {
	m := map[string]any{"name": in.Name, "type": string(in.inputType())}
	if in.Placeholder != "" {
		m["placeholder"] = in.Placeholder
	}
	if in.Required {
		m["required"] = true
	}
	if in.Default != nil {
		m["default"] = in.Default
	}
	if len(in.Options) > 0 {
		m["options"] = in.Options
	}
	if v := in.Validation; v != nil {
		if v.Pattern != "" {
			m["pattern"] = v.Pattern
		}
		if v.Min != nil {
			m["min"] = *v.Min
		}
		if v.Max != nil {
			m["max"] = *v.Max
		}
		if v.Step != nil {
			m["step"] = *v.Step
		}
		if v.MinLen != nil {
			m["minLength"] = *v.MinLen
		}
		if v.MaxLen != nil {
			m["maxLength"] = *v.MaxLen
		}
		if v.Message != "" {
			m["message"] = v.Message
		}
	}
	return m
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
	if i.Input != nil {
		// An input item carries no column: nothing about it is typed, formatted
		// or sorted, and the control's own configuration says everything the
		// runtime needs.
		m := map[string]any{"key": i.Key, "label": i.Label, "input": i.Input.config()}
		if i.Align != "" {
			m["align"] = string(i.Align)
		}
		return m
	}
	m := columnConfig(i.column())
	// A description item is never a sort key; columnConfig emits the flag for
	// tables and card lists, which is noise in this payload.
	delete(m, "sortable")
	if i.Text != "" {
		m["text"] = i.Text
	}
	return m
}

func (i DescriptionItem) validate(ctx string, inputs bool) error {
	if i.Label == "" {
		return fmt.Errorf("%s: Label is required", ctx)
	}
	if i.Input != nil {
		if !inputs {
			return fmt.Errorf("%s: this widget's details are read-only; an Input needs a widget that owns a call", ctx)
		}
		// An input asks for the value, so nothing here may also state it or
		// describe how it would have been rendered.
		if i.Text != "" {
			return fmt.Errorf("%s: Input and Text are mutually exclusive", ctx)
		}
		if i.Type != "" {
			return fmt.Errorf("%s: an Input item cannot be typed %q", ctx, i.Type)
		}
		if i.Format != "" || len(i.Badge) > 0 || i.Link != nil {
			return fmt.Errorf("%s: Format, Badge and Link describe a value, not an Input", ctx)
		}
		return i.Input.validate(ctx)
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

// validate checks every item and rejects two items reading the same field, or
// two inputs sharing an argument name. inputs says whether the embedding
// widget can carry what an input collects: a widget with no call of its own
// has nowhere to send it.
func (d Descriptions) validate(ctx string, inputs bool) error {
	seen := map[string]bool{}
	named := map[string]bool{}
	for n, item := range d.Items {
		ictx := fmt.Sprintf("%s: item %d (%s)", ctx, n, item.Label)
		if err := item.validate(ictx, inputs); err != nil {
			return err
		}
		if item.Input != nil {
			if named[item.Input.Name] {
				return fmt.Errorf("%s: duplicate input name %q", ictx, item.Input.Name)
			}
			named[item.Input.Name] = true
			// An input's Key is a prefill source, not a value on display, so two
			// items may well read the same field — one to show it, one to edit it.
			continue
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

// inputNames lists the arguments the block's controls fill, in reading order.
// The embedding widget checks them against the arguments its own call already
// builds: one name, one value.
func (d Descriptions) inputNames() []string {
	var names []string
	for _, item := range d.Items {
		if item.Input != nil {
			names = append(names, item.Input.Name)
		}
	}
	return names
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
