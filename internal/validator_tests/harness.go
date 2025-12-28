// Package validator_tests provides test infrastructure for WGSL validation tests.
package validator_tests

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/HugoDaniel/miniray/pkg/api"
)

// TestCase represents a single validation test case.
type TestCase struct {
	Name     string
	FilePath string
	Source   string
	Expected ExpectedResult
	SpecRef  string
}

// ExpectedResult describes the expected validation outcome.
type ExpectedResult struct {
	Valid    bool
	Errors   []ExpectedDiagnostic
	Warnings []ExpectedDiagnostic
}

// ExpectedDiagnostic describes an expected diagnostic message.
type ExpectedDiagnostic struct {
	Code    string // e.g., "E0201"
	Pattern string // substring or regex pattern to match in message
	Line    int    // expected line number (0 = any)
}

// Annotation patterns for test files.
var (
	expectValidRe   = regexp.MustCompile(`//\s*@expect-valid`)
	expectErrorRe   = regexp.MustCompile(`//\s*@expect-error\s+(\w+)(?:\s+"([^"]*)")?`)
	expectWarningRe = regexp.MustCompile(`//\s*@expect-warning\s+(\w+)(?:\s+"([^"]*)")?`)
	specRefRe       = regexp.MustCompile(`//\s*@spec-ref:\s*(.+)`)
	testNameRe      = regexp.MustCompile(`//\s*@test:\s*(.+)`)
)

// ParseTestFile parses a WGSL test file and extracts annotations.
func ParseTestFile(path string) (*TestCase, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	source := string(content)
	tc := &TestCase{
		FilePath: path,
		Source:   source,
		Name:     filepath.Base(path),
	}

	// Parse annotations
	scanner := bufio.NewScanner(strings.NewReader(source))
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check for @test annotation
		if match := testNameRe.FindStringSubmatch(line); match != nil {
			tc.Name = strings.TrimSpace(match[1])
		}

		// Check for @expect-valid
		if expectValidRe.MatchString(line) {
			tc.Expected.Valid = true
		}

		// Check for @expect-error
		if match := expectErrorRe.FindStringSubmatch(line); match != nil {
			diag := ExpectedDiagnostic{
				Code: match[1],
			}
			if len(match) > 2 && match[2] != "" {
				diag.Pattern = match[2]
			}
			tc.Expected.Errors = append(tc.Expected.Errors, diag)
		}

		// Check for @expect-warning
		if match := expectWarningRe.FindStringSubmatch(line); match != nil {
			diag := ExpectedDiagnostic{
				Code: match[1],
			}
			if len(match) > 2 && match[2] != "" {
				diag.Pattern = match[2]
			}
			tc.Expected.Warnings = append(tc.Expected.Warnings, diag)
		}

		// Check for @spec-ref
		if match := specRefRe.FindStringSubmatch(line); match != nil {
			tc.Expected.Valid = true
			tc.SpecRef = strings.TrimSpace(match[1])
		}
	}

	// If no explicit expectation, default to valid if no @expect-error found
	if !tc.Expected.Valid && len(tc.Expected.Errors) == 0 {
		tc.Expected.Valid = true
	}

	return tc, nil
}

// RunTestCase executes a single test case and reports results.
func RunTestCase(t *testing.T, tc *TestCase) {
	t.Helper()

	result := api.Validate(tc.Source)

	if tc.Expected.Valid {
		// Expect valid shader
		if !result.Valid {
			t.Errorf("expected valid shader, got %d error(s):", result.ErrorCount)
			for _, d := range result.Diagnostics {
				if d.Severity == "error" {
					t.Errorf("  %d:%d: %s [%s]", d.Line, d.Column, d.Message, d.Code)
				}
			}
		}
	} else {
		// Expect errors
		if result.Valid {
			t.Errorf("expected invalid shader with errors, but validation passed")
			return
		}

		// Check that expected errors are present
		for _, expected := range tc.Expected.Errors {
			found := false
			for _, actual := range result.Diagnostics {
				if actual.Severity != "error" {
					continue
				}
				if expected.Code != "" && actual.Code != expected.Code {
					continue
				}
				if expected.Pattern != "" && !strings.Contains(actual.Message, expected.Pattern) {
					continue
				}
				found = true
				break
			}
			if !found {
				t.Errorf("expected error not found: code=%s pattern=%q", expected.Code, expected.Pattern)
			}
		}
	}

	// Check expected warnings (independent of valid/invalid)
	for _, expected := range tc.Expected.Warnings {
		found := false
		for _, actual := range result.Diagnostics {
			if actual.Severity != "warning" {
				continue
			}
			if expected.Code != "" && actual.Code != expected.Code {
				continue
			}
			if expected.Pattern != "" && !strings.Contains(actual.Message, expected.Pattern) {
				continue
			}
			found = true
			break
		}
		if !found {
			t.Errorf("expected warning not found: code=%s pattern=%q", expected.Code, expected.Pattern)
		}
	}
}

// RunTestDir runs all .wgsl test files in a directory.
func RunTestDir(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read test directory %s: %v", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively process subdirectories
			subdir := filepath.Join(dir, entry.Name())
			t.Run(entry.Name(), func(t *testing.T) {
				RunTestDir(t, subdir)
			})
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".wgsl") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		tc, err := ParseTestFile(path)
		if err != nil {
			t.Errorf("failed to parse test file %s: %v", path, err)
			continue
		}

		t.Run(tc.Name, func(t *testing.T) {
			RunTestCase(t, tc)
		})
	}
}

// TestResult holds the result of running a test case.
type TestResult struct {
	Name    string
	Passed  bool
	Errors  []string
	File    string
}

// RunTestDirCollect runs tests and collects results without failing.
func RunTestDirCollect(dir string) ([]TestResult, error) {
	var results []TestResult

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".wgsl") {
			return nil
		}

		tc, err := ParseTestFile(path)
		if err != nil {
			results = append(results, TestResult{
				Name:   path,
				Passed: false,
				Errors: []string{fmt.Sprintf("parse error: %v", err)},
				File:   path,
			})
			return nil
		}

		result := api.Validate(tc.Source)
		tr := TestResult{
			Name: tc.Name,
			File: path,
		}

		if tc.Expected.Valid {
			if result.Valid {
				tr.Passed = true
			} else {
				tr.Passed = false
				for _, d := range result.Diagnostics {
					if d.Severity == "error" {
						tr.Errors = append(tr.Errors, fmt.Sprintf("%d:%d: %s [%s]", d.Line, d.Column, d.Message, d.Code))
					}
				}
			}
		} else {
			if !result.Valid {
				tr.Passed = true
				// Could also verify specific errors match
			} else {
				tr.Passed = false
				tr.Errors = append(tr.Errors, "expected validation to fail but it passed")
			}
		}

		results = append(results, tr)
		return nil
	})

	return results, err
}

// PrintTestSummary prints a summary of test results.
func PrintTestSummary(results []TestResult) {
	passed := 0
	failed := 0

	for _, r := range results {
		if r.Passed {
			passed++
		} else {
			failed++
			fmt.Printf("FAIL: %s\n", r.Name)
			for _, e := range r.Errors {
				fmt.Printf("  %s\n", e)
			}
		}
	}

	fmt.Printf("\n%d passed, %d failed, %d total\n", passed, failed, len(results))
}
