// Package main provides a C-callable static library for WGSL minification and reflection.
//
// This is built with -buildmode=c-archive to produce libminiray.a
// that can be linked into Zig/C/Rust programs.
//
// Build:
//
//	make lib
//	# or: CGO_ENABLED=1 go build -buildmode=c-archive -o build/libminiray.a ./cmd/miniray-lib
//
// Exported functions:
//
//	miniray_minify(source, source_len, options_json, options_len, out_code, out_code_len, out_json, out_json_len) -> error_code
//	miniray_reflect(source, source_len, out_json, out_len) -> error_code
//	miniray_minify_and_reflect(source, source_len, options_json, options_len, out_code, out_code_len, out_json, out_json_len) -> error_code
//	miniray_free(ptr) -> void
//	miniray_version() -> *char
package main

/*
#include <stdlib.h>
*/
import "C"
import (
	"encoding/json"
	"unsafe"

	"github.com/HugoDaniel/miniray/internal/minifier"
	"github.com/HugoDaniel/miniray/internal/reflect"
)

// Version should match the release version
const version = "0.3.0"

// Error codes
const (
	MINIRAY_OK              = 0
	MINIRAY_ERR_JSON_ENCODE = 1
	MINIRAY_ERR_NULL_INPUT  = 2
	MINIRAY_ERR_JSON_DECODE = 3
)

// MinifyOptions mirrors the Go API options for JSON parsing
type MinifyOptions struct {
	MinifyWhitespace       bool     `json:"minifyWhitespace"`
	MinifyIdentifiers      bool     `json:"minifyIdentifiers"`
	MinifySyntax           bool     `json:"minifySyntax"`
	MangleExternalBindings bool     `json:"mangleExternalBindings"`
	TreeShaking            bool     `json:"treeShaking"`
	KeepNames              []string `json:"keepNames"`
}

// MinifyResult is the JSON result structure for minification
type MinifyResult struct {
	Code         string   `json:"code"`
	Errors       []string `json:"errors,omitempty"`
	OriginalSize int      `json:"originalSize"`
	MinifiedSize int      `json:"minifiedSize"`
}

// MinifyAndReflectResult combines minification and reflection results
type MinifyAndReflectResult struct {
	Code         string               `json:"code"`
	Errors       []string             `json:"errors,omitempty"`
	OriginalSize int                  `json:"originalSize"`
	MinifiedSize int                  `json:"minifiedSize"`
	Reflect      reflect.ReflectResult `json:"reflect"`
}

// miniray_minify minifies WGSL source code.
//
// Parameters:
//   - source: pointer to WGSL source code (UTF-8)
//   - source_len: length of source in bytes
//   - options_json: pointer to JSON options (can be NULL for defaults)
//   - options_len: length of options JSON
//   - out_code: pointer to receive minified code (caller must free with miniray_free)
//   - out_code_len: pointer to receive code length
//   - out_json: pointer to receive JSON result with stats (caller must free with miniray_free)
//   - out_json_len: pointer to receive JSON length
//
// Returns:
//   - 0 on success
//   - non-zero error code on failure
//
//export miniray_minify
func miniray_minify(
	source *C.char, source_len C.int,
	options_json *C.char, options_len C.int,
	out_code **C.char, out_code_len *C.int,
	out_json **C.char, out_json_len *C.int,
) C.int {
	if source == nil || out_code == nil || out_code_len == nil {
		return MINIRAY_ERR_NULL_INPUT
	}

	goSource := C.GoStringN(source, source_len)

	// Parse options or use defaults
	opts := minifier.DefaultOptions()
	if options_json != nil && options_len > 0 {
		var jsonOpts MinifyOptions
		optStr := C.GoStringN(options_json, options_len)
		if err := json.Unmarshal([]byte(optStr), &jsonOpts); err != nil {
			return MINIRAY_ERR_JSON_DECODE
		}
		opts.MinifyWhitespace = jsonOpts.MinifyWhitespace
		opts.MinifyIdentifiers = jsonOpts.MinifyIdentifiers
		opts.MinifySyntax = jsonOpts.MinifySyntax
		opts.MangleExternalBindings = jsonOpts.MangleExternalBindings
		opts.TreeShaking = jsonOpts.TreeShaking
		opts.KeepNames = jsonOpts.KeepNames
	}

	// Run minification
	m := minifier.New(opts)
	result := m.Minify(goSource)

	// Set output code
	*out_code = C.CString(result.Code)
	*out_code_len = C.int(len(result.Code))

	// Build JSON result if requested
	if out_json != nil && out_json_len != nil {
		errors := make([]string, len(result.Errors))
		for i, e := range result.Errors {
			errors[i] = e.Message
		}
		jsonResult := MinifyResult{
			Code:         result.Code,
			Errors:       errors,
			OriginalSize: result.Stats.OriginalSize,
			MinifiedSize: result.Stats.MinifiedSize,
		}
		jsonBytes, err := json.Marshal(jsonResult)
		if err != nil {
			return MINIRAY_ERR_JSON_ENCODE
		}
		*out_json = C.CString(string(jsonBytes))
		*out_json_len = C.int(len(jsonBytes))
	}

	return MINIRAY_OK
}

// miniray_reflect performs WGSL reflection and returns JSON.
//
// Parameters:
//   - source: pointer to WGSL source code (UTF-8)
//   - source_len: length of source in bytes
//   - out_json: pointer to receive JSON result (caller must free with miniray_free)
//   - out_len: pointer to receive JSON length
//
// Returns:
//   - 0 on success
//   - non-zero error code on failure
//
//export miniray_reflect
func miniray_reflect(source *C.char, source_len C.int, out_json **C.char, out_len *C.int) C.int {
	if source == nil || out_json == nil || out_len == nil {
		return MINIRAY_ERR_NULL_INPUT
	}

	goSource := C.GoStringN(source, source_len)
	result := reflect.Reflect(goSource)

	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return MINIRAY_ERR_JSON_ENCODE
	}

	*out_json = C.CString(string(jsonBytes))
	*out_len = C.int(len(jsonBytes))

	return MINIRAY_OK
}

// miniray_minify_and_reflect minifies WGSL and returns reflection with mapped names.
//
// This is the combined API that gives you both minified code and reflection
// data where the mapped name fields (nameMapped, typeMapped, elementTypeMapped)
// contain the actual minified names.
//
// Parameters:
//   - source: pointer to WGSL source code (UTF-8)
//   - source_len: length of source in bytes
//   - options_json: pointer to JSON options (can be NULL for defaults)
//   - options_len: length of options JSON
//   - out_code: pointer to receive minified code (caller must free with miniray_free)
//   - out_code_len: pointer to receive code length
//   - out_json: pointer to receive JSON result with reflection (caller must free with miniray_free)
//   - out_json_len: pointer to receive JSON length
//
// Returns:
//   - 0 on success
//   - non-zero error code on failure
//
//export miniray_minify_and_reflect
func miniray_minify_and_reflect(
	source *C.char, source_len C.int,
	options_json *C.char, options_len C.int,
	out_code **C.char, out_code_len *C.int,
	out_json **C.char, out_json_len *C.int,
) C.int {
	if source == nil || out_code == nil || out_code_len == nil || out_json == nil || out_json_len == nil {
		return MINIRAY_ERR_NULL_INPUT
	}

	goSource := C.GoStringN(source, source_len)

	// Parse options or use defaults
	opts := minifier.DefaultOptions()
	if options_json != nil && options_len > 0 {
		var jsonOpts MinifyOptions
		optStr := C.GoStringN(options_json, options_len)
		if err := json.Unmarshal([]byte(optStr), &jsonOpts); err != nil {
			return MINIRAY_ERR_JSON_DECODE
		}
		opts.MinifyWhitespace = jsonOpts.MinifyWhitespace
		opts.MinifyIdentifiers = jsonOpts.MinifyIdentifiers
		opts.MinifySyntax = jsonOpts.MinifySyntax
		opts.MangleExternalBindings = jsonOpts.MangleExternalBindings
		opts.TreeShaking = jsonOpts.TreeShaking
		opts.KeepNames = jsonOpts.KeepNames
	}

	// Run combined minify + reflect
	m := minifier.New(opts)
	result := m.MinifyAndReflect(goSource)

	// Set output code
	*out_code = C.CString(result.Code)
	*out_code_len = C.int(len(result.Code))

	// Build JSON result
	errors := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		errors[i] = e.Message
	}
	jsonResult := MinifyAndReflectResult{
		Code:         result.Code,
		Errors:       errors,
		OriginalSize: result.Stats.OriginalSize,
		MinifiedSize: result.Stats.MinifiedSize,
		Reflect:      result.Reflect,
	}
	jsonBytes, err := json.Marshal(jsonResult)
	if err != nil {
		return MINIRAY_ERR_JSON_ENCODE
	}
	*out_json = C.CString(string(jsonBytes))
	*out_json_len = C.int(len(jsonBytes))

	return MINIRAY_OK
}

// miniray_free frees memory allocated by miniray functions.
//
// Parameters:
//   - ptr: pointer returned from miniray_minify, miniray_reflect, or miniray_minify_and_reflect
//
//export miniray_free
func miniray_free(ptr *C.char) {
	if ptr != nil {
		C.free(unsafe.Pointer(ptr))
	}
}

// miniray_version returns the library version string.
// The returned pointer is static and must NOT be freed.
//
//export miniray_version
func miniray_version() *C.char {
	return C.CString(version)
}

// Required for c-archive build mode
func main() {}
