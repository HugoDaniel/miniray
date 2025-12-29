# Miniray C API

A C-callable static library for WGSL minification, reflection, and validation.

## Building

```bash
make lib
# Produces: build/libminiray.a and build/libminiray.h
```

Or manually:
```bash
CGO_ENABLED=1 go build -buildmode=c-archive -o build/libminiray.a ./cmd/miniray-lib
```

## Linking

### C/C++
```bash
gcc -o myprogram myprogram.c -L./build -lminiray -lpthread
```

### Zig
```zig
const miniray = @cImport(@cInclude("libminiray.h"));
// Link with: -lminiray
```

### Rust
```rust
// In build.rs:
println!("cargo:rustc-link-lib=static=miniray");
println!("cargo:rustc-link-search=native=path/to/build");
```

## API Reference

### Error Codes

| Code | Name | Description |
|------|------|-------------|
| 0 | `MINIRAY_OK` | Success |
| 1 | `MINIRAY_ERR_JSON_ENCODE` | Failed to encode JSON result |
| 2 | `MINIRAY_ERR_NULL_INPUT` | Required parameter was NULL |
| 3 | `MINIRAY_ERR_JSON_DECODE` | Failed to decode options JSON |

### Functions

#### `miniray_minify`

Minifies WGSL source code.

```c
int miniray_minify(
    char* source,        // WGSL source code (UTF-8)
    int source_len,      // Length in bytes
    char* options_json,  // JSON options (NULL for defaults)
    int options_len,     // Options length
    char** out_code,     // Receives minified code (must free)
    int* out_code_len,   // Receives code length
    char** out_json,     // Receives JSON stats (must free, can be NULL)
    int* out_json_len    // Receives JSON length (can be NULL)
);
```

**Options JSON:**
```json
{
    "minifyWhitespace": true,
    "minifyIdentifiers": true,
    "minifySyntax": true,
    "mangleExternalBindings": false,
    "treeShaking": true,
    "keepNames": ["uniformName"]
}
```

**Result JSON:**
```json
{
    "code": "minified code...",
    "errors": [],
    "originalSize": 150,
    "minifiedSize": 80
}
```

#### `miniray_reflect`

Extracts binding and struct layout information from WGSL.

```c
int miniray_reflect(
    char* source,      // WGSL source code (UTF-8)
    int source_len,    // Length in bytes
    char** out_json,   // Receives JSON result (must free)
    int* out_len       // Receives JSON length
);
```

**Result JSON:**
```json
{
    "bindings": [{
        "group": 0,
        "binding": 0,
        "name": "uniforms",
        "addressSpace": "uniform",
        "type": "MyStruct",
        "layout": {
            "size": 32,
            "alignment": 16,
            "fields": [...]
        }
    }],
    "structs": {...},
    "entryPoints": [{
        "name": "main",
        "stage": "compute",
        "workgroupSize": [8, 8, 1]
    }],
    "errors": []
}
```

#### `miniray_minify_and_reflect`

Combines minification and reflection. The reflection data includes mapped names after minification.

```c
int miniray_minify_and_reflect(
    char* source,        // WGSL source code (UTF-8)
    int source_len,      // Length in bytes
    char* options_json,  // JSON options (NULL for defaults)
    int options_len,     // Options length
    char** out_code,     // Receives minified code (must free)
    int* out_code_len,   // Receives code length
    char** out_json,     // Receives JSON result (must free)
    int* out_json_len    // Receives JSON length
);
```

#### `miniray_validate`

Validates WGSL source code for semantic errors.

```c
int miniray_validate(
    char* source,        // WGSL source code (UTF-8)
    int source_len,      // Length in bytes
    char* options_json,  // JSON options (NULL for defaults)
    int options_len,     // Options length
    char** out_json,     // Receives JSON result (must free)
    int* out_json_len    // Receives JSON length
);
```

**Options JSON:**
```json
{
    "strictMode": false,
    "diagnosticFilters": {
        "derivative_uniformity": "off"
    }
}
```

**Result JSON:**
```json
{
    "valid": false,
    "diagnostics": [{
        "severity": "error",
        "code": "E0200",
        "message": "cannot return 'i32' from function returning 'f32'",
        "line": 4,
        "column": 5
    }],
    "errorCount": 1,
    "warningCount": 0
}
```

#### `miniray_free`

Frees memory allocated by miniray functions.

```c
void miniray_free(char* ptr);
```

**Important:** Always call this to free `out_code` and `out_json` pointers.

#### `miniray_version`

Returns the library version string.

```c
char* miniray_version(void);
```

**Note:** The returned pointer is static and must NOT be freed.

## Complete Example (C)

```c
#include <stdio.h>
#include <string.h>
#include "libminiray.h"

int main() {
    const char* source =
        "@vertex fn main() -> @builtin(position) vec4f {\n"
        "    return vec4f(0.0, 0.0, 0.0, 1.0);\n"
        "}\n";

    char* code = NULL;
    int code_len = 0;
    char* json = NULL;
    int json_len = 0;

    // Minify with default options
    int result = miniray_minify(
        (char*)source, strlen(source),
        NULL, 0,  // Default options
        &code, &code_len,
        &json, &json_len
    );

    if (result == 0) {
        printf("Minified (%d bytes):\n%.*s\n", code_len, code_len, code);
        printf("Stats: %.*s\n", json_len, json);
    } else {
        printf("Error: %d\n", result);
    }

    // Free allocated memory
    miniray_free(code);
    miniray_free(json);

    // Print version
    printf("Version: %s\n", miniray_version());

    return result;
}
```

## Complete Example (Zig)

```zig
const std = @import("std");
const miniray = @cImport(@cInclude("libminiray.h"));

pub fn main() !void {
    const source =
        \\@vertex fn main() -> @builtin(position) vec4f {
        \\    return vec4f(0.0, 0.0, 0.0, 1.0);
        \\}
    ;

    var code: [*c]u8 = null;
    var code_len: c_int = 0;
    var json: [*c]u8 = null;
    var json_len: c_int = 0;

    const result = miniray.miniray_minify(
        @constCast(source.ptr),
        @intCast(source.len),
        null, 0,
        &code, &code_len,
        &json, &json_len,
    );

    if (result == 0) {
        std.debug.print("Minified: {s}\n", .{code[0..@intCast(code_len)]});
    }

    miniray.miniray_free(code);
    miniray.miniray_free(json);
}
```

## Thread Safety

All miniray functions are thread-safe and can be called concurrently from multiple threads.

## Memory Management

- All `out_*` pointers that receive data must be freed with `miniray_free()`
- The pointer from `miniray_version()` must NOT be freed
- Input pointers (`source`, `options_json`) are not modified and not freed by miniray
