package validator_tests

import (
	"os"
	"path/filepath"
	"testing"
)

func getTestDataDir() string {
	// Try to find testdata relative to current working directory
	if _, err := os.Stat("testdata/validation"); err == nil {
		return "testdata/validation"
	}
	// Try relative to project root
	if _, err := os.Stat("../../testdata/validation"); err == nil {
		return "../../testdata/validation"
	}
	return ""
}

func TestExpressions(t *testing.T) {
	dir := getTestDataDir()
	if dir == "" {
		t.Skip("testdata/validation directory not found")
	}
	exprDir := filepath.Join(dir, "expressions")
	if _, err := os.Stat(exprDir); os.IsNotExist(err) {
		t.Skip("testdata/validation/expressions directory not found")
	}
	RunTestDir(t, exprDir)
}

func TestTypes(t *testing.T) {
	dir := getTestDataDir()
	if dir == "" {
		t.Skip("testdata/validation directory not found")
	}
	typesDir := filepath.Join(dir, "types")
	if _, err := os.Stat(typesDir); os.IsNotExist(err) {
		t.Skip("testdata/validation/types directory not found")
	}
	RunTestDir(t, typesDir)
}

func TestDeclarations(t *testing.T) {
	dir := getTestDataDir()
	if dir == "" {
		t.Skip("testdata/validation directory not found")
	}
	declDir := filepath.Join(dir, "declarations")
	if _, err := os.Stat(declDir); os.IsNotExist(err) {
		t.Skip("testdata/validation/declarations directory not found")
	}
	RunTestDir(t, declDir)
}

func TestBuiltins(t *testing.T) {
	dir := getTestDataDir()
	if dir == "" {
		t.Skip("testdata/validation directory not found")
	}
	builtinsDir := filepath.Join(dir, "builtins")
	if _, err := os.Stat(builtinsDir); os.IsNotExist(err) {
		t.Skip("testdata/validation/builtins directory not found")
	}
	RunTestDir(t, builtinsDir)
}

func TestUniformity(t *testing.T) {
	dir := getTestDataDir()
	if dir == "" {
		t.Skip("testdata/validation directory not found")
	}
	uniformityDir := filepath.Join(dir, "uniformity")
	if _, err := os.Stat(uniformityDir); os.IsNotExist(err) {
		t.Skip("testdata/validation/uniformity directory not found")
	}
	RunTestDir(t, uniformityDir)
}

func TestErrors(t *testing.T) {
	dir := getTestDataDir()
	if dir == "" {
		t.Skip("testdata/validation directory not found")
	}
	errorsDir := filepath.Join(dir, "errors")
	if _, err := os.Stat(errorsDir); os.IsNotExist(err) {
		t.Skip("testdata/validation/errors directory not found")
	}
	RunTestDir(t, errorsDir)
}

func TestAllValidation(t *testing.T) {
	dir := getTestDataDir()
	if dir == "" {
		t.Skip("testdata/validation directory not found")
	}
	RunTestDir(t, dir)
}
