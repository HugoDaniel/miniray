// Package minifier_tests provides integration tests for the WGSL minifier.
//
// Following esbuild's pattern, these tests use snapshot files to verify
// complete minification output. Snapshots are stored in the snapshots/
// directory and can be updated with UPDATE_SNAPSHOTS=1.
package minifier_tests

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codeberg.org/saruga/wgsl-minifier/internal/minifier"
)

// ----------------------------------------------------------------------------
// Snapshot Testing Infrastructure
// ----------------------------------------------------------------------------

type testSuite struct {
	t            *testing.T
	snapshotDir  string
	snapshotFile string
	snapshots    map[string]string
	results      []testResult
}

type testResult struct {
	name   string
	output string
}

func newTestSuite(t *testing.T, snapshotFile string) *testSuite {
	_, filename, _, _ := runtime.Caller(0)
	snapshotDir := filepath.Join(filepath.Dir(filename), "snapshots")

	suite := &testSuite{
		t:            t,
		snapshotDir:  snapshotDir,
		snapshotFile: snapshotFile,
		snapshots:    make(map[string]string),
		results:      make([]testResult, 0),
	}

	// Load existing snapshots
	suite.loadSnapshots()
	return suite
}

func (s *testSuite) loadSnapshots() {
	path := filepath.Join(s.snapshotDir, s.snapshotFile)
	data, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist yet, that's OK
		return
	}

	// Parse snapshot format:
	// TestName
	// ---------- /out.wgsl ----------
	// <output>
	// ================================================================================
	content := string(data)
	tests := strings.Split(content, "\n================================================================================\n")

	for _, test := range tests {
		test = strings.TrimSpace(test)
		if test == "" {
			continue
		}

		lines := strings.SplitN(test, "\n", 3)
		if len(lines) < 3 {
			continue
		}

		name := strings.TrimSpace(lines[0])
		// lines[1] is the separator
		output := lines[2]

		s.snapshots[name] = output
	}
}

func (s *testSuite) saveSnapshots() {
	var builder strings.Builder

	for _, result := range s.results {
		builder.WriteString(result.name)
		builder.WriteString("\n---------- /out.wgsl ----------\n")
		builder.WriteString(result.output)
		builder.WriteString("\n================================================================================\n")
	}

	path := filepath.Join(s.snapshotDir, s.snapshotFile)
	os.MkdirAll(s.snapshotDir, 0755)
	os.WriteFile(path, []byte(builder.String()), 0644)
}

func (s *testSuite) expectMinified(name string, input string, opts minifier.Options) {
	s.t.Run(name, func(t *testing.T) {
		m := minifier.New(opts)
		result := m.Minify(input)

		s.results = append(s.results, testResult{
			name:   name,
			output: result.Code,
		})

		// Check against snapshot
		if expected, ok := s.snapshots[name]; ok {
			if result.Code != expected {
				t.Errorf("\nexpected:\n%s\nactual:\n%s", expected, result.Code)
			}
		} else if os.Getenv("UPDATE_SNAPSHOTS") == "" {
			// No snapshot and not updating - record for later
			t.Logf("No snapshot for %s (run with UPDATE_SNAPSHOTS=1 to create)", name)
		}
	})
}

func (s *testSuite) done() {
	if os.Getenv("UPDATE_SNAPSHOTS") == "1" {
		s.saveSnapshots()
		s.t.Logf("Updated snapshots in %s", s.snapshotFile)
	}
}

// ----------------------------------------------------------------------------
// Test Cases
// ----------------------------------------------------------------------------

// minified is a test case configuration.
type minified struct {
	input   string
	options minifier.Options
}

// defaultOpts returns full minification options.
func defaultOpts() minifier.Options {
	return minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
	}
}

// whitespaceOnly returns whitespace-only minification.
func whitespaceOnly() minifier.Options {
	return minifier.Options{
		MinifyWhitespace: true,
	}
}

// ----------------------------------------------------------------------------
// Basic Minification Tests
// ----------------------------------------------------------------------------

func TestMinifyBasic(t *testing.T) {
	suite := newTestSuite(t, "snapshots_basic.txt")
	defer suite.done()

	// Simple const declarations
	suite.expectMinified("ConstSimple", `
const x = 1;
const y = 2;
const z = x + y;
`, whitespaceOnly())

	// Variable declarations (without sized arrays - parser limitation)
	suite.expectMinified("VarDeclarations", `
var<private> counter: i32;
var<workgroup> flag: bool;
`, whitespaceOnly())

	// Function declaration
	suite.expectMinified("FunctionSimple", `
fn add(a: i32, b: i32) -> i32 {
    return a + b;
}
`, whitespaceOnly())

	// Struct declaration
	suite.expectMinified("StructSimple", `
struct Point {
    x: f32,
    y: f32,
    z: f32,
}
`, whitespaceOnly())
}

// ----------------------------------------------------------------------------
// Whitespace Minification Tests
// ----------------------------------------------------------------------------

func TestMinifyWhitespace(t *testing.T) {
	suite := newTestSuite(t, "snapshots_whitespace.txt")
	defer suite.done()

	suite.expectMinified("RemoveNewlines", `
const a = 1;

const b = 2;

const c = 3;
`, whitespaceOnly())

	suite.expectMinified("RemoveIndentation", `
fn foo() {
    if true {
        return;
    }
}
`, whitespaceOnly())

	suite.expectMinified("CompactOperators", `
const x = 1 + 2 * 3 - 4 / 5;
`, whitespaceOnly())

	suite.expectMinified("CompactFunction", `
fn compute(a: f32, b: f32, c: f32) -> f32 {
    let temp = a + b;
    return temp * c;
}
`, whitespaceOnly())
}

// ----------------------------------------------------------------------------
// Entry Point Preservation Tests
// ----------------------------------------------------------------------------

func TestPreserveEntryPoints(t *testing.T) {
	suite := newTestSuite(t, "snapshots_entrypoints.txt")
	defer suite.done()

	// Vertex shader entry point
	suite.expectMinified("VertexEntryPoint", `
@vertex
fn vertexMain(@location(0) position: vec4f) -> @builtin(position) vec4f {
    return position;
}
`, defaultOpts())

	// Fragment shader entry point
	suite.expectMinified("FragmentEntryPoint", `
@fragment
fn fragmentMain() -> @location(0) vec4f {
    return vec4f(1.0, 0.0, 0.0, 1.0);
}
`, defaultOpts())

	// Compute shader entry point
	suite.expectMinified("ComputeEntryPoint", `
@compute @workgroup_size(64)
fn computeMain(@builtin(global_invocation_id) id: vec3u) {
    // compute work
}
`, defaultOpts())
}

// ----------------------------------------------------------------------------
// Attribute Preservation Tests
// ----------------------------------------------------------------------------

func TestPreserveAttributes(t *testing.T) {
	suite := newTestSuite(t, "snapshots_attributes.txt")
	defer suite.done()

	// Binding attributes must be preserved
	suite.expectMinified("BindingAttributes", `
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var textureSampler: sampler;
@group(0) @binding(2) var texture: texture_2d<f32>;
`, whitespaceOnly())

	// Builtin attributes must be preserved
	suite.expectMinified("BuiltinAttributes", `
struct VertexOutput {
    @builtin(position) position: vec4f,
    @location(0) color: vec3f,
}
`, whitespaceOnly())

	// Workgroup size must be preserved
	suite.expectMinified("WorkgroupSize", `
@compute @workgroup_size(8, 8, 1)
fn main() {}
`, whitespaceOnly())
}

// ----------------------------------------------------------------------------
// Complex Shader Tests
// ----------------------------------------------------------------------------

func TestMinifyComplexShaders(t *testing.T) {
	suite := newTestSuite(t, "snapshots_complex.txt")
	defer suite.done()

	// Vertex shader with structs
	suite.expectMinified("VertexShaderWithStructs", `
struct Uniforms {
    modelViewProjection: mat4x4f,
}

struct VertexInput {
    @location(0) position: vec4f,
    @location(1) uv: vec2f,
}

struct VertexOutput {
    @builtin(position) position: vec4f,
    @location(0) uv: vec2f,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;

@vertex
fn main(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.position = uniforms.modelViewProjection * input.position;
    output.uv = input.uv;
    return output;
}
`, whitespaceOnly())

	// Fragment shader with texture sampling
	suite.expectMinified("FragmentShaderWithTexture", `
@group(0) @binding(1) var textureSampler: sampler;
@group(0) @binding(2) var texture: texture_2d<f32>;

@fragment
fn main(@location(0) uv: vec2f) -> @location(0) vec4f {
    return textureSample(texture, textureSampler, uv);
}
`, whitespaceOnly())

	// Compute shader (simplified - sized arrays not yet supported)
	suite.expectMinified("ComputeShaderSimple", `
@group(0) @binding(0) var<storage, read_write> data: array<f32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3u) {
    let idx = id.x;
    data[idx] = data[idx] * 2.0;
}
`, whitespaceOnly())
}

// ----------------------------------------------------------------------------
// Control Flow Tests
// ----------------------------------------------------------------------------

func TestMinifyControlFlow(t *testing.T) {
	suite := newTestSuite(t, "snapshots_controlflow.txt")
	defer suite.done()

	suite.expectMinified("IfElse", `
fn test(x: i32) -> i32 {
    if x > 0 {
        return 1;
    } else if x < 0 {
        return -1;
    } else {
        return 0;
    }
}
`, whitespaceOnly())

	// Note: For loops not yet fully supported by parser
	suite.expectMinified("WhileLoop", `
fn countdown(n: i32) {
    var x = n;
    while x > 0 {
        x--;
    }
}
`, whitespaceOnly())

	suite.expectMinified("LoopSimple", `
fn loopTest() {
    var i = 0;
    loop {
        if i >= 10 {
            break;
        }
        i++;
    }
}
`, whitespaceOnly())

	suite.expectMinified("Switch", `
fn getColor(index: i32) -> vec3f {
    switch index {
        case 0: {
            return vec3f(1.0, 0.0, 0.0);
        }
        case 1: {
            return vec3f(0.0, 1.0, 0.0);
        }
        case 2: {
            return vec3f(0.0, 0.0, 1.0);
        }
        default: {
            return vec3f(0.0, 0.0, 0.0);
        }
    }
}
`, whitespaceOnly())
}

// ----------------------------------------------------------------------------
// Expression Tests
// ----------------------------------------------------------------------------

func TestMinifyExpressions(t *testing.T) {
	suite := newTestSuite(t, "snapshots_expressions.txt")
	defer suite.done()

	suite.expectMinified("Arithmetic", `
const a = 1 + 2 * 3 - 4 / 5 % 6;
const b = (1 + 2) * (3 - 4);
const c = -1 + -2;
`, whitespaceOnly())

	suite.expectMinified("Logical", `
const a = true && false;
const b = true || false;
const c = !true;
const d = true && (false || true);
`, whitespaceOnly())

	suite.expectMinified("Bitwise", `
const a = 1 & 2;
const b = 1 | 2;
const c = 1 ^ 2;
const d = ~1;
const e = 1 << 2;
const f = 8 >> 2;
`, whitespaceOnly())

	suite.expectMinified("Comparison", `
const a = 1 == 2;
const b = 1 != 2;
const c = 1 < 2;
const d = 1 <= 2;
const e = 1 > 2;
const f = 1 >= 2;
`, whitespaceOnly())

	suite.expectMinified("MemberAccess", `
const a = v.x;
const b = v.xyz;
const c = m[0][1];
const d = arr[i].field;
`, whitespaceOnly())

	suite.expectMinified("FunctionCalls", `
const a = sin(1.0);
const b = max(1.0, 2.0);
const c = clamp(x, 0.0, 1.0);
const d = dot(v1, v2);
`, whitespaceOnly())

	// Note: array constructors with size not yet fully supported by parser
	suite.expectMinified("TypeConstructors", `
const a = vec3f(1.0);
const b = vec3f(1.0, 2.0, 3.0);
const c = vec4f(v.xyz, 1.0);
const d = mat4x4f();
`, whitespaceOnly())
}

// ----------------------------------------------------------------------------
// Type Tests
// ----------------------------------------------------------------------------

func TestMinifyTypes(t *testing.T) {
	suite := newTestSuite(t, "snapshots_types.txt")
	defer suite.done()

	suite.expectMinified("ScalarTypes", `
var a: bool;
var b: i32;
var c: u32;
var d: f32;
var e: f16;
`, whitespaceOnly())

	suite.expectMinified("VectorTypes", `
var a: vec2f;
var b: vec3f;
var c: vec4f;
var d: vec2<f32>;
var e: vec3<i32>;
var f: vec4<u32>;
`, whitespaceOnly())

	suite.expectMinified("MatrixTypes", `
var a: mat2x2f;
var b: mat3x3f;
var c: mat4x4f;
var d: mat2x3<f32>;
var e: mat3x4<f32>;
`, whitespaceOnly())

	// Note: Sized arrays not yet fully supported by parser
	suite.expectMinified("ArrayTypes", `
var a: array<f32>;
var b: array<vec3f>;
`, whitespaceOnly())

	suite.expectMinified("TextureTypes", `
var a: texture_2d<f32>;
var b: texture_3d<f32>;
var c: texture_cube<f32>;
var d: texture_2d_array<f32>;
var e: texture_storage_2d<rgba8unorm, write>;
`, whitespaceOnly())

	suite.expectMinified("SamplerTypes", `
var a: sampler;
var b: sampler_comparison;
`, whitespaceOnly())
}

// ----------------------------------------------------------------------------
// Directive Tests
// ----------------------------------------------------------------------------

func TestMinifyDirectives(t *testing.T) {
	suite := newTestSuite(t, "snapshots_directives.txt")
	defer suite.done()

	suite.expectMinified("EnableDirective", `
enable f16;
enable chromium_experimental_dp4a;
`, whitespaceOnly())

	suite.expectMinified("DiagnosticDirective", `
diagnostic(off, derivative_uniformity);
`, whitespaceOnly())

	suite.expectMinified("ConstAssert", `
const SIZE = 64;
const_assert SIZE > 0;
const_assert SIZE <= 256;
`, whitespaceOnly())
}

// ----------------------------------------------------------------------------
// Identifier Minification Tests
// ----------------------------------------------------------------------------

func TestMinifyIdentifiers(t *testing.T) {
	suite := newTestSuite(t, "snapshots_identifiers.txt")
	defer suite.done()

	// Local variables should be renamed
	suite.expectMinified("LocalVariables", `
fn compute() -> f32 {
    let firstValue = 1.0;
    let secondValue = 2.0;
    let result = firstValue + secondValue;
    return result;
}
`, defaultOpts())

	// Function parameters should be renamed
	suite.expectMinified("FunctionParameters", `
fn add(firstNumber: f32, secondNumber: f32) -> f32 {
    return firstNumber + secondNumber;
}
`, defaultOpts())

	// Helper functions should be renamed
	suite.expectMinified("HelperFunctions", `
fn helperFunction(value: f32) -> f32 {
    return value * 2.0;
}

fn anotherHelper(input: f32) -> f32 {
    return helperFunction(input) + 1.0;
}
`, defaultOpts())

	// Entry points should NOT be renamed
	suite.expectMinified("EntryPointPreserved", `
fn helperFunc(x: f32) -> f32 {
    return x * 2.0;
}

@vertex
fn vertexMain(@location(0) pos: vec4f) -> @builtin(position) vec4f {
    return pos;
}
`, defaultOpts())

	// Struct names and members (members can be renamed internally)
	suite.expectMinified("StructRenaming", `
struct MyCustomStruct {
    fieldOne: f32,
    fieldTwo: f32,
}

fn useStruct() -> f32 {
    var instance: MyCustomStruct;
    instance.fieldOne = 1.0;
    instance.fieldTwo = 2.0;
    return instance.fieldOne + instance.fieldTwo;
}
`, defaultOpts())

	// Const declarations
	suite.expectMinified("ConstRenaming", `
const MY_CONSTANT = 42;
const ANOTHER_CONST = MY_CONSTANT * 2;

fn useConsts() -> i32 {
    return MY_CONSTANT + ANOTHER_CONST;
}
`, defaultOpts())

	// Multiple functions with same local variable names
	suite.expectMinified("SameScopedNames", `
fn funcOne() -> f32 {
    let temp = 1.0;
    return temp;
}

fn funcTwo() -> f32 {
    let temp = 2.0;
    return temp;
}
`, defaultOpts())

	// Nested scopes
	suite.expectMinified("NestedScopes", `
fn outer() -> f32 {
    let outerVar = 1.0;
    if true {
        let innerVar = 2.0;
        return outerVar + innerVar;
    }
    return outerVar;
}
`, defaultOpts())
}

// ----------------------------------------------------------------------------
// Combined Minification Tests (all options)
// ----------------------------------------------------------------------------

func TestMinifyCombined(t *testing.T) {
	suite := newTestSuite(t, "snapshots_combined.txt")
	defer suite.done()

	// Full shader with all optimizations
	suite.expectMinified("FullVertexShader", `
struct Uniforms {
    transform: mat4x4f,
}

struct VertexInput {
    @location(0) position: vec4f,
    @location(1) texcoord: vec2f,
}

struct VertexOutput {
    @builtin(position) position: vec4f,
    @location(0) texcoord: vec2f,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;

fn transformPosition(pos: vec4f) -> vec4f {
    return uniforms.transform * pos;
}

@vertex
fn vertexMain(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    output.position = transformPosition(input.position);
    output.texcoord = input.texcoord;
    return output;
}
`, defaultOpts())

	// Fragment shader with all optimizations
	suite.expectMinified("FullFragmentShader", `
@group(0) @binding(1) var texSampler: sampler;
@group(0) @binding(2) var tex: texture_2d<f32>;

fn sampleTexture(uv: vec2f) -> vec4f {
    return textureSample(tex, texSampler, uv);
}

fn adjustColor(color: vec4f, brightness: f32) -> vec4f {
    return vec4f(color.rgb * brightness, color.a);
}

@fragment
fn fragmentMain(@location(0) texcoord: vec2f) -> @location(0) vec4f {
    let baseColor = sampleTexture(texcoord);
    let adjustedColor = adjustColor(baseColor, 1.2);
    return adjustedColor;
}
`, defaultOpts())
}

// ----------------------------------------------------------------------------
// External Binding Alias Tests
// ----------------------------------------------------------------------------

func TestExternalBindingAliases(t *testing.T) {
	suite := newTestSuite(t, "snapshots_external_bindings.txt")
	defer suite.done()

	// Single uniform binding should keep original name and get alias
	suite.expectMinified("SingleUniformBinding", `
@group(0) @binding(0) var<uniform> uniforms: f32;

fn useUniform() -> f32 {
    return uniforms * 2.0;
}
`, defaultOpts())

	// Multiple uniform bindings
	suite.expectMinified("MultipleUniformBindings", `
@group(0) @binding(0) var<uniform> modelMatrix: mat4x4f;
@group(0) @binding(1) var<uniform> viewMatrix: mat4x4f;

fn transform(pos: vec4f) -> vec4f {
    return viewMatrix * modelMatrix * pos;
}
`, defaultOpts())

	// Storage binding
	suite.expectMinified("StorageBinding", `
@group(0) @binding(0) var<storage, read_write> data: array<f32>;

fn readData(idx: u32) -> f32 {
    return data[idx];
}
`, defaultOpts())

	// Mixed uniform and storage
	suite.expectMinified("MixedUniformStorage", `
@group(0) @binding(0) var<uniform> params: vec4f;
@group(0) @binding(1) var<storage> buffer: array<f32>;

fn process(idx: u32) -> f32 {
    return buffer[idx] * params.x;
}
`, defaultOpts())

	// Uniform with texture and sampler (textures/samplers are not external bindings)
	suite.expectMinified("UniformWithTextures", `
@group(0) @binding(0) var<uniform> scale: f32;
@group(0) @binding(1) var tex: texture_2d<f32>;
@group(0) @binding(2) var samp: sampler;

@fragment
fn main(@location(0) uv: vec2f) -> @location(0) vec4f {
    return textureSample(tex, samp, uv) * scale;
}
`, defaultOpts())

	// Uniform used multiple times (alias should be efficient)
	suite.expectMinified("UniformMultipleUses", `
@group(0) @binding(0) var<uniform> multiplier: f32;

fn calculate(a: f32, b: f32, c: f32) -> f32 {
    let x = a * multiplier;
    let y = b * multiplier;
    let z = c * multiplier;
    return x + y + z;
}
`, defaultOpts())

	// Struct uniform (common pattern)
	suite.expectMinified("StructUniform", `
struct Uniforms {
    time: f32,
    resolution: vec2f,
}

@group(0) @binding(0) var<uniform> u: Uniforms;

fn getAspect() -> f32 {
    return u.resolution.x / u.resolution.y;
}

fn animate(pos: vec2f) -> vec2f {
    return pos + vec2f(sin(u.time), cos(u.time));
}
`, defaultOpts())

	// Unused uniform should not get alias (no uses = no alias needed)
	suite.expectMinified("UnusedUniform", `
@group(0) @binding(0) var<uniform> unused: f32;

fn constant() -> f32 {
    return 1.0;
}
`, defaultOpts())

	// Binding with long name that benefits from aliasing
	suite.expectMinified("LongBindingName", `
@group(0) @binding(0) var<uniform> veryLongUniformBindingName: f32;

fn getValue() -> f32 {
    return veryLongUniformBindingName + veryLongUniformBindingName;
}
`, defaultOpts())
}

// ----------------------------------------------------------------------------
// MangleExternalBindings Tests
// ----------------------------------------------------------------------------

func TestMangleExternalBindings(t *testing.T) {
	suite := newTestSuite(t, "snapshots_mangle_external.txt")
	defer suite.done()

	// Helper for options with MangleExternalBindings enabled
	mangleExternalOpts := func() minifier.Options {
		return minifier.Options{
			MinifyWhitespace:       true,
			MinifyIdentifiers:      true,
			MinifySyntax:           true,
			MangleExternalBindings: true,
		}
	}

	// Single uniform with mangling enabled - should rename directly
	suite.expectMinified("MangledUniform", `
@group(0) @binding(0) var<uniform> uniforms: f32;

fn useUniform() -> f32 {
    return uniforms * 2.0;
}
`, mangleExternalOpts())

	// Multiple uniforms with mangling
	suite.expectMinified("MangledMultipleUniforms", `
@group(0) @binding(0) var<uniform> modelMatrix: mat4x4f;
@group(0) @binding(1) var<uniform> viewMatrix: mat4x4f;

fn transform(pos: vec4f) -> vec4f {
    return viewMatrix * modelMatrix * pos;
}
`, mangleExternalOpts())

	// Storage buffer with mangling
	suite.expectMinified("MangledStorage", `
@group(0) @binding(0) var<storage, read_write> data: array<f32>;

fn readData(idx: u32) -> f32 {
    return data[idx];
}
`, mangleExternalOpts())

	// Compare size: mangled vs aliased
	// The mangled version should be slightly smaller (no let alias declarations)
	suite.expectMinified("MangledVsAliased", `
@group(0) @binding(0) var<uniform> uniforms: f32;

fn a() -> f32 { return uniforms; }
fn b() -> f32 { return uniforms; }
fn c() -> f32 { return uniforms; }
`, mangleExternalOpts())
}
