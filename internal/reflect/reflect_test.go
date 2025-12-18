package reflect

import (
	"testing"
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
