package lexer

import (
	"testing"
)

// ----------------------------------------------------------------------------
// Test Helpers (esbuild-style)
// ----------------------------------------------------------------------------

func expectToken(t *testing.T, input string, expected TokenKind) {
	t.Helper()
	l := New(input)
	tok := l.Next()
	if tok.Kind != expected {
		t.Errorf("input %q: expected %v, got %v", input, expected, tok.Kind)
	}
}

func expectTokenValue(t *testing.T, input string, expectedKind TokenKind, expectedValue string) {
	t.Helper()
	l := New(input)
	tok := l.Next()
	if tok.Kind != expectedKind {
		t.Errorf("input %q: expected kind %v, got %v", input, expectedKind, tok.Kind)
	}
	if tok.Value != expectedValue {
		t.Errorf("input %q: expected value %q, got %q", input, expectedValue, tok.Value)
	}
}

func expectTokens(t *testing.T, input string, expected []TokenKind) {
	t.Helper()
	l := New(input)
	for i, exp := range expected {
		tok := l.Next()
		if tok.Kind != exp {
			t.Errorf("input %q token %d: expected %v, got %v", input, i, exp, tok.Kind)
		}
	}
}

func expectError(t *testing.T, input string) {
	t.Helper()
	l := New(input)
	tok := l.Next()
	if tok.Kind != TokError {
		t.Errorf("input %q: expected error, got %v", input, tok.Kind)
	}
}

// ----------------------------------------------------------------------------
// Keyword Tests
// ----------------------------------------------------------------------------

func TestKeywords(t *testing.T) {
	cases := []struct {
		input string
		kind  TokenKind
	}{
		// All 26 WGSL keywords
		{"alias", TokAlias},
		{"break", TokBreak},
		{"case", TokCase},
		{"const", TokConst},
		{"const_assert", TokConstAssert},
		{"continue", TokContinue},
		{"continuing", TokContinuing},
		{"default", TokDefault},
		{"diagnostic", TokDiagnostic},
		{"discard", TokDiscard},
		{"else", TokElse},
		{"enable", TokEnable},
		{"false", TokFalse},
		{"fn", TokFn},
		{"for", TokFor},
		{"if", TokIf},
		{"let", TokLet},
		{"loop", TokLoop},
		{"override", TokOverride},
		{"requires", TokRequires},
		{"return", TokReturn},
		{"struct", TokStruct},
		{"switch", TokSwitch},
		{"true", TokTrue},
		{"var", TokVar},
		{"while", TokWhile},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectToken(t, tc.input, tc.kind)
		})
	}
}

func TestBooleanLiterals(t *testing.T) {
	expectToken(t, "true", TokTrue)
	expectToken(t, "false", TokFalse)
}

// ----------------------------------------------------------------------------
// Identifier Tests
// ----------------------------------------------------------------------------

func TestIdentifiers(t *testing.T) {
	cases := []struct {
		input string
		value string
	}{
		{"foo", "foo"},
		{"_bar", "_bar"},
		{"camelCase", "camelCase"},
		{"snake_case", "snake_case"},
		{"UPPER_CASE", "UPPER_CASE"},
		{"a1", "a1"},
		{"vec3f", "vec3f"},
		{"mat4x4f", "mat4x4f"},
		{"i32", "i32"},
		{"Position", "Position"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectTokenValue(t, tc.input, TokIdent, tc.value)
		})
	}
}

func TestUnicodeIdentifiers(t *testing.T) {
	// WGSL supports XID_Start / XID_Continue
	cases := []struct {
		input string
		value string
	}{
		{"α", "α"},           // Greek letter
		{"αβγ", "αβγ"},       // Greek letters
		{"日本語", "日本語"}, // CJK characters
		{"_über", "_über"},   // Mixed ASCII and Unicode
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectTokenValue(t, tc.input, TokIdent, tc.value)
		})
	}
}

func TestInvalidIdentifiers(t *testing.T) {
	// Single underscore is the placeholder, not a valid identifier
	expectToken(t, "_", TokUnderscore)

	// Double underscore prefix is reserved
	expectError(t, "__reserved")
	expectError(t, "__foo")
}

func TestReservedWords(t *testing.T) {
	// Sample of reserved words that should produce errors
	// Note: "private" is NOT a reserved word in WGSL - it's an address space keyword
	reserved := []string{
		"NULL", "Self", "abstract", "async", "await",
		"class", "enum", "import", "interface", "module",
		"namespace", "new", "null", "public",
		"static", "super", "this", "throw", "try",
		"typeof", "yield",
	}

	for _, word := range reserved {
		t.Run(word, func(t *testing.T) {
			expectError(t, word)
		})
	}
}

// ----------------------------------------------------------------------------
// Numeric Literal Tests
// ----------------------------------------------------------------------------

func TestDecimalIntegers(t *testing.T) {
	cases := []struct {
		input string
		value string
	}{
		{"0", "0"},
		{"1", "1"},
		{"42", "42"},
		{"123456789", "123456789"},
		{"0i", "0i"},   // Signed int suffix
		{"42i", "42i"}, // Signed int suffix
		{"0u", "0u"},   // Unsigned int suffix
		{"42u", "42u"}, // Unsigned int suffix
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectTokenValue(t, tc.input, TokIntLiteral, tc.value)
		})
	}
}

func TestHexIntegers(t *testing.T) {
	cases := []struct {
		input string
		value string
	}{
		{"0x0", "0x0"},
		{"0x1", "0x1"},
		{"0xABCDEF", "0xABCDEF"},
		{"0xabcdef", "0xabcdef"},
		{"0X1234", "0X1234"},
		{"0xFFi", "0xFFi"},
		{"0xFFu", "0xFFu"},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectTokenValue(t, tc.input, TokIntLiteral, tc.value)
		})
	}
}

func TestDecimalFloats(t *testing.T) {
	cases := []struct {
		input string
		value string
	}{
		{"0.0", "0.0"},
		{"1.0", "1.0"},
		{"3.14159", "3.14159"},
		{".5", ".5"},
		{"0.", "0."},
		{"1e10", "1e10"},
		{"1E10", "1E10"},
		{"1e+10", "1e+10"},
		{"1e-10", "1e-10"},
		{"1.5e10", "1.5e10"},
		{"0.5f", "0.5f"},   // f32 suffix
		{"0.5h", "0.5h"},   // f16 suffix
		{"1.0f", "1.0f"},
		{"1f", "1f"},       // Integer with float suffix becomes float
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectTokenValue(t, tc.input, TokFloatLiteral, tc.value)
		})
	}
}

func TestHexFloats(t *testing.T) {
	// WGSL supports hex floats with binary exponent
	cases := []struct {
		input string
		value string
	}{
		{"0x1p0", "0x1p0"},
		{"0x1.0p0", "0x1.0p0"},
		{"0x1P10", "0x1P10"},
		{"0x1.ABCp+10", "0x1.ABCp+10"},
		{"0x1.0p-10", "0x1.0p-10"},
		{"0x1p0f", "0x1p0f"},   // With float suffix
		{"0x1p0h", "0x1p0h"},   // With half suffix
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectTokenValue(t, tc.input, TokFloatLiteral, tc.value)
		})
	}
}

// ----------------------------------------------------------------------------
// Operator Tests
// ----------------------------------------------------------------------------

func TestSingleCharOperators(t *testing.T) {
	cases := []struct {
		input string
		kind  TokenKind
	}{
		{"+", TokPlus},
		{"-", TokMinus},
		{"*", TokStar},
		{"/", TokSlash},
		{"%", TokPercent},
		{"&", TokAmp},
		{"|", TokPipe},
		{"^", TokCaret},
		{"~", TokTilde},
		{"!", TokBang},
		{"<", TokLt},
		{">", TokGt},
		{"=", TokEq},
		{".", TokDot},
		{"@", TokAt},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectToken(t, tc.input, tc.kind)
		})
	}
}

func TestMultiCharOperators(t *testing.T) {
	cases := []struct {
		input string
		kind  TokenKind
	}{
		{"++", TokPlusPlus},
		{"--", TokMinusMinus},
		{"&&", TokAmpAmp},
		{"||", TokPipePipe},
		{"<<", TokLtLt},
		{">>", TokGtGt},
		{"<=", TokLtEq},
		{">=", TokGtEq},
		{"==", TokEqEq},
		{"!=", TokBangEq},
		{"->", TokArrow},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectToken(t, tc.input, tc.kind)
		})
	}
}

func TestAssignmentOperators(t *testing.T) {
	cases := []struct {
		input string
		kind  TokenKind
	}{
		{"+=", TokPlusEq},
		{"-=", TokMinusEq},
		{"*=", TokStarEq},
		{"/=", TokSlashEq},
		{"%=", TokPercentEq},
		{"&=", TokAmpEq},
		{"|=", TokPipeEq},
		{"^=", TokCaretEq},
		{"<<=", TokLtLtEq},
		{">>=", TokGtGtEq},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectToken(t, tc.input, tc.kind)
		})
	}
}

func TestDelimiters(t *testing.T) {
	cases := []struct {
		input string
		kind  TokenKind
	}{
		{"(", TokLParen},
		{")", TokRParen},
		{"{", TokLBrace},
		{"}", TokRBrace},
		{"[", TokLBracket},
		{"]", TokRBracket},
		{";", TokSemicolon},
		{":", TokColon},
		{",", TokComma},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			expectToken(t, tc.input, tc.kind)
		})
	}
}

// ----------------------------------------------------------------------------
// Comment Tests
// ----------------------------------------------------------------------------

func TestLineComments(t *testing.T) {
	// Comments should be skipped
	expectToken(t, "// comment\nfoo", TokIdent)
	expectTokenValue(t, "// comment\nbar", TokIdent, "bar")

	// Comment at end of file
	l := New("foo // comment")
	tok := l.Next()
	if tok.Kind != TokIdent || tok.Value != "foo" {
		t.Errorf("expected identifier 'foo', got %v %q", tok.Kind, tok.Value)
	}
	tok = l.Next()
	if tok.Kind != TokEOF {
		t.Errorf("expected EOF after comment, got %v", tok.Kind)
	}
}

func TestBlockComments(t *testing.T) {
	// Block comments should be skipped
	expectToken(t, "/* comment */ foo", TokIdent)
	expectTokenValue(t, "/* comment */ bar", TokIdent, "bar")

	// Multi-line block comment
	expectTokenValue(t, "/* line1\nline2\nline3 */ baz", TokIdent, "baz")
}

func TestNestedBlockComments(t *testing.T) {
	// WGSL allows nested block comments (unlike JS/C)
	expectTokenValue(t, "/* outer /* inner */ still outer */ foo", TokIdent, "foo")

	// Deeply nested
	expectTokenValue(t, "/* a /* b /* c */ b */ a */ x", TokIdent, "x")
}

// ----------------------------------------------------------------------------
// Whitespace Tests
// ----------------------------------------------------------------------------

func TestWhitespace(t *testing.T) {
	// Various whitespace characters
	expectTokenValue(t, "  \t\n\r  foo", TokIdent, "foo")
	expectTokenValue(t, "\n\n\nbar", TokIdent, "bar")
}

// ----------------------------------------------------------------------------
// Token Sequence Tests
// ----------------------------------------------------------------------------

func TestTokenSequence(t *testing.T) {
	// Test a realistic WGSL snippet
	input := "fn main() -> vec4f { return vec4f(1.0); }"
	expected := []TokenKind{
		TokFn,
		TokIdent,     // main
		TokLParen,
		TokRParen,
		TokArrow,
		TokIdent,     // vec4f
		TokLBrace,
		TokReturn,
		TokIdent,     // vec4f
		TokLParen,
		TokFloatLiteral,
		TokRParen,
		TokSemicolon,
		TokRBrace,
		TokEOF,
	}

	expectTokens(t, input, expected)
}

func TestStructDeclaration(t *testing.T) {
	input := `struct VertexOutput {
		@builtin(position) pos: vec4f,
		@location(0) color: vec3f,
	}`
	expected := []TokenKind{
		TokStruct,
		TokIdent,      // VertexOutput
		TokLBrace,
		TokAt,
		TokIdent,      // builtin
		TokLParen,
		TokIdent,      // position
		TokRParen,
		TokIdent,      // pos
		TokColon,
		TokIdent,      // vec4f
		TokComma,
		TokAt,
		TokIdent,      // location
		TokLParen,
		TokIntLiteral, // 0
		TokRParen,
		TokIdent,      // color
		TokColon,
		TokIdent,      // vec3f
		TokComma,
		TokRBrace,
		TokEOF,
	}

	expectTokens(t, input, expected)
}

func TestVarDeclaration(t *testing.T) {
	input := `@group(0) @binding(1) var<uniform> uniforms: Uniforms;`
	expected := []TokenKind{
		TokAt,
		TokIdent,      // group
		TokLParen,
		TokIntLiteral, // 0
		TokRParen,
		TokAt,
		TokIdent,      // binding
		TokLParen,
		TokIntLiteral, // 1
		TokRParen,
		TokVar,
		TokLt,
		TokIdent,      // uniform
		TokGt,
		TokIdent,      // uniforms
		TokColon,
		TokIdent,      // Uniforms
		TokSemicolon,
		TokEOF,
	}

	expectTokens(t, input, expected)
}

func TestComputeShaderHeader(t *testing.T) {
	input := `@compute @workgroup_size(64, 1, 1)
fn main(@builtin(global_invocation_id) id: vec3u) {`
	expected := []TokenKind{
		TokAt,
		TokIdent,      // compute
		TokAt,
		TokIdent,      // workgroup_size
		TokLParen,
		TokIntLiteral, // 64
		TokComma,
		TokIntLiteral, // 1
		TokComma,
		TokIntLiteral, // 1
		TokRParen,
		TokFn,
		TokIdent,      // main
		TokLParen,
		TokAt,
		TokIdent,      // builtin
		TokLParen,
		TokIdent,      // global_invocation_id
		TokRParen,
		TokIdent,      // id
		TokColon,
		TokIdent,      // vec3u
		TokRParen,
		TokLBrace,
		TokEOF,
	}

	expectTokens(t, input, expected)
}

// ----------------------------------------------------------------------------
// Edge Cases
// ----------------------------------------------------------------------------

func TestEmptyInput(t *testing.T) {
	l := New("")
	tok := l.Next()
	if tok.Kind != TokEOF {
		t.Errorf("expected EOF for empty input, got %v", tok.Kind)
	}
}

func TestOnlyWhitespace(t *testing.T) {
	l := New("   \t\n\r\n   ")
	tok := l.Next()
	if tok.Kind != TokEOF {
		t.Errorf("expected EOF for whitespace-only input, got %v", tok.Kind)
	}
}

func TestOnlyComment(t *testing.T) {
	l := New("// just a comment")
	tok := l.Next()
	if tok.Kind != TokEOF {
		t.Errorf("expected EOF for comment-only input, got %v", tok.Kind)
	}
}

func TestNumberDotMember(t *testing.T) {
	// "1.xxx" should NOT be a float followed by identifier
	// It should be int 1, dot, identifier xxx
	// But in WGSL, numbers can't have methods, so this is likely an error case
	// Let's test "v.x" where v could be a vector
	input := "v.x"
	expected := []TokenKind{
		TokIdent, // v
		TokDot,
		TokIdent, // x
		TokEOF,
	}
	expectTokens(t, input, expected)
}

func TestSwizzle(t *testing.T) {
	input := "pos.xyz"
	expected := []TokenKind{
		TokIdent, // pos
		TokDot,
		TokIdent, // xyz
		TokEOF,
	}
	expectTokens(t, input, expected)
}

func TestChainedMemberAccess(t *testing.T) {
	input := "a.b.c.d"
	expected := []TokenKind{
		TokIdent, // a
		TokDot,
		TokIdent, // b
		TokDot,
		TokIdent, // c
		TokDot,
		TokIdent, // d
		TokEOF,
	}
	expectTokens(t, input, expected)
}

// ----------------------------------------------------------------------------
// Benchmarks
// ----------------------------------------------------------------------------

// Sample WGSL code for benchmarking - representative of real shaders
var benchmarkSource = `
// Vertex shader for basic rendering
struct Uniforms {
    modelViewProjectionMatrix: mat4x4f,
    normalMatrix: mat3x3f,
    lightPosition: vec3f,
    cameraPosition: vec3f,
}

struct VertexInput {
    @location(0) position: vec3f,
    @location(1) normal: vec3f,
    @location(2) texcoord: vec2f,
}

struct VertexOutput {
    @builtin(position) position: vec4f,
    @location(0) worldPosition: vec3f,
    @location(1) worldNormal: vec3f,
    @location(2) texcoord: vec2f,
}

@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var texSampler: sampler;
@group(0) @binding(2) var baseColorTexture: texture_2d<f32>;

@vertex
fn vertexMain(input: VertexInput) -> VertexOutput {
    var output: VertexOutput;
    let worldPos = vec4f(input.position, 1.0);
    output.position = uniforms.modelViewProjectionMatrix * worldPos;
    output.worldPosition = worldPos.xyz;
    output.worldNormal = uniforms.normalMatrix * input.normal;
    output.texcoord = input.texcoord;
    return output;
}

@fragment
fn fragmentMain(input: VertexOutput) -> @location(0) vec4f {
    let baseColor = textureSample(baseColorTexture, texSampler, input.texcoord);
    let normal = normalize(input.worldNormal);
    let lightDir = normalize(uniforms.lightPosition - input.worldPosition);
    let viewDir = normalize(uniforms.cameraPosition - input.worldPosition);
    let halfDir = normalize(lightDir + viewDir);

    let ambient = 0.1;
    let diffuse = max(dot(normal, lightDir), 0.0);
    let specular = pow(max(dot(normal, halfDir), 0.0), 32.0);

    let lighting = ambient + diffuse + specular * 0.5;
    return vec4f(baseColor.rgb * lighting, baseColor.a);
}
`

// BenchmarkLexer measures tokenization performance
func BenchmarkLexer(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchmarkSource)))

	for i := 0; i < b.N; i++ {
		l := New(benchmarkSource)
		_ = l.Tokenize()
	}
}

// BenchmarkLexerIdentifiers tests identifier-heavy code (where ASCII fast path helps most)
func BenchmarkLexerIdentifiers(b *testing.B) {
	// Generate identifier-heavy source
	source := ""
	for i := 0; i < 1000; i++ {
		source += "let variableName" + string(rune('A'+i%26)) + " = someFunction(arg1, arg2, arg3);\n"
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l := New(source)
		_ = l.Tokenize()
	}
}

// BenchmarkLexerUnicode tests performance with Unicode identifiers
func BenchmarkLexerUnicode(b *testing.B) {
	// Mix of ASCII and Unicode identifiers
	source := ""
	for i := 0; i < 500; i++ {
		source += "let α" + string(rune('α'+i%20)) + " = β * γ + δ;\n"
		source += "let position" + string(rune('A'+i%26)) + " = vec3f(1.0, 2.0, 3.0);\n"
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l := New(source)
		_ = l.Tokenize()
	}
}

// BenchmarkLexerNumbers tests number literal parsing
func BenchmarkLexerNumbers(b *testing.B) {
	source := ""
	for i := 0; i < 500; i++ {
		source += "const a = 123456789;\n"
		source += "const b = 0xABCDEF;\n"
		source += "const c = 3.14159265f;\n"
		source += "const d = 1.0e-10;\n"
		source += "const e = 0x1.5p10;\n"
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l := New(source)
		_ = l.Tokenize()
	}
}

// BenchmarkLexerComments tests comment skipping performance
func BenchmarkLexerComments(b *testing.B) {
	source := ""
	for i := 0; i < 200; i++ {
		source += "// This is a line comment that describes the next line of code\n"
		source += "let x = 1;\n"
		source += "/* Block comment with some content */ let y = 2;\n"
		source += "/* Nested /* comment */ still going */ let z = 3;\n"
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(source)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		l := New(source)
		_ = l.Tokenize()
	}
}
