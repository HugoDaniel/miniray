/**
 * Options for WGSL minification.
 */
export interface MinifyOptions {
  /**
   * Remove unnecessary whitespace and newlines.
   * @default true
   */
  minifyWhitespace?: boolean;

  /**
   * Rename identifiers to shorter names.
   * Entry points and API-facing declarations are preserved.
   * @default true
   */
  minifyIdentifiers?: boolean;

  /**
   * Apply syntax-level optimizations (numeric literals, etc).
   * @default true
   */
  minifySyntax?: boolean;

  /**
   * Rename uniform/storage variable declarations directly.
   * When false (default), original names are preserved and short aliases are used.
   * Set to true only if you don't use WebGPU's binding reflection APIs.
   * @default false
   */
  mangleExternalBindings?: boolean;

  /**
   * Enable dead code elimination to remove unused declarations.
   * @default true
   */
  treeShaking?: boolean;

  /**
   * Automatically preserve struct type names that are used in
   * var<uniform> or var<storage> declarations.
   * Useful for frameworks that detect uniforms by struct type name.
   * @default false
   */
  preserveUniformStructTypes?: boolean;

  /**
   * Identifier names that should not be renamed.
   */
  keepNames?: string[];
}

/**
 * Error information from minification.
 */
export interface MinifyError {
  /** Error message */
  message: string;
  /** Line number (1-indexed, 0 if unknown) */
  line: number;
  /** Column number (1-indexed, 0 if unknown) */
  column: number;
}

/**
 * Result of minification.
 */
export interface MinifyResult {
  /** Minified WGSL code */
  code: string;
  /** Errors encountered during minification */
  errors: MinifyError[];
  /** Size of input in bytes */
  originalSize: number;
  /** Size of output in bytes */
  minifiedSize: number;
}

/**
 * Options for initializing the WASM module.
 */
export interface InitializeOptions {
  /**
   * URL or path to the wgslmin.wasm file.
   * Required unless wasmModule is provided.
   */
  wasmURL?: string | URL;

  /**
   * Pre-compiled WebAssembly.Module.
   * Use this to share a module across multiple instances.
   */
  wasmModule?: WebAssembly.Module;
}

/**
 * Initialize the WASM module. Must be called before minify().
 * @param options - Initialization options
 */
export function initialize(options: InitializeOptions): Promise<void>;

/**
 * Minify WGSL source code.
 * @param source - WGSL source code to minify
 * @param options - Minification options (defaults to full minification)
 * @returns Minification result
 */
export function minify(source: string, options?: MinifyOptions): MinifyResult;

/**
 * Check if the WASM module is initialized.
 */
export function isInitialized(): boolean;

/**
 * Get the version of the minifier.
 */
export const version: string;
