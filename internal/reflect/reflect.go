// Package reflect provides WGSL shader reflection capabilities.
// It extracts binding information, struct layouts, and entry points
// from WGSL source code.
package reflect

import (
	"strconv"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/lexer"
	"github.com/HugoDaniel/miniray/internal/parser"
)

// ReflectResult contains all reflection information for a shader module.
type ReflectResult struct {
	Bindings    []BindingInfo          `json:"bindings"`
	Structs     map[string]StructLayout `json:"structs"`
	EntryPoints []EntryPointInfo       `json:"entryPoints"`
	Errors      []string               `json:"errors,omitempty"`
}

// BindingInfo describes a single @group/@binding variable.
type BindingInfo struct {
	Group        int           `json:"group"`
	Binding      int           `json:"binding"`
	Name         string        `json:"name"`
	NameMapped   string        `json:"nameMapped"`
	AddressSpace string        `json:"addressSpace"`
	AccessMode   string        `json:"accessMode,omitempty"`
	Type         string        `json:"type"`
	TypeMapped   string        `json:"typeMapped"`
	Layout       *StructLayout `json:"layout,omitempty"` // only for non-array struct types
	Array        *ArrayInfo    `json:"array,omitempty"`  // only for array types
}

// ArrayInfo describes array-specific information for array types.
// For nested arrays (e.g., array<array<f32, 4>, 10>), Array field contains nested info.
type ArrayInfo struct {
	Depth             int           `json:"depth"`                   // nesting depth (1 = simple array, 2+ = nested)
	ElementCount      *int          `json:"elementCount"`            // null for runtime-sized arrays
	ElementStride     int           `json:"elementStride"`           // stride in bytes (size + alignment padding)
	TotalSize         *int          `json:"totalSize"`               // elementCount * elementStride, null for runtime-sized
	ElementType       string        `json:"elementType"`             // original name: "Particle", "vec4f", "array<f32, 4>"
	ElementTypeMapped string        `json:"elementTypeMapped"`       // minified name: "a", "vec4f", "array<f32, 4>"
	ElementLayout     *StructLayout `json:"elementLayout,omitempty"` // layout if element is a struct
	Array             *ArrayInfo    `json:"array,omitempty"`         // nested array info (for array<array<...>>)
}

// StructLayout describes the memory layout of a struct.
type StructLayout struct {
	Size      int         `json:"size"`
	Alignment int         `json:"alignment"`
	Fields    []FieldInfo `json:"fields"`
}

// FieldInfo describes a single struct field.
type FieldInfo struct {
	Name       string        `json:"name"`
	NameMapped string        `json:"nameMapped"`
	Type       string        `json:"type"`
	TypeMapped string        `json:"typeMapped"`
	Offset     int           `json:"offset"`
	Size       int           `json:"size"`
	Alignment  int           `json:"alignment"`
	Layout     *StructLayout `json:"layout,omitempty"` // for nested structs
}

// EntryPointInfo describes a shader entry point function.
type EntryPointInfo struct {
	Name          string `json:"name"`
	Stage         string `json:"stage"` // "vertex", "fragment", "compute"
	WorkgroupSize []int  `json:"workgroupSize"` // null for vertex/fragment
}

// Reflect extracts binding and struct information from WGSL source.
func Reflect(source string) ReflectResult {
	// Parse the source
	p := parser.New(source)
	module, errs := p.Parse()

	if len(errs) > 0 {
		errors := make([]string, len(errs))
		for i, e := range errs {
			errors[i] = e.Message
		}
		return ReflectResult{
			Bindings:    []BindingInfo{},
			Structs:     make(map[string]StructLayout),
			EntryPoints: []EntryPointInfo{},
			Errors:      errors,
		}
	}

	return ReflectModule(module)
}

// ReflectModule extracts reflection information from a parsed module.
func ReflectModule(module *ast.Module) ReflectResult {
	return ReflectModuleWithRenamer(module, nil)
}

// ReflectModuleWithRenamer extracts reflection information from a parsed module,
// using the provided renamer for mapped names. If renamer is nil, mapped names
// will be the same as original names.
func ReflectModuleWithRenamer(module *ast.Module, renamer Renamer) ReflectResult {
	result := ReflectResult{
		Bindings:    []BindingInfo{},
		Structs:     make(map[string]StructLayout),
		EntryPoints: []EntryPointInfo{},
	}

	lc := NewLayoutComputer(module)
	if renamer != nil {
		lc.SetRenamer(renamer)
	}

	// First pass: collect all struct definitions
	for _, decl := range module.Declarations {
		if structDecl, ok := decl.(*ast.StructDecl); ok {
			name := lc.getSymbolName(structDecl.Name)
			if name != "" {
				layout := lc.computeStructLayout(structDecl)
				result.Structs[name] = *layout
			}
		}
	}

	// Second pass: collect bindings and entry points
	for _, decl := range module.Declarations {
		switch d := decl.(type) {
		case *ast.VarDecl:
			binding := extractBinding(d, module.Symbols, lc)
			if binding != nil {
				result.Bindings = append(result.Bindings, *binding)
			}

		case *ast.FunctionDecl:
			entryPoint := extractEntryPoint(d, module.Symbols)
			if entryPoint != nil {
				result.EntryPoints = append(result.EntryPoints, *entryPoint)
			}
		}
	}

	return result
}

// extractBinding extracts binding info from a VarDecl if it has @group/@binding.
func extractBinding(varDecl *ast.VarDecl, symbols []ast.Symbol, lc *LayoutComputer) *BindingInfo {
	group, binding := -1, -1

	// Parse attributes
	for _, attr := range varDecl.Attributes {
		if attr.Name == "group" && len(attr.Args) > 0 {
			group = parseIntAttr(attr.Args[0])
		}
		if attr.Name == "binding" && len(attr.Args) > 0 {
			binding = parseIntAttr(attr.Args[0])
		}
	}

	// Skip if no @group/@binding
	if group < 0 || binding < 0 {
		return nil
	}

	// Determine address space - infer "handle" for texture/sampler types
	addressSpace := varDecl.AddressSpace
	if addressSpace == ast.AddressSpaceNone {
		// Check if type is texture or sampler (handle address space)
		if isHandleType(varDecl.Type) {
			addressSpace = ast.AddressSpaceHandle
		}
	}

	name := getSymbolName(varDecl.Name, symbols)
	info := &BindingInfo{
		Group:        group,
		Binding:      binding,
		Name:         name,
		NameMapped:   lc.getMappedName(varDecl.Name),
		AddressSpace: addressSpaceToString(addressSpace),
		Type:         lc.typeToStringMapped(varDecl.Type, false),
		TypeMapped:   lc.typeToStringMapped(varDecl.Type, true),
	}

	// Add access mode for storage bindings
	if varDecl.AccessMode != ast.AccessModeNone {
		info.AccessMode = varDecl.AccessMode.String()
	}

	// Handle array types - extract array info and put struct layout inside
	if arrayType, ok := varDecl.Type.(*ast.ArrayType); ok {
		if varDecl.AddressSpace == ast.AddressSpaceUniform || varDecl.AddressSpace == ast.AddressSpaceStorage {
			info.Array = lc.extractArrayInfo(arrayType, 1)
		}
		// For array types, layout goes inside array.elementLayout, not at binding level
		return info
	}

	// Add layout for non-array struct types (uniform/storage only)
	if varDecl.AddressSpace == ast.AddressSpaceUniform || varDecl.AddressSpace == ast.AddressSpaceStorage {
		if identType, ok := varDecl.Type.(*ast.IdentType); ok && identType.Ref.IsValid() {
			if layout := lc.GetStructLayout(identType.Ref); layout != nil {
				info.Layout = layout
			}
		}
	}

	return info
}

// extractEntryPoint extracts entry point info from a FunctionDecl if it's an entry point.
func extractEntryPoint(fn *ast.FunctionDecl, symbols []ast.Symbol) *EntryPointInfo {
	var stage string
	var workgroupSize []int

	for _, attr := range fn.Attributes {
		switch attr.Name {
		case "vertex":
			stage = "vertex"
		case "fragment":
			stage = "fragment"
		case "compute":
			stage = "compute"
		case "workgroup_size":
			workgroupSize = parseWorkgroupSize(attr.Args)
		}
	}

	if stage == "" {
		return nil // Not an entry point
	}

	return &EntryPointInfo{
		Name:          getSymbolName(fn.Name, symbols),
		Stage:         stage,
		WorkgroupSize: workgroupSize,
	}
}

// parseIntAttr parses an integer attribute argument.
func parseIntAttr(expr ast.Expr) int {
	if expr == nil {
		return -1
	}

	switch e := expr.(type) {
	case *ast.LiteralExpr:
		if e.Kind == lexer.TokIntLiteral {
			val, err := strconv.Atoi(e.Value)
			if err == nil {
				return val
			}
		}
	}
	return -1
}

// parseWorkgroupSize parses @workgroup_size(x, y, z) arguments.
func parseWorkgroupSize(args []ast.Expr) []int {
	if len(args) == 0 {
		return nil
	}

	result := make([]int, 0, 3)
	for _, arg := range args {
		val := parseIntAttr(arg)
		if val < 0 {
			val = 1 // Default to 1 if we can't parse
		}
		result = append(result, val)
	}

	// Pad to 3 elements with 1s
	for len(result) < 3 {
		result = append(result, 1)
	}

	return result
}

// getSymbolName returns the original name for a symbol reference.
func getSymbolName(ref ast.Ref, symbols []ast.Symbol) string {
	if !ref.IsValid() {
		return ""
	}
	if int(ref.InnerIndex) >= len(symbols) {
		return ""
	}
	return symbols[ref.InnerIndex].OriginalName
}

// addressSpaceToString converts an address space to a string.
// Uses "handle" for textures/samplers instead of empty string.
func addressSpaceToString(as ast.AddressSpace) string {
	switch as {
	case ast.AddressSpaceHandle:
		return "handle"
	default:
		return as.String()
	}
}

// isHandleType returns true if the type is a texture or sampler type.
// These types use the "handle" address space in WGSL.
func isHandleType(t ast.Type) bool {
	if t == nil {
		return false
	}

	switch typ := t.(type) {
	case *ast.SamplerType:
		return true
	case *ast.TextureType:
		return true
	case *ast.IdentType:
		// Check for sampler types and texture types parsed as IdentType
		switch typ.Name {
		case "sampler", "sampler_comparison":
			return true
		case "texture_1d", "texture_2d", "texture_2d_array", "texture_3d",
			"texture_cube", "texture_cube_array", "texture_multisampled_2d",
			"texture_storage_1d", "texture_storage_2d", "texture_storage_2d_array",
			"texture_storage_3d", "texture_depth_2d", "texture_depth_2d_array",
			"texture_depth_cube", "texture_depth_cube_array",
			"texture_depth_multisampled_2d", "texture_external":
			return true
		}
	}
	return false
}
