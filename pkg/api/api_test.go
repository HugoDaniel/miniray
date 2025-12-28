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
