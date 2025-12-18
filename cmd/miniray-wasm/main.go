//go:build js && wasm

// Command miniray-wasm is the WebAssembly build of the WGSL minifier.
// It exposes minification functions to JavaScript via syscall/js.
package main

import (
	"encoding/json"
	"syscall/js"

	"github.com/HugoDaniel/miniray/internal/minifier"
	"github.com/HugoDaniel/miniray/internal/reflect"
)

var version = "0.1.0"

// jsOptions mirrors the JavaScript options object.
type jsOptions struct {
	MinifyWhitespace           *bool    `json:"minifyWhitespace"`
	MinifyIdentifiers          *bool    `json:"minifyIdentifiers"`
	MinifySyntax               *bool    `json:"minifySyntax"`
	MangleExternalBindings     *bool    `json:"mangleExternalBindings"`
	TreeShaking                *bool    `json:"treeShaking"`
	PreserveUniformStructTypes *bool    `json:"preserveUniformStructTypes"`
	KeepNames                  []string `json:"keepNames"`
	SourceMap                  *bool    `json:"sourceMap"`
	SourceMapSources           *bool    `json:"sourceMapSources"`
}

func main() {
	// Export functions to JavaScript
	js.Global().Set("__miniray", js.ValueOf(map[string]interface{}{
		"minify":  js.FuncOf(minifyJS),
		"reflect": js.FuncOf(reflectJS),
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
		if jsOpts.PreserveUniformStructTypes != nil {
			opts.PreserveUniformStructTypes = *jsOpts.PreserveUniformStructTypes
		}
		if jsOpts.KeepNames != nil {
			opts.KeepNames = jsOpts.KeepNames
		}
		if jsOpts.SourceMap != nil {
			opts.GenerateSourceMap = *jsOpts.SourceMap
		}
		if jsOpts.SourceMapSources != nil {
			opts.SourceMapOptions.IncludeSource = *jsOpts.SourceMapSources
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

	// Build result object
	resultObj := map[string]interface{}{
		"code":         result.Code,
		"errors":       errors,
		"originalSize": result.Stats.OriginalSize,
		"minifiedSize": result.Stats.MinifiedSize,
	}

	// Include source map if generated
	if result.SourceMap != nil {
		resultObj["sourceMap"] = result.SourceMap.ToJSON()
	}

	return resultObj
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
	if v := jsVal.Get("preserveUniformStructTypes"); !v.IsUndefined() {
		b := v.Bool()
		opts.PreserveUniformStructTypes = &b
	}
	if v := jsVal.Get("keepNames"); !v.IsUndefined() && v.Type() == js.TypeObject {
		length := v.Get("length").Int()
		opts.KeepNames = make([]string, length)
		for i := 0; i < length; i++ {
			opts.KeepNames[i] = v.Index(i).String()
		}
	}
	if v := jsVal.Get("sourceMap"); !v.IsUndefined() {
		b := v.Bool()
		opts.SourceMap = &b
	}
	if v := jsVal.Get("sourceMapSources"); !v.IsUndefined() {
		b := v.Bool()
		opts.SourceMapSources = &b
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

// reflectJS is the JavaScript-callable reflect function.
// Signature: __miniray.reflect(source: string) => object
func reflectJS(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return makeReflectError("reflect requires 1 argument (source)")
	}

	source := args[0].String()
	result := reflect.Reflect(source)

	// Convert errors to []interface{}
	errors := make([]interface{}, len(result.Errors))
	for i, e := range result.Errors {
		errors[i] = e
	}

	// Convert to JS-friendly format
	return map[string]interface{}{
		"bindings":    convertBindingsToJS(result.Bindings),
		"structs":     convertStructsToJS(result.Structs),
		"entryPoints": convertEntryPointsToJS(result.EntryPoints),
		"errors":      errors,
	}
}

// makeReflectError creates a reflect result object with an error.
func makeReflectError(msg string) interface{} {
	return map[string]interface{}{
		"bindings":    []interface{}{},
		"structs":     map[string]interface{}{},
		"entryPoints": []interface{}{},
		"errors":      []interface{}{msg},
	}
}

// convertBindingsToJS converts bindings to JS-friendly format.
func convertBindingsToJS(bindings []reflect.BindingInfo) []interface{} {
	result := make([]interface{}, len(bindings))
	for i, b := range bindings {
		binding := map[string]interface{}{
			"group":        b.Group,
			"binding":      b.Binding,
			"name":         b.Name,
			"addressSpace": b.AddressSpace,
			"type":         b.Type,
			"layout":       nil,
		}
		if b.AccessMode != "" {
			binding["accessMode"] = b.AccessMode
		}
		if b.Layout != nil {
			binding["layout"] = convertStructLayoutToJS(b.Layout)
		}
		result[i] = binding
	}
	return result
}

// convertStructsToJS converts struct map to JS-friendly format.
func convertStructsToJS(structs map[string]reflect.StructLayout) map[string]interface{} {
	result := make(map[string]interface{}, len(structs))
	for name, s := range structs {
		result[name] = map[string]interface{}{
			"size":      s.Size,
			"alignment": s.Alignment,
			"fields":    convertFieldsToJS(s.Fields),
		}
	}
	return result
}

// convertStructLayoutToJS converts a struct layout to JS-friendly format.
func convertStructLayoutToJS(layout *reflect.StructLayout) interface{} {
	if layout == nil {
		return nil
	}
	return map[string]interface{}{
		"size":      layout.Size,
		"alignment": layout.Alignment,
		"fields":    convertFieldsToJS(layout.Fields),
	}
}

// convertFieldsToJS converts fields to JS-friendly format.
func convertFieldsToJS(fields []reflect.FieldInfo) []interface{} {
	result := make([]interface{}, len(fields))
	for i, f := range fields {
		field := map[string]interface{}{
			"name":      f.Name,
			"type":      f.Type,
			"offset":    f.Offset,
			"size":      f.Size,
			"alignment": f.Alignment,
		}
		if f.Layout != nil {
			field["layout"] = convertStructLayoutToJS(f.Layout)
		}
		result[i] = field
	}
	return result
}

// convertEntryPointsToJS converts entry points to JS-friendly format.
func convertEntryPointsToJS(entryPoints []reflect.EntryPointInfo) []interface{} {
	result := make([]interface{}, len(entryPoints))
	for i, ep := range entryPoints {
		entry := map[string]interface{}{
			"name":          ep.Name,
			"stage":         ep.Stage,
			"workgroupSize": nil,
		}
		if ep.WorkgroupSize != nil {
			// Convert []int to []interface{} for JS
			wgSize := make([]interface{}, len(ep.WorkgroupSize))
			for j, v := range ep.WorkgroupSize {
				wgSize[j] = v
			}
			entry["workgroupSize"] = wgSize
		}
		result[i] = entry
	}
	return result
}
