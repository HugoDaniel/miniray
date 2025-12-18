/**
 * miniray - WGSL Minifier for WebGPU Shaders (Node.js Build)
 *
 * Usage:
 *   const { initialize, minify } = require('miniray')
 *   await initialize()
 *   const result = minify(source, { minifyWhitespace: true })
 */

'use strict';

const fs = require('fs');
const path = require('path');

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
async function initialize(options) {
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
  // Set up globals required by wasm_exec.js
  globalThis.require = require;
  globalThis.fs = require('fs');
  globalThis.path = require('path');
  globalThis.TextEncoder = require('util').TextEncoder;
  globalThis.TextDecoder = require('util').TextDecoder;
  globalThis.performance ??= require('perf_hooks').performance;
  globalThis.crypto ??= require('crypto');

  // Load wasm_exec.js to set up Go runtime
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
  require(wasmExecPath);

  if (typeof Go === 'undefined') {
    throw new Error('Go runtime not found after loading wasm_exec.js');
  }

  _go = new Go();

  let instance;
  if (wasmModule) {
    instance = await WebAssembly.instantiate(wasmModule, _go.importObject);
  } else {
    const wasmPath = wasmURL instanceof URL ? wasmURL.pathname : wasmURL;
    const wasmBuffer = fs.readFileSync(wasmPath);
    const result = await WebAssembly.instantiate(wasmBuffer, _go.importObject);
    instance = result.instance;
  }

  // Run the Go program (don't await - it runs in background)
  _go.run(instance);

  // Wait for __miniray to be available
  await _waitForGlobal('__miniray', 1000);
}

function _waitForGlobal(name, timeout) {
  return new Promise((resolve, reject) => {
    const start = Date.now();
    const check = () => {
      if (typeof global[name] !== 'undefined') {
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
function minify(source, options) {
  if (!_initialized) {
    throw new Error('miniray not initialized. Call initialize() first.');
  }

  if (typeof source !== 'string') {
    throw new Error('source must be a string');
  }

  return global.__miniray.minify(source, options || {});
}

/**
 * Reflect WGSL source to extract binding and struct information.
 * @param {string} source - WGSL source code
 * @returns {Object} Reflection result with bindings, structs, entryPoints, and errors
 */
function reflect(source) {
  if (!_initialized) {
    throw new Error('miniray not initialized. Call initialize() first.');
  }

  if (typeof source !== 'string') {
    throw new Error('source must be a string');
  }

  return global.__miniray.reflect(source);
}

/**
 * Check if initialized.
 * @returns {boolean}
 */
function isInitialized() {
  return _initialized;
}

/**
 * Get version.
 * @returns {string}
 */
function getVersion() {
  if (!_initialized) {
    return 'unknown';
  }
  return global.__miniray.version;
}

module.exports = {
  initialize,
  minify,
  reflect,
  isInitialized,
  get version() { return getVersion(); }
};
