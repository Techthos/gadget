package gadget

import (
	"fmt"

	"github.com/techthos/gadget/theme"
	"github.com/techthos/gadget/uispec"
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
	// Fields defines the form fields (required, non-empty).
	Fields []Field
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
	// Theme overrides gadget design tokens for this widget.
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
	FTime        FieldType = "time"
	FHidden      FieldType = "hidden"
	FReadonly    FieldType = "readonly"
)

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
	// FCheckbox, []string for FMultiSelect.
	Default any
	// Options are required for FSelect and FMultiSelect.
	Options []Option
	// Validation adds client-side constraints.
	Validation *Validation
	// Rows sets the textarea height (FTextarea).
	Rows int
}

func (f Field) fieldType() FieldType {
	if f.Type == "" {
		return FText
	}
	return f.Type
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
		return fmt.Errorf("gadget: form: %w", err)
	}
	if len(f.Fields) == 0 {
		return fmt.Errorf("gadget: form %s: at least one field is required", f.URI)
	}
	if f.Submit.Tool == "" {
		return fmt.Errorf("gadget: form %s: Submit.Tool is required", f.URI)
	}
	seen := map[string]bool{}
	for i, fd := range f.Fields {
		ctx := fmt.Sprintf("gadget: form %s: field %d (%s)", f.URI, i, fd.Name)
		if fd.Name == "" {
			return fmt.Errorf("%s: name is required", ctx)
		}
		if seen[fd.Name] {
			return fmt.Errorf("%s: duplicate field name %q", ctx, fd.Name)
		}
		seen[fd.Name] = true
		switch fd.fieldType() {
		case FText, FTextarea, FNumber, FCheckbox, FDate, FTime, FHidden, FReadonly:
		case FSelect, FMultiSelect:
			if len(fd.Options) == 0 {
				return fmt.Errorf("%s: select fields need options", ctx)
			}
		default:
			return fmt.Errorf("%s: unknown field type %q", ctx, fd.Type)
		}
	}
	if err := f.Brand.Validate(); err != nil {
		return fmt.Errorf("gadget: form %s: %w", f.URI, err)
	}
	if err := f.Theme.Validate(); err != nil {
		return fmt.Errorf("gadget: form %s: %w", f.URI, err)
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

// config builds the #gadget-config island content. The runtime needs field
// names/types for value coercion, plus submit wiring; markup is SSR'd.
func (f *Form) config() map[string]any {
	fields := make([]map[string]any, len(f.Fields))
	for i, fd := range f.Fields {
		fc := map[string]any{
			"name": fd.Name,
			"type": string(fd.fieldType()),
		}
		if fd.Validation != nil && fd.Validation.Message != "" {
			fc["message"] = fd.Validation.Message
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
