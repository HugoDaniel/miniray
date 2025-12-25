// Command miniray minifies WGSL shader source code.
//
// Usage:
//
//	miniray [options] <input.wgsl>
//	miniray reflect [options] <input.wgsl>
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
//	--source-map               Generate source map file (.map)
//	--source-map-inline        Embed source map as inline data URI
//	--source-map-sources       Include original source in source map
//	--version                  Print version and exit
//	--help                     Print help and exit
//
// Reflect subcommand:
//
//	miniray reflect [options] <input.wgsl>
//	  -o <file>     Write JSON output to file (default: stdout)
//	  --compact     Output compact JSON (default: pretty-printed)
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
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/HugoDaniel/miniray/internal/config"
	"github.com/HugoDaniel/miniray/internal/minifier"
	"github.com/HugoDaniel/miniray/internal/reflect"
)

var (
	version = "0.1.0"
	commit  = "dev"
)

func main() {
	// Check for subcommands
	if len(os.Args) > 1 && os.Args[1] == "reflect" {
		if err := runReflect(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	// Flags
	var (
		outputFile                 string
		configFile                 string
		noConfig                   bool
		minifyAll                  bool
		minifyWhitespace           bool
		minifyIdentifiers          bool
		minifySyntax               bool
		noMangle                   bool
		mangleExternalBindings     bool
		noTreeShaking              bool
		preserveUniformStructTypes bool
		keepNames                  string
		sourceMap                  bool
		sourceMapInline            bool
		sourceMapSources           bool
		showVersion                bool
		showHelp                   bool
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
	flag.BoolVar(&noTreeShaking, "no-tree-shaking", false, "Disable dead code elimination")
	flag.BoolVar(&preserveUniformStructTypes, "preserve-uniform-struct-types", false, "Preserve struct types used in uniform/storage declarations")
	flag.StringVar(&keepNames, "keep-names", "", "Comma-separated names to preserve")
	flag.BoolVar(&sourceMap, "source-map", false, "Generate source map file (.map)")
	flag.BoolVar(&sourceMapInline, "source-map-inline", false, "Embed source map as inline data URI")
	flag.BoolVar(&sourceMapSources, "source-map-sources", false, "Include original source in source map")
	flag.BoolVar(&showVersion, "version", false, "Print version and exit")
	flag.BoolVar(&showHelp, "help", false, "Print help and exit")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "miniray - WGSL Minifier v%s\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: miniray [options] <input.wgsl>\n")
		fmt.Fprintf(os.Stderr, "       miniray reflect [options] <input.wgsl>\n")
		fmt.Fprintf(os.Stderr, "       cat input.wgsl | miniray [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nSubcommands:\n")
		fmt.Fprintf(os.Stderr, "  reflect    Extract bindings, struct layouts, and entry points as JSON\n")
		fmt.Fprintf(os.Stderr, "             Run 'miniray reflect --help' for details\n")
		fmt.Fprintf(os.Stderr, "\nConfig file:\n")
		fmt.Fprintf(os.Stderr, "  Searches for miniray.json or .minirayrc in current and parent directories.\n")
		fmt.Fprintf(os.Stderr, "  CLI flags override config file settings.\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  miniray shader.wgsl -o shader.min.wgsl\n")
		fmt.Fprintf(os.Stderr, "  miniray reflect shader.wgsl -o info.json\n")
		fmt.Fprintf(os.Stderr, "  cat shader.wgsl | miniray > shader.min.wgsl\n")
		fmt.Fprintf(os.Stderr, "  miniray --no-mangle shader.wgsl\n")
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
			NoMangle:      noMangle,
			NoTreeShaking: noTreeShaking,
			KeepNames:     cliKeepNames,
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
		if preserveUniformStructTypes {
			cliOpts.PreserveUniformStructTypes = &preserveUniformStructTypes
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

		// Set tree shaking (on by default)
		opts.TreeShaking = !noTreeShaking

		// Set preserve uniform struct types
		opts.PreserveUniformStructTypes = preserveUniformStructTypes

		// Parse keep-names
		if keepNames != "" {
			opts.KeepNames = strings.Split(keepNames, ",")
			for i := range opts.KeepNames {
				opts.KeepNames[i] = strings.TrimSpace(opts.KeepNames[i])
			}
		}
	}

	// Configure source map options
	generateSourceMap := sourceMap || sourceMapInline
	if generateSourceMap {
		opts.GenerateSourceMap = true
		opts.SourceMapOptions.IncludeSource = sourceMapSources

		// Determine source and output file names for source map
		if flag.NArg() > 0 {
			opts.SourceMapOptions.SourceName = filepath.Base(flag.Arg(0))
		}
		if outputFile != "" {
			opts.SourceMapOptions.File = filepath.Base(outputFile)
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

	// Prepare output code
	outputCode := result.Code

	// Handle inline source map
	if sourceMapInline && result.SourceMap != nil {
		outputCode += "\n//# sourceMappingURL=" + result.SourceMap.ToDataURI()
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

	_, err = io.WriteString(output, outputCode)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	// Write external source map file
	if sourceMap && !sourceMapInline && result.SourceMap != nil && outputFile != "" {
		mapFile := outputFile + ".map"
		if err := os.WriteFile(mapFile, []byte(result.SourceMap.ToJSON()), 0644); err != nil {
			return fmt.Errorf("writing source map: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Source map: %s\n", mapFile)
	}

	// Print stats to stderr if output is to file
	if outputFile != "" {
		ratio := float64(result.Stats.MinifiedSize) / float64(result.Stats.OriginalSize) * 100
		fmt.Fprintf(os.Stderr, "Minified: %d -> %d bytes (%.1f%%)\n",
			result.Stats.OriginalSize, result.Stats.MinifiedSize, ratio)
	}

	return nil
}

// runReflect handles the "reflect" subcommand.
func runReflect(args []string) error {
	fs := flag.NewFlagSet("reflect", flag.ExitOnError)

	var (
		outputFile  string
		compact     bool
		showHelp    bool
		showVersion bool
	)

	fs.StringVar(&outputFile, "o", "", "Write JSON output to `file`")
	fs.BoolVar(&compact, "compact", false, "Output compact JSON (default: pretty-printed)")
	fs.BoolVar(&showHelp, "help", false, "Print help and exit")
	fs.BoolVar(&showVersion, "version", false, "Print version and exit")

	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "miniray reflect - WGSL Shader Reflection v%s\n\n", version)
		fmt.Fprintf(os.Stderr, "Extract binding information, struct layouts, and entry points from WGSL source.\n\n")
		fmt.Fprintf(os.Stderr, "Usage: miniray reflect [options] <input.wgsl>\n")
		fmt.Fprintf(os.Stderr, "       cat input.wgsl | miniray reflect [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nOutput:\n")
		fmt.Fprintf(os.Stderr, "  JSON object with bindings, structs, entryPoints, and errors.\n")
		fmt.Fprintf(os.Stderr, "  Memory layouts follow WGSL specification (vec3 align=16, size=12, etc).\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  miniray reflect shader.wgsl\n")
		fmt.Fprintf(os.Stderr, "  miniray reflect shader.wgsl -o info.json\n")
		fmt.Fprintf(os.Stderr, "  miniray reflect --compact shader.wgsl | jq '.bindings'\n")
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if showHelp {
		fs.Usage()
		return nil
	}

	if showVersion {
		fmt.Printf("miniray reflect v%s (%s)\n", version, commit)
		return nil
	}

	// Read input
	var source []byte
	var err error

	if fs.NArg() > 0 {
		source, err = os.ReadFile(fs.Arg(0))
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}
	} else {
		// Check if stdin is a pipe
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) != 0 {
			fs.Usage()
			return fmt.Errorf("no input file specified")
		}
		source, err = io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("reading stdin: %w", err)
		}
	}

	// Run reflection
	result := reflect.Reflect(string(source))

	// Convert to JSON
	var jsonBytes []byte
	if compact {
		jsonBytes, err = json.Marshal(result)
	} else {
		jsonBytes, err = json.MarshalIndent(result, "", "  ")
	}
	if err != nil {
		return fmt.Errorf("encoding JSON: %w", err)
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

	_, err = output.Write(jsonBytes)
	if err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	// Add newline for terminal output
	if outputFile == "" {
		fmt.Println()
	}

	return nil
}
