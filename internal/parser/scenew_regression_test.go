package parser

import (
	"testing"
)

// These tests document WGSL features that need to be supported.
// They are based on real-world code from ~/Development/scene/demos/inercia2025/demo/wgsl/sceneW.wgsl

// ----------------------------------------------------------------------------
// Inline Array Initialization
// ----------------------------------------------------------------------------

func TestInlineArrayInitialization(t *testing.T) {
	// WGSL allows inline array initialization with array(...) syntax
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "simple inline array",
			input: `fn test() {
  var pos = array(1, 2, 3);
}`,
		},
		{
			name: "inline array with vec2f",
			input: `fn test() {
  var pos = array(
    vec2f(-1.0, -1.0),
    vec2f(-1.0, 3.0),
    vec2f(3.0, -1.0),
  );
}`,
		},
		{
			name: "inline array indexing",
			input: `fn test(idx: u32) -> vec2f {
  var pos = array(
    vec2f(-1.0, -1.0),
    vec2f(-1.0, 3.0),
  );
  return pos[idx];
}`,
		},
		{
			name: "inline array in expression",
			input: `fn test(index: u32) -> vec2f {
  let position = array<vec2<f32>, 3>(
    vec2f(0.0, 0.0),
    vec2f(1.0, 0.0),
    vec2f(0.0, 1.0)
  )[index];
  return position;
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			_, errs := p.Parse()
			if len(errs) > 0 {
				t.Errorf("parse errors:")
				for _, e := range errs {
					t.Errorf("  %s", e.Message)
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Struct Declaration Variations
// ----------------------------------------------------------------------------

func TestStructDeclarationVariations(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "struct with trailing semicolon",
			input: `struct Foo {
  x: f32,
  y: f32,
};`,
		},
		{
			name: "struct without trailing semicolon",
			input: `struct Foo {
  x: f32,
  y: f32,
}`,
		},
		{
			name: "struct with trailing comma on last member",
			input: `struct Foo {
  x: f32,
  y: f32,
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			_, errs := p.Parse()
			if len(errs) > 0 {
				t.Errorf("parse errors:")
				for _, e := range errs {
					t.Errorf("  %s", e.Message)
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// For Loop Parsing
// ----------------------------------------------------------------------------

func TestForLoopParsing(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "basic for loop with var",
			input: `fn test() {
  for (var i = 0; i < 10; i++) {
    // body
  }
}`,
		},
		{
			name: "for loop with i += 1",
			input: `fn test() {
  for (var i = 0; i < 10; i += 1) {
    // body
  }
}`,
		},
		{
			name: "for loop with typed var",
			input: `fn test() {
  for (var i: u32 = 0u; i < 10u; i++) {
    // body
  }
}`,
		},
		{
			name: "for loop in function returning value",
			input: `fn sum() -> i32 {
  var result = 0;
  for (var i = 0; i < 10; i++) {
    result += i;
  }
  return result;
}`,
		},
		{
			name: "nested for loops",
			input: `fn test() {
  for (var i = 0u; i < 10u; i++) {
    for (var j = 0u; j < 10u; j++) {
      // inner body
    }
  }
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			_, errs := p.Parse()
			if len(errs) > 0 {
				t.Errorf("parse errors:")
				for _, e := range errs {
					t.Errorf("  %s", e.Message)
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Type Casting / Conversion
// ----------------------------------------------------------------------------

func TestTypeCastingParsing(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "u32 cast",
			input: `fn test(x: f32) -> u32 {
  return u32(x);
}`,
		},
		{
			name: "i32 cast",
			input: `fn test(x: f32) -> i32 {
  return i32(x);
}`,
		},
		{
			name: "f32 cast",
			input: `fn test(x: i32) -> f32 {
  return f32(x);
}`,
		},
		{
			name: "cast in switch",
			input: `fn test(phase: f32) {
  switch u32(phase) {
    case 0u: {}
    default: {}
  }
}`,
		},
		{
			name: "cast in expression",
			input: `fn test(n: u32) -> f32 {
  return f32(n) * 2.0;
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			_, errs := p.Parse()
			if len(errs) > 0 {
				t.Errorf("parse errors:")
				for _, e := range errs {
					t.Errorf("  %s", e.Message)
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Array Type with Size Expression
// ----------------------------------------------------------------------------

func TestArrayTypeWithSizeExpression(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "array with const size",
			input: `const N = 10;
var arr: array<f32, N>;`,
		},
		{
			name: "array with literal size",
			input: `var arr: array<f32, 64>;`,
		},
		{
			name: "array with expression size",
			input: `const movements: u32 = 3;
fn test() {
  let position = array<vec2<f32>, movements>(
    vec2f(0.0),
    vec2f(1.0),
    vec2f(2.0)
  );
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			_, errs := p.Parse()
			if len(errs) > 0 {
				t.Errorf("parse errors:")
				for _, e := range errs {
					t.Errorf("  %s", e.Message)
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Const with Type Annotation
// ----------------------------------------------------------------------------

func TestConstWithTypeAnnotation(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "const with u32 type",
			input: `const movements: u32 = 3;`,
		},
		{
			name: "const with f32 type",
			input: `const PI: f32 = 3.14159;`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			_, errs := p.Parse()
			if len(errs) > 0 {
				t.Errorf("parse errors:")
				for _, e := range errs {
					t.Errorf("  %s", e.Message)
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Complex Real-World Patterns from sceneW.wgsl
// ----------------------------------------------------------------------------

func TestSceneWPatterns(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "vertex shader with inline array",
			input: `@vertex
fn vs_test(@builtin(vertex_index) vertexIndex: u32) -> @builtin(position) vec4f {
  var pos = array(
    vec2f(-1.0, -1.0),
    vec2f(-1.0, 3.0),
    vec2f(3.0, -1.0),
  );
  let xy = pos[vertexIndex];
  return vec4f(xy, 0.0, 1.0);
}`,
		},
		{
			name: "struct member accessor",
			input: `struct VertexOutput {
  @builtin(position) position: vec4f,
  @location(0) uv: vec2f,
}

fn get_uv(i: VertexOutput) -> vec2f {
  return i.uv;
}`,
		},
		{
			name: "switch with u32 cast",
			input: `fn test(beat: f32) -> f32 {
  let phase = floor(beat / 4.0) % 4.0;
  var value: f32;
  switch u32(phase) {
    case 0u: {
      value = 1.0;
    }
    case 2u: {
      value = 2.0;
    }
    default: {
      value = 0.0;
    }
  }
  return value;
}`,
		},
		{
			name: "bezier result struct",
			input: `struct BezierResult {
  dist: f32,
  point: vec2f,
}

fn bezier(pos: vec2f, A: vec2f, B: vec2f, C: vec2f) -> BezierResult {
  return BezierResult(1.0, vec2f(0.0));
}`,
		},
		{
			name: "for loop with u32 iteration",
			input: `fn test() {
  for (var i = 0u; i < 7u; i++) {
    // loop body
  }
}`,
		},
		{
			name: "for loop with i32 comparison",
			input: `fn test() {
  let numCables = i32(10);
  for (var i = 1; i < numCables; i++) {
    // loop body
  }
}`,
		},
		{
			name: "texture_external type",
			input: `@group(1) @binding(1) var videoTexture: texture_external;`,
		},
		{
			name: "textureSampleBaseClampToEdge call",
			input: `@group(0) @binding(0) var videoTexture: texture_external;
@group(0) @binding(1) var videoSampler: sampler;

fn sampleVideo(uv: vec2f) -> vec4f {
  return textureSampleBaseClampToEdge(videoTexture, videoSampler, uv);
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			_, errs := p.Parse()
			if len(errs) > 0 {
				t.Errorf("parse errors:")
				for _, e := range errs {
					t.Errorf("  %s", e.Message)
				}
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Trailing Comma in Function Parameters
// ----------------------------------------------------------------------------

func TestTrailingCommaInFunctionParameters(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "single parameter with trailing comma",
			input: `fn test(x: f32,) -> f32 {
  return x;
}`,
		},
		{
			name: "multiple parameters with trailing comma",
			input: `fn test(x: f32, y: f32,) -> f32 {
  return x + y;
}`,
		},
		{
			name: "vertex shader with trailing comma",
			input: `@vertex
fn vs_main(
  @builtin(vertex_index) vertexIndex: u32,
  @location(0) position: vec4f,
) -> @builtin(position) vec4f {
  return position;
}`,
		},
		{
			name: "fragment shader with trailing comma",
			input: `@fragment
fn fs_main(
  @location(0) uv: vec2f,
) -> @location(0) vec4f {
  return vec4f(uv, 0.0, 1.0);
}`,
		},
		{
			name: "compute shader with trailing comma",
			input: `@compute @workgroup_size(64)
fn main(
  @builtin(global_invocation_id) id: vec3u,
) {
  // body
}`,
		},
		{
			name: "helper function with trailing comma",
			input: `fn lerp(
  a: f32,
  b: f32,
  t: f32,
) -> f32 {
  return a + (b - a) * t;
}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.input)
			_, errs := p.Parse()
			if len(errs) > 0 {
				t.Errorf("parse errors:")
				for _, e := range errs {
					t.Errorf("  %s", e.Message)
				}
			}
		})
	}
}
