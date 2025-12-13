// Package dce implements dead code elimination for WGSL modules.
//
// DCE works by:
// 1. Finding entry points (@vertex, @fragment, @compute functions)
// 2. Building a dependency graph of symbol references
// 3. Marking all symbols reachable from entry points as "live"
// 4. Filtering out declarations for non-live symbols during printing
package dce

import (
	"github.com/HugoDaniel/miniray/internal/ast"
)

// Mark performs dead code elimination on the module.
// It marks all symbols reachable from entry points with the IsLive flag.
// Returns the number of symbols marked as dead (not live).
func Mark(module *ast.Module) int {
	if module == nil || len(module.Symbols) == 0 {
		return 0
	}

	// Build dependency graph: for each symbol, which other symbols does it reference?
	deps := buildDependencyGraph(module)

	// Find entry points
	entryPoints := findEntryPoints(module)

	// If no entry points found, mark everything as live (conservative)
	if len(entryPoints) == 0 {
		for i := range module.Symbols {
			module.Symbols[i].Flags |= ast.IsLive
		}
		return 0
	}

	// Mark reachable symbols starting from entry points
	visited := make(map[uint32]bool)
	for _, ep := range entryPoints {
		markLive(ep, module.Symbols, deps, visited)
	}

	// Count dead symbols
	deadCount := 0
	for i := range module.Symbols {
		if !module.Symbols[i].Flags.Has(ast.IsLive) {
			deadCount++
		}
	}

	return deadCount
}

// buildDependencyGraph builds a map from symbol index to the symbols it references.
func buildDependencyGraph(module *ast.Module) map[uint32][]uint32 {
	deps := make(map[uint32][]uint32)

	for _, decl := range module.Declarations {
		collectDeclDeps(decl, deps)
	}

	return deps
}

// collectDeclDeps collects symbol dependencies from a declaration.
func collectDeclDeps(decl ast.Decl, deps map[uint32][]uint32) {
	switch d := decl.(type) {
	case *ast.ConstDecl:
		if d.Name.IsValid() {
			refs := collectExprRefs(d.Initializer)
			refs = append(refs, collectTypeRefs(d.Type)...)
			deps[d.Name.InnerIndex] = refs
		}

	case *ast.OverrideDecl:
		if d.Name.IsValid() {
			var refs []uint32
			if d.Initializer != nil {
				refs = collectExprRefs(d.Initializer)
			}
			refs = append(refs, collectTypeRefs(d.Type)...)
			deps[d.Name.InnerIndex] = refs
		}

	case *ast.VarDecl:
		if d.Name.IsValid() {
			var refs []uint32
			if d.Initializer != nil {
				refs = collectExprRefs(d.Initializer)
			}
			refs = append(refs, collectTypeRefs(d.Type)...)
			deps[d.Name.InnerIndex] = refs
		}

	case *ast.LetDecl:
		if d.Name.IsValid() {
			refs := collectExprRefs(d.Initializer)
			refs = append(refs, collectTypeRefs(d.Type)...)
			deps[d.Name.InnerIndex] = refs
		}

	case *ast.FunctionDecl:
		if d.Name.IsValid() {
			var refs []uint32
			// Collect from parameters
			for _, param := range d.Parameters {
				refs = append(refs, collectTypeRefs(param.Type)...)
			}
			// Collect from return type
			refs = append(refs, collectTypeRefs(d.ReturnType)...)
			// Collect from body
			if d.Body != nil {
				refs = append(refs, collectStmtRefs(d.Body)...)
			}
			deps[d.Name.InnerIndex] = refs
		}

	case *ast.StructDecl:
		if d.Name.IsValid() {
			var refs []uint32
			for _, member := range d.Members {
				refs = append(refs, collectTypeRefs(member.Type)...)
			}
			deps[d.Name.InnerIndex] = refs
		}

	case *ast.AliasDecl:
		if d.Name.IsValid() {
			refs := collectTypeRefs(d.Type)
			deps[d.Name.InnerIndex] = refs
		}
	}
}

// collectExprRefs collects symbol references from an expression.
func collectExprRefs(expr ast.Expr) []uint32 {
	if expr == nil {
		return nil
	}

	var refs []uint32

	switch e := expr.(type) {
	case *ast.IdentExpr:
		if e.Ref.IsValid() {
			refs = append(refs, e.Ref.InnerIndex)
		}

	case *ast.BinaryExpr:
		refs = append(refs, collectExprRefs(e.Left)...)
		refs = append(refs, collectExprRefs(e.Right)...)

	case *ast.UnaryExpr:
		refs = append(refs, collectExprRefs(e.Operand)...)

	case *ast.CallExpr:
		refs = append(refs, collectExprRefs(e.Func)...)
		for _, arg := range e.Args {
			refs = append(refs, collectExprRefs(arg)...)
		}

	case *ast.IndexExpr:
		refs = append(refs, collectExprRefs(e.Base)...)
		refs = append(refs, collectExprRefs(e.Index)...)

	case *ast.MemberExpr:
		refs = append(refs, collectExprRefs(e.Base)...)

	case *ast.ParenExpr:
		refs = append(refs, collectExprRefs(e.Expr)...)
	}

	return refs
}

// collectTypeRefs collects symbol references from a type.
func collectTypeRefs(typ ast.Type) []uint32 {
	if typ == nil {
		return nil
	}

	var refs []uint32

	switch t := typ.(type) {
	case *ast.IdentType:
		if t.Ref.IsValid() {
			refs = append(refs, t.Ref.InnerIndex)
		}

	case *ast.VecType:
		refs = append(refs, collectTypeRefs(t.ElemType)...)

	case *ast.MatType:
		refs = append(refs, collectTypeRefs(t.ElemType)...)

	case *ast.ArrayType:
		refs = append(refs, collectTypeRefs(t.ElemType)...)
		refs = append(refs, collectExprRefs(t.Size)...)

	case *ast.PtrType:
		refs = append(refs, collectTypeRefs(t.ElemType)...)

	case *ast.AtomicType:
		refs = append(refs, collectTypeRefs(t.ElemType)...)

	case *ast.TextureType:
		refs = append(refs, collectTypeRefs(t.SampledType)...)
	}

	return refs
}

// collectStmtRefs collects symbol references from a statement.
func collectStmtRefs(stmt ast.Stmt) []uint32 {
	if stmt == nil {
		return nil
	}

	var refs []uint32

	switch s := stmt.(type) {
	case *ast.CompoundStmt:
		for _, inner := range s.Stmts {
			refs = append(refs, collectStmtRefs(inner)...)
		}

	case *ast.ReturnStmt:
		refs = append(refs, collectExprRefs(s.Value)...)

	case *ast.IfStmt:
		refs = append(refs, collectExprRefs(s.Condition)...)
		refs = append(refs, collectStmtRefs(s.Body)...)
		refs = append(refs, collectStmtRefs(s.Else)...)

	case *ast.SwitchStmt:
		refs = append(refs, collectExprRefs(s.Expr)...)
		for _, c := range s.Cases {
			for _, sel := range c.Selectors {
				refs = append(refs, collectExprRefs(sel)...)
			}
			refs = append(refs, collectStmtRefs(c.Body)...)
		}

	case *ast.ForStmt:
		refs = append(refs, collectStmtRefs(s.Init)...)
		refs = append(refs, collectExprRefs(s.Condition)...)
		refs = append(refs, collectStmtRefs(s.Update)...)
		refs = append(refs, collectStmtRefs(s.Body)...)

	case *ast.WhileStmt:
		refs = append(refs, collectExprRefs(s.Condition)...)
		refs = append(refs, collectStmtRefs(s.Body)...)

	case *ast.LoopStmt:
		refs = append(refs, collectStmtRefs(s.Body)...)
		refs = append(refs, collectStmtRefs(s.Continuing)...)

	case *ast.BreakIfStmt:
		refs = append(refs, collectExprRefs(s.Condition)...)

	case *ast.AssignStmt:
		refs = append(refs, collectExprRefs(s.Left)...)
		refs = append(refs, collectExprRefs(s.Right)...)

	case *ast.IncrDecrStmt:
		refs = append(refs, collectExprRefs(s.Expr)...)

	case *ast.CallStmt:
		refs = append(refs, collectExprRefs(s.Call)...)

	case *ast.DeclStmt:
		// For local declarations, collect their references
		switch d := s.Decl.(type) {
		case *ast.ConstDecl:
			refs = append(refs, collectExprRefs(d.Initializer)...)
			refs = append(refs, collectTypeRefs(d.Type)...)
		case *ast.LetDecl:
			refs = append(refs, collectExprRefs(d.Initializer)...)
			refs = append(refs, collectTypeRefs(d.Type)...)
		case *ast.VarDecl:
			if d.Initializer != nil {
				refs = append(refs, collectExprRefs(d.Initializer)...)
			}
			refs = append(refs, collectTypeRefs(d.Type)...)
		}
	}

	return refs
}

// findEntryPoints finds all entry point function symbols.
func findEntryPoints(module *ast.Module) []uint32 {
	var entryPoints []uint32

	for i := range module.Symbols {
		if module.Symbols[i].Flags.Has(ast.IsEntryPoint) {
			entryPoints = append(entryPoints, uint32(i))
		}
	}

	return entryPoints
}

// markLive marks a symbol and all its dependencies as live.
func markLive(symbolIdx uint32, symbols []ast.Symbol, deps map[uint32][]uint32, visited map[uint32]bool) {
	// Already visited?
	if visited[symbolIdx] {
		return
	}
	visited[symbolIdx] = true

	// Mark as live
	if int(symbolIdx) < len(symbols) {
		symbols[symbolIdx].Flags |= ast.IsLive
	}

	// Recursively mark dependencies
	for _, depIdx := range deps[symbolIdx] {
		markLive(depIdx, symbols, deps, visited)
	}
}

// IsDeclarationLive returns true if the declaration should be included in output.
func IsDeclarationLive(decl ast.Decl, symbols []ast.Symbol) bool {
	var ref ast.Ref

	switch d := decl.(type) {
	case *ast.ConstDecl:
		ref = d.Name
	case *ast.OverrideDecl:
		ref = d.Name
	case *ast.VarDecl:
		ref = d.Name
	case *ast.LetDecl:
		ref = d.Name
	case *ast.FunctionDecl:
		ref = d.Name
	case *ast.StructDecl:
		ref = d.Name
	case *ast.AliasDecl:
		ref = d.Name
	case *ast.ConstAssertDecl:
		// const_assert is always kept
		return true
	default:
		return true
	}

	if !ref.IsValid() {
		return true
	}

	if int(ref.InnerIndex) >= len(symbols) {
		return true
	}

	return symbols[ref.InnerIndex].Flags.Has(ast.IsLive)
}
