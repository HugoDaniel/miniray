// Command miniray minifies WGSL shader source code.
//
// Usage:
//
//	miniray [options] <input.wgsl>
//	cat input.wgsl | miniray [options]
//
// Options:
//
//	-o <file>                  Write output to file (default: stdout)
//	--config <file>            Use specific config file
//	--no-config                Ignore config files
//	--minify                   Enable all minification (default)
//	--minify-whitespace        Remove unnecessary whitespace
//	--minify-identifiers       Shorten identifier names
//	--minify-syntax            Apply syntax optimizations
//	--no-mangle                Don't rename identifiers
//	--mangle-external-bindings Rename uniform/storage vars directly (no aliases)
//	--keep-names <names>       Comma-separated names to preserve
//	--version                  Print version and exit
//	--help                     Print help and exit
//
// Config file:
//
//	miniray looks for miniray.json or .minirayrc in the current directory
//	and parent directories. Config file options are overridden by CLI flags.
//
// Example miniray.json:
//
//	{
//	    "minifyWhitespace": true,
//	    "minifyIdentifiers": true,
//	    "minifySyntax": true,
//	    "mangleExternalBindings": false,
//	    "keepNames": ["myUniform"]
//	}
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/HugoDaniel/miniray/internal/config"
	"github.com/HugoDaniel/miniray/internal/minifier"
)

var (
	version = "0.1.0"
	commit  = "dev"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Flags
	var (
		outputFile             string
		configFile             string
		noConfig               bool
		minifyAll              bool
		minifyWhitespace       bool
		minifyIdentifiers      bool
		minifySyntax           bool
		noMangle               bool
		mangleExternalBindings bool
		keepNames              string
		showVersion            bool
		showHelp               bool
	)

	flag.StringVar(&outputFile, "o", "", "Write output to `file`")
	flag.StringVar(&configFile, "config", "", "Use specific config `file`")
	flag.BoolVar(&noConfig, "no-config", false, "Ignore config files")
	flag.BoolVar(&minifyAll, "minify", true, "Enable all minification")
	flag.BoolVar(&minifyWhitespace, "minify-whitespace", false, "Remove unnecessary whitespace")
	flag.BoolVar(&minifyIdentifiers, "minify-identifiers", false, "Shorten identifier names")
	flag.BoolVar(&minifySyntax, "minify-syntax", false, "Apply syntax optimizations")
	flag.BoolVar(&noMangle, "no-mangle", false, "Don't rename identifiers")
	flag.BoolVar(&mangleExternalBindings, "mangle-external-bindings", false, "Rename uniform/storage vars directly")
	flag.StringVar(&keepNames, "keep-names", "", "Comma-separated names to preserve")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.BoolVar(&showHelp, "help", false, "Print help and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "miniray - WGSL Minifier v%s\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: miniray [options] <input.wgsl>\n")
		fmt.Fprintf(os.Stderr, "       cat input.wgsl | miniray [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nConfig file:\n")
		fmt.Fprintf(os.Stderr, "  Searches for miniray.json or .minirayrc in current and parent directories.\n")
		fmt.Fprintf(os.Stderr, "  CLI flags override config file settings.\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  miniray shader.wgsl -o shader.min.wgsl\n")
		fmt.Fprintf(os.Stderr, "  cat shader.wgsl | miniray > shader.min.wgsl\n")
		fmt.Fprintf(os.Stderr, "  miniray --no-mangle shader.wgsl\n")
		fmt.Fprintf(os.Stderr, "  miniray --mangle-external-bindings shader.wgsl\n")
	}

	flag.Parse()

	if showHelp {
		flag.Usage()
		return nil
	}

	if showVersion {
		fmt.Printf("miniray v%s (%s)\n", version, commit)
		return nil
	}

	// Read input
	var source []byte
	var err error

	if flag.NArg() > 0 {
		// Read from file
		source, err = os.ReadFile(flag.Arg(0))
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
	} else {
		// Check if stdin is a pipe
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			flag.Usage()
			return fmt.Errorf("no input file specified")
		}
		// Read from stdin
		source, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
	}

	// Load config file
	var cfg *config.Config
	var configPath string
	if !noConfig {
		var err error
		if configFile != "" {
			// Use specified config file
			cfg, err = config.LoadFile(configFile)
			if err != nil {
				return fmt.Errorf("loading config file %s: %w", configFile, err)
			}
			configPath = configFile
		} else {
			// Search for config file
			startDir, _ := os.Getwd()
			if flag.NArg() > 0 {
				startDir = filepath.Dir(flag.Arg(0))
			}
			cfg, configPath, err = config.Load(startDir)
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
		}
	}

	// Build options from config (or defaults) and CLI overrides
	var opts minifier.Options
	if cfg != nil {
		// Parse keep-names from CLI
		var cliKeepNames []string
		if keepNames != "" {
			cliKeepNames = strings.Split(keepNames, ",")
			for i := range cliKeepNames {
				cliKeepNames[i] = strings.TrimSpace(cliKeepNames[i])
			}
		}

		// Build CLI overrides - only set if explicitly specified
		cliOpts := config.MergeOptions{
			NoMangle:  noMangle,
			KeepNames: cliKeepNames,
		}

		// Check if specific minify flags were set
		if minifyWhitespace {
			cliOpts.MinifyWhitespace = &minifyWhitespace
		}
		if minifyIdentifiers {
			cliOpts.MinifyIdentifiers = &minifyIdentifiers
		}
		if minifySyntax {
			cliOpts.MinifySyntax = &minifySyntax
		}
		if mangleExternalBindings {
			cliOpts.MangleExternalBindings = &mangleExternalBindings
		}

		opts = cfg.Merge(cliOpts)

		// Print config file path if verbose
		if outputFile != "" && configPath != "" {
			fmt.Fprintf(os.Stderr, "Using config: %s\n", configPath)
		}
	} else {
		// No config file, use defaults + CLI flags
		opts = minifier.Options{}

		// If specific flags are set, use them; otherwise use minifyAll
		if minifyWhitespace || minifyIdentifiers || minifySyntax {
			opts.MinifyWhitespace = minifyWhitespace
			opts.MinifyIdentifiers = minifyIdentifiers
			opts.MinifySyntax = minifySyntax
		} else if minifyAll {
			opts.MinifyWhitespace = true
			opts.MinifyIdentifiers = true
			opts.MinifySyntax = true
		}

		// Override with no-mangle
		if noMangle {
			opts.MinifyIdentifiers = false
		}

		// Set mangle external bindings
		opts.MangleExternalBindings = mangleExternalBindings

		// Parse keep-names
		if keepNames != "" {
			opts.KeepNames = strings.Split(keepNames, ",")
			for i := range opts.KeepNames {
				opts.KeepNames[i] = strings.TrimSpace(opts.KeepNames[i])
			}
		}
	}

	// Minify
	m := minifier.New(opts)
	result := m.Minify(string(source))

	// Check for errors
	if len(result.Errors) > 0 {
		for _, e := range result.Errors {
			fmt.Fprintf(os.Stderr, "error: %s\n", e.Message)
		}
		return fmt.Errorf("minification failed with %d error(s)", len(result.Errors))
	}

	// Write output
	var output io.Writer = os.Stdout
	if outputFile != "" {
		f, err := os.Create(outputFile)
		if err != nil {
			return fmt.Errorf("creating output file: %w", err)
		}
		defer f.Close()
		output = f
	}

	_, err = io.WriteString(output, result.Code)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	// Print stats to stderr if output is to file
	if outputFile != "" {
		ratio := float64(result.Stats.MinifiedSize) / float64(result.Stats.OriginalSize) * 100
		fmt.Fprintf(os.Stderr, "Minified: %d -> %d bytes (%.1f%%)\n",
			result.Stats.OriginalSize, result.Stats.MinifiedSize, ratio)
	}

	return nil
}
