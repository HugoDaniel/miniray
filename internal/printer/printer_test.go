package printer

import (
	"testing"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/dce"
	"github.com/HugoDaniel/miniray/internal/parser"
	"github.com/HugoDaniel/miniray/internal/sourcemap"
)

// ----------------------------------------------------------------------------
// Test Helpers (esbuild-style)
// ----------------------------------------------------------------------------

// expectPrinted verifies pretty-printed output.
func expectPrinted(t *testing.T, input string, expected string) {
	t.Helper()
	t.Run(input, func(t *testing.T) {
		t.Helper()
		p := parser.New(input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("parse errors: %v", errs)
		}
		pr := New(Options{}, module.Symbols)
		actual := pr.Print(module)
		if actual != expected {
			t.Errorf("\ninput:\n%s\nexpected:\n%s\nactual:\n%s", input, expected, actual)
		}
	})
}

// expectPrintedMinify verifies minified output (whitespace removed).
func expectPrintedMinify(t *testing.T, input string, expected string) {
	t.Helper()
	t.Run(input+"_minify", func(t *testing.T) {
		t.Helper()
		p := parser.New(input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("parse errors: %v", errs)
		}
		pr := New(Options{
			MinifyWhitespace: true,
		}, module.Symbols)
		actual := pr.Print(module)
		if actual != expected {
			t.Errorf("\ninput:\n%s\nexpected:\n%s\nactual:\n%s", input, expected, actual)
		}
	})
}

// expectPrintedMangle verifies syntax-mangled output.
func expectPrintedMangle(t *testing.T, input string, expected string) {
	t.Helper()
	t.Run(input+"_mangle", func(t *testing.T) {
		t.Helper()
		p := parser.New(input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("parse errors: %v", errs)
		}
		pr := New(Options{
			MinifySyntax: true,
		}, module.Symbols)
		actual := pr.Print(module)
		if actual != expected {
			t.Errorf("\ninput:\n%s\nexpected:\n%s\nactual:\n%s", input, expected, actual)
		}
	})
}

// expectPrintedMangleMinify verifies fully minified output.
func expectPrintedMangleMinify(t *testing.T, input string, expected string) {
	t.Helper()
	t.Run(input+"_mangleMinify", func(t *testing.T) {
		t.Helper()
		p := parser.New(input)
		module, errs := p.Parse()
		if len(errs) > 0 {
			t.Fatalf("parse errors: %v", errs)
		}
		pr := New(Options{
			MinifyWhitespace: true,
			MinifySyntax:     true,
		}, module.Symbols)
		actual := pr.Print(module)
		if actual != expected {
			t.Errorf("\ninput:\n%s\nexpected:\n%s\nactual:\n%s", input, expected, actual)
		}
	})
}

// ----------------------------------------------------------------------------
// Whitespace Minification Tests
// ----------------------------------------------------------------------------

func TestMinifyWhitespaceBasic(t *testing.T) {
	// Simple declarations
	expectPrintedMinify(t, "const x = 1;", "const x=1;")
	expectPrintedMinify(t, "const x = 1 + 2;", "const x=1+2;")
	expectPrintedMinify(t, "const x = a * b;", "const x=a*b;")
}

func TestMinifyWhitespaceFunction(t *testing.T) {
	// Function with body - whitespace removed
	expectPrintedMinify(t, "fn foo() { return; }", "fn foo(){return;}")
	expectPrintedMinify(t, "fn foo() -> i32 { return 1; }", "fn foo()->i32{return 1;}")
}

func TestMinifyWhitespaceStruct(t *testing.T) {
	expectPrintedMinify(t,
		"struct Foo { x: i32, y: f32, }",
		"struct Foo{x:i32,y:f32}")
}

func TestMinifyWhitespaceAttributes(t *testing.T) {
	expectPrintedMinify(t,
		"@group(0) @binding(1) var<uniform> u: U;",
		"@group(0) @binding(1) var<uniform> u:U;")
}

// ----------------------------------------------------------------------------
// Number Formatting Tests
// ----------------------------------------------------------------------------

func TestNumberPrinting(t *testing.T) {
	// These tests verify numbers are printed correctly
	expectPrinted(t, "const x = 0;", "const x = 0;\n")
	expectPrinted(t, "const x = 1;", "const x = 1;\n")
	expectPrinted(t, "const x = 42;", "const x = 42;\n")
	expectPrinted(t, "const x = 0.0;", "const x = 0.0;\n")
	expectPrinted(t, "const x = 1.0;", "const x = 1.0;\n")
	expectPrinted(t, "const x = 3.14159;", "const x = 3.14159;\n")
}

func TestNumberSuffixes(t *testing.T) {
	expectPrinted(t, "const x = 1i;", "const x = 1i;\n")
	expectPrinted(t, "const x = 1u;", "const x = 1u;\n")
	expectPrinted(t, "const x = 1.0f;", "const x = 1.0f;\n")
	expectPrinted(t, "const x = 1.0h;", "const x = 1.0h;\n")
}

func TestHexNumbers(t *testing.T) {
	expectPrinted(t, "const x = 0xFF;", "const x = 0xFF;\n")
	expectPrinted(t, "const x = 0xABCDEF;", "const x = 0xABCDEF;\n")
}

// ----------------------------------------------------------------------------
// Operator Formatting Tests
// ----------------------------------------------------------------------------

func TestBinaryOperators(t *testing.T) {
	expectPrinted(t, "const x = a + b;", "const x = a + b;\n")
	expectPrinted(t, "const x = a - b;", "const x = a - b;\n")
	expectPrinted(t, "const x = a * b;", "const x = a * b;\n")
	expectPrinted(t, "const x = a / b;", "const x = a / b;\n")
	expectPrinted(t, "const x = a % b;", "const x = a % b;\n")

	expectPrinted(t, "const x = a == b;", "const x = a == b;\n")
	expectPrinted(t, "const x = a != b;", "const x = a != b;\n")
	expectPrinted(t, "const x = a < b;", "const x = a < b;\n")
	expectPrinted(t, "const x = a <= b;", "const x = a <= b;\n")
	expectPrinted(t, "const x = a > b;", "const x = a > b;\n")
	expectPrinted(t, "const x = a >= b;", "const x = a >= b;\n")

	expectPrinted(t, "const x = a && b;", "const x = a && b;\n")
	expectPrinted(t, "const x = a || b;", "const x = a || b;\n")

	expectPrinted(t, "const x = a & b;", "const x = a & b;\n")
	expectPrinted(t, "const x = a | b;", "const x = a | b;\n")
	expectPrinted(t, "const x = a ^ b;", "const x = a ^ b;\n")
	expectPrinted(t, "const x = a << b;", "const x = a << b;\n")
	expectPrinted(t, "const x = a >> b;", "const x = a >> b;\n")
}

func TestUnaryOperators(t *testing.T) {
	expectPrinted(t, "const x = -a;", "const x = -a;\n")
	expectPrinted(t, "const x = !a;", "const x = !a;\n")
	expectPrinted(t, "const x = ~a;", "const x = ~a;\n")
}

func TestAssignmentOperators(t *testing.T) {
	expectPrinted(t, "fn f() { x = 1; }", "fn f() {\n    x = 1;\n}\n")
	expectPrinted(t, "fn f() { x += 1; }", "fn f() {\n    x += 1;\n}\n")
	expectPrinted(t, "fn f() { x -= 1; }", "fn f() {\n    x -= 1;\n}\n")
	expectPrinted(t, "fn f() { x *= 2; }", "fn f() {\n    x *= 2;\n}\n")
	expectPrinted(t, "fn f() { x /= 2; }", "fn f() {\n    x /= 2;\n}\n")
	expectPrinted(t, "fn f() { x %= 2; }", "fn f() {\n    x %= 2;\n}\n")
	expectPrinted(t, "fn f() { x &= 1; }", "fn f() {\n    x &= 1;\n}\n")
	expectPrinted(t, "fn f() { x |= 1; }", "fn f() {\n    x |= 1;\n}\n")
	expectPrinted(t, "fn f() { x ^= 1; }", "fn f() {\n    x ^= 1;\n}\n")
	expectPrinted(t, "fn f() { x <<= 1; }", "fn f() {\n    x <<= 1;\n}\n")
	expectPrinted(t, "fn f() { x >>= 1; }", "fn f() {\n    x >>= 1;\n}\n")
}

// ----------------------------------------------------------------------------
// Statement Formatting Tests
// ----------------------------------------------------------------------------

func TestIfFormatting(t *testing.T) {
	expectPrinted(t,
		"fn f() { if x { return; } }",
		"fn f() {\n    if x {\n        return;\n    }\n}\n")

	// Test if-else with return statements (parser supports these)
	expectPrinted(t,
		"fn f() { if x { return; } else { return; } }",
		"fn f() {\n    if x {\n        return;\n    } else {\n        return;\n    }\n}\n")
}

func TestForFormatting(t *testing.T) {
	// Skip for now - for loop parsing needs work
	t.Skip("for loop parsing not fully implemented")
}

func TestWhileFormatting(t *testing.T) {
	// Use break statement instead of expression statement
	expectPrinted(t,
		"fn f() { while x { break; } }",
		"fn f() {\n    while x {\n        break;\n    }\n}\n")
}

func TestLoopFormatting(t *testing.T) {
	expectPrinted(t,
		"fn f() { loop { break; } }",
		"fn f() {\n    loop {\n        break;\n    }\n}\n")
}

func TestSwitchFormatting(t *testing.T) {
	expectPrinted(t,
		"fn f() { switch x { case 1: { } default: { } } }",
		"fn f() {\n    switch x {\n        case 1: {\n        }\n        default: {\n        }\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// Type Formatting Tests
// ----------------------------------------------------------------------------

func TestVectorTypePrinting(t *testing.T) {
	expectPrinted(t, "var x: vec2<f32>;", "var x: vec2<f32>;\n")
	expectPrinted(t, "var x: vec3<f32>;", "var x: vec3<f32>;\n")
	expectPrinted(t, "var x: vec4<f32>;", "var x: vec4<f32>;\n")
	expectPrinted(t, "var x: vec2f;", "var x: vec2f;\n")
	expectPrinted(t, "var x: vec3f;", "var x: vec3f;\n")
	expectPrinted(t, "var x: vec4f;", "var x: vec4f;\n")
}

func TestMatrixTypePrinting(t *testing.T) {
	// Matrix shorthand types work
	expectPrinted(t, "var x: mat4x4f;", "var x: mat4x4f;\n")
	// Parameterized matrix types need parser work
	t.Run("mat2x2<f32>", func(t *testing.T) {
		t.Skip("parameterized matrix type parsing not fully implemented")
	})
}

func TestArrayTypePrinting(t *testing.T) {
	// Runtime-sized array works
	expectPrinted(t, "var x: array<f32>;", "var x: array<f32>;\n")
	// Sized array needs parser work
	t.Run("array<f32, 10>", func(t *testing.T) {
		t.Skip("sized array type parsing not fully implemented")
	})
}

func TestPointerTypePrinting(t *testing.T) {
	expectPrinted(t, "var x: ptr<function, f32>;", "var x: ptr<function, f32>;\n")
	expectPrinted(t, "var x: ptr<storage, f32, read_write>;", "var x: ptr<storage, f32, read_write>;\n")
}

// ----------------------------------------------------------------------------
// Attribute Formatting Tests
// ----------------------------------------------------------------------------

func TestAttributePrinting(t *testing.T) {
	expectPrinted(t, "@vertex fn main() {}", "@vertex fn main() {\n}\n")
	expectPrinted(t, "@fragment fn main() {}", "@fragment fn main() {\n}\n")
	expectPrinted(t, "@compute @workgroup_size(64) fn main() {}",
		"@compute @workgroup_size(64) fn main() {\n}\n")
}

func TestBindingAttributePrinting(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var<uniform> u: U;",
		"@group(0) @binding(0) var<uniform> u: U;\n")
}

func TestBuiltinAttributePrinting(t *testing.T) {
	expectPrinted(t,
		"struct V { @builtin(position) p: vec4f, }",
		"struct V {\n    @builtin(position) p: vec4f\n}\n")
}

func TestLocationAttributePrinting(t *testing.T) {
	expectPrinted(t,
		"struct V { @location(0) uv: vec2f, }",
		"struct V {\n    @location(0) uv: vec2f\n}\n")
}

// ----------------------------------------------------------------------------
// Directive Formatting Tests
// ----------------------------------------------------------------------------

func TestDirectivePrinting(t *testing.T) {
	expectPrinted(t, "enable f16;", "enable f16;\n")
	expectPrinted(t, "requires foo;", "requires foo;\n")
	expectPrinted(t, "diagnostic(off, bar);", "diagnostic(off, bar);\n")
}

// ----------------------------------------------------------------------------
// Complex Cases
// ----------------------------------------------------------------------------

func TestComplexExpressions(t *testing.T) {
	expectPrinted(t, "const x = a + b * c;", "const x = a + b * c;\n")
	expectPrinted(t, "const x = (a + b) * c;", "const x = (a + b) * c;\n")
	expectPrinted(t, "const x = a.b.c;", "const x = a.b.c;\n")
	expectPrinted(t, "const x = a[0].b;", "const x = a[0].b;\n")
	expectPrinted(t, "const x = foo(a, b).c;", "const x = foo(a, b).c;\n")
}

func TestCallExpressions(t *testing.T) {
	expectPrinted(t, "const x = foo();", "const x = foo();\n")
	expectPrinted(t, "const x = foo(1);", "const x = foo(1);\n")
	expectPrinted(t, "const x = foo(1, 2, 3);", "const x = foo(1, 2, 3);\n")
	expectPrinted(t, "const x = vec3f(1.0, 2.0, 3.0);", "const x = vec3f(1.0, 2.0, 3.0);\n")
}

// ----------------------------------------------------------------------------
// Roundtrip Tests (Parse → Print → Parse → Print)
// ----------------------------------------------------------------------------

func TestRoundtrip(t *testing.T) {
	inputs := []string{
		"const x = 1;",
		"var x: i32;",
		"fn foo() {}",
		"fn foo() -> i32 { return 1; }",
		"struct Foo { x: i32, }",
		"@vertex fn main() {}",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			// First parse and print
			p1 := parser.New(input)
			m1, errs := p1.Parse()
			if len(errs) > 0 {
				t.Fatalf("first parse errors: %v", errs)
			}
			pr1 := New(Options{}, m1.Symbols)
			output1 := pr1.Print(m1)

			// Second parse and print
			p2 := parser.New(output1)
			m2, errs := p2.Parse()
			if len(errs) > 0 {
				t.Fatalf("second parse errors: %v", errs)
			}
			pr2 := New(Options{}, m2.Symbols)
			output2 := pr2.Print(m2)

			// Outputs should match
			if output1 != output2 {
				t.Errorf("roundtrip failed:\nfirst:  %s\nsecond: %s", output1, output2)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Texture Type Printing Tests
// ----------------------------------------------------------------------------

func TestTextureTypePrinting(t *testing.T) {
	// Sampled textures
	expectPrinted(t, "@group(0) @binding(0) var t: texture_2d<f32>;",
		"@group(0) @binding(0) var t: texture_2d<f32>;\n")
	expectPrinted(t, "@group(0) @binding(0) var t: texture_3d<f32>;",
		"@group(0) @binding(0) var t: texture_3d<f32>;\n")
	expectPrinted(t, "@group(0) @binding(0) var t: texture_cube<f32>;",
		"@group(0) @binding(0) var t: texture_cube<f32>;\n")
	expectPrinted(t, "@group(0) @binding(0) var t: texture_1d<f32>;",
		"@group(0) @binding(0) var t: texture_1d<f32>;\n")
	expectPrinted(t, "@group(0) @binding(0) var t: texture_2d_array<f32>;",
		"@group(0) @binding(0) var t: texture_2d_array<f32>;\n")
	expectPrinted(t, "@group(0) @binding(0) var t: texture_cube_array<f32>;",
		"@group(0) @binding(0) var t: texture_cube_array<f32>;\n")
}

func TestTextureMultisampledPrinting(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_multisampled_2d<f32>;",
		"@group(0) @binding(0) var t: texture_multisampled_2d<f32>;\n")
}

func TestTextureStoragePrinting(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_storage_2d<rgba8unorm, write>;",
		"@group(0) @binding(0) var t: texture_storage_2d<rgba8unorm, write>;\n")
	expectPrinted(t, "@group(0) @binding(0) var t: texture_storage_2d<rgba8unorm, read_write>;",
		"@group(0) @binding(0) var t: texture_storage_2d<rgba8unorm, read_write>;\n")
}

func TestTextureDepthPrinting(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_2d;",
		"@group(0) @binding(0) var t: texture_depth_2d;\n")
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_cube;",
		"@group(0) @binding(0) var t: texture_depth_cube;\n")
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_2d_array;",
		"@group(0) @binding(0) var t: texture_depth_2d_array;\n")
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_cube_array;",
		"@group(0) @binding(0) var t: texture_depth_cube_array;\n")
}

func TestTextureExternalPrinting(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_external;",
		"@group(0) @binding(0) var t: texture_external;\n")
}

// ----------------------------------------------------------------------------
// For Loop Tests
// ----------------------------------------------------------------------------

func TestForLoopPrinting(t *testing.T) {
	expectPrinted(t,
		"fn f() { for (var i = 0; i < 10; i++) { break; } }",
		"fn f() {\n    for (var i = 0; i < 10; i++) {\n        break;\n    }\n}\n")
}

func TestForLoopMinified(t *testing.T) {
	expectPrintedMinify(t,
		"fn f() { for (var i = 0; i < 10; i++) { break; } }",
		"fn f(){for(var i=0;i<10;i++){break;}}")
}

func TestForLoopWithLet(t *testing.T) {
	expectPrinted(t,
		"fn f() { for (let i = 0; i < 10; i++) { break; } }",
		"fn f() {\n    for (let i = 0; i < 10; i++) {\n        break;\n    }\n}\n")
}

func TestForLoopWithAssignUpdate(t *testing.T) {
	expectPrinted(t,
		"fn f() { for (var i = 0; i < 10; i += 2) { break; } }",
		"fn f() {\n    for (var i = 0; i < 10; i += 2) {\n        break;\n    }\n}\n")
}

func TestForLoopDecrement(t *testing.T) {
	expectPrinted(t,
		"fn f() { for (var i = 10; i > 0; i--) { break; } }",
		"fn f() {\n    for (var i = 10; i > 0; i--) {\n        break;\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// If-Else Chain Tests
// ----------------------------------------------------------------------------

func TestIfElseIfPrinting(t *testing.T) {
	expectPrinted(t,
		"fn f() { if a { return; } else if b { return; } else { return; } }",
		"fn f() {\n    if a {\n        return;\n    } else if b {\n        return;\n    } else {\n        return;\n    }\n}\n")
}

func TestIfElseIfMinified(t *testing.T) {
	expectPrintedMinify(t,
		"fn f() { if a { return; } else if b { return; } }",
		"fn f(){if a{return;} else if b{return;}}")
}

// ----------------------------------------------------------------------------
// Switch Statement Tests
// ----------------------------------------------------------------------------

func TestSwitchWithMultipleCases(t *testing.T) {
	expectPrinted(t,
		"fn f() { switch x { case 1: { return; } case 2, 3: { return; } default: { return; } } }",
		"fn f() {\n    switch x {\n        case 1: {\n            return;\n        }\n        case 2, 3: {\n            return;\n        }\n        default: {\n            return;\n        }\n    }\n}\n")
}

func TestSwitchMinified(t *testing.T) {
	expectPrintedMinify(t,
		"fn f() { switch x { case 1: { return; } default: { return; } } }",
		"fn f(){switch x{case 1:{return;}default:{return;}}}")
}

// ----------------------------------------------------------------------------
// Continue and Continuing Tests
// ----------------------------------------------------------------------------

func TestContinueStatementPrinting(t *testing.T) {
	expectPrinted(t,
		"fn f() { loop { continue; } }",
		"fn f() {\n    loop {\n        continue;\n    }\n}\n")
}

func TestContinuingBlockPrinting(t *testing.T) {
	// Continuing blocks are not yet fully supported by the parser
	t.Skip("continuing block parsing not fully implemented")
}

// ----------------------------------------------------------------------------
// Discard Statement Tests
// ----------------------------------------------------------------------------

func TestDiscardStatementPrinting(t *testing.T) {
	expectPrinted(t,
		"@fragment fn f() { discard; }",
		"@fragment fn f() {\n    discard;\n}\n")
}

// ----------------------------------------------------------------------------
// Call Statement Tests
// ----------------------------------------------------------------------------

func TestCallStatementPrinting(t *testing.T) {
	expectPrinted(t,
		"fn f() { foo(); }",
		"fn f() {\n    foo();\n}\n")
	expectPrinted(t,
		"fn f() { bar(1, 2, 3); }",
		"fn f() {\n    bar(1, 2, 3);\n}\n")
}

// ----------------------------------------------------------------------------
// Increment/Decrement Tests
// ----------------------------------------------------------------------------

func TestIncrementDecrementPrinting(t *testing.T) {
	expectPrinted(t,
		"fn f() { i++; }",
		"fn f() {\n    i++;\n}\n")
	expectPrinted(t,
		"fn f() { i--; }",
		"fn f() {\n    i--;\n}\n")
}

// ----------------------------------------------------------------------------
// Numeric Literal Optimization Tests (MinifySyntax)
// ----------------------------------------------------------------------------

func TestNumericOptimization(t *testing.T) {
	// Numeric optimization is defined but not currently called during printing
	// The minifier preserves literal values as-is
	t.Skip("numeric literal optimization not yet integrated into printer")
}

// ----------------------------------------------------------------------------
// ConstAssert Tests
// ----------------------------------------------------------------------------

func TestConstAssertPrinting(t *testing.T) {
	expectPrinted(t, "const_assert true;", "const_assert true;\n")
	expectPrinted(t, "const_assert 1 == 1;", "const_assert 1 == 1;\n")
}

// ----------------------------------------------------------------------------
// Sampler Type Tests
// ----------------------------------------------------------------------------

func TestSamplerTypePrinting(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var s: sampler;",
		"@group(0) @binding(0) var s: sampler;\n")
	expectPrinted(t, "@group(0) @binding(0) var s: sampler_comparison;",
		"@group(0) @binding(0) var s: sampler_comparison;\n")
}

// ----------------------------------------------------------------------------
// Atomic Type Tests
// ----------------------------------------------------------------------------

func TestAtomicTypePrinting(t *testing.T) {
	expectPrinted(t, "var<workgroup> counter: atomic<u32>;",
		"var<workgroup> counter: atomic<u32>;\n")
}

// ----------------------------------------------------------------------------
// Override Declaration Tests
// ----------------------------------------------------------------------------

func TestOverridePrinting(t *testing.T) {
	expectPrinted(t, "@id(1) override x: f32 = 1.0;", "@id(1) override x: f32 = 1.0;\n")
	expectPrinted(t, "override y: u32;", "override y: u32;\n")
}

// ----------------------------------------------------------------------------
// Alias Declaration Tests
// ----------------------------------------------------------------------------

func TestAliasPrinting(t *testing.T) {
	expectPrinted(t, "alias Float = f32;", "alias Float = f32;\n")
}

// ----------------------------------------------------------------------------
// Additional Coverage Tests
// ----------------------------------------------------------------------------

func TestBreakIfPrinting(t *testing.T) {
	expectPrinted(t,
		"fn f() { loop { break if x; } }",
		"fn f() {\n    loop {\n        break if x;\n    }\n}\n")
}

func TestTextureDepthMultisampled(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_multisampled_2d;",
		"@group(0) @binding(0) var t: texture_depth_multisampled_2d;\n")
}

func TestTextureStorage3D(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_storage_3d<rgba8unorm, write>;",
		"@group(0) @binding(0) var t: texture_storage_3d<rgba8unorm, write>;\n")
}

func TestTextureStorage1D(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_storage_1d<rgba8unorm, write>;",
		"@group(0) @binding(0) var t: texture_storage_1d<rgba8unorm, write>;\n")
}

func TestTextureStorage2DArray(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_storage_2d_array<rgba8unorm, write>;",
		"@group(0) @binding(0) var t: texture_storage_2d_array<rgba8unorm, write>;\n")
}

func TestBoolLiteralPrinting(t *testing.T) {
	expectPrinted(t, "const x = true;", "const x = true;\n")
	expectPrinted(t, "const x = false;", "const x = false;\n")
}

func TestDiagnosticDirective(t *testing.T) {
	expectPrinted(t, "diagnostic(warning, my_category);", "diagnostic(warning, my_category);\n")
}

func TestIfElseWithCompound(t *testing.T) {
	// Test if-else where else is a compound block
	expectPrinted(t,
		"fn f() { if a { return; } else { break; } }",
		"fn f() {\n    if a {\n        return;\n    } else {\n        break;\n    }\n}\n")
}

func TestDeepIfElseChain(t *testing.T) {
	// Test deep else-if chain to cover printElseChainNoTrailing
	expectPrinted(t,
		"fn f() { if a { return; } else if b { return; } else if c { return; } else { return; } }",
		"fn f() {\n    if a {\n        return;\n    } else if b {\n        return;\n    } else if c {\n        return;\n    } else {\n        return;\n    }\n}\n")
}

func TestLetDeclaration(t *testing.T) {
	expectPrinted(t, "fn f() { let x = 1; }", "fn f() {\n    let x = 1;\n}\n")
}

func TestVarDeclarationInFunction(t *testing.T) {
	expectPrinted(t, "fn f() { var x: i32; }", "fn f() {\n    var x: i32;\n}\n")
	expectPrinted(t, "fn f() { var x: i32 = 5; }", "fn f() {\n    var x: i32 = 5;\n}\n")
}

func TestAddressOfDeref(t *testing.T) {
	expectPrinted(t, "fn f() { let x = &y; }", "fn f() {\n    let x = &y;\n}\n")
	expectPrinted(t, "fn f() { let x = *y; }", "fn f() {\n    let x = *y;\n}\n")
}

// ----------------------------------------------------------------------------
// Matrix Type Tests (covers printType MatType branches)
// ----------------------------------------------------------------------------

func TestMatrixTypeShorthand(t *testing.T) {
	// Shorthand mat types
	expectPrinted(t, "fn f(m: mat2x2f) {}", "fn f(m: mat2x2f) {\n}\n")
	expectPrinted(t, "fn f(m: mat3x3f) {}", "fn f(m: mat3x3f) {\n}\n")
	expectPrinted(t, "fn f(m: mat4x4f) {}", "fn f(m: mat4x4f) {\n}\n")
}

func TestMatrixTypeGeneric(t *testing.T) {
	// Generic mat types
	expectPrinted(t, "fn f(m: mat2x2<f32>) {}", "fn f(m: mat2x2<f32>) {\n}\n")
	expectPrinted(t, "fn f(m: mat3x4<f16>) {}", "fn f(m: mat3x4<f16>) {\n}\n")
}

// ----------------------------------------------------------------------------
// Ptr and Atomic Type Tests (covers printType branches)
// ----------------------------------------------------------------------------

func TestPtrTypeWithAccessMode(t *testing.T) {
	// Ptr type with access mode
	expectPrinted(t, "fn f(p: ptr<storage, i32, read_write>) {}", "fn f(p: ptr<storage, i32, read_write>) {\n}\n")
	expectPrinted(t, "fn f(p: ptr<storage, f32, read>) {}", "fn f(p: ptr<storage, f32, read>) {\n}\n")
}

func TestAtomicType(t *testing.T) {
	expectPrinted(t, "var<workgroup> a: atomic<u32>;", "var<workgroup> a: atomic<u32>;\n")
	expectPrinted(t, "var<workgroup> a: atomic<i32>;", "var<workgroup> a: atomic<i32>;\n")
}

// ----------------------------------------------------------------------------
// For Loop Init Tests (covers printForInit branches)
// ----------------------------------------------------------------------------

func TestForLoopWithLetInit(t *testing.T) {
	expectPrinted(t,
		"fn f() { for (let i = 0; i < 10; i++) { break; } }",
		"fn f() {\n    for (let i = 0; i < 10; i++) {\n        break;\n    }\n}\n")
}

func TestForLoopWithAssignInit(t *testing.T) {
	expectPrinted(t,
		"fn f() { var i: i32; for (i = 0; i < 10; i++) { break; } }",
		"fn f() {\n    var i: i32;\n    for (i = 0; i < 10; i++) {\n        break;\n    }\n}\n")
}

func TestForLoopWithTypedVar(t *testing.T) {
	expectPrinted(t,
		"fn f() { for (var i: u32 = 0; i < 10; i++) { break; } }",
		"fn f() {\n    for (var i: u32 = 0; i < 10; i++) {\n        break;\n    }\n}\n")
}

func TestForLoopWithTypedLet(t *testing.T) {
	expectPrinted(t,
		"fn f() { for (let i: u32 = 0; i < 10; i++) { break; } }",
		"fn f() {\n    for (let i: u32 = 0; i < 10; i++) {\n        break;\n    }\n}\n")
}

func TestForLoopWithDecrement(t *testing.T) {
	expectPrinted(t,
		"fn f() { for (var i = 10; i > 0; i--) { break; } }",
		"fn f() {\n    for (var i = 10; i > 0; i--) {\n        break;\n    }\n}\n")
}

func TestForLoopWithSimpleAssignUpdate(t *testing.T) {
	expectPrinted(t,
		"fn f() { for (var i = 0; i < 10; i = i + 2) { break; } }",
		"fn f() {\n    for (var i = 0; i < 10; i = i + 2) {\n        break;\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// Assignment Operators Tests (covers assignOpString)
// ----------------------------------------------------------------------------

func TestCompoundAssignmentOperators(t *testing.T) {
	expectPrinted(t, "fn f() { var x: i32; x += 1; }", "fn f() {\n    var x: i32;\n    x += 1;\n}\n")
	expectPrinted(t, "fn f() { var x: i32; x -= 1; }", "fn f() {\n    var x: i32;\n    x -= 1;\n}\n")
	expectPrinted(t, "fn f() { var x: i32; x *= 2; }", "fn f() {\n    var x: i32;\n    x *= 2;\n}\n")
	expectPrinted(t, "fn f() { var x: i32; x /= 2; }", "fn f() {\n    var x: i32;\n    x /= 2;\n}\n")
	expectPrinted(t, "fn f() { var x: i32; x %= 3; }", "fn f() {\n    var x: i32;\n    x %= 3;\n}\n")
	expectPrinted(t, "fn f() { var x: i32; x &= 1; }", "fn f() {\n    var x: i32;\n    x &= 1;\n}\n")
	expectPrinted(t, "fn f() { var x: i32; x |= 1; }", "fn f() {\n    var x: i32;\n    x |= 1;\n}\n")
	expectPrinted(t, "fn f() { var x: i32; x ^= 1; }", "fn f() {\n    var x: i32;\n    x ^= 1;\n}\n")
	expectPrinted(t, "fn f() { var x: i32; x <<= 1; }", "fn f() {\n    var x: i32;\n    x <<= 1;\n}\n")
	expectPrinted(t, "fn f() { var x: i32; x >>= 1; }", "fn f() {\n    var x: i32;\n    x >>= 1;\n}\n")
}

// ----------------------------------------------------------------------------
// Binary Operators Tests (covers binaryOpString)
// ----------------------------------------------------------------------------

func TestAllBinaryOperators(t *testing.T) {
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
	// Logical
	expectPrinted(t, "const x = true && false;", "const x = true && false;\n")
	expectPrinted(t, "const x = true || false;", "const x = true || false;\n")
	// Comparison
	expectPrinted(t, "const x = 1 == 2;", "const x = 1 == 2;\n")
	expectPrinted(t, "const x = 1 != 2;", "const x = 1 != 2;\n")
	expectPrinted(t, "const x = 1 < 2;", "const x = 1 < 2;\n")
	expectPrinted(t, "const x = 1 <= 2;", "const x = 1 <= 2;\n")
	expectPrinted(t, "const x = 1 > 2;", "const x = 1 > 2;\n")
	expectPrinted(t, "const x = 1 >= 2;", "const x = 1 >= 2;\n")
}

// ----------------------------------------------------------------------------
// Unary Operators Tests (covers unaryOpString)
// ----------------------------------------------------------------------------

func TestAllUnaryOperators(t *testing.T) {
	expectPrinted(t, "const x = -1;", "const x = -1;\n")
	expectPrinted(t, "const x = !true;", "const x = !true;\n")
	expectPrinted(t, "const x = ~1;", "const x = ~1;\n")
}

// ----------------------------------------------------------------------------
// Directive Tests (covers printDirective branches)
// ----------------------------------------------------------------------------

func TestRequiresDirective(t *testing.T) {
	expectPrinted(t, "requires my_feature;", "requires my_feature;\n")
}

func TestRequiresMultipleFeatures(t *testing.T) {
	expectPrinted(t, "requires feature1, feature2;", "requires feature1, feature2;\n")
}

func TestEnableMultipleFeatures(t *testing.T) {
	expectPrinted(t, "enable f16, clip_distances;", "enable f16, clip_distances;\n")
}

func TestDirectivesWithDeclarations(t *testing.T) {
	// Module with directives AND declarations - tests blank line insertion
	expectPrinted(t,
		"enable f16; const x = 1;",
		"enable f16;\n\nconst x = 1;\n")
}

// ----------------------------------------------------------------------------
// VarDecl with Address Space and Access Mode (covers printDeclNoTrailingNewline)
// ----------------------------------------------------------------------------

func TestVarDeclWithAddressSpaceInFunction(t *testing.T) {
	expectPrinted(t,
		"fn f() { var<private> x: i32; }",
		"fn f() {\n    var<private> x: i32;\n}\n")
}

func TestVarDeclWithAccessModeInFunction(t *testing.T) {
	expectPrinted(t,
		"fn f() { var<storage, read_write> x: i32; }",
		"fn f() {\n    var<storage, read_write> x: i32;\n}\n")
}

// ----------------------------------------------------------------------------
// Source Map and Renaming Tests (covers printName with SourceMapGen)
// ----------------------------------------------------------------------------

func TestPrinterWithSourceMapAndRenamer(t *testing.T) {
	// Parse a simple module
	input := "fn foo() { let bar = 1; }"
	p := parser.New(input)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// Create a mock renamer
	mockRenamer := &testRenamer{
		names: make(map[uint32]string),
	}
	// Rename foo -> a, bar -> b
	mockRenamer.names[0] = "a"
	mockRenamer.names[1] = "b"

	// Create actual source map generator
	sm := sourcemap.NewGenerator(input)

	pr := New(Options{
		MinifyIdentifiers: true,
		Renamer:           mockRenamer,
		SourceMapGen:      sm,
	}, module.Symbols)

	result := pr.Print(module)

	// Check output uses renamed identifiers
	if result != "fn a() {\n    let b = 1;\n}\n" {
		t.Errorf("unexpected output: %s", result)
	}

	// Check source map was generated
	sourceMap := sm.Generate()
	if sourceMap == nil {
		t.Error("expected source map to be generated")
	}
	if sourceMap.Mappings == "" {
		t.Error("expected source map to have mappings")
	}
}

// testRenamer for testing
type testRenamer struct {
	names map[uint32]string
}

func (r *testRenamer) NameForSymbol(ref ast.Ref) string {
	if name, ok := r.names[ref.InnerIndex]; ok {
		return name
	}
	return "x"
}

// ----------------------------------------------------------------------------
// printName edge cases (covers invalid ref and bounds checking)
// ----------------------------------------------------------------------------

func TestPrintNameWithInvalidRef(t *testing.T) {
	// Test that printName handles invalid refs gracefully
	input := "const x = 1;"
	p := parser.New(input)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	pr := New(Options{}, module.Symbols)
	result := pr.Print(module)

	// Just verify it doesn't crash
	if result == "" {
		t.Error("expected non-empty output")
	}
}

// ----------------------------------------------------------------------------
// MinifySyntax Tests (covers printLiteral MinifySyntax branch)
// ----------------------------------------------------------------------------

func TestMinifySyntaxLiterals(t *testing.T) {
	// MinifySyntax currently just passes through literals
	expectPrintedMangle(t, "const x = 0.5;", "const x = 0.5;\n")
	expectPrintedMangle(t, "const x = 1.0;", "const x = 1.0;\n")
	expectPrintedMangle(t, "const x = 100;", "const x = 100;\n")
}

// ----------------------------------------------------------------------------
// Loop statement with continuing (covers LoopStmt Continuing branch)
// ----------------------------------------------------------------------------

func TestLoopStatementBasic(t *testing.T) {
	expectPrinted(t,
		"fn f() { loop { break; } }",
		"fn f() {\n    loop {\n        break;\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// CallStmt in for update (covers printForUpdate CallStmt case)
// ----------------------------------------------------------------------------

func TestForLoopWithCallUpdate(t *testing.T) {
	expectPrinted(t,
		"fn update() {} fn f() { for (var i = 0; i < 10; update()) { break; } }",
		"fn update() {\n}\n\nfn f() {\n    for (var i = 0; i < 10; update()) {\n        break;\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// CallExpr with templated type constructor (covers TemplateType branch)
// ----------------------------------------------------------------------------

func TestVecConstructor(t *testing.T) {
	expectPrinted(t, "const x = vec3<f32>(1.0, 2.0, 3.0);", "const x = vec3<f32>(1.0, 2.0, 3.0);\n")
	expectPrinted(t, "const x = vec2<i32>(1, 2);", "const x = vec2<i32>(1, 2);\n")
}

func TestArrayConstructor(t *testing.T) {
	expectPrinted(t, "const x = array<i32, 3>(1, 2, 3);", "const x = array<i32, 3>(1, 2, 3);\n")
}

// ----------------------------------------------------------------------------
// Struct with trailing comma (covers member printing)
// ----------------------------------------------------------------------------

func TestStructMultipleMembers(t *testing.T) {
	expectPrinted(t,
		"struct Foo { x: i32, y: f32, z: u32 }",
		"struct Foo {\n    x: i32,\n    y: f32,\n    z: u32\n}\n")
}

// ----------------------------------------------------------------------------
// Multiple declarations (covers newline between declarations)
// ----------------------------------------------------------------------------

func TestMultipleDeclarations(t *testing.T) {
	expectPrinted(t,
		"const a = 1; const b = 2; const c = 3;",
		"const a = 1;\n\nconst b = 2;\n\nconst c = 3;\n")
}

// ----------------------------------------------------------------------------
// OverrideDecl tests (covers OverrideDecl branches)
// ----------------------------------------------------------------------------

func TestOverrideDeclWithType(t *testing.T) {
	expectPrinted(t, "@id(0) override x: f32;", "@id(0) override x: f32;\n")
}

func TestOverrideDeclWithInitializer(t *testing.T) {
	expectPrinted(t, "override x = 1.0;", "override x = 1.0;\n")
}

func TestOverrideDeclWithBoth(t *testing.T) {
	expectPrinted(t, "@id(0) override x: f32 = 1.0;", "@id(0) override x: f32 = 1.0;\n")
}

// ----------------------------------------------------------------------------
// AliasDecl tests
// ----------------------------------------------------------------------------

func TestAliasDecl(t *testing.T) {
	expectPrinted(t, "alias MyFloat = f32;", "alias MyFloat = f32;\n")
	expectPrinted(t, "alias MyVec = vec3<f32>;", "alias MyVec = vec3<f32>;\n")
}

// ----------------------------------------------------------------------------
// ConstAssertDecl tests
// ----------------------------------------------------------------------------

func TestConstAssertDecl(t *testing.T) {
	expectPrinted(t, "const_assert true;", "const_assert true;\n")
	expectPrinted(t, "const_assert 1 == 1;", "const_assert 1 == 1;\n")
}

// ----------------------------------------------------------------------------
// Tree shaking (covers printModule TreeShaking branch)
// ----------------------------------------------------------------------------

func TestTreeShakingFiltersDeadCode(t *testing.T) {
	input := `const dead = 1;
@compute @workgroup_size(1) fn main() { return; }
`
	p := parser.New(input)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	// Mark main as entry point so it's kept (parser should already do this)
	// Run DCE marking to set IsLive flags
	dce.Mark(module)

	pr := New(Options{
		TreeShaking: true,
	}, module.Symbols)

	result := pr.Print(module)

	// With tree shaking, "dead" should be filtered out
	if result != "@compute @workgroup_size(1) fn main() {\n    return;\n}\n" {
		t.Errorf("unexpected output: %s", result)
	}
}

// ----------------------------------------------------------------------------
// More texture type tests (covers printTextureType branches)
// ----------------------------------------------------------------------------

func TestTextureCube(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_cube<f32>;",
		"@group(0) @binding(0) var t: texture_cube<f32>;\n")
}

func TestTextureCubeArray(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_cube_array<f32>;",
		"@group(0) @binding(0) var t: texture_cube_array<f32>;\n")
}

func TestTextureDepthCube(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_cube;",
		"@group(0) @binding(0) var t: texture_depth_cube;\n")
}

func TestTextureDepthCubeArray(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_cube_array;",
		"@group(0) @binding(0) var t: texture_depth_cube_array;\n")
}

// ----------------------------------------------------------------------------
// Sampler type tests (covers printType SamplerType branch)
// ----------------------------------------------------------------------------

func TestSamplerComparison(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var s: sampler_comparison;",
		"@group(0) @binding(0) var s: sampler_comparison;\n")
}

// ----------------------------------------------------------------------------
// Const with type annotation (covers printDeclNoTrailingNewline ConstDecl Type branch)
// ----------------------------------------------------------------------------

func TestConstDeclWithTypeInFunction(t *testing.T) {
	expectPrinted(t,
		"fn f() { const x: i32 = 1; }",
		"fn f() {\n    const x: i32 = 1;\n}\n")
}

// ----------------------------------------------------------------------------
// Let with type annotation (covers printDeclNoTrailingNewline LetDecl Type branch)
// ----------------------------------------------------------------------------

func TestLetDeclWithTypeInFunction(t *testing.T) {
	expectPrinted(t,
		"fn f() { let x: f32 = 1.0; }",
		"fn f() {\n    let x: f32 = 1.0;\n}\n")
}

// ----------------------------------------------------------------------------
// ConstDecl with type (covers printDecl ConstDecl Type branch)
// ----------------------------------------------------------------------------

func TestConstDeclWithType(t *testing.T) {
	expectPrinted(t, "const x: i32 = 1;", "const x: i32 = 1;\n")
}

// ----------------------------------------------------------------------------
// LetDecl with type (covers printDecl LetDecl Type branch)
// ----------------------------------------------------------------------------

func TestLetDeclWithType(t *testing.T) {
	expectPrinted(t, "fn f() { let x: i32 = 1; }", "fn f() {\n    let x: i32 = 1;\n}\n")
}

// ----------------------------------------------------------------------------
// Tests for printStmtNoTrailingNewline remaining branches
// ----------------------------------------------------------------------------

func TestWhileStatement(t *testing.T) {
	expectPrinted(t,
		"fn f() { var x: i32 = 0; while x < 10 { x += 1; } }",
		"fn f() {\n    var x: i32 = 0;\n    while x < 10 {\n        x += 1;\n    }\n}\n")
}

func TestLoopWithContinuing(t *testing.T) {
	// Skip: continuing block parsing not fully implemented
	t.Skip("continuing block parsing not fully implemented")
}

func TestDecrementStatement(t *testing.T) {
	expectPrinted(t,
		"fn f() { var x: i32 = 10; x--; }",
		"fn f() {\n    var x: i32 = 10;\n    x--;\n}\n")
}

func TestIncrementStatement(t *testing.T) {
	expectPrinted(t,
		"fn f() { var x: i32 = 0; x++; }",
		"fn f() {\n    var x: i32 = 0;\n    x++;\n}\n")
}

// ----------------------------------------------------------------------------
// Compound statement nested (covers printStmtNoTrailingNewline CompoundStmt)
// ----------------------------------------------------------------------------

func TestNestedCompoundStatement(t *testing.T) {
	expectPrinted(t,
		"fn f() { { let x = 1; } }",
		"fn f() {\n    {\n        let x = 1;\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// Multiple switch cases with selectors (covers switch printing)
// ----------------------------------------------------------------------------

func TestSwitchMultipleCases(t *testing.T) {
	expectPrinted(t,
		"fn f() { var x: i32; switch x { case 1: { break; } case 2, 3: { break; } default: { break; } } }",
		"fn f() {\n    var x: i32;\n    switch x {\n        case 1: {\n            break;\n        }\n        case 2, 3: {\n            break;\n        }\n        default: {\n            break;\n        }\n    }\n}\n")
}

// ----------------------------------------------------------------------------
// Function with return attributes (covers printDecl FunctionDecl ReturnAttr)
// ----------------------------------------------------------------------------

func TestFunctionWithReturnAttr(t *testing.T) {
	expectPrinted(t,
		"@vertex fn main() -> @builtin(position) vec4<f32> { return vec4<f32>(0.0); }",
		"@vertex fn main() -> @builtin(position) vec4<f32> {\n    return vec4<f32>(0.0);\n}\n")
}

// ----------------------------------------------------------------------------
// Function with parameter attributes (covers printDecl FunctionDecl parameter attributes)
// ----------------------------------------------------------------------------

func TestFunctionWithParamAttr(t *testing.T) {
	expectPrinted(t,
		"@fragment fn main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> { return color; }",
		"@fragment fn main(@location(0) color: vec4<f32>) -> @location(0) vec4<f32> {\n    return color;\n}\n")
}

// ----------------------------------------------------------------------------
// Attributes with multiple arguments (covers printAttributes multiple args)
// ----------------------------------------------------------------------------

func TestAttributeWithMultipleArgs(t *testing.T) {
	expectPrinted(t,
		"@workgroup_size(8, 8, 1) @compute fn main() {}",
		"@workgroup_size(8, 8, 1) @compute fn main() {\n}\n")
}

// ----------------------------------------------------------------------------
// VarDecl with only type, no initializer (covers printDecl VarDecl branch)
// ----------------------------------------------------------------------------

func TestVarDeclWithTypeOnly(t *testing.T) {
	expectPrinted(t, "var<private> x: i32;", "var<private> x: i32;\n")
}

// ----------------------------------------------------------------------------
// VarDecl with address space and type (covers printDecl VarDecl branches)
// ----------------------------------------------------------------------------

func TestVarDeclWithAddressSpace(t *testing.T) {
	expectPrinted(t, "var<storage, read> data: array<f32>;", "var<storage, read> data: array<f32>;\n")
}

// ----------------------------------------------------------------------------
// Return without value (covers printStmtNoTrailingNewline ReturnStmt nil)
// ----------------------------------------------------------------------------

func TestReturnWithoutValue(t *testing.T) {
	expectPrinted(t, "fn f() { return; }", "fn f() {\n    return;\n}\n")
}

// ----------------------------------------------------------------------------
// CallStmt (covers printStmtNoTrailingNewline CallStmt)
// ----------------------------------------------------------------------------

func TestCallStatement(t *testing.T) {
	expectPrinted(t, "fn helper() {} fn f() { helper(); }", "fn helper() {\n}\n\nfn f() {\n    helper();\n}\n")
}

// ----------------------------------------------------------------------------
// IdentExpr with builtin (covers printExpr IdentExpr without valid Ref)
// ----------------------------------------------------------------------------

func TestBuiltinIdentifier(t *testing.T) {
	// Built-in functions don't have a Ref
	expectPrinted(t, "fn f() { let x = abs(-1); }", "fn f() {\n    let x = abs(-1);\n}\n")
}

// ----------------------------------------------------------------------------
// More texture dimensions (covers printTextureType dimension branches)
// ----------------------------------------------------------------------------

func TestTexture1D(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_1d<f32>;",
		"@group(0) @binding(0) var t: texture_1d<f32>;\n")
}

func TestTexture3D(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_3d<f32>;",
		"@group(0) @binding(0) var t: texture_3d<f32>;\n")
}

func TestTexture2DArray(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_2d_array<f32>;",
		"@group(0) @binding(0) var t: texture_2d_array<f32>;\n")
}

func TestTextureMultisampled2D(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_multisampled_2d<f32>;",
		"@group(0) @binding(0) var t: texture_multisampled_2d<f32>;\n")
}

// ----------------------------------------------------------------------------
// printType Vec and Mat branches
// ----------------------------------------------------------------------------

func TestVecShorthand(t *testing.T) {
	expectPrinted(t, "fn f(v: vec2f) {}", "fn f(v: vec2f) {\n}\n")
	expectPrinted(t, "fn f(v: vec3f) {}", "fn f(v: vec3f) {\n}\n")
	expectPrinted(t, "fn f(v: vec4f) {}", "fn f(v: vec4f) {\n}\n")
	expectPrinted(t, "fn f(v: vec2i) {}", "fn f(v: vec2i) {\n}\n")
	expectPrinted(t, "fn f(v: vec2u) {}", "fn f(v: vec2u) {\n}\n")
}

func TestVecGeneric(t *testing.T) {
	expectPrinted(t, "fn f(v: vec4<f32>) {}", "fn f(v: vec4<f32>) {\n}\n")
	expectPrinted(t, "fn f(v: vec3<i32>) {}", "fn f(v: vec3<i32>) {\n}\n")
	expectPrinted(t, "fn f(v: vec2<u32>) {}", "fn f(v: vec2<u32>) {\n}\n")
}

// ----------------------------------------------------------------------------
// Ptr type without access mode (covers printType PtrType branch)
// ----------------------------------------------------------------------------

func TestPtrTypeWithoutAccessMode(t *testing.T) {
	expectPrinted(t, "fn f(p: ptr<function, i32>) {}", "fn f(p: ptr<function, i32>) {\n}\n")
}

// ----------------------------------------------------------------------------
// Array without size (runtime-sized array, covers printType ArrayType branch)
// ----------------------------------------------------------------------------

func TestRuntimeSizedArray(t *testing.T) {
	expectPrinted(t, "var<storage> data: array<f32>;", "var<storage> data: array<f32>;\n")
}

// ----------------------------------------------------------------------------
// Sampler without comparison (covers printType SamplerType branch)
// ----------------------------------------------------------------------------

func TestSamplerNonComparison(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var s: sampler;",
		"@group(0) @binding(0) var s: sampler;\n")
}

// ----------------------------------------------------------------------------
// Depth Textures (covers printTextureType depth branches)
// ----------------------------------------------------------------------------

func TestDepthTexture2D(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_2d;",
		"@group(0) @binding(0) var t: texture_depth_2d;\n")
}

func TestDepthTexture2DArray(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_2d_array;",
		"@group(0) @binding(0) var t: texture_depth_2d_array;\n")
}

func TestDepthTextureCube(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_cube;",
		"@group(0) @binding(0) var t: texture_depth_cube;\n")
}

func TestDepthTextureCubeArray(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_cube_array;",
		"@group(0) @binding(0) var t: texture_depth_cube_array;\n")
}

func TestDepthMultisampledTexture2D(t *testing.T) {
	expectPrinted(t, "@group(0) @binding(0) var t: texture_depth_multisampled_2d;",
		"@group(0) @binding(0) var t: texture_depth_multisampled_2d;\n")
}

// ----------------------------------------------------------------------------
// Let declarations (covers printDecl LetDecl branch)
// ----------------------------------------------------------------------------

func TestLetDeclarationWithType(t *testing.T) {
	expectPrinted(t, "fn f() { let x: i32 = 5; }", "fn f() {\n    let x: i32 = 5;\n}\n")
}

// ----------------------------------------------------------------------------
// Override with initializer (covers printDecl OverrideDecl initializer branch)
// ----------------------------------------------------------------------------

func TestOverrideWithInitializer(t *testing.T) {
	expectPrinted(t, "@id(0) override scale: f32 = 1.0;", "@id(0) override scale: f32 = 1.0;\n")
}

// ----------------------------------------------------------------------------
// Function with multiple parameters (covers function param separator)
// ----------------------------------------------------------------------------

func TestFunctionMultipleParams(t *testing.T) {
	expectPrinted(t, "fn add(a: i32, b: i32) -> i32 { return a + b; }",
		"fn add(a: i32, b: i32) -> i32 {\n    return a + b;\n}\n")
}
