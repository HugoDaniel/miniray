// Package minifier_tests provides integration tests for the WGSL minifier.
//
// Following esbuild's pattern, these tests use snapshot files to verify
// complete minification output. Snapshots are stored in the snapshots/
// directory and can be updated with UPDATE_SNAPSHOTS=1.
package minifier_tests

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/HugoDaniel/miniray/internal/minifier"
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

// ----------------------------------------------------------------------------
// External Binding Alias Keyword Tests
// ----------------------------------------------------------------------------

// TestExternalBindingsKeepOriginalNames verifies that external bindings
// keep their original names when MangleExternalBindings is false (default).
func TestExternalBindingsKeepOriginalNames(t *testing.T) {
	source := `
@group(0) @binding(0) var<uniform> myUniform: f32;

fn getValue() -> f32 {
    return myUniform * 2.0;
}
`
	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		TreeShaking:       true,
		// MangleExternalBindings is false (default)
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// Verify the original binding name is preserved in both declaration and usage
	if !strings.Contains(result.Code, "myUniform") {
		t.Errorf("Original binding name should be preserved. Got:\n%s", result.Code)
	}

	// Should not have any aliases at module scope
	if strings.Contains(result.Code, "const a=") || strings.Contains(result.Code, "let a=") {
		t.Errorf("Should not have aliases at module scope. Got:\n%s", result.Code)
	}
}

// TestNoLetAtModuleScopeWithBindings verifies that minified output
// never has 'let' at module scope (which is invalid WGSL).
func TestNoLetAtModuleScopeWithBindings(t *testing.T) {
	testCases := []struct {
		name   string
		source string
	}{
		{
			name: "SingleBinding",
			source: `
@group(0) @binding(0) var<uniform> u: f32;
fn f() -> f32 { return u + u + u; }
`,
		},
		{
			name: "MultipleBindings",
			source: `
@group(0) @binding(0) var<uniform> a: f32;
@group(0) @binding(1) var<uniform> b: f32;
fn f() -> f32 { return a + b; }
`,
		},
		{
			name: "StructBinding",
			source: `
struct Params { x: f32, y: f32 }
@group(0) @binding(0) var<uniform> params: Params;
fn f() -> f32 { return params.x + params.y; }
`,
		},
		{
			name: "StorageBinding",
			source: `
@group(0) @binding(0) var<storage> data: array<f32>;
fn f(i: u32) -> f32 { return data[i]; }
`,
		},
	}

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		TreeShaking:       true,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := minifier.New(opts)
			result := m.Minify(tc.source)

			// Find module scope (everything before the first function body)
			fnIndex := strings.Index(result.Code, "fn ")
			if fnIndex == -1 {
				fnIndex = len(result.Code)
			}
			moduleScope := result.Code[:fnIndex]

			// Check that no 'let' appears at module scope
			if strings.Contains(moduleScope, "let ") {
				t.Errorf("Found 'let' at module scope which is invalid WGSL.\nModule scope:\n%s\nFull output:\n%s", moduleScope, result.Code)
			}
		})
	}
}

// TestNoLetAtModuleScope is a regression test ensuring 'let' is never used
// at module scope in minified output.
func TestNoLetAtModuleScope(t *testing.T) {
	source := `
struct PngineInputs {
    time: f32,
    canvasW: f32,
    canvasH: f32,
}

@group(0) @binding(0) var<uniform> pngine: PngineInputs;

@fragment
fn main() -> @location(0) vec4f {
    let t = pngine.time;
    return vec4f(t, 0.0, 0.0, 1.0);
}
`
	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		TreeShaking:       true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// Find module scope (everything before the first function)
	fnIndex := strings.Index(result.Code, "fn ")
	if fnIndex == -1 {
		fnIndex = len(result.Code)
	}
	moduleScope := result.Code[:fnIndex]

	// Check for 'let' in module scope (which is invalid WGSL)
	if strings.Contains(moduleScope, "let ") {
		t.Errorf("Found 'let' at module scope (invalid WGSL).\nModule scope:\n%s\n\nFull output:\n%s", moduleScope, result.Code)
	}

	// Verify external binding name is preserved
	if !strings.Contains(result.Code, "pngine") {
		t.Errorf("External binding name 'pngine' should be preserved. Got:\n%s", result.Code)
	}
}

// TestTemplatedConstructorTypeRenaming verifies that user-defined types inside
// templated type constructors (like array<MyStruct, N>()) get properly renamed.
func TestTemplatedConstructorTypeRenaming(t *testing.T) {
	// Test case: struct type inside array constructor should be renamed
	source := `
struct Transform2D {
    pos: vec2f,
    scale: vec2f,
}

fn makeTransforms() -> array<Transform2D, 3> {
    return array<Transform2D, 3>(
        Transform2D(vec2f(0.0), vec2f(1.0)),
        Transform2D(vec2f(1.0), vec2f(1.0)),
        Transform2D(vec2f(2.0), vec2f(1.0)),
    );
}
`

	opts := minifier.Options{
		MinifyWhitespace:       true,
		MinifyIdentifiers:      true,
		MangleExternalBindings: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// The struct 'Transform2D' should be renamed to a shorter name
	// Check that 'Transform2D' no longer appears in the output
	if strings.Contains(result.Code, "Transform2D") {
		t.Errorf("Expected struct 'Transform2D' to be renamed, but it still appears in output:\n%s", result.Code)
	}

	// The output should still contain 'array<' since that's a built-in type
	if !strings.Contains(result.Code, "array<") {
		t.Errorf("Expected 'array<' to appear in output:\n%s", result.Code)
	}

	// Verify the minified name (likely 'a') appears in array constructor
	// The pattern should be array<a, 3> or array<a,3> where 'a' is the renamed struct
	if !strings.Contains(result.Code, "array<a") {
		t.Errorf("Expected renamed struct in array constructor (array<a...), got:\n%s", result.Code)
	}
}

// TestNestedTemplatedTypeRenaming verifies that nested templated types with
// user-defined types get properly renamed.
func TestNestedTemplatedTypeRenaming(t *testing.T) {
	source := `
struct Point {
    x: f32,
    y: f32,
}

fn makeNestedArray() {
    var arr: array<array<Point, 2>, 3>;
    arr = array<array<Point, 2>, 3>();
}
`

	opts := minifier.Options{
		MinifyWhitespace:       true,
		MinifyIdentifiers:      true,
		MangleExternalBindings: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// 'Point' should be renamed
	if strings.Contains(result.Code, "Point") {
		t.Errorf("Expected struct 'Point' to be renamed, but it still appears:\n%s", result.Code)
	}
}

// TestArraySizeConstantRenaming verifies that constant size references in
// array constructors get properly renamed.
func TestArraySizeConstantRenaming(t *testing.T) {
	source := `
const TRANSFORM_COUNT = 9u;

struct Transform2D {
    pos: vec2f,
}

fn makeTransforms() {
    let transforms = array<Transform2D, TRANSFORM_COUNT>();
}
`

	opts := minifier.Options{
		MinifyWhitespace:       true,
		MinifyIdentifiers:      true,
		MangleExternalBindings: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// Both 'Transform2D' and 'TRANSFORM_COUNT' should be renamed
	if strings.Contains(result.Code, "Transform2D") {
		t.Errorf("Expected struct 'Transform2D' to be renamed:\n%s", result.Code)
	}
	if strings.Contains(result.Code, "TRANSFORM_COUNT") {
		t.Errorf("Expected constant 'TRANSFORM_COUNT' to be renamed:\n%s", result.Code)
	}
}

// TestBuiltinTypesNotRenamed verifies that built-in types in templated
// constructors are NOT renamed.
func TestBuiltinTypesInTemplatedConstructors(t *testing.T) {
	source := `
fn makeVecs() {
    let v1 = vec3<f32>(1.0, 2.0, 3.0);
    let v2 = array<f32, 4>(1.0, 2.0, 3.0, 4.0);
    let v3 = mat2x2<f32>(1.0, 0.0, 0.0, 1.0);
}
`

	opts := minifier.Options{
		MinifyWhitespace:       true,
		MinifyIdentifiers:      true,
		MangleExternalBindings: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// Built-in types like f32 should NOT be renamed
	if !strings.Contains(result.Code, "vec3<f32>") {
		t.Errorf("Expected 'vec3<f32>' to remain unchanged:\n%s", result.Code)
	}
	if !strings.Contains(result.Code, "array<f32") {
		t.Errorf("Expected 'array<f32' to remain unchanged:\n%s", result.Code)
	}
	if !strings.Contains(result.Code, "mat2x2<f32>") {
		t.Errorf("Expected 'mat2x2<f32>' to remain unchanged:\n%s", result.Code)
	}
}

// TestLocalShadowsFunction verifies that when a local variable shadows a function name,
// function calls that appear BEFORE the local variable declaration bind to the function,
// not the local variable.
func TestLocalShadowsFunction(t *testing.T) {
	source := `
fn box(p: vec2f, b: vec2f) -> f32 {
    let q = abs(p) - b;
    return length(max(q, vec2f(0.0))) + min(max(q.x, q.y), 0.0);
}

fn test() -> f32 {
    let raw = box(vec2f(1.0), vec2f(0.5));
    let box = raw * 2.0;
    return box;
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// The function 'box' should be renamed (e.g., to 'a')
	// The local variable 'box' should also be renamed (e.g., to 'b')
	// The call to box() should use the function's renamed name, not the local's

	// Verify no errors (the bug caused "unknown identifier" errors)
	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}

	// The output should contain a function call pattern like "a(" or "b("
	// followed by the function body, not just variable assignments
	if !strings.Contains(result.Code, "fn ") {
		t.Errorf("Expected minified function definition, got:\n%s", result.Code)
	}

	// Original 'box' identifier should not appear in output
	if strings.Contains(result.Code, "box") {
		t.Errorf("Expected 'box' to be renamed, but it still appears:\n%s", result.Code)
	}
}

// TestLocalShadowsFunctionComplex tests a more complex shadowing scenario
// similar to the sceneW.wgsl bug.
func TestLocalShadowsFunctionComplex(t *testing.T) {
	source := `
fn box(p: vec2f, b: vec2f) -> f32 {
    return length(max(abs(p) - b, vec2f(0.0)));
}

fn scale_sdf(d: f32, s: f32) -> f32 {
    return d * s;
}

fn render(p: vec2f) -> f32 {
    let raw = box(p, vec2f(0.5));
    let box = scale_sdf(raw, 2.0);
    return box;
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// Should compile without errors
	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got: %v", result.Errors)
	}

	// The minified code should be valid - both function and local should have different names
	// and the call should use the function's name
	if strings.Contains(result.Code, "box") {
		t.Errorf("Expected 'box' to be renamed:\n%s", result.Code)
	}
}

// ============================================================================
// Else-If Spacing Bug Tests (was outputting "elseif" instead of "else if")
// ============================================================================

// TestElseIfSpacing verifies that else-if chains have proper spacing.
// Bug: The minifier was outputting "elseif" (no space) instead of "else if".
func TestElseIfSpacing(t *testing.T) {
	testCases := []struct {
		name   string
		source string
	}{
		{
			name: "SimpleElseIf",
			source: `
fn test(x: i32) -> i32 {
    if (x > 0) {
        return 1;
    } else if (x < 0) {
        return -1;
    } else {
        return 0;
    }
}
`,
		},
		{
			name: "ChainedElseIf",
			source: `
fn classify(x: i32) -> i32 {
    if (x > 100) {
        return 4;
    } else if (x > 50) {
        return 3;
    } else if (x > 10) {
        return 2;
    } else if (x > 0) {
        return 1;
    } else {
        return 0;
    }
}
`,
		},
		{
			name: "NestedElseIf",
			source: `
fn nested(x: i32, y: i32) -> i32 {
    if (x > 0) {
        if (y > 0) {
            return 1;
        } else if (y < 0) {
            return 2;
        } else {
            return 3;
        }
    } else if (x < 0) {
        return -1;
    } else {
        return 0;
    }
}
`,
		},
		{
			name: "ElseIfWithoutFinalElse",
			source: `
fn partial(x: i32) -> i32 {
    if (x > 0) {
        return 1;
    } else if (x < 0) {
        return -1;
    }
    return 0;
}
`,
		},
		{
			name: "ElseIfWithComplexConditions",
			source: `
fn complex(a: i32, b: i32) -> i32 {
    if (a > 0 && b > 0) {
        return 1;
    } else if (a < 0 || b < 0) {
        return -1;
    } else if (a == 0 && b == 0) {
        return 0;
    } else {
        return 2;
    }
}
`,
		},
	}

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := minifier.New(opts)
			result := m.Minify(tc.source)

			// Must NOT contain "elseif" (no space)
			if strings.Contains(result.Code, "elseif") {
				t.Errorf("Found invalid 'elseif' (should be 'else if'):\n%s", result.Code)
			}

			// Must contain "else if" with space (if there's an else-if in the source)
			if strings.Contains(tc.source, "else if") && !strings.Contains(result.Code, "else if") {
				t.Errorf("Expected 'else if' with space, but not found:\n%s", result.Code)
			}

			// Should have no errors
			if len(result.Errors) > 0 {
				t.Errorf("Unexpected errors: %v", result.Errors)
			}
		})
	}
}

// TestElseIfInComputeShader tests else-if in a realistic compute shader context.
func TestElseIfInComputeShader(t *testing.T) {
	source := `
@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3u) {
    var value: f32 = 0.0;
    let x = f32(id.x);

    if (x > 100.0) {
        value = 1.0;
    } else if (x > 50.0) {
        value = 0.75;
    } else if (x > 25.0) {
        value = 0.5;
    } else if (x > 0.0) {
        value = 0.25;
    } else {
        value = 0.0;
    }
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// Verify proper else if spacing
	if strings.Contains(result.Code, "elseif") {
		t.Errorf("Found invalid 'elseif':\n%s", result.Code)
	}

	// Count the number of "else if" occurrences - should be at least 3
	// (the source has 4, but exact count may vary based on optimization)
	count := strings.Count(result.Code, "else if")
	if count < 3 {
		t.Errorf("Expected at least 3 'else if' occurrences, got %d:\n%s", count, result.Code)
	}
}

// ============================================================================
// External Binding Alias Bug Tests (was generating invalid "const a=uniforms;")
// ============================================================================

// TestNoInvalidConstAliases verifies that the minifier doesn't generate
// invalid const aliases for external bindings.
// Bug: The minifier was generating "const a=uniforms;" which is invalid because
// const requires constant expressions, and uniform references aren't constants.
func TestNoInvalidConstAliases(t *testing.T) {
	testCases := []struct {
		name   string
		source string
	}{
		{
			name: "SingleUniform",
			source: `
struct Params { time: f32, scale: f32 }
@group(0) @binding(0) var<uniform> params: Params;

fn getValue() -> f32 {
    return params.time * params.scale;
}
`,
		},
		{
			name: "MultipleUniforms",
			source: `
@group(0) @binding(0) var<uniform> time: f32;
@group(0) @binding(1) var<uniform> scale: f32;

fn getValue() -> f32 {
    return time * scale * time * scale;
}
`,
		},
		{
			name: "StorageBuffer",
			source: `
@group(0) @binding(0) var<storage, read_write> data: array<f32>;

fn process(idx: u32) {
    data[idx] = data[idx] * 2.0;
}
`,
		},
		{
			name: "MixedBindings",
			source: `
struct Uniforms { multiplier: f32 }
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var<storage> input: array<f32>;
@group(0) @binding(2) var<storage, read_write> output: array<f32>;

fn process(idx: u32) {
    output[idx] = input[idx] * uniforms.multiplier;
}
`,
		},
		{
			name: "UniformUsedManyTimes",
			source: `
@group(0) @binding(0) var<uniform> factor: f32;

fn compute(a: f32, b: f32, c: f32) -> f32 {
    let x = a * factor;
    let y = b * factor;
    let z = c * factor;
    let w = (x + y + z) * factor;
    return w * factor;
}
`,
		},
	}

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		TreeShaking:       true,
		// MangleExternalBindings is false (default)
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := minifier.New(opts)
			result := m.Minify(tc.source)

			// Must NOT contain alias patterns like "const a=varname;"
			// These are invalid because uniform/storage references aren't constant expressions
			aliasPattern := regexp.MustCompile(`const [a-z]=[a-zA-Z]`)
			if aliasPattern.MatchString(result.Code) {
				t.Errorf("Found invalid const alias pattern:\n%s", result.Code)
			}

			// Must NOT contain "let" at module scope
			fnIndex := strings.Index(result.Code, "fn ")
			if fnIndex == -1 {
				fnIndex = len(result.Code)
			}
			moduleScope := result.Code[:fnIndex]
			if strings.Contains(moduleScope, "let ") {
				t.Errorf("Found 'let' at module scope (invalid WGSL):\n%s", moduleScope)
			}

			// Should have no errors
			if len(result.Errors) > 0 {
				t.Errorf("Unexpected errors: %v", result.Errors)
			}
		})
	}
}

// TestExternalBindingsPreserveNames verifies that external bindings keep
// their original names when MangleExternalBindings is false.
func TestExternalBindingsPreserveNames(t *testing.T) {
	source := `
@group(0) @binding(0) var<uniform> myUniforms: f32;
@group(0) @binding(1) var<storage> myStorage: array<f32>;

fn process(idx: u32) -> f32 {
    return myStorage[idx] * myUniforms;
}
`

	opts := minifier.Options{
		MinifyWhitespace:       true,
		MinifyIdentifiers:      true,
		MangleExternalBindings: false, // Explicit
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// External binding names should be preserved
	if !strings.Contains(result.Code, "myUniforms") {
		t.Errorf("Expected 'myUniforms' to be preserved:\n%s", result.Code)
	}
	if !strings.Contains(result.Code, "myStorage") {
		t.Errorf("Expected 'myStorage' to be preserved:\n%s", result.Code)
	}

	// Function and parameters should still be renamed
	if strings.Contains(result.Code, "process") {
		t.Errorf("Expected function 'process' to be renamed:\n%s", result.Code)
	}
}

// TestExternalBindingsMangled verifies that external bindings are renamed
// when MangleExternalBindings is true.
func TestExternalBindingsMangled(t *testing.T) {
	source := `
@group(0) @binding(0) var<uniform> myUniforms: f32;

fn getValue() -> f32 {
    return myUniforms * 2.0;
}
`

	opts := minifier.Options{
		MinifyWhitespace:       true,
		MinifyIdentifiers:      true,
		MangleExternalBindings: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// External binding should be renamed
	if strings.Contains(result.Code, "myUniforms") {
		t.Errorf("Expected 'myUniforms' to be renamed when MangleExternalBindings=true:\n%s", result.Code)
	}
}

// ============================================================================
// Local Variable Shadowing Function Bug Tests
// ============================================================================

// TestShadowingBasic tests basic shadowing where local shadows function.
func TestShadowingBasic(t *testing.T) {
	source := `
fn helper(x: f32) -> f32 {
    return x * 2.0;
}

fn main() -> f32 {
    let a = helper(1.0);
    let helper = a + 1.0;
    return helper;
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v\nCode:\n%s", result.Errors, result.Code)
	}

	// Should not contain original names
	if strings.Contains(result.Code, "helper") {
		t.Errorf("Expected 'helper' to be renamed:\n%s", result.Code)
	}
}

// TestShadowingMultipleFunctions tests shadowing with multiple functions.
func TestShadowingMultipleFunctions(t *testing.T) {
	source := `
fn circle(p: vec2f, r: f32) -> f32 {
    return length(p) - r;
}

fn box(p: vec2f, b: vec2f) -> f32 {
    let d = abs(p) - b;
    return length(max(d, vec2f(0.0))) + min(max(d.x, d.y), 0.0);
}

fn scene(p: vec2f) -> f32 {
    let d1 = circle(p, 0.5);
    let d2 = box(p, vec2f(0.3));

    let circle = min(d1, d2);
    let box = circle * 2.0;

    return box;
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v\nCode:\n%s", result.Errors, result.Code)
	}

	// All original names should be renamed
	for _, name := range []string{"circle", "box", "scene"} {
		if strings.Contains(result.Code, name) {
			t.Errorf("Expected '%s' to be renamed:\n%s", name, result.Code)
		}
	}
}

// TestShadowingInNestedScopes tests shadowing in nested scopes.
func TestShadowingInNestedScopes(t *testing.T) {
	source := `
fn outer(x: f32) -> f32 {
    return x + 1.0;
}

fn test() -> f32 {
    let a = outer(1.0);
    if (a > 0.0) {
        let outer = a * 2.0;
        return outer;
    }
    return outer(a);
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v\nCode:\n%s", result.Errors, result.Code)
	}
}

// TestShadowingWithLoops tests shadowing with loop variables.
func TestShadowingWithLoops(t *testing.T) {
	source := `
fn index(i: u32) -> u32 {
    return i * 2u;
}

fn sum() -> u32 {
    var total: u32 = 0u;
    for (var i: u32 = 0u; i < 10u; i++) {
        let idx = index(i);
        let index = idx + 1u;
        total += index;
    }
    return total;
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v\nCode:\n%s", result.Errors, result.Code)
	}
}

// TestShadowingCallBeforeAndAfter tests that calls before and after shadow work correctly.
func TestShadowingCallBeforeAndAfter(t *testing.T) {
	source := `
fn foo(x: f32) -> f32 {
    return x * 2.0;
}

fn test() -> f32 {
    let a = foo(1.0);
    let foo = 3.0;
    let b = foo + a;
    return b;
}

fn test2() -> f32 {
    return foo(2.0);
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v\nCode:\n%s", result.Errors, result.Code)
	}

	// test2 should still be able to call foo function
	// The code should contain two function calls (one in test, one in test2)
	if strings.Contains(result.Code, "foo") {
		t.Errorf("Expected 'foo' to be renamed:\n%s", result.Code)
	}
}

// TestShadowingStruct tests shadowing with struct names.
func TestShadowingStruct(t *testing.T) {
	source := `
struct Data {
    value: f32,
}

fn Data_new(v: f32) -> Data {
    return Data(v);
}

fn test() -> f32 {
    let d = Data_new(1.0);
    let Data = d.value;
    return Data;
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v\nCode:\n%s", result.Errors, result.Code)
	}
}

// TestShadowingParameter tests that parameters don't incorrectly shadow.
func TestShadowingParameter(t *testing.T) {
	source := `
fn transform(p: vec2f) -> vec2f {
    return p * 2.0;
}

fn test(transform: f32) -> f32 {
    return transform + 1.0;
}

fn test2() -> vec2f {
    return transform(vec2f(1.0));
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v\nCode:\n%s", result.Errors, result.Code)
	}
}

// ============================================================================
// PreserveUniformStructTypes Tests (PNGine Integration)
// ============================================================================

// TestPreserveUniformStructTypes_Basic tests that struct types used in uniform
// declarations are preserved when PreserveUniformStructTypes is enabled.
func TestPreserveUniformStructTypes_Basic(t *testing.T) {
	source := `
struct MyUniforms {
    time: f32,
    scale: f32,
}

@group(0) @binding(0) var<uniform> u: MyUniforms;

fn getValue() -> f32 {
    return u.time * u.scale;
}
`

	opts := minifier.Options{
		MinifyWhitespace:           true,
		MinifyIdentifiers:          true,
		MinifySyntax:               true,
		PreserveUniformStructTypes: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// Struct type 'MyUniforms' should be preserved
	if !strings.Contains(result.Code, "MyUniforms") {
		t.Errorf("Expected struct type 'MyUniforms' to be preserved with PreserveUniformStructTypes=true:\n%s", result.Code)
	}

	// Variable name 'u' should be preserved (external binding)
	if !strings.Contains(result.Code, "var<uniform> u") {
		t.Errorf("Expected uniform variable 'u' to be preserved:\n%s", result.Code)
	}

	// Function 'getValue' should be renamed (not an entry point)
	if strings.Contains(result.Code, "getValue") {
		t.Errorf("Expected function 'getValue' to be renamed:\n%s", result.Code)
	}
}

// TestPreserveUniformStructTypes_MultipleStructs tests preservation of multiple
// struct types used in different uniform/storage declarations.
func TestPreserveUniformStructTypes_MultipleStructs(t *testing.T) {
	source := `
struct UniformA {
    x: f32,
}

struct StorageB {
    y: f32,
}

struct NotInBinding {
    z: f32,
}

@group(0) @binding(0) var<uniform> a: UniformA;
@group(0) @binding(1) var<storage> b: StorageB;

fn process() -> f32 {
    var local: NotInBinding;
    local.z = 1.0;
    return a.x + b.y + local.z;
}
`

	opts := minifier.Options{
		MinifyWhitespace:           true,
		MinifyIdentifiers:          true,
		PreserveUniformStructTypes: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// UniformA and StorageB should be preserved (used in bindings)
	if !strings.Contains(result.Code, "UniformA") {
		t.Errorf("Expected 'UniformA' to be preserved:\n%s", result.Code)
	}
	if !strings.Contains(result.Code, "StorageB") {
		t.Errorf("Expected 'StorageB' to be preserved:\n%s", result.Code)
	}

	// NotInBinding struct should be renamed (used but not in any binding)
	if strings.Contains(result.Code, "NotInBinding") {
		t.Errorf("Expected 'NotInBinding' to be renamed:\n%s", result.Code)
	}
}

// TestPreserveUniformStructTypes_NestedType tests that only the direct type
// is preserved, not nested types within the struct.
func TestPreserveUniformStructTypes_NestedType(t *testing.T) {
	source := `
struct Inner {
    v: f32,
}

struct Outer {
    inner: Inner,
}

@group(0) @binding(0) var<uniform> u: Outer;

fn getValue() -> f32 {
    return u.inner.v;
}
`

	opts := minifier.Options{
		MinifyWhitespace:           true,
		MinifyIdentifiers:          true,
		PreserveUniformStructTypes: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// Outer should be preserved (direct type in uniform)
	if !strings.Contains(result.Code, "Outer") {
		t.Errorf("Expected 'Outer' to be preserved:\n%s", result.Code)
	}

	// Inner may be renamed (it's a nested type, not directly in the binding)
	// This is the expected behavior - only direct types are preserved
}

// TestPreserveUniformStructTypes_Disabled tests that struct types are renamed
// when PreserveUniformStructTypes is false (default).
func TestPreserveUniformStructTypes_Disabled(t *testing.T) {
	source := `
struct MyUniforms {
    time: f32,
}

@group(0) @binding(0) var<uniform> u: MyUniforms;

fn getValue() -> f32 {
    return u.time;
}
`

	opts := minifier.Options{
		MinifyWhitespace:           true,
		MinifyIdentifiers:          true,
		PreserveUniformStructTypes: false, // Default
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// Struct type 'MyUniforms' should be renamed when option is disabled
	if strings.Contains(result.Code, "MyUniforms") {
		t.Errorf("Expected 'MyUniforms' to be renamed when PreserveUniformStructTypes=false:\n%s", result.Code)
	}
}

// TestPreserveUniformStructTypes_WithKeepNames tests that both options work together.
func TestPreserveUniformStructTypes_WithKeepNames(t *testing.T) {
	source := `
struct AutoPreserved {
    a: f32,
}

struct ManuallyKept {
    b: f32,
}

struct ShouldRename {
    c: f32,
}

@group(0) @binding(0) var<uniform> u: AutoPreserved;

fn helper() -> f32 {
    var m: ManuallyKept;
    var s: ShouldRename;
    return u.a + m.b + s.c;
}
`

	opts := minifier.Options{
		MinifyWhitespace:           true,
		MinifyIdentifiers:          true,
		PreserveUniformStructTypes: true,
		KeepNames:                  []string{"ManuallyKept"},
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// AutoPreserved: preserved via PreserveUniformStructTypes
	if !strings.Contains(result.Code, "AutoPreserved") {
		t.Errorf("Expected 'AutoPreserved' to be preserved:\n%s", result.Code)
	}

	// ManuallyKept: preserved via keepNames
	if !strings.Contains(result.Code, "ManuallyKept") {
		t.Errorf("Expected 'ManuallyKept' to be preserved via keepNames:\n%s", result.Code)
	}

	// ShouldRename: not in keepNames, not in uniform binding
	if strings.Contains(result.Code, "ShouldRename") {
		t.Errorf("Expected 'ShouldRename' to be renamed:\n%s", result.Code)
	}
}

// TestPreserveUniformStructTypes_PngineBuiltins tests the specific PNGine use case.
func TestPreserveUniformStructTypes_PngineBuiltins(t *testing.T) {
	source := `
struct PngineInputs {
    time: f32,
    canvasW: f32,
    canvasH: f32,
}

@group(0) @binding(0) var<uniform> pngine: PngineInputs;

fn computeUV(pos: vec2f) -> vec2f {
    return pos / vec2f(pngine.canvasW, pngine.canvasH);
}

@fragment
fn main(@builtin(position) pos: vec4f) -> @location(0) vec4f {
    let uv = computeUV(pos.xy);
    let t = pngine.time;
    return vec4f(uv, t, 1.0);
}
`

	opts := minifier.Options{
		MinifyWhitespace:           true,
		MinifyIdentifiers:          true,
		MinifySyntax:               true,
		PreserveUniformStructTypes: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	// PngineInputs should be preserved (this is what PNGine needs!)
	if !strings.Contains(result.Code, "PngineInputs") {
		t.Errorf("Expected 'PngineInputs' to be preserved for PNGine compatibility:\n%s", result.Code)
	}

	// pngine variable should be preserved (external binding)
	if !strings.Contains(result.Code, "pngine") {
		t.Errorf("Expected 'pngine' variable to be preserved:\n%s", result.Code)
	}

	// Entry point 'main' should be preserved
	if !strings.Contains(result.Code, "fn main") {
		t.Errorf("Expected entry point 'main' to be preserved:\n%s", result.Code)
	}

	// Helper function 'computeUV' should be renamed
	if strings.Contains(result.Code, "computeUV") {
		t.Errorf("Expected helper function 'computeUV' to be renamed:\n%s", result.Code)
	}

	// Field names should be preserved (accessed via `.`)
	if !strings.Contains(result.Code, ".time") {
		t.Errorf("Expected field name '.time' to be preserved:\n%s", result.Code)
	}
	if !strings.Contains(result.Code, ".canvasW") {
		t.Errorf("Expected field name '.canvasW' to be preserved:\n%s", result.Code)
	}
}

// TestShadowingRealisticSDF tests a realistic SDF rendering pattern.
func TestShadowingRealisticSDF(t *testing.T) {
	source := `
fn box(p: vec2f, b: vec2f) -> f32 {
    let d = abs(p) - b;
    return length(max(d, vec2f(0.0))) + min(max(d.x, d.y), 0.0);
}

fn circle(p: vec2f, r: f32) -> f32 {
    return length(p) - r;
}

fn smoothMin(a: f32, b: f32, k: f32) -> f32 {
    let h = max(k - abs(a - b), 0.0) / k;
    return min(a, b) - h * h * k * 0.25;
}

fn scene(p: vec2f) -> f32 {
    let box = box(p - vec2f(0.5, 0.0), vec2f(0.3, 0.2));
    let circle = circle(p + vec2f(0.5, 0.0), 0.25);
    let combined = smoothMin(box, circle, 0.1);
    return combined;
}

@fragment
fn main(@location(0) uv: vec2f) -> @location(0) vec4f {
    let d = scene(uv);
    return vec4f(vec3f(d), 1.0);
}
`

	opts := minifier.Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
	}

	m := minifier.New(opts)
	result := m.Minify(source)

	if len(result.Errors) > 0 {
		t.Errorf("Unexpected errors: %v\nCode:\n%s", result.Errors, result.Code)
	}

	// Should be minified (at least 25% smaller)
	if len(result.Code) > len(source)*3/4 {
		t.Errorf("Expected at least 25%% minification. Original: %d, Minified: %d", len(source), len(result.Code))
	}
}
