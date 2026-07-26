// Package uispec defines constants and _meta types for the MCP Apps
// extension (io.modelcontextprotocol/ui), spec version 2026-01-26.
//
// It is deliberately free of any MCP SDK dependency: everything is expressed
// as plain Go types and map[string]any meta maps, so gomukit widgets can be
// wired into any Go MCP implementation. The official go-sdk adapter lives in
// package gosdk.
//
// Spec names that may still shift with upcoming spec revisions (the meta key,
// notification method names) are encoded here exactly once; the TypeScript
// runtime mirrors them from ui/src/spec-constants.json, and tests cross-check
// the two.
package uispec

import (
	"encoding/json"
	"fmt"
	"strings"
)

const (
	// ExtensionID is the MCP Apps extension identifier used in capability
	// negotiation.
	ExtensionID = "io.modelcontextprotocol/ui"

	// SpecVersion is the ext-apps specification version gomukit targets.
	SpecVersion = "2026-01-26"

	// MIMEType is the media type of MCP Apps HTML template resources.
	MIMEType = "text/html;profile=mcp-app"

	// MetaKey is the key under which UI metadata is nested inside _meta on
	// tools, resources, and tool results.
	MetaKey = "ui"

	// URIScheme is the URI scheme for UI template resources ("ui://...").
	URIScheme = "ui"
)

// Visibility values for ToolUIMeta.Visibility.
const (
	VisibilityModel = "model" // tool is callable by the model
	VisibilityApp   = "app"   // tool is callable from the app UI only
)

// Permission is the presence marker for a requested sandbox permission. Per
// the MCP Apps spec each requested capability is a key mapping to an empty
// object, so a *Permission serializes to {} when set and is omitted when nil.
type Permission struct{}

// Grant is the marker used to request a permission, e.g.
// Permissions{Camera: uispec.Grant}.
var Grant = &Permission{}

// Permissions declares the browser capabilities a UI resource requests. Hosts
// MAY honor these by setting the sandbox iframe's allow attribute; apps must
// not assume they are granted. Serializes to the spec object shape, e.g.
// {"camera":{},"clipboardWrite":{}}.
type Permissions struct {
	Camera         *Permission `json:"camera,omitempty"`
	Microphone     *Permission `json:"microphone,omitempty"`
	Geolocation    *Permission `json:"geolocation,omitempty"`
	ClipboardWrite *Permission `json:"clipboardWrite,omitempty"`
}

// CSP declares the external origins a UI resource needs. Hosts apply a
// fully locked-down policy by default; every domain must be predeclared.
type CSP struct {
	ConnectDomains  []string `json:"connectDomains,omitempty"`
	ResourceDomains []string `json:"resourceDomains,omitempty"`
	FrameDomains    []string `json:"frameDomains,omitempty"`
	BaseURIDomains  []string `json:"baseUriDomains,omitempty"`
}

// ResourceUIMeta is the _meta.ui payload attached to a ui:// resource.
type ResourceUIMeta struct {
	CSP           *CSP         `json:"csp,omitempty"`
	Permissions   *Permissions `json:"permissions,omitempty"`
	Domain        string       `json:"domain,omitempty"`
	PrefersBorder *bool        `json:"prefersBorder,omitempty"`
}

// ToolUIMeta is the _meta.ui payload attached to a tool, linking it to its
// UI template resource.
type ToolUIMeta struct {
	ResourceURI string   `json:"resourceUri"`
	Visibility  []string `json:"visibility,omitempty"`
}

// ResourceDescriptor carries everything needed to register a widget's
// template resource with an MCP server, independent of the SDK in use.
type ResourceDescriptor struct {
	URI         string
	Name        string
	Title       string
	Description string
	MIMEType    string // always MIMEType for gomukit widgets
	UI          *ResourceUIMeta
}

// MetaMap returns the resource _meta map: {"ui": {...}}.
func (m ResourceUIMeta) MetaMap() map[string]any { return metaMap(m) }

// MetaMap returns the tool _meta map: {"ui": {"resourceUri": ..., ...}}.
func (m ToolUIMeta) MetaMap() map[string]any { return metaMap(m) }

// MetaMap returns the descriptor's resource _meta map, or nil when the
// descriptor declares no UI metadata.
func (d ResourceDescriptor) MetaMap() map[string]any {
	if d.UI == nil {
		return nil
	}
	return d.UI.MetaMap()
}

// metaMap nests v under MetaKey, converting it to plain maps via a JSON
// round-trip so callers can treat the result as ordinary _meta data.
func metaMap(v any) map[string]any {
	b, err := json.Marshal(v)
	if err != nil {
		// The meta types marshal unconditionally; reaching this is a bug.
		panic(fmt.Sprintf("uispec: marshal meta: %v", err))
	}
	var inner map[string]any
	if err := json.Unmarshal(b, &inner); err != nil {
		panic(fmt.Sprintf("uispec: unmarshal meta: %v", err))
	}
	return map[string]any{MetaKey: inner}
}

// MergeMeta merges src into dst recursively (map values merge, everything
// else overwrites) and returns dst. A nil dst is allocated. src is not
// modified; nested maps from src are copied on merge conflicts only — callers
// must not retain and mutate src maps that end up shared in dst.
func MergeMeta(dst, src map[string]any) map[string]any {
	if dst == nil {
		dst = make(map[string]any, len(src))
	}
	for k, v := range src {
		if vm, ok := v.(map[string]any); ok {
			if dm, ok := dst[k].(map[string]any); ok {
				dst[k] = MergeMeta(dm, vm)
				continue
			}
		}
		dst[k] = v
	}
	return dst
}

// ValidateURI checks that uri is a well-formed ui:// resource URI.
func ValidateURI(uri string) error {
	const prefix = URIScheme + "://"
	if !strings.HasPrefix(uri, prefix) {
		return fmt.Errorf("uispec: URI %q must use the %s scheme", uri, prefix)
	}
	if len(uri) == len(prefix) {
		return fmt.Errorf("uispec: URI %q has an empty path", uri)
	}
	return nil
}
