package uispec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// specConstants mirrors ui/src/spec-constants.json, the shared fixture the
// TypeScript runtime reads its protocol constants from.
type specConstants struct {
	SpecVersion string            `json:"specVersion"`
	ExtensionID string            `json:"extensionId"`
	MIMEType    string            `json:"mimeType"`
	MetaKey     string            `json:"metaKey"`
	URIScheme   string            `json:"uriScheme"`
	Methods     map[string]string `json:"methods"`
}

func TestConstantsMatchSharedFixture(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "ui", "src", "spec-constants.json"))
	if err != nil {
		t.Fatalf("read shared fixture: %v", err)
	}
	var sc specConstants
	if err := json.Unmarshal(raw, &sc); err != nil {
		t.Fatalf("parse shared fixture: %v", err)
	}

	if sc.SpecVersion != SpecVersion {
		t.Errorf("specVersion: fixture %q, Go %q", sc.SpecVersion, SpecVersion)
	}
	if sc.ExtensionID != ExtensionID {
		t.Errorf("extensionId: fixture %q, Go %q", sc.ExtensionID, ExtensionID)
	}
	if sc.MIMEType != MIMEType {
		t.Errorf("mimeType: fixture %q, Go %q", sc.MIMEType, MIMEType)
	}
	if sc.MetaKey != MetaKey {
		t.Errorf("metaKey: fixture %q, Go %q", sc.MetaKey, MetaKey)
	}
	if sc.URIScheme != URIScheme {
		t.Errorf("uriScheme: fixture %q, Go %q", sc.URIScheme, URIScheme)
	}
	if len(sc.Methods) == 0 {
		t.Error("fixture methods map is empty")
	}
}

func TestToolUIMetaMap(t *testing.T) {
	m := ToolUIMeta{
		ResourceURI: "ui://demo/users",
		Visibility:  []string{VisibilityApp},
	}.MetaMap()

	got, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ui":{"resourceUri":"ui://demo/users","visibility":["app"]}}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestResourceUIMetaMap(t *testing.T) {
	border := true
	m := ResourceUIMeta{
		CSP:           &CSP{ConnectDomains: []string{"https://api.example.com"}},
		Permissions:   []string{PermissionClipboardWrite},
		PrefersBorder: &border,
	}.MetaMap()

	got, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"ui":{"csp":{"connectDomains":["https://api.example.com"]},"permissions":["clipboardWrite"],"prefersBorder":true}}`
	if string(got) != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

func TestResourceDescriptorMetaMapNilUI(t *testing.T) {
	d := ResourceDescriptor{URI: "ui://x/y", MIMEType: MIMEType}
	if m := d.MetaMap(); m != nil {
		t.Errorf("expected nil meta map, got %v", m)
	}
}

func TestMergeMeta(t *testing.T) {
	dst := map[string]any{
		"ui":    map[string]any{"resourceUri": "ui://a"},
		"other": 1,
	}
	src := map[string]any{
		"ui":  map[string]any{"visibility": []string{"app"}},
		"new": "x",
	}
	got := MergeMeta(dst, src)
	want := map[string]any{
		"ui": map[string]any{
			"resourceUri": "ui://a",
			"visibility":  []string{"app"},
		},
		"other": 1,
		"new":   "x",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMergeMetaNilDst(t *testing.T) {
	got := MergeMeta(nil, map[string]any{"k": "v"})
	if got["k"] != "v" {
		t.Errorf("got %v", got)
	}
}

func TestValidateURI(t *testing.T) {
	for _, uri := range []string{"ui://demo/users", "ui://x"} {
		if err := ValidateURI(uri); err != nil {
			t.Errorf("ValidateURI(%q) = %v, want nil", uri, err)
		}
	}
	for _, uri := range []string{"", "http://x", "ui://", "UI://x", "ui:/x"} {
		if err := ValidateURI(uri); err == nil {
			t.Errorf("ValidateURI(%q) = nil, want error", uri)
		}
	}
}
