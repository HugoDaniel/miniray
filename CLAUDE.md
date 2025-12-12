# CLAUDE.md - wgsl-minifier

## Project Overview

A high-performance WGSL (WebGPU Shading Language) minifier written in Go, with WASM builds for browser/Node.js usage. Architecture inspired by esbuild.

## Quick Commands

```bash
# Build
make build              # Native binary to build/wgslmin
make build-wasm         # WASM to build/wgslmin.wasm
make build-all          # All platforms + WASM

# Test
go test ./...           # All tests
go test ./internal/minifier_tests/... -v  # Minifier tests with output
UPDATE_SNAPSHOTS=1 go test ./internal/minifier_tests/...  # Regenerate snapshots

# Run
./build/wgslmin shader.wgsl                    # Basic minification
./build/wgslmin --config configs/compute.toys.json shader.wgsl  # With config
echo 'fn main() {}' | ./build/wgslmin          # From stdin

# NPM package
cd npm/wgslmin-wasm && node test.js            # Run JS tests
cd npm/wgslmin-wasm && npm pack --dry-run      # Check package contents
```

## Architecture

```
Source → Lexer → Parser → AST → Minifier → Printer → Output
                           ↓
                       Renamer
```

### Package Structure

| Package | Purpose |
|---------|---------|
| `internal/ast` | AST nodes, Symbol table, Scope tree, Purity tracking |
| `internal/lexer` | Tokenizer with fast ASCII path |
| `internal/parser` | Two-pass parser (parse → visit/bind) |
| `internal/printer` | Code generator with minification |
| `internal/renamer` | Frequency-based identifier renaming |
| `internal/config` | JSON config file support |
| `internal/minifier` | Orchestrates the pipeline |
| `pkg/api` | Public Go API |
| `cmd/wgslmin` | CLI |
| `cmd/wgslmin-wasm` | WASM entry point |
| `npm/wgslmin-wasm` | NPM package |

### Key Design Decisions

**Symbol References (Ref)**
- Dual-index like esbuild: `{SourceIndex, InnerIndex}`
- `InvalidRef()` returns `{^uint32(0), ^uint32(0)}`
- **Critical**: Always use `Ref: ast.InvalidRef()` when creating AST nodes - Go zero-value `{0,0}` passes `IsValid()` check!

**Two-Pass Parser**
- Pass 1: Build AST, declare symbols with `UseCount: 0`
- Pass 2: Bind references, increment use counts, mark purity

**External Bindings**
- `@group/@binding` vars marked with `IsExternalBinding` flag
- Default: Create aliases (`let a = uniforms;`) to preserve API
- With `--mangle-external-bindings`: Rename directly

## Common Tasks

### Adding a New AST Node Type

1. Add type to `internal/ast/ast.go`
2. Add parsing in `internal/parser/parser.go`
3. Add printing in `internal/printer/printer.go`
4. Add to visit pass if it contains identifiers/types
5. Update snapshots: `UPDATE_SNAPSHOTS=1 go test ./internal/minifier_tests/...`

### Adding a New CLI Flag

1. Add flag parsing in `cmd/wgslmin/main.go`
2. Add to `internal/config/config.go` if it should be in config files
3. Add to `pkg/api/api.go` MinifyOptions
4. Wire through `internal/minifier/minifier.go`
5. Update README.md

### Debugging Type Renaming Issues

If types like `MyStruct` aren't being renamed:
1. Check `visitType()` is called for the type reference
2. Verify `lookupSymbol()` finds the struct
3. Ensure `printType()` uses `printName(typ.Ref)` when `Ref.IsValid()`

If built-in types like `f32` are being renamed:
1. Check IdentType creation uses `Ref: ast.InvalidRef()`
2. Go zero-value `{0,0}` passes `IsValid()` - this is the bug!

## Test Data

**compute.toys shaders** (`testdata/compute.toys/`):
- Real-world shaders verified working after minification
- Use with `--config configs/compute.toys.json`
- Size reductions: 55-71% typical

**Snapshot tests** (`internal/minifier_tests/snapshots/`):
- Golden file testing for minification output
- Regenerate with `UPDATE_SNAPSHOTS=1`

## WGSL Specifics

**Reserved Words**: 120+ words reserved for future use (see `internal/lexer/lexer.go`)

**Not Reserved** (common gotchas):
- `private`, `workgroup`, `uniform`, `storage` - address space keywords, not reserved
- `read`, `write`, `read_write` - access modes

**Nested Comments**: WGSL `/* */` comments nest (unlike C/JS)

**No Hoisting**: Declarations must precede use in text order

**No Recursion**: Functions cannot call themselves

## NPM Package

```javascript
// Node.js
const { initialize, minify } = require('wgslmin-wasm');
await initialize();
const result = minify(source, { minifyWhitespace: true, minifyIdentifiers: true });

// Browser
import { initialize, minify } from 'wgslmin-wasm';
await initialize({ wasmURL: '/wgslmin.wasm' });
```

**CLI**: `npx wgslmin shader.wgsl -o shader.min.wgsl`

## Files to Know

| File | Lines | What |
|------|-------|------|
| `internal/ast/ast.go` | ~1200 | All AST types, Symbol, Scope, Purity |
| `internal/parser/parser.go` | ~1600 | Parser + visitor pass |
| `internal/printer/printer.go` | ~1000 | Code generation |
| `internal/renamer/renamer.go` | ~370 | Name assignment algorithm |
| `internal/minifier/minifier.go` | ~450 | Pipeline orchestration |
| `configs/compute.toys.json` | ~60 | Example platform config |

## Gotchas

1. **Ref zero-value bug**: `Ref{0,0}` passes `IsValid()`. Always init with `ast.InvalidRef()`

2. **UseCount for renaming**: Only symbols with `UseCount > 0` get renamed

3. **Snapshot updates**: Tests fail if output changes. Use `UPDATE_SNAPSHOTS=1` to accept changes

4. **wasm_exec.js**: Must match Go version. Copy from `$(go env GOROOT)/misc/wasm/wasm_exec.js`

5. **keepNames for struct fields**: Not needed - fields accessed via `.` operator, not as identifiers
