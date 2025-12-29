# miniray

A high-performance WGSL minifier, validator, and reflection tool.

**[Try the online demo](https://hugodaniel.com/pages/miniray/)**

```bash
# CLI
miniray shader.wgsl -o shader.min.wgsl

# npm
npm install miniray
```

## Quick Start

```javascript
import { initialize, minify, reflect, validate } from "miniray";

await initialize();

// Minify
const result = minify(source);
console.log(result.code); // Minified WGSL

// Validate
const validation = validate(source);
console.log(validation.valid); // true/false

// Reflect
const info = reflect(source);
console.log(info.bindings); // Uniform/storage bindings
console.log(info.entryPoints); // Entry point metadata
```

## Features

| Feature            | Description                                                    |
| ------------------ | -------------------------------------------------------------- |
| **Minification**   | Whitespace removal, identifier renaming, dead code elimination |
| **Validation**     | Type checking, symbol resolution, uniformity analysis          |
| **Reflection**     | Extract bindings, struct layouts, entry points                 |
| **Source Maps**    | Debug minified shaders with v3 source maps                     |
| **Multi-platform** | CLI, Go library, npm/WASM, C library (FFI)                     |
| **Well tested**    | >99% test coverage, validated against Dawn Tint test suite     |

## Installation

### CLI

```bash
go install github.com/HugoDaniel/miniray/cmd/miniray@latest
```

### npm (Browser/Node.js)

```bash
npm install miniray
```

### Go Library

```go
import "github.com/HugoDaniel/miniray/pkg/api"

result := api.Minify(source)
```

### C Library

```bash
make lib  # Creates build/libminiray.a
```

## CLI Usage

```bash
# Basic minification
miniray shader.wgsl -o shader.min.wgsl

# Validate shader
miniray validate shader.wgsl

# Extract reflection data
miniray reflect shader.wgsl

# Whitespace-only (safest)
miniray --no-mangle shader.wgsl

# With source map
miniray --source-map shader.wgsl -o shader.min.wgsl
```

### CLI Options

| Flag                         | Description                          |
| ---------------------------- | ------------------------------------ |
| `-o <file>`                  | Output file (default: stdout)        |
| `--no-mangle`                | Don't rename identifiers             |
| `--mangle-external-bindings` | Rename uniform/storage vars directly |
| `--keep-names <names>`       | Preserve specific names              |
| `--no-tree-shaking`          | Keep unused declarations             |
| `--source-map`               | Generate source map                  |
| `--config <file>`            | Use config file                      |

### Subcommands

```bash
# Validate - check for errors without minifying
miniray validate shader.wgsl
miniray validate --json shader.wgsl
miniray validate --strict shader.wgsl  # Warnings as errors

# Reflect - extract binding/struct info as JSON
miniray reflect shader.wgsl
miniray reflect --compact shader.wgsl
```

## What Gets Preserved

| Always Preserved                                       | Minified            |
| ------------------------------------------------------ | ------------------- |
| Entry point names (`@vertex`, `@fragment`, `@compute`) | Local variables     |
| `@builtin` names                                       | Function parameters |
| `@location` members                                    | Helper functions    |
| `@group`/`@binding` indices                            | Private structs     |
| `override` names                                       | Type aliases        |
| Uniform/storage var names*                             |                     |

*Use `--mangle-external-bindings` to also minify uniform/storage names.

## JavaScript/TypeScript API

```javascript
import {
  initialize,
  minify,
  minifyAndReflect,
  reflect,
  validate,
} from "miniray";

await initialize({ wasmURL: "/miniray.wasm" });

// Minify with options
const result = minify(source, {
  minifyWhitespace: true,
  minifyIdentifiers: true,
  minifySyntax: true,
  treeShaking: true,
  keepNames: ["myHelper"],
});

// Validate (strictMode treats warnings as errors)
const validation = validate(source, { strictMode: true });
if (!validation.valid) {
  for (const d of validation.diagnostics) {
    console.log(`${d.line}:${d.column}: ${d.message}`);
  }
}

// Reflect
const info = reflect(source);
for (const b of info.bindings) {
  console.log(`@group(${b.group}) @binding(${b.binding}) ${b.name}: ${b.type}`);
}

// Combined minify + reflect (with minified names)
const combined = minifyAndReflect(source);
console.log(combined.code);
console.log(combined.reflect.bindings);
```

See [npm/miniray/README.md](npm/miniray/README.md) for full API documentation.

## Go API

```go
import "github.com/HugoDaniel/miniray/pkg/api"

// Minify
result := api.Minify(source)
fmt.Println(result.Code)

// With options
result := api.MinifyWithOptions(source, api.MinifyOptions{
    MinifyIdentifiers: true,
    TreeShaking:       true,
    KeepNames:         []string{"myHelper"},
})

// Validate
val := api.Validate(source)
if !val.Valid {
    for _, d := range val.Diagnostics {
        fmt.Printf("%d:%d: %s\n", d.Line, d.Column, d.Message)
    }
}

// Reflect
info := api.Reflect(source)
for _, b := range info.Bindings {
    fmt.Printf("@group(%d) @binding(%d) %s\n", b.Group, b.Binding, b.Name)
}
```

## C API

Build with `make lib` to get `libminiray.a` and `libminiray.h`.

```c
#include "libminiray.h"

char* code = NULL;
int code_len = 0;

int err = miniray_minify(source, strlen(source), NULL, 0, &code, &code_len, NULL, NULL);
if (err == 0) {
    printf("Minified: %.*s\n", code_len, code);
}
miniray_free(code);
```

See [docs/C-API.md](docs/C-API.md) for full C API documentation.

## Config File

Create `miniray.json` in your project:

```json
{
  "minifyWhitespace": true,
  "minifyIdentifiers": true,
  "minifySyntax": true,
  "treeShaking": true,
  "keepNames": ["myUniform"]
}
```

Pre-built configs available in `configs/`:

- `compute.toys.json` - For [compute.toys](https://compute.toys) shaders
- `pngine.json` - For [PNGine](https://github.com/HugoDaniel/pngine)

## Source Maps

```bash
miniray --source-map shader.wgsl -o shader.min.wgsl
# Creates shader.min.wgsl and shader.min.wgsl.map
```

```javascript
const result = minify(source, { sourceMap: true, sourceMapSources: true });
// result.sourceMap contains v3 source map JSON
```

## Development

```bash
make build      # Build CLI
make build-wasm # Build WASM
make lib        # Build C library
make test       # Run tests
```

## Documentation

- [Why minify WGSL?](docs/why-minify-wgsl.md) - Benefits of shader minification
- [npm package docs](npm/miniray/README.md) - JavaScript/TypeScript API
- [C API docs](docs/C-API.md) - C library reference
- [Building with miniray](BUILDING_WITH_MINIRAY.md) - Integration guide
- [Tint tests](docs/tint-test-import-plan.md) - Running Dawn Tint test suite

## License

CC0 Public Domain - See [LICENSE](LICENSE)
