//go:build js && wasm

// Command miniray-wasm is the WebAssembly build of the WGSL minifier.
// It exposes minification functions to JavaScript via syscall/js.
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/HugoDaniel/miniray/internal/minifier"
)

var version = "0.1.0"

// jsOptions mirrors the JavaScript options object.
type jsOptions struct {
	MinifyWhitespace       *bool    `json:"minifyWhitespace"`
	MinifyIdentifiers      *bool    `json:"minifyIdentifiers"`
	MinifySyntax           *bool    `json:"minifySyntax"`
	MangleExternalBindings *bool    `json:"mangleExternalBindings"`
	TreeShaking            *bool    `json:"treeShaking"`
	KeepNames              []string `json:"keepNames"`
}

func main() {
	// Export functions to JavaScript
	js.Global().Set("__miniray", js.ValueOf(map[string]interface{}{
		"minify":  js.FuncOf(minifyJS),
		"version": version,
	}))

	// Keep the Go runtime alive
	select {}
}

// minifyJS is the JavaScript-callable minify function.
// Signature: __miniray.minify(source: string, options?: object) => object
func minifyJS(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return makeError("minify requires at least 1 argument (source)")
	}

	source := args[0].String()

	// Parse options (default to full minification)
	opts := minifier.Options{
		MinifyWhitespace:       true,
		MinifyIdentifiers:      true,
		MinifySyntax:           true,
		MangleExternalBindings: false,
		TreeShaking:            true,
	}

	if len(args) > 1 && !args[1].IsUndefined() && !args[1].IsNull() {
		jsOpts := parseOptions(args[1])
		if jsOpts.MinifyWhitespace != nil {
			opts.MinifyWhitespace = *jsOpts.MinifyWhitespace
		}
		if jsOpts.MinifyIdentifiers != nil {
			opts.MinifyIdentifiers = *jsOpts.MinifyIdentifiers
		}
		if jsOpts.MinifySyntax != nil {
			opts.MinifySyntax = *jsOpts.MinifySyntax
		}
		if jsOpts.MangleExternalBindings != nil {
			opts.MangleExternalBindings = *jsOpts.MangleExternalBindings
		}
		if jsOpts.TreeShaking != nil {
			opts.TreeShaking = *jsOpts.TreeShaking
		}
		if jsOpts.KeepNames != nil {
			opts.KeepNames = jsOpts.KeepNames
		}
	}

	// Run minification
	m := minifier.New(opts)
	result := m.Minify(source)

	// Convert errors to JS array
	errors := make([]interface{}, len(result.Errors))
	for i, e := range result.Errors {
		errors[i] = map[string]interface{}{
			"message": e.Message,
			"line":    e.Line,
			"column":  e.Column,
		}
	}

	// Return result object
	return map[string]interface{}{
		"code":         result.Code,
		"errors":       errors,
		"originalSize": result.Stats.OriginalSize,
		"minifiedSize": result.Stats.MinifiedSize,
	}
}

// parseOptions extracts options from a JS object.
func parseOptions(jsVal js.Value) jsOptions {
	var opts jsOptions

	// Try JSON serialization first (handles complex objects better)
	jsonStr := js.Global().Get("JSON").Call("stringify", jsVal).String()
	if err := json.Unmarshal([]byte(jsonStr), &opts); err == nil {
		return opts
	}

	// Fallback to direct property access
	if v := jsVal.Get("minifyWhitespace"); !v.IsUndefined() {
		b := v.Bool()
		opts.MinifyWhitespace = &b
	}
	if v := jsVal.Get("minifyIdentifiers"); !v.IsUndefined() {
		b := v.Bool()
		opts.MinifyIdentifiers = &b
	}
	if v := jsVal.Get("minifySyntax"); !v.IsUndefined() {
		b := v.Bool()
		opts.MinifySyntax = &b
	}
	if v := jsVal.Get("mangleExternalBindings"); !v.IsUndefined() {
		b := v.Bool()
		opts.MangleExternalBindings = &b
	}
	if v := jsVal.Get("treeShaking"); !v.IsUndefined() {
		b := v.Bool()
		opts.TreeShaking = &b
	}
	if v := jsVal.Get("keepNames"); !v.IsUndefined() && v.Type() == js.TypeObject {
		length := v.Get("length").Int()
		opts.KeepNames = make([]string, length)
		for i := 0; i < length; i++ {
			opts.KeepNames[i] = v.Index(i).String()
		}
	}

	return opts
}

// makeError creates a result object with an error.
func makeError(msg string) interface{} {
	return map[string]interface{}{
		"code": "",
		"errors": []interface{}{
			map[string]interface{}{
				"message": msg,
				"line":    0,
				"column":  0,
			},
		},
		"originalSize": 0,
		"minifiedSize": 0,
	}
}
