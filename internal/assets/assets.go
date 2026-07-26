// Package assets embeds the compiled gomukit runtime (JavaScript) and
// stylesheet. The files in dist/ are built from the ui/ workspace by esbuild
// and committed; run `make assets` (or go generate) after changing ui/ sources.
package assets

import _ "embed"

//go:generate npm --prefix ../../ui run build

// RuntimeJS is the bundled, minified gomukit runtime (IIFE, self-mounting).
//
//go:embed dist/gomukit.js
var RuntimeJS string

// StylesCSS is the bundled, minified gomukit stylesheet (tokens + widgets).
//
//go:embed dist/gomukit.css
var StylesCSS string
