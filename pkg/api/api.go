// Package api provides the public API for the WGSL minifier.
//
// This package is intended for programmatic use of the minifier.
// For CLI usage, see cmd/wgslmin.
package api

import (
	"github.com/HugoDaniel/miniray/internal/minifier"
	"github.com/HugoDaniel/miniray/internal/reflect"
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

	// SourceMap enables source map generation.
	// If true, the result will include a source map.
	SourceMap bool

	// SourceMapOptions configures source map generation.
	// Only used when SourceMap is true.
	SourceMapOptions SourceMapOptions
}

// SourceMapOptions configures source map generation.
type SourceMapOptions struct {
	// File is the name of the generated file (for the "file" field in the source map).
	File string

	// SourceName is the name of the original source file (for the "sources" array).
	SourceName string

	// IncludeSource embeds the original source code in "sourcesContent".
	// This makes the source map self-contained but increases its size.
	IncludeSource bool
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

	// SourceMap is the generated source map as a JSON string.
	// Empty if source map generation was not requested.
	SourceMap string

	// SourceMapDataURI is the source map as a data URI for inline embedding.
	// Empty if source map generation was not requested.
	SourceMapDataURI string
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
		GenerateSourceMap:      opts.SourceMap,
		SourceMapOptions: minifier.SourceMapOptions{
			File:          opts.SourceMapOptions.File,
			SourceName:    opts.SourceMapOptions.SourceName,
			IncludeSource: opts.SourceMapOptions.IncludeSource,
		},
	})

	result := m.Minify(source)

	// Convert errors
	errors := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		errors[i] = e.Message
	}

	apiResult := MinifyResult{
		Code:         result.Code,
		Errors:       errors,
		OriginalSize: result.Stats.OriginalSize,
		MinifiedSize: result.Stats.MinifiedSize,
	}

	// Include source map if generated
	if result.SourceMap != nil {
		apiResult.SourceMap = result.SourceMap.ToJSON()
		apiResult.SourceMapDataURI = result.SourceMap.ToDataURI()
	}

	return apiResult
}

// MinifyWhitespaceOnly removes whitespace without renaming identifiers.
// This is the safest minification option.
func MinifyWhitespaceOnly(source string) MinifyResult {
	return MinifyWithOptions(source, MinifyOptions{
		MinifyWhitespace: true,
	})
}

// ----------------------------------------------------------------------------
// Reflection API
// ----------------------------------------------------------------------------

// ReflectResult contains binding and struct information from a WGSL shader.
type ReflectResult struct {
	// Bindings contains all @group/@binding variable declarations.
	Bindings []BindingInfo `json:"bindings"`

	// Structs contains layout information for all struct types.
	Structs map[string]StructLayout `json:"structs"`

	// EntryPoints contains all shader entry point functions.
	EntryPoints []EntryPointInfo `json:"entryPoints"`

	// Errors contains any errors encountered during parsing.
	Errors []string `json:"errors,omitempty"`
}

// BindingInfo describes a variable with @group/@binding attributes.
type BindingInfo struct {
	// Group is the binding group index from @group(n).
	Group int `json:"group"`

	// Binding is the binding index from @binding(n).
	Binding int `json:"binding"`

	// Name is the variable name.
	Name string `json:"name"`

	// AddressSpace is the memory address space: "uniform", "storage", "handle", or "".
	AddressSpace string `json:"addressSpace"`

	// AccessMode is the access mode for storage bindings: "read", "write", "read_write", or "".
	AccessMode string `json:"accessMode,omitempty"`

	// Type is the type as a string (e.g., "MyStruct", "texture_2d<f32>").
	Type string `json:"type"`

	// Layout contains the memory layout for struct types.
	// Nil for textures and samplers.
	Layout *StructLayout `json:"layout"`
}

// StructLayout describes the memory layout of a struct type.
type StructLayout struct {
	// Size is the total size in bytes.
	Size int `json:"size"`

	// Alignment is the required alignment in bytes.
	Alignment int `json:"alignment"`

	// Fields contains layout information for each struct field.
	Fields []FieldInfo `json:"fields"`
}

// FieldInfo describes a struct field with its memory layout.
type FieldInfo struct {
	// Name is the field name.
	Name string `json:"name"`

	// Type is the field type as a string.
	Type string `json:"type"`

	// Offset is the byte offset from the start of the struct.
	Offset int `json:"offset"`

	// Size is the size in bytes.
	Size int `json:"size"`

	// Alignment is the required alignment in bytes.
	Alignment int `json:"alignment"`

	// Layout contains nested layout for struct or array-of-struct fields.
	Layout *StructLayout `json:"layout,omitempty"`
}

// EntryPointInfo describes a shader entry point function.
type EntryPointInfo struct {
	// Name is the function name.
	Name string `json:"name"`

	// Stage is the shader stage: "vertex", "fragment", or "compute".
	Stage string `json:"stage"`

	// WorkgroupSize is [x, y, z] for compute shaders, nil for others.
	WorkgroupSize []int `json:"workgroupSize"`
}

// Reflect extracts binding, struct, and entry point information from WGSL source.
// This is useful for shader introspection without minification.
func Reflect(source string) ReflectResult {
	result := reflect.Reflect(source)

	// Convert internal types to API types
	return ReflectResult{
		Bindings:    convertBindings(result.Bindings),
		Structs:     convertStructs(result.Structs),
		EntryPoints: convertEntryPoints(result.EntryPoints),
		Errors:      result.Errors,
	}
}

// convertBindings converts internal binding info to API types.
func convertBindings(bindings []reflect.BindingInfo) []BindingInfo {
	result := make([]BindingInfo, len(bindings))
	for i, b := range bindings {
		result[i] = BindingInfo{
			Group:        b.Group,
			Binding:      b.Binding,
			Name:         b.Name,
			AddressSpace: b.AddressSpace,
			AccessMode:   b.AccessMode,
			Type:         b.Type,
			Layout:       convertStructLayout(b.Layout),
		}
	}
	return result
}

// convertStructs converts internal struct layouts to API types.
func convertStructs(structs map[string]reflect.StructLayout) map[string]StructLayout {
	result := make(map[string]StructLayout, len(structs))
	for name, s := range structs {
		result[name] = StructLayout{
			Size:      s.Size,
			Alignment: s.Alignment,
			Fields:    convertFields(s.Fields),
		}
	}
	return result
}

// convertStructLayout converts a single struct layout.
func convertStructLayout(layout *reflect.StructLayout) *StructLayout {
	if layout == nil {
		return nil
	}
	return &StructLayout{
		Size:      layout.Size,
		Alignment: layout.Alignment,
		Fields:    convertFields(layout.Fields),
	}
}

// convertFields converts field info to API types.
func convertFields(fields []reflect.FieldInfo) []FieldInfo {
	result := make([]FieldInfo, len(fields))
	for i, f := range fields {
		result[i] = FieldInfo{
			Name:      f.Name,
			Type:      f.Type,
			Offset:    f.Offset,
			Size:      f.Size,
			Alignment: f.Alignment,
			Layout:    convertStructLayout(f.Layout),
		}
	}
	return result
}

// convertEntryPoints converts entry point info to API types.
func convertEntryPoints(entryPoints []reflect.EntryPointInfo) []EntryPointInfo {
	result := make([]EntryPointInfo, len(entryPoints))
	for i, ep := range entryPoints {
		result[i] = EntryPointInfo{
			Name:          ep.Name,
			Stage:         ep.Stage,
			WorkgroupSize: ep.WorkgroupSize,
		}
	}
	return result
}
