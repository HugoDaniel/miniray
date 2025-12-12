// Package minifier provides the main minification API.
//
// It coordinates lexing, parsing, analysis, renaming, and printing
// to produce minified WGSL output.
package minifier

import (
	"codeberg.org/saruga/wgsl-minifier/internal/ast"
	"codeberg.org/saruga/wgsl-minifier/internal/parser"
	"codeberg.org/saruga/wgsl-minifier/internal/printer"
	"codeberg.org/saruga/wgsl-minifier/internal/renamer"
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

	// KeepNames prevents specific names from being renamed
	KeepNames []string
}

// DefaultOptions returns options for maximum minification.
func DefaultOptions() Options {
	return Options{
		MinifyWhitespace:  true,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		MangleProps:       false,
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
}

// Error represents a minification error.
type Error struct {
	Message string
	Line    int
	Column  int
}

// Stats provides minification statistics.
type Stats struct {
	OriginalSize  int
	MinifiedSize  int
	SymbolsTotal  int
	SymbolsRenamed int
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
	moduleResult := m.MinifyModule(module)
	result.Code = moduleResult.Code
	result.Stats = moduleResult.Stats
	result.Stats.OriginalSize = len(source)

	return result
}

// MinifyModule minifies a pre-parsed AST module.
func (m *Minifier) MinifyModule(module *ast.Module) Result {
	result := Result{}

	// Build reserved names set
	reserved := renamer.ComputeReservedNames()

	// Add user-specified keep names
	for _, name := range m.options.KeepNames {
		reserved[name] = true
	}

	// Mark API-facing symbols as non-renameable
	m.markAPIFacingSymbols(module)

	// Compute symbol usage before renaming
	uses := m.computeSymbolUsage(module)

	// Build external binding aliases - maps original ref to alias name
	// Only needed when NOT mangling external bindings (the default)
	var externalAliases []ExternalAlias
	if m.options.MinifyIdentifiers && !m.options.MangleExternalBindings {
		externalAliases = m.buildExternalAliases(module, uses, reserved)
	}

	// Create renamer
	var ren printer.Renamer
	if m.options.MinifyIdentifiers {
		minRenamer := renamer.NewMinifyRenamer(module.Symbols, reserved)
		minRenamer.AccumulateSymbolUseCounts(uses)
		minRenamer.AllocateSlots()
		minRenamer.AssignNames()
		ren = minRenamer
	} else {
		ren = renamer.NewNoOpRenamer(module.Symbols)
	}

	// Convert external aliases to printer format
	var printerAliases []printer.ExternalAlias
	for _, a := range externalAliases {
		printerAliases = append(printerAliases, printer.ExternalAlias{
			OriginalRef:  a.OriginalRef,
			OriginalName: a.OriginalName,
			AliasName:    a.AliasName,
		})
	}

	// Print
	p := printer.New(printer.Options{
		MinifyWhitespace:  m.options.MinifyWhitespace,
		MinifyIdentifiers: m.options.MinifyIdentifiers,
		MinifySyntax:      m.options.MinifySyntax,
		Renamer:           ren,
		ExternalAliases:   printerAliases,
	}, module.Symbols)

	result.Code = p.Print(module)
	result.Stats.MinifiedSize = len(result.Code)
	result.Stats.SymbolsTotal = len(module.Symbols)

	return result
}

// ExternalAlias represents an alias for an external binding.
type ExternalAlias struct {
	OriginalRef  ast.Ref
	OriginalName string
	AliasName    string
}

// buildExternalAliases creates short aliases for external bindings that are used.
func (m *Minifier) buildExternalAliases(module *ast.Module, uses map[ast.Ref]uint32, reserved map[string]bool) []ExternalAlias {
	var aliases []ExternalAlias

	// Find external bindings that are used
	type bindingWithCount struct {
		ref   ast.Ref
		name  string
		count uint32
	}

	var bindings []bindingWithCount
	for i := range module.Symbols {
		sym := &module.Symbols[i]
		if sym.Flags.Has(ast.IsExternalBinding) {
			ref := ast.Ref{InnerIndex: uint32(i)}
			if count, ok := uses[ref]; ok && count > 0 {
				bindings = append(bindings, bindingWithCount{
					ref:   ref,
					name:  sym.OriginalName,
					count: count,
				})
			}
		}
	}

	if len(bindings) == 0 {
		return nil
	}

	// Sort by usage count (most used first for shorter names)
	for i := 0; i < len(bindings)-1; i++ {
		for j := i + 1; j < len(bindings); j++ {
			if bindings[j].count > bindings[i].count {
				bindings[i], bindings[j] = bindings[j], bindings[i]
			}
		}
	}

	// Generate alias names
	nameGen := renamer.DefaultNameMinifier()
	nameIdx := 0
	for _, b := range bindings {
		// Find next available name
		var aliasName string
		for {
			aliasName = nameGen.NumberToMinifiedName(nameIdx)
			nameIdx++
			// Skip reserved names and the original name itself
			if !reserved[aliasName] && aliasName != b.name {
				break
			}
		}
		// Add to reserved to avoid conflicts
		reserved[aliasName] = true

		aliases = append(aliases, ExternalAlias{
			OriginalRef:  b.ref,
			OriginalName: b.name,
			AliasName:    aliasName,
		})
	}

	return aliases
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
// Convenience Functions
// ----------------------------------------------------------------------------

// Minify minifies WGSL source with default options.
func Minify(source string) Result {
	return New(DefaultOptions()).Minify(source)
}

// MinifyWithOptions minifies WGSL source with custom options.
func MinifyWithOptions(source string, options Options) Result {
	return New(options).Minify(source)
}
