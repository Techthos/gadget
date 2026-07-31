package gomukit

import (
	"fmt"

	"github.com/techthos/gomukit/theme"
	"github.com/techthos/gomukit/uispec"
)

// Form is a create/edit form widget: typed fields with client-side
// validation, submit as an MCP tool call, and server-side field errors
// mapped back inline.
//
// For edit forms, prefill values arrive at runtime in the tool result's
// structuredContent under PrefillKey; the submit call's response may return
// field errors under ErrorsKey ({"field": "message"}).
type Form struct {
	// URI is the widget's ui:// resource URI (required).
	URI string
	// Title is shown above the form and as the document title.
	Title string
	// Fields defines the ungrouped form fields, rendered above FieldSets.
	// A form needs at least one field, here or in a FieldSet.
	Fields []Field
	// FieldSets are titled groups of fields, rendered in order after Fields.
	FieldSets []FieldSet
	// Columns is how many columns the fields lay themselves out in: 1 (the
	// zero value) through 4. A FieldSet may set its own; a Field may span
	// several. The grid drops to fewer columns as the widget narrows, so the
	// same document reads in a chat pane and in a wide panel.
	Columns int
	// Submit configures the submit tool call (required).
	Submit SubmitSpec
	// Cancel, when set, adds a reset button.
	Cancel *CancelSpec

	// PrefillKey is the structuredContent key holding {"field": value}
	// prefill data. Defaults to "values".
	PrefillKey string
	// ErrorsKey is the structuredContent key holding {"field": "message"}
	// validation errors. Defaults to "errors".
	ErrorsKey string

	// InitialData is an optional structuredContent-shaped snapshot baked
	// into the document (e.g. {"values": {...}} for a pre-filled edit form).
	InitialData map[string]any

	// LoadTool, when set, names a read tool the runtime calls once on load
	// (after the host handshake) to hydrate the form's prefill from fresh
	// data, replacing the baked InitialData snapshot. The tool must return
	// the prefill values under PrefillKey in its structuredContent.
	LoadTool string
	// LoadArgs are optional static arguments passed to LoadTool.
	LoadArgs map[string]any

	// Brand renders the application logo/name on the widget.
	Brand *Brand
	// Theme overrides gomukit design tokens for this widget.
	Theme *theme.Theme
	// UI overrides resource _meta.ui.
	UI *uispec.ResourceUIMeta
}

// SubmitSpec configures form submission.
type SubmitSpec struct {
	// Tool is the MCP tool called with {field: value, ...} merged over
	// StaticArgs (required).
	Tool string
	// Label defaults to "Submit".
	Label string
	// StaticArgs are fixed arguments merged under the field values.
	StaticArgs map[string]any
	// SuccessMessage is shown after a successful submit.
	SuccessMessage string
}

// CancelSpec adds a reset button to the form.
type CancelSpec struct {
	// Label defaults to "Cancel".
	Label string
}

// FieldSet is a titled group of fields — the block a long form is read in
// rather than as one undifferentiated column. It renders as a <fieldset>, so
// assistive technology announces the group's title with every control in it.
//
// Its fields share the form's namespace: a name is unique across the whole
// form, grouped or not, and every field submits the same way.
type FieldSet struct {
	// Title names the group (required).
	Title string
	// Description is a line under the title saying what the group is for.
	Description string
	// Fields are the group's fields (required, non-empty).
	Fields []Field
	// Columns overrides Form.Columns for this group (1..4). Zero inherits.
	Columns int
	// Boxed draws the group as a bordered panel with a filled header, rather
	// than a heading over a rule.
	Boxed bool
}

// maxFormColumns caps the grid. Past four, a column is narrower than the
// controls in it at any width a widget is given.
const maxFormColumns = 4

// columns is the form-wide default: one, unless asked for more.
func (f *Form) columns() int {
	if f.Columns <= 0 {
		return 1
	}
	return f.Columns
}

// columns is the group's own count, inheriting the form's when unset.
func (fs FieldSet) columns(form int) int {
	if fs.Columns <= 0 {
		return form
	}
	return fs.Columns
}

// fieldGroup is one laid-out block of fields: the form's ungrouped Fields, or
// a FieldSet. Validation and rendering both walk this list, so what they see
// cannot drift apart.
type fieldGroup struct {
	set    *FieldSet // nil for the ungrouped block
	fields []Field
	cols   int
	ctx    string // error-message prefix
}

// groups lists the form's blocks in rendering order. The ungrouped block is
// omitted when empty, so a form built entirely out of field sets renders no
// stray grid before them.
func (f *Form) groups() []fieldGroup {
	cols := f.columns()
	var out []fieldGroup
	if len(f.Fields) > 0 {
		out = append(out, fieldGroup{fields: f.Fields, cols: cols, ctx: "gomukit: form " + f.URI})
	}
	for i := range f.FieldSets {
		fs := &f.FieldSets[i]
		out = append(out, fieldGroup{
			set:    fs,
			fields: fs.Fields,
			cols:   fs.columns(cols),
			ctx:    fmt.Sprintf("gomukit: form %s: fieldset %d (%s)", f.URI, i, fs.Title),
		})
	}
	return out
}

// widestGroup is the largest column count the form lays out anywhere. The
// document's own width follows it: a two-column form needs more room than the
// single column the widget otherwise caps itself at.
func (f *Form) widestGroup() int {
	widest := 1
	for _, gr := range f.groups() {
		if gr.cols > widest {
			widest = gr.cols
		}
	}
	return widest
}

// allFields walks every field of the form, grouped or not, in reading order.
// The runtime knows nothing about groups — a field is a field — so this is
// what the config island and the name checks are built from.
func (f *Form) allFields() []Field {
	out := make([]Field, 0, len(f.Fields))
	for _, gr := range f.groups() {
		out = append(out, gr.fields...)
	}
	return out
}

// FieldType selects the control a Field renders.
type FieldType string

const (
	FText        FieldType = "text" // the zero-value default
	FTextarea    FieldType = "textarea"
	FNumber      FieldType = "number"
	FCheckbox    FieldType = "checkbox"
	FSelect      FieldType = "select"
	FMultiSelect FieldType = "multiselect"
	FDate        FieldType = "date"
	FDateRange   FieldType = "daterange"
	FTime        FieldType = "time"
	FHidden      FieldType = "hidden"
	FReadonly    FieldType = "readonly"
)

// dateFieldTypes are the field types that render a calendar. Both keep native
// date inputs as their value holders, so a document whose script never runs
// still has a working control.
var dateFieldTypes = map[FieldType]bool{FDate: true, FDateRange: true}

// Option is a choice in a select or multiselect field.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Opt returns an Option whose label equals its value.
func Opt(value string) Option { return Option{Value: value, Label: value} }

// Validation declares client-side constraints, rendered as native HTML
// validation attributes and enforced before submit.
type Validation struct {
	// Pattern is a regular expression the value must match (HTML pattern
	// attribute semantics).
	Pattern string
	// Min/Max/Step constrain number, date, and time fields.
	Min  *float64
	Max  *float64
	Step *float64
	// MinLen/MaxLen constrain text lengths.
	MinLen *int
	MaxLen *int
	// Message overrides the browser's validation message.
	Message string
}

// Field defines one form field.
type Field struct {
	// Name is the tool-call argument name (required, unique).
	Name  string
	Label string
	// Description renders as help text under the control.
	Description string
	Placeholder string
	// Type defaults to FText.
	Type FieldType
	// Required marks the field as mandatory.
	Required bool
	// Default is the initial value: string-like for most fields, bool for
	// FCheckbox, []string for FMultiSelect, and []string{start, end} for
	// FDateRange.
	Default any
	// Options are required for FSelect and FMultiSelect.
	Options []Option
	// Validation adds client-side constraints. Date fields take their bounds
	// from Calendar instead.
	Validation *Validation
	// Rows sets the textarea height (FTextarea).
	Rows int
	// Span is how many of its group's columns the field occupies: 1 (the zero
	// value) through the group's own column count, so a field can take the
	// whole row in a two-column form. It is ignored while the grid is down to
	// one column.
	Span int

	// Calendar configures the grid an FDate or FDateRange field opens, exactly
	// as it configures the standalone DatePicker widget: bounds, blocked days,
	// presets, month travel. Nil is the zero value — one month for a date, two
	// for a range, every day selectable.
	Calendar *Calendar
	// EndName is the tool-call argument carrying a range's end date
	// (FDateRange only). Defaults to Name + "_end". It shares the field's
	// namespace, so it must not collide with another field's Name.
	EndName string
}

// span is the number of columns the field takes, clamped to what the group
// actually has. Validate rejects a span too wide for its group; the clamp is
// what keeps a rendering honest anyway.
func (f Field) span(cols int) int {
	if f.Span <= 1 {
		return 1
	}
	if f.Span > cols {
		return cols
	}
	return f.Span
}

func (f Field) fieldType() FieldType {
	if f.Type == "" {
		return FText
	}
	return f.Type
}

// dateMode is the calendar mode the field's type implies.
func (f Field) dateMode() DateMode {
	if f.fieldType() == FDateRange {
		return DateRange
	}
	return DateSingle
}

// endName is the argument a range's end date travels in. Derived from Name
// rather than required, so the common case is one line.
func (f Field) endName() string {
	if f.fieldType() != FDateRange {
		return ""
	}
	if f.EndName != "" {
		return f.EndName
	}
	return f.Name + "_end"
}

// rangeDefaults splits an FDateRange Default into its two ends. Anything else
// than a two-element string list is no default at all.
func (f Field) rangeDefaults() (string, string) {
	switch x := f.Default.(type) {
	case []string:
		switch len(x) {
		case 1:
			return x[0], ""
		case 2:
			return x[0], x[1]
		}
	case [2]string:
		return x[0], x[1]
	case string:
		return x, ""
	}
	return "", ""
}

// --- Widget implementation ---

func (f *Form) prefillKey() string {
	if f.PrefillKey == "" {
		return "values"
	}
	return f.PrefillKey
}

func (f *Form) errorsKey() string {
	if f.ErrorsKey == "" {
		return "errors"
	}
	return f.ErrorsKey
}

// Validate implements Widget.
func (f *Form) Validate() error {
	if err := uispec.ValidateURI(f.URI); err != nil {
		return fmt.Errorf("gomukit: form: %w", err)
	}
	if len(f.Fields) == 0 && len(f.FieldSets) == 0 {
		return fmt.Errorf("gomukit: form %s: at least one field is required", f.URI)
	}
	if f.Submit.Tool == "" {
		return fmt.Errorf("gomukit: form %s: Submit.Tool is required", f.URI)
	}
	if f.Columns < 0 || f.Columns > maxFormColumns {
		return fmt.Errorf("gomukit: form %s: Columns must be 1..%d, got %d", f.URI, maxFormColumns, f.Columns)
	}
	for i, fs := range f.FieldSets {
		ctx := fmt.Sprintf("gomukit: form %s: fieldset %d (%s)", f.URI, i, fs.Title)
		if fs.Title == "" {
			return fmt.Errorf("%s: Title is required", ctx)
		}
		if len(fs.Fields) == 0 {
			return fmt.Errorf("%s: at least one field is required", ctx)
		}
		if fs.Columns < 0 || fs.Columns > maxFormColumns {
			return fmt.Errorf("%s: Columns must be 1..%d, got %d", ctx, maxFormColumns, fs.Columns)
		}
	}
	// One namespace for both: a range's end argument is as much a tool
	// argument as any field name, and a collision would send one value where
	// the tool expects the other.
	seen := map[string]bool{}
	claim := func(ctx, name, what string) error {
		if seen[name] {
			return fmt.Errorf("%s: duplicate %s %q", ctx, what, name)
		}
		seen[name] = true
		return nil
	}
	for _, gr := range f.groups() {
		for i, fd := range gr.fields {
			if err := validateField(gr, i, fd, claim); err != nil {
				return err
			}
		}
	}
	if err := f.Brand.Validate(); err != nil {
		return fmt.Errorf("gomukit: form %s: %w", f.URI, err)
	}
	if err := f.Theme.Validate(); err != nil {
		return fmt.Errorf("gomukit: form %s: %w", f.URI, err)
	}
	return nil
}

// validateField checks one field within the group it is laid out in. claim
// reserves its tool-argument names against every other field of the form.
func validateField(gr fieldGroup, i int, fd Field, claim func(ctx, name, what string) error) error {
	ctx := fmt.Sprintf("%s: field %d (%s)", gr.ctx, i, fd.Name)
	if fd.Name == "" {
		return fmt.Errorf("%s: name is required", ctx)
	}
	if err := claim(ctx, fd.Name, "field name"); err != nil {
		return err
	}
	if fd.Span < 0 || fd.Span > gr.cols {
		return fmt.Errorf("%s: Span must be 1..%d (the columns of its group), got %d", ctx, gr.cols, fd.Span)
	}
	switch fd.fieldType() {
	case FText, FTextarea, FNumber, FCheckbox, FTime, FHidden, FReadonly:
	case FSelect, FMultiSelect:
		if len(fd.Options) == 0 {
			return fmt.Errorf("%s: select fields need options", ctx)
		}
	case FDate, FDateRange:
		if fd.fieldType() == FDateRange {
			if err := claim(ctx, fd.endName(), "field name"); err != nil {
				return err
			}
		}
		start, end := fd.rangeDefaults()
		if err := validateDefaultRange(ctx, start, end); err != nil {
			return err
		}
		if err := fd.Calendar.validate(ctx+": calendar", fd.dateMode()); err != nil {
			return err
		}
		if err := fd.Calendar.validateWithin(ctx, start, end); err != nil {
			return err
		}
	default:
		return fmt.Errorf("%s: unknown field type %q", ctx, fd.Type)
	}
	if fd.Calendar != nil && !dateFieldTypes[fd.fieldType()] {
		return fmt.Errorf("%s: Calendar needs FDate or FDateRange", ctx)
	}
	if fd.EndName != "" && fd.fieldType() != FDateRange {
		return fmt.Errorf("%s: EndName needs FDateRange", ctx)
	}
	return nil
}

// Descriptor implements Widget.
func (f *Form) Descriptor() uispec.ResourceDescriptor {
	return uispec.ResourceDescriptor{
		URI:      f.URI,
		Name:     resourceName(f.URI),
		Title:    f.Title,
		MIMEType: uispec.MIMEType,
		UI:       f.UI,
	}
}

// ToolMeta implements Widget.
func (f *Form) ToolMeta() map[string]any {
	return uispec.ToolUIMeta{ResourceURI: f.URI}.MetaMap()
}

// config builds the #gomu-config island content. The runtime needs field
// names/types for value coercion, plus submit wiring; markup is SSR'd.
func (f *Form) config() map[string]any {
	all := f.allFields()
	fields := make([]map[string]any, len(all))
	for i, fd := range all {
		fc := map[string]any{
			"name": fd.Name,
			"type": string(fd.fieldType()),
		}
		if fd.Validation != nil && fd.Validation.Message != "" {
			fc["message"] = fd.Validation.Message
		}
		// A date field's grid is runtime-built (month names come from the
		// host's locale), so its configuration travels here rather than in the
		// markup — the native inputs the runtime upgrades are the value
		// holders, and they carry no calendar of their own.
		if dateFieldTypes[fd.fieldType()] {
			fc["calendar"] = fd.Calendar.config(fd.dateMode())
			if end := fd.endName(); end != "" {
				fc["endName"] = end
			}
			if fd.Required {
				fc["required"] = true
			}
		}
		fields[i] = fc
	}
	submit := map[string]any{"tool": f.Submit.Tool}
	if len(f.Submit.StaticArgs) > 0 {
		submit["staticArgs"] = f.Submit.StaticArgs
	}
	if f.Submit.SuccessMessage != "" {
		submit["successMessage"] = f.Submit.SuccessMessage
	}
	cfg := map[string]any{
		"widget":     "form",
		"prefillKey": f.prefillKey(),
		"errorsKey":  f.errorsKey(),
		"submit":     submit,
		"fields":     fields,
	}
	if f.LoadTool != "" {
		cfg["loadTool"] = f.LoadTool
		if len(f.LoadArgs) > 0 {
			cfg["loadArgs"] = f.LoadArgs
		}
	}
	return cfg
}
