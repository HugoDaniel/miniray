/**
 * miniray - WGSL Minifier for WebGPU Shaders (Node.js ESM Build)
 *
 * Usage:
 *   import { initialize, minify } from 'miniray'
 *   await initialize()
 *   const result = minify(source, { minifyWhitespace: true })
 */

import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { createRequire } from 'module';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const require = createRequire(import.meta.url);

let _initialized = false;
let _initPromise = null;
let _go = null;

/**
 * Initialize the WASM module.
 * @param {Object} options
 * @param {string} [options.wasmURL] - Path to miniray.wasm
 * @param {WebAssembly.Module} [options.wasmModule] - Pre-compiled module
 * @returns {Promise<void>}
 */
export async function initialize(options) {
  if (_initialized) {
    return;
  }
  if (_initPromise) {
    return _initPromise;
  }

  options = options || {};
  let wasmURL = options.wasmURL;
  const wasmModule = options.wasmModule;

  // Default to miniray.wasm in the package directory
  // Use require.resolve to find files within the package, which works
  // regardless of bundling or working directory
  if (!wasmURL && !wasmModule) {
    try {
      wasmURL = require.resolve('miniray/miniray.wasm');
    } catch {
      // Fallback to relative path for local development
      wasmURL = path.join(__dirname, '..', 'miniray.wasm');
    }
  }

  _initPromise = _doInitialize(wasmURL, wasmModule);

  try {
    await _initPromise;
    _initialized = true;
  } catch (err) {
    _initPromise = null;
    throw err;
  }
}

async function _doInitialize(wasmURL, wasmModule) {
  // Load wasm_exec.js - this defines Go globally
  // Use require.resolve to find files within the package, which works
  // regardless of bundling or working directory
  let wasmExecPath;
  try {
    wasmExecPath = require.resolve('miniray/wasm_exec.js');
  } catch {
    // Fallback to relative path for local development
    wasmExecPath = path.join(__dirname, '..', 'wasm_exec.js');
  }

  if (!fs.existsSync(wasmExecPath)) {
    throw new Error(`wasm_exec.js not found at ${wasmExecPath}`);
  }

  // Use require to load wasm_exec.js since it modifies globalThis
  require(wasmExecPath);

  if (typeof globalThis.Go === 'undefined') {
    throw new Error('Go runtime not found after loading wasm_exec.js');
  }

  _go = new globalThis.Go();

  let instance;
  if (wasmModule) {
    instance = await WebAssembly.instantiate(wasmModule, _go.importObject);
  } else {
    const wasmPath = wasmURL instanceof URL ? wasmURL.pathname : wasmURL;
    const wasmBuffer = fs.readFileSync(wasmPath);
    const result = await WebAssembly.instantiate(wasmBuffer, _go.importObject);
    instance = result.instance;
  }

  // Run the Go program
  _go.run(instance);

  // Wait for __miniray to be available
  await _waitForGlobal('__miniray', 1000);
}

function _waitForGlobal(name, timeout) {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const check = () => {
      if (typeof globalThis[name] !== 'undefined') {
        resolve();
      } else if (Date.now() - start > timeout) {
        reject(new Error(`Timeout waiting for ${name} to be defined`));
      } else {
        setTimeout(check, 10);
      }
    };
    check();
  });
}

/**
 * Minify WGSL source code.
 * @param {string} source - WGSL source code
 * @param {Object} [options] - Minification options
 * @returns {Object} Result
 */
export function minify(source, options) {
  if (!_initialized) {
    throw new Error('miniray not initialized. Call initialize() first.');
  }

  if (typeof source !== 'string') {
    throw new Error('source must be a string');
  }

  return globalThis.__miniray.minify(source, options || {});
}

/**
 * Reflect WGSL source to extract binding and struct information.
 * @param {string} source - WGSL source code
 * @returns {Object} Reflection result with bindings, structs, entryPoints, and errors
 */
export function reflect(source) {
  if (!_initialized) {
    throw new Error('miniray not initialized. Call initialize() first.');
  }

  if (typeof source !== 'string') {
    throw new Error('source must be a string');
  }

  return globalThis.__miniray.reflect(source);
}

/**
 * Check if initialized.
 * @returns {boolean}
 */
export function isInitialized() {
  return _initialized;
}

/**
 * Get version.
 * @returns {string}
 */
export function getVersion() {
  if (!_initialized) {
    return 'unknown';
  }
  return globalThis.__miniray.version;
}

export const version = getVersion;

// Default export
export default {
  initialize,
  minify,
  reflect,
  isInitialized,
  version: getVersion
};
