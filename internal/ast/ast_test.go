package ast

import (
	"testing"

	"github.com/HugoDaniel/miniray/internal/lexer"
)

// ----------------------------------------------------------------------------
// Index32 Tests
// ----------------------------------------------------------------------------

func TestMakeIndex32(t *testing.T) {
	idx := MakeIndex32(42)
	if !idx.IsValid() {
		t.Error("MakeIndex32 should create a valid index")
	}
	if idx.GetIndex() != 42 {
		t.Errorf("GetIndex() = %d, want 42", idx.GetIndex())
	}
}

func TestIndex32Invalid(t *testing.T) {
	var idx Index32 // zero value
	if idx.IsValid() {
		t.Error("zero value Index32 should be invalid")
	}
	if idx.GetIndex() != 0 {
		t.Errorf("GetIndex() on invalid = %d, want 0", idx.GetIndex())
	}
}

// ----------------------------------------------------------------------------
// SymbolFlags Tests
// ----------------------------------------------------------------------------

func TestSymbolFlagsHas(t *testing.T) {
	flags := MustNotBeRenamed | IsEntryPoint

	if !flags.Has(MustNotBeRenamed) {
		t.Error("flags should have MustNotBeRenamed")
	}
	if !flags.Has(IsEntryPoint) {
		t.Error("flags should have IsEntryPoint")
	}
	if flags.Has(IsAPIFacing) {
		t.Error("flags should NOT have IsAPIFacing")
	}
	if flags.Has(IsBuiltin) {
		t.Error("flags should NOT have IsBuiltin")
	}
	if flags.Has(IsExternalBinding) {
		t.Error("flags should NOT have IsExternalBinding")
	}
	if flags.Has(IsLive) {
		t.Error("flags should NOT have IsLive")
	}
}

// ----------------------------------------------------------------------------
// AddressSpace.String() Tests
// ----------------------------------------------------------------------------

func TestAddressSpaceString(t *testing.T) {
	tests := []struct {
		space    AddressSpace
		expected string
	}{
		{AddressSpaceFunction, "function"},
		{AddressSpacePrivate, "private"},
		{AddressSpaceWorkgroup, "workgroup"},
		{AddressSpaceUniform, "uniform"},
		{AddressSpaceStorage, "storage"},
		{AddressSpaceHandle, ""}, // Handle doesn't have a string representation
		{AddressSpace(99), ""},   // Unknown value
	}

	for _, tt := range tests {
		got := tt.space.String()
		if got != tt.expected {
			t.Errorf("AddressSpace(%d).String() = %q, want %q", tt.space, got, tt.expected)
		}
	}
}

// ----------------------------------------------------------------------------
// AccessMode.String() Tests
// ----------------------------------------------------------------------------

func TestAccessModeString(t *testing.T) {
	tests := []struct {
		mode     AccessMode
		expected string
	}{
		{AccessModeNone, ""},
		{AccessModeRead, "read"},
		{AccessModeWrite, "write"},
		{AccessModeReadWrite, "read_write"},
		{AccessMode(99), ""}, // Unknown value
	}

	for _, tt := range tests {
		got := tt.mode.String()
		if got != tt.expected {
			t.Errorf("AccessMode(%d).String() = %q, want %q", tt.mode, got, tt.expected)
		}
	}
}

// ----------------------------------------------------------------------------
// NewScope Tests
// ----------------------------------------------------------------------------

func TestNewScope(t *testing.T) {
	// Create root scope with no parent
	root := NewScope(nil)
	if root == nil {
		t.Fatal("NewScope should not return nil")
	}
	if root.Parent != nil {
		t.Error("root scope should have nil parent")
	}
	if root.Members == nil {
		t.Error("scope.Members should be initialized")
	}

	// Create child scope
	child := NewScope(root)
	if child.Parent != root {
		t.Error("child scope should have root as parent")
	}
	if child.Members == nil {
		t.Error("child scope.Members should be initialized")
	}
}

// ----------------------------------------------------------------------------
// isSymbolPure Edge Cases
// ----------------------------------------------------------------------------

func TestIsSymbolPureInvalidRef(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Test with invalid ref
	ref := InvalidRef()
	if ctx.isSymbolPure(ref) {
		t.Error("isSymbolPure should return false for invalid ref")
	}
}

func TestIsSymbolPureOutOfBounds(t *testing.T) {
	symbols := []Symbol{
		{OriginalName: "x", Kind: SymbolConst},
	}
	ctx := NewPurityContext(symbols)

	// Test with out of bounds ref
	ref := Ref{InnerIndex: 999} // valid but out of bounds
	if ctx.isSymbolPure(ref) {
		t.Error("isSymbolPure should return false for out of bounds ref")
	}
}

func TestIsSymbolPureFunction(t *testing.T) {
	symbols := []Symbol{
		{OriginalName: "myFunc", Kind: SymbolFunction},
	}
	ctx := NewPurityContext(symbols)

	ref := Ref{InnerIndex: 0}
	if !ctx.isSymbolPure(ref) {
		t.Error("function symbol should be considered pure to read")
	}
}

func TestIsSymbolPureAlias(t *testing.T) {
	symbols := []Symbol{
		{OriginalName: "MyType", Kind: SymbolAlias},
	}
	ctx := NewPurityContext(symbols)

	ref := Ref{InnerIndex: 0}
	if !ctx.isSymbolPure(ref) {
		t.Error("alias symbol should be considered pure to read")
	}
}

func TestIsSymbolPureStruct(t *testing.T) {
	symbols := []Symbol{
		{OriginalName: "MyStruct", Kind: SymbolStruct},
	}
	ctx := NewPurityContext(symbols)

	ref := Ref{InnerIndex: 0}
	if !ctx.isSymbolPure(ref) {
		t.Error("struct symbol should be considered pure to read")
	}
}

func TestIsSymbolPureMember(t *testing.T) {
	symbols := []Symbol{
		{OriginalName: "field", Kind: SymbolMember},
	}
	ctx := NewPurityContext(symbols)

	ref := Ref{InnerIndex: 0}
	if !ctx.isSymbolPure(ref) {
		t.Error("member symbol should be considered pure to read")
	}
}

func TestIsSymbolPureUnbound(t *testing.T) {
	symbols := []Symbol{
		{OriginalName: "unknown", Kind: SymbolUnbound},
	}
	ctx := NewPurityContext(symbols)

	ref := Ref{InnerIndex: 0}
	if !ctx.isSymbolPure(ref) {
		t.Error("unbound symbol should be considered pure to read")
	}
}

// ----------------------------------------------------------------------------
// StmtCanBeRemovedIfUnused Additional Tests
// ----------------------------------------------------------------------------

func TestStmtCanBeRemovedIfUnusedNil(t *testing.T) {
	ctx := NewPurityContext(nil)
	if !ctx.StmtCanBeRemovedIfUnused(nil) {
		t.Error("nil statement should be removable")
	}
}

func TestStmtCanBeRemovedReturnWithValue(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Return with pure value
	pureReturn := &ReturnStmt{
		Value: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"},
	}
	if !ctx.StmtCanBeRemovedIfUnused(pureReturn) {
		t.Error("return with pure value should be removable")
	}

	// Return without value
	emptyReturn := &ReturnStmt{}
	if !ctx.StmtCanBeRemovedIfUnused(emptyReturn) {
		t.Error("return without value should be removable")
	}

	// Return with impure value
	impureReturn := &ReturnStmt{
		Value: &CallExpr{
			Func: &IdentExpr{Name: "impureFunc"},
			Args: []Expr{},
		},
	}
	if ctx.StmtCanBeRemovedIfUnused(impureReturn) {
		t.Error("return with impure value should NOT be removable")
	}
}

func TestStmtCanBeRemovedIncrDecrStmt(t *testing.T) {
	ctx := NewPurityContext(nil)

	stmt := &IncrDecrStmt{
		Expr:      &IdentExpr{Name: "x"},
		Increment: true,
	}
	if ctx.StmtCanBeRemovedIfUnused(stmt) {
		t.Error("increment/decrement statement should NOT be removable")
	}
}

func TestStmtCanBeRemovedControlFlow(t *testing.T) {
	ctx := NewPurityContext(nil)

	// If statement
	ifStmt := &IfStmt{
		Condition: &LiteralExpr{Kind: lexer.TokTrue, Value: "true"},
	}
	if ctx.StmtCanBeRemovedIfUnused(ifStmt) {
		t.Error("if statement should NOT be removable")
	}

	// For statement
	forStmt := &ForStmt{}
	if ctx.StmtCanBeRemovedIfUnused(forStmt) {
		t.Error("for statement should NOT be removable")
	}

	// While statement
	whileStmt := &WhileStmt{
		Condition: &LiteralExpr{Kind: lexer.TokTrue, Value: "true"},
	}
	if ctx.StmtCanBeRemovedIfUnused(whileStmt) {
		t.Error("while statement should NOT be removable")
	}

	// Loop statement
	loopStmt := &LoopStmt{}
	if ctx.StmtCanBeRemovedIfUnused(loopStmt) {
		t.Error("loop statement should NOT be removable")
	}

	// Switch statement
	switchStmt := &SwitchStmt{
		Expr: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "1"},
	}
	if ctx.StmtCanBeRemovedIfUnused(switchStmt) {
		t.Error("switch statement should NOT be removable")
	}

	// Break statement
	breakStmt := &BreakStmt{}
	if ctx.StmtCanBeRemovedIfUnused(breakStmt) {
		t.Error("break statement should NOT be removable")
	}

	// Continue statement
	continueStmt := &ContinueStmt{}
	if ctx.StmtCanBeRemovedIfUnused(continueStmt) {
		t.Error("continue statement should NOT be removable")
	}

	// Discard statement
	discardStmt := &DiscardStmt{}
	if ctx.StmtCanBeRemovedIfUnused(discardStmt) {
		t.Error("discard statement should NOT be removable")
	}
}

// ----------------------------------------------------------------------------
// DeclCanBeRemovedIfUnused Additional Tests
// ----------------------------------------------------------------------------

func TestDeclCanBeRemovedIfUnusedNil(t *testing.T) {
	ctx := NewPurityContext(nil)
	if !ctx.DeclCanBeRemovedIfUnused(nil) {
		t.Error("nil declaration should be removable")
	}
}

func TestDeclCanBeRemovedAliasDecl(t *testing.T) {
	ctx := NewPurityContext(nil)

	aliasDecl := &AliasDecl{
		Name: Ref{InnerIndex: 0},
	}
	if !ctx.DeclCanBeRemovedIfUnused(aliasDecl) {
		t.Error("alias declaration should be removable if unused")
	}
}

func TestDeclCanBeRemovedConstAssertDecl(t *testing.T) {
	ctx := NewPurityContext(nil)

	constAssert := &ConstAssertDecl{
		Expr: &LiteralExpr{Kind: lexer.TokTrue, Value: "true"},
	}
	if ctx.DeclCanBeRemovedIfUnused(constAssert) {
		t.Error("const_assert declaration should NOT be removable")
	}
}

// ----------------------------------------------------------------------------
// MarkExprPurity Additional Tests
// ----------------------------------------------------------------------------

func TestMarkExprPurityNil(t *testing.T) {
	ctx := NewPurityContext(nil)
	// Should not panic
	ctx.MarkExprPurity(nil)
}

func TestMarkExprPurityIdentWithConstSymbol(t *testing.T) {
	symbols := []Symbol{
		{OriginalName: "MY_CONST", Kind: SymbolConst},
	}
	ctx := NewPurityContext(symbols)

	ident := &IdentExpr{Name: "MY_CONST", Ref: Ref{InnerIndex: 0}}
	ctx.MarkExprPurity(ident)

	if !ident.Flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("const identifier should be marked as removable")
	}
	if !ident.Flags.Has(ExprFlagIsConstant) {
		t.Error("const identifier should be marked as constant")
	}
}

func TestMarkExprPurityIdentWithNonConstSymbol(t *testing.T) {
	symbols := []Symbol{
		{OriginalName: "myVar", Kind: SymbolVar},
	}
	ctx := NewPurityContext(symbols)

	ident := &IdentExpr{Name: "myVar", Ref: Ref{InnerIndex: 0}}
	ctx.MarkExprPurity(ident)

	if !ident.Flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("var identifier should be marked as removable (reading is pure)")
	}
	if ident.Flags.Has(ExprFlagIsConstant) {
		t.Error("var identifier should NOT be marked as constant")
	}
}

func TestMarkExprPurityBinaryExpr(t *testing.T) {
	ctx := NewPurityContext(nil)

	binExpr := &BinaryExpr{
		Op:    BinOpAdd,
		Left:  &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "1"},
		Right: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "2"},
	}
	ctx.MarkExprPurity(binExpr)

	if !binExpr.Flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("pure binary expression should be marked as removable")
	}
}

func TestMarkExprPurityUnaryExpr(t *testing.T) {
	ctx := NewPurityContext(nil)

	unaryExpr := &UnaryExpr{
		Op:      UnaryOpNeg,
		Operand: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"},
	}
	ctx.MarkExprPurity(unaryExpr)

	if !unaryExpr.Flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("pure unary expression should be marked as removable")
	}
}

func TestMarkExprPurityCallWithImpureArg(t *testing.T) {
	ctx := NewPurityContext(nil)

	call := &CallExpr{
		Func: &IdentExpr{Name: "sin"},
		Args: []Expr{
			&CallExpr{
				Func: &IdentExpr{Name: "impureFunc"},
				Args: []Expr{},
			},
		},
	}
	ctx.MarkExprPurity(call)

	if !call.Flags.Has(ExprFlagFromPureFunction) {
		t.Error("call to sin should be marked as pure function")
	}
	if call.Flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("call with impure args should NOT be marked as removable")
	}
}

func TestMarkExprPurityIndexExpr(t *testing.T) {
	ctx := NewPurityContext(nil)

	indexExpr := &IndexExpr{
		Base:  &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "0"},
		Index: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "1"},
	}
	ctx.MarkExprPurity(indexExpr)

	if !indexExpr.Flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("pure index expression should be marked as removable")
	}
}

func TestMarkExprPurityMemberExpr(t *testing.T) {
	ctx := NewPurityContext(nil)

	memberExpr := &MemberExpr{
		Base:   &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "0"},
		Member: "x",
	}
	ctx.MarkExprPurity(memberExpr)

	if !memberExpr.Flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("pure member expression should be marked as removable")
	}
}

func TestMarkExprPurityParenExpr(t *testing.T) {
	ctx := NewPurityContext(nil)

	parenExpr := &ParenExpr{
		Expr: &LiteralExpr{Kind: lexer.TokIntLiteral, Value: "42"},
	}
	ctx.MarkExprPurity(parenExpr)

	if !parenExpr.Flags.Has(ExprFlagCanBeRemovedIfUnused) {
		t.Error("pure paren expression should be marked as removable")
	}
}

// ----------------------------------------------------------------------------
// ExprCanBeRemovedIfUnused Additional Tests
// ----------------------------------------------------------------------------

func TestExprCanBeRemovedCallWithFlag(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Call with CanBeRemovedIfUnused flag set
	call := &CallExpr{
		Func:  &IdentExpr{Name: "unknownFunc"},
		Args:  []Expr{},
		Flags: ExprFlagCanBeRemovedIfUnused,
	}
	if !ctx.ExprCanBeRemovedIfUnused(call) {
		t.Error("call with CanBeRemovedIfUnused flag should be removable")
	}
}

func TestExprCanBeRemovedCallWithPureFunctionFlag(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Call with FromPureFunction flag set
	call := &CallExpr{
		Func:  &IdentExpr{Name: "unknownFunc"},
		Args:  []Expr{},
		Flags: ExprFlagFromPureFunction,
	}
	if !ctx.ExprCanBeRemovedIfUnused(call) {
		t.Error("call with FromPureFunction flag should be removable")
	}
}

func TestExprCanBeRemovedIdentWithFlag(t *testing.T) {
	ctx := NewPurityContext(nil)

	// Identifier with CanBeRemovedIfUnused flag set
	ident := &IdentExpr{
		Name:  "someVar",
		Ref:   Ref{InnerIndex: 999}, // out of bounds
		Flags: ExprFlagCanBeRemovedIfUnused,
	}
	if !ctx.ExprCanBeRemovedIfUnused(ident) {
		t.Error("identifier with CanBeRemovedIfUnused flag should be removable")
	}
}

// customExpr is a test type that implements Expr but is not handled by ExprCanBeRemovedIfUnused
type customExpr struct{}

func (*customExpr) isExpr() {}

func TestExprCanBeRemovedUnknownExprType(t *testing.T) {
	ctx := NewPurityContext(nil)

	// customExpr is not handled explicitly - should default to false
	custom := &customExpr{}
	if ctx.ExprCanBeRemovedIfUnused(custom) {
		t.Error("unknown expression type should NOT be removable by default")
	}
}

// customStmt is a test type that implements Stmt but is not handled
type customStmt struct{}

func (*customStmt) isStmt() {}

func TestStmtCanBeRemovedUnknownStmtType(t *testing.T) {
	ctx := NewPurityContext(nil)

	// customStmt is not handled explicitly - should default to false
	custom := &customStmt{}
	if ctx.StmtCanBeRemovedIfUnused(custom) {
		t.Error("unknown statement type should NOT be removable by default")
	}
}

// customDecl is a test type that implements Decl but is not handled
type customDecl struct{}

func (*customDecl) isDecl() {}

func TestDeclCanBeRemovedUnknownDeclType(t *testing.T) {
	ctx := NewPurityContext(nil)

	// customDecl is not handled explicitly - should default to false
	custom := &customDecl{}
	if ctx.DeclCanBeRemovedIfUnused(custom) {
		t.Error("unknown declaration type should NOT be removable by default")
	}
}

// ----------------------------------------------------------------------------
// Ref Tests
// ----------------------------------------------------------------------------

func TestRefIsValid(t *testing.T) {
	// Valid ref
	valid := Ref{SourceIndex: 0, InnerIndex: 0}
	if !valid.IsValid() {
		t.Error("Ref{0, 0} should be valid")
	}

	// Invalid ref
	invalid := InvalidRef()
	if invalid.IsValid() {
		t.Error("InvalidRef() should not be valid")
	}

	// Partially invalid (only SourceIndex is max)
	partial := Ref{SourceIndex: ^uint32(0), InnerIndex: 0}
	if partial.IsValid() {
		t.Error("Ref with max SourceIndex should not be valid")
	}
}
