// Package renamer provides symbol renaming for minification.
//
// Following esbuild's approach, the renamer:
// - Assigns short names to frequently-used symbols
// - Reuses names across non-overlapping scopes
// - Avoids reserved words and API-facing names
// - Uses frequency analysis for optimal gzip compression
package renamer

import (
	"sort"
	"sync/atomic"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/lexer"
)

// ----------------------------------------------------------------------------
// Renamer Interface
// ----------------------------------------------------------------------------

// Renamer provides minified names for symbols.
type Renamer interface {
	NameForSymbol(ref ast.Ref) string
}

// ----------------------------------------------------------------------------
// NoOp Renamer
// ----------------------------------------------------------------------------

// NoOpRenamer returns original symbol names (no minification).
type NoOpRenamer struct {
	symbols []ast.Symbol
}

// NewNoOpRenamer creates a renamer that returns original names.
func NewNoOpRenamer(symbols []ast.Symbol) *NoOpRenamer {
	return &NoOpRenamer{symbols: symbols}
}

// NameForSymbol returns the original name.
func (r *NoOpRenamer) NameForSymbol(ref ast.Ref) string {
	if ref.IsValid() && int(ref.InnerIndex) < len(r.symbols) {
		return r.symbols[ref.InnerIndex].OriginalName
	}
	return ""
}

// ----------------------------------------------------------------------------
// Minify Renamer
// ----------------------------------------------------------------------------

// MinifyRenamer assigns short names based on usage frequency.
type MinifyRenamer struct {
	symbols       []ast.Symbol
	reservedNames map[string]bool
	slots         []symbolSlot
	nameMinifier  *NameMinifier

	// Mapping from top-level symbols to slots
	topLevelSlots map[ast.Ref]uint32
}

type symbolSlot struct {
	name  string
	count uint32
}

// NewMinifyRenamer creates a new minifying renamer.
func NewMinifyRenamer(symbols []ast.Symbol, reservedNames map[string]bool) *MinifyRenamer {
	r := &MinifyRenamer{
		symbols:       symbols,
		reservedNames: reservedNames,
		topLevelSlots: make(map[ast.Ref]uint32),
		nameMinifier:  DefaultNameMinifier(),
	}
	return r
}

// AccumulateSymbolUseCounts counts symbol usage for frequency analysis.
// This can be called in parallel for different parts of the AST.
func (r *MinifyRenamer) AccumulateSymbolUseCounts(uses map[ast.Ref]uint32) {
	for ref, count := range uses {
		if !ref.IsValid() {
			continue
		}
		idx := ref.InnerIndex
		if int(idx) >= len(r.symbols) {
			continue
		}

		symbol := &r.symbols[idx]

		// Skip symbols that must not be renamed
		if symbol.Flags.Has(ast.MustNotBeRenamed) {
			continue
		}

		// Accumulate count (thread-safe)
		atomic.AddUint32(&symbol.UseCount, count)
	}
}

// AllocateSlots allocates name slots based on accumulated usage counts.
func (r *MinifyRenamer) AllocateSlots() {
	// Collect renameable symbols sorted by usage
	type symbolWithCount struct {
		ref   ast.Ref
		count uint32
	}

	var renameable []symbolWithCount
	for i := range r.symbols {
		sym := &r.symbols[i]
		if sym.Flags.Has(ast.MustNotBeRenamed) {
			continue
		}
		if sym.UseCount > 0 {
			renameable = append(renameable, symbolWithCount{
				ref:   ast.Ref{InnerIndex: uint32(i)},
				count: sym.UseCount,
			})
		}
	}

	// Sort by count descending (most used first)
	sort.Slice(renameable, func(i, j int) bool {
		return renameable[i].count > renameable[j].count
	})

	// Allocate slots
	r.slots = make([]symbolSlot, len(renameable))
	for i, item := range renameable {
		r.topLevelSlots[item.ref] = uint32(i)
		r.slots[i] = symbolSlot{count: item.count}
	}
}

// AssignNames assigns minified names to slots.
func (r *MinifyRenamer) AssignNames() {
	nameIndex := 0
	for i := range r.slots {
		// Generate name, skipping reserved ones
		name := r.nameMinifier.NumberToMinifiedName(nameIndex)
		for r.reservedNames[name] {
			nameIndex++
			name = r.nameMinifier.NumberToMinifiedName(nameIndex)
		}
		r.slots[i].name = name
		nameIndex++
	}
}

// NameForSymbol returns the minified name for a symbol.
func (r *MinifyRenamer) NameForSymbol(ref ast.Ref) string {
	if !ref.IsValid() {
		return ""
	}

	idx := ref.InnerIndex
	if int(idx) >= len(r.symbols) {
		return ""
	}

	symbol := &r.symbols[idx]

	// Return original name if not renameable
	if symbol.Flags.Has(ast.MustNotBeRenamed) {
		return symbol.OriginalName
	}

	// Look up slot
	if slotIdx, ok := r.topLevelSlots[ref]; ok {
		return r.slots[slotIdx].name
	}

	// Fallback to original
	return symbol.OriginalName
}

// ----------------------------------------------------------------------------
// Name Generation
// ----------------------------------------------------------------------------

// NameMinifier generates minified identifier names.
type NameMinifier struct {
	// Characters allowed as first character of identifier
	head string
	// Characters allowed in rest of identifier
	tail string
}

// DefaultNameMinifier creates a minifier for WGSL identifiers.
func DefaultNameMinifier() *NameMinifier {
	// WGSL identifiers: XID_Start for first char, XID_Continue for rest
	// For simplicity, use ASCII subset which is always valid
	return &NameMinifier{
		head: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ",
		tail: "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789",
	}
}

// NumberToMinifiedName converts a number to a minified identifier.
// The sequence is: a, b, ..., z, A, ..., Z, aa, ba, ca, ...
func (m *NameMinifier) NumberToMinifiedName(n int) string {
	nHead := len(m.head)
	nTail := len(m.tail)

	// First character from head alphabet
	result := make([]byte, 0, 4)
	result = append(result, m.head[n%nHead])
	n = n / nHead

	// Subsequent characters from tail alphabet
	for n > 0 {
		n--
		result = append(result, m.tail[n%nTail])
		n = n / nTail
	}

	return string(result)
}

// ShuffleByCharFreq reorders the alphabet based on character frequency
// in the source code. This improves gzip compression.
func (m *NameMinifier) ShuffleByCharFreq(freq CharFreq) *NameMinifier {
	type charCount struct {
		char  byte
		count int32
	}

	// Build sortable list
	chars := make([]charCount, len(m.tail))
	for i := 0; i < len(m.tail); i++ {
		chars[i] = charCount{char: m.tail[i], count: freq[i]}
	}

	// Sort by frequency descending
	sort.Slice(chars, func(i, j int) bool {
		return chars[i].count > chars[j].count
	})

	// Build new alphabets
	var newHead, newTail []byte
	for _, c := range chars {
		newTail = append(newTail, c.char)
		// Head excludes digits
		if c.char < '0' || c.char > '9' {
			newHead = append(newHead, c.char)
		}
	}

	return &NameMinifier{
		head: string(newHead),
		tail: string(newTail),
	}
}

// ----------------------------------------------------------------------------
// Character Frequency Analysis
// ----------------------------------------------------------------------------

// CharFreq is a histogram of character frequencies.
type CharFreq [64]int32

// Scan accumulates character frequencies from text.
func (freq *CharFreq) Scan(text string, delta int32) {
	for i := 0; i < len(text); i++ {
		c := text[i]
		var idx int
		switch {
		case c >= 'a' && c <= 'z':
			idx = int(c - 'a')
		case c >= 'A' && c <= 'Z':
			idx = int(c - 'A' + 26)
		case c >= '0' && c <= '9':
			idx = int(c - '0' + 52)
		case c == '_':
			idx = 62
		default:
			continue
		}
		freq[idx] += delta
	}
}

// ----------------------------------------------------------------------------
// Reserved Names
// ----------------------------------------------------------------------------

// ComputeReservedNames builds a set of names that cannot be used.
func ComputeReservedNames() map[string]bool {
	reserved := make(map[string]bool)

	// WGSL keywords
	for kw := range lexer.Keywords {
		reserved[kw] = true
	}

	// WGSL reserved words
	for word := range lexer.ReservedWords {
		reserved[word] = true
	}

	// Single underscore is invalid
	reserved["_"] = true

	// Built-in type names
	builtinTypes := []string{
		"bool", "i32", "u32", "f32", "f16",
		"vec2", "vec3", "vec4",
		"vec2i", "vec3i", "vec4i",
		"vec2u", "vec3u", "vec4u",
		"vec2f", "vec3f", "vec4f",
		"vec2h", "vec3h", "vec4h",
		"mat2x2", "mat2x3", "mat2x4",
		"mat3x2", "mat3x3", "mat3x4",
		"mat4x2", "mat4x3", "mat4x4",
		"mat2x2f", "mat2x3f", "mat2x4f",
		"mat3x2f", "mat3x3f", "mat3x4f",
		"mat4x2f", "mat4x3f", "mat4x4f",
		"mat2x2h", "mat2x3h", "mat2x4h",
		"mat3x2h", "mat3x3h", "mat3x4h",
		"mat4x2h", "mat4x3h", "mat4x4h",
		"array", "ptr", "atomic",
		"sampler", "sampler_comparison",
		"texture_1d", "texture_2d", "texture_2d_array",
		"texture_3d", "texture_cube", "texture_cube_array",
		"texture_multisampled_2d",
		"texture_storage_1d", "texture_storage_2d", "texture_storage_2d_array", "texture_storage_3d",
		"texture_depth_2d", "texture_depth_2d_array", "texture_depth_cube", "texture_depth_cube_array",
		"texture_depth_multisampled_2d",
		"texture_external",
	}
	for _, t := range builtinTypes {
		reserved[t] = true
	}

	// Address spaces
	addressSpaces := []string{"function", "private", "workgroup", "uniform", "storage"}
	for _, s := range addressSpaces {
		reserved[s] = true
	}

	// Access modes
	accessModes := []string{"read", "write", "read_write"}
	for _, m := range accessModes {
		reserved[m] = true
	}

	// Texel formats
	texelFormats := []string{
		"rgba8unorm", "rgba8snorm", "rgba8uint", "rgba8sint",
		"rgba16uint", "rgba16sint", "rgba16float",
		"r32uint", "r32sint", "r32float",
		"rg32uint", "rg32sint", "rg32float",
		"rgba32uint", "rgba32sint", "rgba32float",
		"bgra8unorm",
	}
	for _, f := range texelFormats {
		reserved[f] = true
	}

	return reserved
}
