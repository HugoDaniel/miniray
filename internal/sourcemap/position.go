package sourcemap

import (
	"sort"
	"unicode/utf8"
)

// LineIndex provides efficient byte offset to line/column conversion.
// It pre-computes line start positions for O(log n) lookups.
type LineIndex struct {
	source     string
	lineStarts []int // byte offset of each line start
}

// NewLineIndex creates a LineIndex for the given source.
func NewLineIndex(source string) *LineIndex {
	idx := &LineIndex{
		source:     source,
		lineStarts: []int{0}, // First line starts at offset 0
	}

	// Scan for newlines
	for i := 0; i < len(source); i++ {
		c := source[i]
		if c == '\n' {
			// LF - next line starts after this (unless at end of source)
			nextLineStart := i + 1
			if nextLineStart < len(source) {
				idx.lineStarts = append(idx.lineStarts, nextLineStart)
			}
		} else if c == '\r' {
			// CR - check for CRLF
			if i+1 < len(source) && source[i+1] == '\n' {
				// CRLF - next line starts after both (unless at end)
				nextLineStart := i + 2
				if nextLineStart < len(source) {
					idx.lineStarts = append(idx.lineStarts, nextLineStart)
				}
				i++ // Skip the LF
			} else {
				// Standalone CR - next line starts after this (unless at end)
				nextLineStart := i + 1
				if nextLineStart < len(source) {
					idx.lineStarts = append(idx.lineStarts, nextLineStart)
				}
			}
		}
	}

	return idx
}

// LineCount returns the number of lines in the source.
func (idx *LineIndex) LineCount() int {
	return len(idx.lineStarts)
}

// ByteOffsetToLineColumn converts a byte offset to 0-indexed line and column.
// The column is in bytes (not UTF-16 code units).
func (idx *LineIndex) ByteOffsetToLineColumn(offset int) (line, col int) {
	if offset < 0 {
		return 0, 0
	}
	if offset >= len(idx.source) {
		// Clamp to end of source
		if len(idx.source) == 0 {
			return 0, 0
		}
		offset = len(idx.source)
	}

	// Binary search for the line containing this offset
	line = sort.Search(len(idx.lineStarts), func(i int) bool {
		return idx.lineStarts[i] > offset
	}) - 1

	if line < 0 {
		line = 0
	}

	col = offset - idx.lineStarts[line]
	return line, col
}

// ByteOffsetToLineColumnUTF16 converts a byte offset to 0-indexed line and column.
// The column is in UTF-16 code units, as required by the source map spec.
func (idx *LineIndex) ByteOffsetToLineColumnUTF16(offset int) (line, col int) {
	if offset < 0 {
		return 0, 0
	}
	if offset >= len(idx.source) {
		if len(idx.source) == 0 {
			return 0, 0
		}
		offset = len(idx.source)
	}

	// Find the line
	line = sort.Search(len(idx.lineStarts), func(i int) bool {
		return idx.lineStarts[i] > offset
	}) - 1

	if line < 0 {
		line = 0
	}

	// Calculate UTF-16 column
	lineStart := idx.lineStarts[line]
	col = utf8ToUTF16Column(idx.source[lineStart:], offset-lineStart)

	return line, col
}

// utf8ToUTF16Column converts a byte offset within a string to UTF-16 column.
// This handles the conversion from UTF-8 bytes to UTF-16 code units.
func utf8ToUTF16Column(s string, byteOffset int) int {
	if byteOffset <= 0 {
		return 0
	}
	if byteOffset >= len(s) {
		byteOffset = len(s)
	}

	col := 0
	for i := 0; i < byteOffset; {
		r, size := utf8.DecodeRuneInString(s[i:])
		if r == utf8.RuneError && size == 1 {
			// Invalid UTF-8, treat as single byte
			col++
			i++
			continue
		}

		// Count UTF-16 code units for this rune
		if r >= 0x10000 {
			// Supplementary plane - needs surrogate pair (2 UTF-16 code units)
			col += 2
		} else {
			// BMP character - 1 UTF-16 code unit
			col++
		}

		i += size
	}

	return col
}

// LineColumnToByteOffset converts a 0-indexed line and column to byte offset.
// The column is expected in bytes.
func (idx *LineIndex) LineColumnToByteOffset(line, col int) int {
	if line < 0 {
		line = 0
	}
	if line >= len(idx.lineStarts) {
		line = len(idx.lineStarts) - 1
	}

	offset := idx.lineStarts[line] + col

	// Clamp to source bounds
	if offset < 0 {
		return 0
	}
	if offset > len(idx.source) {
		return len(idx.source)
	}

	return offset
}
