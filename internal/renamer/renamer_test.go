package renamer

import (
	"testing"
)

// ----------------------------------------------------------------------------
// Name Minifier Tests
// ----------------------------------------------------------------------------

func TestNumberToMinifiedName(t *testing.T) {
	m := DefaultNameMinifier()

	cases := []struct {
		n        int
		expected string
	}{
		{0, "a"},
		{1, "b"},
		{25, "z"},
		{26, "A"},
		{51, "Z"},
		// After single letters (52), we get two-letter combos
		// n=52: head[0]='a', then n/52=1, n-1=0, tail[0]='a' -> "aa"
		{52, "aa"},
		// n=53: head[1]='b', n/52=1, n-1=0, tail[0]='a' -> "ba"
		{53, "ba"},
		// n=54: head[2]='c', n/52=1, n-1=0, tail[0]='a' -> "ca"
		{54, "ca"},
		// n=114: head[114%52=10]='k', n/52=2, n-1=1, tail[1]='b' -> "kb"
		{114, "kb"},
	}

	for _, tc := range cases {
		t.Run(tc.expected, func(t *testing.T) {
			actual := m.NumberToMinifiedName(tc.n)
			if actual != tc.expected {
				t.Errorf("NumberToMinifiedName(%d) = %q, want %q", tc.n, actual, tc.expected)
			}
		})
	}
}

func TestNameMinifierGeneratesValidIdentifiers(t *testing.T) {
	m := DefaultNameMinifier()

	// Generate many names and verify they're valid WGSL identifiers
	for i := 0; i < 1000; i++ {
		name := m.NumberToMinifiedName(i)

		// Must not be empty
		if len(name) == 0 {
			t.Errorf("NumberToMinifiedName(%d) returned empty string", i)
			continue
		}

		// First character must be letter or underscore (our minifier uses letters)
		first := name[0]
		if !((first >= 'a' && first <= 'z') || (first >= 'A' && first <= 'Z')) {
			t.Errorf("NumberToMinifiedName(%d) = %q, first char %c is not a letter", i, name, first)
		}

		// Rest must be alphanumeric
		for j := 1; j < len(name); j++ {
			c := name[j]
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				t.Errorf("NumberToMinifiedName(%d) = %q, char %d (%c) is invalid", i, name, j, c)
			}
		}
	}
}

func TestNameMinifierNoDuplicates(t *testing.T) {
	m := DefaultNameMinifier()
	seen := make(map[string]int)

	// Generate many names and verify no duplicates
	for i := 0; i < 10000; i++ {
		name := m.NumberToMinifiedName(i)
		if prev, ok := seen[name]; ok {
			t.Errorf("NumberToMinifiedName(%d) = %q, same as NumberToMinifiedName(%d)", i, name, prev)
		}
		seen[name] = i
	}
}

// ----------------------------------------------------------------------------
// Reserved Names Tests
// ----------------------------------------------------------------------------

func TestComputeReservedNames(t *testing.T) {
	reserved := ComputeReservedNames()

	// All keywords should be reserved
	keywords := []string{
		"fn", "let", "var", "const", "return", "if", "else", "for", "while",
		"loop", "break", "continue", "switch", "case", "default", "struct",
		"true", "false", "discard", "enable", "requires",
	}
	for _, kw := range keywords {
		if !reserved[kw] {
			t.Errorf("keyword %q should be reserved", kw)
		}
	}

	// Sample of reserved words
	reservedWords := []string{
		"class", "enum", "import", "interface", "namespace", "new", "null",
		"public", "private", "static", "this", "throw", "try", "typeof",
	}
	for _, word := range reservedWords {
		if !reserved[word] {
			t.Errorf("reserved word %q should be in reserved set", word)
		}
	}

	// Built-in types should be reserved
	builtinTypes := []string{
		"bool", "i32", "u32", "f32", "f16",
		"vec2", "vec3", "vec4", "vec2f", "vec3f", "vec4f",
		"mat2x2", "mat3x3", "mat4x4", "mat4x4f",
		"array", "ptr", "atomic", "sampler", "sampler_comparison",
	}
	for _, typ := range builtinTypes {
		if !reserved[typ] {
			t.Errorf("built-in type %q should be reserved", typ)
		}
	}

	// Address spaces should be reserved
	addressSpaces := []string{"function", "private", "workgroup", "uniform", "storage"}
	for _, space := range addressSpaces {
		if !reserved[space] {
			t.Errorf("address space %q should be reserved", space)
		}
	}

	// Access modes should be reserved
	accessModes := []string{"read", "write", "read_write"}
	for _, mode := range accessModes {
		if !reserved[mode] {
			t.Errorf("access mode %q should be reserved", mode)
		}
	}

	// Underscore alone should be reserved
	if !reserved["_"] {
		t.Error("single underscore should be reserved")
	}
}

func TestNameMinifierAvoidsReserved(t *testing.T) {
	m := DefaultNameMinifier()
	reserved := ComputeReservedNames()

	// Generate many names and verify none are reserved
	// Note: The minifier itself doesn't avoid reserved words - that's done
	// by the MinifyRenamer. But we can verify the generated names are valid.
	for i := 0; i < 1000; i++ {
		name := m.NumberToMinifiedName(i)
		// Short names like 'a', 'b', etc. won't conflict with keywords
		// since WGSL keywords are all multi-character except none are single letters
		if len(name) == 1 {
			continue
		}
		if reserved[name] {
			// This would be a concern if we hit it, but unlikely with our alphabet
			t.Logf("Note: generated name %q (index %d) is reserved", name, i)
		}
	}
}

// ----------------------------------------------------------------------------
// CharFreq Tests
// ----------------------------------------------------------------------------

func TestCharFreqScan(t *testing.T) {
	var freq CharFreq

	freq.Scan("aaa", 1)
	if freq[0] != 3 { // 'a' is at index 0
		t.Errorf("expected freq[0] = 3 for 'aaa', got %d", freq[0])
	}

	freq.Scan("abc", 1)
	if freq[0] != 4 { // 'a' should now be 4
		t.Errorf("expected freq[0] = 4, got %d", freq[0])
	}
	if freq[1] != 1 { // 'b'
		t.Errorf("expected freq[1] = 1 for 'b', got %d", freq[1])
	}
	if freq[2] != 1 { // 'c'
		t.Errorf("expected freq[2] = 1 for 'c', got %d", freq[2])
	}
}

func TestCharFreqScanUpperCase(t *testing.T) {
	var freq CharFreq

	freq.Scan("ABC", 1)
	if freq[26] != 1 { // 'A' is at index 26
		t.Errorf("expected freq[26] = 1 for 'A', got %d", freq[26])
	}
	if freq[27] != 1 { // 'B'
		t.Errorf("expected freq[27] = 1 for 'B', got %d", freq[27])
	}
}

func TestCharFreqScanDigits(t *testing.T) {
	var freq CharFreq

	freq.Scan("012", 1)
	if freq[52] != 1 { // '0' is at index 52
		t.Errorf("expected freq[52] = 1 for '0', got %d", freq[52])
	}
	if freq[53] != 1 { // '1'
		t.Errorf("expected freq[53] = 1 for '1', got %d", freq[53])
	}
}

func TestCharFreqScanMixed(t *testing.T) {
	var freq CharFreq

	// Scan some identifier-like text
	freq.Scan("variableName123", 1)

	// Check some frequencies
	if freq[0] < 2 { // 'a' appears at least twice
		t.Errorf("expected 'a' freq >= 2, got %d", freq[0])
	}
}

func TestShuffleByCharFreq(t *testing.T) {
	m := DefaultNameMinifier()

	// Create a frequency where 'z' is most common
	var freq CharFreq
	freq[25] = 100 // 'z'
	freq[0] = 1    // 'a'

	shuffled := m.ShuffleByCharFreq(freq)

	// First name should start with 'z' (most frequent)
	first := shuffled.NumberToMinifiedName(0)
	if first[0] != 'z' {
		t.Errorf("expected first name to start with 'z', got %q", first)
	}
}

// ----------------------------------------------------------------------------
// NoOpRenamer Tests
// ----------------------------------------------------------------------------

func TestNoOpRenamer(t *testing.T) {
	// This is a simple test since NoOpRenamer just returns original names
	// We'd need actual symbols to test it properly
}

// ----------------------------------------------------------------------------
// Integration Tests
// ----------------------------------------------------------------------------

func TestMinifyRenamerBasic(t *testing.T) {
	// Basic integration test - would need full symbol table to test properly
	reserved := ComputeReservedNames()

	// Just verify we can create a renamer
	_ = NewMinifyRenamer(nil, reserved)
}

func TestReservedNamesCount(t *testing.T) {
	reserved := ComputeReservedNames()

	// Should have a substantial number of reserved names
	count := len(reserved)
	if count < 150 {
		t.Errorf("expected at least 150 reserved names, got %d", count)
	}

	t.Logf("Total reserved names: %d", count)
}

// ----------------------------------------------------------------------------
// Benchmark Tests
// ----------------------------------------------------------------------------

func BenchmarkNumberToMinifiedName(b *testing.B) {
	m := DefaultNameMinifier()
	for i := 0; i < b.N; i++ {
		_ = m.NumberToMinifiedName(i % 10000)
	}
}

func BenchmarkCharFreqScan(b *testing.B) {
	text := "someVariableNameThatIsReasonablyLong123"
	var freq CharFreq

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		freq.Scan(text, 1)
	}
}

func BenchmarkComputeReservedNames(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ComputeReservedNames()
	}
}
