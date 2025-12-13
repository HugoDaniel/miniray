// Package config handles loading minifier configuration from files.
//
// Configuration can be specified in a JSON file named wgslmin.json or .wgslminrc.
// The config file is searched for in the current directory and parent directories.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/HugoDaniel/miniray/internal/minifier"
)

// Config represents the configuration file structure.
// All fields are optional and will use default values if not specified.
type Config struct {
	// MinifyWhitespace removes unnecessary whitespace and newlines
	MinifyWhitespace *bool `json:"minifyWhitespace,omitempty"`

	// MinifyIdentifiers renames identifiers to shorter names
	MinifyIdentifiers *bool `json:"minifyIdentifiers,omitempty"`

	// MinifySyntax applies syntax-level optimizations
	MinifySyntax *bool `json:"minifySyntax,omitempty"`

	// MangleProps renames struct member names
	MangleProps *bool `json:"mangleProps,omitempty"`

	// MangleExternalBindings controls whether uniform/storage variable names
	// are mangled directly (true) or kept with original names and aliased (false)
	MangleExternalBindings *bool `json:"mangleExternalBindings,omitempty"`

	// TreeShaking enables dead code elimination (default true)
	TreeShaking *bool `json:"treeShaking,omitempty"`

	// KeepNames lists identifier names that should not be renamed
	KeepNames []string `json:"keepNames,omitempty"`
}

// ConfigFileNames are the names searched for config files, in order of preference.
var ConfigFileNames = []string{
	"wgslmin.json",
	".wgslminrc",
	".wgslminrc.json",
}

// Load searches for a config file starting from the given directory
// and walking up to parent directories. Returns nil if no config file is found.
func Load(startDir string) (*Config, string, error) {
	dir := startDir
	for {
		for _, name := range ConfigFileNames {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				cfg, err := LoadFile(path)
				return cfg, path, err
			}
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached root, no config found
			return nil, "", nil
		}
		dir = parent
	}
}

// LoadFile loads configuration from a specific file path.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ToOptions converts a Config to minifier.Options, using defaults for unset fields.
func (c *Config) ToOptions() minifier.Options {
	opts := minifier.DefaultOptions()

	if c.MinifyWhitespace != nil {
		opts.MinifyWhitespace = *c.MinifyWhitespace
	}
	if c.MinifyIdentifiers != nil {
		opts.MinifyIdentifiers = *c.MinifyIdentifiers
	}
	if c.MinifySyntax != nil {
		opts.MinifySyntax = *c.MinifySyntax
	}
	if c.MangleProps != nil {
		opts.MangleProps = *c.MangleProps
	}
	if c.MangleExternalBindings != nil {
		opts.MangleExternalBindings = *c.MangleExternalBindings
	}
	if c.TreeShaking != nil {
		opts.TreeShaking = *c.TreeShaking
	}
	if len(c.KeepNames) > 0 {
		opts.KeepNames = c.KeepNames
	}

	return opts
}

// Merge combines config file options with CLI options.
// CLI options take precedence over config file options.
type MergeOptions struct {
	// CLI flags (nil means not specified on CLI)
	MinifyWhitespace       *bool
	MinifyIdentifiers      *bool
	MinifySyntax           *bool
	MangleExternalBindings *bool
	NoMangle               bool
	NoTreeShaking          bool
	KeepNames              []string
}

// Merge merges CLI options with config file options.
// CLI options override config file options when specified.
func (c *Config) Merge(cli MergeOptions) minifier.Options {
	opts := c.ToOptions()

	// CLI overrides
	if cli.MinifyWhitespace != nil {
		opts.MinifyWhitespace = *cli.MinifyWhitespace
	}
	if cli.MinifyIdentifiers != nil {
		opts.MinifyIdentifiers = *cli.MinifyIdentifiers
	}
	if cli.MinifySyntax != nil {
		opts.MinifySyntax = *cli.MinifySyntax
	}
	if cli.MangleExternalBindings != nil {
		opts.MangleExternalBindings = *cli.MangleExternalBindings
	}
	if cli.NoMangle {
		opts.MinifyIdentifiers = false
	}
	if cli.NoTreeShaking {
		opts.TreeShaking = false
	}
	if len(cli.KeepNames) > 0 {
		// Append CLI keep names to config keep names
		opts.KeepNames = append(opts.KeepNames, cli.KeepNames...)
	}

	return opts
}
