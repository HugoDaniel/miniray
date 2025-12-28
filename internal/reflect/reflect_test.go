package reflect

import (
	"testing"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/parser"
)

func TestReflectBasicStruct(t *testing.T) {
	source := `
struct Inputs {
    time: f32,
    resolution: vec2<u32>,
    brightness: f32,
}
@group(0) @binding(0) var<uniform> u: Inputs;
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Check bindings
	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Group != 0 {
		t.Errorf("expected group 0, got %d", binding.Group)
	}
	if binding.Binding != 0 {
		t.Errorf("expected binding 0, got %d", binding.Binding)
	}
	if binding.Name != "u" {
		t.Errorf("expected name 'u', got '%s'", binding.Name)
	}
	if binding.AddressSpace != "uniform" {
		t.Errorf("expected addressSpace 'uniform', got '%s'", binding.AddressSpace)
	}
	if binding.Type != "Inputs" {
		t.Errorf("expected type 'Inputs', got '%s'", binding.Type)
	}
	if binding.Layout == nil {
		t.Fatal("expected layout to be set")
	}

	// Check struct layout
	// time: f32 (offset 0, size 4, align 4)
	// resolution: vec2<u32> (offset 8 after alignment, size 8, align 8)
	// brightness: f32 (offset 16, size 4, align 4)
	// Total: 20 bytes, rounded up to alignment 8 = 24 bytes
	layout := binding.Layout
	if layout.Alignment != 8 {
		t.Errorf("expected alignment 8, got %d", layout.Alignment)
	}
	if layout.Size != 24 {
		t.Errorf("expected size 24, got %d", layout.Size)
	}
	if len(layout.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(layout.Fields))
	}

	// Check field offsets
	expectedFields := []struct {
		name   string
		offset int
		size   int
		align  int
	}{
		{"time", 0, 4, 4},
		{"resolution", 8, 8, 8}, // Aligned to 8
		{"brightness", 16, 4, 4},
	}

	for i, expected := range expectedFields {
		field := layout.Fields[i]
		if field.Name != expected.name {
			t.Errorf("field %d: expected name '%s', got '%s'", i, expected.name, field.Name)
		}
		if field.Offset != expected.offset {
			t.Errorf("field %d (%s): expected offset %d, got %d", i, expected.name, expected.offset, field.Offset)
		}
		if field.Size != expected.size {
			t.Errorf("field %d (%s): expected size %d, got %d", i, expected.name, expected.size, field.Size)
		}
		if field.Alignment != expected.align {
			t.Errorf("field %d (%s): expected alignment %d, got %d", i, expected.name, expected.align, field.Alignment)
		}
	}
}

func TestVec3Alignment(t *testing.T) {
	// vec3 has alignment=16 but size=12 - this is a critical edge case
	source := `
struct WithVec3 {
    a: f32,
    b: vec3<f32>,
    c: f32,
}
@group(0) @binding(0) var<uniform> u: WithVec3;
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	layout := result.Bindings[0].Layout
	if layout == nil {
		t.Fatal("expected layout to be set")
	}

	// a: f32 (offset 0, size 4, align 4)
	// b: vec3<f32> (offset 16 after alignment to 16, size 12, align 16)
	// c: f32 (offset 28, size 4, align 4)
	// Total: 32 bytes, rounded up to alignment 16 = 32 bytes
	if layout.Alignment != 16 {
		t.Errorf("expected alignment 16, got %d", layout.Alignment)
	}
	if layout.Size != 32 {
		t.Errorf("expected size 32, got %d", layout.Size)
	}

	expectedFields := []struct {
		name   string
		offset int
		size   int
		align  int
	}{
		{"a", 0, 4, 4},
		{"b", 16, 12, 16}, // Aligned to 16, size is 12 (not 16!)
		{"c", 28, 4, 4},
	}

	for i, expected := range expectedFields {
		field := layout.Fields[i]
		if field.Name != expected.name {
			t.Errorf("field %d: expected name '%s', got '%s'", i, expected.name, field.Name)
		}
		if field.Offset != expected.offset {
			t.Errorf("field %d (%s): expected offset %d, got %d", i, expected.name, expected.offset, field.Offset)
		}
		if field.Size != expected.size {
			t.Errorf("field %d (%s): expected size %d, got %d", i, expected.name, expected.size, field.Size)
		}
		if field.Alignment != expected.align {
			t.Errorf("field %d (%s): expected alignment %d, got %d", i, expected.name, expected.align, field.Alignment)
		}
	}
}

func TestEntryPoints(t *testing.T) {
	source := `
@compute @workgroup_size(8, 8, 1)
fn main() {}

@vertex
fn vertMain() -> @builtin(position) vec4f {
    return vec4f(0.0);
}

@fragment
fn fragMain() -> @location(0) vec4f {
    return vec4f(1.0);
}

fn helperFunc() {}
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.EntryPoints) != 3 {
		t.Fatalf("expected 3 entry points, got %d", len(result.EntryPoints))
	}

	// Check compute entry point
	var compute, vertex, fragment *EntryPointInfo
	for i := range result.EntryPoints {
		ep := &result.EntryPoints[i]
		switch ep.Stage {
		case "compute":
			compute = ep
		case "vertex":
			vertex = ep
		case "fragment":
			fragment = ep
		}
	}

	if compute == nil {
		t.Fatal("compute entry point not found")
	}
	if compute.Name != "main" {
		t.Errorf("expected compute name 'main', got '%s'", compute.Name)
	}
	if len(compute.WorkgroupSize) != 3 {
		t.Errorf("expected workgroup size length 3, got %d", len(compute.WorkgroupSize))
	} else {
		if compute.WorkgroupSize[0] != 8 || compute.WorkgroupSize[1] != 8 || compute.WorkgroupSize[2] != 1 {
			t.Errorf("expected workgroup size [8,8,1], got %v", compute.WorkgroupSize)
		}
	}

	if vertex == nil {
		t.Fatal("vertex entry point not found")
	}
	if vertex.Name != "vertMain" {
		t.Errorf("expected vertex name 'vertMain', got '%s'", vertex.Name)
	}

	if fragment == nil {
		t.Fatal("fragment entry point not found")
	}
	if fragment.Name != "fragMain" {
		t.Errorf("expected fragment name 'fragMain', got '%s'", fragment.Name)
	}
}

func TestMultipleBindings(t *testing.T) {
	source := `
struct Uniforms {
    mvp: mat4x4f,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var texSampler: sampler;
@group(0) @binding(2) var texture: texture_2d<f32>;
@group(1) @binding(0) var<storage, read_write> data: array<f32>;
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 4 {
		t.Fatalf("expected 4 bindings, got %d", len(result.Bindings))
	}

	// Find bindings by name
	bindings := make(map[string]*BindingInfo)
	for i := range result.Bindings {
		b := &result.Bindings[i]
		bindings[b.Name] = b
	}

	// Check uniform binding
	if ub, ok := bindings["uniforms"]; ok {
		if ub.Group != 0 || ub.Binding != 0 {
			t.Errorf("uniforms: expected group 0 binding 0, got group %d binding %d", ub.Group, ub.Binding)
		}
		if ub.AddressSpace != "uniform" {
			t.Errorf("uniforms: expected addressSpace 'uniform', got '%s'", ub.AddressSpace)
		}
		if ub.Layout == nil {
			t.Error("uniforms: expected layout to be set")
		}
	} else {
		t.Error("uniforms binding not found")
	}

	// Check sampler binding
	if sb, ok := bindings["texSampler"]; ok {
		if sb.Group != 0 || sb.Binding != 1 {
			t.Errorf("texSampler: expected group 0 binding 1, got group %d binding %d", sb.Group, sb.Binding)
		}
		if sb.AddressSpace != "handle" {
			t.Errorf("texSampler: expected addressSpace 'handle', got '%s'", sb.AddressSpace)
		}
		if sb.Type != "sampler" {
			t.Errorf("texSampler: expected type 'sampler', got '%s'", sb.Type)
		}
		if sb.Layout != nil {
			t.Error("texSampler: expected layout to be nil")
		}
	} else {
		t.Error("texSampler binding not found")
	}

	// Check texture binding
	if tb, ok := bindings["texture"]; ok {
		if tb.Group != 0 || tb.Binding != 2 {
			t.Errorf("texture: expected group 0 binding 2, got group %d binding %d", tb.Group, tb.Binding)
		}
		if tb.Layout != nil {
			t.Error("texture: expected layout to be nil")
		}
	} else {
		t.Error("texture binding not found")
	}

	// Check storage binding
	if db, ok := bindings["data"]; ok {
		if db.Group != 1 || db.Binding != 0 {
			t.Errorf("data: expected group 1 binding 0, got group %d binding %d", db.Group, db.Binding)
		}
		if db.AddressSpace != "storage" {
			t.Errorf("data: expected addressSpace 'storage', got '%s'", db.AddressSpace)
		}
		if db.AccessMode != "read_write" {
			t.Errorf("data: expected accessMode 'read_write', got '%s'", db.AccessMode)
		}
	} else {
		t.Error("data binding not found")
	}
}

func TestNestedStruct(t *testing.T) {
	source := `
struct Inner {
    x: f32,
    y: f32,
}

struct Outer {
    a: f32,
    inner: Inner,
    b: f32,
}

@group(0) @binding(0) var<uniform> u: Outer;
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	layout := result.Bindings[0].Layout
	if layout == nil {
		t.Fatal("expected layout to be set")
	}

	// Outer struct:
	// a: f32 (offset 0, size 4)
	// inner: Inner (offset 4, size 8, align 4)
	// b: f32 (offset 12, size 4)
	// Total: 16 bytes
	if layout.Size != 16 {
		t.Errorf("expected size 16, got %d", layout.Size)
	}

	// Check nested struct has layout
	if len(layout.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(layout.Fields))
	}

	innerField := layout.Fields[1]
	if innerField.Name != "inner" {
		t.Errorf("expected field name 'inner', got '%s'", innerField.Name)
	}
	if innerField.Layout == nil {
		t.Error("expected inner field to have layout")
	} else {
		if innerField.Layout.Size != 8 {
			t.Errorf("expected inner size 8, got %d", innerField.Layout.Size)
		}
	}
}

func TestMatrixLayout(t *testing.T) {
	source := `
struct WithMatrix {
    m2x2: mat2x2f,
    m3x3: mat3x3f,
    m4x4: mat4x4f,
}
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["WithMatrix"]
	if !ok {
		t.Fatal("WithMatrix struct not found")
	}

	// mat2x2f: size 16, align 8
	// mat3x3f: offset 16, size 48, align 16 -> offset becomes 16
	// mat4x4f: offset 64, size 64, align 16
	// Total: 128, alignment 16
	expectedFields := []struct {
		name   string
		offset int
		size   int
		align  int
	}{
		{"m2x2", 0, 16, 8},
		{"m3x3", 16, 48, 16},
		{"m4x4", 64, 64, 16},
	}

	for i, expected := range expectedFields {
		field := layout.Fields[i]
		if field.Name != expected.name {
			t.Errorf("field %d: expected name '%s', got '%s'", i, expected.name, field.Name)
		}
		if field.Offset != expected.offset {
			t.Errorf("field %d (%s): expected offset %d, got %d", i, expected.name, expected.offset, field.Offset)
		}
		if field.Size != expected.size {
			t.Errorf("field %d (%s): expected size %d, got %d", i, expected.name, expected.size, field.Size)
		}
		if field.Alignment != expected.align {
			t.Errorf("field %d (%s): expected alignment %d, got %d", i, expected.name, expected.align, field.Alignment)
		}
	}

	if layout.Size != 128 {
		t.Errorf("expected struct size 128, got %d", layout.Size)
	}
}

func TestArrayLayout(t *testing.T) {
	source := `
struct WithArray {
    values: array<f32, 4>,
}
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["WithArray"]
	if !ok {
		t.Fatal("WithArray struct not found")
	}

	// array<f32, 4>: size 16, align 4
	if len(layout.Fields) != 1 {
		t.Fatalf("expected 1 field, got %d", len(layout.Fields))
	}

	field := layout.Fields[0]
	if field.Size != 16 {
		t.Errorf("expected array size 16, got %d", field.Size)
	}
	if field.Alignment != 4 {
		t.Errorf("expected array alignment 4, got %d", field.Alignment)
	}
}

func TestParseErrors(t *testing.T) {
	// Invalid syntax should produce parse errors
	source := `struct { invalid syntax here`
	result := Reflect(source)

	if len(result.Errors) == 0 {
		t.Error("expected parse errors")
	}
}

func TestRoundUp(t *testing.T) {
	tests := []struct {
		x, align, want int
	}{
		{0, 4, 0},
		{1, 4, 4},
		{4, 4, 4},
		{5, 4, 8},
		{12, 16, 16},
		{16, 16, 16},
		{17, 16, 32},
	}

	for _, tc := range tests {
		got := roundUp(tc.x, tc.align)
		if got != tc.want {
			t.Errorf("roundUp(%d, %d) = %d, want %d", tc.x, tc.align, got, tc.want)
		}
	}
}

// The following tests are verified against:
// https://webgpufundamentals.org/webgpu/lessons/resources/wgsl-offset-computer.html

func TestMediumStructSize(t *testing.T) {
	// struct custom {
	//   mode: u32,
	//   power: f32,
	//   range: f32,
	//   innerAngle: f32,
	//   outerAngle: f32,
	//   direction: vec3f,
	//   position: vec3f,
	// }
	source := `struct custom {
		mode: u32,
		power: f32,
		range_: f32,
		innerAngle: f32,
		outerAngle: f32,
		direction: vec3f,
		position: vec3f,
	}`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["custom"]
	if !ok {
		t.Fatal("custom struct not found")
	}

	// Verified at webgpufundamentals offset calculator
	expected := 64
	if layout.Size != expected {
		t.Errorf("expected size %d, got %d", expected, layout.Size)
	}
}

func TestStructWithArrayOfStructs(t *testing.T) {
	// struct Light {
	//   mode: u32,
	//   power: f32,
	//   range: f32,
	//   innerAngle: f32,
	//   outerAngle: f32,
	//   direction: vec3f,
	//   position: vec3f,
	// }
	// struct custom {
	//   colorMult: vec4f,
	//   specularFactor: f32,
	//   lights: array<Light, 2>,
	// }
	source := `struct Light {
		mode: u32,
		power: f32,
		range_: f32,
		innerAngle: f32,
		outerAngle: f32,
		direction: vec3f,
		position: vec3f,
	}
	struct custom {
		colorMult: vec4f,
		specularFactor: f32,
		lights: array<Light, 2>,
	}`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["custom"]
	if !ok {
		t.Fatal("custom struct not found")
	}

	// Verified at webgpufundamentals offset calculator
	expected := 160
	if layout.Size != expected {
		t.Errorf("expected size %d, got %d", expected, layout.Size)
	}
}

func TestStructWithFourMatrices(t *testing.T) {
	// struct custom {
	//   projectionMatrix: mat4x4f,
	//   viewMatrix: mat4x4f,
	//   modelMatrix: mat4x4f,
	//   normalMatrix: mat4x4f,
	// }
	source := `struct custom {
		projectionMatrix: mat4x4f,
		viewMatrix: mat4x4f,
		modelMatrix: mat4x4f,
		normalMatrix: mat4x4f,
	}`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["custom"]
	if !ok {
		t.Fatal("custom struct not found")
	}

	// Verified at webgpufundamentals offset calculator
	expected := 256
	if layout.Size != expected {
		t.Errorf("expected size %d, got %d", expected, layout.Size)
	}
}

func TestStructWithMixedVectors(t *testing.T) {
	// struct custom {
	//   position: vec4f,
	//   texcoord: vec2f,
	//   normal: vec3f,
	// }
	source := `struct custom {
		position: vec4f,
		texcoord: vec2f,
		normal: vec3f,
	}`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["custom"]
	if !ok {
		t.Fatal("custom struct not found")
	}

	// Verified at webgpufundamentals offset calculator
	expected := 48
	if layout.Size != expected {
		t.Errorf("expected size %d, got %d", expected, layout.Size)
	}
}

func TestSingleVec3fStruct(t *testing.T) {
	// struct custom { orientation: vec3f }
	source := `struct custom { orientation: vec3f }`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["custom"]
	if !ok {
		t.Fatal("custom struct not found")
	}

	// vec3f: align=16, size=12, struct size rounded up to 16
	expected := 16
	if layout.Size != expected {
		t.Errorf("expected size %d, got %d", expected, layout.Size)
	}
}

func TestComplexNestedStruct(t *testing.T) {
	// struct info { velocity: vec3f }
	// struct custom {
	//   orientation: vec3f,
	//   size: f32,
	//   direction: array<vec3f, 2>,
	//   scale: f32,
	//   info: info,
	//   friction: f32,
	// }
	source := `struct info { velocity: vec3f }
	struct custom {
		orientation: vec3f,
		size: f32,
		direction: array<vec3f, 2>,
		scale: f32,
		info: info,
		friction: f32,
	}`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["custom"]
	if !ok {
		t.Fatal("custom struct not found")
	}

	// Verified at webgpufundamentals offset calculator
	expected := 96
	if layout.Size != expected {
		t.Errorf("expected size %d, got %d", expected, layout.Size)
	}
}

func TestTwoVec3fStruct(t *testing.T) {
	// struct custom { orientation: vec3f, normal: vec3f }
	source := `struct custom { orientation: vec3f, normal: vec3f }`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["custom"]
	if !ok {
		t.Fatal("custom struct not found")
	}

	// orientation: offset=0, size=12, align=16
	// normal: offset=16 (aligned), size=12, align=16
	// Total: 28, rounded up to 32
	expected := 32
	if layout.Size != expected {
		t.Errorf("expected size %d, got %d", expected, layout.Size)
	}

	// Check field offsets
	if len(layout.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(layout.Fields))
	}
	if layout.Fields[0].Offset != 0 {
		t.Errorf("orientation offset: expected 0, got %d", layout.Fields[0].Offset)
	}
	if layout.Fields[1].Offset != 16 {
		t.Errorf("normal offset: expected 16, got %d", layout.Fields[1].Offset)
	}
}

func TestStructWithNestedStructAndVec3f(t *testing.T) {
	// struct info { velocity: vec3f }
	// struct custom {
	//   orientation: vec3f,
	//   size: f32,
	//   scale: f32,
	//   info: info,
	//   friction: f32,
	// }
	source := `struct info { velocity: vec3f }
	struct custom {
		orientation: vec3f,
		size: f32,
		scale: f32,
		info: info,
		friction: f32,
	}`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["custom"]
	if !ok {
		t.Fatal("custom struct not found")
	}

	// Verified at webgpufundamentals offset calculator
	expected := 64
	if layout.Size != expected {
		t.Errorf("expected size %d, got %d", expected, layout.Size)
	}
}

func TestArrayOfVec2f(t *testing.T) {
	// struct custom { orientation: array<vec2f, 3> }
	source := `struct custom { orientation: array<vec2f, 3> }`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["custom"]
	if !ok {
		t.Fatal("custom struct not found")
	}

	// array<vec2f, 3>: stride=8, size=24, align=8
	expected := 24
	if layout.Size != expected {
		t.Errorf("expected size %d, got %d", expected, layout.Size)
	}
}

func TestComplexStructWithArrays(t *testing.T) {
	// struct info { velocity: vec3f }
	// struct custom {
	//   scale: f32,
	//   orientation: array<vec2f, 3>,
	//   size: vec2f,
	//   pos: vec2f,
	//   info: array<info, 2>,
	// }
	source := `struct info { velocity: vec3f }
	struct custom {
		scale: f32,
		orientation: array<vec2f, 3>,
		size: vec2f,
		pos: vec2f,
		info: array<info, 2>,
	}`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	layout, ok := result.Structs["custom"]
	if !ok {
		t.Fatal("custom struct not found")
	}

	// Verified at webgpufundamentals offset calculator
	expected := 80
	if layout.Size != expected {
		t.Errorf("expected size %d, got %d", expected, layout.Size)
	}
}

// --- Array Reflection Tests ---

func TestArrayBindingSimple(t *testing.T) {
	source := `@group(0) @binding(0) var<storage> data: array<f32, 100>;`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Name != "data" {
		t.Errorf("expected name 'data', got '%s'", binding.Name)
	}
	if binding.NameMapped != "data" {
		t.Errorf("expected nameMapped 'data', got '%s'", binding.NameMapped)
	}
	if binding.Type != "array<f32, 100>" {
		t.Errorf("expected type 'array<f32, 100>', got '%s'", binding.Type)
	}
	if binding.TypeMapped != "array<f32, 100>" {
		t.Errorf("expected typeMapped 'array<f32, 100>', got '%s'", binding.TypeMapped)
	}

	// Layout should be nil for array types
	if binding.Layout != nil {
		t.Error("expected layout to be nil for array type")
	}

	// Array info should be present
	if binding.Array == nil {
		t.Fatal("expected array info to be set")
	}

	arr := binding.Array
	if arr.Depth != 1 {
		t.Errorf("expected depth 1, got %d", arr.Depth)
	}
	if arr.ElementCount == nil || *arr.ElementCount != 100 {
		t.Errorf("expected elementCount 100, got %v", arr.ElementCount)
	}
	if arr.ElementStride != 4 {
		t.Errorf("expected elementStride 4, got %d", arr.ElementStride)
	}
	if arr.TotalSize == nil || *arr.TotalSize != 400 {
		t.Errorf("expected totalSize 400, got %v", arr.TotalSize)
	}
	if arr.ElementType != "f32" {
		t.Errorf("expected elementType 'f32', got '%s'", arr.ElementType)
	}
	if arr.ElementTypeMapped != "f32" {
		t.Errorf("expected elementTypeMapped 'f32', got '%s'", arr.ElementTypeMapped)
	}
	if arr.ElementLayout != nil {
		t.Error("expected elementLayout to be nil for primitive type")
	}
	if arr.Array != nil {
		t.Error("expected nested array to be nil")
	}
}

func TestArrayBindingRuntimeSized(t *testing.T) {
	source := `@group(0) @binding(0) var<storage> data: array<f32>;`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Array == nil {
		t.Fatal("expected array info to be set")
	}

	arr := binding.Array
	if arr.Depth != 1 {
		t.Errorf("expected depth 1, got %d", arr.Depth)
	}
	if arr.ElementCount != nil {
		t.Errorf("expected elementCount nil, got %v", arr.ElementCount)
	}
	if arr.TotalSize != nil {
		t.Errorf("expected totalSize nil, got %v", arr.TotalSize)
	}
	if arr.ElementStride != 4 {
		t.Errorf("expected elementStride 4, got %d", arr.ElementStride)
	}
}

func TestArrayBindingOfStructs(t *testing.T) {
	source := `
struct Particle {
	pos: vec3f,
	vel: f32,
}
@group(0) @binding(0) var<storage, read_write> data: array<Particle, 10000>;
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Type != "array<Particle, 10000>" {
		t.Errorf("expected type 'array<Particle, 10000>', got '%s'", binding.Type)
	}
	if binding.AccessMode != "read_write" {
		t.Errorf("expected accessMode 'read_write', got '%s'", binding.AccessMode)
	}

	// Array info should be present
	if binding.Array == nil {
		t.Fatal("expected array info to be set")
	}

	arr := binding.Array
	if arr.Depth != 1 {
		t.Errorf("expected depth 1, got %d", arr.Depth)
	}
	if arr.ElementCount == nil || *arr.ElementCount != 10000 {
		t.Errorf("expected elementCount 10000, got %v", arr.ElementCount)
	}
	if arr.ElementType != "Particle" {
		t.Errorf("expected elementType 'Particle', got '%s'", arr.ElementType)
	}

	// Element layout should be present for struct types
	if arr.ElementLayout == nil {
		t.Fatal("expected elementLayout to be set for struct type")
	}

	// Particle: pos: vec3f (offset 0, size 12, align 16), vel: f32 (offset 12, size 4, align 4)
	// Total: 16 bytes
	if arr.ElementLayout.Size != 16 {
		t.Errorf("expected elementLayout size 16, got %d", arr.ElementLayout.Size)
	}
	if arr.ElementStride != 16 {
		t.Errorf("expected elementStride 16, got %d", arr.ElementStride)
	}
	if arr.TotalSize == nil || *arr.TotalSize != 160000 {
		t.Errorf("expected totalSize 160000, got %v", arr.TotalSize)
	}

	// Check element layout fields
	if len(arr.ElementLayout.Fields) != 2 {
		t.Fatalf("expected 2 fields, got %d", len(arr.ElementLayout.Fields))
	}
	if arr.ElementLayout.Fields[0].Name != "pos" {
		t.Errorf("expected field 0 name 'pos', got '%s'", arr.ElementLayout.Fields[0].Name)
	}
	if arr.ElementLayout.Fields[1].Name != "vel" {
		t.Errorf("expected field 1 name 'vel', got '%s'", arr.ElementLayout.Fields[1].Name)
	}
}

func TestArrayBindingNested(t *testing.T) {
	source := `@group(0) @binding(0) var<storage> matrix: array<array<f32, 4>, 10>;`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Type != "array<array<f32, 4>, 10>" {
		t.Errorf("expected type 'array<array<f32, 4>, 10>', got '%s'", binding.Type)
	}

	if binding.Array == nil {
		t.Fatal("expected array info to be set")
	}

	// Outer array
	outer := binding.Array
	if outer.Depth != 1 {
		t.Errorf("expected outer depth 1, got %d", outer.Depth)
	}
	if outer.ElementCount == nil || *outer.ElementCount != 10 {
		t.Errorf("expected outer elementCount 10, got %v", outer.ElementCount)
	}
	if outer.ElementType != "array<f32, 4>" {
		t.Errorf("expected outer elementType 'array<f32, 4>', got '%s'", outer.ElementType)
	}
	if outer.ElementStride != 16 {
		t.Errorf("expected outer elementStride 16, got %d", outer.ElementStride)
	}
	if outer.TotalSize == nil || *outer.TotalSize != 160 {
		t.Errorf("expected outer totalSize 160, got %v", outer.TotalSize)
	}
	if outer.ElementLayout != nil {
		t.Error("expected outer elementLayout to be nil (inner is array, not struct)")
	}

	// Inner array
	if outer.Array == nil {
		t.Fatal("expected nested array info to be set")
	}
	inner := outer.Array
	if inner.Depth != 2 {
		t.Errorf("expected inner depth 2, got %d", inner.Depth)
	}
	if inner.ElementCount == nil || *inner.ElementCount != 4 {
		t.Errorf("expected inner elementCount 4, got %v", inner.ElementCount)
	}
	if inner.ElementType != "f32" {
		t.Errorf("expected inner elementType 'f32', got '%s'", inner.ElementType)
	}
	if inner.ElementStride != 4 {
		t.Errorf("expected inner elementStride 4, got %d", inner.ElementStride)
	}
	if inner.TotalSize == nil || *inner.TotalSize != 16 {
		t.Errorf("expected inner totalSize 16, got %v", inner.TotalSize)
	}
	if inner.Array != nil {
		t.Error("expected inner nested array to be nil")
	}
}

func TestArrayBindingDeeplyNested(t *testing.T) {
	source := `@group(0) @binding(0) var<storage> tensor: array<array<array<f32, 2>, 3>, 4>;`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Array == nil {
		t.Fatal("expected array info to be set")
	}

	// Level 1
	level1 := binding.Array
	if level1.Depth != 1 {
		t.Errorf("expected level1 depth 1, got %d", level1.Depth)
	}
	if level1.ElementCount == nil || *level1.ElementCount != 4 {
		t.Errorf("expected level1 elementCount 4, got %v", level1.ElementCount)
	}
	if level1.Array == nil {
		t.Fatal("expected level1 nested array")
	}

	// Level 2
	level2 := level1.Array
	if level2.Depth != 2 {
		t.Errorf("expected level2 depth 2, got %d", level2.Depth)
	}
	if level2.ElementCount == nil || *level2.ElementCount != 3 {
		t.Errorf("expected level2 elementCount 3, got %v", level2.ElementCount)
	}
	if level2.Array == nil {
		t.Fatal("expected level2 nested array")
	}

	// Level 3
	level3 := level2.Array
	if level3.Depth != 3 {
		t.Errorf("expected level3 depth 3, got %d", level3.Depth)
	}
	if level3.ElementCount == nil || *level3.ElementCount != 2 {
		t.Errorf("expected level3 elementCount 2, got %v", level3.ElementCount)
	}
	if level3.ElementType != "f32" {
		t.Errorf("expected level3 elementType 'f32', got '%s'", level3.ElementType)
	}
	if level3.Array != nil {
		t.Error("expected level3 nested array to be nil")
	}
}

func TestArrayBindingVec3Stride(t *testing.T) {
	// array<vec3f, N> has stride 16 (vec3f alignment), not 12 (vec3f size)
	source := `@group(0) @binding(0) var<storage> data: array<vec3f, 10>;`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Array == nil {
		t.Fatal("expected array info to be set")
	}

	arr := binding.Array
	// vec3f: size=12, alignment=16, so stride=16
	if arr.ElementStride != 16 {
		t.Errorf("expected elementStride 16 (vec3f alignment), got %d", arr.ElementStride)
	}
	if arr.TotalSize == nil || *arr.TotalSize != 160 {
		t.Errorf("expected totalSize 160, got %v", arr.TotalSize)
	}
}

func TestFieldMappedNames(t *testing.T) {
	source := `
struct MyStruct {
	position: vec3f,
	velocity: vec4f,
}
@group(0) @binding(0) var<uniform> u: MyStruct;
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Layout == nil {
		t.Fatal("expected layout to be set")
	}

	// Check that mapped names are same as original (no minification)
	for _, field := range binding.Layout.Fields {
		if field.Name != field.NameMapped {
			t.Errorf("expected field name '%s' to equal nameMapped '%s'", field.Name, field.NameMapped)
		}
		if field.Type != field.TypeMapped {
			t.Errorf("expected field type '%s' to equal typeMapped '%s'", field.Type, field.TypeMapped)
		}
	}
}

// --- Edge Case and Error Handling Tests ---

func TestArrayBindingNoGroupBinding(t *testing.T) {
	// Array without @group/@binding should not appear in bindings
	source := `var<private> data: array<f32, 10>;`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 0 {
		t.Errorf("expected 0 bindings for private var, got %d", len(result.Bindings))
	}
}

func TestArrayBindingUniformAddressSpace(t *testing.T) {
	// Arrays can be in uniform address space too
	source := `
struct Data { values: array<f32, 4> }
@group(0) @binding(0) var<uniform> u: Data;
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.AddressSpace != "uniform" {
		t.Errorf("expected addressSpace 'uniform', got '%s'", binding.AddressSpace)
	}
	// This is a struct containing an array, not an array binding
	if binding.Array != nil {
		t.Error("expected array to be nil (struct type, not array type)")
	}
	if binding.Layout == nil {
		t.Fatal("expected layout to be set for struct type")
	}
}

func TestArrayBindingAtomicElements(t *testing.T) {
	// array<atomic<u32>, N>
	source := `@group(0) @binding(0) var<storage, read_write> counters: array<atomic<u32>, 64>;`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Array == nil {
		t.Fatal("expected array info to be set")
	}

	arr := binding.Array
	if arr.ElementType != "atomic<u32>" {
		t.Errorf("expected elementType 'atomic<u32>', got '%s'", arr.ElementType)
	}
	// atomic<u32> has same layout as u32: size=4, align=4
	if arr.ElementStride != 4 {
		t.Errorf("expected elementStride 4, got %d", arr.ElementStride)
	}
}

func TestArrayBindingMat4x4Elements(t *testing.T) {
	// array<mat4x4f, N> - common for bone matrices
	source := `@group(0) @binding(0) var<storage> bones: array<mat4x4f, 100>;`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Array == nil {
		t.Fatal("expected array info to be set")
	}

	arr := binding.Array
	if arr.ElementType != "mat4x4f" {
		t.Errorf("expected elementType 'mat4x4f', got '%s'", arr.ElementType)
	}
	// mat4x4f: size=64, align=16, stride=64
	if arr.ElementStride != 64 {
		t.Errorf("expected elementStride 64, got %d", arr.ElementStride)
	}
	if arr.TotalSize == nil || *arr.TotalSize != 6400 {
		t.Errorf("expected totalSize 6400, got %v", arr.TotalSize)
	}
}

func TestArrayBindingMixedWithOtherBindings(t *testing.T) {
	// Multiple bindings including arrays and non-arrays
	source := `
struct Uniforms { time: f32 }
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var<storage> positions: array<vec4f, 1000>;
@group(0) @binding(2) var texSampler: sampler;
@group(0) @binding(3) var<storage, read_write> velocities: array<vec4f>;
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 4 {
		t.Fatalf("expected 4 bindings, got %d", len(result.Bindings))
	}

	// Find bindings by name
	bindings := make(map[string]*BindingInfo)
	for i := range result.Bindings {
		b := &result.Bindings[i]
		bindings[b.Name] = b
	}

	// uniforms - struct, not array
	if u, ok := bindings["uniforms"]; ok {
		if u.Array != nil {
			t.Error("uniforms: expected array to be nil")
		}
		if u.Layout == nil {
			t.Error("uniforms: expected layout to be set")
		}
	}

	// positions - fixed-size array
	if p, ok := bindings["positions"]; ok {
		if p.Array == nil {
			t.Fatal("positions: expected array info")
		}
		if p.Array.ElementCount == nil || *p.Array.ElementCount != 1000 {
			t.Errorf("positions: expected elementCount 1000")
		}
		if p.Layout != nil {
			t.Error("positions: expected layout to be nil for array type")
		}
	}

	// texSampler - sampler, no array
	if s, ok := bindings["texSampler"]; ok {
		if s.Array != nil {
			t.Error("texSampler: expected array to be nil")
		}
	}

	// velocities - runtime-sized array
	if v, ok := bindings["velocities"]; ok {
		if v.Array == nil {
			t.Fatal("velocities: expected array info")
		}
		if v.Array.ElementCount != nil {
			t.Errorf("velocities: expected elementCount nil for runtime-sized")
		}
	}
}

func TestArrayBindingEmptyStruct(t *testing.T) {
	// Edge case: array of empty struct (size 0)
	// This is technically invalid WGSL but parser might allow it
	source := `
struct Empty {}
@group(0) @binding(0) var<storage> data: array<Empty, 10>;
`
	result := Reflect(source)

	// This might produce parse errors, which is fine
	// The main goal is not to crash
	if len(result.Errors) > 0 {
		// Expected - empty structs are not valid
		return
	}

	// If it did parse, check we don't crash
	if len(result.Bindings) > 0 && result.Bindings[0].Array != nil {
		// Check it doesn't panic accessing the array info
		_ = result.Bindings[0].Array.ElementStride
	}
}

func TestArrayBindingNestedStructInArray(t *testing.T) {
	// array<Outer> where Outer contains Inner struct
	source := `
struct Inner {
	x: f32,
	y: f32,
}
struct Outer {
	a: f32,
	inner: Inner,
	b: f32,
}
@group(0) @binding(0) var<storage> data: array<Outer, 100>;
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]
	if binding.Array == nil {
		t.Fatal("expected array info to be set")
	}

	arr := binding.Array
	if arr.ElementLayout == nil {
		t.Fatal("expected elementLayout for struct type")
	}

	// Outer: a(4) + Inner(8) + b(4) = 16 bytes
	if arr.ElementLayout.Size != 16 {
		t.Errorf("expected elementLayout size 16, got %d", arr.ElementLayout.Size)
	}

	// Check Inner field has nested layout
	if len(arr.ElementLayout.Fields) != 3 {
		t.Fatalf("expected 3 fields in Outer, got %d", len(arr.ElementLayout.Fields))
	}
	innerField := arr.ElementLayout.Fields[1]
	if innerField.Name != "inner" {
		t.Errorf("expected field 1 to be 'inner', got '%s'", innerField.Name)
	}
	if innerField.Layout == nil {
		t.Error("expected nested layout for Inner field")
	}
}

func TestParseErrorsInArrayBinding(t *testing.T) {
	// Invalid WGSL should produce errors
	testCases := []struct {
		name   string
		source string
	}{
		{"missing_element_type", "@group(0) @binding(0) var<storage> data: array<>;"},
		{"invalid_syntax", "@group(0) @binding(0) var<storage> data: array<f32,>;"},
		{"unclosed_bracket", "@group(0) @binding(0) var<storage> data: array<f32, 10"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := Reflect(tc.source)
			// We expect parse errors, not panics
			if len(result.Errors) == 0 {
				t.Error("expected parse errors for invalid syntax")
			}
		})
	}
}

func TestArrayBindingZeroSize(t *testing.T) {
	// array<f32, 0> - zero-size array
	source := `@group(0) @binding(0) var<storage> data: array<f32, 0>;`
	result := Reflect(source)

	// This might be a parse error or produce elementCount=0
	// Main goal: don't crash
	if len(result.Errors) > 0 {
		return // Parse error is acceptable
	}

	if len(result.Bindings) == 1 && result.Bindings[0].Array != nil {
		arr := result.Bindings[0].Array
		if arr.ElementCount != nil && *arr.ElementCount == 0 {
			// Zero-size arrays should have totalSize=0
			if arr.TotalSize == nil || *arr.TotalSize != 0 {
				t.Errorf("expected totalSize 0 for zero-size array")
			}
		}
	}
}

func TestArrayBindingLargeCount(t *testing.T) {
	// Large array count
	source := `@group(0) @binding(0) var<storage> data: array<f32, 1000000>;`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	arr := result.Bindings[0].Array
	if arr == nil {
		t.Fatal("expected array info")
	}
	if arr.ElementCount == nil || *arr.ElementCount != 1000000 {
		t.Errorf("expected elementCount 1000000")
	}
	if arr.TotalSize == nil || *arr.TotalSize != 4000000 {
		t.Errorf("expected totalSize 4000000")
	}
}

// --- Real Shader Tests ---

func TestRealShaderWithArrayOfStructs(t *testing.T) {
	// Based on starsParticlesModule.wgsl pattern
	source := `
struct StarParticle {
  pos : vec4f,
  vel : vec4f,
}

struct StarsParticles {
  particles : array<StarParticle>,
}

struct StarsSimParams {
  deltaT: f32,
  simId: f32,
  rule1Distance: f32,
  rule2Distance: f32,
  rule3Distance: f32,
  rule1Scale: f32,
  rule2Scale: f32,
  rule3Scale: f32,
}

@binding(0) @group(0) var<storage, read> particlesA : StarsParticles;
@binding(1) @group(0) var<storage, read_write> particlesB : StarsParticles;
@binding(2) @group(0) var<uniform> params : StarsSimParams;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id : vec3u) {
  let index = id.x;
  particlesB.particles[index].pos = particlesA.particles[index].pos;
}
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Should have 3 bindings
	if len(result.Bindings) != 3 {
		t.Fatalf("expected 3 bindings, got %d", len(result.Bindings))
	}

	// Find bindings by name
	bindings := make(map[string]*BindingInfo)
	for i := range result.Bindings {
		b := &result.Bindings[i]
		bindings[b.Name] = b
	}

	// Check particlesA - storage binding with struct containing runtime-sized array
	if pA, ok := bindings["particlesA"]; ok {
		if pA.AddressSpace != "storage" {
			t.Errorf("particlesA: expected addressSpace 'storage', got '%s'", pA.AddressSpace)
		}
		if pA.Type != "StarsParticles" {
			t.Errorf("particlesA: expected type 'StarsParticles', got '%s'", pA.Type)
		}
		// This is a struct type, not an array type
		if pA.Array != nil {
			t.Error("particlesA: expected array to be nil (struct type)")
		}
		if pA.Layout == nil {
			t.Fatal("particlesA: expected layout to be set")
		}
		// StarsParticles has one field: particles (runtime-sized array)
		if len(pA.Layout.Fields) != 1 {
			t.Errorf("particlesA layout: expected 1 field, got %d", len(pA.Layout.Fields))
		}
	} else {
		t.Error("particlesA binding not found")
	}

	// Check params - uniform binding with struct
	if p, ok := bindings["params"]; ok {
		if p.AddressSpace != "uniform" {
			t.Errorf("params: expected addressSpace 'uniform', got '%s'", p.AddressSpace)
		}
		if p.Layout == nil {
			t.Fatal("params: expected layout to be set")
		}
		// StarsSimParams has 8 f32 fields
		if len(p.Layout.Fields) != 8 {
			t.Errorf("params layout: expected 8 fields, got %d", len(p.Layout.Fields))
		}
		// Size should be 8 * 4 = 32 bytes
		if p.Layout.Size != 32 {
			t.Errorf("params layout: expected size 32, got %d", p.Layout.Size)
		}
	} else {
		t.Error("params binding not found")
	}

	// Check entry point
	if len(result.EntryPoints) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(result.EntryPoints))
	}
	ep := result.EntryPoints[0]
	if ep.Name != "main" {
		t.Errorf("expected entry point name 'main', got '%s'", ep.Name)
	}
	if ep.Stage != "compute" {
		t.Errorf("expected stage 'compute', got '%s'", ep.Stage)
	}
	if len(ep.WorkgroupSize) != 3 || ep.WorkgroupSize[0] != 64 {
		t.Errorf("expected workgroup size [64,1,1], got %v", ep.WorkgroupSize)
	}
}

func TestRealShaderDirectArrayBinding(t *testing.T) {
	// Direct array binding (not wrapped in struct)
	source := `
struct Particle {
  position: vec4f,
  velocity: vec4f,
  color: vec4f,
}

@group(0) @binding(0) var<storage, read> inputParticles: array<Particle>;
@group(0) @binding(1) var<storage, read_write> outputParticles: array<Particle>;
@group(0) @binding(2) var<storage> fixedParticles: array<Particle, 1000>;

@compute @workgroup_size(256)
fn simulate(@builtin(global_invocation_id) id: vec3u) {
  let i = id.x;
  outputParticles[i] = inputParticles[i];
}
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 3 {
		t.Fatalf("expected 3 bindings, got %d", len(result.Bindings))
	}

	// Find bindings by name
	bindings := make(map[string]*BindingInfo)
	for i := range result.Bindings {
		b := &result.Bindings[i]
		bindings[b.Name] = b
	}

	// Check inputParticles - runtime-sized array
	if inp, ok := bindings["inputParticles"]; ok {
		if inp.Array == nil {
			t.Fatal("inputParticles: expected array info to be set")
		}
		if inp.Array.ElementCount != nil {
			t.Error("inputParticles: expected elementCount nil for runtime-sized")
		}
		if inp.Array.ElementType != "Particle" {
			t.Errorf("inputParticles: expected elementType 'Particle', got '%s'", inp.Array.ElementType)
		}
		if inp.Array.ElementLayout == nil {
			t.Fatal("inputParticles: expected elementLayout for struct type")
		}
		// Particle: 3 * vec4f = 3 * 16 = 48 bytes
		if inp.Array.ElementLayout.Size != 48 {
			t.Errorf("inputParticles elementLayout: expected size 48, got %d", inp.Array.ElementLayout.Size)
		}
		if inp.Array.ElementStride != 48 {
			t.Errorf("inputParticles: expected elementStride 48, got %d", inp.Array.ElementStride)
		}
		if inp.Layout != nil {
			t.Error("inputParticles: expected layout nil for array type")
		}
	} else {
		t.Error("inputParticles binding not found")
	}

	// Check fixedParticles - fixed-size array
	if fixed, ok := bindings["fixedParticles"]; ok {
		if fixed.Array == nil {
			t.Fatal("fixedParticles: expected array info to be set")
		}
		if fixed.Array.ElementCount == nil || *fixed.Array.ElementCount != 1000 {
			t.Errorf("fixedParticles: expected elementCount 1000")
		}
		if fixed.Array.TotalSize == nil || *fixed.Array.TotalSize != 48000 {
			t.Errorf("fixedParticles: expected totalSize 48000, got %v", fixed.Array.TotalSize)
		}
		if fixed.Array.Depth != 1 {
			t.Errorf("fixedParticles: expected depth 1, got %d", fixed.Array.Depth)
		}
	} else {
		t.Error("fixedParticles binding not found")
	}
}

func TestRealShaderComplexTypes(t *testing.T) {
	// Test with various complex types including textures and samplers
	source := `
struct Camera {
  view: mat4x4f,
  projection: mat4x4f,
  position: vec3f,
  _pad: f32,
}

struct Light {
  color: vec3f,
  intensity: f32,
  position: vec3f,
  range: f32,
}

@group(0) @binding(0) var<uniform> camera: Camera;
@group(0) @binding(1) var<storage> lights: array<Light, 16>;
@group(1) @binding(0) var albedoTexture: texture_2d<f32>;
@group(1) @binding(1) var normalTexture: texture_2d<f32>;
@group(1) @binding(2) var texSampler: sampler;

@fragment
fn main() -> @location(0) vec4f {
  return vec4f(1.0);
}
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 5 {
		t.Fatalf("expected 5 bindings, got %d", len(result.Bindings))
	}

	// Find bindings
	bindings := make(map[string]*BindingInfo)
	for i := range result.Bindings {
		b := &result.Bindings[i]
		bindings[b.Name] = b
	}

	// Check camera - uniform struct with matrices
	if cam, ok := bindings["camera"]; ok {
		if cam.Layout == nil {
			t.Fatal("camera: expected layout to be set")
		}
		// Camera: 2 * mat4x4f(64) + vec3f(12 padded to 16) + f32(4) = 128 + 16 + 4 = 148?
		// Actually: view(64) + projection(64) + position(12, align 16 so offset 128) + _pad(4)
		// Total: 144, rounded to alignment 16 = 144
		if cam.Layout.Size != 144 {
			t.Errorf("camera layout: expected size 144, got %d", cam.Layout.Size)
		}
	}

	// Check lights - fixed-size array of structs
	if lights, ok := bindings["lights"]; ok {
		if lights.Array == nil {
			t.Fatal("lights: expected array info")
		}
		if lights.Array.ElementCount == nil || *lights.Array.ElementCount != 16 {
			t.Error("lights: expected elementCount 16")
		}
		if lights.Array.ElementLayout == nil {
			t.Fatal("lights: expected elementLayout")
		}
		// Light: color(vec3f, 12 @ align 16) + intensity(4) + position(vec3f, 12 @ align 16) + range(4)
		// color: offset 0, size 12, align 16
		// intensity: offset 12, size 4, align 4
		// position: offset 16, size 12, align 16
		// range: offset 28, size 4, align 4
		// Total: 32, align 16
		if lights.Array.ElementLayout.Size != 32 {
			t.Errorf("lights elementLayout: expected size 32, got %d", lights.Array.ElementLayout.Size)
		}
		if lights.Array.ElementStride != 32 {
			t.Errorf("lights: expected elementStride 32, got %d", lights.Array.ElementStride)
		}
	}

	// Check texture and sampler have no array info
	if tex, ok := bindings["albedoTexture"]; ok {
		if tex.Array != nil {
			t.Error("albedoTexture: expected array to be nil for texture type")
		}
		if tex.AddressSpace != "handle" {
			t.Errorf("albedoTexture: expected addressSpace 'handle', got '%s'", tex.AddressSpace)
		}
	}

	if samp, ok := bindings["texSampler"]; ok {
		if samp.Array != nil {
			t.Error("texSampler: expected array to be nil for sampler type")
		}
		if samp.Type != "sampler" {
			t.Errorf("texSampler: expected type 'sampler', got '%s'", samp.Type)
		}
	}

	// Check entry point is fragment shader
	if len(result.EntryPoints) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(result.EntryPoints))
	}
	if result.EntryPoints[0].Stage != "fragment" {
		t.Errorf("expected stage 'fragment', got '%s'", result.EntryPoints[0].Stage)
	}
}

// ----------------------------------------------------------------------------
// Mapped Name Tests
// ----------------------------------------------------------------------------

// mockRenamer is a simple renamer for testing that maps symbol refs to short names.
type mockRenamer struct {
	nameMap map[uint32]string
}

func (m *mockRenamer) NameForSymbol(ref ast.Ref) string {
	if name, ok := m.nameMap[ref.InnerIndex]; ok {
		return name
	}
	return ""
}

func TestMappedNamesWithoutRenamer(t *testing.T) {
	source := `
struct MyStruct {
    position: vec3f,
    color: vec4f,
}
@group(0) @binding(0) var<storage> data: array<MyStruct>;
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Without renamer, mapped names should equal original names
	binding := result.Bindings[0]
	if binding.Name != binding.NameMapped {
		t.Errorf("without renamer: Name '%s' should equal NameMapped '%s'", binding.Name, binding.NameMapped)
	}
	if binding.Type != binding.TypeMapped {
		t.Errorf("without renamer: Type '%s' should equal TypeMapped '%s'", binding.Type, binding.TypeMapped)
	}

	// Check array element mapped names
	if binding.Array == nil {
		t.Fatal("expected array info")
	}
	if binding.Array.ElementType != binding.Array.ElementTypeMapped {
		t.Errorf("without renamer: ElementType '%s' should equal ElementTypeMapped '%s'",
			binding.Array.ElementType, binding.Array.ElementTypeMapped)
	}

	// Check struct field mapped names
	structLayout, ok := result.Structs["MyStruct"]
	if !ok {
		t.Fatal("expected MyStruct in structs map")
	}
	for _, field := range structLayout.Fields {
		if field.Name != field.NameMapped {
			t.Errorf("without renamer: field Name '%s' should equal NameMapped '%s'", field.Name, field.NameMapped)
		}
		if field.Type != field.TypeMapped {
			t.Errorf("without renamer: field Type '%s' should equal TypeMapped '%s'", field.Type, field.TypeMapped)
		}
	}
}

func TestMappedNamesWithRenamer(t *testing.T) {
	source := `
struct Particle {
    position: vec3f,
    velocity: vec3f,
}
@group(0) @binding(0) var<storage> particles: array<Particle>;
@compute @workgroup_size(64)
fn main() {}
`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// Create a mock renamer that maps symbols to short names
	// We need to find the symbol indices for Particle struct and particles binding
	renameMap := make(map[uint32]string)
	for i, sym := range module.Symbols {
		switch sym.OriginalName {
		case "Particle":
			renameMap[uint32(i)] = "a" // Particle -> a
		case "particles":
			renameMap[uint32(i)] = "b" // particles -> b
		case "position":
			renameMap[uint32(i)] = "c" // position -> c
		case "velocity":
			renameMap[uint32(i)] = "d" // velocity -> d
		}
	}
	renamer := &mockRenamer{nameMap: renameMap}

	result := ReflectModuleWithRenamer(module, renamer)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Check binding mapped names
	binding := result.Bindings[0]
	if binding.Name != "particles" {
		t.Errorf("expected Name 'particles', got '%s'", binding.Name)
	}
	if binding.NameMapped != "b" {
		t.Errorf("expected NameMapped 'b', got '%s'", binding.NameMapped)
	}

	// Type should show original, TypeMapped should show minified
	if binding.Type != "array<Particle>" {
		t.Errorf("expected Type 'array<Particle>', got '%s'", binding.Type)
	}
	if binding.TypeMapped != "array<a>" {
		t.Errorf("expected TypeMapped 'array<a>', got '%s'", binding.TypeMapped)
	}

	// Check array element type mapping
	if binding.Array == nil {
		t.Fatal("expected array info")
	}
	if binding.Array.ElementType != "Particle" {
		t.Errorf("expected ElementType 'Particle', got '%s'", binding.Array.ElementType)
	}
	if binding.Array.ElementTypeMapped != "a" {
		t.Errorf("expected ElementTypeMapped 'a', got '%s'", binding.Array.ElementTypeMapped)
	}

	// Check struct field mapped names
	structLayout, ok := result.Structs["Particle"]
	if !ok {
		t.Fatal("expected Particle in structs map")
	}

	// Find position field
	var posField *FieldInfo
	for i := range structLayout.Fields {
		if structLayout.Fields[i].Name == "position" {
			posField = &structLayout.Fields[i]
			break
		}
	}
	if posField == nil {
		t.Fatal("expected position field")
	}
	if posField.NameMapped != "c" {
		t.Errorf("expected position NameMapped 'c', got '%s'", posField.NameMapped)
	}
}

func TestMappedNamesNestedArray(t *testing.T) {
	source := `
struct Inner {
    value: f32,
}
@group(0) @binding(0) var<storage> data: array<array<Inner, 4> >;
`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// Create renamer that maps Inner -> x
	renameMap := make(map[uint32]string)
	for i, sym := range module.Symbols {
		if sym.OriginalName == "Inner" {
			renameMap[uint32(i)] = "x"
		}
	}
	renamer := &mockRenamer{nameMap: renameMap}

	result := ReflectModuleWithRenamer(module, renamer)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	binding := result.Bindings[0]

	// Outer array element type should be mapped
	if binding.Array == nil {
		t.Fatal("expected array info")
	}
	if binding.Array.ElementType != "array<Inner, 4>" {
		t.Errorf("expected ElementType 'array<Inner, 4>', got '%s'", binding.Array.ElementType)
	}
	if binding.Array.ElementTypeMapped != "array<x, 4>" {
		t.Errorf("expected ElementTypeMapped 'array<x, 4>', got '%s'", binding.Array.ElementTypeMapped)
	}

	// Nested array element type should also be mapped
	if binding.Array.Array == nil {
		t.Fatal("expected nested array info")
	}
	if binding.Array.Array.ElementType != "Inner" {
		t.Errorf("expected nested ElementType 'Inner', got '%s'", binding.Array.Array.ElementType)
	}
	if binding.Array.Array.ElementTypeMapped != "x" {
		t.Errorf("expected nested ElementTypeMapped 'x', got '%s'", binding.Array.Array.ElementTypeMapped)
	}
}
