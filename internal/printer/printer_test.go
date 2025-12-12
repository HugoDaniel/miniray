package printer

import (
	"testing"

	"github.com/HugoDaniel/miniray/internal/parser"
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
