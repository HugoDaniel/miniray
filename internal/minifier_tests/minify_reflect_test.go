// Package minifier_tests provides integration tests for the WGSL minifier.
package minifier_tests

import (
	"testing"

	"github.com/HugoDaniel/miniray/internal/minifier"
)

// TestMinifyAndReflect tests the combined minification and reflection functionality.
func TestMinifyAndReflect(t *testing.T) {
	source := `
struct Uniforms {
    time: f32,
    resolution: vec2f,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var texSampler: sampler;
@group(0) @binding(2) var texture: texture_2d<f32>;

@fragment
fn main(@location(0) uv: vec2f) -> @location(0) vec4f {
    let t = uniforms.time;
    return textureSample(texture, texSampler, uv);
}
`

	m := minifier.New(minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	})

	result := m.MinifyAndReflect(source)

	// Check that minification worked
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	if result.Code == "" {
		t.Error("expected minified code, got empty string")
	}

	if result.Stats.MinifiedSize >= result.Stats.OriginalSize {
		t.Errorf("expected minified size < original size, got %d >= %d",
			result.Stats.MinifiedSize, result.Stats.OriginalSize)
	}

	// Check that reflection worked
	if len(result.Reflect.Bindings) == 0 {
		t.Error("expected bindings in reflection result")
	}

	if len(result.Reflect.EntryPoints) == 0 {
		t.Error("expected entry points in reflection result")
	}

	// Check for the uniforms binding
	foundUniforms := false
	for _, b := range result.Reflect.Bindings {
		if b.Name == "uniforms" {
			foundUniforms = true
			if b.Group != 0 || b.Binding != 0 {
				t.Errorf("uniforms binding: expected group=0, binding=0, got group=%d, binding=%d",
					b.Group, b.Binding)
			}
		}
	}
	if !foundUniforms {
		t.Error("expected to find 'uniforms' binding")
	}

	// Check for entry point
	foundMain := false
	for _, ep := range result.Reflect.EntryPoints {
		if ep.Name == "main" {
			foundMain = true
			if ep.Stage != "fragment" {
				t.Errorf("main entry point: expected stage='fragment', got '%s'", ep.Stage)
			}
		}
	}
	if !foundMain {
		t.Error("expected to find 'main' entry point")
	}
}

// TestMinifyAndReflectParseError tests error handling for invalid source.
func TestMinifyAndReflectParseError(t *testing.T) {
	source := `fn invalid( { }`

	m := minifier.New(minifier.DefaultOptions())
	result := m.MinifyAndReflect(source)

	if len(result.Errors) == 0 {
		t.Error("expected parse errors")
	}

	// Should return original source on error
	if result.Code != source {
		t.Errorf("expected original source on error, got %q", result.Code)
	}
}

// TestMinifyAndReflectWithTreeShaking tests MinifyAndReflect with tree shaking.
func TestMinifyAndReflectWithTreeShaking(t *testing.T) {
	source := `
fn unused() -> i32 {
    return 42;
}

@compute @workgroup_size(1)
fn main() {
    // Does nothing
}
`

	m := minifier.New(minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		TreeShaking:       true,
	})

	result := m.MinifyAndReflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// unused function should be eliminated
	if result.Stats.SymbolsDead == 0 {
		t.Error("expected dead symbols with tree shaking enabled")
	}

	// Check entry point is preserved
	foundMain := false
	for _, ep := range result.Reflect.EntryPoints {
		if ep.Name == "main" {
			foundMain = true
		}
	}
	if !foundMain {
		t.Error("expected main entry point to be preserved")
	}
}

// TestMinifyAndReflectStructLayout tests that struct layouts are included.
func TestMinifyAndReflectStructLayout(t *testing.T) {
	source := `
struct MyStruct {
    a: f32,
    b: vec3f,
    c: mat4x4f,
}

@group(0) @binding(0) var<uniform> data: MyStruct;

@compute @workgroup_size(1)
fn main() {
    let x = data.a;
}
`

	m := minifier.New(minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	})

	result := m.MinifyAndReflect(source)

	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}

	// Check struct layout is included
	if len(result.Reflect.Structs) == 0 {
		t.Error("expected struct layouts in reflection result")
	}

	// Find MyStruct (might be renamed)
	foundStruct := false
	for _, layout := range result.Reflect.Structs {
		if len(layout.Fields) == 3 {
			foundStruct = true
			// Check first field
			if layout.Fields[0].Name != "a" {
				t.Errorf("expected first field name 'a', got '%s'", layout.Fields[0].Name)
			}
		}
	}
	if !foundStruct {
		t.Error("expected to find struct with 3 fields")
	}
}

// TestConvenienceMinifyFunction tests the package-level Minify convenience function.
func TestConvenienceMinifyFunction(t *testing.T) {
	source := `fn foo() { let x = 1; }`

	// Test with default options
	result := minifier.Minify(source)
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
	if result.Code == "" {
		t.Error("expected minified code")
	}

	// Test with custom options
	result = minifier.Minify(source, minifier.Options{
		MinifyWhitespace: true,
	})
	if len(result.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", result.Errors)
	}
}
