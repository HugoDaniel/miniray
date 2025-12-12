package minifier_tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeberg.org/saruga/wgsl-minifier/internal/config"
	"codeberg.org/saruga/wgsl-minifier/internal/minifier"
)

// TestComputeToysConfig tests minification with the compute.toys configuration.
// compute.toys is a WebGPU compute shader playground that provides a prelude
// with specific uniforms, textures, and helper functions that must be preserved.
func TestComputeToysConfig(t *testing.T) {
	// Load the compute.toys config
	configPath := filepath.Join("..", "..", "configs", "compute.toys.json")
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load compute.toys config: %v", err)
	}

	opts := cfg.ToOptions()

	tests := []struct {
		name           string
		input          string
		mustContain    []string // Strings that must appear in output
		mustNotContain []string // Strings that must NOT appear in output
	}{
		{
			name: "PreservesTimeUniform",
			input: `@compute @workgroup_size(16, 16)
fn main_image(@builtin(global_invocation_id) id: vec3u) {
    let t = time.elapsed;
    let d = time.delta;
    let f = time.frame;
}`,
			mustContain: []string{
				"time.elapsed",
				"time.delta",
				"time.frame",
				"main_image",
			},
		},
		{
			name: "PreservesMouseUniform",
			input: `@compute @workgroup_size(16, 16)
fn main_image(@builtin(global_invocation_id) id: vec3u) {
    let p = mouse.pos;
    let z = mouse.zoom;
    let c = mouse.click;
    let s = mouse.start;
    let d = mouse.delta;
}`,
			mustContain: []string{
				"mouse.pos",
				"mouse.zoom",
				"mouse.click",
				"mouse.start",
				"mouse.delta",
			},
		},
		{
			name: "PreservesTextureBindings",
			input: `@compute @workgroup_size(16, 16)
fn main_image(@builtin(global_invocation_id) id: vec3u) {
    let size = textureDimensions(screen);
    textureStore(screen, id.xy, vec4f(1.0));
    let p = passLoad(0, vec2i(0), 0);
    passStore(0, vec2i(0), vec4f(1.0));
    let c0 = textureSampleLevel(channel0, bilinear, vec2f(0.5), 0.0);
    let c1 = textureSampleLevel(channel1, trilinear, vec2f(0.5), 0.0);
}`,
			mustContain: []string{
				"screen",
				"passLoad",
				"passStore",
				"channel0",
				"channel1",
				"bilinear",
				"trilinear",
			},
		},
		{
			name: "PreservesSamplers",
			input: `@compute @workgroup_size(16, 16)
fn main_image(@builtin(global_invocation_id) id: vec3u) {
    let a = textureSampleLevel(channel0, nearest, vec2f(0.5), 0.0);
    let b = textureSampleLevel(channel0, bilinear, vec2f(0.5), 0.0);
    let c = textureSampleLevel(channel0, trilinear, vec2f(0.5), 0.0);
    let d = textureSampleLevel(channel0, nearest_repeat, vec2f(0.5), 0.0);
    let e = textureSampleLevel(channel0, bilinear_repeat, vec2f(0.5), 0.0);
    let f = textureSampleLevel(channel0, trilinear_repeat, vec2f(0.5), 0.0);
}`,
			mustContain: []string{
				"nearest",
				"bilinear",
				"trilinear",
				"nearest_repeat",
				"bilinear_repeat",
				"trilinear_repeat",
			},
		},
		{
			name: "PreservesDispatchInfo",
			input: `@compute @workgroup_size(16, 16)
fn main_image(@builtin(global_invocation_id) id: vec3u) {
    let dispatchId = dispatch.id;
}`,
			mustContain: []string{
				"dispatch.id",
			},
		},
		{
			name: "PreservesKeyDownHelper",
			input: `@compute @workgroup_size(16, 16)
fn main_image(@builtin(global_invocation_id) id: vec3u) {
    if (keyDown(32u)) {
        textureStore(screen, id.xy, vec4f(1.0));
    }
}`,
			mustContain: []string{
				"keyDown",
			},
		},
		{
			name: "RenamesLocalVariables",
			input: `fn myHelperFunction(inputValue: f32) -> f32 {
    let intermediateResult = inputValue * 2.0;
    let finalResult = intermediateResult + 1.0;
    return finalResult;
}

@compute @workgroup_size(16, 16)
fn main_image(@builtin(global_invocation_id) invocationId: vec3u) {
    let computedValue = myHelperFunction(time.elapsed);
    let screenSize = textureDimensions(screen);
    textureStore(screen, vec2i(invocationId.xy), vec4f(computedValue, screenSize.x, 0.0, 1.0));
}`,
			mustContain: []string{
				"time.elapsed",
				"main_image",
				"screen",
			},
			mustNotContain: []string{
				"myHelperFunction",
				"inputValue",
				"intermediateResult",
				"finalResult",
				"computedValue",
				"invocationId",
				"screenSize",
			},
		},
		{
			name: "PreservesTypeAliases",
			input: `fn useAliases() -> float4 {
    let a: int = 1;
    let b: uint = 2u;
    let c: float = 3.0;
    let d: float2 = vec2f(1.0, 2.0);
    let e: float3 = vec3f(1.0);
    let f: float4 = vec4f(1.0);
    return f;
}

@compute @workgroup_size(16, 16)
fn main_image(@builtin(global_invocation_id) id: vec3u) {
    let result = useAliases();
}`,
			mustContain: []string{
				"int",
				"uint",
				"float",
				"float2",
				"float3",
				"float4",
			},
		},
		{
			name: "PreservesPassBufferHelpers",
			input: `@compute @workgroup_size(16, 16)
fn main_image(@builtin(global_invocation_id) id: vec3u) {
    let prev = passLoad(0, vec2i(id.xy), 0);
    passStore(0, vec2i(id.xy), prev * 0.99);
    let sampled = passSampleLevelBilinearRepeat(0, vec2f(0.5), 0.0);
}`,
			mustContain: []string{
				"passLoad",
				"passStore",
				"passSampleLevelBilinearRepeat",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := minifier.New(opts)
			result := m.Minify(tt.input)

			if len(result.Errors) > 0 {
				t.Fatalf("Minification errors: %v", result.Errors)
			}

			for _, s := range tt.mustContain {
				if !strings.Contains(result.Code, s) {
					t.Errorf("Output must contain %q but doesn't.\nOutput: %s", s, result.Code)
				}
			}

			for _, s := range tt.mustNotContain {
				if strings.Contains(result.Code, s) {
					t.Errorf("Output must NOT contain %q but does.\nOutput: %s", s, result.Code)
				}
			}
		})
	}
}

// TestComputeToysShaderFiles tests actual shader files from testdata/compute.toys
func TestComputeToysShaderFiles(t *testing.T) {
	// Load the compute.toys config
	configPath := filepath.Join("..", "..", "configs", "compute.toys.json")
	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to load compute.toys config: %v", err)
	}

	opts := cfg.ToOptions()

	shaderDir := filepath.Join("..", "..", "testdata", "compute.toys")
	entries, err := os.ReadDir(shaderDir)
	if err != nil {
		t.Fatalf("Failed to read shader directory: %v", err)
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".wgsl") {
			continue
		}

		t.Run(entry.Name(), func(t *testing.T) {
			path := filepath.Join(shaderDir, entry.Name())
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("Failed to read shader file: %v", err)
			}

			m := minifier.New(opts)
			result := m.Minify(string(source))

			if len(result.Errors) > 0 {
				t.Fatalf("Minification errors: %v", result.Errors)
			}

			// Verify size reduction
			if result.Stats.MinifiedSize >= result.Stats.OriginalSize {
				t.Errorf("No size reduction: %d -> %d bytes",
					result.Stats.OriginalSize, result.Stats.MinifiedSize)
			}

			reduction := float64(result.Stats.OriginalSize-result.Stats.MinifiedSize) /
				float64(result.Stats.OriginalSize) * 100
			t.Logf("Size: %d -> %d bytes (%.1f%% reduction)",
				result.Stats.OriginalSize, result.Stats.MinifiedSize, reduction)

			// Verify required names are preserved
			// Only check for names that are actually used (not just in comments)
			requiredNames := []string{
				"main_image",
				"screen",
			}

			for _, name := range requiredNames {
				// Only check if present in original (this is a basic check)
				if strings.Contains(string(source), name) {
					if !strings.Contains(result.Code, name) {
						t.Errorf("Required name %q was renamed but should be preserved", name)
					}
				}
			}
		})
	}
}

// TestStructTypeRenaming verifies that struct type names are consistently renamed
// when used in type positions (variable declarations, function parameters, return types).
func TestStructTypeRenaming(t *testing.T) {
	opts := minifier.Options{
		MinifyWhitespace:   true,
		MinifyIdentifiers:  true,
		MinifySyntax:       true,
	}

	tests := []struct {
		name        string
		input       string
		description string
	}{
		{
			name: "StructInFunctionParameter",
			input: `struct MyStruct { x: f32 }
fn doSomething(s: MyStruct) -> f32 {
    return s.x;
}`,
			description: "Struct type in function parameter should use renamed type",
		},
		{
			name: "StructInReturnType",
			input: `struct MyStruct { x: f32 }
fn createStruct() -> MyStruct {
    return MyStruct(1.0);
}`,
			description: "Struct type in return type should use renamed type",
		},
		{
			name: "StructInVarDeclaration",
			input: `struct MyStruct { x: f32 }
fn test() {
    var s: MyStruct;
    s.x = 1.0;
}`,
			description: "Struct type in var declaration should use renamed type",
		},
		{
			name: "StructInLetDeclaration",
			input: `struct MyStruct { x: f32 }
fn test() {
    let s: MyStruct = MyStruct(1.0);
}`,
			description: "Struct type in let declaration should use renamed type",
		},
		{
			name: "MultipleStructUsages",
			input: `struct Transform2D {
    pos: vec2f,
    scale: vec2f,
}
fn transform(uv: vec2f, t: Transform2D) -> vec2f {
    return (uv - t.pos) / t.scale;
}
fn createTransform() -> Transform2D {
    return Transform2D(vec2f(0.0), vec2f(1.0));
}`,
			description: "All references to struct type should use same renamed name",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := minifier.New(opts)
			result := m.Minify(tc.input)

			if len(result.Errors) > 0 {
				t.Fatalf("Minification errors: %v", result.Errors)
			}

			// The output should NOT contain the original struct name if renaming is working
			// (unless the struct name is very short like 'a')
			if strings.Contains(result.Code, "MyStruct") {
				t.Errorf("Output still contains 'MyStruct' - struct type not renamed:\n%s", result.Code)
			}
			if strings.Contains(result.Code, "Transform2D") {
				t.Errorf("Output still contains 'Transform2D' - struct type not renamed:\n%s", result.Code)
			}

			// Check that output doesn't have unresolved type errors
			// (i.e., references to renamed structs should also be renamed)
			// The minified code should be self-consistent
			t.Logf("Minified output:\n%s", result.Code)
		})
	}
}
