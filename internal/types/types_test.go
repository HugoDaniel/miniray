package types

import (
	"fmt"
	"testing"
)

func TestMultiplyResultTypeDebug(t *testing.T) {
	// Simulate what the validator does
	mat := &Matrix{Cols: 4, Rows: 4, Element: F32}
	vec := &Vector{Width: 4, Element: F32}

	fmt.Printf("Testing mat * vec:\n")
	fmt.Printf("  mat: %s (Cols=%d, Rows=%d)\n", mat.String(), mat.Cols, mat.Rows)
	fmt.Printf("  vec: %s (Width=%d)\n", vec.String(), vec.Width)

	// Check CommonType first
	common := CommonType(mat, vec)
	fmt.Printf("  CommonType result: %v\n", common)

	// Check if they're pointer types being converted
	leftConc := ConcreteType(mat)
	rightConc := ConcreteType(vec)
	fmt.Printf("  leftConc: %T %s\n", leftConc, leftConc.String())
	fmt.Printf("  rightConc: %T %s\n", rightConc, rightConc.String())

	// Check matrix * vector branch
	if matC, ok := leftConc.(*Matrix); ok {
		fmt.Printf("  Left is matrix: Cols=%d, Rows=%d\n", matC.Cols, matC.Rows)
		if vecC, ok := rightConc.(*Vector); ok {
			fmt.Printf("  Right is vector: Width=%d\n", vecC.Width)
			fmt.Printf("  mat.Cols == vec.Width: %v\n", matC.Cols == vecC.Width)
		}
	}

	result := MultiplyResultType(mat, vec)
	if result == nil {
		t.Error("Expected result, got nil")
	} else {
		fmt.Printf("  Result: %s\n", result.String())
	}
}

func TestMultiplyResultType(t *testing.T) {
	tests := []struct {
		name   string
		left   Type
		right  Type
		expect string
	}{
		{
			name:   "mat4x4 * vec4",
			left:   &Matrix{Cols: 4, Rows: 4, Element: F32},
			right:  &Vector{Width: 4, Element: F32},
			expect: "vec4<f32>",
		},
		{
			name:   "mat3x3 * vec3",
			left:   &Matrix{Cols: 3, Rows: 3, Element: F32},
			right:  &Vector{Width: 3, Element: F32},
			expect: "vec3<f32>",
		},
		{
			name:   "vec3 * mat3x3",
			left:   &Vector{Width: 3, Element: F32},
			right:  &Matrix{Cols: 3, Rows: 3, Element: F32},
			expect: "vec3<f32>",
		},
		{
			name:   "vec3 * f32",
			left:   &Vector{Width: 3, Element: F32},
			right:  F32,
			expect: "vec3<f32>",
		},
		{
			name:   "f32 * vec3",
			left:   F32,
			right:  &Vector{Width: 3, Element: F32},
			expect: "vec3<f32>",
		},
		{
			name:   "mat4x4 * f32",
			left:   &Matrix{Cols: 4, Rows: 4, Element: F32},
			right:  F32,
			expect: "mat4x4<f32>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MultiplyResultType(tt.left, tt.right)
			if result == nil {
				t.Errorf("expected %s, got nil", tt.expect)
				return
			}
			if result.String() != tt.expect {
				t.Errorf("expected %s, got %s", tt.expect, result.String())
			}
		})
	}
}
