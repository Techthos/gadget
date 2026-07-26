package gadget

import (
	"encoding/json"
	"fmt"
)

// ActionKind selects what an Action does when triggered.
type ActionKind string

const (
	// ActionTool calls an MCP tool (the zero-value default).
	ActionTool ActionKind = "tool"
	// ActionLink asks the host to open a URL taken from the row.
	ActionLink ActionKind = "link"
)

// ActionVariant selects button styling.
type ActionVariant string

const (
	VariantDefault ActionVariant = ""
	VariantPrimary ActionVariant = "primary"
	VariantDanger  ActionVariant = "danger"
)

// Action is a user-triggerable operation on a widget: a per-row button, a
// bulk action over selected rows, or a link.
//
// A tool action normally calls Tool and lets the widget handle the result —
// re-rendering from returned rows, or reporting the outcome. When the tool
// answers with a widget of its own instead (an edit form, a detail view), the
// host is the one that must open it, and a host that runs a view-initiated
// call out of band opens nothing. Prompt routes such an action through the
// chat for those hosts: see its documentation below.
type Action struct {
	Label string
	// Kind defaults to ActionTool.
	Kind ActionKind
	// Tool is the MCP tool name to call (Kind == ActionTool).
	Tool string
	// Args maps tool argument names to their sources. Ignored when Prompt is
	// set — the model chooses the arguments there.
	Args map[string]ArgSource
	// Prompt, when set, makes the action ask the host to post this text as a
	// user message (ui/message) instead of calling Tool itself. Write it as
	// the request a user would type; the model reads it and decides which
	// tool answers, so Tool documents what the action opens.
	//
	// The text is fixed: it carries no row values, because the model works
	// out which record is meant from the conversation. Only tool actions may
	// set it — a link action already navigates on its own.
	Prompt string
	// HrefKey is the row field holding the URL (Kind == ActionLink).
	HrefKey string
	// Confirm, when set, requires a second confirming click showing this
	// text before the action fires. (Rendered inline: native confirm()
	// dialogs are silently disabled in sandboxed MCP Apps iframes.)
	Confirm string
	Variant ActionVariant
}

func (a Action) kind() ActionKind {
	if a.Kind == "" {
		return ActionTool
	}
	return a.Kind
}

func (a Action) validate(context string) error {
	if a.Label == "" {
		return fmt.Errorf("%s: action label is required", context)
	}
	switch a.kind() {
	case ActionTool:
		if a.Tool == "" {
			return fmt.Errorf("%s: action %q: tool name is required", context, a.Label)
		}
	case ActionLink:
		if a.HrefKey == "" {
			return fmt.Errorf("%s: action %q: HrefKey is required for link actions", context, a.Label)
		}
		if a.Prompt != "" {
			return fmt.Errorf("%s: action %q: Prompt does not apply to link actions", context, a.Label)
		}
	default:
		return fmt.Errorf("%s: action %q: unknown kind %q", context, a.Label, a.Kind)
	}
	for name, src := range a.Args {
		if !src.valid() {
			return fmt.Errorf("%s: action %q: argument %q must be built with Static, FromRow, or FromSelection", context, a.Label, name)
		}
	}
	return nil
}

// config returns the action's JSON-island representation.
func (a Action) config() map[string]any {
	m := map[string]any{
		"label": a.Label,
		"kind":  string(a.kind()),
	}
	if a.Tool != "" {
		m["tool"] = a.Tool
	}
	if a.Prompt != "" {
		// A prompt action never calls the tool from the view, so its args
		// would be dead weight in the island.
		m["prompt"] = a.Prompt
	} else if len(a.Args) > 0 {
		m["args"] = a.Args
	}
	if a.HrefKey != "" {
		m["hrefKey"] = a.HrefKey
	}
	if a.Confirm != "" {
		m["confirm"] = a.Confirm
	}
	if a.Variant != "" {
		m["variant"] = string(a.Variant)
	}
	return m
}

func actionConfigs(actions []Action) []map[string]any {
	out := make([]map[string]any, len(actions))
	for i, a := range actions {
		out[i] = a.config()
	}
	return out
}

// ArgSource declares where a tool-call argument value comes from when an
// action fires. Construct with Static, FromRow, or FromSelection.
type ArgSource struct {
	static    any
	staticSet bool
	row       string
	selection string
}

// Static supplies a fixed value.
func Static(v any) ArgSource { return ArgSource{static: v, staticSet: true} }

// FromRow takes the value of field on the row the action was triggered on.
func FromRow(field string) ArgSource { return ArgSource{row: field} }

// FromSelection collects the values of field across all selected rows
// (bulk actions).
func FromSelection(field string) ArgSource { return ArgSource{selection: field} }

func (s ArgSource) valid() bool {
	n := 0
	if s.staticSet {
		n++
	}
	if s.row != "" {
		n++
	}
	if s.selection != "" {
		n++
	}
	return n == 1
}

// MarshalJSON emits {"static": v} | {"row": "field"} | {"selection": "field"}.
func (s ArgSource) MarshalJSON() ([]byte, error) {
	switch {
	case s.staticSet:
		return json.Marshal(map[string]any{"static": s.static})
	case s.row != "":
		return json.Marshal(map[string]string{"row": s.row})
	case s.selection != "":
		return json.Marshal(map[string]string{"selection": s.selection})
	default:
		return nil, fmt.Errorf("gadget: ArgSource must be built with Static, FromRow, or FromSelection")
	}
}
