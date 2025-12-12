/**
 * miniray - WGSL Minifier for WebGPU Shaders (Browser Build)
 *
 * Usage:
 *   import { initialize, minify } from 'miniray'
 *   await initialize({ wasmURL: '/miniray.wasm' })
 *   const result = minify(source, { minifyWhitespace: true })
 */

(function (root, factory) {
  if (typeof define === 'function' && define.amd) {
    define([], factory);
  } else if (typeof module === 'object' && module.exports) {
    module.exports = factory();
  } else {
    root.miniray = factory();
  }
}(typeof self !== 'undefined' ? self : this, function () {
  'use strict';

  let _initialized = false;
  let _initPromise = null;
  let _go = null;

  /**
   * Initialize the WASM module.
   * @param {Object} options
   * @param {string|URL} [options.wasmURL] - URL to miniray.wasm
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
    const wasmURL = options.wasmURL;
    const wasmModule = options.wasmModule;

    if (!wasmURL && !wasmModule) {
      throw new Error('Must provide either wasmURL or wasmModule');
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
    // Load wasm_exec.js if Go is not defined
    if (typeof Go === 'undefined') {
      throw new Error(
        'Go runtime not found. Make sure to include wasm_exec.js before using miniray:\n' +
        '<script src="wasm_exec.js"></script>'
      );
    }

    _go = new Go();

    let instance;
    if (wasmModule) {
      // Use pre-compiled module
      instance = await WebAssembly.instantiate(wasmModule, _go.importObject);
    } else {
      // Fetch and instantiate
      const url = wasmURL instanceof URL ? wasmURL.href : wasmURL;

      if (typeof WebAssembly.instantiateStreaming === 'function') {
        try {
          const response = await fetch(url);
          if (!response.ok) {
            throw new Error(`Failed to fetch ${url}: ${response.status}`);
          }
          const result = await WebAssembly.instantiateStreaming(response, _go.importObject);
          instance = result.instance;
        } catch (err) {
          // Fall back to arrayBuffer if streaming fails (e.g., wrong MIME type)
          if (err.message && err.message.includes('MIME')) {
            const response = await fetch(url);
            const bytes = await response.arrayBuffer();
            const result = await WebAssembly.instantiate(bytes, _go.importObject);
            instance = result.instance;
          } else {
            throw err;
          }
        }
      } else {
        // Fallback for older browsers
        const response = await fetch(url);
        const bytes = await response.arrayBuffer();
        const result = await WebAssembly.instantiate(bytes, _go.importObject);
        instance = result.instance;
      }
    }

    // Run the Go program (this sets up __miniray global)
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
   * @param {boolean} [options.minifyWhitespace=true] - Remove whitespace
   * @param {boolean} [options.minifyIdentifiers=true] - Rename identifiers
   * @param {boolean} [options.minifySyntax=true] - Optimize syntax
   * @param {boolean} [options.mangleExternalBindings=false] - Mangle uniform/storage names
   * @param {string[]} [options.keepNames] - Names to preserve
   * @returns {Object} Result with code, errors, originalSize, minifiedSize
   */
  function minify(source, options) {
    if (!_initialized) {
      throw new Error('miniray not initialized. Call initialize() first.');
    }

    if (typeof source !== 'string') {
      throw new Error('source must be a string');
    }

    return globalThis.__miniray.minify(source, options || {});
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
    return globalThis.__miniray.version;
  }

  return {
    initialize: initialize,
    minify: minify,
    isInitialized: isInitialized,
    get version() { return getVersion(); }
  };
}));
