package sourcemap

import (
	"fmt"
	"strings"
	"testing"
)

// ============================================================================
// Line Index Tests
// ============================================================================

func TestLineIndexEmpty(t *testing.T) {
	idx := NewLineIndex("")
	if idx.LineCount() != 1 {
		t.Errorf("Empty source LineCount() = %d, want 1", idx.LineCount())
	}

	line, col := idx.ByteOffsetToLineColumn(0)
	if line != 0 || col != 0 {
		t.Errorf("Empty source offset 0: got (%d, %d), want (0, 0)", line, col)
	}
}

func TestLineIndexSingleLine(t *testing.T) {
	source := "const x = 1;"
	idx := NewLineIndex(source)

	if idx.LineCount() != 1 {
		t.Errorf("Single line LineCount() = %d, want 1", idx.LineCount())
	}

	tests := []struct {
		offset int
		line   int
		col    int
	}{
		{0, 0, 0},   // 'c'
		{6, 0, 6},   // 'x'
		{11, 0, 11}, // ';'
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("offset_%d", tt.offset), func(t *testing.T) {
			line, col := idx.ByteOffsetToLineColumn(tt.offset)
			if line != tt.line || col != tt.col {
				t.Errorf("offset %d: got (%d, %d), want (%d, %d)",
					tt.offset, line, col, tt.line, tt.col)
			}
		})
	}
}

func TestLineIndexMultiLine(t *testing.T) {
	source := "const x = 1;\nconst y = 2;\nconst z = 3;"
	idx := NewLineIndex(source)

	if idx.LineCount() != 3 {
		t.Errorf("LineCount() = %d, want 3", idx.LineCount())
	}

	tests := []struct {
		offset int
		line   int
		col    int
	}{
		{0, 0, 0},   // 'c' of first line
		{6, 0, 6},   // 'x' of first line
		{12, 0, 12}, // ';' of first line
		{13, 1, 0},  // 'c' of second line (after \n)
		{19, 1, 6},  // 'y' of second line
		{26, 2, 0},  // 'c' of third line
		{32, 2, 6},  // 'z' of third line
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("offset_%d", tt.offset), func(t *testing.T) {
			line, col := idx.ByteOffsetToLineColumn(tt.offset)
			if line != tt.line || col != tt.col {
				t.Errorf("offset %d: got (%d, %d), want (%d, %d)",
					tt.offset, line, col, tt.line, tt.col)
			}
		})
	}
}

func TestLineIndexNewlineStyles(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		lineCount int
	}{
		{"unix_lf", "a\nb\nc", 3},
		{"windows_crlf", "a\r\nb\r\nc", 3},
		{"old_mac_cr", "a\rb\rc", 3},
		{"trailing_lf", "a\nb\n", 2},
		{"trailing_crlf", "a\r\nb\r\n", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			idx := NewLineIndex(tt.source)
			if idx.LineCount() != tt.lineCount {
				t.Errorf("LineCount() = %d, want %d", idx.LineCount(), tt.lineCount)
			}
		})
	}
}

func TestLineIndexCRLFPositions(t *testing.T) {
	// Test that CRLF is treated as single newline
	source := "ab\r\ncd\r\nef"
	idx := NewLineIndex(source)

	tests := []struct {
		offset int
		line   int
		col    int
	}{
		{0, 0, 0}, // 'a'
		{1, 0, 1}, // 'b'
		{2, 0, 2}, // '\r' (still on line 0)
		{4, 1, 0}, // 'c' (first char of line 1)
		{5, 1, 1}, // 'd'
		{8, 2, 0}, // 'e' (first char of line 2)
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("offset_%d", tt.offset), func(t *testing.T) {
			line, col := idx.ByteOffsetToLineColumn(tt.offset)
			if line != tt.line || col != tt.col {
				t.Errorf("offset %d: got (%d, %d), want (%d, %d)",
					tt.offset, line, col, tt.line, tt.col)
			}
		})
	}
}

func TestByteOffsetToLineColumnOutOfBounds(t *testing.T) {
	source := "abc"
	idx := NewLineIndex(source)

	// Test offset beyond source length
	line, col := idx.ByteOffsetToLineColumn(100)
	// Should clamp to end of source
	if line != 0 || col != 3 {
		t.Errorf("Out of bounds offset: got (%d, %d), want (0, 3)", line, col)
	}

	// Test negative offset
	line, col = idx.ByteOffsetToLineColumn(-1)
	if line != 0 || col != 0 {
		t.Errorf("Negative offset: got (%d, %d), want (0, 0)", line, col)
	}
}

func TestUTF8MultibyteBasic(t *testing.T) {
	// Basic ASCII - columns should match bytes
	source := "const x = 1;"
	idx := NewLineIndex(source)

	line, col := idx.ByteOffsetToLineColumn(6)
	if line != 0 || col != 6 {
		t.Errorf("ASCII offset 6: got (%d, %d), want (0, 6)", line, col)
	}
}

func TestUTF8MultibyteEmoji(t *testing.T) {
	// Emoji test: "😀" is 4 UTF-8 bytes but 2 UTF-16 code units
	// Source map spec uses UTF-16 columns
	source := "a😀b"
	idx := NewLineIndex(source)

	tests := []struct {
		offset   int
		line     int
		col      int // UTF-16 column
		describe string
	}{
		{0, 0, 0, "before emoji"},
		{1, 0, 1, "start of emoji"}, // 😀 starts at byte 1
		{5, 0, 3, "after emoji"},    // 'b' is at byte 5, but UTF-16 col 3 (1 + 2 for emoji)
	}

	for _, tt := range tests {
		t.Run(tt.describe, func(t *testing.T) {
			line, col := idx.ByteOffsetToLineColumnUTF16(tt.offset)
			if line != tt.line || col != tt.col {
				t.Errorf("offset %d (%s): got (%d, %d), want (%d, %d)",
					tt.offset, tt.describe, line, col, tt.line, tt.col)
			}
		})
	}
}

func TestUTF8MultibyteMultipleEmojis(t *testing.T) {
	// Multiple emojis: "👍👎" - each is 4 UTF-8 bytes, 2 UTF-16 code units
	source := "a👍👎b"
	idx := NewLineIndex(source)

	// 'a' at byte 0, col 0
	// 👍 at bytes 1-4, UTF-16 cols 1-2
	// 👎 at bytes 5-8, UTF-16 cols 3-4
	// 'b' at byte 9, UTF-16 col 5

	tests := []struct {
		offset int
		col    int // UTF-16 column
	}{
		{0, 0}, // 'a'
		{1, 1}, // start of 👍
		{5, 3}, // start of 👎
		{9, 5}, // 'b'
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("offset_%d", tt.offset), func(t *testing.T) {
			_, col := idx.ByteOffsetToLineColumnUTF16(tt.offset)
			if col != tt.col {
				t.Errorf("offset %d: UTF-16 col = %d, want %d", tt.offset, col, tt.col)
			}
		})
	}
}

func TestUTF8MultibyteMixedContent(t *testing.T) {
	// Mix of ASCII and multibyte chars
	// "café" - 'é' is 2 UTF-8 bytes but 1 UTF-16 code unit (BMP character)
	source := "café"
	idx := NewLineIndex(source)

	// 'c' byte 0, col 0
	// 'a' byte 1, col 1
	// 'f' byte 2, col 2
	// 'é' bytes 3-4, col 3 (single UTF-16 code unit)

	tests := []struct {
		offset int
		col    int
	}{
		{0, 0}, // 'c'
		{1, 1}, // 'a'
		{2, 2}, // 'f'
		{3, 3}, // 'é' (start)
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("offset_%d", tt.offset), func(t *testing.T) {
			_, col := idx.ByteOffsetToLineColumnUTF16(tt.offset)
			if col != tt.col {
				t.Errorf("offset %d: UTF-16 col = %d, want %d", tt.offset, col, tt.col)
			}
		})
	}
}

func TestVeryLongLine(t *testing.T) {
	// Create a source with a very long line
	var builder strings.Builder
	builder.WriteString("const x = ")
	for i := 0; i < 10000; i++ {
		builder.WriteString("a")
	}
	builder.WriteString(";")
	source := builder.String()

	idx := NewLineIndex(source)

	if idx.LineCount() != 1 {
		t.Errorf("LineCount() = %d, want 1", idx.LineCount())
	}

	// Check position near end
	offset := len(source) - 1
	line, col := idx.ByteOffsetToLineColumn(offset)
	if line != 0 {
		t.Errorf("Line = %d, want 0", line)
	}
	if col != offset {
		t.Errorf("Col = %d, want %d", col, offset)
	}
}

func TestManyLines(t *testing.T) {
	// Create source with many lines
	var builder strings.Builder
	lineCount := 10000
	for i := 0; i < lineCount; i++ {
		builder.WriteString(fmt.Sprintf("const x%d = %d;\n", i, i))
	}
	source := builder.String()

	idx := NewLineIndex(source)

	if idx.LineCount() != lineCount {
		t.Errorf("LineCount() = %d, want %d", idx.LineCount(), lineCount)
	}

	// Check first line
	line, col := idx.ByteOffsetToLineColumn(0)
	if line != 0 || col != 0 {
		t.Errorf("First char: got (%d, %d), want (0, 0)", line, col)
	}

	// Check middle of source - find a known line
	// Line 5000 should start somewhere in the middle
	// Just verify we get a reasonable line number for a middle offset
	midOffset := len(source) / 2
	line, _ = idx.ByteOffsetToLineColumn(midOffset)
	if line < lineCount/4 || line > lineCount*3/4 {
		t.Errorf("Middle offset %d mapped to line %d, expected between %d and %d",
			midOffset, line, lineCount/4, lineCount*3/4)
	}

	// Check last line
	lastLineStart := len(source) - 20 // approximate start of last line
	line, _ = idx.ByteOffsetToLineColumn(lastLineStart)
	if line != lineCount-1 {
		t.Errorf("Last line = %d, want %d", line, lineCount-1)
	}
}

// Benchmark tests
func BenchmarkNewLineIndex(b *testing.B) {
	var builder strings.Builder
	for i := 0; i < 1000; i++ {
		builder.WriteString(fmt.Sprintf("const x%d = %d;\n", i, i))
	}
	source := builder.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewLineIndex(source)
	}
}

func BenchmarkByteOffsetToLineColumn(b *testing.B) {
	var builder strings.Builder
	for i := 0; i < 1000; i++ {
		builder.WriteString(fmt.Sprintf("const x%d = %d;\n", i, i))
	}
	source := builder.String()
	idx := NewLineIndex(source)
	offset := len(source) / 2

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx.ByteOffsetToLineColumn(offset)
	}
}

// ============================================================================
// LineColumnToByteOffset Tests
// ============================================================================

func TestLineColumnToByteOffsetBasic(t *testing.T) {
	source := "const x = 1;\nconst y = 2;\n"
	idx := NewLineIndex(source)

	tests := []struct {
		line   int
		col    int
		offset int
	}{
		{0, 0, 0},  // Start of first line
		{0, 6, 6},  // Middle of first line
		{1, 0, 13}, // Start of second line
		{1, 6, 19}, // Middle of second line
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("line%d_col%d", tt.line, tt.col), func(t *testing.T) {
			offset := idx.LineColumnToByteOffset(tt.line, tt.col)
			if offset != tt.offset {
				t.Errorf("LineColumnToByteOffset(%d, %d) = %d, want %d",
					tt.line, tt.col, offset, tt.offset)
			}
		})
	}
}

func TestLineColumnToByteOffsetNegativeLine(t *testing.T) {
	source := "abc\ndef\n"
	idx := NewLineIndex(source)

	// Negative line should clamp to 0
	offset := idx.LineColumnToByteOffset(-1, 2)
	if offset != 2 {
		t.Errorf("LineColumnToByteOffset(-1, 2) = %d, want 2", offset)
	}
}

func TestLineColumnToByteOffsetLineOutOfBounds(t *testing.T) {
	source := "abc\ndef\n"
	idx := NewLineIndex(source)

	// Line beyond end should clamp to last line
	offset := idx.LineColumnToByteOffset(100, 0)
	// Last line starts at 4 ("def\n")
	if offset != 4 {
		t.Errorf("LineColumnToByteOffset(100, 0) = %d, want 4", offset)
	}
}

func TestLineColumnToByteOffsetColumnOutOfBounds(t *testing.T) {
	source := "abc"
	idx := NewLineIndex(source)

	// Column beyond source should clamp
	offset := idx.LineColumnToByteOffset(0, 100)
	if offset != 3 {
		t.Errorf("LineColumnToByteOffset(0, 100) = %d, want 3", offset)
	}
}

func TestLineColumnToByteOffsetNegativeColumn(t *testing.T) {
	source := "abc"
	idx := NewLineIndex(source)

	// Negative column should return 0
	offset := idx.LineColumnToByteOffset(0, -10)
	if offset != 0 {
		t.Errorf("LineColumnToByteOffset(0, -10) = %d, want 0", offset)
	}
}

// ============================================================================
// UTF-16 Column Edge Cases
// ============================================================================

func TestByteOffsetToLineColumnUTF16Negative(t *testing.T) {
	source := "abc"
	idx := NewLineIndex(source)

	line, col := idx.ByteOffsetToLineColumnUTF16(-1)
	if line != 0 || col != 0 {
		t.Errorf("Negative offset: got (%d, %d), want (0, 0)", line, col)
	}
}

func TestByteOffsetToLineColumnUTF16Empty(t *testing.T) {
	idx := NewLineIndex("")

	line, col := idx.ByteOffsetToLineColumnUTF16(0)
	if line != 0 || col != 0 {
		t.Errorf("Empty source: got (%d, %d), want (0, 0)", line, col)
	}

	line, col = idx.ByteOffsetToLineColumnUTF16(10)
	if line != 0 || col != 0 {
		t.Errorf("Empty source out of bounds: got (%d, %d), want (0, 0)", line, col)
	}
}

func TestByteOffsetToLineColumnUTF16Clamp(t *testing.T) {
	source := "abc"
	idx := NewLineIndex(source)

	// Offset beyond source length
	line, col := idx.ByteOffsetToLineColumnUTF16(100)
	if line != 0 || col != 3 {
		t.Errorf("Out of bounds: got (%d, %d), want (0, 3)", line, col)
	}
}

func TestUTF8ToUTF16ColumnInvalidUTF8(t *testing.T) {
	// Invalid UTF-8 sequence should be handled gracefully
	// \xff is not valid UTF-8
	s := "a\xffb"
	col := utf8ToUTF16Column(s, 2)
	// 'a' = 1, invalid byte = 1, so offset 2 should give col 2
	if col != 2 {
		t.Errorf("Invalid UTF-8: col = %d, want 2", col)
	}
}

func TestUTF8ToUTF16ColumnBoundaries(t *testing.T) {
	s := "abc"

	// Zero offset
	col := utf8ToUTF16Column(s, 0)
	if col != 0 {
		t.Errorf("Zero offset: col = %d, want 0", col)
	}

	// Negative offset
	col = utf8ToUTF16Column(s, -1)
	if col != 0 {
		t.Errorf("Negative offset: col = %d, want 0", col)
	}

	// Offset beyond string
	col = utf8ToUTF16Column(s, 100)
	if col != 3 {
		t.Errorf("Beyond string: col = %d, want 3", col)
	}
}

// Test edge cases for ByteOffsetToLineColumn with empty source
func TestByteOffsetToLineColumnEmptySourcePositiveOffset(t *testing.T) {
	idx := NewLineIndex("")
	// Test with positive offset on empty source
	line, col := idx.ByteOffsetToLineColumn(10)
	if line != 0 || col != 0 {
		t.Errorf("Empty source offset 10: got (%d, %d), want (0, 0)", line, col)
	}
}

// Test edge cases for ByteOffsetToLineColumnUTF16 with empty source
func TestByteOffsetToLineColumnUTF16EmptySourcePositiveOffset(t *testing.T) {
	idx := NewLineIndex("")
	// Test with positive offset on empty source
	line, col := idx.ByteOffsetToLineColumnUTF16(10)
	if line != 0 || col != 0 {
		t.Errorf("Empty source offset 10: got (%d, %d), want (0, 0)", line, col)
	}
}
