package reflect

import (
	"strconv"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/lexer"
)

// LayoutComputer computes memory layouts for WGSL types.
type LayoutComputer struct {
	module      *ast.Module
	structCache map[string]*StructLayout
}

// NewLayoutComputer creates a layout computer for a module.
func NewLayoutComputer(module *ast.Module) *LayoutComputer {
	return &LayoutComputer{
		module:      module,
		structCache: make(map[string]*StructLayout),
	}
}

// ComputeTypeLayout computes layout for any WGSL type.
func (lc *LayoutComputer) ComputeTypeLayout(t ast.Type) TypeLayout {
	if t == nil {
		return TypeLayout{}
	}

	switch typ := t.(type) {
	case *ast.IdentType:
		// Check primitive types first
		if layout, ok := primitiveLayouts[typ.Name]; ok {
			return layout
		}
		// Check struct cache by name (handles shadowed types where member name
		// shadows struct type name, e.g., `info: info`)
		if cached, ok := lc.structCache[typ.Name]; ok {
			return TypeLayout{
				Size:      cached.Size,
				Alignment: cached.Alignment,
			}
		}
		// Look up struct definition by ref
		if typ.Ref.IsValid() {
			if structLayout := lc.GetStructLayout(typ.Ref); structLayout != nil {
				return TypeLayout{
					Size:      structLayout.Size,
					Alignment: structLayout.Alignment,
				}
			}
		}
		return TypeLayout{}

	case *ast.VecType:
		return lc.computeVecTypeLayout(typ)

	case *ast.MatType:
		return lc.computeMatTypeLayout(typ)

	case *ast.ArrayType:
		return lc.computeArrayTypeLayout(typ)

	case *ast.AtomicType:
		// atomic<T> has same layout as T
		return lc.ComputeTypeLayout(typ.ElemType)

	case *ast.SamplerType, *ast.TextureType:
		// Samplers and textures don't have a host-addressable layout
		return TypeLayout{}

	case *ast.PtrType:
		// Pointers don't have a meaningful size for reflection
		return TypeLayout{}

	default:
		return TypeLayout{}
	}
}

// computeVecTypeLayout computes layout for a vector type.
func (lc *LayoutComputer) computeVecTypeLayout(typ *ast.VecType) TypeLayout {
	// Check for shorthand forms first
	if typ.Shorthand != "" {
		if layout, ok := primitiveLayouts[typ.Shorthand]; ok {
			return layout
		}
	}

	// Compute from element type
	elemSize := 4 // Default to f32 size
	if typ.ElemType != nil {
		elemLayout := lc.ComputeTypeLayout(typ.ElemType)
		if elemLayout.Size > 0 {
			elemSize = elemLayout.Size
		}
	}

	return computeVecLayout(int(typ.Size), elemSize)
}

// computeMatTypeLayout computes layout for a matrix type.
func (lc *LayoutComputer) computeMatTypeLayout(typ *ast.MatType) TypeLayout {
	// Check for shorthand forms first
	if typ.Shorthand != "" {
		if layout, ok := primitiveLayouts[typ.Shorthand]; ok {
			return layout
		}
	}

	// Compute from element type
	elemSize := 4 // Default to f32 size
	if typ.ElemType != nil {
		elemLayout := lc.ComputeTypeLayout(typ.ElemType)
		if elemLayout.Size > 0 {
			elemSize = elemLayout.Size
		}
	}

	return computeMatLayout(int(typ.Cols), int(typ.Rows), elemSize)
}

// computeArrayTypeLayout computes layout for an array type.
func (lc *LayoutComputer) computeArrayTypeLayout(typ *ast.ArrayType) TypeLayout {
	elem := lc.ComputeTypeLayout(typ.ElemType)
	if elem.Size == 0 || elem.Alignment == 0 {
		return TypeLayout{}
	}

	stride := roundUp(elem.Size, elem.Alignment)

	// Runtime-sized arrays have unknown size
	if typ.Size == nil {
		return TypeLayout{
			Size:      0, // Unknown at compile time
			Alignment: elem.Alignment,
			Stride:    stride,
		}
	}

	// Fixed-size arrays - try to evaluate the size expression
	count := lc.evaluateConstExpr(typ.Size)
	if count < 0 {
		return TypeLayout{
			Size:      0,
			Alignment: elem.Alignment,
			Stride:    stride,
		}
	}

	return TypeLayout{
		Size:      count * stride,
		Alignment: elem.Alignment,
		Stride:    stride,
	}
}

// evaluateConstExpr attempts to evaluate a constant expression to an integer.
// Returns -1 if the expression cannot be evaluated.
func (lc *LayoutComputer) evaluateConstExpr(expr ast.Expr) int {
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
		return -1

	case *ast.IdentExpr:
		// Could look up const values, but for now return unknown
		return -1

	default:
		return -1
	}
}

// GetStructLayout returns the layout for a struct by reference.
// Returns nil if the reference is not a struct.
func (lc *LayoutComputer) GetStructLayout(ref ast.Ref) *StructLayout {
	if !ref.IsValid() || lc.module == nil {
		return nil
	}

	// Get symbol name
	if int(ref.InnerIndex) >= len(lc.module.Symbols) {
		return nil
	}
	sym := &lc.module.Symbols[ref.InnerIndex]
	if sym.Kind != ast.SymbolStruct {
		return nil
	}

	// Check cache
	if cached, ok := lc.structCache[sym.OriginalName]; ok {
		return cached
	}

	// Find the struct declaration
	for _, decl := range lc.module.Declarations {
		if structDecl, ok := decl.(*ast.StructDecl); ok {
			if structDecl.Name == ref {
				return lc.computeStructLayout(structDecl)
			}
		}
	}

	return nil
}

// computeStructLayout computes the memory layout for a struct.
func (lc *LayoutComputer) computeStructLayout(decl *ast.StructDecl) *StructLayout {
	name := lc.getSymbolName(decl.Name)

	// Check cache
	if cached, ok := lc.structCache[name]; ok {
		return cached
	}

	// Create placeholder in cache to handle recursive types
	layout := &StructLayout{
		Fields: make([]FieldInfo, 0, len(decl.Members)),
	}
	lc.structCache[name] = layout

	var offset int
	var maxAlign int = 1 // Minimum alignment is 1

	for _, member := range decl.Members {
		memberLayout := lc.ComputeTypeLayout(member.Type)
		if memberLayout.Alignment == 0 {
			memberLayout.Alignment = 1
		}

		// Align offset to member alignment
		offset = roundUp(offset, memberLayout.Alignment)

		field := FieldInfo{
			Name:      lc.getSymbolName(member.Name),
			Type:      lc.typeToString(member.Type),
			Offset:    offset,
			Size:      memberLayout.Size,
			Alignment: memberLayout.Alignment,
		}

		// Add nested layout for struct types
		if identType, ok := member.Type.(*ast.IdentType); ok && identType.Ref.IsValid() {
			if nestedLayout := lc.GetStructLayout(identType.Ref); nestedLayout != nil {
				field.Layout = nestedLayout
			}
		}

		// Add nested layout for array of structs
		if arrayType, ok := member.Type.(*ast.ArrayType); ok {
			if identType, ok := arrayType.ElemType.(*ast.IdentType); ok && identType.Ref.IsValid() {
				if nestedLayout := lc.GetStructLayout(identType.Ref); nestedLayout != nil {
					field.Layout = nestedLayout
				}
			}
		}

		layout.Fields = append(layout.Fields, field)

		offset += memberLayout.Size
		if memberLayout.Alignment > maxAlign {
			maxAlign = memberLayout.Alignment
		}
	}

	// Struct size is rounded up to struct alignment
	layout.Alignment = maxAlign
	layout.Size = roundUp(offset, maxAlign)

	return layout
}

// getSymbolName returns the original name for a symbol reference.
func (lc *LayoutComputer) getSymbolName(ref ast.Ref) string {
	if !ref.IsValid() || lc.module == nil {
		return ""
	}
	if int(ref.InnerIndex) >= len(lc.module.Symbols) {
		return ""
	}
	return lc.module.Symbols[ref.InnerIndex].OriginalName
}

// typeToString converts an AST type to a string representation.
func (lc *LayoutComputer) typeToString(t ast.Type) string {
	if t == nil {
		return ""
	}

	switch typ := t.(type) {
	case *ast.IdentType:
		return typ.Name

	case *ast.VecType:
		if typ.Shorthand != "" {
			return typ.Shorthand
		}
		elemStr := lc.typeToString(typ.ElemType)
		return "vec" + strconv.Itoa(int(typ.Size)) + "<" + elemStr + ">"

	case *ast.MatType:
		if typ.Shorthand != "" {
			return typ.Shorthand
		}
		elemStr := lc.typeToString(typ.ElemType)
		return "mat" + strconv.Itoa(int(typ.Cols)) + "x" + strconv.Itoa(int(typ.Rows)) + "<" + elemStr + ">"

	case *ast.ArrayType:
		elemStr := lc.typeToString(typ.ElemType)
		if typ.Size != nil {
			size := lc.evaluateConstExpr(typ.Size)
			if size >= 0 {
				return "array<" + elemStr + ", " + strconv.Itoa(size) + ">"
			}
		}
		return "array<" + elemStr + ">"

	case *ast.AtomicType:
		return "atomic<" + lc.typeToString(typ.ElemType) + ">"

	case *ast.SamplerType:
		if typ.Comparison {
			return "sampler_comparison"
		}
		return "sampler"

	case *ast.TextureType:
		return lc.textureTypeToString(typ)

	case *ast.PtrType:
		return "ptr<" + typ.AddressSpace.String() + ", " + lc.typeToString(typ.ElemType) + ">"

	default:
		return ""
	}
}

// textureTypeToString converts a texture type to string.
func (lc *LayoutComputer) textureTypeToString(typ *ast.TextureType) string {
	var prefix string
	switch typ.Kind {
	case ast.TextureSampled:
		prefix = "texture"
	case ast.TextureMultisampled:
		prefix = "texture_multisampled"
	case ast.TextureStorage:
		prefix = "texture_storage"
	case ast.TextureDepth:
		prefix = "texture_depth"
	case ast.TextureDepthMultisampled:
		prefix = "texture_depth_multisampled"
	case ast.TextureExternal:
		return "texture_external"
	}

	var dim string
	switch typ.Dimension {
	case ast.Texture1D:
		dim = "_1d"
	case ast.Texture2D:
		dim = "_2d"
	case ast.Texture2DArray:
		dim = "_2d_array"
	case ast.Texture3D:
		dim = "_3d"
	case ast.TextureCube:
		dim = "_cube"
	case ast.TextureCubeArray:
		dim = "_cube_array"
	}

	result := prefix + dim

	// Add type parameter for sampled textures
	if typ.Kind == ast.TextureSampled && typ.SampledType != nil {
		result += "<" + lc.typeToString(typ.SampledType) + ">"
	}

	// Add format for storage textures
	if typ.Kind == ast.TextureStorage && typ.TexelFormat != "" {
		result += "<" + typ.TexelFormat
		if typ.AccessMode != ast.AccessModeNone {
			result += ", " + typ.AccessMode.String()
		}
		result += ">"
	}

	return result
}
