package gadget

import (
	"fmt"
	"strconv"

	g "maragu.dev/gomponents"
	h "maragu.dev/gomponents/html"

	"github.com/techthos/gadget/internal/assets"
	"github.com/techthos/gadget/internal/htmlx"
)

// Document implements Widget. Field structure is fully SSR'd (it is static
// config); prefill values and server-side field errors are runtime state.
func (f *Form) Document() (string, error) {
	if err := f.Validate(); err != nil {
		return "", err
	}
	var data any
	if len(f.InitialData) > 0 {
		data = f.InitialData
	}
	return htmlx.Document(htmlx.DocConfig{
		Title:     docTitle(f.Title, "Form"),
		CSS:       assets.StylesCSS,
		ThemeCSS:  f.Theme.CSS(),
		Body:      f.shell(),
		Config:    f.config(),
		Data:      data,
		RuntimeJS: assets.RuntimeJS,
	})
}

func (f *Form) shell() g.Node {
	var body []g.Node
	brand := brandNode(f.Brand)
	if f.Title != "" || brand != nil {
		toolbar := []g.Node{h.Class("gadget-toolbar"), brand}
		if f.Title != "" {
			toolbar = append(toolbar, h.H2(h.Class("gadget-title"), g.Text(f.Title)))
		}
		body = append(body, h.Div(toolbar...))
	}
	// novalidate: the runtime runs checkValidity itself and renders inline
	// errors; native validation would swallow the submit event and rely on
	// browser bubbles that hosts' sandboxed iframes may not show.
	formChildren := []g.Node{h.Class("gadget-form"), htmlx.Data("form", ""), g.Attr("novalidate")}
	for _, fd := range f.Fields {
		formChildren = append(formChildren, fieldNode(fd))
	}

	submitLabel := f.Submit.Label
	if submitLabel == "" {
		submitLabel = "Submit"
	}
	actions := []g.Node{h.Class("gadget-form-actions")}
	if f.Cancel != nil {
		cancelLabel := f.Cancel.Label
		if cancelLabel == "" {
			cancelLabel = "Cancel"
		}
		actions = append(actions, h.Button(h.Type("button"), h.Class("gadget-btn"), htmlx.Data("cancel", ""), g.Text(cancelLabel)))
	}
	// type=button, not submit: hosts sandbox the widget iframe without
	// allow-forms, which blocks native form submission outright (the submit
	// event never fires). The runtime drives submission from this click.
	actions = append(actions, h.Button(
		h.Type("button"),
		h.Class("gadget-btn gadget-btn--primary"),
		htmlx.Data("submit", ""),
		g.Text(submitLabel),
	))
	formChildren = append(formChildren, h.Div(actions...))

	body = append(body, h.Form(formChildren...), statusNode())

	return h.Div(h.Class("gadget-root"), htmlx.Data("widget", "form"),
		h.Div(append([]g.Node{h.Class("gadget-card")}, body...)...),
	)
}

func fieldNode(fd Field) g.Node {
	ft := fd.fieldType()
	id := "gadget-f-" + fd.Name

	if ft == FHidden {
		return h.Input(h.Type("hidden"), h.Name(fd.Name), h.Value(defaultString(fd.Default)))
	}

	var control g.Node
	switch ft {
	case FTextarea:
		rows := fd.Rows
		if rows <= 0 {
			rows = 3
		}
		control = h.Textarea(append(controlAttrs(fd, id),
			h.Rows(strconv.Itoa(rows)),
			g.Text(defaultString(fd.Default)),
		)...)
	case FSelect, FMultiSelect:
		attrs := controlAttrs(fd, id)
		if ft == FMultiSelect {
			attrs = append(attrs, h.Multiple())
		}
		selected := selectedSet(fd.Default)
		for _, opt := range fd.Options {
			optAttrs := []g.Node{h.Value(opt.Value), g.Text(opt.Label)}
			if selected[opt.Value] {
				optAttrs = append(optAttrs, h.Selected())
			}
			attrs = append(attrs, h.Option(optAttrs...))
		}
		control = h.Select(attrs...)
	case FCheckbox:
		attrs := append(controlAttrs(fd, id), h.Type("checkbox"))
		if b, ok := fd.Default.(bool); ok && b {
			attrs = append(attrs, h.Checked())
		}
		control = h.Input(attrs...)
	default: // text, number, date, time, readonly
		attrs := append(controlAttrs(fd, id), h.Type(inputType(ft)))
		if ft == FReadonly {
			attrs = append(attrs, h.ReadOnly())
		}
		if v := defaultString(fd.Default); v != "" {
			attrs = append(attrs, h.Value(v))
		}
		control = h.Input(attrs...)
	}

	labelText := fd.Label
	if labelText == "" {
		labelText = fd.Name
	}
	var labelChildren []g.Node
	labelChildren = append(labelChildren, h.For(id), g.Text(labelText))
	if fd.Required {
		labelChildren = append(labelChildren, h.Span(h.Class("gadget-required"), h.Aria("hidden", "true"), g.Text(" *")))
	}

	nodes := []g.Node{h.Class("gadget-field gadget-field--" + string(ft))}
	if ft == FCheckbox {
		// Checkbox: control first, label after.
		nodes = append(nodes, control, h.Label(labelChildren...))
	} else {
		nodes = append(nodes, h.Label(labelChildren...), control)
	}
	if fd.Description != "" {
		nodes = append(nodes, h.P(h.Class("gadget-field-desc"), g.Text(fd.Description)))
	}
	nodes = append(nodes, h.P(h.Class("gadget-field-error"), htmlx.Data("error-for", fd.Name), g.Attr("hidden")))
	return h.Div(nodes...)
}

// controlAttrs renders the shared attributes incl. native validation.
func controlAttrs(fd Field, id string) []g.Node {
	attrs := []g.Node{h.ID(id), h.Name(fd.Name), h.Class("gadget-input")}
	if fd.Placeholder != "" {
		attrs = append(attrs, h.Placeholder(fd.Placeholder))
	}
	if fd.Required {
		attrs = append(attrs, h.Required())
	}
	if v := fd.Validation; v != nil {
		if v.Pattern != "" {
			attrs = append(attrs, h.Pattern(v.Pattern))
		}
		if v.Min != nil {
			attrs = append(attrs, h.Min(formatFloat(*v.Min)))
		}
		if v.Max != nil {
			attrs = append(attrs, h.Max(formatFloat(*v.Max)))
		}
		if v.Step != nil {
			attrs = append(attrs, h.Step(formatFloat(*v.Step)))
		}
		if v.MinLen != nil {
			attrs = append(attrs, h.MinLength(strconv.Itoa(*v.MinLen)))
		}
		if v.MaxLen != nil {
			attrs = append(attrs, h.MaxLength(strconv.Itoa(*v.MaxLen)))
		}
	}
	return attrs
}

func inputType(ft FieldType) string {
	switch ft {
	case FNumber:
		return "number"
	case FDate:
		return "date"
	case FTime:
		return "time"
	default:
		return "text"
	}
}

func defaultString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return ""
	default:
		return fmt.Sprint(x)
	}
}

func selectedSet(v any) map[string]bool {
	out := map[string]bool{}
	switch x := v.(type) {
	case string:
		if x != "" {
			out[x] = true
		}
	case []string:
		for _, s := range x {
			out[s] = true
		}
	}
	return out
}

func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}
