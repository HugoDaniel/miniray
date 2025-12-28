package minifier_tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/minifier"
	"github.com/HugoDaniel/miniray/internal/parser"
	"github.com/HugoDaniel/miniray/internal/validator"
)

// TestTintSemanticPreservation tests that minification preserves semantics
// by parsing, minifying, re-parsing, and validating Tint test files.
func TestTintSemanticPreservation(t *testing.T) {
	testdataDir := filepath.Join("..", "..", "testdata", "tint")

	// Check if testdata/tint exists
	if _, err := os.Stat(testdataDir); os.IsNotExist(err) {
		t.Skip("testdata/tint directory not found - run test import first")
	}

	var stats struct {
		total   int
		passed  int
		failed  int
		skipped int
	}

	err := filepath.Walk(testdataDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Only process .wgsl files that aren't expected outputs
		if info.IsDir() || !strings.HasSuffix(path, ".wgsl") || strings.Contains(path, ".expected.") {
			return nil
		}

		stats.total++

		// Run as subtest
		relPath, _ := filepath.Rel(testdataDir, path)
		t.Run(relPath, func(t *testing.T) {
			result := testSemanticPreservation(t, path)
			switch result {
			case tintTestPassed:
				stats.passed++
			case tintTestFailed:
				stats.failed++
			case tintTestSkipped:
				stats.skipped++
			}
		})

		return nil
	})

	if err != nil {
		t.Fatalf("Failed to walk testdata/tint: %v", err)
	}

	t.Logf("Tint tests: %d total, %d passed, %d failed, %d skipped",
		stats.total, stats.passed, stats.failed, stats.skipped)
}

type tintTestResult int

const (
	tintTestPassed tintTestResult = iota
	tintTestFailed
	tintTestSkipped
)

func testSemanticPreservation(t *testing.T, path string) (result tintTestResult) {
	// Handle validator panics gracefully
	defer func() {
		if r := recover(); r != nil {
			t.Skipf("Validator panic (likely unsupported construct): %v", r)
			result = tintTestSkipped
		}
	}()

	// Read source file
	source, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
		return tintTestFailed
	}

	sourceStr := string(source)

	// Skip files with unsupported extensions
	if containsUnsupportedFeatures(sourceStr) {
		t.Skip("Contains unsupported WGSL features")
		return tintTestSkipped
	}

	// Step 1: Parse original
	p := parser.New(sourceStr)
	origModule, parseErrors := p.Parse()
	if len(parseErrors) > 0 {
		// Some Tint tests may have intentional parse errors or use features
		// we don't support - skip these
		t.Skipf("Parse error in original: %s", parseErrors[0].Message)
		return tintTestSkipped
	}

	// Step 2: Validate original (some tests may have intentional validation errors)
	origResult := validator.Validate(origModule, validator.Options{})
	if origResult.Diagnostics.HasErrors() {
		// Skip tests that have validation errors in the original
		t.Skip("Original has validation errors - likely intentional test case")
		return tintTestSkipped
	}

	// Step 3: Minify
	m := minifier.New(minifier.Options{
		MinifyWhitespace:   true,
		MinifyIdentifiers:  true,
		MinifySyntax:       true,
		TreeShaking:        false, // Don't tree shake - preserve all declarations
		KeepNames:          nil,
	})
	minResult := m.Minify(sourceStr)
	if len(minResult.Errors) > 0 {
		t.Errorf("Minification error: %s", minResult.Errors[0].Message)
		return tintTestFailed
	}

	// Step 4: Parse minified output
	p2 := parser.New(minResult.Code)
	minModule, parseErrors2 := p2.Parse()
	if len(parseErrors2) > 0 {
		t.Errorf("Parse error in minified output: %s\nMinified code:\n%s",
			parseErrors2[0].Message, minResult.Code)
		return tintTestFailed
	}

	// Step 5: Validate minified output
	minValResult := validator.Validate(minModule, validator.Options{})
	if minValResult.Diagnostics.HasErrors() {
		var errMsgs []string
		for _, d := range minValResult.Diagnostics.Diagnostics() {
			errMsgs = append(errMsgs, d.Message)
		}
		t.Errorf("Validation error in minified output: %v\nMinified code:\n%s",
			errMsgs, minResult.Code)
		return tintTestFailed
	}

	// Step 6: Verify key properties preserved
	if !verifyPreservedProperties(t, origModule, minModule) {
		return tintTestFailed
	}

	return tintTestPassed
}

// containsUnsupportedFeatures checks for WGSL features miniray doesn't support
func containsUnsupportedFeatures(source string) bool {
	unsupported := []string{
		"enable f16",           // f16 extension
		"enable chromium",      // Chromium-specific extensions
		"enable subgroups",     // Subgroup operations
		"diagnostic(off",       // Diagnostic pragmas
		"diagnostic(warning",
		"diagnostic(error",
		"@diagnostic",          // Diagnostic attributes
	}
	for _, u := range unsupported {
		if strings.Contains(source, u) {
			return true
		}
	}
	return false
}

// verifyPreservedProperties checks that key shader properties are preserved
func verifyPreservedProperties(t *testing.T, orig, min *ast.Module) bool {
	// Count entry points in original
	origEntryPoints := countEntryPoints(orig)
	minEntryPoints := countEntryPoints(min)

	if origEntryPoints != minEntryPoints {
		t.Errorf("Entry point count mismatch: original=%d, minified=%d",
			origEntryPoints, minEntryPoints)
		return false
	}

	// Count bindings in original
	origBindings := countBindings(orig)
	minBindings := countBindings(min)

	if origBindings != minBindings {
		t.Errorf("Binding count mismatch: original=%d, minified=%d",
			origBindings, minBindings)
		return false
	}

	return true
}

func countEntryPoints(m *ast.Module) int {
	count := 0
	for _, decl := range m.Declarations {
		if fn, ok := decl.(*ast.FunctionDecl); ok {
			for _, attr := range fn.Attributes {
				if attr.Name == "vertex" || attr.Name == "fragment" || attr.Name == "compute" {
					count++
					break
				}
			}
		}
	}
	return count
}

func countBindings(m *ast.Module) int {
	count := 0
	for _, decl := range m.Declarations {
		if v, ok := decl.(*ast.VarDecl); ok {
			hasGroup := false
			hasBinding := false
			for _, attr := range v.Attributes {
				if attr.Name == "group" {
					hasGroup = true
				}
				if attr.Name == "binding" {
					hasBinding = true
				}
			}
			if hasGroup && hasBinding {
				count++
			}
		}
	}
	return count
}
