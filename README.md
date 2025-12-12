# miniray

A high-performance WGSL (WebGPU Shading Language) minifier written in Go, inspired by [esbuild](https://esbuild.github.io/)'s architecture.

**[Try the online demo](https://hugodaniel.com/pages/miniray/)**

## Features

- **Whitespace minification** - Remove unnecessary whitespace and newlines
- **Identifier renaming** - Shorten local variable and function names
- **Syntax optimization** - Optimize numeric literals and syntax patterns
- **API-aware** - Preserves entry point names, `@location`, `@binding`, and `@builtin` declarations
- **WebAssembly build** - Run in browsers and Node.js via `miniray` package

## Installation

### CLI (Go)

```bash
# From source
go install github.com/HugoDaniel/miniray/cmd/miniray@latest

# Or build locally
git clone https://github.com/HugoDaniel/miniray.git
cd miniray
make build
```

### Browser/Node.js (WASM)

```bash
npm install miniray
```

```javascript
import { initialize, minify } from 'miniray';

await initialize({ wasmURL: '/path/to/miniray.wasm' });

const result = minify(source, {
  minifyWhitespace: true,
  minifyIdentifiers: true,
});

console.log(result.code);
```

See [npm/miniray/README.md](npm/miniray/README.md) for full WASM documentation.

## Usage

```bash
# Minify a file
miniray shader.wgsl -o shader.min.wgsl

# Minify from stdin
cat shader.wgsl | miniray > shader.min.wgsl

# Whitespace-only minification (safest)
miniray --no-mangle shader.wgsl

# Preserve specific names
miniray --keep-names myHelper,computeOffset shader.wgsl

# Mangle uniform/storage bindings directly (smaller output)
miniray --mangle-external-bindings shader.wgsl

# Use a specific config file
miniray --config myconfig.json shader.wgsl
```

### Options

| Flag | Description |
|------|-------------|
| `-o <file>` | Write output to file (default: stdout) |
| `--config <file>` | Use specific config file |
| `--no-config` | Ignore config files |
| `--minify` | Enable all minification (default) |
| `--minify-whitespace` | Remove unnecessary whitespace |
| `--minify-identifiers` | Shorten identifier names |
| `--minify-syntax` | Apply syntax optimizations |
| `--no-mangle` | Don't rename identifiers |
| `--mangle-external-bindings` | Rename uniform/storage vars directly |
| `--keep-names <names>` | Comma-separated names to preserve |
| `--version` | Print version and exit |
| `--help` | Print help and exit |

### Config File

The minifier searches for config files in the current directory and parent directories:
- `miniray.json`
- `.minirayrc`
- `.minirayrc.json`

Example `miniray.json`:
```json
{
    "minifyWhitespace": true,
    "minifyIdentifiers": true,
    "minifySyntax": true,
    "mangleExternalBindings": false,
    "keepNames": ["myUniform", "myBuffer"]
}
```

CLI flags override config file settings.

### Platform Configurations

Pre-built configuration templates are available in the `configs/` directory:

#### compute.toys

[compute.toys](https://compute.toys) is a WebGPU compute shader playground. Use the provided config to preserve all platform-specific bindings and helpers:

```bash
miniray --config configs/compute.toys.json my_shader.wgsl
```

Or copy `configs/compute.toys.json` to your project directory as `miniray.json` for automatic detection.

**Preserved names:**
- Uniforms: `time`, `mouse`, `custom`, `dispatch`
- Textures: `screen`, `pass_in`, `pass_out`, `channel0`, `channel1`
- Samplers: `nearest`, `bilinear`, `trilinear`, `*_repeat` variants
- Helpers: `keyDown`, `passLoad`, `passStore`, `passSampleLevelBilinearRepeat`
- Type aliases: `int`, `float`, `float2`..`float4`, `float2x2`..`float4x4`, etc.
- Entry point: `main_image`

**Size reduction:** ~55% on typical shaders

### Go API

```go
import "github.com/HugoDaniel/miniray/pkg/api"

// Full minification
result := api.Minify(source)

// Custom options
result := api.MinifyWithOptions(source, api.MinifyOptions{
    MinifyWhitespace:       true,
    MinifyIdentifiers:      true,
    MinifySyntax:           true,
    MangleExternalBindings: false, // keep uniform/storage names for reflection
    KeepNames:              []string{"myHelper"},
})

// Safe whitespace-only
result := api.MinifyWhitespaceOnly(source)

fmt.Printf("Minified: %d -> %d bytes\n",
    result.OriginalSize, result.MinifiedSize)
fmt.Println(result.Code)
```

## What Gets Minified

### Always Preserved
- Entry point function names (`@vertex`, `@fragment`, `@compute`)
- `@builtin` names (e.g., `position`, `global_invocation_id`)
- `@location` struct member names
- `@group`/`@binding` binding indices
- `override` constant names (unless using `@id`)

### External Bindings (uniform/storage)

By default, `var<uniform>` and `var<storage>` declarations keep their original names for WebGPU binding reflection compatibility:

```wgsl
// Input
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
fn main() { return uniforms.value * 2.0; }

// Output (default) - declaration preserved, alias used internally
var<uniform> uniforms:Uniforms;let a=uniforms;fn b(){return a.value*2.0;}

// Output (--mangle-external-bindings) - smaller, but breaks reflection
var<uniform> a:Uniforms;fn b(){return a.value*2.0;}
```

Use `--mangle-external-bindings` only if you don't use WebGPU's binding reflection APIs.

### Minified (with `--minify-identifiers`)
- Local variables (`let`, `var`)
- Function parameters
- Non-entry-point function names
- Private struct names (no API interface)
- Type alias names

### Syntax Optimizations (with `--minify-syntax`)
- Numeric literal shortening: `0.5` → `.5`, `1.0` → `1.`
- Scientific notation: `1000000.0` → `1e6`
- Vector constructor shorthand: `vec3<f32>` → `vec3f`

## Architecture

The minifier follows esbuild's architecture:

```
Source → Lexer → Parser → AST → Minifier → Printer → Output
                           ↑
                        Renamer
```

### Packages

| Package | Description |
|---------|-------------|
| `internal/lexer` | Tokenizes WGSL source (Unicode XID, nested comments) |
| `internal/parser` | Recursive descent parser with symbol table |
| `internal/ast` | Complete WGSL AST types |
| `internal/renamer` | Frequency-based identifier renaming |
| `internal/printer` | Code generation with minification |
| `internal/minifier` | Orchestrates the minification pipeline |
| `pkg/api` | Public API for programmatic use |
| `cmd/miniray` | CLI entry point |

## Development

```bash
# Build
make build

# Test
make test

# Format and lint
make check

# Build for all platforms
make build-all
```

## WGSL Spec Compliance

This minifier targets WGSL as specified in the [WebGPU Shading Language](https://www.w3.org/TR/WGSL/) specification. Key considerations:

- Identifiers use Unicode XID_Start/XID_Continue
- Block comments can nest (`/* /* nested */ */`)
- Reserved words (~120) cannot be used as identifiers
- `_` alone and `__*` prefixes are invalid identifiers
- Boolean literals `true`/`false` are keywords (not expressions)

## License

See [LICENSE](LICENSE) file.

## Contributing

Contributions welcome! Please open an issue or pull request.
