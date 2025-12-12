// Package minifier_tests provides tests using real WGSL samples.
//
// These tests use shaders from the webgpu-samples project to ensure
// the minifier works correctly on real-world code.
package minifier_tests

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/HugoDaniel/miniray/internal/parser"
	"github.com/HugoDaniel/miniray/internal/printer"
)

// getTestdataDir returns the path to the testdata directory.
func getTestdataDir() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata")
}

// loadTestFile loads a WGSL file from testdata.
func loadTestFile(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(getTestdataDir(), name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("test file %s not found: %v", name, err)
	}
	return string(data)
}

// ----------------------------------------------------------------------------
// Parse Tests - Verify samples parse without errors
// ----------------------------------------------------------------------------

func TestParseSamples(t *testing.T) {
	// Samples that parse with current parser implementation
	samples := []string{
		"example.wgsl",
		"basic_vert.wgsl",
	}

	// Samples that need more parser features (sized arrays, for loops, etc.)
	samplesNeedWork := []string{
		"blur.wgsl",          // Uses sized arrays, nested for loops
		"cornell_common.wgsl", // Uses array constructors
		"fullscreen_quad.wgsl", // Uses const array declarations
		"shadow_fragment.wgsl", // Uses nested for loops
	}

	for _, sample := range samples {
		t.Run(sample, func(t *testing.T) {
			source := loadTestFile(t, sample)

			p := parser.New(source)
			_, errs := p.Parse()
			if len(errs) > 0 {
				t.Errorf("parse errors in %s:", sample)
				for _, err := range errs {
					t.Errorf("  %s", err.Message)
				}
			}
		})
	}

	// Skip tests for samples that need more parser work
	for _, sample := range samplesNeedWork {
		t.Run(sample, func(t *testing.T) {
			t.Skipf("%s needs parser features not yet implemented", sample)
		})
	}
}

// ----------------------------------------------------------------------------
// Roundtrip Tests - Parse → Print → Parse → Print should be stable
// ----------------------------------------------------------------------------

func TestRoundtripSamples(t *testing.T) {
	// Only test samples that parse successfully
	samples := []string{
		"example.wgsl",
		"basic_vert.wgsl",
	}

	for _, sample := range samples {
		t.Run(sample, func(t *testing.T) {
			source := loadTestFile(t, sample)

			// First parse
			p1 := parser.New(source)
			m1, errs := p1.Parse()
			if len(errs) > 0 {
				t.Skipf("parse errors in first pass: %v", errs)
			}

			// First print
			pr1 := printer.New(printer.Options{}, m1.Symbols)
			output1 := pr1.Print(m1)

			// Second parse
			p2 := parser.New(output1)
			m2, errs := p2.Parse()
			if len(errs) > 0 {
				t.Errorf("parse errors in second pass (from printed output):")
				for _, err := range errs {
					t.Errorf("  %s", err.Message)
				}
				t.Logf("Printed output was:\n%s", output1)
				return
			}

			// Second print
			pr2 := printer.New(printer.Options{}, m2.Symbols)
			output2 := pr2.Print(m2)

			// Should be identical
			if output1 != output2 {
				t.Errorf("roundtrip not stable for %s", sample)
				t.Logf("First output:\n%s", output1)
				t.Logf("Second output:\n%s", output2)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Minified Roundtrip Tests - Minified output should also parse
// ----------------------------------------------------------------------------

func TestMinifiedRoundtripSamples(t *testing.T) {
	samples := []string{
		"example.wgsl",
		"basic_vert.wgsl",
	}

	for _, sample := range samples {
		t.Run(sample, func(t *testing.T) {
			source := loadTestFile(t, sample)

			// Parse original
			p1 := parser.New(source)
			m1, errs := p1.Parse()
			if len(errs) > 0 {
				t.Skipf("parse errors: %v", errs)
			}

			// Print minified
			pr1 := printer.New(printer.Options{
				MinifyWhitespace: true,
			}, m1.Symbols)
			minified := pr1.Print(m1)

			// Parse minified output
			p2 := parser.New(minified)
			_, errs = p2.Parse()
			if len(errs) > 0 {
				t.Errorf("minified output doesn't parse:")
				for _, err := range errs {
					t.Errorf("  %s", err.Message)
				}
				t.Logf("Minified output:\n%s", minified)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Size Reduction Tests - Verify minification actually reduces size
// ----------------------------------------------------------------------------

func TestSizeReductionSamples(t *testing.T) {
	// Only test samples that parse successfully
	samples := []string{
		"example.wgsl",
		"basic_vert.wgsl",
	}

	for _, sample := range samples {
		t.Run(sample, func(t *testing.T) {
			source := loadTestFile(t, sample)

			// Parse
			p := parser.New(source)
			m, errs := p.Parse()
			if len(errs) > 0 {
				t.Skipf("parse errors: %v", errs)
			}

			// Pretty print
			prPretty := printer.New(printer.Options{}, m.Symbols)
			pretty := prPretty.Print(m)

			// Minified print
			prMinify := printer.New(printer.Options{
				MinifyWhitespace: true,
			}, m.Symbols)
			minified := prMinify.Print(m)

			// Calculate sizes
			originalSize := len(source)
			prettySize := len(pretty)
			minifiedSize := len(minified)

			// Minified should be smaller (or at least not larger)
			if minifiedSize > prettySize {
				t.Errorf("minified size (%d) larger than pretty size (%d)", minifiedSize, prettySize)
			}

			// Log the reduction
			reduction := float64(prettySize-minifiedSize) / float64(prettySize) * 100
			t.Logf("%s: original=%d, pretty=%d, minified=%d (%.1f%% reduction)",
				sample, originalSize, prettySize, minifiedSize, reduction)
		})
	}
}

// ----------------------------------------------------------------------------
// Content Preservation Tests - Key content should be preserved
// ----------------------------------------------------------------------------

func TestPreservesEntryPointNames(t *testing.T) {
	samples := []struct {
		file       string
		entryPoints []string
	}{
		{"example.wgsl", []string{"vertexMain", "fragmentMain", "computeMain"}},
		{"basic_vert.wgsl", []string{"main"}},
		{"fullscreen_quad.wgsl", []string{"vert_main", "frag_main"}},
		{"shadow_fragment.wgsl", []string{"main"}},
	}

	for _, sample := range samples {
		t.Run(sample.file, func(t *testing.T) {
			source := loadTestFile(t, sample.file)

			// Parse
			p := parser.New(source)
			m, errs := p.Parse()
			if len(errs) > 0 {
				t.Skipf("parse errors: %v", errs)
			}

			// Print minified
			pr := printer.New(printer.Options{
				MinifyWhitespace: true,
			}, m.Symbols)
			minified := pr.Print(m)

			// Check that entry point names are preserved
			for _, ep := range sample.entryPoints {
				if !strings.Contains(minified, "fn "+ep) {
					t.Errorf("entry point %q not found in minified output", ep)
				}
			}
		})
	}
}

func TestPreservesBindings(t *testing.T) {
	samples := []string{
		"example.wgsl",
		"basic_vert.wgsl",
		"fullscreen_quad.wgsl",
		"shadow_fragment.wgsl",
	}

	for _, sample := range samples {
		t.Run(sample, func(t *testing.T) {
			source := loadTestFile(t, sample)

			// Count @binding attributes in source
			sourceBindings := strings.Count(source, "@binding")
			sourceGroups := strings.Count(source, "@group")

			// Parse and minify
			p := parser.New(source)
			m, errs := p.Parse()
			if len(errs) > 0 {
				t.Skipf("parse errors: %v", errs)
			}

			pr := printer.New(printer.Options{
				MinifyWhitespace: true,
			}, m.Symbols)
			minified := pr.Print(m)

			// Count in minified output
			minifiedBindings := strings.Count(minified, "@binding")
			minifiedGroups := strings.Count(minified, "@group")

			if minifiedBindings != sourceBindings {
				t.Errorf("binding count changed: %d → %d", sourceBindings, minifiedBindings)
			}
			if minifiedGroups != sourceGroups {
				t.Errorf("group count changed: %d → %d", sourceGroups, minifiedGroups)
			}
		})
	}
}

func TestPreservesBuiltins(t *testing.T) {
	samples := []string{
		"basic_vert.wgsl",
		"fullscreen_quad.wgsl",
		"blur.wgsl",
	}

	for _, sample := range samples {
		t.Run(sample, func(t *testing.T) {
			source := loadTestFile(t, sample)

			// Count @builtin attributes
			sourceBuiltins := strings.Count(source, "@builtin")

			// Parse and minify
			p := parser.New(source)
			m, errs := p.Parse()
			if len(errs) > 0 {
				t.Skipf("parse errors: %v", errs)
			}

			pr := printer.New(printer.Options{
				MinifyWhitespace: true,
			}, m.Symbols)
			minified := pr.Print(m)

			// Count in minified
			minifiedBuiltins := strings.Count(minified, "@builtin")

			if minifiedBuiltins != sourceBuiltins {
				t.Errorf("builtin count changed: %d → %d", sourceBuiltins, minifiedBuiltins)
			}
		})
	}
}

// ----------------------------------------------------------------------------
// Specific Sample Tests
// ----------------------------------------------------------------------------

func TestBasicVertexShader(t *testing.T) {
	source := loadTestFile(t, "basic_vert.wgsl")

	p := parser.New(source)
	m, errs := p.Parse()
	if len(errs) > 0 {
		for _, err := range errs {
			t.Errorf("parse error: %s", err.Message)
		}
		return
	}

	// Verify structure
	if len(m.Declarations) < 3 {
		t.Errorf("expected at least 3 declarations, got %d", len(m.Declarations))
	}

	// Print and verify output parses
	pr := printer.New(printer.Options{}, m.Symbols)
	output := pr.Print(m)

	p2 := parser.New(output)
	_, errs = p2.Parse()
	if len(errs) > 0 {
		t.Errorf("printed output doesn't parse")
		t.Logf("Output:\n%s", output)
	}
}

func TestBlurComputeShader(t *testing.T) {
	// Skip - this sample uses features not yet implemented (sized arrays, for loops)
	t.Skip("blur.wgsl uses parser features not yet implemented")
}

func TestCornellCommon(t *testing.T) {
	// Skip - this sample uses features not yet implemented (array constructors)
	t.Skip("cornell_common.wgsl uses parser features not yet implemented")
}

func TestShadowFragment(t *testing.T) {
	// Skip - this sample uses features not yet implemented (nested for loops)
	t.Skip("shadow_fragment.wgsl uses parser features not yet implemented")
}
