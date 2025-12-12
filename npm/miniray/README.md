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
  minifyWhitespace?: boolean;        // Remove whitespace (default: true)
  minifyIdentifiers?: boolean;       // Rename identifiers (default: true)
  minifySyntax?: boolean;            // Optimize syntax (default: true)
  mangleExternalBindings?: boolean;  // Mangle uniform/storage names (default: false)
  keepNames?: string[];              // Names to preserve from renaming
}

interface MinifyResult {
  code: string;           // Minified code
  errors: MinifyError[];  // Parse/minification errors
  originalSize: number;   // Input size in bytes
  minifiedSize: number;   // Output size in bytes
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

### `keepNames`

Array of identifier names that should not be renamed:

```javascript
minify(source, {
  minifyIdentifiers: true,
  keepNames: ['myHelper', 'computeValue'],
});
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
