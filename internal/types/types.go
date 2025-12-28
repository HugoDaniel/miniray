// Package types provides the WGSL type system for semantic validation.
//
// This implements the type system as defined in WGSL spec section 6,
// supporting type inference, type checking, and overload resolution.
package types

import (
	"fmt"
	"strings"
)

// Type represents a WGSL type.
type Type interface {
	// String returns the WGSL syntax for this type.
	String() string
	// Equals returns true if this type equals another type.
	Equals(Type) bool
	// IsConstructible returns true if values of this type can be constructed.
	IsConstructible() bool
	// IsConcrete returns true if this is not an abstract type.
	IsConcrete() bool
	// IsStorable returns true if values can be stored in memory.
	IsStorable() bool
	// IsHostShareable returns true if this type can cross the CPU/GPU boundary.
	IsHostShareable() bool
	// Size returns the size in bytes (0 for unsized types).
	Size() int
	// Align returns the alignment in bytes.
	Align() int
	// isType is a marker method.
	isType()
}

// ----------------------------------------------------------------------------
// Scalar Types
// ----------------------------------------------------------------------------

// ScalarKind represents the kind of scalar type.
type ScalarKind uint8

const (
	ScalarBool ScalarKind = iota
	ScalarI32
	ScalarU32
	ScalarF32
	ScalarF16
	ScalarAbstractInt
	ScalarAbstractFloat
)

// Scalar represents a scalar type (bool, i32, u32, f32, f16).
type Scalar struct {
	Kind ScalarKind
}

func (s *Scalar) String() string {
	switch s.Kind {
	case ScalarBool:
		return "bool"
	case ScalarI32:
		return "i32"
	case ScalarU32:
		return "u32"
	case ScalarF32:
		return "f32"
	case ScalarF16:
		return "f16"
	case ScalarAbstractInt:
		return "abstract-int"
	case ScalarAbstractFloat:
		return "abstract-float"
	default:
		return "unknown"
	}
}

func (s *Scalar) Equals(other Type) bool {
	if o, ok := other.(*Scalar); ok {
		return s.Kind == o.Kind
	}
	return false
}

func (s *Scalar) IsConstructible() bool {
	return s.IsConcrete()
}

func (s *Scalar) IsConcrete() bool {
	return s.Kind != ScalarAbstractInt && s.Kind != ScalarAbstractFloat
}

func (s *Scalar) IsStorable() bool {
	return s.IsConcrete()
}

func (s *Scalar) IsHostShareable() bool {
	// bool is NOT host-shareable in WGSL
	return s.Kind != ScalarBool && s.IsConcrete()
}

func (s *Scalar) Size() int {
	switch s.Kind {
	case ScalarBool, ScalarI32, ScalarU32, ScalarF32:
		return 4
	case ScalarF16:
		return 2
	default:
		return 0 // Abstract types have no size
	}
}

func (s *Scalar) Align() int {
	return s.Size()
}

func (s *Scalar) isType() {}

// IsNumeric returns true if this is a numeric scalar type.
func (s *Scalar) IsNumeric() bool {
	return s.Kind != ScalarBool
}

// IsInteger returns true if this is an integer type.
func (s *Scalar) IsInteger() bool {
	return s.Kind == ScalarI32 || s.Kind == ScalarU32 || s.Kind == ScalarAbstractInt
}

// IsFloat returns true if this is a floating-point type.
func (s *Scalar) IsFloat() bool {
	return s.Kind == ScalarF32 || s.Kind == ScalarF16 || s.Kind == ScalarAbstractFloat
}

// ----------------------------------------------------------------------------
// Vector Types
// ----------------------------------------------------------------------------

// Vector represents vec2<T>, vec3<T>, vec4<T>.
type Vector struct {
	Width   int // 2, 3, or 4
	Element *Scalar
}

func (v *Vector) String() string {
	return fmt.Sprintf("vec%d<%s>", v.Width, v.Element.String())
}

func (v *Vector) Equals(other Type) bool {
	if o, ok := other.(*Vector); ok {
		return v.Width == o.Width && v.Element.Equals(o.Element)
	}
	return false
}

func (v *Vector) IsConstructible() bool {
	return v.Element.IsConstructible()
}

func (v *Vector) IsConcrete() bool {
	return v.Element.IsConcrete()
}

func (v *Vector) IsStorable() bool {
	return v.Element.IsStorable()
}

func (v *Vector) IsHostShareable() bool {
	return v.Element.IsHostShareable()
}

func (v *Vector) Size() int {
	return v.Element.Size() * v.Width
}

func (v *Vector) Align() int {
	// vec2 aligns to 2*element, vec3 and vec4 align to 4*element
	if v.Width == 2 {
		return v.Element.Size() * 2
	}
	return v.Element.Size() * 4
}

func (v *Vector) isType() {}

// ----------------------------------------------------------------------------
// Matrix Types
// ----------------------------------------------------------------------------

// Matrix represents matCxR<T>.
type Matrix struct {
	Cols    int // 2, 3, or 4
	Rows    int // 2, 3, or 4
	Element *Scalar
}

func (m *Matrix) String() string {
	return fmt.Sprintf("mat%dx%d<%s>", m.Cols, m.Rows, m.Element.String())
}

func (m *Matrix) Equals(other Type) bool {
	if o, ok := other.(*Matrix); ok {
		return m.Cols == o.Cols && m.Rows == o.Rows && m.Element.Equals(o.Element)
	}
	return false
}

func (m *Matrix) IsConstructible() bool {
	return m.Element.IsConstructible()
}

func (m *Matrix) IsConcrete() bool {
	return m.Element.IsConcrete()
}

func (m *Matrix) IsStorable() bool {
	return m.Element.IsStorable()
}

func (m *Matrix) IsHostShareable() bool {
	return m.Element.IsHostShareable()
}

func (m *Matrix) Size() int {
	// Each column is a vector
	colVec := &Vector{Width: m.Rows, Element: m.Element}
	return colVec.Align() * m.Cols
}

func (m *Matrix) Align() int {
	// Matrix aligns to column vector alignment
	colVec := &Vector{Width: m.Rows, Element: m.Element}
	return colVec.Align()
}

func (m *Matrix) isType() {}

// ----------------------------------------------------------------------------
// Array Types
// ----------------------------------------------------------------------------

// Array represents array<T, N> or array<T> (runtime-sized).
type Array struct {
	Element Type
	Count   int // 0 for runtime-sized arrays
}

func (a *Array) String() string {
	if a.Count == 0 {
		return fmt.Sprintf("array<%s>", a.Element.String())
	}
	return fmt.Sprintf("array<%s, %d>", a.Element.String(), a.Count)
}

func (a *Array) Equals(other Type) bool {
	if o, ok := other.(*Array); ok {
		return a.Count == o.Count && a.Element.Equals(o.Element)
	}
	return false
}

func (a *Array) IsConstructible() bool {
	return a.Count > 0 && a.Element.IsConstructible()
}

func (a *Array) IsConcrete() bool {
	return a.Element.IsConcrete()
}

func (a *Array) IsStorable() bool {
	return a.Element.IsStorable()
}

func (a *Array) IsHostShareable() bool {
	return a.Element.IsHostShareable()
}

func (a *Array) Size() int {
	if a.Count == 0 {
		return 0 // Runtime-sized
	}
	// Array stride is element size rounded up to element alignment
	elemSize := a.Element.Size()
	elemAlign := a.Element.Align()
	stride := (elemSize + elemAlign - 1) / elemAlign * elemAlign
	return stride * a.Count
}

func (a *Array) Align() int {
	return a.Element.Align()
}

func (a *Array) isType() {}

// IsRuntimeSized returns true if this is a runtime-sized array.
func (a *Array) IsRuntimeSized() bool {
	return a.Count == 0
}

// ----------------------------------------------------------------------------
// Struct Types
// ----------------------------------------------------------------------------

// StructField represents a struct member.
type StructField struct {
	Name   string
	Type   Type
	Offset int // Computed during type resolution
}

// Struct represents a user-defined struct type.
type Struct struct {
	Name    string
	Fields  []StructField
	size    int // Cached size
	align   int // Cached alignment
	hasRuntimeArray bool
}

func (s *Struct) String() string {
	return s.Name
}

func (s *Struct) Equals(other Type) bool {
	// Struct types are compared by name (nominal typing)
	if o, ok := other.(*Struct); ok {
		return s.Name == o.Name
	}
	return false
}

func (s *Struct) IsConstructible() bool {
	// Struct is constructible if all fields are constructible
	// and it has no runtime-sized array
	if s.hasRuntimeArray {
		return false
	}
	for _, f := range s.Fields {
		if !f.Type.IsConstructible() {
			return false
		}
	}
	return true
}

func (s *Struct) IsConcrete() bool {
	for _, f := range s.Fields {
		if !f.Type.IsConcrete() {
			return false
		}
	}
	return true
}

func (s *Struct) IsStorable() bool {
	for _, f := range s.Fields {
		if !f.Type.IsStorable() {
			return false
		}
	}
	return true
}

func (s *Struct) IsHostShareable() bool {
	for _, f := range s.Fields {
		if !f.Type.IsHostShareable() {
			return false
		}
	}
	return true
}

func (s *Struct) Size() int {
	return s.size
}

func (s *Struct) Align() int {
	return s.align
}

func (s *Struct) isType() {}

// ComputeLayout computes field offsets, struct size, and alignment.
func (s *Struct) ComputeLayout() {
	offset := 0
	maxAlign := 1

	for i := range s.Fields {
		f := &s.Fields[i]
		fieldAlign := f.Type.Align()
		if fieldAlign > maxAlign {
			maxAlign = fieldAlign
		}

		// Align the offset
		offset = (offset + fieldAlign - 1) / fieldAlign * fieldAlign
		f.Offset = offset

		// Check for runtime-sized array (only valid as last field)
		if arr, ok := f.Type.(*Array); ok && arr.IsRuntimeSized() {
			s.hasRuntimeArray = true
		}

		offset += f.Type.Size()
	}

	// Struct size is rounded up to alignment
	s.align = maxAlign
	s.size = (offset + maxAlign - 1) / maxAlign * maxAlign
}

// GetField returns the field with the given name, or nil.
func (s *Struct) GetField(name string) *StructField {
	for i := range s.Fields {
		if s.Fields[i].Name == name {
			return &s.Fields[i]
		}
	}
	return nil
}

// ----------------------------------------------------------------------------
// Pointer Types
// ----------------------------------------------------------------------------

// AddressSpace represents WGSL address spaces.
type AddressSpace uint8

const (
	AddressSpaceNone AddressSpace = iota
	AddressSpaceFunction
	AddressSpacePrivate
	AddressSpaceWorkgroup
	AddressSpaceUniform
	AddressSpaceStorage
	AddressSpaceHandle
)

func (a AddressSpace) String() string {
	switch a {
	case AddressSpaceFunction:
		return "function"
	case AddressSpacePrivate:
		return "private"
	case AddressSpaceWorkgroup:
		return "workgroup"
	case AddressSpaceUniform:
		return "uniform"
	case AddressSpaceStorage:
		return "storage"
	case AddressSpaceHandle:
		return "handle"
	default:
		return ""
	}
}

// AccessMode represents WGSL access modes.
type AccessMode uint8

const (
	AccessModeNone AccessMode = iota
	AccessModeRead
	AccessModeWrite
	AccessModeReadWrite
)

func (a AccessMode) String() string {
	switch a {
	case AccessModeRead:
		return "read"
	case AccessModeWrite:
		return "write"
	case AccessModeReadWrite:
		return "read_write"
	default:
		return ""
	}
}

// Pointer represents ptr<space, T, access>.
type Pointer struct {
	AddressSpace AddressSpace
	Element      Type
	AccessMode   AccessMode
}

func (p *Pointer) String() string {
	if p.AccessMode != AccessModeNone {
		return fmt.Sprintf("ptr<%s, %s, %s>", p.AddressSpace, p.Element.String(), p.AccessMode)
	}
	return fmt.Sprintf("ptr<%s, %s>", p.AddressSpace, p.Element.String())
}

func (p *Pointer) Equals(other Type) bool {
	if o, ok := other.(*Pointer); ok {
		return p.AddressSpace == o.AddressSpace &&
			p.AccessMode == o.AccessMode &&
			p.Element.Equals(o.Element)
	}
	return false
}

func (p *Pointer) IsConstructible() bool {
	return false // Pointers are not constructible
}

func (p *Pointer) IsConcrete() bool {
	return true
}

func (p *Pointer) IsStorable() bool {
	return false // Pointers cannot be stored
}

func (p *Pointer) IsHostShareable() bool {
	return false
}

func (p *Pointer) Size() int {
	return 0 // Pointers have no defined size
}

func (p *Pointer) Align() int {
	return 0
}

func (p *Pointer) isType() {}

// Reference represents a reference type (implicit pointer).
type Reference struct {
	AddressSpace AddressSpace
	Element      Type
	AccessMode   AccessMode
}

func (r *Reference) String() string {
	return fmt.Sprintf("ref<%s, %s, %s>", r.AddressSpace, r.Element.String(), r.AccessMode)
}

func (r *Reference) Equals(other Type) bool {
	if o, ok := other.(*Reference); ok {
		return r.AddressSpace == o.AddressSpace &&
			r.AccessMode == o.AccessMode &&
			r.Element.Equals(o.Element)
	}
	return false
}

func (r *Reference) IsConstructible() bool { return false }
func (r *Reference) IsConcrete() bool      { return true }
func (r *Reference) IsStorable() bool      { return false }
func (r *Reference) IsHostShareable() bool { return false }
func (r *Reference) Size() int             { return 0 }
func (r *Reference) Align() int            { return 0 }
func (r *Reference) isType()               {}

// ----------------------------------------------------------------------------
// Atomic Types
// ----------------------------------------------------------------------------

// Atomic represents atomic<T>.
type Atomic struct {
	Element *Scalar // Must be i32 or u32
}

func (a *Atomic) String() string {
	return fmt.Sprintf("atomic<%s>", a.Element.String())
}

func (a *Atomic) Equals(other Type) bool {
	if o, ok := other.(*Atomic); ok {
		return a.Element.Equals(o.Element)
	}
	return false
}

func (a *Atomic) IsConstructible() bool { return false }
func (a *Atomic) IsConcrete() bool      { return true }
func (a *Atomic) IsStorable() bool      { return true }
func (a *Atomic) IsHostShareable() bool { return true }
func (a *Atomic) Size() int             { return a.Element.Size() }
func (a *Atomic) Align() int            { return a.Element.Align() }
func (a *Atomic) isType()               {}

// ----------------------------------------------------------------------------
// Sampler Types
// ----------------------------------------------------------------------------

// Sampler represents sampler or sampler_comparison.
type Sampler struct {
	Comparison bool
}

func (s *Sampler) String() string {
	if s.Comparison {
		return "sampler_comparison"
	}
	return "sampler"
}

func (s *Sampler) Equals(other Type) bool {
	if o, ok := other.(*Sampler); ok {
		return s.Comparison == o.Comparison
	}
	return false
}

func (s *Sampler) IsConstructible() bool { return false }
func (s *Sampler) IsConcrete() bool      { return true }
func (s *Sampler) IsStorable() bool      { return false }
func (s *Sampler) IsHostShareable() bool { return false }
func (s *Sampler) Size() int             { return 0 }
func (s *Sampler) Align() int            { return 0 }
func (s *Sampler) isType()               {}

// ----------------------------------------------------------------------------
// Texture Types
// ----------------------------------------------------------------------------

// TextureKind indicates the texture category.
type TextureKind uint8

const (
	TextureSampled TextureKind = iota
	TextureMultisampled
	TextureStorage
	TextureDepth
	TextureDepthMultisampled
	TextureExternal
)

// TextureDimension indicates texture dimensionality.
type TextureDimension uint8

const (
	Texture1D TextureDimension = iota
	Texture2D
	Texture2DArray
	Texture3D
	TextureCube
	TextureCubeArray
)

func (d TextureDimension) String() string {
	switch d {
	case Texture1D:
		return "1d"
	case Texture2D:
		return "2d"
	case Texture2DArray:
		return "2d_array"
	case Texture3D:
		return "3d"
	case TextureCube:
		return "cube"
	case TextureCubeArray:
		return "cube_array"
	default:
		return ""
	}
}

// Texture represents texture types.
type Texture struct {
	Kind        TextureKind
	Dimension   TextureDimension
	SampledType *Scalar // For sampled textures
	TexelFormat string  // For storage textures
	AccessMode  AccessMode
}

func (t *Texture) String() string {
	var parts []string

	switch t.Kind {
	case TextureSampled:
		parts = append(parts, "texture_"+t.Dimension.String())
		if t.SampledType != nil {
			parts[0] = parts[0] + "<" + t.SampledType.String() + ">"
		}
	case TextureMultisampled:
		parts = append(parts, "texture_multisampled_2d")
		if t.SampledType != nil {
			parts[0] = parts[0] + "<" + t.SampledType.String() + ">"
		}
	case TextureStorage:
		parts = append(parts, fmt.Sprintf("texture_storage_%s<%s, %s>",
			t.Dimension, t.TexelFormat, t.AccessMode))
	case TextureDepth:
		parts = append(parts, "texture_depth_"+t.Dimension.String())
	case TextureDepthMultisampled:
		parts = append(parts, "texture_depth_multisampled_2d")
	case TextureExternal:
		parts = append(parts, "texture_external")
	}

	return strings.Join(parts, "")
}

func (t *Texture) Equals(other Type) bool {
	if o, ok := other.(*Texture); ok {
		if t.Kind != o.Kind || t.Dimension != o.Dimension {
			return false
		}
		if t.SampledType != nil && o.SampledType != nil {
			if !t.SampledType.Equals(o.SampledType) {
				return false
			}
		}
		if t.TexelFormat != o.TexelFormat {
			return false
		}
		return t.AccessMode == o.AccessMode
	}
	return false
}

func (t *Texture) IsConstructible() bool { return false }
func (t *Texture) IsConcrete() bool      { return true }
func (t *Texture) IsStorable() bool      { return false }
func (t *Texture) IsHostShareable() bool { return false }
func (t *Texture) Size() int             { return 0 }
func (t *Texture) Align() int            { return 0 }
func (t *Texture) isType()               {}

// ----------------------------------------------------------------------------
// Function Types
// ----------------------------------------------------------------------------

// Function represents a function type signature.
type Function struct {
	Parameters []Type
	ReturnType Type // nil for void
}

func (f *Function) String() string {
	var params []string
	for _, p := range f.Parameters {
		params = append(params, p.String())
	}
	if f.ReturnType != nil {
		return fmt.Sprintf("fn(%s) -> %s", strings.Join(params, ", "), f.ReturnType.String())
	}
	return fmt.Sprintf("fn(%s)", strings.Join(params, ", "))
}

func (f *Function) Equals(other Type) bool {
	if o, ok := other.(*Function); ok {
		if len(f.Parameters) != len(o.Parameters) {
			return false
		}
		for i := range f.Parameters {
			if !f.Parameters[i].Equals(o.Parameters[i]) {
				return false
			}
		}
		if f.ReturnType == nil && o.ReturnType == nil {
			return true
		}
		if f.ReturnType == nil || o.ReturnType == nil {
			return false
		}
		return f.ReturnType.Equals(o.ReturnType)
	}
	return false
}

func (f *Function) IsConstructible() bool { return false }
func (f *Function) IsConcrete() bool      { return true }
func (f *Function) IsStorable() bool      { return false }
func (f *Function) IsHostShareable() bool { return false }
func (f *Function) Size() int             { return 0 }
func (f *Function) Align() int            { return 0 }
func (f *Function) isType()               {}

// ----------------------------------------------------------------------------
// Void Type (for functions with no return)
// ----------------------------------------------------------------------------

// Void represents the absence of a return type.
type Void struct{}

func (v *Void) String() string             { return "void" }
func (v *Void) Equals(other Type) bool     { _, ok := other.(*Void); return ok }
func (v *Void) IsConstructible() bool      { return false }
func (v *Void) IsConcrete() bool           { return true }
func (v *Void) IsStorable() bool           { return false }
func (v *Void) IsHostShareable() bool      { return false }
func (v *Void) Size() int                  { return 0 }
func (v *Void) Align() int                 { return 0 }
func (v *Void) isType()                    {}

// ----------------------------------------------------------------------------
// Singleton Type Instances
// ----------------------------------------------------------------------------

var (
	Bool          = &Scalar{Kind: ScalarBool}
	I32           = &Scalar{Kind: ScalarI32}
	U32           = &Scalar{Kind: ScalarU32}
	F32           = &Scalar{Kind: ScalarF32}
	F16           = &Scalar{Kind: ScalarF16}
	AbstractInt   = &Scalar{Kind: ScalarAbstractInt}
	AbstractFloat = &Scalar{Kind: ScalarAbstractFloat}
	VoidType      = &Void{}
)

// Vec creates a vector type.
func Vec(size int, elem *Scalar) *Vector {
	return &Vector{Width: size, Element: elem}
}

// Mat creates a matrix type.
func Mat(cols, rows int, elem *Scalar) *Matrix {
	return &Matrix{Cols: cols, Rows: rows, Element: elem}
}

// Arr creates an array type.
func Arr(elem Type, count int) *Array {
	return &Array{Element: elem, Count: count}
}

// RuntimeArray creates a runtime-sized array type.
func RuntimeArray(elem Type) *Array {
	return &Array{Element: elem, Count: 0}
}

// Ptr creates a pointer type.
func Ptr(space AddressSpace, elem Type, access AccessMode) *Pointer {
	return &Pointer{AddressSpace: space, Element: elem, AccessMode: access}
}

// Ref creates a reference type.
func Ref(space AddressSpace, elem Type, access AccessMode) *Reference {
	return &Reference{AddressSpace: space, Element: elem, AccessMode: access}
}

// AtomicType creates an atomic type.
func AtomicType(elem *Scalar) *Atomic {
	return &Atomic{Element: elem}
}

// SamplerType creates a sampler type.
func SamplerType(comparison bool) *Sampler {
	return &Sampler{Comparison: comparison}
}

// ----------------------------------------------------------------------------
// Type Utilities
// ----------------------------------------------------------------------------

// IsScalar returns true if t is a scalar type.
func IsScalar(t Type) bool {
	_, ok := t.(*Scalar)
	return ok
}

// IsVector returns true if t is a vector type.
func IsVector(t Type) bool {
	_, ok := t.(*Vector)
	return ok
}

// IsMatrix returns true if t is a matrix type.
func IsMatrix(t Type) bool {
	_, ok := t.(*Matrix)
	return ok
}

// IsArray returns true if t is an array type.
func IsArray(t Type) bool {
	_, ok := t.(*Array)
	return ok
}

// IsStruct returns true if t is a struct type.
func IsStruct(t Type) bool {
	_, ok := t.(*Struct)
	return ok
}

// IsPointer returns true if t is a pointer type.
func IsPointer(t Type) bool {
	_, ok := t.(*Pointer)
	return ok
}

// IsReference returns true if t is a reference type.
func IsReference(t Type) bool {
	_, ok := t.(*Reference)
	return ok
}

// IsTexture returns true if t is a texture type.
func IsTexture(t Type) bool {
	_, ok := t.(*Texture)
	return ok
}

// IsSampler returns true if t is a sampler type.
func IsSampler(t Type) bool {
	_, ok := t.(*Sampler)
	return ok
}

// IsNumeric returns true if t is a numeric type (scalar or vector of numeric).
func IsNumeric(t Type) bool {
	if s, ok := t.(*Scalar); ok {
		return s.IsNumeric()
	}
	if v, ok := t.(*Vector); ok {
		return v.Element.IsNumeric()
	}
	return false
}

// IsInteger returns true if t is an integer type (scalar or vector of integer).
func IsInteger(t Type) bool {
	if s, ok := t.(*Scalar); ok {
		return s.IsInteger()
	}
	if v, ok := t.(*Vector); ok {
		return v.Element.IsInteger()
	}
	return false
}

// IsFloat returns true if t is a floating-point type.
func IsFloat(t Type) bool {
	if s, ok := t.(*Scalar); ok {
		return s.IsFloat()
	}
	if v, ok := t.(*Vector); ok {
		return v.Element.IsFloat()
	}
	return false
}

// ElementType returns the element type of composite types, or nil.
func ElementType(t Type) Type {
	switch v := t.(type) {
	case *Vector:
		return v.Element
	case *Matrix:
		return &Vector{Width: v.Rows, Element: v.Element}
	case *Array:
		return v.Element
	case *Pointer:
		return v.Element
	case *Reference:
		return v.Element
	case *Atomic:
		return v.Element
	default:
		return nil
	}
}

// CanConvertTo returns true if src can be implicitly converted to dst.
func CanConvertTo(src, dst Type) bool {
	// Same type is always ok
	if src.Equals(dst) {
		return true
	}

	// Abstract types can convert to concrete types
	if srcScalar, ok := src.(*Scalar); ok {
		if dstScalar, ok := dst.(*Scalar); ok {
			// AbstractInt -> i32, u32, f32, f16, AbstractFloat
			if srcScalar.Kind == ScalarAbstractInt {
				return dstScalar.Kind == ScalarI32 ||
					dstScalar.Kind == ScalarU32 ||
					dstScalar.Kind == ScalarF32 ||
					dstScalar.Kind == ScalarF16 ||
					dstScalar.Kind == ScalarAbstractFloat
			}
			// AbstractFloat -> f32, f16
			if srcScalar.Kind == ScalarAbstractFloat {
				return dstScalar.Kind == ScalarF32 || dstScalar.Kind == ScalarF16
			}
		}
	}

	// Vector of abstract can convert to vector of concrete
	if srcVec, ok := src.(*Vector); ok {
		if dstVec, ok := dst.(*Vector); ok {
			if srcVec.Width == dstVec.Width {
				return CanConvertTo(srcVec.Element, dstVec.Element)
			}
		}
	}

	// Matrix of abstract can convert to matrix of concrete
	if srcMat, ok := src.(*Matrix); ok {
		if dstMat, ok := dst.(*Matrix); ok {
			if srcMat.Cols == dstMat.Cols && srcMat.Rows == dstMat.Rows {
				return CanConvertTo(srcMat.Element, dstMat.Element)
			}
		}
	}

	return false
}

// CommonType returns the common type of two types for binary operations.
func CommonType(a, b Type) Type {
	if a.Equals(b) {
		return a
	}

	// If one can convert to the other, use the target
	if CanConvertTo(a, b) {
		return b
	}
	if CanConvertTo(b, a) {
		return a
	}

	// Both abstract - prefer AbstractFloat over AbstractInt
	if aScalar, ok := a.(*Scalar); ok {
		if bScalar, ok := b.(*Scalar); ok {
			if aScalar.Kind == ScalarAbstractInt && bScalar.Kind == ScalarAbstractFloat {
				return b
			}
			if bScalar.Kind == ScalarAbstractInt && aScalar.Kind == ScalarAbstractFloat {
				return a
			}
		}
	}

	return nil // No common type
}

// MultiplyResultType returns the result type of a * b, or nil if invalid.
// This handles all WGSL multiplication cases:
//   - scalar * scalar → scalar
//   - vector * vector → vector (component-wise)
//   - matrix * matrix → matrix
//   - scalar * vector → vector
//   - vector * scalar → vector
//   - scalar * matrix → matrix
//   - matrix * scalar → matrix
//   - matrix * vector → vector (matrix-vector multiplication)
//   - vector * matrix → vector (vector-matrix multiplication)
func MultiplyResultType(left, right Type) Type {
	// Try common type first (handles scalar*scalar, vec*vec, mat*mat with same types)
	if common := CommonType(left, right); common != nil {
		return common
	}

	// Get concrete types for abstract handling
	leftConc := ConcreteType(left)
	rightConc := ConcreteType(right)

	// Matrix * Vector
	if mat, ok := leftConc.(*Matrix); ok {
		if vec, ok := rightConc.(*Vector); ok {
			// mat<C,R> * vec<C> → vec<R>
			if mat.Cols == vec.Width {
				elem := commonScalarType(mat.Element, vec.Element)
				if elem != nil {
					return &Vector{Width: mat.Rows, Element: elem}
				}
			}
		}
	}

	// Vector * Matrix
	if vec, ok := leftConc.(*Vector); ok {
		if mat, ok := rightConc.(*Matrix); ok {
			// vec<R> * mat<C,R> → vec<C>
			if vec.Width == mat.Rows {
				elem := commonScalarType(vec.Element, mat.Element)
				if elem != nil {
					return &Vector{Width: mat.Cols, Element: elem}
				}
			}
		}
	}

	// Scalar * Vector or Vector * Scalar
	if IsScalar(leftConc) && IsVector(rightConc) {
		vec := rightConc.(*Vector)
		leftScalar := leftConc.(*Scalar)
		elem := commonScalarType(leftScalar, vec.Element)
		if elem != nil {
			return &Vector{Width: vec.Width, Element: elem}
		}
	}
	if IsVector(leftConc) && IsScalar(rightConc) {
		vec := leftConc.(*Vector)
		rightScalar := rightConc.(*Scalar)
		elem := commonScalarType(vec.Element, rightScalar)
		if elem != nil {
			return &Vector{Width: vec.Width, Element: elem}
		}
	}

	// Scalar * Matrix or Matrix * Scalar
	if IsScalar(leftConc) && IsMatrix(rightConc) {
		mat := rightConc.(*Matrix)
		leftScalar := leftConc.(*Scalar)
		elem := commonScalarType(leftScalar, mat.Element)
		if elem != nil {
			return &Matrix{Cols: mat.Cols, Rows: mat.Rows, Element: elem}
		}
	}
	if IsMatrix(leftConc) && IsScalar(rightConc) {
		mat := leftConc.(*Matrix)
		rightScalar := rightConc.(*Scalar)
		elem := commonScalarType(mat.Element, rightScalar)
		if elem != nil {
			return &Matrix{Cols: mat.Cols, Rows: mat.Rows, Element: elem}
		}
	}

	return nil
}

// AddSubResultType returns the result type of a +/- b, or nil if invalid.
// Addition and subtraction require matching types or vector/matrix operations.
func AddSubResultType(left, right Type) Type {
	// Common type handles most cases
	if common := CommonType(left, right); common != nil {
		return common
	}

	// Get concrete types
	leftConc := ConcreteType(left)
	rightConc := ConcreteType(right)

	// Vector +/- Vector with compatible element types
	if leftVec, ok := leftConc.(*Vector); ok {
		if rightVec, ok := rightConc.(*Vector); ok {
			if leftVec.Width == rightVec.Width {
				elem := commonScalarType(leftVec.Element, rightVec.Element)
				if elem != nil {
					return &Vector{Width: leftVec.Width, Element: elem}
				}
			}
		}
	}

	// Matrix +/- Matrix with compatible element types
	if leftMat, ok := leftConc.(*Matrix); ok {
		if rightMat, ok := rightConc.(*Matrix); ok {
			if leftMat.Cols == rightMat.Cols && leftMat.Rows == rightMat.Rows {
				elem := commonScalarType(leftMat.Element, rightMat.Element)
				if elem != nil {
					return &Matrix{Cols: leftMat.Cols, Rows: leftMat.Rows, Element: elem}
				}
			}
		}
	}

	return nil
}

// DivResultType returns the result type of a / b.
// Division supports:
//   - scalar / scalar → scalar
//   - vector / vector → vector (component-wise)
//   - vector / scalar → vector
//   - scalar / vector → vector (broadcasts scalar)
func DivResultType(left, right Type) Type {
	// Common type handles scalar/scalar and vec/vec
	if common := CommonType(left, right); common != nil {
		return common
	}

	leftConc := ConcreteType(left)
	rightConc := ConcreteType(right)

	// Vector / Scalar
	if leftVec, ok := leftConc.(*Vector); ok {
		if rightScalar, ok := rightConc.(*Scalar); ok {
			elem := commonScalarType(leftVec.Element, rightScalar)
			if elem != nil {
				return &Vector{Width: leftVec.Width, Element: elem}
			}
		}
	}

	// Scalar / Vector (broadcasts scalar)
	if leftScalar, ok := leftConc.(*Scalar); ok {
		if rightVec, ok := rightConc.(*Vector); ok {
			elem := commonScalarType(leftScalar, rightVec.Element)
			if elem != nil {
				return &Vector{Width: rightVec.Width, Element: elem}
			}
		}
	}

	return nil
}

// commonScalarType returns the common scalar type between two scalars.
func commonScalarType(a, b *Scalar) *Scalar {
	if a.Equals(b) {
		return a
	}

	// Abstract types can convert to concrete
	if a.Kind == ScalarAbstractFloat || a.Kind == ScalarAbstractInt {
		if b.IsNumeric() {
			return b
		}
	}
	if b.Kind == ScalarAbstractFloat || b.Kind == ScalarAbstractInt {
		if a.IsNumeric() {
			return a
		}
	}

	return nil
}

// ConcreteType returns the concrete version of an abstract type.
// For abstract-float returns f32, for abstract-int returns i32.
// For vectors/matrices with abstract elements, returns concrete element versions.
// For already concrete types, returns the type unchanged.
func ConcreteType(t Type) Type {
	switch ty := t.(type) {
	case *Scalar:
		switch ty.Kind {
		case ScalarAbstractFloat:
			return F32
		case ScalarAbstractInt:
			return I32
		}
	case *Vector:
		if ty.Element.Kind == ScalarAbstractFloat {
			return &Vector{Width: ty.Width, Element: F32}
		}
		if ty.Element.Kind == ScalarAbstractInt {
			return &Vector{Width: ty.Width, Element: I32}
		}
	case *Matrix:
		if ty.Element.Kind == ScalarAbstractFloat {
			return &Matrix{Cols: ty.Cols, Rows: ty.Rows, Element: F32}
		}
	case *Array:
		if concreteElem := ConcreteType(ty.Element); concreteElem != ty.Element {
			return &Array{Element: concreteElem, Count: ty.Count}
		}
	}
	return t
}
