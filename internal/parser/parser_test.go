package parser

import (
	"strings"
	"testing"

	"codeberg.org/saruga/wgsl-minifier/internal/printer"
)

// ----------------------------------------------------------------------------
// Test Helpers (esbuild-style)
// ----------------------------------------------------------------------------

// expectPrinted parses input and verifies the printed output matches expected.
// This is the core testing pattern from esbuild.
func expectPrinted(t *testing.T, input string, expected string) {
	t.Helper()
	t.Run(input, func(t *testing.T) {
		t.Helper()
		p := New(input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("parse errors: %v", errs)
		}
		pr := printer.New(printer.Options{}, module.Symbols)
		actual := pr.Print(module)
		if actual != expected {
			t.Errorf("\ninput:\n%s\nexpected:\n%s\nactual:\n%s", input, expected, actual)
		}
	})
}

// expectPrintedMinify parses input and verifies minified output matches expected.
func expectPrintedMinify(t *testing.T, input string, expected string) {
	t.Helper()
	t.Run(input+"_minify", func(t *testing.T) {
		t.Helper()
		p := New(input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("parse errors: %v", errs)
		}
		pr := printer.New(printer.Options{
			MinifyWhitespace: true,
		}, module.Symbols)
		actual := pr.Print(module)
		if actual != expected {
			t.Errorf("\ninput:\n%s\nexpected:\n%s\nactual:\n%s", input, expected, actual)
		}
	})
}

// expectParseError verifies that parsing produces an error containing the substring.
func expectParseError(t *testing.T, input string, errorSubstring string) {
	t.Helper()
	t.Run(input+"_error", func(t *testing.T) {
		t.Helper()
		p := New(input)
		_, errs := p.Parse()
		if len(errs) == 0 {
			t.Errorf("expected parse error containing %q, got none", errorSubstring)
			return
		}
		found := false
		for _, err := range errs {
			if strings.Contains(err.Message, errorSubstring) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected error containing %q, got: %v", errorSubstring, errs)
		}
	})
}

// expectNoParse verifies that parsing produces at least one error.
func expectNoParse(t *testing.T, input string) {
	t.Helper()
	t.Run(input+"_noParse", func(t *testing.T) {
		t.Helper()
		p := New(input)
		_, errs := p.Parse()
		if len(errs) == 0 {
			t.Errorf("expected parse error for %q, got none", input)
		}
	})
}

// ----------------------------------------------------------------------------
// Const Declaration Tests
// ----------------------------------------------------------------------------

func TestConstDeclaration(t *testing.T) {
	expectPrinted(t, "const x = 1;", "const x = 1;\n")
	expectPrinted(t, "const x: i32 = 1;", "const x: i32 = 1;\n")
	expectPrinted(t, "const x = 1 + 2;", "const x = 1 + 2;\n")
	expectPrinted(t, "const PI = 3.14159;", "const PI = 3.14159;\n")
}

func TestConstExpressions(t *testing.T) {
	expectPrinted(t, "const x = 1 + 2 * 3;", "const x = 1 + 2 * 3;\n")
	expectPrinted(t, "const x = (1 + 2) * 3;", "const x = (1 + 2) * 3;\n")
	expectPrinted(t, "const x = -1;", "const x = -1;\n")
	expectPrinted(t, "const x = !true;", "const x = !true;\n")
}

// ----------------------------------------------------------------------------
// Let Declaration Tests
// ----------------------------------------------------------------------------

func TestLetDeclaration(t *testing.T) {
	// Let declarations are function-scope only, but we test parsing
	expectPrinted(t, "let x = 1;", "let x = 1;\n")
	expectPrinted(t, "let x: f32 = 1.0;", "let x: f32 = 1.0;\n")
}

// ----------------------------------------------------------------------------
// Var Declaration Tests
// ----------------------------------------------------------------------------

func TestVarDeclaration(t *testing.T) {
	expectPrinted(t, "var x: i32;", "var x: i32;\n")
	expectPrinted(t, "var x: i32 = 0;", "var x: i32 = 0;\n")
	expectPrinted(t, "var<private> x: i32;", "var<private> x: i32;\n")
	expectPrinted(t, "var<workgroup> odds: array<i32, 16>;", "var<workgroup> odds: array<i32, 16>;\n")
	expectPrinted(t, "var<storage, read_write> data: array<f32>;", "var<storage, read_write> data: array<f32>;\n")
}

func TestVarWithAttributes(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var<uniform> u: Uniforms;",
		"@group(0) @binding(0) var<uniform> u: Uniforms;\n")
	expectPrinted(t, "@group(0) @binding(1) var tex: texture_2d<f32>;",
		"@group(0) @binding(1) var tex: texture_2d<f32>;\n")
	expectPrinted(t, "@group(0) @binding(2) var samp: sampler;",
		"@group(0) @binding(2) var samp: sampler;\n")
}

// ----------------------------------------------------------------------------
// Override Declaration Tests
// ----------------------------------------------------------------------------

func TestOverrideDeclaration(t *testing.T) {
	expectPrinted(t, "override x: f32;", "override x: f32;\n")
	expectPrinted(t, "override x: f32 = 1.0;", "override x: f32 = 1.0;\n")
	expectPrinted(t, "@id(0) override x: f32;", "@id(0) override x: f32;\n")
}

// ----------------------------------------------------------------------------
// Struct Declaration Tests
// ----------------------------------------------------------------------------

func TestStructDeclaration(t *testing.T) {
	expectPrinted(t,
		"struct Foo { x: i32, }",
		"struct Foo {\n    x: i32\n}\n")

	expectPrinted(t,
		"struct Point { x: f32, y: f32, }",
		"struct Point {\n    x: f32,\n    y: f32\n}\n")
}

func TestStructWithAttributes(t *testing.T) {
	expectPrinted(t,
		"struct VertexOutput { @builtin(position) pos: vec4f, @location(0) uv: vec2f, }",
		"struct VertexOutput {\n    @builtin(position) pos: vec4f,\n    @location(0) uv: vec2f\n}\n")
}

// ----------------------------------------------------------------------------
// Alias Declaration Tests
// ----------------------------------------------------------------------------

func TestAliasDeclaration(t *testing.T) {
	expectPrinted(t, "alias Float = f32;", "alias Float = f32;\n")
	expectPrinted(t, "alias Vec3 = vec3<f32>;", "alias Vec3 = vec3<f32>;\n")
}

// ----------------------------------------------------------------------------
// Function Declaration Tests
// ----------------------------------------------------------------------------

func TestFunctionDeclaration(t *testing.T) {
	expectPrinted(t, "fn foo() {}", "fn foo() {\n}\n")
	expectPrinted(t, "fn foo() -> i32 { return 1; }", "fn foo() -> i32 {\n    return 1;\n}\n")
	expectPrinted(t, "fn add(a: i32, b: i32) -> i32 { return a + b; }",
		"fn add(a: i32, b: i32) -> i32 {\n    return a + b;\n}\n")
}

func TestEntryPointFunctions(t *testing.T) {
	expectPrinted(t, "@vertex fn main() -> @builtin(position) vec4f { return vec4f(); }",
		"@vertex fn main() -> @builtin(position) vec4f {\n    return vec4f();\n}\n")

	expectPrinted(t, "@fragment fn main() -> @location(0) vec4f { return vec4f(1.0); }",
		"@fragment fn main() -> @location(0) vec4f {\n    return vec4f(1.0);\n}\n")

	expectPrinted(t, "@compute @workgroup_size(64) fn main() {}",
		"@compute @workgroup_size(64) fn main() {\n}\n")
}

func TestFunctionWithParameterAttributes(t *testing.T) {
	expectPrinted(t,
		"@vertex fn main(@location(0) pos: vec4f) -> @builtin(position) vec4f { return pos; }",
		"@vertex fn main(@location(0) pos: vec4f) -> @builtin(position) vec4f {\n    return pos;\n}\n")
}

// ----------------------------------------------------------------------------
// Expression Tests
// ----------------------------------------------------------------------------

func TestBinaryExpressions(t *testing.T) {
	// Arithmetic
	expectPrinted(t, "const x = 1 + 2;", "const x = 1 + 2;\n")
	expectPrinted(t, "const x = 1 - 2;", "const x = 1 - 2;\n")
	expectPrinted(t, "const x = 1 * 2;", "const x = 1 * 2;\n")
	expectPrinted(t, "const x = 1 / 2;", "const x = 1 / 2;\n")
	expectPrinted(t, "const x = 1 % 2;", "const x = 1 % 2;\n")

	// Bitwise
	expectPrinted(t, "const x = 1 & 2;", "const x = 1 & 2;\n")
	expectPrinted(t, "const x = 1 | 2;", "const x = 1 | 2;\n")
	expectPrinted(t, "const x = 1 ^ 2;", "const x = 1 ^ 2;\n")
	expectPrinted(t, "const x = 1 << 2;", "const x = 1 << 2;\n")
	expectPrinted(t, "const x = 1 >> 2;", "const x = 1 >> 2;\n")

	// Comparison
	expectPrinted(t, "const x = 1 == 2;", "const x = 1 == 2;\n")
	expectPrinted(t, "const x = 1 != 2;", "const x = 1 != 2;\n")
	expectPrinted(t, "const x = 1 < 2;", "const x = 1 < 2;\n")
	expectPrinted(t, "const x = 1 <= 2;", "const x = 1 <= 2;\n")
	expectPrinted(t, "const x = 1 > 2;", "const x = 1 > 2;\n")
	expectPrinted(t, "const x = 1 >= 2;", "const x = 1 >= 2;\n")

	// Logical
	expectPrinted(t, "const x = true && false;", "const x = true && false;\n")
	expectPrinted(t, "const x = true || false;", "const x = true || false;\n")
}

func TestUnaryExpressions(t *testing.T) {
	expectPrinted(t, "const x = -1;", "const x = -1;\n")
	expectPrinted(t, "const x = !true;", "const x = !true;\n")
	expectPrinted(t, "const x = ~1;", "const x = ~1;\n")
}

func TestCallExpressions(t *testing.T) {
	expectPrinted(t, "const x = foo();", "const x = foo();\n")
	expectPrinted(t, "const x = foo(1);", "const x = foo(1);\n")
	expectPrinted(t, "const x = foo(1, 2);", "const x = foo(1, 2);\n")
	expectPrinted(t, "const x = foo(1, 2, 3);", "const x = foo(1, 2, 3);\n")
}

func TestTypeConstructors(t *testing.T) {
	expectPrinted(t, "const x = vec3f(1.0);", "const x = vec3f(1.0);\n")
	expectPrinted(t, "const x = vec3f(1.0, 2.0, 3.0);", "const x = vec3f(1.0, 2.0, 3.0);\n")
	expectPrinted(t, "const x = vec4f(v.xyz, 1.0);", "const x = vec4f(v.xyz, 1.0);\n")
	expectPrinted(t, "const x = mat4x4f();", "const x = mat4x4f();\n")
}

func TestMemberAccess(t *testing.T) {
	expectPrinted(t, "const x = a.b;", "const x = a.b;\n")
	expectPrinted(t, "const x = a.b.c;", "const x = a.b.c;\n")
	expectPrinted(t, "const x = v.xyz;", "const x = v.xyz;\n")
	expectPrinted(t, "const x = v.xyzw;", "const x = v.xyzw;\n")
}

func TestIndexAccess(t *testing.T) {
	expectPrinted(t, "const x = a[0];", "const x = a[0];\n")
	expectPrinted(t, "const x = a[i];", "const x = a[i];\n")
	expectPrinted(t, "const x = a[i + 1];", "const x = a[i + 1];\n")
	expectPrinted(t, "const x = a[0][1];", "const x = a[0][1];\n")
}

func TestParentheses(t *testing.T) {
	expectPrinted(t, "const x = (1);", "const x = (1);\n")
	expectPrinted(t, "const x = (1 + 2) * 3;", "const x = (1 + 2) * 3;\n")
	expectPrinted(t, "const x = a * (b + c);", "const x = a * (b + c);\n")
}

// ----------------------------------------------------------------------------
// Statement Tests
// ----------------------------------------------------------------------------

func TestReturnStatement(t *testing.T) {
	expectPrinted(t, "fn foo() { return; }", "fn foo() {\n    return;\n}\n")
	expectPrinted(t, "fn foo() -> i32 { return 1; }", "fn foo() -> i32 {\n    return 1;\n}\n")
}

func TestIfStatement(t *testing.T) {
	expectPrinted(t, "fn foo() { if true { return; } }",
		"fn foo() {\n    if true {\n        return;\n    }\n}\n")

	expectPrinted(t, "fn foo() { if true { return; } else { return; } }",
		"fn foo() {\n    if true {\n        return;\n    } else {\n        return;\n    }\n}\n")

	expectPrinted(t, "fn foo() { if a { } else if b { } else { } }",
		"fn foo() {\n    if a {\n    } else if b {\n    } else {\n    }\n}\n")
}

func TestForStatement(t *testing.T) {
	expectPrinted(t, "fn foo() { for (var i: i32 = 0; i < 4; i++) { } }",
		"fn foo() {\n    for (var i: i32 = 0; i < 4; i++) {\n    }\n}\n")

	expectPrinted(t, "fn foo() { for (var i = 0u; i < 10u; i++) { x++; } }",
		"fn foo() {\n    for (var i = 0u; i < 10u; i++) {\n        x++;\n    }\n}\n")

	expectPrinted(t, "fn foo() { for (var i: i32 = 0; i < 4; i += 2) { } }",
		"fn foo() {\n    for (var i: i32 = 0; i < 4; i += 2) {\n    }\n}\n")
}

func TestWhileStatement(t *testing.T) {
	expectPrinted(t, "fn foo() { while true { } }",
		"fn foo() {\n    while true {\n    }\n}\n")

	expectPrinted(t, "fn foo() { while x < 10 { x++; } }",
		"fn foo() {\n    while x < 10 {\n        x++;\n    }\n}\n")
}

func TestLoopStatement(t *testing.T) {
	expectPrinted(t, "fn foo() { loop { break; } }",
		"fn foo() {\n    loop {\n        break;\n    }\n}\n")

	expectPrinted(t, "fn foo() { loop { if x { break; } } }",
		"fn foo() {\n    loop {\n        if x {\n            break;\n        }\n    }\n}\n")
}

func TestSwitchStatement(t *testing.T) {
	expectPrinted(t, "fn foo() { switch x { case 1: { } default: { } } }",
		"fn foo() {\n    switch x {\n        case 1: {\n        }\n        default: {\n        }\n    }\n}\n")
}

func TestBreakContinue(t *testing.T) {
	expectPrinted(t, "fn foo() { loop { break; } }",
		"fn foo() {\n    loop {\n        break;\n    }\n}\n")

	expectPrinted(t, "fn foo() { loop { continue; } }",
		"fn foo() {\n    loop {\n        continue;\n    }\n}\n")
}

func TestDiscard(t *testing.T) {
	expectPrinted(t, "@fragment fn main() { discard; }",
		"@fragment fn main() {\n    discard;\n}\n")
}

func TestAssignment(t *testing.T) {
	expectPrinted(t, "fn foo() { x = 1; }", "fn foo() {\n    x = 1;\n}\n")
	expectPrinted(t, "fn foo() { x += 1; }", "fn foo() {\n    x += 1;\n}\n")
	expectPrinted(t, "fn foo() { x -= 1; }", "fn foo() {\n    x -= 1;\n}\n")
	expectPrinted(t, "fn foo() { x *= 2; }", "fn foo() {\n    x *= 2;\n}\n")
	expectPrinted(t, "fn foo() { x /= 2; }", "fn foo() {\n    x /= 2;\n}\n")
}

func TestIncrementDecrement(t *testing.T) {
	expectPrinted(t, "fn foo() { x++; }", "fn foo() {\n    x++;\n}\n")
	expectPrinted(t, "fn foo() { x--; }", "fn foo() {\n    x--;\n}\n")
}

// ----------------------------------------------------------------------------
// Type Tests
// ----------------------------------------------------------------------------

func TestScalarTypes(t *testing.T) {
	expectPrinted(t, "var x: bool;", "var x: bool;\n")
	expectPrinted(t, "var x: i32;", "var x: i32;\n")
	expectPrinted(t, "var x: u32;", "var x: u32;\n")
	expectPrinted(t, "var x: f32;", "var x: f32;\n")
	expectPrinted(t, "var x: f16;", "var x: f16;\n")
}

func TestVectorTypes(t *testing.T) {
	expectPrinted(t, "var x: vec2<f32>;", "var x: vec2<f32>;\n")
	expectPrinted(t, "var x: vec3<f32>;", "var x: vec3<f32>;\n")
	expectPrinted(t, "var x: vec4<f32>;", "var x: vec4<f32>;\n")
	expectPrinted(t, "var x: vec2f;", "var x: vec2f;\n")
	expectPrinted(t, "var x: vec3f;", "var x: vec3f;\n")
	expectPrinted(t, "var x: vec4f;", "var x: vec4f;\n")
	expectPrinted(t, "var x: vec3i;", "var x: vec3i;\n")
	expectPrinted(t, "var x: vec3u;", "var x: vec3u;\n")
}

func TestMatrixTypes(t *testing.T) {
	// Shorthand types
	expectPrinted(t, "var x: mat4x4f;", "var x: mat4x4f;\n")
	// Generic matrix types
	expectPrinted(t, "var x: mat2x2<f32>;", "var x: mat2x2<f32>;\n")
	expectPrinted(t, "var x: mat3x3<f32>;", "var x: mat3x3<f32>;\n")
	expectPrinted(t, "var x: mat4x4<f32>;", "var x: mat4x4<f32>;\n")
	expectPrinted(t, "var x: mat2x3<f32>;", "var x: mat2x3<f32>;\n")
}

func TestArrayTypes(t *testing.T) {
	// Runtime-sized array
	expectPrinted(t, "var x: array<f32>;", "var x: array<f32>;\n")
	// Sized arrays
	expectPrinted(t, "var x: array<f32, 10>;", "var x: array<f32, 10>;\n")
	expectPrinted(t, "var x: array<vec3<f32>, 8>;", "var x: array<vec3<f32>, 8>;\n")
}

func TestPointerTypes(t *testing.T) {
	expectPrinted(t, "var x: ptr<function, f32>;", "var x: ptr<function, f32>;\n")
	expectPrinted(t, "var x: ptr<private, i32>;", "var x: ptr<private, i32>;\n")
	expectPrinted(t, "var x: ptr<storage, f32, read_write>;", "var x: ptr<storage, f32, read_write>;\n")
}

func TestAtomicTypes(t *testing.T) {
	expectPrinted(t, "var x: atomic<i32>;", "var x: atomic<i32>;\n")
	expectPrinted(t, "var x: atomic<u32>;", "var x: atomic<u32>;\n")
}

func TestTextureTypes(t *testing.T) {
	expectPrinted(t, "var tex: texture_2d<f32>;", "var tex: texture_2d<f32>;\n")
	expectPrinted(t, "var tex: texture_3d<f32>;", "var tex: texture_3d<f32>;\n")
	expectPrinted(t, "var tex: texture_cube<f32>;", "var tex: texture_cube<f32>;\n")
}

func TestSamplerTypes(t *testing.T) {
	expectPrinted(t, "var s: sampler;", "var s: sampler;\n")
	expectPrinted(t, "var s: sampler_comparison;", "var s: sampler_comparison;\n")
}

// ----------------------------------------------------------------------------
// Directive Tests
// ----------------------------------------------------------------------------

func TestEnableDirective(t *testing.T) {
	expectPrinted(t, "enable f16;", "enable f16;\n")
	expectPrinted(t, "enable f16, dual_source_blending;", "enable f16, dual_source_blending;\n")
}

func TestRequiresDirective(t *testing.T) {
	expectPrinted(t, "requires readonly_and_readwrite_storage_textures;",
		"requires readonly_and_readwrite_storage_textures;\n")
}

func TestDiagnosticDirective(t *testing.T) {
	expectPrinted(t, "diagnostic(off, derivative_uniformity);",
		"diagnostic(off, derivative_uniformity);\n")
}

// ----------------------------------------------------------------------------
// Const Assert Tests
// ----------------------------------------------------------------------------

func TestConstAssert(t *testing.T) {
	expectPrinted(t, "const_assert 1 == 1;", "const_assert 1 == 1;\n")
	expectPrinted(t, "const_assert SIZE > 0;", "const_assert SIZE > 0;\n")
}

// ----------------------------------------------------------------------------
// Templated Constructor Tests
// ----------------------------------------------------------------------------

func TestTemplatedConstructors(t *testing.T) {
	// Templated type constructors must preserve their type parameters
	// to avoid type inference issues (e.g., vec3(0) would be vec3<i32>)
	expectPrinted(t, "var x = vec3<f32>(0);", "var x = vec3<f32>(0);\n")
	expectPrinted(t, "var x = vec2<i32>(1, 2);", "var x = vec2<i32>(1, 2);\n")
	expectPrinted(t, "var x = vec4<u32>(0, 0, 0, 1);", "var x = vec4<u32>(0, 0, 0, 1);\n")
	expectPrinted(t, "var x = mat2x2<f32>(1, 0, 0, 1);", "var x = mat2x2<f32>(1, 0, 0, 1);\n")
	expectPrinted(t, "var x = array<f32, 4>(1.0, 2.0, 3.0, 4.0);", "var x = array<f32,4>(1.0, 2.0, 3.0, 4.0);\n")
}

// ----------------------------------------------------------------------------
// Complex Example Tests
// ----------------------------------------------------------------------------

func TestCompleteVertexShader(t *testing.T) {
	input := `struct VertexOutput {
	@builtin(position) pos: vec4f,
	@location(0) color: vec3f,
}

@vertex
fn main(@location(0) position: vec3f) -> VertexOutput {
	var output: VertexOutput;
	output.pos = vec4f(position, 1.0);
	output.color = vec3f(1.0, 0.0, 0.0);
	return output;
}`

	// Just verify it parses without errors
	p := New(input)
	_, errs := p.Parse()
	if len(errs) > 0 {
		t.Errorf("parse errors: %v", errs)
	}
}

func TestCompleteComputeShader(t *testing.T) {
	input := `@group(0) @binding(0) var<storage, read_write> data: array<f32>;

@compute @workgroup_size(64)
fn main(@builtin(global_invocation_id) id: vec3u) {
	let index = id.x;
	if index < arrayLength(&data) {
		data[index] = data[index] * 2.0;
	}
}`

	p := New(input)
	_, errs := p.Parse()
	if len(errs) > 0 {
		t.Errorf("parse errors: %v", errs)
	}
}
