# miniray

WGSL minifier for WebGPU shaders - WebAssembly build.

This package provides a WASM build of the [miniray](https://github.com/HugoDaniel/miniray) that runs in browsers and Node.js.

## Installation

```bash
npm install miniray
```

## Browser Usage

```html
<script src="node_modules/miniray/wasm_exec.js"></script>
<script src="node_modules/miniray/lib/browser.js"></script>
<script>
  (async () => {
    await miniray.initialize({ wasmURL: 'node_modules/miniray/miniray.wasm' });

    const result = miniray.minify(`
      @vertex fn main() -> @builtin(position) vec4f {
        return vec4f(0.0);
      }
    `);

    console.log(result.code);
    // "@vertex fn main()->@builtin(position) vec4f{return vec4f(0.0);}"
  })();
</script>
```

### ESM Usage

```javascript
import { initialize, minify } from 'miniray';

await initialize({ wasmURL: '/path/to/miniray.wasm' });

const result = minify(source, {
  minifyWhitespace: true,
  minifyIdentifiers: true,
  minifySyntax: true,
});
```

## Node.js Usage

```javascript
const { initialize, minify } = require('miniray');

await initialize(); // Automatically finds miniray.wasm

const result = minify(source);
console.log(result.code);
```

## API

### `initialize(options)`

Initialize the WASM module. Must be called before `minify()`.

```typescript
interface InitializeOptions {
  wasmURL?: string | URL;           // Path or URL to miniray.wasm
  wasmModule?: WebAssembly.Module;  // Pre-compiled module
}
```

### `minify(source, options?)`

Minify WGSL source code.

```typescript
interface MinifyOptions {
  minifyWhitespace?: boolean;           // Remove whitespace (default: true)
  minifyIdentifiers?: boolean;          // Rename identifiers (default: true)
  minifySyntax?: boolean;               // Optimize syntax (default: true)
  mangleExternalBindings?: boolean;     // Mangle uniform/storage names (default: false)
  treeShaking?: boolean;                // Remove unused declarations (default: true)
  preserveUniformStructTypes?: boolean; // Keep struct types used in uniforms (default: false)
  keepNames?: string[];                 // Names to preserve from renaming
  sourceMap?: boolean;                  // Generate source map (default: false)
  sourceMapSources?: boolean;           // Include source in sourcesContent (default: false)
}

interface MinifyResult {
  code: string;           // Minified code
  errors: MinifyError[];  // Parse/minification errors
  originalSize: number;   // Input size in bytes
  minifiedSize: number;   // Output size in bytes
  sourceMap?: string;     // Source map JSON (if sourceMap: true)
}
```

### `isInitialized()`

Returns `true` if the WASM module is initialized.

### `version`

The version of the minifier.

## Options

### `minifyWhitespace`

Remove unnecessary whitespace and newlines.

### `minifyIdentifiers`

Rename local variables, function parameters, and helper functions to shorter names. Entry points and API-facing declarations are preserved.

### `minifySyntax`

Apply syntax-level optimizations like numeric literal shortening.

### `mangleExternalBindings`

Control how `var<uniform>` and `var<storage>` names are handled:

- `false` (default): Original names are preserved in declarations, short aliases are used internally. This maintains compatibility with WebGPU's binding reflection APIs.
- `true`: Names are mangled directly for smaller output, but breaks binding reflection.

```javascript
// Input
const shader = `
@group(0) @binding(0) var<uniform> uniforms: f32;
fn getValue() -> f32 { return uniforms * 2.0; }
`;

// With mangleExternalBindings: false (default)
// Output: "@group(0) @binding(0) var<uniform> uniforms:f32;let a=uniforms;fn b()->f32{return a*2.0;}"

// With mangleExternalBindings: true
// Output: "@group(0) @binding(0) var<uniform> a:f32;fn b()->f32{return a*2.0;}"
```

### `treeShaking`

Enable dead code elimination to remove unused declarations (default: `true`):

```javascript
minify(source, {
  treeShaking: true,  // Remove unreachable code
});
```

### `preserveUniformStructTypes`

Automatically preserve struct type names that are used in `var<uniform>` or `var<storage>` declarations (default: `false`):

```javascript
// Input
const shader = `
struct MyUniforms { time: f32 }
@group(0) @binding(0) var<uniform> u: MyUniforms;
@fragment fn main() -> @location(0) vec4f { return vec4f(u.time); }
`;

// With preserveUniformStructTypes: false (default)
// struct MyUniforms -> struct a

// With preserveUniformStructTypes: true
// struct MyUniforms preserved
minify(source, {
  preserveUniformStructTypes: true,
});
```

This is particularly useful for frameworks like PNGine that detect builtin uniforms by struct type name.

### `keepNames`

Array of identifier names that should not be renamed:

```javascript
minify(source, {
  minifyIdentifiers: true,
  keepNames: ['myHelper', 'computeValue'],
});
```

### `sourceMap`

Generate a source map to debug minified shaders by mapping back to original source:

```javascript
const result = minify(source, {
  minifyIdentifiers: true,
  sourceMap: true,
});

console.log(result.code);
// "const a=42;fn b()->i32{return a;}"

console.log(result.sourceMap);
// '{"version":3,"sources":[],"names":["longVariable","myFunction"],"mappings":"MAAAA,..."}'
```

The source map follows the [Source Map v3 specification](https://sourcemaps.info/spec.html) and includes:
- `version`: Always 3
- `names`: Original names of renamed identifiers
- `mappings`: VLQ-encoded position mappings

### `sourceMapSources`

Include the original source code in the source map's `sourcesContent` field for self-contained debugging:

```javascript
const result = minify(source, {
  sourceMap: true,
  sourceMapSources: true,  // Embed original source
});

const map = JSON.parse(result.sourceMap);
console.log(map.sourcesContent);
// ["const longVariable = 42;\nfn myFunction() -> i32 { return longVariable; }"]
```

### Source Map Example: Complete Workflow

```javascript
import { initialize, minify } from 'miniray';

await initialize({ wasmURL: '/miniray.wasm' });

const source = `
const longVariableName = 42;

fn helperFunction(value: i32) -> i32 {
    return value * 2;
}

@compute @workgroup_size(1)
fn main() {
    let result = helperFunction(longVariableName);
}
`;

const result = minify(source, {
  minifyWhitespace: true,
  minifyIdentifiers: true,
  sourceMap: true,
  sourceMapSources: true,
});

console.log('Minified code:');
console.log(result.code);
// "const a=42;fn b(c:i32)->i32{return c*2;}@compute @workgroup_size(1) fn main(){let d=b(a);}"

console.log('\nSource map:');
const map = JSON.parse(result.sourceMap);
console.log('Names:', map.names);
// Names: ["longVariableName", "helperFunction", "value", "result"]

// To use the source map inline:
const codeWithSourceMap = result.code +
  '\n//# sourceMappingURL=data:application/json;base64,' +
  btoa(result.sourceMap);
```

## Using with Bundlers

### Vite

```javascript
import { initialize, minify } from 'miniray';
import wasmURL from 'miniray/miniray.wasm?url';

await initialize({ wasmURL });
```

### Webpack

```javascript
import { initialize, minify } from 'miniray';

// Configure webpack to handle .wasm files
await initialize({ wasmURL: new URL('miniray/miniray.wasm', import.meta.url) });
```

## Pre-compiling WASM

For better performance when creating multiple instances:

```javascript
const wasmModule = await WebAssembly.compileStreaming(
  fetch('/miniray.wasm')
);

// Share module across workers
await initialize({ wasmModule });
```

## Performance

- WASM binary: ~3.6MB
- Initialization: ~100ms (first load)
- Transform: <1ms for typical shaders

## License

MIT
