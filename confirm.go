package gomukit

import (
	"fmt"

	"github.com/techthos/gomukit/theme"
	"github.com/techthos/gomukit/uispec"
)

// Confirm is an approval widget: it states an operation, shows the record it
// targets and the side effects it will have, and offers exactly two outcomes —
// accept, which calls a tool, or reject, which does not.
//
// It is the long form of Action.Confirm. That one re-labels a button for a
// second click and has room for a few words; this one is a view of its own,
// for operations whose consequences the reader has to weigh before deciding.
//
// The prompt, the effects a server can name up front, and the guards are
// authored here. Everything that depends on the particular call arrives at
// runtime in structuredContent: the record under RowsKey (the widget shows
// rows[0], same contract as Card) and the side effects under EffectsKey,
// which replace the authored ones.
//
// A decision is final: once the reader accepts and the tool succeeds, or
// rejects, the buttons are gone. A failed accept is the exception — the
// widget re-arms so a transient error can be retried.
type Confirm struct {
	// URI is the widget's ui:// resource URI (required).
	URI string
	// Title is shown in the toolbar and the document title.
	Title string
	// Prompt is the headline question, e.g. "Delete Ada Lovelace?"
	// (required).
	Prompt string
	// Body is supporting prose under the prompt.
	Body string
	// Severity colors the widget: BadgeInfo (the default), BadgeWarning or
	// BadgeDanger. It also picks the accept button's variant unless
	// Accept.Variant says otherwise.
	Severity BadgeVariant

	// Details describes the record the operation targets, read from rows[0].
	Details Descriptions
	// Effects are the side effects known at registration time. Any effects
	// delivered under EffectsKey at runtime replace this list wholesale.
	Effects []Effect

	// Acknowledge, when set, is the label of a checkbox the reader must tick
	// before the accept button enables ("I understand this cannot be undone").
	Acknowledge string
	// TypeToConfirm, when set, is a phrase the reader must type exactly
	// before the accept button enables — the record's name, usually.
	TypeToConfirm string

	// Accept configures the confirming call (required).
	Accept AcceptSpec
	// Reject configures the declining button. Nil renders no such button,
	// leaving the host's own affordances as the only way out.
	Reject *RejectSpec

	// RowsKey is the structuredContent key holding the record array; the
	// widget reads rows[0]. Defaults to "rows".
	RowsKey string
	// EffectsKey is the structuredContent key holding the side-effect array.
	// Defaults to "effects".
	EffectsKey string
	// RowID is the record field used for FromRow args. Defaults to "id".
	RowID string

	// InitialData is an optional structuredContent-shaped snapshot baked into
	// the document as a JSON island.
	InitialData map[string]any

	// LoadTool, when set, names a read tool the runtime calls once on load to
	// fetch the record and effects fresh, replacing InitialData. Use it when
	// the consequences must be current at decision time rather than at render
	// time.
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

// Effect is one consequence of the operation, listed so the reader can weigh
// it before accepting.
type Effect struct {
	// Text states the consequence, e.g. "Deletes the audit trail" (required).
	Text string
	// Detail is a secondary line qualifying it.
	Detail string
	// Value is a magnitude shown at the end of the row, e.g. "12 records".
	Value string
	// Severity colors the row: BadgeNeutral (the default), BadgeInfo,
	// BadgeSuccess, BadgeWarning or BadgeDanger.
	Severity BadgeVariant
}

func (e Effect) validate(ctx string) error {
	if e.Text == "" {
		return fmt.Errorf("%s: Text is required", ctx)
	}
	if e.Severity != "" && !badgeVariants[e.Severity] {
		return fmt.Errorf("%s: unknown severity %q", ctx, e.Severity)
	}
	return nil
}

func (e Effect) config() map[string]any {
	m := map[string]any{"text": e.Text}
	if e.Detail != "" {
		m["detail"] = e.Detail
	}
	if e.Value != "" {
		m["value"] = e.Value
	}
	if e.Severity != "" {
		m["severity"] = string(e.Severity)
	}
	return m
}

// AcceptSpec configures the confirming call.
type AcceptSpec struct {
	// Tool is the MCP tool called when the reader accepts (required).
	Tool string
	// Label defaults to "Confirm".
	Label string
	// Args maps tool argument names to their sources. Static and FromRow
	// apply; FromSelection does not — a confirmation has no selection.
	// Ignored when ChatPrompt is set.
	Args map[string]ArgSource
	// ChatPrompt, when set, makes accepting post this text as a user message
	// (ui/message) instead of calling Tool directly, for hosts that answer a
	// view-initiated call without opening the widget behind it. Write it as
	// the request a user would type; the model then makes the call.
	//
	// Named apart from Confirm.Prompt, which is the question put to the
	// reader rather than a message sent on their behalf.
	ChatPrompt string
	// Variant overrides the button styling derived from Severity.
	Variant ActionVariant
	// SuccessMessage is shown in place of the buttons once the call succeeds.
	// Defaults to the tool result's own text.
	SuccessMessage string
}

// RejectSpec configures the declining button.
type RejectSpec struct {
	// Label defaults to "Cancel".
	Label string
	// Tool is an optional MCP tool called when the reader declines. Without
	// it, declining is a local outcome the server never hears about — set it
	// when the operation needs an explicit "no".
	Tool string
	// Args maps tool argument names to their sources (Static and FromRow).
	Args map[string]ArgSource
	// Message is shown in place of the buttons once the reader declines.
	// Defaults to "Cancelled."
	Message string
}

func (c *Confirm) rowsKey() string {
	if c.RowsKey == "" {
		return "rows"
	}
	return c.RowsKey
}

func (c *Confirm) effectsKey() string {
	if c.EffectsKey == "" {
		return "effects"
	}
	return c.EffectsKey
}

func (c *Confirm) rowID() string {
	if c.RowID == "" {
		return "id"
	}
	return c.RowID
}

func (c *Confirm) severity() BadgeVariant {
	if c.Severity == "" {
		return BadgeInfo
	}
	return c.Severity
}

func (c *Confirm) acceptLabel() string {
	if c.Accept.Label != "" {
		return c.Accept.Label
	}
	return "Confirm"
}

// acceptVariant styles the accept button. A danger confirmation gets the
// danger button; anything else gets the primary one, because accepting is the
// widget's purpose even when it is only informational.
func (c *Confirm) acceptVariant() ActionVariant {
	if c.Accept.Variant != "" {
		return c.Accept.Variant
	}
	if c.severity() == BadgeDanger {
		return VariantDanger
	}
	return VariantPrimary
}

func (c *Confirm) rejectLabel() string {
	if c.Reject != nil && c.Reject.Label != "" {
		return c.Reject.Label
	}
	return "Cancel"
}

// Validate implements Widget.
func (c *Confirm) Validate() error {
	if err := uispec.ValidateURI(c.URI); err != nil {
		return fmt.Errorf("gomukit: confirm: %w", err)
	}
	ctx := fmt.Sprintf("gomukit: confirm %s", c.URI)
	if c.Prompt == "" {
		return fmt.Errorf("%s: Prompt is required", ctx)
	}
	if !badgeVariants[c.severity()] {
		return fmt.Errorf("%s: unknown severity %q", ctx, c.Severity)
	}
	if c.Accept.Tool == "" {
		return fmt.Errorf("%s: Accept.Tool is required", ctx)
	}
	if err := confirmArgs(ctx+": accept", c.Accept.Args); err != nil {
		return err
	}
	if c.Reject != nil {
		if err := confirmArgs(ctx+": reject", c.Reject.Args); err != nil {
			return err
		}
		if c.Reject.Tool == "" && len(c.Reject.Args) > 0 {
			return fmt.Errorf("%s: reject: Args need Reject.Tool", ctx)
		}
	}
	if err := c.Details.validate(ctx + ": details"); err != nil {
		return err
	}
	for n, e := range c.Effects {
		if err := e.validate(fmt.Sprintf("%s: effect %d", ctx, n)); err != nil {
			return err
		}
	}
	if err := c.Brand.Validate(); err != nil {
		return fmt.Errorf("%s: %w", ctx, err)
	}
	if err := c.Theme.Validate(); err != nil {
		return fmt.Errorf("%s: %w", ctx, err)
	}
	return nil
}

// confirmArgs checks the arg sources of an accept or reject call. A
// confirmation acts on one record, so FromSelection has nothing to resolve
// against.
func confirmArgs(ctx string, args map[string]ArgSource) error {
	for name, src := range args {
		if !src.valid() {
			return fmt.Errorf("%s: argument %q must be built with Static or FromRow", ctx, name)
		}
		if src.selection != "" {
			return fmt.Errorf("%s: argument %q: FromSelection is only valid in bulk actions", ctx, name)
		}
	}
	return nil
}

// Descriptor implements Widget.
func (c *Confirm) Descriptor() uispec.ResourceDescriptor {
	return uispec.ResourceDescriptor{
		URI:      c.URI,
		Name:     resourceName(c.URI),
		Title:    c.Title,
		MIMEType: uispec.MIMEType,
		UI:       c.UI,
	}
}

// ToolMeta implements Widget.
func (c *Confirm) ToolMeta() map[string]any {
	return uispec.ToolUIMeta{ResourceURI: c.URI}.MetaMap()
}

// config serializes what the runtime needs: the two calls, the guards it
// enforces, the detail items it renders, and the authored effects it shows
// until a tool result supplies its own. Prompt, body and guard labels are
// already in the markup.
func (c *Confirm) config() map[string]any {
	accept := map[string]any{"tool": c.Accept.Tool}
	if c.Accept.ChatPrompt != "" {
		// The chat path never calls the tool from the view, so its args would
		// be dead weight in the island.
		accept["chatPrompt"] = c.Accept.ChatPrompt
	} else if len(c.Accept.Args) > 0 {
		accept["args"] = c.Accept.Args
	}
	if c.Accept.SuccessMessage != "" {
		accept["successMessage"] = c.Accept.SuccessMessage
	}

	cfg := map[string]any{
		"widget":     "confirm",
		"rowsKey":    c.rowsKey(),
		"effectsKey": c.effectsKey(),
		"rowId":      c.rowID(),
		"accept":     accept,
	}
	if c.Reject != nil {
		reject := map[string]any{"message": c.rejectMessage()}
		if c.Reject.Tool != "" {
			reject["tool"] = c.Reject.Tool
		}
		if len(c.Reject.Args) > 0 {
			reject["args"] = c.Reject.Args
		}
		cfg["reject"] = reject
	}
	if !c.Details.empty() {
		cfg["details"] = c.Details.config()
	}
	if len(c.Effects) > 0 {
		effects := make([]map[string]any, len(c.Effects))
		for n, e := range c.Effects {
			effects[n] = e.config()
		}
		cfg["effects"] = effects
	}
	if c.TypeToConfirm != "" {
		cfg["typeToConfirm"] = c.TypeToConfirm
	}
	if c.Acknowledge != "" {
		cfg["acknowledge"] = true
	}
	if c.LoadTool != "" {
		cfg["loadTool"] = c.LoadTool
		if len(c.LoadArgs) > 0 {
			cfg["loadArgs"] = c.LoadArgs
		}
	}
	return cfg
}

func (c *Confirm) rejectMessage() string {
	if c.Reject != nil && c.Reject.Message != "" {
		return c.Reject.Message
	}
	return "Cancelled."
}
