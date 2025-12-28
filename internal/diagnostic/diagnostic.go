// Package diagnostic provides error reporting and diagnostic messages for WGSL validation.
//
// The diagnostic system is compatible with WebGPU Dawn Tint compiler's error reporting,
// providing accurate source locations, severity levels, and WGSL spec references.
package diagnostic

import (
	"fmt"
	"strings"

	"github.com/HugoDaniel/miniray/internal/sourcemap"
)

// Severity represents the severity level of a diagnostic.
type Severity uint8

const (
	// Error prevents shader compilation.
	Error Severity = iota
	// Warning is a non-blocking issue.
	Warning
	// Info is an informational message.
	Info
	// Note provides additional context for another diagnostic.
	Note
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	case Info:
		return "info"
	case Note:
		return "note"
	default:
		return "unknown"
	}
}

// Position represents a position in source code.
type Position struct {
	Offset int // Byte offset (0-based)
	Line   int // Line number (1-based)
	Column int // Column number (1-based)
}

// Range represents a range in source code.
type Range struct {
	Start Position
	End   Position
}

// RelatedInfo provides additional location information for a diagnostic.
type RelatedInfo struct {
	Range   Range
	Message string
}

// Diagnostic represents a single diagnostic message.
type Diagnostic struct {
	Severity Severity
	Code     string        // Error code (e.g., "E0001", "type-mismatch")
	Message  string        // Human-readable message
	Range    Range         // Source location
	Related  []RelatedInfo // Related locations
	SpecRef  string        // WGSL spec section reference (e.g., "6.4")
}

// Error returns a formatted error string.
func (d *Diagnostic) Error() string {
	return fmt.Sprintf("%d:%d: %s: %s", d.Range.Start.Line, d.Range.Start.Column, d.Severity, d.Message)
}

// DiagnosticList collects diagnostics during compilation.
type DiagnosticList struct {
	diagnostics []Diagnostic
	lineIndex   *sourcemap.LineIndex
	source      string
	hasErrors   bool
}

// NewDiagnosticList creates a new diagnostic list for the given source.
func NewDiagnosticList(source string) *DiagnosticList {
	return &DiagnosticList{
		diagnostics: make([]Diagnostic, 0),
		lineIndex:   sourcemap.NewLineIndex(source),
		source:      source,
	}
}

// Add adds a diagnostic to the list.
func (dl *DiagnosticList) Add(d Diagnostic) {
	dl.diagnostics = append(dl.diagnostics, d)
	if d.Severity == Error {
		dl.hasErrors = true
	}
}

// AddError adds an error diagnostic at the given byte offset.
func (dl *DiagnosticList) AddError(offset int, message string) {
	dl.AddErrorRange(offset, offset+1, message)
}

// AddErrorRange adds an error diagnostic for a byte range.
func (dl *DiagnosticList) AddErrorRange(start, end int, message string) {
	dl.Add(Diagnostic{
		Severity: Error,
		Message:  message,
		Range:    dl.MakeRange(start, end),
	})
}

// AddErrorWithCode adds an error diagnostic with an error code.
func (dl *DiagnosticList) AddErrorWithCode(offset int, code, message string) {
	dl.Add(Diagnostic{
		Severity: Error,
		Code:     code,
		Message:  message,
		Range:    dl.MakeRange(offset, offset+1),
	})
}

// AddWarning adds a warning diagnostic at the given byte offset.
func (dl *DiagnosticList) AddWarning(offset int, message string) {
	dl.Add(Diagnostic{
		Severity: Warning,
		Message:  message,
		Range:    dl.MakeRange(offset, offset+1),
	})
}

// AddNote adds a note diagnostic at the given byte offset.
func (dl *DiagnosticList) AddNote(offset int, message string) {
	dl.Add(Diagnostic{
		Severity: Note,
		Message:  message,
		Range:    dl.MakeRange(offset, offset+1),
	})
}

// MakePosition converts a byte offset to a Position.
func (dl *DiagnosticList) MakePosition(offset int) Position {
	line, col := dl.lineIndex.ByteOffsetToLineColumn(offset)
	return Position{
		Offset: offset,
		Line:   line + 1, // Convert to 1-based
		Column: col + 1,  // Convert to 1-based
	}
}

// MakeRange converts byte offsets to a Range.
func (dl *DiagnosticList) MakeRange(start, end int) Range {
	return Range{
		Start: dl.MakePosition(start),
		End:   dl.MakePosition(end),
	}
}

// HasErrors returns true if there are any error-level diagnostics.
func (dl *DiagnosticList) HasErrors() bool {
	return dl.hasErrors
}

// Diagnostics returns all collected diagnostics.
func (dl *DiagnosticList) Diagnostics() []Diagnostic {
	return dl.diagnostics
}

// Errors returns only error-level diagnostics.
func (dl *DiagnosticList) Errors() []Diagnostic {
	var errors []Diagnostic
	for _, d := range dl.diagnostics {
		if d.Severity == Error {
			errors = append(errors, d)
		}
	}
	return errors
}

// Warnings returns only warning-level diagnostics.
func (dl *DiagnosticList) Warnings() []Diagnostic {
	var warnings []Diagnostic
	for _, d := range dl.diagnostics {
		if d.Severity == Warning {
			warnings = append(warnings, d)
		}
	}
	return warnings
}

// Count returns the total number of diagnostics.
func (dl *DiagnosticList) Count() int {
	return len(dl.diagnostics)
}

// ErrorCount returns the number of error-level diagnostics.
func (dl *DiagnosticList) ErrorCount() int {
	count := 0
	for _, d := range dl.diagnostics {
		if d.Severity == Error {
			count++
		}
	}
	return count
}

// Format formats all diagnostics as a human-readable string.
func (dl *DiagnosticList) Format() string {
	if len(dl.diagnostics) == 0 {
		return ""
	}

	var sb strings.Builder
	for _, d := range dl.diagnostics {
		sb.WriteString(dl.FormatDiagnostic(&d))
		sb.WriteByte('\n')
	}
	return sb.String()
}

// FormatDiagnostic formats a single diagnostic with source context.
func (dl *DiagnosticList) FormatDiagnostic(d *Diagnostic) string {
	var sb strings.Builder

	// Main error line
	sb.WriteString(fmt.Sprintf("%d:%d: %s: %s\n",
		d.Range.Start.Line, d.Range.Start.Column, d.Severity, d.Message))

	// Add spec reference if present
	if d.SpecRef != "" {
		sb.WriteString(fmt.Sprintf("  [WGSL spec section %s]\n", d.SpecRef))
	}

	// Add source context
	sourceLine := dl.getSourceLine(d.Range.Start.Line)
	if sourceLine != "" {
		sb.WriteString(fmt.Sprintf("    %s\n", sourceLine))
		// Add caret indicator
		caret := strings.Repeat(" ", d.Range.Start.Column-1+4) + "^"
		if d.Range.End.Line == d.Range.Start.Line && d.Range.End.Column > d.Range.Start.Column {
			caret += strings.Repeat("~", d.Range.End.Column-d.Range.Start.Column-1)
		}
		sb.WriteString(caret)
		sb.WriteByte('\n')
	}

	// Add related info
	for _, rel := range d.Related {
		sb.WriteString(fmt.Sprintf("  %d:%d: note: %s\n",
			rel.Range.Start.Line, rel.Range.Start.Column, rel.Message))
	}

	return sb.String()
}

// getSourceLine returns the source code line at the given 1-based line number.
func (dl *DiagnosticList) getSourceLine(line int) string {
	if line < 1 {
		return ""
	}
	lines := strings.Split(dl.source, "\n")
	if line > len(lines) {
		return ""
	}
	return strings.TrimRight(lines[line-1], "\r")
}

// Clear removes all diagnostics.
func (dl *DiagnosticList) Clear() {
	dl.diagnostics = dl.diagnostics[:0]
	dl.hasErrors = false
}

// DiagnosticCode defines standard error codes.
type DiagnosticCode string

const (
	// Syntax errors (E00xx)
	CodeUnexpectedToken    DiagnosticCode = "E0001"
	CodeUnterminatedString DiagnosticCode = "E0002"
	CodeInvalidNumber      DiagnosticCode = "E0003"

	// Symbol errors (E01xx)
	CodeUndefinedSymbol    DiagnosticCode = "E0100"
	CodeDuplicateSymbol    DiagnosticCode = "E0101"
	CodeUseBeforeDecl      DiagnosticCode = "E0102"
	CodeRecursiveFunction  DiagnosticCode = "E0103"
	CodeRecursiveType      DiagnosticCode = "E0104"

	// Type errors (E02xx)
	CodeTypeMismatch       DiagnosticCode = "E0200"
	CodeInvalidOperand     DiagnosticCode = "E0201"
	CodeInvalidArgCount    DiagnosticCode = "E0202"
	CodeInvalidArgType     DiagnosticCode = "E0203"
	CodeNotCallable        DiagnosticCode = "E0204"
	CodeNotIndexable       DiagnosticCode = "E0205"
	CodeNoSuchMember       DiagnosticCode = "E0206"
	CodeInvalidReturn      DiagnosticCode = "E0207"
	CodeMissingReturn      DiagnosticCode = "E0208"
	CodeInvalidConversion  DiagnosticCode = "E0209"
	CodeInvalidAssignment  DiagnosticCode = "E0210"

	// Declaration errors (E03xx)
	CodeMissingInitializer DiagnosticCode = "E0300"
	CodeInvalidInitializer DiagnosticCode = "E0301"
	CodeInvalidConstExpr   DiagnosticCode = "E0302"
	CodeInvalidOverride    DiagnosticCode = "E0303"
	CodeInvalidAddressSpace DiagnosticCode = "E0304"
	CodeInvalidAccessMode  DiagnosticCode = "E0305"

	// Attribute errors (E04xx)
	CodeInvalidAttribute   DiagnosticCode = "E0400"
	CodeDuplicateAttribute DiagnosticCode = "E0401"
	CodeMissingAttribute   DiagnosticCode = "E0402"
	CodeInvalidBuiltin     DiagnosticCode = "E0403"
	CodeInvalidLocation    DiagnosticCode = "E0404"

	// Control flow errors (E05xx)
	CodeBreakOutsideLoop    DiagnosticCode = "E0500"
	CodeContinueOutsideLoop DiagnosticCode = "E0501"
	CodeDiscardOutsideFragment DiagnosticCode = "E0502"
	CodeUnreachableCode     DiagnosticCode = "E0503"

	// Entry point errors (E06xx)
	CodeInvalidEntryPoint  DiagnosticCode = "E0600"
	CodeMissingEntryPoint  DiagnosticCode = "E0601"
	CodeInvalidShaderIO    DiagnosticCode = "E0602"

	// Uniformity errors (E07xx)
	CodeNonUniformDerivative DiagnosticCode = "E0700"
	CodeNonUniformBarrier    DiagnosticCode = "E0701"
	CodeNonUniformTexture    DiagnosticCode = "E0702"
	CodeNonUniformSubgroup   DiagnosticCode = "E0703"

	// Memory errors (E08xx)
	CodeInvalidWorkgroupVar DiagnosticCode = "E0800"
	CodeInvalidStorageVar   DiagnosticCode = "E0801"
	CodeInvalidUniformVar   DiagnosticCode = "E0802"
	CodeMissingBinding      DiagnosticCode = "E0803"
)

// DiagnosticFilter controls which diagnostics are reported.
type DiagnosticFilter struct {
	// Rules maps diagnostic rule names to their severity override.
	// A nil value means use default severity.
	// Special severity "off" disables the diagnostic.
	Rules map[string]Severity
}

// NewDiagnosticFilter creates a new filter with default settings.
func NewDiagnosticFilter() *DiagnosticFilter {
	return &DiagnosticFilter{
		Rules: make(map[string]Severity),
	}
}

// SetRule sets the severity for a diagnostic rule.
func (f *DiagnosticFilter) SetRule(rule string, severity Severity) {
	f.Rules[rule] = severity
}

// DisableRule disables a diagnostic rule.
func (f *DiagnosticFilter) DisableRule(rule string) {
	// Use a special sentinel value to indicate disabled
	f.Rules[rule] = Severity(255)
}

// IsDisabled returns true if the rule is disabled.
func (f *DiagnosticFilter) IsDisabled(rule string) bool {
	if sev, ok := f.Rules[rule]; ok {
		return sev == Severity(255)
	}
	return false
}

// GetSeverity returns the severity for a rule, or the default if not set.
func (f *DiagnosticFilter) GetSeverity(rule string, defaultSev Severity) Severity {
	if sev, ok := f.Rules[rule]; ok {
		if sev == Severity(255) {
			return defaultSev // Return default for disabled (caller should check IsDisabled first)
		}
		return sev
	}
	return defaultSev
}

// Standard diagnostic rules (from WGSL spec).
const (
	RuleDerivativeUniformity = "derivative_uniformity"
	RuleSubgroupUniformity   = "subgroup_uniformity"
)
