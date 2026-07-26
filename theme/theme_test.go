package theme

import (
	"strings"
	"testing"
)

func TestZeroThemeEmitsNothing(t *testing.T) {
	var th Theme
	if got := th.CSS(); got != "" {
		t.Errorf("zero theme CSS() = %q, want empty", got)
	}
	var nilTheme *Theme
	if got := nilTheme.CSS(); got != "" {
		t.Errorf("nil theme CSS() = %q, want empty", got)
	}
}

func TestCSSEmission(t *testing.T) {
	th := Theme{
		ColorPrimary: "#0f62fe",
		RadiusM:      "0.5rem",
		FontFamily:   `"Inter", sans-serif`,
		Extra: map[string]string{
			"--gadget-table-stripe": "rgb(0 0 0 / 4%)",
			"--gadget-focus-width":  "2px",
		},
	}
	got := th.CSS()
	want := `.gadget-root{--gadget-color-primary:#0f62fe;--gadget-font:"Inter", sans-serif;--gadget-radius-m:0.5rem;--gadget-focus-width:2px;--gadget-table-stripe:rgb(0 0 0 / 4%)}`
	if got != want {
		t.Errorf("CSS():\n got %q\nwant %q", got, want)
	}
	if err := th.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
}

func TestTransparentEmission(t *testing.T) {
	th := Theme{Transparent: true}
	got := th.CSS()
	want := `:root{--gadget-page-pad:0}.gadget-root{--gadget-color-page:transparent}`
	if got != want {
		t.Errorf("CSS():\n got %q\nwant %q", got, want)
	}
	if err := th.Validate(); err != nil {
		t.Errorf("Validate() = %v, want nil", err)
	}
	// Transparent wins over the individual knobs it subsumes.
	th = Theme{Transparent: true, ColorPage: "#fff", PagePad: "2rem"}
	if got := th.CSS(); got != want {
		t.Errorf("Transparent with overrides:\n got %q\nwant %q", got, want)
	}
}

func TestPageTokensEmission(t *testing.T) {
	th := Theme{ColorPage: "#faf7f2", PagePad: "0"}
	got := th.CSS()
	want := `:root{--gadget-page-pad:0}.gadget-root{--gadget-color-page:#faf7f2}`
	if got != want {
		t.Errorf("CSS():\n got %q\nwant %q", got, want)
	}
}

func TestUnsafeValuesSkippedAndReported(t *testing.T) {
	th := Theme{
		ColorText:    "red}</style><script>alert(1)</script>",
		ColorPrimary: "#0f62fe",
	}
	got := th.CSS()
	if strings.Contains(got, "script") || strings.Contains(got, "}<") {
		t.Errorf("unsafe value leaked into CSS: %q", got)
	}
	if !strings.Contains(got, "--gadget-color-primary:#0f62fe") {
		t.Errorf("safe value missing from CSS: %q", got)
	}
	if err := th.Validate(); err == nil {
		t.Error("Validate() = nil, want error for unsafe value")
	}
}

func TestExtraKeyValidation(t *testing.T) {
	cases := map[string]Theme{
		"bad prefix":  {Extra: map[string]string{"--evil": "x"}},
		"unsafe key":  {Extra: map[string]string{"--gadget-a:b": "x"}},
		"empty value": {Extra: map[string]string{"--gadget-a": ""}},
	}
	for name, th := range cases {
		if err := th.Validate(); err == nil {
			t.Errorf("%s: Validate() = nil, want error", name)
		}
		if got := th.CSS(); got != "" {
			t.Errorf("%s: CSS() = %q, want empty", name, got)
		}
	}
}
