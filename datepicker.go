package gadget

import (
	"fmt"

	"github.com/techthos/gadget/theme"
	"github.com/techthos/gadget/uispec"
)

// DatePicker asks the reader for a date, or for the span between two, and then
// to submit it. Nothing happens until they do: picking is local, and only the
// submit button calls a tool.
//
// It is the standalone form of the same grid a Form's FDate and FDateRange
// fields render in a popover. Use the widget when the date is the whole
// question — "when should this ship?", "which nights?" — and the field when it
// is one answer among several. The grid is configured identically either way:
// see Calendar for bounds, blocked days, presets and month travel.
//
// The calendar is inline rather than behind a trigger: the widget is a view of
// its own, and a view whose only job is a calendar should not ask to be opened.
//
// What the reader picks travels as plain "YYYY-MM-DD" strings: Submit.ValueArg
// carries the date, and in a range picker Submit.EndArg carries the end.
//
// The decision is final. Once the submit call succeeds, or the reader cancels,
// the controls are gone; a failed submit re-arms so a transient error can be
// retried.
type DatePicker struct {
	// URI is the widget's ui:// resource URI (required).
	URI string
	// Title is shown in the toolbar and the document title.
	Title string
	// Prompt is the headline question, e.g. "When should we deliver?"
	// (required).
	Prompt string
	// Body is supporting prose under the prompt.
	Body string

	// Mode picks one date (the default) or a span.
	Mode DateMode
	// Calendar configures the grid. Nil is the zero value: one month for a
	// single date, two for a range, every day selectable.
	Calendar *Calendar

	// Default preselects a date ("YYYY-MM-DD"). In a range picker it is the
	// start, and DefaultEnd is the end.
	Default string
	// DefaultEnd preselects the end of the span (DateRange only).
	DefaultEnd string

	// Details describes the record the question is about, read from rows[0].
	// It sits under the prompt, above the calendar.
	Details Descriptions

	// Submit configures the call the decision makes (required).
	Submit DateSubmit
	// Cancel configures the declining button. Nil renders no such button,
	// leaving the host's own affordances as the only way out.
	Cancel *RejectSpec

	// ValueKey is the structuredContent key holding the runtime selection and
	// any runtime bounds. Defaults to "value". See docs/widgets.md for the
	// shape; briefly, a "YYYY-MM-DD" string, or an object with start, end,
	// min, max and disabled.
	ValueKey string
	// RowsKey is the structuredContent key holding the context record; the
	// widget reads rows[0]. Defaults to "rows".
	RowsKey string
	// RowID is the record field used for FromRow args. Defaults to "id".
	RowID string

	// InitialData is an optional structuredContent-shaped snapshot baked into
	// the document as a JSON island.
	InitialData map[string]any

	// LoadTool, when set, names a read tool the runtime calls once on load to
	// fetch the selection and bounds fresh, replacing InitialData. Use it when
	// what is still free to book can change between registration and the
	// question.
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

// DateSubmit configures the call a DatePicker makes once the reader submits.
type DateSubmit struct {
	// Tool is the MCP tool called on submit (required).
	Tool string
	// Label defaults to "Continue".
	Label string
	// ValueArg is the tool argument carrying the picked date, as
	// "YYYY-MM-DD". Defaults to "date" for a single date and "start" for a
	// range.
	ValueArg string
	// EndArg is the tool argument carrying the end of the span. Defaults to
	// "end". DateRange only.
	EndArg string
	// Args maps further tool argument names to their sources. Static and
	// FromRow apply; FromSelection does not — a date has no row selection.
	// Ignored when ChatPrompt is set.
	Args map[string]ArgSource
	// ChatPrompt, when set, makes submitting post a user message (ui/message)
	// instead of calling Tool directly, for hosts that answer a view-initiated
	// call without opening the widget behind it. The picked date is appended
	// to this text, since a picker's whole output is that date and ValueArg
	// has no counterpart in a chat turn.
	//
	// Named apart from DatePicker.Prompt, which is the question put to the
	// reader rather than a message sent on their behalf.
	ChatPrompt string
	// Variant overrides the submit button styling (VariantPrimary).
	Variant ActionVariant
	// SuccessMessage is shown in place of the controls once the call succeeds.
	// Defaults to the tool result's own text.
	SuccessMessage string
}

func (d *DatePicker) mode() DateMode {
	if d.Mode == DateRange {
		return DateRange
	}
	return DateSingle
}

// modeName is the mode as the runtime and the CSS name it; the zero value is
// spelled out rather than left blank.
func (d *DatePicker) modeName() string {
	if d.mode() == DateRange {
		return "range"
	}
	return "single"
}

func (d *DatePicker) valueKey() string {
	if d.ValueKey == "" {
		return "value"
	}
	return d.ValueKey
}

func (d *DatePicker) rowsKey() string {
	if d.RowsKey == "" {
		return "rows"
	}
	return d.RowsKey
}

func (d *DatePicker) rowID() string {
	if d.RowID == "" {
		return "id"
	}
	return d.RowID
}

func (d *DatePicker) submitLabel() string {
	if d.Submit.Label != "" {
		return d.Submit.Label
	}
	return "Continue"
}

func (d *DatePicker) submitVariant() ActionVariant {
	if d.Submit.Variant != "" {
		return d.Submit.Variant
	}
	return VariantPrimary
}

// valueArg names the argument the date travels in. A range's two ends read as
// start and end; a lone date is just the date.
func (d *DatePicker) valueArg() string {
	if d.Submit.ValueArg != "" {
		return d.Submit.ValueArg
	}
	if d.mode() == DateRange {
		return "start"
	}
	return "date"
}

func (d *DatePicker) endArg() string {
	if d.mode() != DateRange {
		return ""
	}
	if d.Submit.EndArg != "" {
		return d.Submit.EndArg
	}
	return "end"
}

func (d *DatePicker) cancelLabel() string {
	if d.Cancel != nil && d.Cancel.Label != "" {
		return d.Cancel.Label
	}
	return "Cancel"
}

func (d *DatePicker) cancelMessage() string {
	if d.Cancel != nil && d.Cancel.Message != "" {
		return d.Cancel.Message
	}
	return "Cancelled."
}

// Validate implements Widget.
func (d *DatePicker) Validate() error {
	if err := uispec.ValidateURI(d.URI); err != nil {
		return fmt.Errorf("gadget: datepicker: %w", err)
	}
	ctx := fmt.Sprintf("gadget: datepicker %s", d.URI)
	if d.Prompt == "" {
		return fmt.Errorf("%s: Prompt is required", ctx)
	}
	if !dateModes[d.Mode] {
		return fmt.Errorf("%s: unknown mode %q", ctx, d.Mode)
	}
	if d.Submit.Tool == "" {
		return fmt.Errorf("%s: Submit.Tool is required", ctx)
	}
	if err := confirmArgs(ctx+": submit", d.Submit.Args); err != nil {
		return err
	}
	// The date and a static argument cannot share a name: one would silently
	// overwrite the other in the call.
	for _, name := range []string{d.valueArg(), d.endArg()} {
		if name == "" {
			continue
		}
		if _, clash := d.Submit.Args[name]; clash {
			return fmt.Errorf("%s: submit: argument %q is also a date argument", ctx, name)
		}
	}
	if d.mode() == DateRange && d.valueArg() == d.endArg() {
		return fmt.Errorf("%s: submit: ValueArg and EndArg are both %q", ctx, d.valueArg())
	}
	if d.Submit.EndArg != "" && d.mode() != DateRange {
		return fmt.Errorf("%s: submit: EndArg needs Mode DateRange", ctx)
	}
	if d.DefaultEnd != "" && d.mode() != DateRange {
		return fmt.Errorf("%s: DefaultEnd needs Mode DateRange", ctx)
	}
	if err := validateDefaultRange(ctx, d.Default, d.DefaultEnd); err != nil {
		return err
	}
	if err := d.Calendar.validate(ctx+": calendar", d.mode()); err != nil {
		return err
	}
	if err := d.Calendar.validateWithin(ctx, d.Default, d.DefaultEnd); err != nil {
		return err
	}
	if d.Cancel != nil {
		if err := confirmArgs(ctx+": cancel", d.Cancel.Args); err != nil {
			return err
		}
		if d.Cancel.Tool == "" && len(d.Cancel.Args) > 0 {
			return fmt.Errorf("%s: cancel: Args need Cancel.Tool", ctx)
		}
	}
	if err := d.Details.validate(ctx + ": details"); err != nil {
		return err
	}
	if err := d.Brand.Validate(); err != nil {
		return fmt.Errorf("%s: %w", ctx, err)
	}
	if err := d.Theme.Validate(); err != nil {
		return fmt.Errorf("%s: %w", ctx, err)
	}
	return nil
}

// validateDefaultRange checks a preselected date, or the pair of them: both
// must parse, and a span must not run backwards.
func validateDefaultRange(ctx, start, end string) error {
	from, err := parseISODate(ctx+": Default", start)
	if err != nil {
		return err
	}
	to, err := parseISODate(ctx+": DefaultEnd", end)
	if err != nil {
		return err
	}
	if end != "" && start == "" {
		return fmt.Errorf("%s: DefaultEnd needs Default", ctx)
	}
	if start != "" && end != "" && to.Before(from) {
		return fmt.Errorf("%s: DefaultEnd %s is before Default %s", ctx, end, start)
	}
	return nil
}

// validateWithin checks preselected dates against the grid's own bounds: a
// default the reader could not have picked is a configuration mistake, not a
// state the widget should open in.
func (c *Calendar) validateWithin(ctx string, dates ...string) error {
	if c == nil {
		return nil
	}
	min, _ := parseISODate("", c.Min)
	max, _ := parseISODate("", c.Max)
	blocked := map[string]bool{}
	for _, d := range c.Disabled {
		blocked[d] = true
	}
	for _, s := range dates {
		if s == "" {
			continue
		}
		day, err := parseISODate(ctx, s)
		if err != nil {
			return err
		}
		if c.Min != "" && day.Before(min) {
			return fmt.Errorf("%s: default date %s is before Calendar.Min %s", ctx, s, c.Min)
		}
		if c.Max != "" && day.After(max) {
			return fmt.Errorf("%s: default date %s is after Calendar.Max %s", ctx, s, c.Max)
		}
		if blocked[s] {
			return fmt.Errorf("%s: default date %s is in Calendar.Disabled", ctx, s)
		}
	}
	return nil
}

// Descriptor implements Widget.
func (d *DatePicker) Descriptor() uispec.ResourceDescriptor {
	return uispec.ResourceDescriptor{
		URI:      d.URI,
		Name:     resourceName(d.URI),
		Title:    d.Title,
		MIMEType: uispec.MIMEType,
		UI:       d.UI,
	}
}

// ToolMeta implements Widget.
func (d *DatePicker) ToolMeta() map[string]any {
	return uispec.ToolUIMeta{ResourceURI: d.URI}.MetaMap()
}

// config serializes what the runtime needs: the grid, the preselection, and
// the two calls. Prompt, body and button labels are already in the markup.
func (d *DatePicker) config() map[string]any {
	submit := map[string]any{"tool": d.Submit.Tool, "valueArg": d.valueArg()}
	if end := d.endArg(); end != "" {
		submit["endArg"] = end
	}
	if d.Submit.ChatPrompt != "" {
		// The chat path never calls the tool from the view, so its args would
		// be dead weight in the island.
		submit["chatPrompt"] = d.Submit.ChatPrompt
	} else if len(d.Submit.Args) > 0 {
		submit["args"] = d.Submit.Args
	}
	if d.Submit.SuccessMessage != "" {
		submit["successMessage"] = d.Submit.SuccessMessage
	}

	cfg := map[string]any{
		"widget":   "datepicker",
		"valueKey": d.valueKey(),
		"rowsKey":  d.rowsKey(),
		"rowId":    d.rowID(),
		"calendar": d.Calendar.config(d.mode()),
		"submit":   submit,
	}
	if d.Default != "" {
		cfg["default"] = d.Default
	}
	if d.DefaultEnd != "" {
		cfg["defaultEnd"] = d.DefaultEnd
	}
	if d.Cancel != nil {
		cancel := map[string]any{"message": d.cancelMessage()}
		if d.Cancel.Tool != "" {
			cancel["tool"] = d.Cancel.Tool
		}
		if len(d.Cancel.Args) > 0 {
			cancel["args"] = d.Cancel.Args
		}
		cfg["cancel"] = cancel
	}
	if !d.Details.empty() {
		cfg["details"] = d.Details.config()
	}
	if d.LoadTool != "" {
		cfg["loadTool"] = d.LoadTool
		if len(d.LoadArgs) > 0 {
			cfg["loadArgs"] = d.LoadArgs
		}
	}
	return cfg
}
