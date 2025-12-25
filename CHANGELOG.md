# Changelog

All notable changes to miniray will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.1] - 2025-12-25

### Fixed

- Show `reflect` subcommand in main `--help` output

## [0.2.0] - 2025-12-25

### Added

- **Shader Reflection API** - Extract binding information, struct memory layouts, and entry points from WGSL shaders
  - CLI: `miniray reflect shader.wgsl` subcommand with `-o` and `--compact` options
  - Go API: `api.Reflect(source)` function returning `ReflectResult`
  - JavaScript/WASM: `reflect(source)` function with full TypeScript definitions
  - Computes WGSL-spec compliant memory layouts (vec3 alignment quirks, struct padding)
  - Returns bindings with group/binding indices, address space, access mode, and type
  - Returns entry points with stage (vertex/fragment/compute) and workgroup size
  - Returns all struct definitions with size, alignment, and field layouts

- **Source Map Generation** - Debug minified shaders with source position mapping
  - `--source-map` flag generates external `.map` file
  - `--source-map-inline` embeds source map as data URI comment
  - `--source-map-sources` includes original source content
  - Maps renamed identifiers back to original names
  - Uses UTF-16 columns matching WebGPU's `GPUCompilationMessage` format

- **Trailing comma support** in function parameters

### Fixed

- Trailing comma in function parameter lists now parses correctly

## [0.1.4] - 2025-12-18

### Added

- `preserveUniformStructTypes` option to preserve struct types used in uniform/storage declarations
- PNGine platform configuration preset (`configs/pngine.json`)

## [0.1.3] - 2025-12-15

### Added

- Tree shaking (dead code elimination) - enabled by default
- `--no-tree-shaking` flag to disable

## [0.1.2] - 2025-12-10

### Added

- compute.toys platform configuration preset
- Config file support (miniray.json, .minirayrc)

## [0.1.1] - 2025-12-05

### Added

- NPM package with WASM build for browser/Node.js
- TypeScript type definitions

## [0.1.0] - 2025-12-01

### Added

- Initial release
- WGSL lexer with Unicode XID support
- Recursive descent parser with full WGSL grammar
- Whitespace minification
- Identifier renaming (frequency-based)
- Syntax optimizations (numeric literals, vector constructors)
- External binding preservation (default) with optional mangling
- `--keep-names` flag for preserving specific identifiers
