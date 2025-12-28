package sourcemap

import (
	"encoding/base64"
	"encoding/json"
	"strings"
)

// SourceMap represents a Source Map v3.
// See https://sourcemaps.info/spec.html
type SourceMap struct {
	Version        int      `json:"version"`
	File           string   `json:"file,omitempty"`
	SourceRoot     string   `json:"sourceRoot,omitempty"`
	Sources        []string `json:"sources"`
	SourcesContent []string `json:"sourcesContent,omitempty"`
	Names          []string `json:"names"`
	Mappings       string   `json:"mappings"`
}

// Mapping represents a decoded source map mapping.
type Mapping struct {
	GenLine   int  // Generated line (0-indexed)
	GenCol    int  // Generated column (0-indexed)
	SrcIndex  int  // Source file index
	SrcLine   int  // Source line (0-indexed)
	SrcCol    int  // Source column (0-indexed)
	NameIndex int  // Name index (-1 if no name)
	HasName   bool // Whether this mapping has a name
}

// Generator builds a source map incrementally.
type Generator struct {
	source        string
	lineIndex     *LineIndex
	mappings      []Mapping
	names         map[string]int // name -> index
	namesList     []string
	file          string
	sourceName    string
	includeSource bool

	// Current generated line for tracking
	currentGenLine int

	// coverLinesWithoutMappings adds a mapping at column 0 for lines that
	// would otherwise have no mappings. This is a workaround for a bug in
	// Mozilla's source-map library that returns null for lines without a
	// mapping at column 0.
	coverLinesWithoutMappings bool
}

// NewGenerator creates a new source map generator for the given original source.
func NewGenerator(source string) *Generator {
	return &Generator{
		source:                    source,
		lineIndex:                 NewLineIndex(source),
		mappings:                  make([]Mapping, 0),
		names:                     make(map[string]int),
		namesList:                 make([]string, 0),
		coverLinesWithoutMappings: true, // Enable by default for Mozilla compatibility
	}
}

// SetFile sets the generated file name.
func (g *Generator) SetFile(file string) {
	g.file = file
}

// SetSourceName sets the original source file name.
func (g *Generator) SetSourceName(name string) {
	g.sourceName = name
}

// IncludeSourceContent sets whether to include original source in sourcesContent.
func (g *Generator) IncludeSourceContent(include bool) {
	g.includeSource = include
}

// SetCoverLinesWithoutMappings enables or disables the line coverage workaround.
// When enabled (default), a mapping at column 0 is added for any line that would
// otherwise have no mappings. This works around a bug in Mozilla's source-map
// library that returns null for lines without a column 0 mapping.
func (g *Generator) SetCoverLinesWithoutMappings(cover bool) {
	g.coverLinesWithoutMappings = cover
}

// AddMapping adds a mapping from generated position to source position.
// genLine and genCol are 0-indexed positions in the generated output.
// srcOffset is the byte offset in the original source.
// name is the original name (empty string if no name mapping needed).
func (g *Generator) AddMapping(genLine, genCol, srcOffset int, name string) {
	srcLine, srcCol := g.lineIndex.ByteOffsetToLineColumnUTF16(srcOffset)

	m := Mapping{
		GenLine:   genLine,
		GenCol:    genCol,
		SrcIndex:  0, // Single source file
		SrcLine:   srcLine,
		SrcCol:    srcCol,
		NameIndex: -1,
		HasName:   false,
	}

	if name != "" {
		idx, ok := g.names[name]
		if !ok {
			idx = len(g.namesList)
			g.names[name] = idx
			g.namesList = append(g.namesList, name)
		}
		m.NameIndex = idx
		m.HasName = true
	}

	g.mappings = append(g.mappings, m)
}

// Generate produces the final SourceMap.
func (g *Generator) Generate() *SourceMap {
	sm := &SourceMap{
		Version:  3,
		File:     g.file,
		Sources:  []string{g.sourceName},
		Names:    g.namesList,
		Mappings: g.encodeMappings(),
	}

	if g.sourceName == "" {
		sm.Sources = []string{}
	}

	if g.includeSource && g.source != "" {
		sm.SourcesContent = []string{g.source}
	}

	return sm
}

// encodeMappings encodes all mappings as VLQ.
func (g *Generator) encodeMappings() string {
	if len(g.mappings) == 0 {
		return ""
	}

	var buf strings.Builder

	// State for delta encoding
	prevGenCol := 0
	prevSrcIndex := 0
	prevSrcLine := 0
	prevSrcCol := 0
	prevNameIndex := 0

	currentLine := 0
	firstOnLine := true

	// Track last mapping for line coverage workaround
	var lastMapping *Mapping

	for i := range g.mappings {
		m := &g.mappings[i]

		// Emit semicolons for skipped lines
		for currentLine < m.GenLine {
			buf.WriteByte(';')
			currentLine++
			prevGenCol = 0 // Reset column on new line
			firstOnLine = true

			// Line coverage workaround: if we're about to skip this line too,
			// add a mapping at column 0 so Mozilla's source-map library works
			if currentLine < m.GenLine && g.coverLinesWithoutMappings && lastMapping != nil {
				// Emit coverage mapping at column 0, pointing to last known source location
				buf.WriteString(EncodeVLQ(0 - prevGenCol))
				prevGenCol = 0
				buf.WriteString(EncodeVLQ(lastMapping.SrcIndex - prevSrcIndex))
				prevSrcIndex = lastMapping.SrcIndex
				buf.WriteString(EncodeVLQ(lastMapping.SrcLine - prevSrcLine))
				prevSrcLine = lastMapping.SrcLine
				buf.WriteString(EncodeVLQ(lastMapping.SrcCol - prevSrcCol))
				prevSrcCol = lastMapping.SrcCol
				firstOnLine = false
			}
		}

		// Emit comma if not first on line
		if !firstOnLine {
			buf.WriteByte(',')
		}
		firstOnLine = false

		// Encode segment
		// Field 1: Generated column (delta from previous on this line)
		buf.WriteString(EncodeVLQ(m.GenCol - prevGenCol))
		prevGenCol = m.GenCol

		// Field 2: Source index (delta)
		buf.WriteString(EncodeVLQ(m.SrcIndex - prevSrcIndex))
		prevSrcIndex = m.SrcIndex

		// Field 3: Source line (delta)
		buf.WriteString(EncodeVLQ(m.SrcLine - prevSrcLine))
		prevSrcLine = m.SrcLine

		// Field 4: Source column (delta)
		buf.WriteString(EncodeVLQ(m.SrcCol - prevSrcCol))
		prevSrcCol = m.SrcCol

		// Field 5: Name index (delta, only if has name)
		if m.HasName {
			buf.WriteString(EncodeVLQ(m.NameIndex - prevNameIndex))
			prevNameIndex = m.NameIndex
		}

		lastMapping = m
	}

	return buf.String()
}

// ToJSON returns the source map as a JSON string.
func (sm *SourceMap) ToJSON() string {
	data, _ := json.Marshal(sm)
	return string(data)
}

// ToDataURI returns the source map as a data URI for inline embedding.
func (sm *SourceMap) ToDataURI() string {
	jsonData := sm.ToJSON()
	encoded := base64.StdEncoding.EncodeToString([]byte(jsonData))
	return "data:application/json;base64," + encoded
}

// ToComment returns a source map comment for appending to generated code.
// For WGSL, we use JavaScript-style comments as WGSL doesn't have a standard.
func (sm *SourceMap) ToComment(inline bool) string {
	if inline {
		return "//# sourceMappingURL=" + sm.ToDataURI()
	}
	return "//# sourceMappingURL=" + sm.File + ".map"
}

// DecodeMappings decodes a VLQ-encoded mappings string.
func DecodeMappings(mappings string) ([]Mapping, error) {
	if mappings == "" {
		return nil, nil
	}

	result := make([]Mapping, 0)

	// State for delta decoding
	genCol := 0
	srcIndex := 0
	srcLine := 0
	srcCol := 0
	nameIndex := 0

	genLine := 0

	lines := strings.Split(mappings, ";")
	for lineIdx, line := range lines {
		genLine = lineIdx
		genCol = 0 // Reset column for each line

		if line == "" {
			continue
		}

		segments := strings.Split(line, ",")
		for _, segment := range segments {
			if segment == "" {
				continue
			}

			// Decode VLQ values
			pos := 0
			values := make([]int, 0, 5)

			for pos < len(segment) {
				value, consumed := DecodeVLQ(segment[pos:])
				if consumed == 0 {
					break
				}
				values = append(values, value)
				pos += consumed
			}

			if len(values) < 1 {
				continue
			}

			// Apply deltas
			genCol += values[0]

			m := Mapping{
				GenLine:   genLine,
				GenCol:    genCol,
				NameIndex: -1,
				HasName:   false,
			}

			if len(values) >= 4 {
				srcIndex += values[1]
				srcLine += values[2]
				srcCol += values[3]

				m.SrcIndex = srcIndex
				m.SrcLine = srcLine
				m.SrcCol = srcCol
			}

			if len(values) >= 5 {
				nameIndex += values[4]
				m.NameIndex = nameIndex
				m.HasName = true
			}

			result = append(result, m)
		}
	}

	return result, nil
}
