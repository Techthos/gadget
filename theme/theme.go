// Package theme provides global styling overrides for gadget widgets.
//
// Widgets ship a two-layer design-token system as CSS custom properties
// (--gadget-*). Semantic tokens default to host-injected variables (MCP Apps
// hosts deliver theme variables via hostContext.styles.variables) with
// built-in fallbacks. A Theme overrides those defaults: its CSS() block is
// emitted after the base stylesheet, so non-empty fields win the cascade.
package theme

import (
	"fmt"
	"sort"
	"strings"
)

// Theme overrides gadget design tokens. The zero value overrides nothing.
// All fields hold raw CSS values (e.g. "#0f62fe", "0.5rem", "ui-sans-serif,
// system-ui"). Empty fields keep the host-aware defaults.
type Theme struct {
	ColorBackground  string // page/widget background
	ColorSurface     string // cards, table header, inputs
	ColorText        string
	ColorTextMuted   string
	ColorBorder      string
	ColorPrimary     string // accent: primary buttons, focused controls, links
	ColorPrimaryText string // text on primary background
	ColorDanger      string
	ColorSuccess     string
	ColorWarning     string

	FontFamily     string
	FontFamilyMono string

	RadiusS string
	RadiusM string
	RadiusL string

	// SpaceUnit is the base spacing unit all gaps/paddings derive from
	// (default 0.25rem). Increase for a roomier layout, decrease for density.
	SpaceUnit string

	// Extra adds or overrides raw custom properties. Keys must start with
	// "--gadget-". Use it for tokens without a dedicated field.
	Extra map[string]string
}

// tokenFields maps Theme fields to custom-property names in emission order.
var tokenFields = []struct {
	name  string
	value func(*Theme) string
}{
	{"--gadget-color-bg", func(t *Theme) string { return t.ColorBackground }},
	{"--gadget-color-surface", func(t *Theme) string { return t.ColorSurface }},
	{"--gadget-color-text", func(t *Theme) string { return t.ColorText }},
	{"--gadget-color-text-muted", func(t *Theme) string { return t.ColorTextMuted }},
	{"--gadget-color-border", func(t *Theme) string { return t.ColorBorder }},
	{"--gadget-color-primary", func(t *Theme) string { return t.ColorPrimary }},
	{"--gadget-color-primary-text", func(t *Theme) string { return t.ColorPrimaryText }},
	{"--gadget-color-danger", func(t *Theme) string { return t.ColorDanger }},
	{"--gadget-color-success", func(t *Theme) string { return t.ColorSuccess }},
	{"--gadget-color-warning", func(t *Theme) string { return t.ColorWarning }},
	{"--gadget-font", func(t *Theme) string { return t.FontFamily }},
	{"--gadget-font-mono", func(t *Theme) string { return t.FontFamilyMono }},
	{"--gadget-radius-s", func(t *Theme) string { return t.RadiusS }},
	{"--gadget-radius-m", func(t *Theme) string { return t.RadiusM }},
	{"--gadget-radius-l", func(t *Theme) string { return t.RadiusL }},
	{"--gadget-space-unit", func(t *Theme) string { return t.SpaceUnit }},
}

// CSS renders the theme as a ".gadget-root { ... }" declaration block, or ""
// when nothing is set. Entries that fail Validate are skipped; call Validate
// to surface them as errors.
func (t *Theme) CSS() string {
	if t == nil {
		return ""
	}
	var decls []string
	for _, f := range tokenFields {
		if v := f.value(t); v != "" && safeValue(v) {
			decls = append(decls, f.name+":"+v)
		}
	}
	for _, k := range sortedKeys(t.Extra) {
		v := t.Extra[k]
		if strings.HasPrefix(k, "--gadget-") && safeKey(k) && v != "" && safeValue(v) {
			decls = append(decls, k+":"+v)
		}
	}
	if len(decls) == 0 {
		return ""
	}
	return ".gadget-root{" + strings.Join(decls, ";") + "}"
}

// Validate reports invalid Extra keys and unsafe values that CSS() would
// silently skip.
func (t *Theme) Validate() error {
	if t == nil {
		return nil
	}
	for _, f := range tokenFields {
		if v := f.value(t); v != "" && !safeValue(v) {
			return fmt.Errorf("theme: unsafe value for %s: %q", f.name, v)
		}
	}
	for _, k := range sortedKeys(t.Extra) {
		if !strings.HasPrefix(k, "--gadget-") {
			return fmt.Errorf("theme: Extra key %q must start with --gadget-", k)
		}
		if !safeKey(k) {
			return fmt.Errorf("theme: unsafe Extra key %q", k)
		}
		if v := t.Extra[k]; v == "" || !safeValue(v) {
			return fmt.Errorf("theme: unsafe or empty value for Extra key %q: %q", k, v)
		}
	}
	return nil
}

// safeValue rejects values that could escape the declaration block or the
// enclosing <style> element. Legitimate CSS values (colors, lengths, font
// stacks, var()/light-dark() expressions) never contain these sequences.
func safeValue(v string) bool {
	return !strings.ContainsAny(v, "{};") && !strings.Contains(v, "</") && !strings.Contains(v, "<!--")
}

// safeKey rejects custom-property names with characters outside the safe set.
func safeKey(k string) bool {
	for _, r := range k {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
		default:
			return false
		}
	}
	return true
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
