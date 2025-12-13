package minifier_tests

import (
	"strings"
	"testing"

	"github.com/HugoDaniel/miniray/internal/minifier"
	"github.com/HugoDaniel/miniray/internal/parser"
)

// minifyWithDCE minifies with tree shaking enabled but identifier renaming disabled
// for easier testing.
func minifyWithDCE(source string) string {
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		return "PARSE ERROR"
	}

	opts := minifier.DefaultOptions()
	opts.MinifyIdentifiers = false // Keep names for easier testing
	opts.TreeShaking = true
	m := minifier.New(opts)
	result := m.MinifyModule(module)

	return result.Code
}

func TestDCEBasicUnusedFunction(t *testing.T) {
	source := `
fn unused() {}
@fragment fn main() -> @location(0) vec4f {
    return vec4f(1.0);
}
`
	result := minifyWithDCE(source)

	if strings.Contains(result, "unused") {
		t.Errorf("DCE should remove unused function, got: %s", result)
	}
	if !strings.Contains(result, "main") {
		t.Errorf("DCE should keep entry point, got: %s", result)
	}
}

func TestDCEBasicUsedFunction(t *testing.T) {
	source := `
fn helper() -> f32 { return 1.0; }
@fragment fn main() -> @location(0) vec4f {
    return vec4f(helper());
}
`
	result := minifyWithDCE(source)

	// The helper function should be kept (though renamed)
	// Check that we have at least 2 functions
	fnCount := strings.Count(result, "fn ")
	if fnCount < 2 {
		t.Errorf("DCE should keep used function, got %d functions: %s", fnCount, result)
	}
}

func TestDCEUnusedConst(t *testing.T) {
	source := `
const UNUSED: f32 = 3.14;
const USED: f32 = 2.71;
@fragment fn main() -> @location(0) vec4f {
    return vec4f(USED);
}
`
	result := minifyWithDCE(source)

	// UNUSED should be removed, USED should be kept
	constCount := strings.Count(result, "const ")
	if constCount != 1 {
		t.Errorf("DCE should remove unused const, expected 1 const, got %d: %s", constCount, result)
	}
}

func TestDCEUnusedStruct(t *testing.T) {
	source := `
struct Unused { x: f32 }
struct Used { y: f32 }
@fragment fn main() -> @location(0) vec4f {
    var u: Used;
    u.y = 1.0;
    return vec4f(u.y);
}
`
	result := minifyWithDCE(source)

	// Should have 1 struct
	structCount := strings.Count(result, "struct ")
	if structCount != 1 {
		t.Errorf("DCE should remove unused struct, expected 1 struct, got %d: %s", structCount, result)
	}
}

func TestDCETransitiveDependency(t *testing.T) {
	source := `
const A: f32 = 1.0;
const B: f32 = A + 1.0;
const C: f32 = B + 1.0;
const UNUSED: f32 = 999.0;
@fragment fn main() -> @location(0) vec4f {
    return vec4f(C);
}
`
	result := minifyWithDCE(source)

	// A, B, C should be kept, UNUSED should be removed
	constCount := strings.Count(result, "const ")
	if constCount != 3 {
		t.Errorf("DCE should keep transitive deps, expected 3 consts, got %d: %s", constCount, result)
	}
}

func TestDCEFunctionCallChain(t *testing.T) {
	source := `
fn a() -> f32 { return 1.0; }
fn b() -> f32 { return a() + 1.0; }
fn c() -> f32 { return b() + 1.0; }
fn unused() -> f32 { return 0.0; }
@fragment fn main() -> @location(0) vec4f {
    return vec4f(c());
}
`
	result := minifyWithDCE(source)

	// a, b, c, main should be kept, unused should be removed
	fnCount := strings.Count(result, "fn ")
	if fnCount != 4 {
		t.Errorf("DCE should keep call chain, expected 4 functions, got %d: %s", fnCount, result)
	}
}

func TestDCEMultipleEntryPoints(t *testing.T) {
	source := `
fn used_by_both() -> f32 { return 1.0; }
fn vertex_only() -> f32 { return 2.0; }
fn fragment_only() -> f32 { return 3.0; }
fn unused() -> f32 { return 4.0; }

@vertex fn vs_main(@builtin(vertex_index) idx: u32) -> @builtin(position) vec4f {
    return vec4f(used_by_both() + vertex_only());
}

@fragment fn fs_main() -> @location(0) vec4f {
    return vec4f(used_by_both() + fragment_only());
}
`
	result := minifyWithDCE(source)

	// All except unused should be kept
	fnCount := strings.Count(result, "fn ")
	if fnCount != 5 {
		t.Errorf("DCE should keep functions for all entry points, expected 5 functions, got %d: %s", fnCount, result)
	}
}

func TestDCEStructUsedInType(t *testing.T) {
	source := `
struct VertexOutput {
    @builtin(position) pos: vec4f,
}

struct Unused {
    x: f32,
}

@vertex fn main() -> VertexOutput {
    var out: VertexOutput;
    out.pos = vec4f(0.0);
    return out;
}
`
	result := minifyWithDCE(source)

	// VertexOutput should be kept, Unused should be removed
	structCount := strings.Count(result, "struct ")
	if structCount != 1 {
		t.Errorf("DCE should keep struct used in return type, expected 1 struct, got %d: %s", structCount, result)
	}
}

func TestDCEComputeShader(t *testing.T) {
	source := `
struct Particle { pos: vec3f, vel: vec3f }

fn unused_helper() {}

fn apply_force(p: ptr<function, Particle>) {
    (*p).vel += vec3f(0.0, -9.8, 0.0);
}

@group(0) @binding(0) var<storage, read_write> particles: array<Particle>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3u) {
    var p = particles[id.x];
    apply_force(&p);
    particles[id.x] = p;
}
`
	result := minifyWithDCE(source)

	// unused_helper should be removed
	fnCount := strings.Count(result, "fn ")
	if fnCount != 2 { // main + apply_force
		t.Errorf("DCE should remove unused helper in compute shader, expected 2 functions, got %d: %s", fnCount, result)
	}
}

func TestDCENoEntryPoint(t *testing.T) {
	// When there's no entry point, keep everything (conservative)
	source := `
fn a() -> f32 { return 1.0; }
fn b() -> f32 { return 2.0; }
`
	result := minifyWithDCE(source)

	fnCount := strings.Count(result, "fn ")
	if fnCount != 2 {
		t.Errorf("DCE with no entry point should keep everything, expected 2 functions, got %d: %s", fnCount, result)
	}
}

func TestDCEAliasType(t *testing.T) {
	source := `
alias UsedFloat = f32;
alias UnusedInt = i32;

@fragment fn main() -> @location(0) vec4f {
    var x: UsedFloat = 1.0;
    return vec4f(x);
}
`
	result := minifyWithDCE(source)

	aliasCount := strings.Count(result, "alias ")
	if aliasCount != 1 {
		t.Errorf("DCE should remove unused alias, expected 1 alias, got %d: %s", aliasCount, result)
	}
}

func TestDCEOverride(t *testing.T) {
	source := `
override USED: f32 = 1.0;
override UNUSED: f32 = 2.0;

@fragment fn main() -> @location(0) vec4f {
    return vec4f(USED);
}
`
	result := minifyWithDCE(source)

	overrideCount := strings.Count(result, "override ")
	if overrideCount != 1 {
		t.Errorf("DCE should remove unused override, expected 1 override, got %d: %s", overrideCount, result)
	}
}

func TestDCEDirectivesKept(t *testing.T) {
	source := `
enable f16;

fn unused() {}

@fragment fn main() -> @location(0) vec4f {
    return vec4f(1.0);
}
`
	result := minifyWithDCE(source)

	if !strings.Contains(result, "enable") {
		t.Errorf("DCE should keep directives: %s", result)
	}
}

func TestDCEDisabled(t *testing.T) {
	source := `
fn unused() {}
@fragment fn main() -> @location(0) vec4f {
    return vec4f(1.0);
}
`
	// Minify without tree shaking
	p := parser.New(source)
	module, _ := p.Parse()

	opts := minifier.DefaultOptions()
	opts.TreeShaking = false
	opts.MinifyIdentifiers = false
	m := minifier.New(opts)
	result := m.MinifyModule(module)

	fnCount := strings.Count(result.Code, "fn ")
	if fnCount != 2 {
		t.Errorf("With DCE disabled, should keep all functions, expected 2, got %d: %s", fnCount, result.Code)
	}
}

func TestDCEArrayTypeWithConstSize(t *testing.T) {
	source := `
const SIZE: u32 = 10u;
const UNUSED: u32 = 20u;

@fragment fn main() -> @location(0) vec4f {
    var arr: array<f32, SIZE>;
    arr[0] = 1.0;
    return vec4f(arr[0]);
}
`
	result := minifyWithDCE(source)

	constCount := strings.Count(result, "const ")
	if constCount != 1 {
		t.Errorf("DCE should keep const used in array size, expected 1 const, got %d: %s", constCount, result)
	}
}

func TestDCEConstAssertKept(t *testing.T) {
	source := `
const_assert 1 == 1;

fn unused() {}

@fragment fn main() -> @location(0) vec4f {
    return vec4f(1.0);
}
`
	result := minifyWithDCE(source)

	if !strings.Contains(result, "const_assert") {
		t.Errorf("DCE should keep const_assert: %s", result)
	}
}

func TestDCEExternalBindingsKept(t *testing.T) {
	source := `
struct Uniforms { time: f32 }
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var<uniform> unused_uniforms: Uniforms;

@fragment fn main() -> @location(0) vec4f {
    return vec4f(uniforms.time);
}
`
	result := minifyWithDCE(source)

	// Only the used uniform should be kept
	varCount := strings.Count(result, "var<uniform>")
	if varCount != 1 {
		t.Errorf("DCE should remove unused external bindings, expected 1 var<uniform>, got %d: %s", varCount, result)
	}
}
