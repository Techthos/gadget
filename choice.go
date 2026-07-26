package gadget

import (
	"fmt"

	"github.com/techthos/gadget/theme"
	"github.com/techthos/gadget/uispec"
)

// ChoiceLayout places the block that describes an option.
//
// The layout is about width, not about taste: a widget with room for two
// columns reads better with the description beside the options, and the same
// widget in a narrow chat pane reads better with it underneath. ChoiceAuto
// picks per width at runtime; the other two settle it for good.
type ChoiceLayout string

const (
	// ChoiceAuto shows the description beside the options while the widget is
	// wide enough for two columns and under the selected option when it is
	// not. It is the default, and the right answer unless the content says
	// otherwise.
	ChoiceAuto ChoiceLayout = ""
	// ChoiceSplit always shows the description in a side panel.
	ChoiceSplit ChoiceLayout = "split"
	// ChoiceStacked always shows the description under its own option.
	ChoiceStacked ChoiceLayout = "stacked"
)

var choiceLayouts = map[ChoiceLayout]bool{
	ChoiceAuto:    true,
	ChoiceSplit:   true,
	ChoiceStacked: true,
}

// Choice asks the reader to pick — one option or several — and then to submit.
// Nothing happens until they do: picking is local, and only the submit button
// calls a tool.
//
// It is the deliberating counterpart to Confirm. Confirm asks a yes/no question
// about one operation; Choice asks which operation, and gives every candidate
// room to argue for itself: a summary in the list, and a description block with
// prose, bullets and a typed detail list. Where that block sits is a matter of
// width — see ChoiceLayout.
//
// Options are authored here, arrive at runtime under OptionsKey, or both: a
// runtime list replaces the authored one wholesale, the same contract Confirm
// uses for effects. The record the question is about (Details, and FromRow
// submit arguments) arrives under RowsKey as rows[0].
//
// The decision is final. Once the submit call succeeds, or the reader cancels,
// the controls are gone; a failed submit re-arms so a transient error can be
// retried.
type Choice struct {
	// URI is the widget's ui:// resource URI (required).
	URI string
	// Title is shown in the toolbar and the document title.
	Title string
	// Prompt is the headline question, e.g. "How should we ship this?"
	// (required).
	Prompt string
	// Body is supporting prose under the prompt.
	Body string

	// Layout places the description block. Defaults to ChoiceAuto.
	Layout ChoiceLayout
	// Multiple lets the reader pick more than one option. Without it the
	// options are radios and exactly one can be chosen.
	Multiple bool
	// Min is the fewest options a multiple choice accepts. Defaults to 1 —
	// submitting nothing is not a choice. Ignored unless Multiple.
	Min int
	// Max is the most a multiple choice accepts; 0 means no limit. Once Max
	// options are ticked the rest disable, so the limit is visible rather
	// than only enforced. Ignored unless Multiple.
	Max int

	// Options are the candidates, in reading order. May be empty when they
	// arrive at runtime under OptionsKey.
	Options []ChoiceOption
	// Details describes the record the question is about, read from rows[0].
	// It sits under the prompt, above the options.
	Details Descriptions

	// Submit configures the call the decision makes (required).
	Submit ChoiceSubmit
	// Cancel configures the declining button. Nil renders no such button,
	// leaving the host's own affordances as the only way out.
	Cancel *RejectSpec

	// RowsKey is the structuredContent key holding the context record; the
	// widget reads rows[0]. Defaults to "rows".
	RowsKey string
	// OptionsKey is the structuredContent key holding the option array.
	// Defaults to "options".
	OptionsKey string
	// RowID is the record field used for FromRow args. Defaults to "id".
	RowID string

	// InitialData is an optional structuredContent-shaped snapshot baked into
	// the document as a JSON island.
	InitialData map[string]any

	// LoadTool, when set, names a read tool the runtime calls once on load to
	// fetch the options and the record fresh, replacing InitialData. Use it
	// when what is on offer can change between registration and the question.
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

// ChoiceOption is one candidate. Label and Summary are what the list shows;
// Body, Bullets and Details are the case for it, shown in the description
// block once it is the option in hand.
//
// Details items typed with a Key read their value from Data — the option's own
// record — so an option can carry facts that are formatted for the host's
// locale rather than pre-rendered into strings.
type ChoiceOption struct {
	// Value is what the submit call sends for this option (required, unique).
	Value string
	// Label is the option's name in the list. Defaults to Value.
	Label string
	// Summary is a single supporting line under the label, shown in the list
	// whether or not the option is selected.
	Summary string

	// Body is prose about the option, shown in the description block.
	Body string
	// Bullets are short points about the option, listed under Body.
	Bullets []string
	// Details is a label/value list about the option, shown under Bullets.
	// Items with a Key read Data; items with Text are fixed.
	Details Descriptions
	// Data is the option's record, the source for Key detail items.
	Data map[string]any

	// Badge is optional short text beside the label — "recommended",
	// "cheapest".
	Badge string
	// BadgeVariant colors the badge. Defaults to BadgeNeutral.
	BadgeVariant BadgeVariant

	// Default preselects the option. A single choice takes at most one.
	Default bool
	// Disabled renders the option unselectable — on offer, but not now.
	Disabled bool
}

func (o ChoiceOption) label() string {
	if o.Label != "" {
		return o.Label
	}
	return o.Value
}

func (o ChoiceOption) validate(ctx string) error {
	if o.Value == "" {
		return fmt.Errorf("%s: Value is required", ctx)
	}
	if o.BadgeVariant != "" && !badgeVariants[o.BadgeVariant] {
		return fmt.Errorf("%s: unknown badge variant %q", ctx, o.BadgeVariant)
	}
	if o.BadgeVariant != "" && o.Badge == "" {
		return fmt.Errorf("%s: BadgeVariant needs Badge text", ctx)
	}
	for n, b := range o.Bullets {
		if b == "" {
			return fmt.Errorf("%s: bullet %d is empty", ctx, n)
		}
	}
	if o.Default && o.Disabled {
		return fmt.Errorf("%s: a disabled option cannot be the default", ctx)
	}
	return o.Details.validate(ctx + ": details")
}

// config serializes the option. Everything about it is runtime state — the
// list is rebuilt whenever options arrive from a tool — so labels travel here
// rather than in the markup.
func (o ChoiceOption) config() map[string]any {
	m := map[string]any{"value": o.Value, "label": o.label()}
	if o.Summary != "" {
		m["summary"] = o.Summary
	}
	if o.Body != "" {
		m["body"] = o.Body
	}
	if len(o.Bullets) > 0 {
		m["bullets"] = o.Bullets
	}
	if !o.Details.empty() {
		m["details"] = o.Details.config()
	}
	if len(o.Data) > 0 {
		m["data"] = o.Data
	}
	if o.Badge != "" {
		m["badge"] = o.Badge
		if o.BadgeVariant != "" {
			m["badgeVariant"] = string(o.BadgeVariant)
		}
	}
	if o.Default {
		m["default"] = true
	}
	if o.Disabled {
		m["disabled"] = true
	}
	return m
}

// ChoiceSubmit configures the call a Choice makes once the reader submits.
type ChoiceSubmit struct {
	// Tool is the MCP tool called on submit (required).
	Tool string
	// Label defaults to "Continue".
	Label string
	// ValueArg is the tool argument carrying the decision: the chosen option's
	// Value, or the array of chosen values in a multiple choice. Defaults to
	// "choice".
	ValueArg string
	// Args maps further tool argument names to their sources. Static and
	// FromRow apply; FromSelection does not — a choice has no row selection.
	// Ignored when ChatPrompt is set.
	Args map[string]ArgSource
	// ChatPrompt, when set, makes submitting post a user message (ui/message)
	// instead of calling Tool directly, for hosts that answer a view-initiated
	// call without opening the widget behind it. The reader's decision is
	// appended to this text, since a choice's whole output is what they picked
	// and ValueArg has no counterpart in a chat turn.
	//
	// Named apart from Choice.Prompt, which is the question put to the reader
	// rather than a message sent on their behalf.
	ChatPrompt string
	// Variant overrides the submit button styling (VariantPrimary).
	Variant ActionVariant
	// SuccessMessage is shown in place of the controls once the call succeeds.
	// Defaults to the tool result's own text.
	SuccessMessage string
}

func (c *Choice) rowsKey() string {
	if c.RowsKey == "" {
		return "rows"
	}
	return c.RowsKey
}

func (c *Choice) optionsKey() string {
	if c.OptionsKey == "" {
		return "options"
	}
	return c.OptionsKey
}

func (c *Choice) rowID() string {
	if c.RowID == "" {
		return "id"
	}
	return c.RowID
}

func (c *Choice) layout() ChoiceLayout {
	if c.Layout == "" {
		return ChoiceAuto
	}
	return c.Layout
}

// layoutName is the layout as the runtime and the CSS name it; the zero value
// is spelled out rather than left blank.
func (c *Choice) layoutName() string {
	if c.layout() == ChoiceAuto {
		return "auto"
	}
	return string(c.layout())
}

func (c *Choice) submitLabel() string {
	if c.Submit.Label != "" {
		return c.Submit.Label
	}
	return "Continue"
}

func (c *Choice) submitVariant() ActionVariant {
	if c.Submit.Variant != "" {
		return c.Submit.Variant
	}
	return VariantPrimary
}

func (c *Choice) valueArg() string {
	if c.Submit.ValueArg != "" {
		return c.Submit.ValueArg
	}
	return "choice"
}

func (c *Choice) cancelLabel() string {
	if c.Cancel != nil && c.Cancel.Label != "" {
		return c.Cancel.Label
	}
	return "Cancel"
}

func (c *Choice) cancelMessage() string {
	if c.Cancel != nil && c.Cancel.Message != "" {
		return c.Cancel.Message
	}
	return "Cancelled."
}

// min is the floor a multiple choice enforces. One, unless the author asked
// for more: a decision with nothing chosen is not a decision.
func (c *Choice) min() int {
	if !c.Multiple {
		return 1
	}
	if c.Min <= 0 {
		return 1
	}
	return c.Min
}

// Validate implements Widget.
func (c *Choice) Validate() error {
	if err := uispec.ValidateURI(c.URI); err != nil {
		return fmt.Errorf("gadget: choice: %w", err)
	}
	ctx := fmt.Sprintf("gadget: choice %s", c.URI)
	if c.Prompt == "" {
		return fmt.Errorf("%s: Prompt is required", ctx)
	}
	if !choiceLayouts[c.Layout] {
		return fmt.Errorf("%s: unknown layout %q", ctx, c.Layout)
	}
	if c.Submit.Tool == "" {
		return fmt.Errorf("%s: Submit.Tool is required", ctx)
	}
	if err := confirmArgs(ctx+": submit", c.Submit.Args); err != nil {
		return err
	}
	// The decision and a static argument cannot share a name: one would
	// silently overwrite the other in the call.
	if _, clash := c.Submit.Args[c.valueArg()]; clash {
		return fmt.Errorf("%s: submit: argument %q is also Submit.ValueArg", ctx, c.valueArg())
	}
	if !c.Multiple && (c.Min != 0 || c.Max != 0) {
		return fmt.Errorf("%s: Min and Max need Multiple", ctx)
	}
	if c.Min < 0 || c.Max < 0 {
		return fmt.Errorf("%s: Min and Max cannot be negative", ctx)
	}
	if c.Max > 0 && c.min() > c.Max {
		return fmt.Errorf("%s: Min %d exceeds Max %d", ctx, c.min(), c.Max)
	}
	if c.Cancel != nil {
		if err := confirmArgs(ctx+": cancel", c.Cancel.Args); err != nil {
			return err
		}
		if c.Cancel.Tool == "" && len(c.Cancel.Args) > 0 {
			return fmt.Errorf("%s: cancel: Args need Cancel.Tool", ctx)
		}
	}
	if err := c.Details.validate(ctx + ": details"); err != nil {
		return err
	}
	seen := map[string]bool{}
	defaults := 0
	for n, o := range c.Options {
		octx := fmt.Sprintf("%s: option %d (%s)", ctx, n, o.label())
		if err := o.validate(octx); err != nil {
			return err
		}
		if seen[o.Value] {
			return fmt.Errorf("%s: duplicate option value %q", octx, o.Value)
		}
		seen[o.Value] = true
		if o.Default {
			defaults++
		}
	}
	if !c.Multiple && defaults > 1 {
		return fmt.Errorf("%s: a single choice takes at most one Default option, got %d", ctx, defaults)
	}
	if c.Max > 0 && defaults > c.Max {
		return fmt.Errorf("%s: %d Default options exceed Max %d", ctx, defaults, c.Max)
	}
	if err := c.Brand.Validate(); err != nil {
		return fmt.Errorf("%s: %w", ctx, err)
	}
	if err := c.Theme.Validate(); err != nil {
		return fmt.Errorf("%s: %w", ctx, err)
	}
	return nil
}

// Descriptor implements Widget.
func (c *Choice) Descriptor() uispec.ResourceDescriptor {
	return uispec.ResourceDescriptor{
		URI:      c.URI,
		Name:     resourceName(c.URI),
		Title:    c.Title,
		MIMEType: uispec.MIMEType,
		UI:       c.UI,
	}
}

// ToolMeta implements Widget.
func (c *Choice) ToolMeta() map[string]any {
	return uispec.ToolUIMeta{ResourceURI: c.URI}.MetaMap()
}

// config serializes what the runtime needs: the selection rules it enforces,
// the options it renders until a tool result supplies its own, and the two
// calls. Prompt, body and button labels are already in the markup.
func (c *Choice) config() map[string]any {
	submit := map[string]any{"tool": c.Submit.Tool, "valueArg": c.valueArg()}
	if c.Submit.ChatPrompt != "" {
		// The chat path never calls the tool from the view, so its args would
		// be dead weight in the island.
		submit["chatPrompt"] = c.Submit.ChatPrompt
	} else if len(c.Submit.Args) > 0 {
		submit["args"] = c.Submit.Args
	}
	if c.Submit.SuccessMessage != "" {
		submit["successMessage"] = c.Submit.SuccessMessage
	}

	cfg := map[string]any{
		"widget":     "choice",
		"layout":     c.layoutName(),
		"rowsKey":    c.rowsKey(),
		"optionsKey": c.optionsKey(),
		"rowId":      c.rowID(),
		"min":        c.min(),
		"submit":     submit,
	}
	if c.Multiple {
		cfg["multiple"] = true
		if c.Max > 0 {
			cfg["max"] = c.Max
		}
	}
	if c.Cancel != nil {
		cancel := map[string]any{"message": c.cancelMessage()}
		if c.Cancel.Tool != "" {
			cancel["tool"] = c.Cancel.Tool
		}
		if len(c.Cancel.Args) > 0 {
			cancel["args"] = c.Cancel.Args
		}
		cfg["cancel"] = cancel
	}
	if !c.Details.empty() {
		cfg["details"] = c.Details.config()
	}
	if len(c.Options) > 0 {
		opts := make([]map[string]any, len(c.Options))
		for n, o := range c.Options {
			opts[n] = o.config()
		}
		cfg["options"] = opts
	}
	if c.LoadTool != "" {
		cfg["loadTool"] = c.LoadTool
		if len(c.LoadArgs) > 0 {
			cfg["loadArgs"] = c.LoadArgs
		}
	}
	return cfg
}
