// Package test provides testing utilities for the WGSL minifier.
//
// This follows esbuild's testing patterns with helper functions
// for assertions, diffs, and common test patterns.
package test

import (
	"fmt"
	"strings"
	"testing"
)

// AssertEqual checks if two values are equal and reports a test error if not.
func AssertEqual[T comparable](t *testing.T, actual, expected T) {
	t.Helper()
	if actual != expected {
		t.Errorf("\nexpected: %v\nactual:   %v", expected, actual)
	}
}

// AssertEqualWithDiff checks if two strings are equal and shows a diff if not.
func AssertEqualWithDiff(t *testing.T, actual, expected string) {
	t.Helper()
	if actual != expected {
		diff := Diff(expected, actual)
		t.Errorf("\n%s", diff)
	}
}

// Diff produces a line-by-line diff between two strings.
// Shows context around differences with +/- prefixes.
func Diff(expected, actual string) string {
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var result strings.Builder
	result.WriteString("--- expected\n+++ actual\n")

	// Simple line-by-line diff (not LCS for simplicity)
	maxLines := len(expectedLines)
	if len(actualLines) > maxLines {
		maxLines = len(actualLines)
	}

	for i := 0; i < maxLines; i++ {
		var expLine, actLine string
		if i < len(expectedLines) {
			expLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actLine = actualLines[i]
		}

		if expLine != actLine {
			if i < len(expectedLines) {
				result.WriteString(fmt.Sprintf("-%s\n", expLine))
			}
			if i < len(actualLines) {
				result.WriteString(fmt.Sprintf("+%s\n", actLine))
			}
		} else {
			result.WriteString(fmt.Sprintf(" %s\n", expLine))
		}
	}

	return result.String()
}

// MarkFailure is a wrapper that marks a test as failed with a message.
func MarkFailure(t *testing.T, format string, args ...interface{}) {
	t.Helper()
	t.Errorf(format, args...)
}

// Suite provides a test context for related tests.
type Suite struct {
	t *testing.T
}

// NewSuite creates a new test suite.
func NewSuite(t *testing.T) *Suite {
	return &Suite{t: t}
}

// Run runs a subtest.
func (s *Suite) Run(name string, fn func(t *testing.T)) {
	s.t.Run(name, fn)
}
