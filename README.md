# miniray

A high-performance WGSL (WebGPU Shading Language) minifier written in Go, inspired by [esbuild](https://esbuild.github.io/)'s architecture.

**[Try the online demo](https://hugodaniel.com/pages/miniray/)**

> **Are you an LLM or AI agent?** See [`BUILDING_WITH_MINIRAY.md`](BUILDING_WITH_MINIRAY.md) for a token-efficient reference covering all options, patterns, and gotchas.

## Features

- **Whitespace minification** - Remove unnecessary whitespace and newlines
- **Identifier renaming** - Shorten local variable and function names
- **Syntax optimization** - Optimize numeric literals and syntax patterns
- **Source maps** - Generate v3 source maps for debugging minified shaders
- **Shader reflection** - Extract bindings, struct layouts, and entry points with WGSL-spec memory layout computation
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

# Reflect shader (extract bindings, structs, entry points as JSON)
miniray reflect shader.wgsl
miniray reflect shader.wgsl -o info.json
miniray reflect --compact shader.wgsl | jq '.bindings'
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
| `--no-tree-shaking` | Disable dead code elimination |
| `--preserve-uniform-struct-types` | Preserve struct types used in uniforms |
| `--keep-names <names>` | Comma-separated names to preserve |
| `--source-map` | Generate source map file (.map) |
| `--source-map-inline` | Embed source map as inline data URI |
| `--source-map-sources` | Include original source in source map |
| `--version` | Print version and exit |
| `--help` | Print help and exit |

### Reflect Subcommand

Extract binding information, struct memory layouts, and entry points as JSON:

```bash
miniray reflect [options] <input.wgsl>
```

| Flag | Description |
|------|-------------|
| `-o <file>` | Write JSON output to file (default: stdout) |
| `--compact` | Output compact JSON (default: pretty-printed) |

Output includes:
- `bindings`: Array of `@group/@binding` variables with memory layouts
- `structs`: Map of struct names to their layouts (size, alignment, fields)
- `entryPoints`: Array of entry point functions with stage and workgroup size
- `errors`: Parse errors if any

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
    "treeShaking": true,
    "preserveUniformStructTypes": false,
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

#### PNGine

[PNGine](https://github.com/HugoDaniel/pngine) is a WebGPU DSL that embeds shader code in PNG files. It detects builtin uniforms by **struct type name**, so these must be preserved:

```bash
miniray --config configs/pngine.json shader.wgsl
```

**Key feature:** Uses `preserveUniformStructTypes: true` to automatically preserve any struct type used in `var<uniform>` or `var<storage>` declarations.

**Preserved names:**
- Struct types: `PngineInputs`, `TimeInputs`, `CanvasInputs`, `SceneTimeInputs`, `GlobalTimeInputs`
- Uniform variable names (preserved by default)
- Struct field names (preserved by default - accessed via `.` operator)
- Entry point function names

**What gets renamed:**
- Helper functions
- Local variables and parameters
- Struct types not used in uniform/storage declarations

**Example:**
```wgsl
// Input
struct PngineInputs { time: f32, canvasW: f32 }
@group(0) @binding(0) var<uniform> inputs: PngineInputs;
fn helper(t: f32) -> f32 { return t * 2.0; }
@vertex fn vs() -> @builtin(position) vec4f {
  let adjusted = helper(inputs.time);
  return vec4f(adjusted);
}

// Output
struct PngineInputs{time:f32,canvasW:f32}@group(0) @binding(0) var<uniform> inputs:PngineInputs;fn a(b:f32)->f32{return b*2.;}@vertex fn vs()->@builtin(position) vec4f{let c=a(inputs.time);return vec4f(c);}
```

### Source Maps

Generate source maps to debug minified shaders by mapping back to original source positions.

```bash
# Generate external source map file (.map)
miniray -o shader.min.wgsl --source-map shader.wgsl
# Creates: shader.min.wgsl and shader.min.wgsl.map

# Embed source map as inline data URI
miniray --source-map-inline shader.wgsl > shader.min.wgsl

# Include original source in source map (self-contained)
miniray -o shader.min.wgsl --source-map --source-map-sources shader.wgsl
```

The generated source map follows the [Source Map v3 specification](https://sourcemaps.info/spec.html):

```json
{
  "version": 3,
  "file": "shader.min.wgsl",
  "sources": ["shader.wgsl"],
  "sourcesContent": ["...original source..."],
  "names": ["longVariableName", "helperFunction"],
  "mappings": "MAAAA,QAAAC,..."
}
```

**What gets mapped:**
- Renamed identifiers (variables, functions, structs) → original names in `names` array
- Generated positions → source positions via VLQ-encoded `mappings`

#### WebGPU Compatibility

WebGPU does **not** natively consume source maps—there's no `sourceMap` field in `GPUShaderModuleDescriptor`. However, miniray's source maps use UTF-16 column positions matching WebGPU's `GPUCompilationMessage` format, making them suitable for custom error translation tooling:

```javascript
import { SourceMapConsumer } from 'source-map';

// Minify with source map
const result = minify(source, { sourceMap: true, sourceMapSources: true });
const module = device.createShaderModule({ code: result.code });

// Translate compilation errors back to original source
const info = await module.getCompilationInfo();
const consumer = await new SourceMapConsumer(JSON.parse(result.sourceMap));

for (const msg of info.messages) {
  if (msg.lineNum > 0) {
    const pos = consumer.originalPositionFor({
      line: msg.lineNum,
      column: msg.linePos - 1  // source-map lib uses 0-based columns
    });
    console.log(`${msg.type}: ${msg.message}`);
    console.log(`  Original: ${pos.source}:${pos.line}:${pos.column + 1}`);
    if (pos.name) console.log(`  Identifier was: ${pos.name}`);
  }
}
```

### Shader Reflection

Extract binding information, struct memory layouts, and entry points from WGSL source:

```javascript
import { initialize, reflect } from 'miniray';

await initialize();

const result = reflect(`
  struct Uniforms { time: f32, resolution: vec2<u32> }
  @group(0) @binding(0) var<uniform> u: Uniforms;
  @group(0) @binding(1) var tex: texture_2d<f32>;
  @compute @workgroup_size(8, 8) fn main() {}
`);

console.log(result.bindings);
// [
//   { group: 0, binding: 0, name: "u", nameMapped: "u", addressSpace: "uniform",
//     type: "Uniforms", typeMapped: "Uniforms",
//     layout: { size: 16, alignment: 8, fields: [...] } },
//   { group: 0, binding: 1, name: "tex", addressSpace: "handle",
//     type: "texture_2d<f32>", layout: null }
// ]

console.log(result.entryPoints);
// [{ name: "main", stage: "compute", workgroupSize: [8, 8, 1] }]

console.log(result.structs);
// { "Uniforms": { size: 16, alignment: 8, fields: [...] } }
```

Memory layouts follow the WGSL specification (vec3 has align=16, size=12; proper struct padding).

#### Array Bindings

For array type bindings (e.g., `array<Particle>` or `array<vec4f, 100>`), reflection provides detailed array metadata:

```javascript
const result = reflect(`
  struct Particle { position: vec3f, velocity: vec3f, lifetime: f32 }
  @group(0) @binding(0) var<storage> particles: array<Particle>;
`);

const binding = result.bindings[0];
console.log(binding.array);
// {
//   depth: 1,                    // Nesting depth (1 = simple array)
//   elementCount: null,          // null for runtime-sized arrays
//   elementStride: 32,           // Bytes per element (size + padding)
//   totalSize: null,             // null for runtime-sized
//   elementType: "Particle",     // Original type name
//   elementTypeMapped: "Particle", // Minified name (see below)
//   elementLayout: { size: 28, alignment: 16, fields: [...] }
// }
```

For nested arrays (e.g., `array<array<f32, 4>, 10>`), the `array` field contains nested `ArrayInfo`:

```javascript
// depth: 1 for outer, depth: 2 for inner
// outer.array points to inner array info
```

#### Combined Minify + Reflect

When you need both minified code and reflection data with actual minified names, use `minifyAndReflect`:

```javascript
import { initialize, minifyAndReflect } from 'miniray';

await initialize();

const result = minifyAndReflect(`
  struct Particle { position: vec3f, velocity: vec3f }
  @group(0) @binding(0) var<storage> particles: array<Particle>;
  @compute @workgroup_size(64) fn main() { /* ... */ }
`);

// result.code - minified WGSL
// result.reflect - reflection with minified names

console.log(result.reflect.bindings[0]);
// {
//   name: "particles",           // Original name
//   nameMapped: "particles",     // Minified (unchanged - external binding)
//   type: "array<Particle>",     // Original type
//   typeMapped: "array<a>",      // Minified type (Particle → a)
//   array: {
//     elementType: "Particle",
//     elementTypeMapped: "a",    // Minified struct name
//     elementLayout: { fields: [
//       { name: "position", nameMapped: "position", ... },
//       { name: "velocity", nameMapped: "velocity", ... }
//     ]}
//   }
// }
```

This is useful when creating buffer bindings that need to reference minified struct field names.

### Go API

```go
import "github.com/HugoDaniel/miniray/pkg/api"

// Full minification
result := api.Minify(source)

// Custom options
result := api.MinifyWithOptions(source, api.MinifyOptions{
    MinifyWhitespace:           true,
    MinifyIdentifiers:          true,
    MinifySyntax:               true,
    MangleExternalBindings:     false, // keep uniform/storage names for reflection
    TreeShaking:                true,  // remove unused declarations
    PreserveUniformStructTypes: true,  // keep struct types used in uniforms
    KeepNames:                  []string{"myHelper"},
})

// Safe whitespace-only
result := api.MinifyWhitespaceOnly(source)

fmt.Printf("Minified: %d -> %d bytes\n",
    result.OriginalSize, result.MinifiedSize)
fmt.Println(result.Code)

// Shader reflection (without minification)
info := api.Reflect(source)
for _, binding := range info.Bindings {
    fmt.Printf("@group(%d) @binding(%d) %s: %s\n",
        binding.Group, binding.Binding, binding.Name, binding.Type)
    if binding.Layout != nil {
        fmt.Printf("  size=%d, alignment=%d\n",
            binding.Layout.Size, binding.Layout.Alignment)
    }
    if binding.Array != nil {
        fmt.Printf("  array: elementStride=%d, elementType=%s\n",
            binding.Array.ElementStride, binding.Array.ElementType)
    }
}

// Combined minify + reflect (with minified names)
combined := api.MinifyAndReflect(source)
fmt.Println(combined.Code)  // Minified WGSL
for _, b := range combined.Reflect.Bindings {
    fmt.Printf("%s -> %s (type: %s -> %s)\n",
        b.Name, b.NameMapped, b.Type, b.TypeMapped)
}
```

#### Source Maps in Go

```go
result := api.MinifyWithOptions(source, api.MinifyOptions{
    MinifyIdentifiers: true,
    SourceMap: true,
    SourceMapOptions: api.SourceMapOptions{
        File:          "shader.min.wgsl",
        SourceName:    "shader.wgsl",
        IncludeSource: true,
    },
})

// result.SourceMap contains JSON string
// result.SourceMapDataURI contains data:application/json;base64,... for inline embedding
fmt.Println(result.SourceMap)
```

### C Library (FFI)

Build a static library for integration with C, Zig, Rust, or other languages:

```bash
make lib
# Creates: build/libminiray.a and build/libminiray.h
```

#### C Example

```c
#include <stdio.h>
#include <stdlib.h>
#include "libminiray.h"

int main() {
    const char* source =
        "struct Data { value: f32 }\n"
        "@group(0) @binding(0) var<uniform> data: Data;\n"
        "@fragment fn main() -> @location(0) vec4f {\n"
        "    return vec4f(data.value);\n"
        "}\n";

    char* code = NULL;
    int code_len = 0;
    char* json = NULL;
    int json_len = 0;

    // Minify with default options (pass NULL for options)
    int err = miniray_minify(
        (char*)source, strlen(source),
        NULL, 0,           // options_json, options_len
        &code, &code_len,
        &json, &json_len   // NULL to skip JSON stats
    );

    if (err == 0) {
        printf("Minified (%d bytes):\n%.*s\n", code_len, code_len, code);
    }

    // Free allocated memory
    miniray_free(code);
    miniray_free(json);

    return err;
}
```

Compile with:
```bash
gcc -o example example.c -L./build -lminiray -lpthread -lm
```

#### Custom Options

```c
// Options as JSON string
const char* options = "{\"minifyWhitespace\":true,\"minifyIdentifiers\":true}";

int err = miniray_minify(
    source, source_len,
    (char*)options, strlen(options),
    &code, &code_len,
    NULL, NULL  // Skip JSON result
);
```

#### Reflection

```c
char* json = NULL;
int json_len = 0;

int err = miniray_reflect(source, strlen(source), &json, &json_len);
if (err == 0) {
    // json contains: {"bindings":[...],"structs":{...},"entryPoints":[...]}
    printf("Reflection: %.*s\n", json_len, json);
}
miniray_free(json);
```

#### Combined Minify + Reflect

Get minified code with reflection that includes mapped (minified) names:

```c
char* code = NULL;
int code_len = 0;
char* json = NULL;
int json_len = 0;

int err = miniray_minify_and_reflect(
    source, strlen(source),
    NULL, 0,  // default options
    &code, &code_len,
    &json, &json_len
);

if (err == 0) {
    // code = minified WGSL
    // json = {"code":"...","reflect":{"bindings":[{"name":"data","nameMapped":"data",...}],...}}
}

miniray_free(code);
miniray_free(json);
```

#### Error Codes

| Code | Constant | Description |
|------|----------|-------------|
| 0 | `MINIRAY_OK` | Success |
| 1 | `MINIRAY_ERR_JSON_ENCODE` | Failed to encode JSON result |
| 2 | `MINIRAY_ERR_NULL_INPUT` | NULL pointer passed for required parameter |
| 3 | `MINIRAY_ERR_JSON_DECODE` | Failed to parse options JSON |

#### Zig Example

```zig
const c = @cImport({
    @cInclude("libminiray.h");
});

pub fn minify(source: []const u8) ![]const u8 {
    var code: [*c]u8 = null;
    var code_len: c_int = 0;

    const err = c.miniray_minify(
        @ptrCast(source.ptr), @intCast(source.len),
        null, 0,
        &code, &code_len,
        null, null
    );

    if (err != 0) return error.MinifyFailed;
    defer c.miniray_free(code);

    return code[0..@intCast(code_len)];
}
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
| `internal/reflect` | Shader reflection and memory layout computation |
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
