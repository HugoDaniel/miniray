package reflect

// TypeLayout holds size and alignment information for a WGSL type.
type TypeLayout struct {
	Size      int
	Alignment int
	Stride    int // For arrays only (0 otherwise)
}

// WGSL primitive type layouts according to the WGSL specification.
// Reference: https://www.w3.org/TR/WGSL/#alignment-and-size
var primitiveLayouts = map[string]TypeLayout{
	// Scalars (Section 6.2.1)
	"bool": {Size: 4, Alignment: 4}, // WGSL bool is 4 bytes in uniform/storage
	"i32":  {Size: 4, Alignment: 4},
	"u32":  {Size: 4, Alignment: 4},
	"f32":  {Size: 4, Alignment: 4},
	"f16":  {Size: 2, Alignment: 2},

	// Vector shorthands - 32-bit element types
	"vec2i": {Size: 8, Alignment: 8},
	"vec3i": {Size: 12, Alignment: 16}, // CRITICAL: align 16, size 12
	"vec4i": {Size: 16, Alignment: 16},
	"vec2u": {Size: 8, Alignment: 8},
	"vec3u": {Size: 12, Alignment: 16},
	"vec4u": {Size: 16, Alignment: 16},
	"vec2f": {Size: 8, Alignment: 8},
	"vec3f": {Size: 12, Alignment: 16},
	"vec4f": {Size: 16, Alignment: 16},
	"vec2b": {Size: 8, Alignment: 8},   // vec2<bool>
	"vec3b": {Size: 12, Alignment: 16}, // vec3<bool>
	"vec4b": {Size: 16, Alignment: 16}, // vec4<bool>

	// Vector shorthands - 16-bit element types (f16)
	"vec2h": {Size: 4, Alignment: 4},
	"vec3h": {Size: 6, Alignment: 8},
	"vec4h": {Size: 8, Alignment: 8},

	// Matrix shorthands - f32 element type
	// MatCxR is C columns of R-element vectors
	// Size = C * roundUp(AlignOf(vecR), SizeOf(vecR))
	"mat2x2f": {Size: 16, Alignment: 8},  // 2 * roundUp(8, 8) = 2 * 8 = 16
	"mat2x3f": {Size: 32, Alignment: 16}, // 2 * roundUp(16, 12) = 2 * 16 = 32
	"mat2x4f": {Size: 32, Alignment: 16}, // 2 * roundUp(16, 16) = 2 * 16 = 32
	"mat3x2f": {Size: 24, Alignment: 8},  // 3 * roundUp(8, 8) = 3 * 8 = 24
	"mat3x3f": {Size: 48, Alignment: 16}, // 3 * roundUp(16, 12) = 3 * 16 = 48
	"mat3x4f": {Size: 48, Alignment: 16}, // 3 * roundUp(16, 16) = 3 * 16 = 48
	"mat4x2f": {Size: 32, Alignment: 8},  // 4 * roundUp(8, 8) = 4 * 8 = 32
	"mat4x3f": {Size: 64, Alignment: 16}, // 4 * roundUp(16, 12) = 4 * 16 = 64
	"mat4x4f": {Size: 64, Alignment: 16}, // 4 * roundUp(16, 16) = 4 * 16 = 64

	// Matrix shorthands - f16 element type
	"mat2x2h": {Size: 8, Alignment: 4},  // 2 * roundUp(4, 4) = 2 * 4 = 8
	"mat2x3h": {Size: 16, Alignment: 8}, // 2 * roundUp(8, 6) = 2 * 8 = 16
	"mat2x4h": {Size: 16, Alignment: 8}, // 2 * roundUp(8, 8) = 2 * 8 = 16
	"mat3x2h": {Size: 12, Alignment: 4}, // 3 * roundUp(4, 4) = 3 * 4 = 12
	"mat3x3h": {Size: 24, Alignment: 8}, // 3 * roundUp(8, 6) = 3 * 8 = 24
	"mat3x4h": {Size: 24, Alignment: 8}, // 3 * roundUp(8, 8) = 3 * 8 = 24
	"mat4x2h": {Size: 16, Alignment: 4}, // 4 * roundUp(4, 4) = 4 * 4 = 16
	"mat4x3h": {Size: 32, Alignment: 8}, // 4 * roundUp(8, 6) = 4 * 8 = 32
	"mat4x4h": {Size: 32, Alignment: 8}, // 4 * roundUp(8, 8) = 4 * 8 = 32
}

// vectorElementSize returns the size of a vector element type.
func vectorElementSize(elemTypeName string) int {
	switch elemTypeName {
	case "f16":
		return 2
	case "f32", "i32", "u32", "bool":
		return 4
	default:
		return 4 // Default to 4 bytes
	}
}

// computeVecLayout computes the layout for a vector type.
// For vec2<T>: align = 2*sizeof(T), size = 2*sizeof(T)
// For vec3<T>: align = 4*sizeof(T), size = 3*sizeof(T)
// For vec4<T>: align = 4*sizeof(T), size = 4*sizeof(T)
func computeVecLayout(size int, elemSize int) TypeLayout {
	switch size {
	case 2:
		return TypeLayout{
			Size:      elemSize * 2,
			Alignment: elemSize * 2,
		}
	case 3:
		// vec3 has alignment of vec4 but size of 3 elements
		return TypeLayout{
			Size:      elemSize * 3,
			Alignment: elemSize * 4,
		}
	case 4:
		return TypeLayout{
			Size:      elemSize * 4,
			Alignment: elemSize * 4,
		}
	default:
		return TypeLayout{}
	}
}

// computeMatLayout computes the layout for a matrix type.
// Matrix is stored as C columns of vecR<T> vectors.
// AlignOf(matCxR<T>) = AlignOf(vecR<T>)
// SizeOf(matCxR<T>) = C * roundUp(AlignOf(vecR<T>), SizeOf(vecR<T>))
func computeMatLayout(cols, rows int, elemSize int) TypeLayout {
	colVec := computeVecLayout(rows, elemSize)
	stride := roundUp(colVec.Size, colVec.Alignment)
	return TypeLayout{
		Size:      cols * stride,
		Alignment: colVec.Alignment,
	}
}

// roundUp rounds x up to the nearest multiple of align.
// This is the WGSL roundUp function from the spec.
func roundUp(x, align int) int {
	if align == 0 {
		return x
	}
	return ((x + align - 1) / align) * align
}
