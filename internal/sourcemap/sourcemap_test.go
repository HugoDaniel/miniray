package sourcemap

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// ============================================================================
// Source Map Structure Tests
// ============================================================================

func TestSourceMapVersion(t *testing.T) {
	g := NewGenerator("")
	sm := g.Generate()

	if sm.Version != 3 {
		t.Errorf("Version = %d, want 3", sm.Version)
	}
}

func TestSourceMapSingleMapping(t *testing.T) {
	source := "const x = 1;"
	g := NewGenerator(source)

	// Add mapping for 'x' at source offset 6
	// Generated position: line 0, col 6
	g.AddMapping(0, 6, 6, "x")

	sm := g.Generate()

	// Check names array
	if len(sm.Names) != 1 || sm.Names[0] != "x" {
		t.Errorf("Names = %v, want [x]", sm.Names)
	}

	// Mappings should be non-empty
	if sm.Mappings == "" {
		t.Error("Mappings is empty")
	}

	// Decode and verify
	decoded, err := DecodeMappings(sm.Mappings)
	if err != nil {
		t.Fatalf("Failed to decode mappings: %v", err)
	}

	if len(decoded) != 1 {
		t.Fatalf("Expected 1 mapping, got %d", len(decoded))
	}

	m := decoded[0]
	if m.GenLine != 0 || m.GenCol != 6 {
		t.Errorf("Generated position = (%d, %d), want (0, 6)", m.GenLine, m.GenCol)
	}
	if m.SrcLine != 0 || m.SrcCol != 6 {
		t.Errorf("Source position = (%d, %d), want (0, 6)", m.SrcLine, m.SrcCol)
	}
	if !m.HasName || m.NameIndex != 0 {
		t.Errorf("Name mapping incorrect: HasName=%v, NameIndex=%d", m.HasName, m.NameIndex)
	}
}

func TestSourceMapMultipleMappingsSameLine(t *testing.T) {
	source := "const a = b + c;"
	g := NewGenerator(source)

	// Add mappings for a, b, c on same generated line
	g.AddMapping(0, 6, 6, "a")
	g.AddMapping(0, 10, 10, "b")
	g.AddMapping(0, 14, 14, "c")

	sm := g.Generate()

	// Check names
	if len(sm.Names) != 3 {
		t.Errorf("Names count = %d, want 3", len(sm.Names))
	}

	// Mappings on same line should be comma-separated
	if strings.Contains(sm.Mappings, ";") {
		t.Error("Single line should not have semicolons in mappings")
	}
	if !strings.Contains(sm.Mappings, ",") {
		t.Error("Multiple mappings should be comma-separated")
	}
}

func TestSourceMapMultipleLines(t *testing.T) {
	source := "const a = 1;\nconst b = 2;\nconst c = 3;"
	g := NewGenerator(source)

	// Add mappings on different generated lines
	g.AddMapping(0, 0, 0, "")  // line 0
	g.AddMapping(1, 0, 13, "") // line 1
	g.AddMapping(2, 0, 26, "") // line 2

	sm := g.Generate()

	// Mappings on different lines should be semicolon-separated
	semicolonCount := strings.Count(sm.Mappings, ";")
	if semicolonCount != 2 {
		t.Errorf("Expected 2 semicolons for 3 lines, got %d", semicolonCount)
	}
}

func TestSourceMapNameDeduplication(t *testing.T) {
	source := "let x = 1; let y = x + x;"
	g := NewGenerator(source)

	// Add multiple mappings with same name
	g.AddMapping(0, 4, 4, "x")
	g.AddMapping(0, 19, 19, "x") // same name
	g.AddMapping(0, 23, 23, "x") // same name

	sm := g.Generate()

	// Names should be deduplicated
	if len(sm.Names) != 1 {
		t.Errorf("Names count = %d, want 1 (deduplicated)", len(sm.Names))
	}
}

func TestSourceMapMappingWithoutName(t *testing.T) {
	source := "return 42;"
	g := NewGenerator(source)

	// Add mapping without name (for keywords, literals, etc.)
	g.AddMapping(0, 0, 0, "")

	sm := g.Generate()

	// Names should be empty
	if len(sm.Names) != 0 {
		t.Errorf("Names = %v, want empty", sm.Names)
	}

	// Should still have mapping
	if sm.Mappings == "" {
		t.Error("Mappings should not be empty")
	}
}

func TestSourceMapEmptySource(t *testing.T) {
	g := NewGenerator("")
	sm := g.Generate()

	if sm.Version != 3 {
		t.Errorf("Version = %d, want 3", sm.Version)
	}
	if sm.Mappings != "" {
		t.Errorf("Mappings = %q, want empty", sm.Mappings)
	}
}

func TestSourceMapJSONFormat(t *testing.T) {
	source := "const x = 1;"
	g := NewGenerator(source)
	g.AddMapping(0, 6, 6, "x")
	g.SetFile("test.wgsl")
	g.SetSourceName("test.wgsl")

	sm := g.Generate()
	jsonStr := sm.ToJSON()

	// Parse JSON
	var parsed SourceMap
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Verify fields
	if parsed.Version != 3 {
		t.Errorf("Parsed version = %d, want 3", parsed.Version)
	}
	if parsed.File != "test.wgsl" {
		t.Errorf("Parsed file = %q, want %q", parsed.File, "test.wgsl")
	}
	if len(parsed.Sources) != 1 || parsed.Sources[0] != "test.wgsl" {
		t.Errorf("Parsed sources = %v, want [test.wgsl]", parsed.Sources)
	}
	if len(parsed.Names) != 1 || parsed.Names[0] != "x" {
		t.Errorf("Parsed names = %v, want [x]", parsed.Names)
	}
}

func TestSourceMapDataURI(t *testing.T) {
	source := "const x = 1;"
	g := NewGenerator(source)
	g.AddMapping(0, 6, 6, "x")

	sm := g.Generate()
	dataURI := sm.ToDataURI()

	// Check prefix
	prefix := "data:application/json;base64,"
	if !strings.HasPrefix(dataURI, prefix) {
		t.Errorf("Data URI should start with %q", prefix)
	}

	// Decode base64
	encoded := strings.TrimPrefix(dataURI, prefix)
	decoded, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatalf("Failed to decode base64: %v", err)
	}

	// Parse JSON
	var parsed SourceMap
	if err := json.Unmarshal(decoded, &parsed); err != nil {
		t.Fatalf("Decoded JSON invalid: %v", err)
	}

	if parsed.Version != 3 {
		t.Errorf("Decoded version = %d, want 3", parsed.Version)
	}
}

func TestSourceMapSourcesContent(t *testing.T) {
	source := "const x = 1;"
	g := NewGenerator(source)
	g.SetSourceName("test.wgsl")
	g.IncludeSourceContent(true)

	sm := g.Generate()

	if len(sm.SourcesContent) != 1 {
		t.Fatalf("SourcesContent length = %d, want 1", len(sm.SourcesContent))
	}
	if sm.SourcesContent[0] != source {
		t.Errorf("SourcesContent[0] = %q, want %q", sm.SourcesContent[0], source)
	}
}

func TestSourceMapFile(t *testing.T) {
	g := NewGenerator("const x = 1;")
	g.SetFile("output.min.wgsl")

	sm := g.Generate()

	if sm.File != "output.min.wgsl" {
		t.Errorf("File = %q, want %q", sm.File, "output.min.wgsl")
	}
}

func TestSourceMapMappingsFormat(t *testing.T) {
	source := "a\nb\nc"
	g := NewGenerator(source)

	// Add mapping on each line
	g.AddMapping(0, 0, 0, "")
	g.AddMapping(1, 0, 2, "")
	g.AddMapping(2, 0, 4, "")

	sm := g.Generate()

	// Should have format: "segment;segment;segment"
	parts := strings.Split(sm.Mappings, ";")
	if len(parts) != 3 {
		t.Errorf("Expected 3 parts separated by ';', got %d: %q", len(parts), sm.Mappings)
	}
}

func TestSourceMapDeltaEncoding(t *testing.T) {
	source := "const abc = 1;\nconst def = 2;"
	g := NewGenerator(source)

	// Add mappings - second one should use deltas from first
	g.AddMapping(0, 6, 6, "abc")
	g.AddMapping(1, 6, 21, "def") // line 1, col 6, source offset 21

	sm := g.Generate()

	// Verify by decoding
	decoded, err := DecodeMappings(sm.Mappings)
	if err != nil {
		t.Fatalf("Failed to decode: %v", err)
	}

	if len(decoded) != 2 {
		t.Fatalf("Expected 2 mappings, got %d", len(decoded))
	}

	// First mapping
	if decoded[0].GenLine != 0 || decoded[0].GenCol != 6 {
		t.Errorf("First mapping gen pos = (%d, %d), want (0, 6)",
			decoded[0].GenLine, decoded[0].GenCol)
	}

	// Second mapping
	if decoded[1].GenLine != 1 || decoded[1].GenCol != 6 {
		t.Errorf("Second mapping gen pos = (%d, %d), want (1, 6)",
			decoded[1].GenLine, decoded[1].GenCol)
	}
}

// ============================================================================
// Mapping Decode Tests
// ============================================================================

func TestDecodeMappingsEmpty(t *testing.T) {
	decoded, err := DecodeMappings("")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(decoded) != 0 {
		t.Errorf("Expected 0 mappings, got %d", len(decoded))
	}
}

func TestDecodeMappingsSimple(t *testing.T) {
	// "AAAA" = col 0, srcIdx 0, srcLine 0, srcCol 0
	decoded, err := DecodeMappings("AAAA")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(decoded) != 1 {
		t.Fatalf("Expected 1 mapping, got %d", len(decoded))
	}

	m := decoded[0]
	if m.GenLine != 0 || m.GenCol != 0 || m.SrcLine != 0 || m.SrcCol != 0 {
		t.Errorf("Mapping = gen(%d,%d) src(%d,%d), want gen(0,0) src(0,0)",
			m.GenLine, m.GenCol, m.SrcLine, m.SrcCol)
	}
}

func TestDecodeMappingsMultipleSegments(t *testing.T) {
	// Two segments on same line
	decoded, err := DecodeMappings("AAAA,CACA")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(decoded) != 2 {
		t.Fatalf("Expected 2 mappings, got %d", len(decoded))
	}

	// First: col 0, src 0,0
	if decoded[0].GenCol != 0 {
		t.Errorf("First mapping col = %d, want 0", decoded[0].GenCol)
	}

	// Second: col 1 (delta), src line +1, col 0
	if decoded[1].GenCol != 1 {
		t.Errorf("Second mapping col = %d, want 1", decoded[1].GenCol)
	}
}

func TestDecodeMappingsMultipleLines(t *testing.T) {
	// Three lines
	decoded, err := DecodeMappings("AAAA;CACA;EAEA")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(decoded) != 3 {
		t.Fatalf("Expected 3 mappings, got %d", len(decoded))
	}

	// Each on different generated line
	for i, m := range decoded {
		if m.GenLine != i {
			t.Errorf("Mapping %d: GenLine = %d, want %d", i, m.GenLine, i)
		}
	}
}

// ============================================================================
// Benchmark Tests
// ============================================================================

func BenchmarkGeneratorAddMapping(b *testing.B) {
	source := strings.Repeat("const x = 1;\n", 1000)
	g := NewGenerator(source)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.AddMapping(i%1000, (i*6)%100, i*13, "x")
	}
}

func BenchmarkGeneratorGenerate(b *testing.B) {
	source := strings.Repeat("const x = 1;\n", 1000)
	g := NewGenerator(source)
	for i := 0; i < 1000; i++ {
		g.AddMapping(i, 6, i*13+6, "x")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		g.Generate()
	}
}
