package parser

import (
	"strings"
	"testing"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/lexer"
	"github.com/HugoDaniel/miniray/internal/printer"
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
	expectPrinted(t, "var x = array<f32, 4>(1.0, 2.0, 3.0, 4.0);", "var x = array<f32, 4>(1.0, 2.0, 3.0, 4.0);\n")
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

// ----------------------------------------------------------------------------
// Parse Error Tests
// ----------------------------------------------------------------------------

func TestInvalidTypeErrors(t *testing.T) {
	// Number used as type should produce error (not hang)
	expectParseError(t, "struct Foo { x: 12341234 }", "expected type")
	expectParseError(t, "var x: 999;", "expected type")
	expectParseError(t, "fn foo(x: 123) {}", "expected type")
	expectParseError(t, "fn foo() -> 456 {}", "expected type")

	// Invalid type in generic
	expectParseError(t, "var x: vec3<123>;", "expected type")
	expectParseError(t, "var x: array<456>;", "expected type")
}

func TestMissingSemicolonErrors(t *testing.T) {
	expectParseError(t, "const x = 1", "expected ;")
	expectParseError(t, "var x: f32", "expected ;")
}

func TestMissingBraceErrors(t *testing.T) {
	expectParseError(t, "fn foo() { return;", "expected }")
	expectParseError(t, "struct Foo { x: f32", "expected }")
}

func TestInvalidExpressionErrors(t *testing.T) {
	expectNoParse(t, "const x = ;")
	expectNoParse(t, "const x = 1 +;")
}

func TestInvalidStatementInBlock(t *testing.T) {
	// Garbage inside a function body should error, not loop forever
	expectNoParse(t, "fn foo() { 12345 }")
	expectNoParse(t, "fn foo() { !@#$ }")
}

func TestInvalidSwitchStatement(t *testing.T) {
	// Missing case/default keyword
	expectNoParse(t, "fn foo() { switch x { 1: {} } }")
}

func TestInvalidDirective(t *testing.T) {
	// Invalid enable directive (missing feature name after comma)
	expectNoParse(t, "enable f16, ;")
}

// ----------------------------------------------------------------------------
// ParseError Tests
// ----------------------------------------------------------------------------

func TestParseErrorString(t *testing.T) {
	// Test the Error() method of ParseError
	err := ParseError{
		Message: "unexpected token",
		Pos:     10,
		Line:    5,
		Column:  3,
	}
	expected := "5:3: unexpected token"
	if err.Error() != expected {
		t.Errorf("ParseError.Error() = %q, want %q", err.Error(), expected)
	}
}

// ----------------------------------------------------------------------------
// GetConstValue Tests
// ----------------------------------------------------------------------------

func TestGetConstValue(t *testing.T) {
	// Parse a const declaration to populate constValues
	input := "const X = 42;"
	p := New(input)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// Find the const symbol
	var constRef ast.Ref
	for i, sym := range module.Symbols {
		if sym.OriginalName == "X" {
			constRef = ast.Ref{InnerIndex: uint32(i)}
			break
		}
	}

	if !constRef.IsValid() {
		t.Fatal("could not find symbol X")
	}

	// Get the const value
	val, ok := p.GetConstValue(constRef)
	if !ok {
		t.Error("GetConstValue returned false for const X")
	}
	if val.Int != 42 {
		t.Errorf("GetConstValue X = %d, want 42", val.Int)
	}
}

func TestGetConstValueNotFound(t *testing.T) {
	// Test GetConstValue with non-existent ref
	p := New("")
	p.Parse()

	val, ok := p.GetConstValue(ast.Ref{InnerIndex: 999})
	if ok {
		t.Error("GetConstValue should return false for non-existent ref")
	}
	if val.Kind != 0 {
		t.Error("GetConstValue should return zero value for non-existent ref")
	}
}

// ----------------------------------------------------------------------------
// Texture Type Tests (comprehensive coverage)
// ----------------------------------------------------------------------------

func TestAllTextureTypes(t *testing.T) {
	// Sampled textures
	expectPrinted(t, "var tex: texture_1d<f32>;", "var tex: texture_1d<f32>;\n")
	expectPrinted(t, "var tex: texture_2d<f32>;", "var tex: texture_2d<f32>;\n")
	expectPrinted(t, "var tex: texture_2d_array<f32>;", "var tex: texture_2d_array<f32>;\n")
	expectPrinted(t, "var tex: texture_3d<f32>;", "var tex: texture_3d<f32>;\n")
	expectPrinted(t, "var tex: texture_cube<f32>;", "var tex: texture_cube<f32>;\n")
	expectPrinted(t, "var tex: texture_cube_array<f32>;", "var tex: texture_cube_array<f32>;\n")

	// Multisampled texture
	expectPrinted(t, "var tex: texture_multisampled_2d<f32>;", "var tex: texture_multisampled_2d<f32>;\n")

	// Storage textures with format and access mode
	expectPrinted(t, "var tex: texture_storage_1d<rgba8unorm, write>;", "var tex: texture_storage_1d<rgba8unorm, write>;\n")
	expectPrinted(t, "var tex: texture_storage_2d<rgba8unorm, read>;", "var tex: texture_storage_2d<rgba8unorm, read>;\n")
	expectPrinted(t, "var tex: texture_storage_2d_array<rgba8unorm, read_write>;", "var tex: texture_storage_2d_array<rgba8unorm, read_write>;\n")
	expectPrinted(t, "var tex: texture_storage_3d<rgba32float, write>;", "var tex: texture_storage_3d<rgba32float, write>;\n")

	// Depth textures
	expectPrinted(t, "var tex: texture_depth_2d;", "var tex: texture_depth_2d;\n")
	expectPrinted(t, "var tex: texture_depth_2d_array;", "var tex: texture_depth_2d_array;\n")
	expectPrinted(t, "var tex: texture_depth_cube;", "var tex: texture_depth_cube;\n")
	expectPrinted(t, "var tex: texture_depth_cube_array;", "var tex: texture_depth_cube_array;\n")
	expectPrinted(t, "var tex: texture_depth_multisampled_2d;", "var tex: texture_depth_multisampled_2d;\n")
}

// ----------------------------------------------------------------------------
// Template Expression Tests
// ----------------------------------------------------------------------------

func TestTemplateAdditiveExpressions(t *testing.T) {
	// Addition in template arguments
	expectPrinted(t, "var x: array<f32, 10 + 5>;", "var x: array<f32, 10 + 5>;\n")
	// Subtraction in template arguments
	expectPrinted(t, "var x: array<f32, 20 - 5>;", "var x: array<f32, 20 - 5>;\n")
}

func TestTemplateMultiplicativeExpressions(t *testing.T) {
	// Multiplication in template arguments
	expectPrinted(t, "var x: array<f32, 2 * 8>;", "var x: array<f32, 2 * 8>;\n")
	// Division in template arguments
	expectPrinted(t, "var x: array<f32, 16 / 2>;", "var x: array<f32, 16 / 2>;\n")
	// Modulo in template arguments
	expectPrinted(t, "var x: array<f32, 17 % 5>;", "var x: array<f32, 17 % 5>;\n")
}

func TestTemplateUnaryExpressions(t *testing.T) {
	// Negation in template arguments
	expectPrinted(t, "var x: array<f32, -10>;", "var x: array<f32, -10>;\n")
	// Logical not in template arguments (uncommon but valid)
	expectPrinted(t, "const x = array<bool, 2>(!true, !false);", "const x = array<bool, 2>(!true, !false);\n")
	// Bitwise not in template arguments
	expectPrinted(t, "var x: array<i32, ~0>;", "var x: array<i32, ~0>;\n")
}

func TestTemplateParenthesesExpressions(t *testing.T) {
	// Parenthesized expressions in templates
	expectPrinted(t, "var x: array<f32, (10 + 5)>;", "var x: array<f32, (10 + 5)>;\n")
	expectPrinted(t, "var x: array<f32, (2 + 3) * 4>;", "var x: array<f32, (2 + 3) * 4>;\n")
}

func TestTemplateComplexExpressions(t *testing.T) {
	// Complex expression combining multiple operators
	expectPrinted(t, "var x: array<f32, 2 + 3 * 4>;", "var x: array<f32, 2 + 3 * 4>;\n")
	expectPrinted(t, "var x: array<f32, (2 + 3) * 4 - 1>;", "var x: array<f32, (2 + 3) * 4 - 1>;\n")
}

func TestTemplateIdentifierExpressions(t *testing.T) {
	// Identifier in template arguments
	input := `const N = 10;
var x: array<f32, N>;`
	expected := `const N = 10;

var x: array<f32, N>;
`
	expectPrinted(t, input, expected)
}

func TestTemplateBoolLiterals(t *testing.T) {
	// Boolean literals in template context
	expectPrinted(t, "const x = vec2<bool>(true, false);", "const x = vec2<bool>(true, false);\n")
}

// ----------------------------------------------------------------------------
// For Loop Update Statement Tests
// ----------------------------------------------------------------------------

func TestForLoopUpdateStatements(t *testing.T) {
	// All compound assignment operators in for loop update
	expectPrinted(t, "fn foo() { for (var i = 0; i < 10; i = i + 1) {} }",
		"fn foo() {\n    for (var i = 0; i < 10; i = i + 1) {\n    }\n}\n")
	expectPrinted(t, "fn foo() { for (var i = 0; i < 10; i += 1) {} }",
		"fn foo() {\n    for (var i = 0; i < 10; i += 1) {\n    }\n}\n")
	expectPrinted(t, "fn foo() { for (var i = 10; i > 0; i -= 1) {} }",
		"fn foo() {\n    for (var i = 10; i > 0; i -= 1) {\n    }\n}\n")
	expectPrinted(t, "fn foo() { for (var i = 1; i < 100; i *= 2) {} }",
		"fn foo() {\n    for (var i = 1; i < 100; i *= 2) {\n    }\n}\n")
	expectPrinted(t, "fn foo() { for (var i = 100; i > 1; i /= 2) {} }",
		"fn foo() {\n    for (var i = 100; i > 1; i /= 2) {\n    }\n}\n")
	expectPrinted(t, "fn foo() { for (var i = 0; i < 10; i %= 3) {} }",
		"fn foo() {\n    for (var i = 0; i < 10; i %= 3) {\n    }\n}\n")
	// Bitwise compound assignments
	expectPrinted(t, "fn foo() { for (var i = 0xFFu; i > 0u; i &= 0x7Fu) {} }",
		"fn foo() {\n    for (var i = 0xFFu; i > 0u; i &= 0x7Fu) {\n    }\n}\n")
	expectPrinted(t, "fn foo() { for (var i = 0u; i < 255u; i |= 1u) {} }",
		"fn foo() {\n    for (var i = 0u; i < 255u; i |= 1u) {\n    }\n}\n")
	expectPrinted(t, "fn foo() { for (var i = 0u; i < 255u; i ^= 1u) {} }",
		"fn foo() {\n    for (var i = 0u; i < 255u; i ^= 1u) {\n    }\n}\n")
	// Shift compound assignments
	expectPrinted(t, "fn foo() { for (var i = 1u; i < 256u; i <<= 1u) {} }",
		"fn foo() {\n    for (var i = 1u; i < 256u; i <<= 1u) {\n    }\n}\n")
	expectPrinted(t, "fn foo() { for (var i = 256u; i > 0u; i >>= 1u) {} }",
		"fn foo() {\n    for (var i = 256u; i > 0u; i >>= 1u) {\n    }\n}\n")
	// Decrement
	expectPrinted(t, "fn foo() { for (var i = 10; i > 0; i--) {} }",
		"fn foo() {\n    for (var i = 10; i > 0; i--) {\n    }\n}\n")
	// Function call as update
	expectPrinted(t, "fn foo() { for (var i = 0; i < 10; update()) {} }",
		"fn foo() {\n    for (var i = 0; i < 10; update()) {\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// Access Mode Tests
// ----------------------------------------------------------------------------

func TestAccessModes(t *testing.T) {
	expectPrinted(t, "var<storage, read> x: f32;", "var<storage, read> x: f32;\n")
	expectPrinted(t, "var<storage, write> x: f32;", "var<storage, write> x: f32;\n")
	expectPrinted(t, "var<storage, read_write> x: f32;", "var<storage, read_write> x: f32;\n")
}

// ----------------------------------------------------------------------------
// Address Space Tests
// ----------------------------------------------------------------------------

func TestAddressSpaces(t *testing.T) {
	expectPrinted(t, "var<function> x: f32;", "var<function> x: f32;\n")
	expectPrinted(t, "var<private> x: f32;", "var<private> x: f32;\n")
	expectPrinted(t, "var<workgroup> x: f32;", "var<workgroup> x: f32;\n")
	expectPrinted(t, "var<uniform> x: f32;", "var<uniform> x: f32;\n")
	expectPrinted(t, "var<storage> x: f32;", "var<storage> x: f32;\n")
}

// ----------------------------------------------------------------------------
// Loop Continuing Tests
// ----------------------------------------------------------------------------

func TestLoopContinuing(t *testing.T) {
	expectPrinted(t, "fn foo() { loop { break; } continuing { i++; } }",
		"fn foo() {\n    loop {\n        break;\n    } continuing {\n        i++;\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// Switch Statement Edge Cases
// ----------------------------------------------------------------------------

func TestSwitchMultipleCases(t *testing.T) {
	expectPrinted(t, "fn foo() { switch x { case 1, 2, 3: { } default: { } } }",
		"fn foo() {\n    switch x {\n        case 1, 2, 3: {\n        }\n        default: {\n        }\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// Expression/Assignment Statement Tests
// ----------------------------------------------------------------------------

func TestCompoundAssignmentStatements(t *testing.T) {
	// Test all compound assignment operators in regular statements
	expectPrinted(t, "fn foo() { x %= 3; }", "fn foo() {\n    x %= 3;\n}\n")
	expectPrinted(t, "fn foo() { x &= 0xFF; }", "fn foo() {\n    x &= 0xFF;\n}\n")
	expectPrinted(t, "fn foo() { x |= 1; }", "fn foo() {\n    x |= 1;\n}\n")
	expectPrinted(t, "fn foo() { x ^= 0xF; }", "fn foo() {\n    x ^= 0xF;\n}\n")
	expectPrinted(t, "fn foo() { x <<= 2u; }", "fn foo() {\n    x <<= 2u;\n}\n")
	expectPrinted(t, "fn foo() { x >>= 2u; }", "fn foo() {\n    x >>= 2u;\n}\n")
}

// ----------------------------------------------------------------------------
// Pointer/Reference Expression Tests
// ----------------------------------------------------------------------------

func TestAddressOfAndDeref(t *testing.T) {
	expectPrinted(t, "fn foo() { let p = &x; }", "fn foo() {\n    let p = &x;\n}\n")
	expectPrinted(t, "fn foo() { let v = *p; }", "fn foo() {\n    let v = *p;\n}\n")
}

// ----------------------------------------------------------------------------
// Multiple Template Arguments
// ----------------------------------------------------------------------------

func TestMultipleTemplateArgs(t *testing.T) {
	// ptr with address space, type, and access mode
	expectPrinted(t, "var x: ptr<storage, f32, read_write>;", "var x: ptr<storage, f32, read_write>;\n")
}

// ----------------------------------------------------------------------------
// Error Recovery Tests
// ----------------------------------------------------------------------------

func TestInvalidTemplateExpression(t *testing.T) {
	// Invalid expression in template argument
	expectNoParse(t, "var x: array<f32, @>;")
}

func TestInvalidForLoopUpdate(t *testing.T) {
	// Invalid statement in for loop update
	expectNoParse(t, "fn foo() { for (var i = 0; i < 10; @invalid) {} }")
}

// ----------------------------------------------------------------------------
// Const Assert Edge Cases
// ----------------------------------------------------------------------------

func TestConstAssertWithExpression(t *testing.T) {
	expectPrinted(t, "const_assert true;", "const_assert true;\n")
	expectPrinted(t, "const_assert 1 + 1 == 2;", "const_assert 1 + 1 == 2;\n")
}

// ----------------------------------------------------------------------------
// Peek at EOF Test
// ----------------------------------------------------------------------------

func TestPeekAtEOF(t *testing.T) {
	// Test peeking beyond end of tokens
	p := New("")
	p.Parse()

	// Peek should return EOF for any offset past the end
	tok := p.peek(0)
	if tok.Kind != lexer.TokEOF {
		t.Errorf("peek(0) on empty input should return EOF, got %v", tok.Kind)
	}

	tok = p.peek(100)
	if tok.Kind != lexer.TokEOF {
		t.Errorf("peek(100) on empty input should return EOF, got %v", tok.Kind)
	}
}

// ----------------------------------------------------------------------------
// Storage Texture Without Access Mode
// ----------------------------------------------------------------------------

func TestStorageTextureWithoutAccessMode(t *testing.T) {
	// Storage texture with just format, no access mode - printer adds trailing comma
	expectPrinted(t, "var tex: texture_storage_2d<rgba8unorm>;", "var tex: texture_storage_2d<rgba8unorm, >;\n")
}

// ----------------------------------------------------------------------------
// Address Space None Test
// ----------------------------------------------------------------------------

func TestVarWithoutAddressSpace(t *testing.T) {
	// Variable without explicit address space template
	expectPrinted(t, "var x: i32 = 0;", "var x: i32 = 0;\n")
}

// ----------------------------------------------------------------------------
// Additional Statement Types
// ----------------------------------------------------------------------------

func TestBreakIfStatement(t *testing.T) {
	// break if statement in loop continuing block
	expectPrinted(t, "fn foo() { loop { } continuing { break if true; } }",
		"fn foo() {\n    loop {\n    } continuing {\n        break if true;\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// Enable Directive Edge Cases
// ----------------------------------------------------------------------------

func TestEnableMultipleFeatures(t *testing.T) {
	// Enable multiple features
	expectPrinted(t, "enable f16, subgroups;", "enable f16, subgroups;\n")
}

// ----------------------------------------------------------------------------
// Additional Const Expression Tests
// ----------------------------------------------------------------------------

func TestConstExprFloat(t *testing.T) {
	// Floating point const expression
	p := New("const PI = 3.14;")
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// Find the const symbol
	for i, sym := range module.Symbols {
		if sym.OriginalName == "PI" {
			val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
			if !ok {
				t.Error("GetConstValue returned false for const PI")
			}
			if val.Float != 3.14 {
				t.Errorf("GetConstValue PI = %f, want 3.14", val.Float)
			}
			return
		}
	}
	t.Fatal("could not find symbol PI")
}

func TestConstExprBool(t *testing.T) {
	// Boolean const expression
	p := New("const B = true;")
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// Find the const symbol
	for i, sym := range module.Symbols {
		if sym.OriginalName == "B" {
			val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
			if !ok {
				t.Error("GetConstValue returned false for const B")
			}
			if val.Bool != true {
				t.Errorf("GetConstValue B = %v, want true", val.Bool)
			}
			return
		}
	}
	t.Fatal("could not find symbol B")
}

func TestConstExprBinaryOp(t *testing.T) {
	// Binary operation const expression
	p := New("const X = 10 + 5;")
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// Find the const symbol
	for i, sym := range module.Symbols {
		if sym.OriginalName == "X" {
			val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
			if !ok {
				t.Error("GetConstValue returned false for const X")
			}
			if val.Int != 15 {
				t.Errorf("GetConstValue X = %d, want 15", val.Int)
			}
			return
		}
	}
	t.Fatal("could not find symbol X")
}

func TestConstExprUnaryOp(t *testing.T) {
	// Unary operation const expression
	p := New("const X = -42;")
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// Find the const symbol
	for i, sym := range module.Symbols {
		if sym.OriginalName == "X" {
			val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
			if !ok {
				t.Error("GetConstValue returned false for const X")
			}
			if val.Int != -42 {
				t.Errorf("GetConstValue X = %d, want -42", val.Int)
			}
			return
		}
	}
	t.Fatal("could not find symbol X")
}

func TestConstExprParens(t *testing.T) {
	// Parenthesized const expression
	p := New("const X = (7);")
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// Find the const symbol
	for i, sym := range module.Symbols {
		if sym.OriginalName == "X" {
			val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
			if !ok {
				t.Error("GetConstValue returned false for const X")
			}
			if val.Int != 7 {
				t.Errorf("GetConstValue X = %d, want 7", val.Int)
			}
			return
		}
	}
	t.Fatal("could not find symbol X")
}

func TestConstExprFloatNeg(t *testing.T) {
	// Float negation const expression
	p := New("const X = -3.14;")
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	for i, sym := range module.Symbols {
		if sym.OriginalName == "X" {
			val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
			if !ok {
				t.Error("GetConstValue returned false for const X")
			}
			if val.Float != -3.14 {
				t.Errorf("GetConstValue X = %f, want -3.14", val.Float)
			}
			return
		}
	}
	t.Fatal("could not find symbol X")
}

func TestConstExprBoolNot(t *testing.T) {
	// Boolean not const expression
	p := New("const X = !false;")
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	for i, sym := range module.Symbols {
		if sym.OriginalName == "X" {
			val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
			if !ok {
				t.Error("GetConstValue returned false for const X")
			}
			if val.Bool != true {
				t.Errorf("GetConstValue X = %v, want true", val.Bool)
			}
			return
		}
	}
	t.Fatal("could not find symbol X")
}

func TestConstExprReference(t *testing.T) {
	// Const expression referencing another const
	input := `const A = 10;
const B = A;`
	p := New(input)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	for i, sym := range module.Symbols {
		if sym.OriginalName == "B" {
			val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
			if !ok {
				t.Error("GetConstValue returned false for const B")
			}
			if val.Int != 10 {
				t.Errorf("GetConstValue B = %d, want 10", val.Int)
			}
			return
		}
	}
	t.Fatal("could not find symbol B")
}

func TestConstExprIntOps(t *testing.T) {
	// Various integer binary operations
	tests := []struct {
		input    string
		expected int64
	}{
		{"const X = 10 - 3;", 7},
		{"const X = 3 * 4;", 12},
		{"const X = 12 / 3;", 4},
		{"const X = 10 % 3;", 1},
		{"const X = 5 & 3;", 1},
		{"const X = 5 | 2;", 7},
		{"const X = 5 ^ 3;", 6},
		{"const X = 1 << 3;", 8},
		{"const X = 8 >> 2;", 2},
	}

	for _, tt := range tests {
		p := New(tt.input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("%s: parse errors: %v", tt.input, errs)
		}

		for i, sym := range module.Symbols {
			if sym.OriginalName == "X" {
				val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
				if !ok {
					t.Errorf("%s: GetConstValue returned false", tt.input)
					break
				}
				if val.Int != tt.expected {
					t.Errorf("%s: got %d, want %d", tt.input, val.Int, tt.expected)
				}
				break
			}
		}
	}
}

func TestConstExprCompare(t *testing.T) {
	// Comparison operations returning bool
	tests := []struct {
		input    string
		expected bool
	}{
		{"const X = 5 == 5;", true},
		{"const X = 5 != 3;", true},
		{"const X = 3 < 5;", true},
		{"const X = 3 <= 3;", true},
		{"const X = 5 > 3;", true},
		{"const X = 5 >= 5;", true},
	}

	for _, tt := range tests {
		p := New(tt.input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("%s: parse errors: %v", tt.input, errs)
		}

		for i, sym := range module.Symbols {
			if sym.OriginalName == "X" {
				val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
				if !ok {
					t.Errorf("%s: GetConstValue returned false", tt.input)
					break
				}
				if val.Bool != tt.expected {
					t.Errorf("%s: got %v, want %v", tt.input, val.Bool, tt.expected)
				}
				break
			}
		}
	}
}

func TestConstExprFloatOps(t *testing.T) {
	// Float binary operations
	tests := []struct {
		input    string
		expected float64
	}{
		{"const X = 1.0 + 2.0;", 3.0},
		{"const X = 5.0 - 2.0;", 3.0},
		{"const X = 2.0 * 3.0;", 6.0},
		{"const X = 6.0 / 2.0;", 3.0},
	}

	for _, tt := range tests {
		p := New(tt.input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("%s: parse errors: %v", tt.input, errs)
		}

		for i, sym := range module.Symbols {
			if sym.OriginalName == "X" {
				val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
				if !ok {
					t.Errorf("%s: GetConstValue returned false", tt.input)
					break
				}
				if val.Float != tt.expected {
					t.Errorf("%s: got %f, want %f", tt.input, val.Float, tt.expected)
				}
				break
			}
		}
	}
}

func TestConstExprBoolOps(t *testing.T) {
	// Boolean binary operations
	tests := []struct {
		input    string
		expected bool
	}{
		{"const X = true && true;", true},
		{"const X = true && false;", false},
		{"const X = false || true;", true},
		{"const X = true == true;", true},
		{"const X = true != false;", true},
	}

	for _, tt := range tests {
		p := New(tt.input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("%s: parse errors: %v", tt.input, errs)
		}

		for i, sym := range module.Symbols {
			if sym.OriginalName == "X" {
				val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
				if !ok {
					t.Errorf("%s: GetConstValue returned false", tt.input)
					break
				}
				if val.Bool != tt.expected {
					t.Errorf("%s: got %v, want %v", tt.input, val.Bool, tt.expected)
				}
				break
			}
		}
	}
}

func TestConstExprFloatCompare(t *testing.T) {
	// Float comparison operations
	tests := []struct {
		input    string
		expected bool
	}{
		{"const X = 1.0 == 1.0;", true},
		{"const X = 1.0 != 2.0;", true},
		{"const X = 1.0 < 2.0;", true},
		{"const X = 1.0 <= 1.0;", true},
		{"const X = 2.0 > 1.0;", true},
		{"const X = 2.0 >= 2.0;", true},
	}

	for _, tt := range tests {
		p := New(tt.input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("%s: parse errors: %v", tt.input, errs)
		}

		for i, sym := range module.Symbols {
			if sym.OriginalName == "X" {
				val, ok := p.GetConstValue(ast.Ref{InnerIndex: uint32(i)})
				if !ok {
					t.Errorf("%s: GetConstValue returned false", tt.input)
					break
				}
				if val.Bool != tt.expected {
					t.Errorf("%s: got %v, want %v", tt.input, val.Bool, tt.expected)
				}
				break
			}
		}
	}
}

// ----------------------------------------------------------------------------
// Declaration Errors
// ----------------------------------------------------------------------------

func TestInvalidDeclaration(t *testing.T) {
	// Invalid declaration syntax
	expectNoParse(t, "fn foo() { 12345 }")
}

// ----------------------------------------------------------------------------
// Call Statement Tests
// ----------------------------------------------------------------------------

func TestCallStatement(t *testing.T) {
	// Function call as statement (void return)
	expectPrinted(t, "fn foo() { bar(); }", "fn foo() {\n    bar();\n}\n")
}

// ----------------------------------------------------------------------------
// Additional Switch Tests
// ----------------------------------------------------------------------------

func TestSwitchOnlyDefault(t *testing.T) {
	// Switch with only default case
	expectPrinted(t, "fn foo() { switch x { default: { } } }",
		"fn foo() {\n    switch x {\n        default: {\n        }\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// Templated Constructor Edge Cases
// ----------------------------------------------------------------------------

func TestTemplatedConstructorWithGeneric(t *testing.T) {
	// Templated constructor with generic type
	expectPrinted(t, "const x = vec3<f32>(1.0, 2.0, 3.0);", "const x = vec3<f32>(1.0, 2.0, 3.0);\n")
	expectPrinted(t, "const x = array<i32, 3>(1, 2, 3);", "const x = array<i32, 3>(1, 2, 3);\n")
}

// ----------------------------------------------------------------------------
// For Loop Without Init/Update
// ----------------------------------------------------------------------------

func TestForLoopEmpty(t *testing.T) {
	// Empty for loop clauses
	expectPrinted(t, "fn foo() { for (;;) { break; } }",
		"fn foo() {\n    for (; ; ) {\n        break;\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// ConstAssert Errors
// ----------------------------------------------------------------------------

func TestConstAssertMissingSemi(t *testing.T) {
	// const_assert without semicolon
	expectNoParse(t, "const_assert true")
}

// ----------------------------------------------------------------------------
// Compound Statement Errors
// ----------------------------------------------------------------------------

func TestUnclosedCompoundStatement(t *testing.T) {
	// Missing closing brace
	expectNoParse(t, "fn foo() {")
}
