// Package api provides the public API for the WGSL minifier.
//
// This package is intended for programmatic use of the minifier.
// For CLI usage, see cmd/wgslmin.
package api

import (
	"codeberg.org/saruga/wgsl-minifier/internal/minifier"
)

// MinifyOptions controls minification behavior.
type MinifyOptions struct {
	// MinifyWhitespace removes unnecessary whitespace and newlines.
	MinifyWhitespace bool

	// MinifyIdentifiers renames identifiers to shorter names.
	// Entry point names and API-facing declarations are preserved.
	MinifyIdentifiers bool

	// MinifySyntax applies syntax-level optimizations like
	// numeric literal shortening.
	MinifySyntax bool

	// MangleExternalBindings controls how uniform/storage variable names are handled.
	// When false (default), original names are preserved in declarations and short
	// aliases are used internally. This maintains compatibility with WebGPU's
	// binding reflection APIs.
	// When true, uniform/storage variables are renamed directly for smaller output,
	// but this breaks binding reflection.
	MangleExternalBindings bool

	// KeepNames specifies identifier names that should not be renamed.
	KeepNames []string
}

// MinifyResult contains the minification output.
type MinifyResult struct {
	// Code is the minified WGSL source code.
	Code string

	// Errors contains any errors encountered during minification.
	// If non-empty, Code may be incomplete or invalid.
	Errors []string

	// OriginalSize is the size of the input in bytes.
	OriginalSize int

	// MinifiedSize is the size of the output in bytes.
	MinifiedSize int
}

// Minify minifies WGSL source code with default options.
// This enables all minification: whitespace removal, identifier
// renaming, and syntax optimizations.
func Minify(source string) MinifyResult {
	return MinifyWithOptions(source, MinifyOptions{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
	})
}

// MinifyWithOptions minifies WGSL source code with custom options.
func MinifyWithOptions(source string, opts MinifyOptions) MinifyResult {
	m := minifier.New(minifier.Options{
		MinifyWhitespace:       opts.MinifyWhitespace,
		MinifyIdentifiers:      opts.MinifyIdentifiers,
		MinifySyntax:           opts.MinifySyntax,
		MangleExternalBindings: opts.MangleExternalBindings,
		KeepNames:              opts.KeepNames,
	})

	result := m.Minify(source)

	// Convert errors
	errors := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		errors[i] = e.Message
	}

	return MinifyResult{
		Code:         result.Code,
		Errors:       errors,
		OriginalSize: result.Stats.OriginalSize,
		MinifiedSize: result.Stats.MinifiedSize,
	}
}

// MinifyWhitespaceOnly removes whitespace without renaming identifiers.
// This is the safest minification option.
func MinifyWhitespaceOnly(source string) MinifyResult {
	return MinifyWithOptions(source, MinifyOptions{
		MinifyWhitespace: true,
	})
}
