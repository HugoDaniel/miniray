package ast

import (
	"testing"

	"codeberg.org/saruga/wgsl-minifier/internal/lexer"
)

// ----------------------------------------------------------------------------
// Pure Built-ins Tests
// ----------------------------------------------------------------------------

func TestComputePureBuiltins(t *testing.T) {
	pure := ComputePureBuiltins()

	// Math functions should be pure
	mathFuncs := []string{
		"sin", "cos", "tan", "abs", "sqrt", "pow", "exp", "log",
		"floor", "ceil", "round", "clamp", "min", "max", "mix",
		"dot", "cross", "length", "normalize", "distance",
	}
	for _, fn := range mathFuncs {
		if !pure[fn] {
			t.Errorf("expected %q to be marked as pure", fn)
		}
	}

	// Type constructors should be pure
	constructors := []string{
		"vec2", "vec3", "vec4", "vec2f", "vec3f", "vec4f",
		"mat4x4", "mat4x4f", "array", "bool", "i32", "u32", "f32",
	}
	for _, fn := range constructors {
		if !pure[fn] {
			t.Errorf("expected constructor %q to be marked as pure", fn)
		}
	}

	// Pack/unpack should be pure
	packFuncs := []string{
		"pack2x16float", "unpack2x16float",
		"pack4x8snorm", "unpack4x8snorm",
	}
	for _, fn := range packFuncs {
		if !pure[fn] {
			t.Errorf("expected %q to be marked as pure", fn)
		}
	}

	// Derivative functions should be pure (no state modification)
	derivFuncs := []string{"dpdx", "dpdy", "fwidth"}
	for _, fn := range derivFuncs {
		if !pure[fn] {
			t.Errorf("expected %q to be marked as pure", fn)
		}
	}
}

// ----------------------------------------------------------------------------
// Expression Purity Tests
// ----------------------------------------------------------------------------

func TestLiteralExprPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Integer literal
	intLit := &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"}
	if !ctx.ExprCanBeRemovedIfUnused(intLit) {
		t.Error("integer literal should be removable")
	}

	// Float literal
	floatLit := &LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "3.14"}
	if !ctx.ExprCanBeRemovedIfUnused(floatLit) {
		t.Error("float literal should be removable")
	}

	// Bool literal (true)
	boolLit := &LiteralExpr{Kind: lexer.TokTrue, Value: "true"}
	if !ctx.ExprCanBeRemovedIfUnused(boolLit) {
		t.Error("bool literal should be removable")
	}
}

func TestIdentExprPurity(t *testing.T) {
	// Create symbol table with various symbol types
	symbols := []Symbol{
		{OriginalName: "myConst", Kind: SymbolConst},
		{OriginalName: "myLet", Kind: SymbolLet},
		{OriginalName: "myVar", Kind: SymbolVar},
		{OriginalName: "myParam", Kind: SymbolParameter},
	}

	ctx := NewPurityContext(symbols)

	// Const reference should be pure
	constRef := &IdentExpr{Name: "myConst", Ref: Ref{InnerIndex: 0}}
	if !ctx.ExprCanBeRemovedIfUnused(constRef) {
		t.Error("const reference should be removable")
	}

	// Let reference should be pure
	letRef := &IdentExpr{Name: "myLet", Ref: Ref{InnerIndex: 1}}
	if !ctx.ExprCanBeRemovedIfUnused(letRef) {
		t.Error("let reference should be removable")
	}

	// Var reference should be pure (reading has no side effects)
	varRef := &IdentExpr{Name: "myVar", Ref: Ref{InnerIndex: 2}}
	if !ctx.ExprCanBeRemovedIfUnused(varRef) {
		t.Error("var reference should be removable (reading is pure)")
	}

	// Parameter reference should be pure
	paramRef := &IdentExpr{Name: "myParam", Ref: Ref{InnerIndex: 3}}
	if !ctx.ExprCanBeRemovedIfUnused(paramRef) {
		t.Error("parameter reference should be removable")
	}
}

func TestBinaryExprPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Pure binary expression: 1 + 2
	pureBinary := &BinaryExpr{
		Op:    BinOpAdd,
		Left:  &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "1"},
		Right: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "2"},
	}
	if !ctx.ExprCanBeRemovedIfUnused(pureBinary) {
		t.Error("pure binary expression should be removable")
	}

	// All binary operators with pure operands should be pure
	ops := []BinaryOp{
		BinOpAdd, BinOpSub, BinOpMul, BinOpDiv, BinOpMod,
		BinOpAnd, BinOpOr, BinOpXor, BinOpShl, BinOpShr,
		BinOpLogicalAnd, BinOpLogicalOr,
		BinOpEq, BinOpNe, BinOpLt, BinOpLe, BinOpGt, BinOpGe,
	}
	for _, op := range ops {
		expr := &BinaryExpr{
			Op:    op,
			Left:  &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "1"},
			Right: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "2"},
		}
		if !ctx.ExprCanBeRemovedIfUnused(expr) {
			t.Errorf("binary op %d with pure operands should be removable", op)
		}
	}
}

func TestUnaryExprPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Negation of literal
	negExpr := &UnaryExpr{
		Op:      UnaryOpNeg,
		Operand: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"},
	}
	if !ctx.ExprCanBeRemovedIfUnused(negExpr) {
		t.Error("negation of literal should be removable")
	}

	// Logical not of literal
	notExpr := &UnaryExpr{
		Op:      UnaryOpNot,
		Operand: &LiteralExpr{Kind: lexer.TokTrue, Value: "true"},
	}
	if !ctx.ExprCanBeRemovedIfUnused(notExpr) {
		t.Error("logical not of literal should be removable")
	}

	// Bitwise not of literal
	bitNotExpr := &UnaryExpr{
		Op:      UnaryOpBitNot,
		Operand: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "0xff"},
	}
	if !ctx.ExprCanBeRemovedIfUnused(bitNotExpr) {
		t.Error("bitwise not of literal should be removable")
	}
}

func TestCallExprPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Pure built-in call: sin(1.0)
	pureCall := &CallExpr{
		Func: &IdentExpr{Name: "sin"},
		Args: []Expr{&LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "1.0"}},
	}
	if !ctx.ExprCanBeRemovedIfUnused(pureCall) {
		t.Error("call to pure built-in sin() should be removable")
	}

	// Pure constructor call: vec3(1.0, 2.0, 3.0)
	constructorCall := &CallExpr{
		Func: &IdentExpr{Name: "vec3"},
		Args: []Expr{
			&LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "1.0"},
			&LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "2.0"},
			&LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "3.0"},
		},
	}
	if !ctx.ExprCanBeRemovedIfUnused(constructorCall) {
		t.Error("call to vec3 constructor should be removable")
	}

	// Potentially impure call: myFunction()
	impureCall := &CallExpr{
		Func: &IdentExpr{Name: "myFunction"},
		Args: []Expr{},
	}
	if ctx.ExprCanBeRemovedIfUnused(impureCall) {
		t.Error("call to unknown function should NOT be removable")
	}
}

func TestIndexExprPurity(t *testing.T) {
	symbols := []Symbol{
		{OriginalName: "arr", Kind: SymbolVar},
	}
	ctx := NewPurityContext(symbols)

	// Pure indexing: arr[0]
	indexExpr := &IndexExpr{
		Base:  &IdentExpr{Name: "arr", Ref: Ref{InnerIndex: 0}},
		Index: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "0"},
	}
	if !ctx.ExprCanBeRemovedIfUnused(indexExpr) {
		t.Error("array indexing with pure base and index should be removable")
	}
}

func TestMemberExprPurity(t *testing.T) {
	symbols := []Symbol{
		{OriginalName: "obj", Kind: SymbolVar},
	}
	ctx := NewPurityContext(symbols)

	// Pure member access: obj.x
	memberExpr := &MemberExpr{
		Base:   &IdentExpr{Name: "obj", Ref: Ref{InnerIndex: 0}},
		Member: "x",
	}
	if !ctx.ExprCanBeRemovedIfUnused(memberExpr) {
		t.Error("member access with pure base should be removable")
	}
}

func TestParenExprPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Pure parenthesized: (1 + 2)
	parenExpr := &ParenExpr{
		Expr: &BinaryExpr{
			Op:    BinOpAdd,
			Left:  &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "1"},
			Right: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "2"},
		},
	}
	if !ctx.ExprCanBeRemovedIfUnused(parenExpr) {
		t.Error("parenthesized pure expression should be removable")
	}
}

// ----------------------------------------------------------------------------
// Declaration Purity Tests
// ----------------------------------------------------------------------------

func TestConstDeclPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Const with pure initializer
	pureConst := &ConstDecl{
		Name:        Ref{InnerIndex: 0},
		Initializer: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"},
	}
	if !ctx.DeclCanBeRemovedIfUnused(pureConst) {
		t.Error("const with pure initializer should be removable")
	}
}

func TestLetDeclPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Let with pure initializer
	pureLet := &LetDecl{
		Name:        Ref{InnerIndex: 0},
		Initializer: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"},
	}
	if !ctx.DeclCanBeRemovedIfUnused(pureLet) {
		t.Error("let with pure initializer should be removable")
	}

	// Let with impure initializer (unknown function call)
	impureLet := &LetDecl{
		Name: Ref{InnerIndex: 0},
		Initializer: &CallExpr{
			Func: &IdentExpr{Name: "impureFunc"},
			Args: []Expr{},
		},
	}
	if ctx.DeclCanBeRemovedIfUnused(impureLet) {
		t.Error("let with impure initializer should NOT be removable")
	}
}

func TestVarDeclPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Var without initializer
	varNoInit := &VarDecl{
		Name: Ref{InnerIndex: 0},
	}
	if !ctx.DeclCanBeRemovedIfUnused(varNoInit) {
		t.Error("var without initializer should be removable")
	}

	// Var with pure initializer
	varPureInit := &VarDecl{
		Name:        Ref{InnerIndex: 0},
		Initializer: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"},
	}
	if !ctx.DeclCanBeRemovedIfUnused(varPureInit) {
		t.Error("var with pure initializer should be removable")
	}
}

func TestFunctionDeclPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Function declarations can be removed if unused (DCE)
	funcDecl := &FunctionDecl{
		Name: Ref{InnerIndex: 0},
	}
	if !ctx.DeclCanBeRemovedIfUnused(funcDecl) {
		t.Error("unused function declaration should be removable")
	}
}

func TestStructDeclPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Struct declarations can be removed if unused
	structDecl := &StructDecl{
		Name: Ref{InnerIndex: 0},
	}
	if !ctx.DeclCanBeRemovedIfUnused(structDecl) {
		t.Error("unused struct declaration should be removable")
	}
}

func TestOverrideDeclPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Override declarations are API-facing, should NOT be removed
	overrideDecl := &OverrideDecl{
		Name: Ref{InnerIndex: 0},
	}
	if ctx.DeclCanBeRemovedIfUnused(overrideDecl) {
		t.Error("override declaration should NOT be removable (API-facing)")
	}
}

// ----------------------------------------------------------------------------
// Statement Purity Tests
// ----------------------------------------------------------------------------

func TestDeclStmtPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// DeclStmt with pure const
	declStmt := &DeclStmt{
		Decl: &ConstDecl{
			Name:        Ref{InnerIndex: 0},
			Initializer: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"},
		},
	}
	if !ctx.StmtCanBeRemovedIfUnused(declStmt) {
		t.Error("decl statement with pure initializer should be removable")
	}
}

func TestCallStmtPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Call statements might have side effects
	callStmt := &CallStmt{
		Call: &CallExpr{
			Func: &IdentExpr{Name: "someFunc"},
			Args: []Expr{},
		},
	}
	if ctx.StmtCanBeRemovedIfUnused(callStmt) {
		t.Error("call statement should NOT be removable (potential side effects)")
	}
}

func TestAssignStmtPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Assignment has side effects
	symbols := []Symbol{{OriginalName: "x", Kind: SymbolVar}}
	ctx = NewPurityContext(symbols)

	assignStmt := &AssignStmt{
		Op:    AssignOpSimple,
		Left:  &IdentExpr{Name: "x", Ref: Ref{InnerIndex: 0}},
		Right: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"},
	}
	if ctx.StmtCanBeRemovedIfUnused(assignStmt) {
		t.Error("assignment statement should NOT be removable (side effect)")
	}
}

// ----------------------------------------------------------------------------
// ExprFlags Tests
// ----------------------------------------------------------------------------

func TestExprFlagsHas(t *testing.T) {
	flags := ExprFlagCanBeRemovedIfUnused | ExprFlagIsConstant

	if !flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("flags should have CanBeRemovedIfUnused")
	}
	if !flags.Has(ExprFlagIsConstant) {
		t.Error("flags should have IsConstant")
	}
	if flags.Has(ExprFlagFromPureFunction) {
		t.Error("flags should NOT have FromPureFunction")
	}
}

func TestMarkExprPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Mark a literal - should get both flags
	lit := &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"}
	ctx.MarkExprPurity(lit)

	if !lit.Flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("literal should be marked as removable")
	}
	if !lit.Flags.Has(ExprFlagIsConstant) {
		t.Error("literal should be marked as constant")
	}

	// Mark a pure function call
	call := &CallExpr{
		Func: &IdentExpr{Name: "sin"},
		Args: []Expr{&LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "1.0"}},
	}
	// First mark the args
	for _, arg := range call.Args {
		ctx.MarkExprPurity(arg)
	}
	ctx.MarkExprPurity(call)

	if !call.Flags.Has(ExprFlagFromPureFunction) {
		t.Error("call to sin() should be marked as pure function")
	}
	if !call.Flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("call to sin() with pure args should be removable")
	}
}

// ----------------------------------------------------------------------------
// Edge Cases
// ----------------------------------------------------------------------------

func TestNilExprPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// nil expressions should be considered pure (no-op)
	if !ctx.ExprCanBeRemovedIfUnused(nil) {
		t.Error("nil expression should be removable")
	}
}

func TestInvalidRefPurity(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Identifier with invalid ref should still be considered pure
	// (unbound identifier - reading it is still side-effect free in WGSL)
	ident := &IdentExpr{Name: "unknown", Ref: InvalidRef()}
	if !ctx.ExprCanBeRemovedIfUnused(ident) {
		t.Error("identifier with invalid ref should be removable")
	}
}

// ----------------------------------------------------------------------------
// Complex Expression Tests
// ----------------------------------------------------------------------------

func TestNestedPureExpressions(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Complex pure expression: sin(1.0 + cos(2.0)) * 3.0
	expr := &BinaryExpr{
		Op: BinOpMul,
		Left: &CallExpr{
			Func: &IdentExpr{Name: "sin"},
			Args: []Expr{
				&BinaryExpr{
					Op:   BinOpAdd,
					Left: &LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "1.0"},
					Right: &CallExpr{
						Func: &IdentExpr{Name: "cos"},
						Args: []Expr{
							&LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "2.0"},
						},
					},
				},
			},
		},
		Right: &LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "3.0"},
	}

	if !ctx.ExprCanBeRemovedIfUnused(expr) {
		t.Error("complex nested pure expression should be removable")
	}
}

func TestMixedPurityExpressions(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Expression with impure call nested: sin(impure())
	expr := &CallExpr{
		Func: &IdentExpr{Name: "sin"},
		Args: []Expr{
			&CallExpr{
				Func: &IdentExpr{Name: "impure"},
				Args: []Expr{},
			},
		},
	}

	// sin is pure, but impure() isn't, so the whole thing isn't removable
	if ctx.ExprCanBeRemovedIfUnused(expr) {
		t.Error("expression with impure subexpression should NOT be removable")
	}
}

// ----------------------------------------------------------------------------
// Benchmarks
// ----------------------------------------------------------------------------

func BenchmarkComputePureBuiltins(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = ComputePureBuiltins()
	}
}

func BenchmarkExprCanBeRemovedIfUnused(b *testing.B) {
	ctx := NewPurityContext(nil)

	// Complex expression to test
	expr := &BinaryExpr{
		Op: BinOpMul,
		Left: &CallExpr{
			Func: &IdentExpr{Name: "sin"},
			Args: []Expr{
				&BinaryExpr{
					Op:    BinOpAdd,
					Left:  &LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "1.0"},
					Right: &LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "2.0"},
				},
			},
		},
		Right: &LiteralExpr{Kind: lexer.TokFloatLiteral, Value: "3.0"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.ExprCanBeRemovedIfUnused(expr)
	}
}
