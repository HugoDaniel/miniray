# Building with miniray - LLM Reference

Quick reference for AI agents integrating the miniray WGSL minifier.

## Decision Tree: Which Options?

```
Need smallest output?
├─ Yes: Full minification (default)
│   └─ Breaking reflection APIs?
│       ├─ OK: --mangle-external-bindings (extra ~5% reduction)
│       └─ No: Keep default (mangleExternalBindings: false)
└─ No: --no-mangle (whitespace + syntax only)

Framework detects uniforms by struct TYPE name?
├─ Yes: --preserve-uniform-struct-types
└─ No: Skip (struct types will be renamed)

Have platform-specific bindings/helpers?
├─ Yes: Use --keep-names or config file
└─ No: Default is fine
```

## API Surfaces

### CLI
```bash
miniray input.wgsl -o output.wgsl          # File to file
cat input.wgsl | miniray > output.wgsl     # Pipe
miniray --config myconfig.json input.wgsl  # With config
```

### Node.js/Browser (WASM)
```javascript
import { initialize, minify } from 'miniray';
await initialize();  // Required once, auto-finds WASM in Node
const result = minify(source, options);
// result: { code, errors[], originalSize, minifiedSize }
```

### Go
```go
import "github.com/HugoDaniel/miniray/pkg/api"
result := api.Minify(source)                    // Full minification
result := api.MinifyWhitespaceOnly(source)      // Safe mode
result := api.MinifyWithOptions(source, opts)   // Custom
```

## Options Reference

| Option | Default | Effect | When to Change |
|--------|---------|--------|----------------|
| `minifyWhitespace` | true | Remove spaces/newlines | Rarely disable |
| `minifyIdentifiers` | true | Rename locals/helpers | Disable for debugging |
| `minifySyntax` | true | `vec3<f32>`→`vec3f`, `.5`→`0.5` | Rarely disable |
| `treeShaking` | true | Remove unused code | Disable if DCE breaks code |
| `mangleExternalBindings` | false | Rename uniform/storage vars | Enable for max compression |
| `preserveUniformStructTypes` | false | Keep struct types in uniforms | Enable for PNGine-like frameworks |
| `keepNames` | [] | Preserve specific identifiers | Platform bindings, debugging |

## What Gets Preserved (Always)

- Entry points: `@vertex fn main()`, `@fragment fn draw()`, `@compute fn calc()`
- Builtin names: `position`, `vertex_index`, `global_invocation_id`
- Override constants: `override size: u32 = 64`
- Struct field names: `.time`, `.position` (accessed via `.`, not renamed)
- Binding indices: `@group(0) @binding(1)` numbers unchanged

## What Gets Renamed

- Local variables: `let myValue` → `let a`
- Function parameters: `fn calc(inputVal: f32)` → `fn calc(a: f32)`
- Helper functions: `fn computeNormal()` → `fn a()`
- Struct type names: `struct MyData` → `struct a` (unless preserved)
- Type aliases: `alias Vec3 = vec3<f32>` → `alias a = vec3f`

## Config File Format

Config auto-discovered as `wgslmin.json` or `.wgslminrc` in cwd or parents.

```json
{
  "minifyWhitespace": true,
  "minifyIdentifiers": true,
  "minifySyntax": true,
  "mangleExternalBindings": false,
  "treeShaking": true,
  "preserveUniformStructTypes": false,
  "keepNames": ["customHelper", "MyStructType"]
}
```

## Platform Presets

### compute.toys
```bash
miniray --config configs/compute.toys.json shader.wgsl
```
Preserves: `time`, `mouse`, `screen`, `pass_in/out`, `channel0/1`, samplers, type aliases, `main_image`

### PNGine
```bash
miniray --config configs/pngine.json shader.wgsl
```
Preserves uniform struct types + `PngineInputs`, `TimeInputs`, `CanvasInputs`, etc.

## Common Patterns

### Pattern 1: Build Pipeline Integration
```bash
# In build script
for f in src/shaders/*.wgsl; do
  miniray "$f" -o "dist/shaders/$(basename "$f" .wgsl).min.wgsl"
done
```

### Pattern 2: Runtime Minification (Browser)
```javascript
// Minify shader before compilation
const minified = minify(shaderSource, { minifyWhitespace: true });
if (minified.errors.length > 0) throw new Error(minified.errors[0].message);
const module = device.createShaderModule({ code: minified.code });
```

### Pattern 3: Preserve Reflection Compatibility
```javascript
// Default behavior - uniform names preserved for getPipelineLayout()
const result = minify(source); // mangleExternalBindings: false by default
// var<uniform> myUniforms stays as myUniforms in declaration
```

### Pattern 4: Maximum Compression
```javascript
const result = minify(source, {
  minifyWhitespace: true,
  minifyIdentifiers: true,
  minifySyntax: true,
  mangleExternalBindings: true,  // Breaks reflection
  treeShaking: true
});
```

### Pattern 5: Debug-Friendly Minification
```javascript
const result = minify(source, {
  minifyWhitespace: true,
  minifyIdentifiers: false,  // Keep readable names
  minifySyntax: true
});
```

## Gotchas & Edge Cases

### 1. Struct Fields Are Never Renamed
```wgsl
// Input
struct Data { value: f32 }
fn get(d: Data) -> f32 { return d.value; }

// Output (note: .value preserved, but 'd' renamed)
struct a{value:f32}fn b(c:a)->f32{return c.value;}
```
**Why**: WGSL accesses fields via `.member` syntax which cannot be safely renamed without whole-program analysis across shader/host boundary.

### 2. External Binding Aliasing
```wgsl
// Input
@group(0) @binding(0) var<uniform> uniforms: Data;
fn main() { return uniforms.x + uniforms.y; }

// Output (default: mangleExternalBindings=false)
@group(0) @binding(0) var<uniform> uniforms:a;let b=uniforms;fn main(){return b.x+b.y;}

// Output (mangleExternalBindings=true)
@group(0) @binding(0) var<uniform> a:b;fn main(){return a.x+a.y;}
```
**Why**: WebGPU reflection APIs use variable names. Default preserves them via alias indirection.

### 3. Tree Shaking Removes Unused Code
```wgsl
// Input
fn unused() -> f32 { return 1.0; }
fn used() -> f32 { return 2.0; }
@fragment fn main() -> @location(0) vec4f { return vec4f(used()); }

// Output (unused() removed entirely)
fn a()->f32{return 2.;}@fragment fn main()->@location(0) vec4f{return vec4f(a());}
```
**Disable with**: `--no-tree-shaking` if code is dynamically referenced.

### 4. preserveUniformStructTypes Only Preserves Direct Types
```wgsl
// Input
struct Inner { x: f32 }
struct Outer { inner: Inner }
@group(0) @binding(0) var<uniform> u: Outer;

// Output (Inner renamed, Outer preserved)
struct a{x:f32}struct Outer{inner:a}...
```
Only the struct type directly in `var<uniform>` declaration is preserved, not nested types.

### 5. Entry Points Must Have Attributes
```wgsl
// This function WON'T be treated as entry point (no attribute)
fn main() -> vec4f { return vec4f(0.0); }  // Gets renamed!

// This WILL be preserved
@fragment fn main() -> @location(0) vec4f { return vec4f(0.0); }
```

### 6. Override Constants Are Never Renamed
```wgsl
override workgroupSize: u32 = 64;  // Name preserved (API-facing)
const localSize: u32 = 32;         // Gets renamed (internal)
```

### 7. Syntax Optimization Shortens Numbers
```wgsl
// Input
let x = vec3<f32>(0.5, 1.0, 1000000.0);

// Output
let a=vec3f(.5,1.,1e6);
```
`vec3<f32>` → `vec3f`, `0.5` → `.5`, `1.0` → `1.`, `1000000.0` → `1e6`

## Error Handling

```javascript
const result = minify(source);
if (result.errors.length > 0) {
  // Parse error - result.code contains original source
  console.error(result.errors[0].message);
  // Use original source as fallback
}
```

Minifier returns original source on parse errors, never partial/corrupted output.

## Size Reduction Expectations

| Mode | Typical Reduction |
|------|-------------------|
| Whitespace only | 25-35% |
| Full (default) | 55-65% |
| Full + mangle bindings | 60-70% |

## Build Outputs

```bash
make build        # Native binary: build/miniray
make build-wasm   # WASM: build/miniray.wasm (~3.6MB)
make build-all    # All platforms + WASM
```

WASM requires `wasm_exec.js` from Go toolchain (included in npm package).
