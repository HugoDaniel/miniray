package sourcemap_tests

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/HugoDaniel/miniray/internal/minifier"
	"github.com/HugoDaniel/miniray/internal/sourcemap"
)

// ============================================================================
// Full Integration Tests
// ============================================================================

func TestSourceMapIntegrationBasic(t *testing.T) {
	source := `const x = 1;
const y = 2;
const z = x + y;`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if len(result.Errors) > 0 {
		t.Fatalf("Minification errors: %v", result.Errors)
	}

	if result.SourceMap == nil {
		t.Fatal("Expected source map to be generated")
	}

	// Verify basic structure
	if result.SourceMap.Version != 3 {
		t.Errorf("Version = %d, want 3", result.SourceMap.Version)
	}

	if result.SourceMap.Mappings == "" {
		t.Error("Mappings should not be empty")
	}
}

func TestSourceMapIdentifierRenaming(t *testing.T) {
	source := `const longVariableName = 42;
fn foo() -> i32 {
    return longVariableName;
}`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if len(result.Errors) > 0 {
		t.Fatalf("Minification errors: %v", result.Errors)
	}

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// Check that original name is in the names array
	found := false
	for _, name := range result.SourceMap.Names {
		if name == "longVariableName" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected 'longVariableName' in source map names array")
	}
}

func TestSourceMapNoRenaming(t *testing.T) {
	source := `const x = 1;`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: false, // No renaming
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// Names should be empty when no renaming occurs
	if len(result.SourceMap.Names) != 0 {
		t.Errorf("Expected empty names array, got %v", result.SourceMap.Names)
	}
}

func TestSourceMapMultipleLines(t *testing.T) {
	source := `fn main() {
    var x = 1;
    var y = 2;
    var z = 3;
}`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  false, // Keep lines
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// Each line should be represented in mappings
	// Semicolons separate lines in source map mappings
	lines := strings.Split(result.SourceMap.Mappings, ";")

	if len(lines) < 2 {
		t.Errorf("Expected multiple lines in mappings, got %d", len(lines))
	}
}

func TestSourceMapDeltaEncoding(t *testing.T) {
	source := `const alpha = 1;
const beta = 2;`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// Decode and verify mappings use proper delta encoding
	mappings, err := sourcemap.DecodeMappings(result.SourceMap.Mappings)
	if err != nil {
		t.Fatalf("Failed to decode mappings: %v", err)
	}

	if len(mappings) == 0 {
		t.Fatal("Expected at least one mapping")
	}

	// Verify mappings are valid (non-negative positions)
	for i, m := range mappings {
		if m.GenLine < 0 || m.GenCol < 0 {
			t.Errorf("Mapping %d has invalid generated position: (%d, %d)", i, m.GenLine, m.GenCol)
		}
		if m.SrcLine < 0 || m.SrcCol < 0 {
			t.Errorf("Mapping %d has invalid source position: (%d, %d)", i, m.SrcLine, m.SrcCol)
		}
	}
}

func TestSourceMapEntryPointNotRenamed(t *testing.T) {
	source := `@compute @workgroup_size(1)
fn main() {
    let x = 1;
}`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// 'main' should NOT be in names since it's preserved
	for _, name := range result.SourceMap.Names {
		if name == "main" {
			t.Error("'main' should not be in names array since it's preserved")
		}
	}
}

func TestSourceMapJSON(t *testing.T) {
	source := `const x = 1;`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
		SourceMapOptions: minifier.SourceMapOptions{
			File:       "test.min.wgsl",
			SourceName: "test.wgsl",
		},
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	jsonStr := result.SourceMap.ToJSON()

	// Parse to verify valid JSON
	var parsed sourcemap.SourceMap
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if parsed.File != "test.min.wgsl" {
		t.Errorf("File = %q, want %q", parsed.File, "test.min.wgsl")
	}

	if len(parsed.Sources) != 1 || parsed.Sources[0] != "test.wgsl" {
		t.Errorf("Sources = %v, want [test.wgsl]", parsed.Sources)
	}
}

func TestSourceMapDataURI(t *testing.T) {
	source := `const x = 1;`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	dataURI := result.SourceMap.ToDataURI()

	if !strings.HasPrefix(dataURI, "data:application/json;base64,") {
		t.Error("Data URI should have correct prefix")
	}
}

func TestSourceMapSourcesContent(t *testing.T) {
	source := `const x = 1;`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
		SourceMapOptions: minifier.SourceMapOptions{
			IncludeSource: true,
		},
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	if len(result.SourceMap.SourcesContent) != 1 {
		t.Fatalf("Expected 1 source content, got %d", len(result.SourceMap.SourcesContent))
	}

	if result.SourceMap.SourcesContent[0] != source {
		t.Error("SourcesContent should contain original source")
	}
}

func TestSourceMapTreeShaking(t *testing.T) {
	source := `const used = 1;
const unused = 2;
@compute @workgroup_size(1)
fn main() {
    let x = used;
}`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		TreeShaking:       true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// 'unused' should NOT appear in names (it's tree-shaken)
	for _, name := range result.SourceMap.Names {
		if name == "unused" {
			t.Error("'unused' should not be in names array (tree-shaken)")
		}
	}
}

func TestSourceMapDisabled(t *testing.T) {
	source := `const x = 1;`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: false, // Disabled
	})

	if result.SourceMap != nil {
		t.Error("Source map should be nil when disabled")
	}
}

func TestSourceMapFunctionParameters(t *testing.T) {
	source := `fn add(alpha: i32, beta: i32) -> i32 {
    return alpha + beta;
}`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// Function parameters should be in names
	foundAlpha := false
	foundBeta := false
	for _, name := range result.SourceMap.Names {
		if name == "alpha" {
			foundAlpha = true
		}
		if name == "beta" {
			foundBeta = true
		}
	}

	if !foundAlpha || !foundBeta {
		t.Errorf("Expected 'alpha' and 'beta' in names, got %v", result.SourceMap.Names)
	}
}

func TestSourceMapStructDeclaration(t *testing.T) {
	source := `struct MyData {
    value: f32,
}

fn use_struct(data: MyData) -> f32 {
    return data.value;
}`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// Struct name should be in names array
	found := false
	for _, name := range result.SourceMap.Names {
		if name == "MyData" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected 'MyData' in names array, got %v", result.SourceMap.Names)
	}
}

func TestSourceMapCallExpression(t *testing.T) {
	source := `fn helper() -> i32 {
    return 42;
}

fn main() -> i32 {
    return helper();
}`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// 'helper' should be in names (renamed function)
	found := false
	for _, name := range result.SourceMap.Names {
		if name == "helper" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected 'helper' in names, got %v", result.SourceMap.Names)
	}
}

func TestSourceMapNestedScopes(t *testing.T) {
	source := `fn outer() {
    var x = 1;
    if x > 0 {
        var x = 2;
    }
}`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// Both x's map to same original name
	// Should have 'x' in names array (possibly only once due to dedup)
	found := false
	for _, name := range result.SourceMap.Names {
		if name == "x" {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected 'x' in names for shadowed variable, got %v", result.SourceMap.Names)
	}
}

func TestSourceMapForLoop(t *testing.T) {
	// Note: 'value' must be used somewhere to be renamed (unused variables keep original name)
	source := `fn loop_test() -> i32 {
    var sum = 0;
    for (var counter = 0; counter < 10; counter++) {
        let value = counter * 2;
        sum = sum + value;
    }
    return sum;
}`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// Loop variable should be in names
	foundCounter := false
	foundValue := false
	for _, name := range result.SourceMap.Names {
		if name == "counter" {
			foundCounter = true
		}
		if name == "value" {
			foundValue = true
		}
	}

	if !foundCounter || !foundValue {
		t.Errorf("Expected 'counter' and 'value' in names, got %v", result.SourceMap.Names)
	}
}

func TestSourceMapValidMappingPositions(t *testing.T) {
	source := `const alpha = 1;
const beta = alpha + 2;
fn compute(x: i32) -> i32 {
    return x * alpha + beta;
}`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	mappings, err := sourcemap.DecodeMappings(result.SourceMap.Mappings)
	if err != nil {
		t.Fatalf("Failed to decode mappings: %v", err)
	}

	// Get output length
	outputLines := strings.Split(result.Code, "\n")

	for i, m := range mappings {
		// Generated positions should be within output bounds
		if m.GenLine >= len(outputLines) {
			t.Errorf("Mapping %d: GenLine %d exceeds output lines %d", i, m.GenLine, len(outputLines))
		}
		if m.GenLine < len(outputLines) && m.GenCol > len(outputLines[m.GenLine]) {
			t.Errorf("Mapping %d: GenCol %d exceeds line length %d", i, m.GenCol, len(outputLines[m.GenLine]))
		}

		// Source positions should be within source bounds
		sourceLines := strings.Split(source, "\n")
		if m.SrcLine >= len(sourceLines) {
			t.Errorf("Mapping %d: SrcLine %d exceeds source lines %d", i, m.SrcLine, len(sourceLines))
		}
	}
}

func TestSourceMapEmptySource(t *testing.T) {
	source := ``

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	// Should handle empty source gracefully
	if result.SourceMap == nil {
		t.Fatal("Expected source map even for empty source")
	}

	if result.SourceMap.Version != 3 {
		t.Errorf("Version = %d, want 3", result.SourceMap.Version)
	}
}

func TestSourceMapCommentsOnly(t *testing.T) {
	source := `// This is a comment
/* Another comment */`

	result := minifier.Minify(source, minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		GenerateSourceMap: true,
	})

	if result.SourceMap == nil {
		t.Fatal("Expected source map")
	}

	// No names for comments-only source
	if len(result.SourceMap.Names) != 0 {
		t.Errorf("Expected empty names for comments-only source, got %v", result.SourceMap.Names)
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkSourceMapGeneration(b *testing.B) {
	source := `
const WIDTH = 1920;
const HEIGHT = 1080;

struct Uniforms {
    time: f32,
    resolution: vec2<f32>,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;

fn hash(p: vec2<f32>) -> f32 {
    let h = dot(p, vec2<f32>(127.1, 311.7));
    return fract(sin(h) * 43758.5453123);
}

@compute @workgroup_size(8, 8)
fn main(@builtin(global_invocation_id) id: vec3<u32>) {
    let coords = vec2<f32>(f32(id.x), f32(id.y));
    let uv = coords / uniforms.resolution;
    let h = hash(uv + uniforms.time);
}
`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		minifier.Minify(source, minifier.Options{
			MinifyWhitespace:  true,
			MinifyIdentifiers: true,
			GenerateSourceMap: true,
		})
	}
}
