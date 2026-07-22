// Package assets embeds the compiled gadget runtime (JavaScript) and
// stylesheet. The files in dist/ are built from the ui/ workspace by esbuild
// and committed; run `make assets` (or go generate) after changing ui/ sources.
package assets

import _ "embed"

//go:generate npm --prefix ../../ui run build

// RuntimeJS is the bundled, minified gadget runtime (IIFE, self-mounting).
//
//go:embed dist/gadget.js
var RuntimeJS string

// StylesCSS is the bundled, minified gadget stylesheet (tokens + widgets).
//
//go:embed dist/gadget.css
var StylesCSS string
