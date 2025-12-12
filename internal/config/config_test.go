package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFile(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "wgslmin.json")

	content := `{
		"minifyWhitespace": false,
		"minifyIdentifiers": true,
		"mangleExternalBindings": true,
		"keepNames": ["foo", "bar"]
	}`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, err := LoadFile(configPath)
	if err != nil {
		t.Fatalf("LoadFile failed: %v", err)
	}

	if cfg.MinifyWhitespace == nil || *cfg.MinifyWhitespace != false {
		t.Errorf("MinifyWhitespace: got %v, want false", cfg.MinifyWhitespace)
	}

	if cfg.MinifyIdentifiers == nil || *cfg.MinifyIdentifiers != true {
		t.Errorf("MinifyIdentifiers: got %v, want true", cfg.MinifyIdentifiers)
	}

	if cfg.MangleExternalBindings == nil || *cfg.MangleExternalBindings != true {
		t.Errorf("MangleExternalBindings: got %v, want true", cfg.MangleExternalBindings)
	}

	if len(cfg.KeepNames) != 2 || cfg.KeepNames[0] != "foo" || cfg.KeepNames[1] != "bar" {
		t.Errorf("KeepNames: got %v, want [foo bar]", cfg.KeepNames)
	}
}

func TestLoad(t *testing.T) {
	// Create nested directories with config in parent
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "project", "shaders")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("failed to create dirs: %v", err)
	}

	// Create config in project dir (one level up from shaders)
	configPath := filepath.Join(tmpDir, "project", "wgslmin.json")
	content := `{"mangleExternalBindings": true}`

	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	// Search from shaders dir - should find config in parent
	cfg, foundPath, err := Load(subDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected config, got nil")
	}

	if foundPath != configPath {
		t.Errorf("found config at %s, expected %s", foundPath, configPath)
	}

	if cfg.MangleExternalBindings == nil || *cfg.MangleExternalBindings != true {
		t.Errorf("MangleExternalBindings: got %v, want true", cfg.MangleExternalBindings)
	}
}

func TestLoadNoConfig(t *testing.T) {
	tmpDir := t.TempDir()

	cfg, path, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg != nil {
		t.Errorf("expected nil config, got %v", cfg)
	}

	if path != "" {
		t.Errorf("expected empty path, got %s", path)
	}
}

func TestToOptions(t *testing.T) {
	trueVal := true
	falseVal := false

	cfg := &Config{
		MinifyWhitespace:       &falseVal,
		MinifyIdentifiers:      &trueVal,
		MangleExternalBindings: &trueVal,
		KeepNames:              []string{"keep1", "keep2"},
	}

	opts := cfg.ToOptions()

	if opts.MinifyWhitespace != false {
		t.Errorf("MinifyWhitespace: got %v, want false", opts.MinifyWhitespace)
	}

	if opts.MinifyIdentifiers != true {
		t.Errorf("MinifyIdentifiers: got %v, want true", opts.MinifyIdentifiers)
	}

	// MinifySyntax should be default (true) since not set in config
	if opts.MinifySyntax != true {
		t.Errorf("MinifySyntax: got %v, want true (default)", opts.MinifySyntax)
	}

	if opts.MangleExternalBindings != true {
		t.Errorf("MangleExternalBindings: got %v, want true", opts.MangleExternalBindings)
	}

	if len(opts.KeepNames) != 2 {
		t.Errorf("KeepNames: got %v, want 2 items", opts.KeepNames)
	}
}

func TestMerge(t *testing.T) {
	trueVal := true
	falseVal := false

	// Config sets MangleExternalBindings to false
	cfg := &Config{
		MangleExternalBindings: &falseVal,
	}

	// CLI overrides to true
	cliOpts := MergeOptions{
		MangleExternalBindings: &trueVal,
	}

	opts := cfg.Merge(cliOpts)

	// CLI should win
	if opts.MangleExternalBindings != true {
		t.Errorf("MangleExternalBindings: got %v, want true (CLI override)", opts.MangleExternalBindings)
	}
}

func TestMergeNoMangle(t *testing.T) {
	trueVal := true

	// Config enables identifiers
	cfg := &Config{
		MinifyIdentifiers: &trueVal,
	}

	// CLI disables with --no-mangle
	cliOpts := MergeOptions{
		NoMangle: true,
	}

	opts := cfg.Merge(cliOpts)

	// NoMangle should disable identifiers
	if opts.MinifyIdentifiers != false {
		t.Errorf("MinifyIdentifiers: got %v, want false (--no-mangle)", opts.MinifyIdentifiers)
	}
}

func TestMergeKeepNames(t *testing.T) {
	// Config has some keep names
	cfg := &Config{
		KeepNames: []string{"configName1", "configName2"},
	}

	// CLI adds more
	cliOpts := MergeOptions{
		KeepNames: []string{"cliName"},
	}

	opts := cfg.Merge(cliOpts)

	// Should have all three
	if len(opts.KeepNames) != 3 {
		t.Errorf("KeepNames: got %d items, want 3", len(opts.KeepNames))
	}
}

func TestConfigFileNames(t *testing.T) {
	// Test that all supported config file names are searched
	tmpDir := t.TempDir()

	// Test .wgslminrc (second priority)
	rcPath := filepath.Join(tmpDir, ".wgslminrc")
	content := `{"mangleExternalBindings": true}`

	if err := os.WriteFile(rcPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, foundPath, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("expected config, got nil")
	}

	if filepath.Base(foundPath) != ".wgslminrc" {
		t.Errorf("expected .wgslminrc, got %s", filepath.Base(foundPath))
	}

	// Now add wgslmin.json (higher priority) - should use that instead
	jsonPath := filepath.Join(tmpDir, "wgslmin.json")
	jsonContent := `{"mangleExternalBindings": false}`

	if err := os.WriteFile(jsonPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("failed to write config file: %v", err)
	}

	cfg, foundPath, err = Load(tmpDir)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if filepath.Base(foundPath) != "wgslmin.json" {
		t.Errorf("expected wgslmin.json (higher priority), got %s", filepath.Base(foundPath))
	}

	// Verify it's the json content (false vs true)
	if cfg.MangleExternalBindings == nil || *cfg.MangleExternalBindings != false {
		t.Errorf("MangleExternalBindings: got %v, want false (from wgslmin.json)", cfg.MangleExternalBindings)
	}
}
