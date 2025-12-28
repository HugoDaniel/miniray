package dce

import (
	"testing"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/parser"
)

// ----------------------------------------------------------------------------
// Mark Tests
// ----------------------------------------------------------------------------

func TestMark_NilModule(t *testing.T) {
	dead := Mark(nil)
	if dead != 0 {
		t.Errorf("expected 0 dead symbols for nil module, got %d", dead)
	}
}

func TestMark_EmptyModule(t *testing.T) {
	module := &ast.Module{
		Symbols: []ast.Symbol{},
	}
	dead := Mark(module)
	if dead != 0 {
		t.Errorf("expected 0 dead symbols for empty module, got %d", dead)
	}
}

func TestMark_NoEntryPoints(t *testing.T) {
	// When no entry points exist, all symbols should be marked as live (conservative)
	source := `const x = 1;
fn helper() { return; }`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	dead := Mark(module)
	if dead != 0 {
		t.Errorf("expected 0 dead symbols (conservative marking), got %d", dead)
	}

	// Verify all symbols are marked as live
	for i, sym := range module.Symbols {
		if !sym.Flags.Has(ast.IsLive) {
			t.Errorf("symbol %d (%s) should be marked as live", i, sym.OriginalName)
		}
	}
}

func TestMark_WithEntryPoint(t *testing.T) {
	// Entry point should be live, unused symbols should be dead
	source := `const dead = 1;
const used = 2;
@compute @workgroup_size(1) fn main() { let x = used; }`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	dead := Mark(module)

	// Check that "dead" is not live
	foundDead := false
	foundUsed := false
	foundMain := false
	for _, sym := range module.Symbols {
		switch sym.OriginalName {
		case "dead":
			foundDead = true
			if sym.Flags.Has(ast.IsLive) {
				t.Error("symbol 'dead' should not be marked as live")
			}
		case "used":
			foundUsed = true
			if !sym.Flags.Has(ast.IsLive) {
				t.Error("symbol 'used' should be marked as live")
			}
		case "main":
			foundMain = true
			if !sym.Flags.Has(ast.IsLive) {
				t.Error("entry point 'main' should be marked as live")
			}
		}
	}

	if !foundDead || !foundUsed || !foundMain {
		t.Errorf("missing expected symbols: dead=%v, used=%v, main=%v", foundDead, foundUsed, foundMain)
	}

	// Note: dead count includes local variable 'x' which isn't marked as live
	// since DCE only marks top-level declarations as live
	if dead < 1 {
		t.Errorf("expected at least 1 dead symbol, got %d", dead)
	}
}

func TestMark_TransitiveDependencies(t *testing.T) {
	// Test that dependencies are transitively marked as live
	source := `
const a = 1;
const b = a;
const c = b;
const unused = 42;
@compute @workgroup_size(1) fn main() { let x = c; }
`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	dead := Mark(module)

	// a, b, c, main should be live; unused should be dead
	for _, sym := range module.Symbols {
		switch sym.OriginalName {
		case "a", "b", "c", "main":
			if !sym.Flags.Has(ast.IsLive) {
				t.Errorf("symbol '%s' should be marked as live", sym.OriginalName)
			}
		case "unused":
			if sym.Flags.Has(ast.IsLive) {
				t.Errorf("symbol 'unused' should not be marked as live")
			}
		}
	}

	// Note: dead count includes local variable 'x' which isn't marked
	if dead < 1 {
		t.Errorf("expected at least 1 dead symbol (unused), got %d", dead)
	}
}

// ----------------------------------------------------------------------------
// collectDeclDeps Tests
// ----------------------------------------------------------------------------

func TestCollectDeclDeps_ConstDecl(t *testing.T) {
	source := `const a = 1; const b = a;`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	deps := buildDependencyGraph(module)

	// b should depend on a
	if len(deps) == 0 {
		t.Error("expected dependencies to be recorded")
	}
}

func TestCollectDeclDeps_InvalidRefs(t *testing.T) {
	// Test that invalid refs are handled gracefully
	module := &ast.Module{
		Declarations: []ast.Decl{
			&ast.ConstDecl{Name: ast.InvalidRef(), Initializer: &ast.LiteralExpr{Value: "1"}},
			&ast.OverrideDecl{Name: ast.InvalidRef()},
			&ast.VarDecl{Name: ast.InvalidRef()},
			&ast.LetDecl{Name: ast.InvalidRef(), Initializer: &ast.LiteralExpr{Value: "1"}},
			&ast.FunctionDecl{Name: ast.InvalidRef()},
			&ast.StructDecl{Name: ast.InvalidRef()},
			&ast.AliasDecl{Name: ast.InvalidRef()},
		},
	}

	deps := buildDependencyGraph(module)
	// Should not panic and should have no dependencies recorded for invalid refs
	if len(deps) != 0 {
		t.Errorf("expected no dependencies for invalid refs, got %d", len(deps))
	}
}

func TestCollectDeclDeps_OverrideWithoutInit(t *testing.T) {
	// Override without initializer
	source := `@id(0) override x: f32;`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	deps := buildDependencyGraph(module)
	// Should work without panic
	_ = deps
}

func TestCollectDeclDeps_VarWithoutInit(t *testing.T) {
	// Var without initializer
	source := `var<private> x: i32;`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	deps := buildDependencyGraph(module)
	// Should work without panic
	_ = deps
}

func TestCollectDeclDeps_FunctionWithoutBody(t *testing.T) {
	// This tests the case where Body is nil (shouldn't happen in valid WGSL, but tests the branch)
	deps := make(map[uint32][]uint32)
	collectDeclDeps(&ast.FunctionDecl{
		Name: ast.Ref{InnerIndex: 0},
		Body: nil,
	}, deps)
	// Should work without panic
	if len(deps) != 1 {
		t.Errorf("expected 1 dependency entry, got %d", len(deps))
	}
}

func TestCollectDeclDeps_OverrideDecl(t *testing.T) {
	source := `const base = 1.0; @id(0) override scale: f32 = base;`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	deps := buildDependencyGraph(module)
	if len(deps) == 0 {
		t.Error("expected dependencies to be recorded")
	}
}

func TestCollectDeclDeps_VarDecl(t *testing.T) {
	source := `const init = 0; var<private> x: i32 = init;`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	deps := buildDependencyGraph(module)
	if len(deps) == 0 {
		t.Error("expected dependencies to be recorded")
	}
}

func TestCollectDeclDeps_FunctionDecl(t *testing.T) {
	source := `
struct Data { value: f32 }
fn helper(d: Data) -> f32 { return d.value; }
fn main(d: Data) -> f32 { return helper(d); }
`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	deps := buildDependencyGraph(module)
	if len(deps) == 0 {
		t.Error("expected dependencies to be recorded")
	}
}

func TestCollectDeclDeps_StructDecl(t *testing.T) {
	source := `
struct Inner { x: f32 }
struct Outer { inner: Inner }
`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	deps := buildDependencyGraph(module)
	// Outer should depend on Inner
	if len(deps) == 0 {
		t.Error("expected dependencies to be recorded")
	}
}

func TestCollectDeclDeps_AliasDecl(t *testing.T) {
	source := `
struct Data { x: f32 }
alias DataRef = Data;
`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	deps := buildDependencyGraph(module)
	// DataRef should depend on Data
	if len(deps) == 0 {
		t.Error("expected dependencies to be recorded")
	}
}

// ----------------------------------------------------------------------------
// collectExprRefs Tests
// ----------------------------------------------------------------------------

func TestCollectExprRefs_IdentExpr(t *testing.T) {
	refs := collectExprRefs(&ast.IdentExpr{
		Name: "x",
		Ref:  ast.Ref{SourceIndex: 0, InnerIndex: 1},
	})
	if len(refs) != 1 || refs[0] != 1 {
		t.Errorf("expected ref [1], got %v", refs)
	}
}

func TestCollectExprRefs_InvalidRef(t *testing.T) {
	refs := collectExprRefs(&ast.IdentExpr{
		Name: "x",
		Ref:  ast.InvalidRef(),
	})
	if len(refs) != 0 {
		t.Errorf("expected no refs for invalid ref, got %v", refs)
	}
}

func TestCollectExprRefs_Nil(t *testing.T) {
	refs := collectExprRefs(nil)
	if refs != nil {
		t.Errorf("expected nil for nil expression, got %v", refs)
	}
}

func TestCollectExprRefs_BinaryExpr(t *testing.T) {
	refs := collectExprRefs(&ast.BinaryExpr{
		Left:  &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
		Right: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 2}},
	})
	if len(refs) != 2 {
		t.Errorf("expected 2 refs, got %v", refs)
	}
}

func TestCollectExprRefs_UnaryExpr(t *testing.T) {
	refs := collectExprRefs(&ast.UnaryExpr{
		Operand: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref, got %v", refs)
	}
}

func TestCollectExprRefs_CallExpr(t *testing.T) {
	refs := collectExprRefs(&ast.CallExpr{
		Func: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 0}},
		Args: []ast.Expr{
			&ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
			&ast.IdentExpr{Ref: ast.Ref{InnerIndex: 2}},
		},
	})
	if len(refs) != 3 {
		t.Errorf("expected 3 refs, got %v", refs)
	}
}

func TestCollectExprRefs_IndexExpr(t *testing.T) {
	refs := collectExprRefs(&ast.IndexExpr{
		Base:  &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
		Index: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 2}},
	})
	if len(refs) != 2 {
		t.Errorf("expected 2 refs, got %v", refs)
	}
}

func TestCollectExprRefs_MemberExpr(t *testing.T) {
	refs := collectExprRefs(&ast.MemberExpr{
		Base:   &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
		Member: "x",
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref, got %v", refs)
	}
}

func TestCollectExprRefs_ParenExpr(t *testing.T) {
	refs := collectExprRefs(&ast.ParenExpr{
		Expr: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref, got %v", refs)
	}
}

// ----------------------------------------------------------------------------
// collectTypeRefs Tests
// ----------------------------------------------------------------------------

func TestCollectTypeRefs_Nil(t *testing.T) {
	refs := collectTypeRefs(nil)
	if refs != nil {
		t.Errorf("expected nil for nil type, got %v", refs)
	}
}

func TestCollectTypeRefs_IdentType(t *testing.T) {
	refs := collectTypeRefs(&ast.IdentType{
		Name: "MyStruct",
		Ref:  ast.Ref{InnerIndex: 1},
	})
	if len(refs) != 1 || refs[0] != 1 {
		t.Errorf("expected ref [1], got %v", refs)
	}
}

func TestCollectTypeRefs_IdentTypeInvalidRef(t *testing.T) {
	refs := collectTypeRefs(&ast.IdentType{
		Name: "f32",
		Ref:  ast.InvalidRef(),
	})
	if len(refs) != 0 {
		t.Errorf("expected no refs for built-in type, got %v", refs)
	}
}

func TestCollectTypeRefs_VecType(t *testing.T) {
	refs := collectTypeRefs(&ast.VecType{
		Size: 3,
		ElemType: &ast.IdentType{
			Name: "MyType",
			Ref:  ast.Ref{InnerIndex: 1},
		},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref from vec element type, got %v", refs)
	}
}

func TestCollectTypeRefs_MatType(t *testing.T) {
	refs := collectTypeRefs(&ast.MatType{
		Cols: 4,
		Rows: 4,
		ElemType: &ast.IdentType{
			Name: "MyType",
			Ref:  ast.Ref{InnerIndex: 1},
		},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref from mat element type, got %v", refs)
	}
}

func TestCollectTypeRefs_ArrayType(t *testing.T) {
	refs := collectTypeRefs(&ast.ArrayType{
		ElemType: &ast.IdentType{
			Name: "MyType",
			Ref:  ast.Ref{InnerIndex: 1},
		},
		Size: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 2}},
	})
	if len(refs) != 2 {
		t.Errorf("expected 2 refs (element type + size), got %v", refs)
	}
}

func TestCollectTypeRefs_PtrType(t *testing.T) {
	refs := collectTypeRefs(&ast.PtrType{
		AddressSpace: ast.AddressSpaceFunction,
		ElemType: &ast.IdentType{
			Name: "MyType",
			Ref:  ast.Ref{InnerIndex: 1},
		},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref from ptr element type, got %v", refs)
	}
}

func TestCollectTypeRefs_AtomicType(t *testing.T) {
	refs := collectTypeRefs(&ast.AtomicType{
		ElemType: &ast.IdentType{
			Name: "u32",
			Ref:  ast.InvalidRef(),
		},
	})
	if len(refs) != 0 {
		t.Errorf("expected 0 refs for atomic<u32>, got %v", refs)
	}
}

func TestCollectTypeRefs_TextureType(t *testing.T) {
	refs := collectTypeRefs(&ast.TextureType{
		Kind:      ast.TextureSampled,
		Dimension: ast.Texture2D,
		SampledType: &ast.IdentType{
			Name: "MyType",
			Ref:  ast.Ref{InnerIndex: 1},
		},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref from texture sampled type, got %v", refs)
	}
}

// ----------------------------------------------------------------------------
// collectStmtRefs Tests
// ----------------------------------------------------------------------------

func TestCollectStmtRefs_Nil(t *testing.T) {
	refs := collectStmtRefs(nil)
	if refs != nil {
		t.Errorf("expected nil for nil stmt, got %v", refs)
	}
}

func TestCollectStmtRefs_CompoundStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.CompoundStmt{
		Stmts: []ast.Stmt{
			&ast.ReturnStmt{
				Value: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
			},
		},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref, got %v", refs)
	}
}

func TestCollectStmtRefs_ReturnStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.ReturnStmt{
		Value: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref, got %v", refs)
	}
}

func TestCollectStmtRefs_IfStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.IfStmt{
		Condition: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 0}},
		Body: &ast.CompoundStmt{
			Stmts: []ast.Stmt{
				&ast.ReturnStmt{Value: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}}},
			},
		},
		Else: &ast.CompoundStmt{
			Stmts: []ast.Stmt{
				&ast.ReturnStmt{Value: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 2}}},
			},
		},
	})
	if len(refs) != 3 {
		t.Errorf("expected 3 refs, got %v", refs)
	}
}

func TestCollectStmtRefs_SwitchStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.SwitchStmt{
		Expr: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 0}},
		Cases: []ast.SwitchCase{
			{
				Selectors: []ast.Expr{
					&ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
				},
				Body: &ast.CompoundStmt{
					Stmts: []ast.Stmt{
						&ast.ReturnStmt{Value: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 2}}},
					},
				},
			},
		},
	})
	if len(refs) != 3 {
		t.Errorf("expected 3 refs, got %v", refs)
	}
}

func TestCollectStmtRefs_ForStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.ForStmt{
		Init: &ast.AssignStmt{
			Left:  &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 0}},
			Right: &ast.LiteralExpr{Value: "0"},
		},
		Condition: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
		Update: &ast.IncrDecrStmt{
			Expr: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 2}},
		},
		Body: &ast.CompoundStmt{
			Stmts: []ast.Stmt{
				&ast.BreakStmt{},
			},
		},
	})
	if len(refs) != 3 {
		t.Errorf("expected 3 refs, got %v", refs)
	}
}

func TestCollectStmtRefs_WhileStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.WhileStmt{
		Condition: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 0}},
		Body: &ast.CompoundStmt{
			Stmts: []ast.Stmt{
				&ast.ReturnStmt{Value: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}}},
			},
		},
	})
	if len(refs) != 2 {
		t.Errorf("expected 2 refs, got %v", refs)
	}
}

func TestCollectStmtRefs_LoopStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.LoopStmt{
		Body: &ast.CompoundStmt{
			Stmts: []ast.Stmt{
				&ast.ReturnStmt{Value: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 0}}},
			},
		},
		Continuing: &ast.CompoundStmt{
			Stmts: []ast.Stmt{
				&ast.CallStmt{Call: &ast.CallExpr{Func: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}}}},
			},
		},
	})
	if len(refs) != 2 {
		t.Errorf("expected 2 refs, got %v", refs)
	}
}

func TestCollectStmtRefs_BreakIfStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.BreakIfStmt{
		Condition: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 0}},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref, got %v", refs)
	}
}

func TestCollectStmtRefs_AssignStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.AssignStmt{
		Left:  &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 0}},
		Right: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
	})
	if len(refs) != 2 {
		t.Errorf("expected 2 refs, got %v", refs)
	}
}

func TestCollectStmtRefs_IncrDecrStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.IncrDecrStmt{
		Expr: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 0}},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref, got %v", refs)
	}
}

func TestCollectStmtRefs_CallStmt(t *testing.T) {
	refs := collectStmtRefs(&ast.CallStmt{
		Call: &ast.CallExpr{
			Func: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 0}},
			Args: []ast.Expr{
				&ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
			},
		},
	})
	if len(refs) != 2 {
		t.Errorf("expected 2 refs, got %v", refs)
	}
}

func TestCollectStmtRefs_DeclStmt_ConstDecl(t *testing.T) {
	refs := collectStmtRefs(&ast.DeclStmt{
		Decl: &ast.ConstDecl{
			Name:        ast.Ref{InnerIndex: 0},
			Initializer: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
		},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref from initializer, got %v", refs)
	}
}

func TestCollectStmtRefs_DeclStmt_LetDecl(t *testing.T) {
	refs := collectStmtRefs(&ast.DeclStmt{
		Decl: &ast.LetDecl{
			Name:        ast.Ref{InnerIndex: 0},
			Initializer: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
		},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref from initializer, got %v", refs)
	}
}

func TestCollectStmtRefs_DeclStmt_VarDecl(t *testing.T) {
	refs := collectStmtRefs(&ast.DeclStmt{
		Decl: &ast.VarDecl{
			Name:        ast.Ref{InnerIndex: 0},
			Initializer: &ast.IdentExpr{Ref: ast.Ref{InnerIndex: 1}},
			Type: &ast.IdentType{
				Ref: ast.Ref{InnerIndex: 2},
			},
		},
	})
	if len(refs) != 2 {
		t.Errorf("expected 2 refs (initializer + type), got %v", refs)
	}
}

func TestCollectStmtRefs_DeclStmt_VarDeclNoInit(t *testing.T) {
	refs := collectStmtRefs(&ast.DeclStmt{
		Decl: &ast.VarDecl{
			Name: ast.Ref{InnerIndex: 0},
			Type: &ast.IdentType{
				Ref: ast.Ref{InnerIndex: 1},
			},
		},
	})
	if len(refs) != 1 {
		t.Errorf("expected 1 ref from type, got %v", refs)
	}
}

// ----------------------------------------------------------------------------
// findEntryPoints Tests
// ----------------------------------------------------------------------------

func TestFindEntryPoints(t *testing.T) {
	source := `
fn helper() {}
@vertex fn vert() -> @builtin(position) vec4<f32> { return vec4<f32>(0.0); }
@fragment fn frag() -> @location(0) vec4<f32> { return vec4<f32>(0.0); }
@compute @workgroup_size(1) fn comp() {}
`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	entryPoints := findEntryPoints(module)
	if len(entryPoints) != 3 {
		t.Errorf("expected 3 entry points (vert, frag, comp), got %d", len(entryPoints))
	}
}

// ----------------------------------------------------------------------------
// markLive Tests
// ----------------------------------------------------------------------------

func TestMarkLive_AlreadyVisited(t *testing.T) {
	symbols := []ast.Symbol{
		{OriginalName: "a"},
		{OriginalName: "b"},
	}
	deps := map[uint32][]uint32{
		0: {1},
		1: {},
	}
	visited := map[uint32]bool{0: true}

	// Should not modify anything since 0 is already visited
	markLive(0, symbols, deps, visited)

	// Symbol 0 shouldn't be re-marked (already in visited)
	// Symbol 1 shouldn't be marked because we returned early
	if symbols[1].Flags.Has(ast.IsLive) {
		t.Error("symbol 1 should not be marked as live")
	}
}

func TestMarkLive_OutOfBoundsIndex(t *testing.T) {
	symbols := []ast.Symbol{
		{OriginalName: "a"},
	}
	deps := map[uint32][]uint32{}
	visited := map[uint32]bool{}

	// Should not panic for out of bounds index
	markLive(999, symbols, deps, visited)
}

// ----------------------------------------------------------------------------
// IsDeclarationLive Tests
// ----------------------------------------------------------------------------

func TestIsDeclarationLive_ConstDecl(t *testing.T) {
	symbols := []ast.Symbol{
		{OriginalName: "x", Flags: ast.IsLive},
		{OriginalName: "y", Flags: 0},
	}

	live := IsDeclarationLive(&ast.ConstDecl{Name: ast.Ref{InnerIndex: 0}}, symbols)
	if !live {
		t.Error("const with IsLive flag should be live")
	}

	dead := IsDeclarationLive(&ast.ConstDecl{Name: ast.Ref{InnerIndex: 1}}, symbols)
	if dead {
		t.Error("const without IsLive flag should be dead")
	}
}

func TestIsDeclarationLive_OverrideDecl(t *testing.T) {
	symbols := []ast.Symbol{
		{OriginalName: "x", Flags: ast.IsLive},
	}
	live := IsDeclarationLive(&ast.OverrideDecl{Name: ast.Ref{InnerIndex: 0}}, symbols)
	if !live {
		t.Error("override with IsLive flag should be live")
	}
}

func TestIsDeclarationLive_VarDecl(t *testing.T) {
	symbols := []ast.Symbol{
		{OriginalName: "x", Flags: ast.IsLive},
	}
	live := IsDeclarationLive(&ast.VarDecl{Name: ast.Ref{InnerIndex: 0}}, symbols)
	if !live {
		t.Error("var with IsLive flag should be live")
	}
}

func TestIsDeclarationLive_LetDecl(t *testing.T) {
	symbols := []ast.Symbol{
		{OriginalName: "x", Flags: ast.IsLive},
	}
	live := IsDeclarationLive(&ast.LetDecl{Name: ast.Ref{InnerIndex: 0}}, symbols)
	if !live {
		t.Error("let with IsLive flag should be live")
	}
}

func TestIsDeclarationLive_FunctionDecl(t *testing.T) {
	symbols := []ast.Symbol{
		{OriginalName: "f", Flags: ast.IsLive},
	}
	live := IsDeclarationLive(&ast.FunctionDecl{Name: ast.Ref{InnerIndex: 0}}, symbols)
	if !live {
		t.Error("function with IsLive flag should be live")
	}
}

func TestIsDeclarationLive_StructDecl(t *testing.T) {
	symbols := []ast.Symbol{
		{OriginalName: "S", Flags: ast.IsLive},
	}
	live := IsDeclarationLive(&ast.StructDecl{Name: ast.Ref{InnerIndex: 0}}, symbols)
	if !live {
		t.Error("struct with IsLive flag should be live")
	}
}

func TestIsDeclarationLive_AliasDecl(t *testing.T) {
	symbols := []ast.Symbol{
		{OriginalName: "A", Flags: ast.IsLive},
	}
	live := IsDeclarationLive(&ast.AliasDecl{Name: ast.Ref{InnerIndex: 0}}, symbols)
	if !live {
		t.Error("alias with IsLive flag should be live")
	}
}

func TestIsDeclarationLive_ConstAssertDecl(t *testing.T) {
	// const_assert is always kept
	live := IsDeclarationLive(&ast.ConstAssertDecl{
		Expr: &ast.LiteralExpr{Value: "true"},
	}, nil)
	if !live {
		t.Error("const_assert should always be live")
	}
}

func TestIsDeclarationLive_InvalidRef(t *testing.T) {
	symbols := []ast.Symbol{}
	live := IsDeclarationLive(&ast.ConstDecl{Name: ast.InvalidRef()}, symbols)
	if !live {
		t.Error("declaration with invalid ref should be kept (conservative)")
	}
}

func TestIsDeclarationLive_OutOfBounds(t *testing.T) {
	symbols := []ast.Symbol{}
	live := IsDeclarationLive(&ast.ConstDecl{Name: ast.Ref{InnerIndex: 999}}, symbols)
	if !live {
		t.Error("declaration with out-of-bounds ref should be kept (conservative)")
	}
}

// Note: The default case in IsDeclarationLive (line 344-345) cannot be tested
// because ast.Decl's isDecl() method is unexported. This is defensive code that
// would only be triggered if a new Decl type is added to the ast package without
// updating the switch statement.

// ----------------------------------------------------------------------------
// Integration Tests
// ----------------------------------------------------------------------------

func TestDCE_ComplexDependencies(t *testing.T) {
	source := `
struct Data { value: f32 }
struct Wrapper { data: Data }

const SCALE = 2.0;
const unused_const = 42.0;

fn helper(w: Wrapper) -> f32 {
    return w.data.value * SCALE;
}

fn unused_helper() -> f32 {
    return unused_const;
}

@compute @workgroup_size(1) fn main() {
    var w: Wrapper;
    let result = helper(w);
}
`
	p := parser.New(source)
	module, errs := p.Parse()
	if len(errs) > 0 {
		t.Fatalf("parse errors: %v", errs)
	}

	dead := Mark(module)

	// Check expectations
	liveNames := map[string]bool{
		"Data":    true,
		"Wrapper": true,
		"SCALE":   true,
		"helper":  true,
		"main":    true,
	}
	deadNames := map[string]bool{
		"unused_const":  true,
		"unused_helper": true,
	}

	for _, sym := range module.Symbols {
		if liveNames[sym.OriginalName] {
			if !sym.Flags.Has(ast.IsLive) {
				t.Errorf("symbol '%s' should be marked as live", sym.OriginalName)
			}
		}
		if deadNames[sym.OriginalName] {
			if sym.Flags.Has(ast.IsLive) {
				t.Errorf("symbol '%s' should be marked as dead", sym.OriginalName)
			}
		}
	}

	// Note: dead count includes local variables, struct fields, etc.
	if dead < 2 {
		t.Errorf("expected at least 2 dead symbols, got %d", dead)
	}
}

// ----------------------------------------------------------------------------
// Additional Coverage Tests
// ----------------------------------------------------------------------------

func TestCollectDeclDeps_LetDecl(t *testing.T) {
	// Test LetDecl branch directly (LetDecl normally only appears inside functions,
	// but we can test the branch by calling collectDeclDeps directly)
	deps := make(map[uint32][]uint32)
	collectDeclDeps(&ast.LetDecl{
		Name:        ast.Ref{InnerIndex: 5},
		Type:        &ast.IdentType{Name: "i32", Ref: ast.Ref{InnerIndex: 10}},
		Initializer: &ast.IdentExpr{Name: "x", Ref: ast.Ref{InnerIndex: 11}},
	}, deps)
	if len(deps) != 1 {
		t.Errorf("expected 1 dependency entry, got %d", len(deps))
	}
	if _, exists := deps[5]; !exists {
		t.Error("expected dependency entry for LetDecl with InnerIndex 5")
	}
}
