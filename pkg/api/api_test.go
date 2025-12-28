package api

import (
	"strings"
	"testing"
)

func TestMinifyAndReflect(t *testing.T) {
	source := `
struct Particle {
    position: vec3f,
    velocity: vec3f,
    lifetime: f32,
}

@group(0) @binding(0) var<storage, read_write> particles: array<Particle>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3u) {
    let idx = id.x;
    particles[idx].position += particles[idx].velocity;
}
`

	result := MinifyAndReflect(source)

	// Check no errors
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if len(result.Reflect.Errors) > 0 {
		t.Fatalf("unexpected reflect errors: %v", result.Reflect.Errors)
	}

	// Check minified code is smaller
	if result.MinifiedSize >= result.OriginalSize {
		t.Errorf("expected minified size < original, got %d >= %d", result.MinifiedSize, result.OriginalSize)
	}

	// Check bindings
	if len(result.Reflect.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Reflect.Bindings))
	}

	binding := result.Reflect.Bindings[0]

	// Original name should be preserved
	if binding.Name != "particles" {
		t.Errorf("expected Name 'particles', got '%s'", binding.Name)
	}

	// Mapped name should be different (minified)
	// Since external bindings are not mangled by default, NameMapped should equal Name
	// But the type should be minified
	if binding.Type != "array<Particle>" {
		t.Errorf("expected Type 'array<Particle>', got '%s'", binding.Type)
	}

	// TypeMapped should show minified struct name
	// The minified name will be short (like "a" or similar)
	if binding.TypeMapped == "array<Particle>" {
		t.Errorf("expected TypeMapped to be minified, got '%s'", binding.TypeMapped)
	}
	if !strings.HasPrefix(binding.TypeMapped, "array<") {
		t.Errorf("expected TypeMapped to start with 'array<', got '%s'", binding.TypeMapped)
	}

	// Check array element info
	if binding.Array == nil {
		t.Fatal("expected array info")
	}
	if binding.Array.ElementType != "Particle" {
		t.Errorf("expected ElementType 'Particle', got '%s'", binding.Array.ElementType)
	}
	// ElementTypeMapped should be minified
	if binding.Array.ElementTypeMapped == "Particle" {
		t.Errorf("expected ElementTypeMapped to be minified, got '%s'", binding.Array.ElementTypeMapped)
	}

	// Check struct layout fields have mapped names
	if binding.Array.ElementLayout == nil {
		t.Fatal("expected ElementLayout")
	}
	if len(binding.Array.ElementLayout.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(binding.Array.ElementLayout.Fields))
	}

	// Find position field and check it has a mapped name
	var positionField *FieldInfo
	for i := range binding.Array.ElementLayout.Fields {
		if binding.Array.ElementLayout.Fields[i].Name == "position" {
			positionField = &binding.Array.ElementLayout.Fields[i]
			break
		}
	}
	if positionField == nil {
		t.Fatal("expected position field")
	}
	// Field names are not renamed by default (MangleProps is false)
	// So NameMapped should equal Name
	if positionField.Name != positionField.NameMapped {
		t.Errorf("expected field NameMapped to equal Name when MangleProps is false, got Name='%s' NameMapped='%s'",
			positionField.Name, positionField.NameMapped)
	}

	// Check entry points
	if len(result.Reflect.EntryPoints) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(result.Reflect.EntryPoints))
	}
	if result.Reflect.EntryPoints[0].Name != "main" {
		t.Errorf("expected entry point name 'main', got '%s'", result.Reflect.EntryPoints[0].Name)
	}
	if result.Reflect.EntryPoints[0].Stage != "compute" {
		t.Errorf("expected stage 'compute', got '%s'", result.Reflect.EntryPoints[0].Stage)
	}
}

func TestMinifyAndReflectWithMangleBindings(t *testing.T) {
	source := `
struct Data { value: f32 }
@group(0) @binding(0) var<uniform> myData: Data;
@fragment fn main() -> @location(0) vec4f { return vec4f(myData.value); }
`

	result := MinifyAndReflectWithOptions(source, MinifyOptions{
		MinifyWhitespace:       true,
		MinifyIdentifiers:      true,
		MangleExternalBindings: true,
	})

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	binding := result.Reflect.Bindings[0]

	// With MangleExternalBindings, both Name and NameMapped should differ
	if binding.Name != "myData" {
		t.Errorf("expected Name 'myData', got '%s'", binding.Name)
	}
	// NameMapped should be minified
	if binding.NameMapped == "myData" {
		t.Errorf("expected NameMapped to be minified with MangleExternalBindings, got '%s'", binding.NameMapped)
	}
}

func TestMinifyAndReflectWithoutMinifyIdentifiers(t *testing.T) {
	source := `
struct Particle { pos: vec3f }
@group(0) @binding(0) var<storage> data: array<Particle>;
@compute @workgroup_size(1) fn main() {}
`

	result := MinifyAndReflectWithOptions(source, MinifyOptions{
		MinifyWhitespace:  true,
		MinifyIdentifiers: false,
	})

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	binding := result.Reflect.Bindings[0]

	// Without MinifyIdentifiers, all mapped names should equal original names
	if binding.Name != binding.NameMapped {
		t.Errorf("without MinifyIdentifiers: Name '%s' should equal NameMapped '%s'", binding.Name, binding.NameMapped)
	}
	if binding.Type != binding.TypeMapped {
		t.Errorf("without MinifyIdentifiers: Type '%s' should equal TypeMapped '%s'", binding.Type, binding.TypeMapped)
	}
	if binding.Array != nil {
		if binding.Array.ElementType != binding.Array.ElementTypeMapped {
			t.Errorf("without MinifyIdentifiers: ElementType '%s' should equal ElementTypeMapped '%s'",
				binding.Array.ElementType, binding.Array.ElementTypeMapped)
		}
	}
}

// Tests for standalone Minify functions

func TestMinify(t *testing.T) {
	source := `
fn addNumbers(a: f32, b: f32) -> f32 {
    return a + b;
}

fn main() {
    let x = addNumbers(1.0, 2.0);
}
`
	result := Minify(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Check that output is smaller
	if result.MinifiedSize >= result.OriginalSize {
		t.Errorf("expected minified size < original, got %d >= %d", result.MinifiedSize, result.OriginalSize)
	}

	// Check that code is not empty
	if len(result.Code) == 0 {
		t.Error("expected non-empty minified code")
	}

	// Check that function name is renamed (functions are always renamed)
	if strings.Contains(result.Code, "addNumbers") {
		t.Error("expected 'addNumbers' function to be renamed")
	}
}

func TestMinifyWithOptions(t *testing.T) {
	source := `
fn compute(value: f32) -> f32 {
    let temp = value * 2.0;
    return temp;
}
`

	// Test with only whitespace minification
	result := MinifyWithOptions(source, MinifyOptions{
		MinifyWhitespace: true,
	})

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// With whitespace only, identifiers should be preserved
	if !strings.Contains(result.Code, "compute") {
		t.Error("expected 'compute' to be preserved with whitespace-only minification")
	}
	if !strings.Contains(result.Code, "temp") {
		t.Error("expected 'temp' to be preserved with whitespace-only minification")
	}

	// Test with both whitespace and identifier minification
	result2 := MinifyWithOptions(source, MinifyOptions{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	})

	if len(result2.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result2.Errors)
	}

	// With identifier minification, internal names should be renamed
	if strings.Contains(result2.Code, "temp") {
		t.Error("expected 'temp' to be renamed with identifier minification")
	}
}

func TestMinifyWhitespaceOnly(t *testing.T) {
	source := `
fn longFunctionName(inputParameter: f32) -> f32 {
    let intermediateValue = inputParameter * 3.14159;
    return intermediateValue;
}
`
	result := MinifyWhitespaceOnly(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// All original names should be preserved
	if !strings.Contains(result.Code, "longFunctionName") {
		t.Error("expected 'longFunctionName' to be preserved")
	}
	if !strings.Contains(result.Code, "inputParameter") {
		t.Error("expected 'inputParameter' to be preserved")
	}
	if !strings.Contains(result.Code, "intermediateValue") {
		t.Error("expected 'intermediateValue' to be preserved")
	}

	// But whitespace should be reduced
	if strings.Contains(result.Code, "    ") {
		t.Error("expected excessive whitespace to be removed")
	}
}

func TestMinifyWithSourceMap(t *testing.T) {
	source := `fn main() { let x = 1; }`

	result := MinifyWithOptions(source, MinifyOptions{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		SourceMap:         true,
		SourceMapOptions: SourceMapOptions{
			File:          "output.wgsl",
			SourceName:    "input.wgsl",
			IncludeSource: true,
		},
	})

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Check that source map was generated
	if result.SourceMap == "" {
		t.Error("expected SourceMap to be non-empty")
	}
	if result.SourceMapDataURI == "" {
		t.Error("expected SourceMapDataURI to be non-empty")
	}

	// Verify source map is valid JSON
	if !strings.HasPrefix(result.SourceMap, "{") {
		t.Error("expected SourceMap to be valid JSON")
	}

	// Verify data URI format
	if !strings.HasPrefix(result.SourceMapDataURI, "data:application/json;base64,") {
		t.Error("expected SourceMapDataURI to have correct prefix")
	}
}

func TestMinifyWithKeepNames(t *testing.T) {
	source := `
fn importantFunction(a: f32) -> f32 {
    let temp = a * 2.0;
    return temp;
}

fn helper(b: f32) -> f32 {
    return importantFunction(b);
}
`
	result := MinifyWithOptions(source, MinifyOptions{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		KeepNames:         []string{"importantFunction"},
	})

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// importantFunction should be preserved
	if !strings.Contains(result.Code, "importantFunction") {
		t.Error("expected 'importantFunction' to be preserved via keepNames")
	}

	// helper should be renamed
	if strings.Contains(result.Code, "helper") {
		t.Error("expected 'helper' to be renamed")
	}
}

func TestMinifyWithErrors(t *testing.T) {
	// Invalid WGSL syntax
	source := `fn main( { }`

	result := Minify(source)

	// Should have errors
	if len(result.Errors) == 0 {
		t.Error("expected errors for invalid syntax")
	}
}

// Tests for standalone Reflect function

func TestReflect(t *testing.T) {
	source := `
struct Light {
    position: vec3f,
    intensity: f32,
}

@group(0) @binding(0) var<uniform> light: Light;
@group(0) @binding(1) var texture: texture_2d<f32>;
@group(0) @binding(2) var texSampler: sampler;

@fragment
fn main() -> @location(0) vec4f {
    return vec4f(light.intensity);
}
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Check bindings
	if len(result.Bindings) != 3 {
		t.Fatalf("expected 3 bindings, got %d", len(result.Bindings))
	}

	// Find light binding
	var lightBinding *BindingInfo
	for i := range result.Bindings {
		if result.Bindings[i].Name == "light" {
			lightBinding = &result.Bindings[i]
			break
		}
	}
	if lightBinding == nil {
		t.Fatal("expected to find 'light' binding")
	}
	if lightBinding.Group != 0 || lightBinding.Binding != 0 {
		t.Errorf("expected light at group 0 binding 0, got group %d binding %d", lightBinding.Group, lightBinding.Binding)
	}
	if lightBinding.AddressSpace != "uniform" {
		t.Errorf("expected uniform address space, got '%s'", lightBinding.AddressSpace)
	}

	// Check that struct layout is provided for light
	if lightBinding.Layout == nil {
		t.Error("expected layout for struct binding")
	} else {
		if len(lightBinding.Layout.Fields) != 2 {
			t.Errorf("expected 2 fields in Light struct, got %d", len(lightBinding.Layout.Fields))
		}
	}

	// Check structs map
	if _, ok := result.Structs["Light"]; !ok {
		t.Error("expected Light struct in Structs map")
	}

	// Check entry points
	if len(result.EntryPoints) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(result.EntryPoints))
	}
	if result.EntryPoints[0].Name != "main" {
		t.Errorf("expected entry point 'main', got '%s'", result.EntryPoints[0].Name)
	}
	if result.EntryPoints[0].Stage != "fragment" {
		t.Errorf("expected fragment stage, got '%s'", result.EntryPoints[0].Stage)
	}
}

func TestReflectComputeShader(t *testing.T) {
	source := `
@compute @workgroup_size(16, 8, 4)
fn compute_main() {}
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.EntryPoints) != 1 {
		t.Fatalf("expected 1 entry point, got %d", len(result.EntryPoints))
	}

	ep := result.EntryPoints[0]
	if ep.Stage != "compute" {
		t.Errorf("expected compute stage, got '%s'", ep.Stage)
	}

	if len(ep.WorkgroupSize) != 3 {
		t.Fatalf("expected 3 workgroup size components, got %d", len(ep.WorkgroupSize))
	}
	if ep.WorkgroupSize[0] != 16 || ep.WorkgroupSize[1] != 8 || ep.WorkgroupSize[2] != 4 {
		t.Errorf("expected workgroup_size(16, 8, 4), got %v", ep.WorkgroupSize)
	}
}

func TestReflectWithErrors(t *testing.T) {
	// Invalid WGSL
	source := `fn broken( {`

	result := Reflect(source)

	if len(result.Errors) == 0 {
		t.Error("expected errors for invalid syntax")
	}
}

func TestReflectArrayBinding(t *testing.T) {
	source := `
struct Item { value: f32 }
@group(0) @binding(0) var<storage, read_write> items: array<Item>;
@compute @workgroup_size(1) fn main() {}
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Bindings))
	}

	binding := result.Bindings[0]

	// Should have array info
	if binding.Array == nil {
		t.Fatal("expected array info for array binding")
	}

	if binding.Array.ElementType != "Item" {
		t.Errorf("expected ElementType 'Item', got '%s'", binding.Array.ElementType)
	}

	// Without minification, mapped names should equal original
	if binding.Array.ElementTypeMapped != "Item" {
		t.Errorf("expected ElementTypeMapped 'Item' (no minification), got '%s'", binding.Array.ElementTypeMapped)
	}

	// Should have element layout for struct
	if binding.Array.ElementLayout == nil {
		t.Error("expected element layout for struct array")
	}
}

func TestReflectTextureTypes(t *testing.T) {
	source := `
@group(0) @binding(0) var tex1: texture_2d<f32>;
@group(0) @binding(1) var tex2: texture_cube<f32>;
@group(0) @binding(2) var samp: sampler;
@group(0) @binding(3) var sampComp: sampler_comparison;
@fragment fn main() -> @location(0) vec4f { return vec4f(0.0); }
`
	result := Reflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Bindings) != 4 {
		t.Fatalf("expected 4 bindings, got %d", len(result.Bindings))
	}

	// Check address space for textures/samplers
	for _, b := range result.Bindings {
		if b.AddressSpace != "handle" {
			t.Errorf("expected 'handle' address space for %s, got '%s'", b.Name, b.AddressSpace)
		}
	}
}

func TestMinifyAndReflectWithSourceMap(t *testing.T) {
	source := `
struct Data { value: f32 }
@group(0) @binding(0) var<uniform> data: Data;
@fragment fn main() -> @location(0) vec4f { return vec4f(data.value); }
`
	result := MinifyAndReflectWithOptions(source, MinifyOptions{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		SourceMap:         true,
		SourceMapOptions: SourceMapOptions{
			File:          "out.wgsl",
			SourceName:    "in.wgsl",
			IncludeSource: true,
		},
	})

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Check source map was generated
	if result.SourceMap == "" {
		t.Error("expected SourceMap to be generated")
	}
	if result.SourceMapDataURI == "" {
		t.Error("expected SourceMapDataURI to be generated")
	}

	// Check reflection still works
	if len(result.Reflect.Bindings) != 1 {
		t.Errorf("expected 1 binding, got %d", len(result.Reflect.Bindings))
	}
}

func TestMinifyAndReflectWithErrors(t *testing.T) {
	// Invalid WGSL syntax
	source := `fn broken( {`

	result := MinifyAndReflect(source)

	// Should have errors
	if len(result.Errors) == 0 {
		t.Error("expected errors for invalid syntax")
	}
}

func TestMinifyAndReflectFieldLayout(t *testing.T) {
	// Test struct with nested struct field to exercise field layout conversion
	source := `
struct Inner { value: f32 }
struct Outer { inner: Inner }
@group(0) @binding(0) var<uniform> data: Outer;
@fragment fn main() -> @location(0) vec4f { return vec4f(data.inner.value); }
`
	result := MinifyAndReflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if len(result.Reflect.Bindings) != 1 {
		t.Fatalf("expected 1 binding, got %d", len(result.Reflect.Bindings))
	}

	binding := result.Reflect.Bindings[0]
	if binding.Layout == nil {
		t.Fatal("expected layout for struct binding")
	}

	// Find inner field and check it has nested layout
	var innerField *FieldInfo
	for i := range binding.Layout.Fields {
		if binding.Layout.Fields[i].Name == "inner" {
			innerField = &binding.Layout.Fields[i]
			break
		}
	}
	if innerField == nil {
		t.Fatal("expected 'inner' field")
	}
	if innerField.Layout == nil {
		t.Error("expected nested layout for struct field")
	}
}
