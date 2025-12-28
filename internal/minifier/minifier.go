// Package minifier provides the main minification API.
//
// It coordinates lexing, parsing, analysis, renaming, and printing
// to produce minified WGSL output.
package minifier

import (
	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/dce"
	"github.com/HugoDaniel/miniray/internal/parser"
	"github.com/HugoDaniel/miniray/internal/printer"
	"github.com/HugoDaniel/miniray/internal/reflect"
	"github.com/HugoDaniel/miniray/internal/renamer"
	"github.com/HugoDaniel/miniray/internal/sourcemap"
)

// Options controls minification behavior.
type Options struct {
	// MinifyWhitespace removes unnecessary whitespace and newlines
	MinifyWhitespace bool

	// MinifyIdentifiers renames identifiers to shorter names
	MinifyIdentifiers bool

	// MinifySyntax applies syntax-level optimizations
	MinifySyntax bool

	// MangleProps renames struct member names (use with caution)
	MangleProps bool

	// MangleExternalBindings controls whether uniform/storage variable declarations
	// are renamed directly (true) or kept with original names and aliased (false).
	// Default is false - bindings keep original names for easier debugging.
	MangleExternalBindings bool

	// TreeShaking enables dead code elimination.
	// Removes declarations that are not reachable from entry points.
	TreeShaking bool

	// PreserveUniformStructTypes automatically preserves struct type names
	// that are used in var<uniform> or var<storage> declarations.
	// This is useful for frameworks like PNGine that detect builtin uniforms
	// by struct type name.
	PreserveUniformStructTypes bool

	// KeepNames prevents specific names from being renamed
	KeepNames []string

	// GenerateSourceMap enables source map generation
	GenerateSourceMap bool

	// SourceMapOptions configures source map output
	SourceMapOptions SourceMapOptions
}

// SourceMapOptions configures source map generation.
type SourceMapOptions struct {
	// File is the name of the generated file (for the "file" field)
	File string

	// SourceName is the name of the original source (for the "sources" array)
	SourceName string

	// IncludeSource embeds the original source in "sourcesContent"
	IncludeSource bool
}

// DefaultOptions returns options for maximum minification.
func DefaultOptions() Options {
	return Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		MangleProps:       false,
		TreeShaking:       true,
	}
}

// Result contains the minification output.
type Result struct {
	// Minified WGSL code
	Code string

	// Errors encountered during minification
	Errors []Error

	// Statistics about the minification
	Stats Stats

	// SourceMap is the generated source map (nil if not requested)
	SourceMap *sourcemap.SourceMap
}

// Error represents a minification error.
type Error struct {
	Message string
	Line    int
	Column  int
}

// Stats provides minification statistics.
type Stats struct {
	OriginalSize   int
	MinifiedSize   int
	SymbolsTotal   int
	SymbolsRenamed int
	SymbolsDead    int // Number of symbols removed by tree shaking
}

// Minifier performs WGSL minification.
type Minifier struct {
	options Options
}

// New creates a new minifier with the given options.
func New(options Options) *Minifier {
	return &Minifier{options: options}
}

// Minify minifies the given WGSL source code.
func (m *Minifier) Minify(source string) Result {
	result := Result{
		Stats: Stats{OriginalSize: len(source)},
	}

	// 1. Parse into AST
	p := parser.New(source)
	module, errs := p.Parse()

	// 2. Report parse errors
	if len(errs) > 0 {
		for _, err := range errs {
			result.Errors = append(result.Errors, Error{
				Message: err.Message,
				Line:    err.Line,
				Column:  err.Column,
			})
		}
		// Return original source on parse error
		result.Code = source
		result.Stats.MinifiedSize = len(source)
		return result
	}

	// 3. Minify the parsed module
	moduleResult := m.MinifyModuleWithSource(module, source)
	result.Code = moduleResult.Code
	result.Stats = moduleResult.Stats
	result.Stats.OriginalSize = len(source)
	result.SourceMap = moduleResult.SourceMap

	return result
}

// MinifyModule minifies a pre-parsed AST module.
// Note: Source map generation is not available without the original source.
// Use MinifyModuleWithSource for source map support.
func (m *Minifier) MinifyModule(module *ast.Module) Result {
	return m.MinifyModuleWithSource(module, "")
}

// MinifyModuleWithSource minifies a pre-parsed AST module with source map support.
func (m *Minifier) MinifyModuleWithSource(module *ast.Module, source string) Result {
	result := Result{}

	// Build reserved names set
	reserved := renamer.ComputeReservedNames()

	// Add user-specified keep names
	for _, name := range m.options.KeepNames {
		reserved[name] = true
	}

	// Mark API-facing symbols as non-renameable
	m.markAPIFacingSymbols(module)

	// Run dead code elimination if enabled
	if m.options.TreeShaking {
		result.Stats.SymbolsDead = dce.Mark(module)
	} else {
		// Mark all symbols as live if tree shaking is disabled
		for i := range module.Symbols {
			module.Symbols[i].Flags |= ast.IsLive
		}
	}

	// Compute symbol usage before renaming
	uses := m.computeSymbolUsage(module)

	// Create renamer
	var ren printer.Renamer
	if m.options.MinifyIdentifiers {
		minRenamer := renamer.NewMinifyRenamer(module.Symbols, reserved)
		minRenamer.AccumulateSymbolUseCounts(uses)
		minRenamer.AllocateSlots()
		minRenamer.ReserveUnrenamedSymbolNames() // Prevent conflicts with unrenamed symbols
		minRenamer.AssignNames()
		ren = minRenamer
	} else {
		ren = renamer.NewNoOpRenamer(module.Symbols)
	}

	// Create source map generator if enabled
	var sourceMapGen *sourcemap.Generator
	if m.options.GenerateSourceMap {
		sourceMapGen = sourcemap.NewGenerator(source)
		sourceMapGen.SetFile(m.options.SourceMapOptions.File)
		sourceMapGen.SetSourceName(m.options.SourceMapOptions.SourceName)
		sourceMapGen.IncludeSourceContent(m.options.SourceMapOptions.IncludeSource)
	}

	// Print
	p := printer.New(printer.Options{
		MinifyWhitespace:  m.options.MinifyWhitespace,
		MinifyIdentifiers: m.options.MinifyIdentifiers,
		MinifySyntax:      m.options.MinifySyntax,
		TreeShaking:       m.options.TreeShaking,
		Renamer:           ren,
		SourceMapGen:      sourceMapGen,
	}, module.Symbols)

	result.Code = p.Print(module)
	result.Stats.MinifiedSize = len(result.Code)
	result.Stats.SymbolsTotal = len(module.Symbols)

	// Generate source map if enabled
	if sourceMapGen != nil {
		result.SourceMap = sourceMapGen.Generate()
	}

	return result
}

// markAPIFacingSymbols marks symbols that cannot be renamed.
func (m *Minifier) markAPIFacingSymbols(module *ast.Module) {
	// Build a set of names to keep for quick lookup
	keepNamesSet := make(map[string]bool)
	for _, name := range m.options.KeepNames {
		keepNamesSet[name] = true
	}

	for i := range module.Symbols {
		sym := &module.Symbols[i]

		// Entry points must keep their names
		if sym.Flags.Has(ast.IsEntryPoint) {
			sym.Flags |= ast.MustNotBeRenamed
		}

		// Built-ins are not ours to rename
		if sym.Kind == ast.SymbolBuiltin {
			sym.Flags |= ast.MustNotBeRenamed
		}

		// Override constants are API-facing (unless using @id)
		// TODO: Check for @id attribute
		if sym.Kind == ast.SymbolOverride {
			sym.Flags |= ast.MustNotBeRenamed
		}

		// External bindings (uniform/storage) keep their original names
		// unless MangleExternalBindings is enabled
		if sym.Flags.Has(ast.IsExternalBinding) && !m.options.MangleExternalBindings {
			sym.Flags |= ast.MustNotBeRenamed
		}

		// User-specified names to keep
		if keepNamesSet[sym.OriginalName] {
			sym.Flags |= ast.MustNotBeRenamed
		}
	}

	// TODO: Mark struct members with @location, @builtin, @group, @binding

	// Preserve struct types used in uniform/storage declarations
	if m.options.PreserveUniformStructTypes {
		uniformStructRefs := m.collectUniformStructTypes(module)
		for ref := range uniformStructRefs {
			if ref.IsValid() && int(ref.InnerIndex) < len(module.Symbols) {
				module.Symbols[ref.InnerIndex].Flags |= ast.MustNotBeRenamed
			}
		}
	}
}

// collectUniformStructTypes finds all struct types used directly in
// var<uniform> or var<storage> declarations.
func (m *Minifier) collectUniformStructTypes(module *ast.Module) map[ast.Ref]bool {
	result := make(map[ast.Ref]bool)

	for _, decl := range module.Declarations {
		varDecl, ok := decl.(*ast.VarDecl)
		if !ok {
			continue
		}

		// Check if this variable has IsExternalBinding flag
		if !varDecl.Name.IsValid() {
			continue
		}
		sym := module.Symbols[varDecl.Name.InnerIndex]
		if !sym.Flags.Has(ast.IsExternalBinding) {
			continue
		}

		// Extract the type reference if it's a user-defined type
		if identType, ok := varDecl.Type.(*ast.IdentType); ok && identType.Ref.IsValid() {
			result[identType.Ref] = true
		}
	}

	return result
}

// computeSymbolUsage walks the AST and counts symbol references.
func (m *Minifier) computeSymbolUsage(module *ast.Module) map[ast.Ref]uint32 {
	uses := make(map[ast.Ref]uint32)

	// Walk declarations
	for _, decl := range module.Declarations {
		m.countDeclUsage(decl, uses)
	}

	return uses
}

func (m *Minifier) countDeclUsage(decl ast.Decl, uses map[ast.Ref]uint32) {
	switch d := decl.(type) {
	case *ast.ConstDecl:
		m.countExprUsage(d.Initializer, uses)

	case *ast.OverrideDecl:
		if d.Initializer != nil {
			m.countExprUsage(d.Initializer, uses)
		}

	case *ast.VarDecl:
		if d.Initializer != nil {
			m.countExprUsage(d.Initializer, uses)
		}

	case *ast.LetDecl:
		m.countExprUsage(d.Initializer, uses)

	case *ast.FunctionDecl:
		// Count the function name itself
		uses[d.Name]++

		// Count body
		if d.Body != nil {
			m.countStmtUsage(d.Body, uses)
		}
	}
}

func (m *Minifier) countExprUsage(expr ast.Expr, uses map[ast.Ref]uint32) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.IdentExpr:
		if e.Ref.IsValid() {
			uses[e.Ref]++
		}

	case *ast.BinaryExpr:
		m.countExprUsage(e.Left, uses)
		m.countExprUsage(e.Right, uses)

	case *ast.UnaryExpr:
		m.countExprUsage(e.Operand, uses)

	case *ast.CallExpr:
		m.countExprUsage(e.Func, uses)
		for _, arg := range e.Args {
			m.countExprUsage(arg, uses)
		}

	case *ast.IndexExpr:
		m.countExprUsage(e.Base, uses)
		m.countExprUsage(e.Index, uses)

	case *ast.MemberExpr:
		m.countExprUsage(e.Base, uses)

	case *ast.ParenExpr:
		m.countExprUsage(e.Expr, uses)
	}
}

func (m *Minifier) countStmtUsage(stmt ast.Stmt, uses map[ast.Ref]uint32) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.CompoundStmt:
		if s == nil {
			return
		}
		for _, inner := range s.Stmts {
			m.countStmtUsage(inner, uses)
		}

	case *ast.ReturnStmt:
		m.countExprUsage(s.Value, uses)

	case *ast.IfStmt:
		m.countExprUsage(s.Condition, uses)
		m.countStmtUsage(s.Body, uses)
		m.countStmtUsage(s.Else, uses)

	case *ast.SwitchStmt:
		m.countExprUsage(s.Expr, uses)
		for _, c := range s.Cases {
			for _, sel := range c.Selectors {
				m.countExprUsage(sel, uses)
			}
			m.countStmtUsage(c.Body, uses)
		}

	case *ast.ForStmt:
		m.countStmtUsage(s.Init, uses)
		m.countExprUsage(s.Condition, uses)
		m.countStmtUsage(s.Update, uses)
		m.countStmtUsage(s.Body, uses)

	case *ast.WhileStmt:
		m.countExprUsage(s.Condition, uses)
		m.countStmtUsage(s.Body, uses)

	case *ast.LoopStmt:
		m.countStmtUsage(s.Body, uses)
		m.countStmtUsage(s.Continuing, uses)

	case *ast.BreakIfStmt:
		m.countExprUsage(s.Condition, uses)

	case *ast.AssignStmt:
		m.countExprUsage(s.Left, uses)
		m.countExprUsage(s.Right, uses)

	case *ast.IncrDecrStmt:
		m.countExprUsage(s.Expr, uses)

	case *ast.CallStmt:
		m.countExprUsage(s.Call, uses)

	case *ast.DeclStmt:
		m.countDeclUsage(s.Decl, uses)
	}
}

// ----------------------------------------------------------------------------
// Combined Minify + Reflect
// ----------------------------------------------------------------------------

// MinifyAndReflectResult contains both minification and reflection output.
type MinifyAndReflectResult struct {
	// Minified result
	Result

	// Reflection information with mapped names
	Reflect reflect.ReflectResult
}

// MinifyAndReflect minifies the source and returns reflection info with mapped names.
// The reflection output uses the renamer from minification, so mapped names
// (NameMapped, TypeMapped, ElementTypeMapped) reflect the actual minified names.
func (m *Minifier) MinifyAndReflect(source string) MinifyAndReflectResult {
	result := MinifyAndReflectResult{
		Result: Result{
			Stats: Stats{OriginalSize: len(source)},
		},
	}

	// 1. Parse into AST
	p := parser.New(source)
	module, errs := p.Parse()

	// 2. Report parse errors
	if len(errs) > 0 {
		for _, err := range errs {
			result.Errors = append(result.Errors, Error{
				Message: err.Message,
				Line:    err.Line,
				Column:  err.Column,
			})
		}
		// Return original source on parse error
		result.Code = source
		result.Stats.MinifiedSize = len(source)
		result.Reflect = reflect.ReflectResult{
			Bindings:    []reflect.BindingInfo{},
			Structs:     make(map[string]reflect.StructLayout),
			EntryPoints: []reflect.EntryPointInfo{},
			Errors:      []string{errs[0].Message},
		}
		return result
	}

	// 3. Minify and get renamer
	minResult, ren := m.minifyModuleWithRenamer(module, source)
	result.Result = minResult
	result.Stats.OriginalSize = len(source) // Restore original size after assignment

	// 4. Reflect with the renamer for mapped names
	result.Reflect = reflect.ReflectModuleWithRenamer(module, ren)

	return result
}

// minifyModuleWithRenamer is like MinifyModuleWithSource but also returns the renamer.
func (m *Minifier) minifyModuleWithRenamer(module *ast.Module, source string) (Result, printer.Renamer) {
	result := Result{}

	// Build reserved names set
	reserved := renamer.ComputeReservedNames()

	// Add user-specified keep names
	for _, name := range m.options.KeepNames {
		reserved[name] = true
	}

	// Mark API-facing symbols as non-renameable
	m.markAPIFacingSymbols(module)

	// Run dead code elimination if enabled
	if m.options.TreeShaking {
		result.Stats.SymbolsDead = dce.Mark(module)
	} else {
		// Mark all symbols as live if tree shaking is disabled
		for i := range module.Symbols {
			module.Symbols[i].Flags |= ast.IsLive
		}
	}

	// Compute symbol usage before renaming
	uses := m.computeSymbolUsage(module)

	// Create renamer
	var ren printer.Renamer
	if m.options.MinifyIdentifiers {
		minRenamer := renamer.NewMinifyRenamer(module.Symbols, reserved)
		minRenamer.AccumulateSymbolUseCounts(uses)
		minRenamer.AllocateSlots()
		minRenamer.ReserveUnrenamedSymbolNames() // Prevent conflicts with unrenamed symbols
		minRenamer.AssignNames()
		ren = minRenamer
	} else {
		ren = renamer.NewNoOpRenamer(module.Symbols)
	}

	// Create source map generator if enabled
	var sourceMapGen *sourcemap.Generator
	if m.options.GenerateSourceMap {
		sourceMapGen = sourcemap.NewGenerator(source)
		sourceMapGen.SetFile(m.options.SourceMapOptions.File)
		sourceMapGen.SetSourceName(m.options.SourceMapOptions.SourceName)
		sourceMapGen.IncludeSourceContent(m.options.SourceMapOptions.IncludeSource)
	}

	// Print
	p := printer.New(printer.Options{
		MinifyWhitespace:  m.options.MinifyWhitespace,
		MinifyIdentifiers: m.options.MinifyIdentifiers,
		MinifySyntax:      m.options.MinifySyntax,
		TreeShaking:       m.options.TreeShaking,
		Renamer:           ren,
		SourceMapGen:      sourceMapGen,
	}, module.Symbols)

	result.Code = p.Print(module)
	result.Stats.MinifiedSize = len(result.Code)
	result.Stats.SymbolsTotal = len(module.Symbols)

	// Generate source map if enabled
	if sourceMapGen != nil {
		result.SourceMap = sourceMapGen.Generate()
	}

	return result, ren
}

// ----------------------------------------------------------------------------
// Convenience Functions
// ----------------------------------------------------------------------------

// Minify minifies WGSL source with optional custom options.
// If no options are provided, DefaultOptions() is used.
func Minify(source string, opts ...Options) Result {
	var options Options
	if len(opts) > 0 {
		options = opts[0]
	} else {
		options = DefaultOptions()
	}
	return New(options).Minify(source)
}
