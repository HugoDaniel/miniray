// Package api provides the public API for the WGSL minifier.
//
// This package is intended for programmatic use of the minifier.
// For CLI usage, see cmd/wgslmin.
package api

import (
	"github.com/HugoDaniel/miniray/internal/diagnostic"
	"github.com/HugoDaniel/miniray/internal/minifier"
	"github.com/HugoDaniel/miniray/internal/parser"
	"github.com/HugoDaniel/miniray/internal/reflect"
	"github.com/HugoDaniel/miniray/internal/validator"
)

// Re-export minifier types that are needed for MinifyAndReflect

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

	// Name is the original variable name.
	Name string `json:"name"`

	// NameMapped is the minified variable name (same as Name if not minified).
	NameMapped string `json:"nameMapped"`

	// AddressSpace is the memory address space: "uniform", "storage", "handle", or "".
	AddressSpace string `json:"addressSpace"`

	// AccessMode is the access mode for storage bindings: "read", "write", "read_write", or "".
	AccessMode string `json:"accessMode,omitempty"`

	// Type is the original type as a string (e.g., "MyStruct", "texture_2d<f32>").
	Type string `json:"type"`

	// TypeMapped is the minified type string (same as Type if not minified).
	TypeMapped string `json:"typeMapped"`

	// Layout contains the memory layout for non-array struct types.
	// Nil for textures, samplers, and array types.
	Layout *StructLayout `json:"layout,omitempty"`

	// Array contains array-specific information for array types.
	// Nil for non-array types.
	Array *ArrayInfo `json:"array,omitempty"`
}

// ArrayInfo describes array-specific information for array types.
// For nested arrays (e.g., array<array<f32, 4>, 10>), Array field contains nested info.
type ArrayInfo struct {
	// Depth is the nesting depth (1 = simple array, 2+ = nested).
	Depth int `json:"depth"`

	// ElementCount is the number of elements. Nil for runtime-sized arrays.
	ElementCount *int `json:"elementCount"`

	// ElementStride is the stride in bytes (size + alignment padding).
	ElementStride int `json:"elementStride"`

	// TotalSize is elementCount * elementStride. Nil for runtime-sized arrays.
	TotalSize *int `json:"totalSize"`

	// ElementType is the original element type name (e.g., "Particle", "vec4f").
	ElementType string `json:"elementType"`

	// ElementTypeMapped is the minified element type name.
	ElementTypeMapped string `json:"elementTypeMapped"`

	// ElementLayout contains struct layout if element is a struct type.
	ElementLayout *StructLayout `json:"elementLayout,omitempty"`

	// Array contains nested array info for array<array<...>> types.
	Array *ArrayInfo `json:"array,omitempty"`
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
	// Name is the original field name.
	Name string `json:"name"`

	// NameMapped is the minified field name (same as Name if not minified).
	NameMapped string `json:"nameMapped"`

	// Type is the original field type as a string.
	Type string `json:"type"`

	// TypeMapped is the minified field type string (same as Type if not minified).
	TypeMapped string `json:"typeMapped"`

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
			NameMapped:   b.NameMapped,
			AddressSpace: b.AddressSpace,
			AccessMode:   b.AccessMode,
			Type:         b.Type,
			TypeMapped:   b.TypeMapped,
			Layout:       convertStructLayout(b.Layout),
			Array:        convertArrayInfo(b.Array),
		}
	}
	return result
}

// convertArrayInfo converts internal array info to API type.
func convertArrayInfo(info *reflect.ArrayInfo) *ArrayInfo {
	if info == nil {
		return nil
	}
	return &ArrayInfo{
		Depth:             info.Depth,
		ElementCount:      info.ElementCount,
		ElementStride:     info.ElementStride,
		TotalSize:         info.TotalSize,
		ElementType:       info.ElementType,
		ElementTypeMapped: info.ElementTypeMapped,
		ElementLayout:     convertStructLayout(info.ElementLayout),
		Array:             convertArrayInfo(info.Array),
	}
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
			Name:       f.Name,
			NameMapped: f.NameMapped,
			Type:       f.Type,
			TypeMapped: f.TypeMapped,
			Offset:     f.Offset,
			Size:       f.Size,
			Alignment:  f.Alignment,
			Layout:     convertStructLayout(f.Layout),
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

// ----------------------------------------------------------------------------
// Combined Minify + Reflect API
// ----------------------------------------------------------------------------

// MinifyAndReflectResult contains both minification and reflection output.
// The reflection information uses the actual minified names, so the mapped
// name fields (NameMapped, TypeMapped, ElementTypeMapped) show the short
// names that appear in the minified code.
type MinifyAndReflectResult struct {
	// Minified code result
	MinifyResult

	// Reflection information with mapped names from minification
	Reflect ReflectResult
}

// MinifyAndReflect minifies the source and returns reflection info with mapped names.
// This is useful when you need both minified output and reflection data, and want
// the reflection to show the actual minified names (not just original names).
//
// Example use case: A WebGPU framework that needs to create buffer bindings
// using minified struct field names after minification.
func MinifyAndReflect(source string) MinifyAndReflectResult {
	return MinifyAndReflectWithOptions(source, MinifyOptions{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
	})
}

// MinifyAndReflectWithOptions minifies with custom options and returns reflection.
func MinifyAndReflectWithOptions(source string, opts MinifyOptions) MinifyAndReflectResult {
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

	result := m.MinifyAndReflect(source)

	// Convert errors
	errors := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		errors[i] = e.Message
	}

	apiResult := MinifyAndReflectResult{
		MinifyResult: MinifyResult{
			Code:         result.Code,
			Errors:       errors,
			OriginalSize: result.Stats.OriginalSize,
			MinifiedSize: result.Stats.MinifiedSize,
		},
		Reflect: ReflectResult{
			Bindings:    convertBindings(result.Reflect.Bindings),
			Structs:     convertStructs(result.Reflect.Structs),
			EntryPoints: convertEntryPoints(result.Reflect.EntryPoints),
			Errors:      result.Reflect.Errors,
		},
	}

	// Include source map if generated
	if result.SourceMap != nil {
		apiResult.MinifyResult.SourceMap = result.SourceMap.ToJSON()
		apiResult.MinifyResult.SourceMapDataURI = result.SourceMap.ToDataURI()
	}

	return apiResult
}

// ----------------------------------------------------------------------------
// Validation API
// ----------------------------------------------------------------------------

// ValidateOptions controls validation behavior.
type ValidateOptions struct {
	// StrictMode treats warnings as errors.
	StrictMode bool

	// DiagnosticFilters control which diagnostics are reported.
	// The map key is the diagnostic rule name (e.g., "derivative_uniformity").
	// The value is the severity: "error", "warning", "info", or "off".
	DiagnosticFilters map[string]string
}

// DiagnosticInfo represents a single validation diagnostic.
type DiagnosticInfo struct {
	// Severity is "error", "warning", "info", or "note".
	Severity string `json:"severity"`

	// Code is the error code (e.g., "E0200" for type mismatch).
	Code string `json:"code,omitempty"`

	// Message is the human-readable error message.
	Message string `json:"message"`

	// Line is the 1-based line number.
	Line int `json:"line"`

	// Column is the 1-based column number.
	Column int `json:"column"`

	// EndLine is the 1-based end line number (for ranges).
	EndLine int `json:"endLine,omitempty"`

	// EndColumn is the 1-based end column number (for ranges).
	EndColumn int `json:"endColumn,omitempty"`

	// SpecRef is a reference to the WGSL spec section.
	SpecRef string `json:"specRef,omitempty"`
}

// ValidateResult contains validation output.
type ValidateResult struct {
	// Valid is true if no errors were found.
	Valid bool `json:"valid"`

	// Diagnostics contains all validation messages.
	Diagnostics []DiagnosticInfo `json:"diagnostics"`

	// ErrorCount is the number of error-level diagnostics.
	ErrorCount int `json:"errorCount"`

	// WarningCount is the number of warning-level diagnostics.
	WarningCount int `json:"warningCount"`
}

// Validate validates WGSL source code and returns diagnostics.
// This performs full semantic validation compatible with the Dawn Tint compiler.
func Validate(source string) ValidateResult {
	return ValidateWithOptions(source, ValidateOptions{})
}

// ValidateWithOptions validates WGSL source code with custom options.
func ValidateWithOptions(source string, opts ValidateOptions) ValidateResult {
	// Parse the source
	p := parser.New(source)
	module, parseErrors := p.Parse()

	// Convert diagnostic filters
	var filters *diagnostic.DiagnosticFilter
	if len(opts.DiagnosticFilters) > 0 {
		filters = diagnostic.NewDiagnosticFilter()
		for rule, severity := range opts.DiagnosticFilters {
			switch severity {
			case "off":
				filters.DisableRule(rule)
			case "error":
				filters.SetRule(rule, diagnostic.Error)
			case "warning":
				filters.SetRule(rule, diagnostic.Warning)
			case "info":
				filters.SetRule(rule, diagnostic.Info)
			}
		}
	}

	// Initialize result
	result := ValidateResult{
		Valid:       true,
		Diagnostics: make([]DiagnosticInfo, 0),
	}

	// Add parse errors
	for _, e := range parseErrors {
		result.Diagnostics = append(result.Diagnostics, DiagnosticInfo{
			Severity: "error",
			Code:     "E0001",
			Message:  e.Message,
			Line:     e.Line,
			Column:   e.Column,
		})
		result.ErrorCount++
		result.Valid = false
	}

	// If parsing succeeded, run semantic validation
	if len(parseErrors) == 0 {
		validatorResult := validator.Validate(module, validator.Options{
			StrictMode:        opts.StrictMode,
			DiagnosticFilters: filters,
		})

		// Convert diagnostics
		for _, d := range validatorResult.Diagnostics.Diagnostics() {
			severity := "error"
			switch d.Severity {
			case diagnostic.Error:
				severity = "error"
				result.ErrorCount++
			case diagnostic.Warning:
				severity = "warning"
				result.WarningCount++
			case diagnostic.Info:
				severity = "info"
			case diagnostic.Note:
				severity = "note"
			}

			result.Diagnostics = append(result.Diagnostics, DiagnosticInfo{
				Severity:  severity,
				Code:      d.Code,
				Message:   d.Message,
				Line:      d.Range.Start.Line,
				Column:    d.Range.Start.Column,
				EndLine:   d.Range.End.Line,
				EndColumn: d.Range.End.Column,
				SpecRef:   d.SpecRef,
			})
		}

		result.Valid = !validatorResult.Diagnostics.HasErrors()
	}

	return result
}
