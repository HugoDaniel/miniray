// Package parser provides WGSL parsing into an AST.
//
// The parser implements a two-pass architecture similar to esbuild:
//
// Pass 1 (Parse): Build AST and scope tree, declare symbols
// Pass 2 (Visit): Bind identifiers to symbols, count usage, track constants
//
// This separation enables:
// - Better constant propagation (all declarations known before binding)
// - More accurate symbol use counts (for frequency-based minification)
// - Cleaner separation of concerns
package parser

import (
	"fmt"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/lexer"
	"github.com/HugoDaniel/miniray/internal/sourcemap"
)

// Parser parses WGSL source into an AST using a two-pass approach.
type Parser struct {
	source    string
	tokens    []lexer.Token
	pos       int
	lineIndex *sourcemap.LineIndex // For converting byte offsets to line/column

	// Symbol table
	symbols []ast.Symbol
	scope   *ast.Scope

	// Two-pass tracking
	scopesInOrder []*ast.Scope // Scopes in parse order for visit pass
	scopeIndex    int          // Current scope index during visit pass
	currentLoc    int          // Current source location during visit pass (for text-order scoping)

	// Constant value tracking (for propagation)
	constValues map[ast.Ref]ConstValue

	// Purity tracking
	purityCtx *ast.PurityContext

	// Errors
	errors []ParseError
}

// ConstValue represents a compile-time constant value.
type ConstValue struct {
	Kind  ConstKind
	Int   int64
	Float float64
	Bool  bool
}

// ConstKind identifies the type of constant value.
type ConstKind uint8

const (
	ConstNone ConstKind = iota
	ConstInt
	ConstFloat
	ConstBool
)

// ParseError represents a parsing error.
type ParseError struct {
	Message string
	Pos     int
	Line    int
	Column  int
}

func (e ParseError) Error() string {
	return fmt.Sprintf("%d:%d: %s", e.Line, e.Column, e.Message)
}

// New creates a new parser for the given source.
func New(source string) *Parser {
	lex := lexer.New(source)
	tokens := lex.Tokenize()

	return &Parser{
		source:      source,
		tokens:      tokens,
		lineIndex:   sourcemap.NewLineIndex(source),
		symbols:     make([]ast.Symbol, 0),
		scope:       ast.NewScope(nil),
		constValues: make(map[ast.Ref]ConstValue),
	}
}

// Parse parses the source and returns the AST module.
// This is the main entry point that runs both passes.
func (p *Parser) Parse() (*ast.Module, []ParseError) {
	module := &ast.Module{
		Source: p.source,
		Scope:  p.scope,
	}

	// Pass 1: Parse - build AST and declare symbols
	p.parseTranslationUnit(module)

	// Pass 2: Visit - bind identifiers and count usage
	p.visitModule(module)

	module.Symbols = p.symbols

	return module, p.errors
}

// ----------------------------------------------------------------------------
// Token Helpers
// ----------------------------------------------------------------------------

func (p *Parser) current() lexer.Token {
	if p.pos >= len(p.tokens) {
		return lexer.Token{Kind: lexer.TokEOF}
	}
	return p.tokens[p.pos]
}

func (p *Parser) peek(offset int) lexer.Token {
	pos := p.pos + offset
	if pos >= len(p.tokens) {
		return lexer.Token{Kind: lexer.TokEOF}
	}
	return p.tokens[pos]
}

func (p *Parser) advance() lexer.Token {
	tok := p.current()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func (p *Parser) expect(kind lexer.TokenKind) (lexer.Token, bool) {
	tok := p.current()
	if tok.Kind != kind {
		p.error(fmt.Sprintf("expected %s, got %s", kind, tok.Kind))
		// Don't advance here - let caller decide how to recover
		// This prevents consuming tokens that might be needed for error recovery
		return tok, false
	}
	p.advance()
	return tok, true
}

func (p *Parser) match(kind lexer.TokenKind) bool {
	if p.current().Kind == kind {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) error(msg string) {
	tok := p.current()
	line, col := p.lineIndex.ByteOffsetToLineColumn(tok.Start)
	p.errors = append(p.errors, ParseError{
		Message: msg,
		Pos:     tok.Start,
		Line:    line + 1, // Convert to 1-based
		Column:  col + 1,  // Convert to 1-based
	})
}

// ----------------------------------------------------------------------------
// Symbol Table (Pass 1)
// ----------------------------------------------------------------------------

func (p *Parser) declareSymbolAt(name string, kind ast.SymbolKind, flags ast.SymbolFlags, loc int) ast.Ref {
	ref := ast.Ref{InnerIndex: uint32(len(p.symbols))}

	p.symbols = append(p.symbols, ast.Symbol{
		OriginalName: name,
		Kind:         kind,
		Flags:        flags,
		UseCount:     0, // Will be counted in visit pass
	})

	// Store declaration location for text-order scoping
	p.scope.Members[name] = ast.ScopeMember{Ref: ref, Loc: loc}
	return ref
}

func (p *Parser) lookupSymbol(name string) (ast.Ref, bool) {
	for scope := p.scope; scope != nil; scope = scope.Parent {
		if member, ok := scope.Members[name]; ok {
			// Module scope symbols are always visible (scope.Parent == nil)
			// Local symbols are only visible if declared before current location
			// During parse pass (currentLoc == 0), we allow all lookups
			if scope.Parent == nil || p.currentLoc == 0 || member.Loc < p.currentLoc {
				return member.Ref, true
			}
			// Symbol exists but declared after current location - continue to outer scope
		}
	}
	return ast.InvalidRef(), false
}

func (p *Parser) pushScope() {
	newScope := ast.NewScope(p.scope)
	p.scope.Children = append(p.scope.Children, newScope)
	p.scope = newScope
	// Track scope order for visit pass
	p.scopesInOrder = append(p.scopesInOrder, newScope)
}

func (p *Parser) popScope() {
	if p.scope.Parent != nil {
		p.scope = p.scope.Parent
	}
}

// ----------------------------------------------------------------------------
// Pass 2: Visit - Bind identifiers and count usage
// ----------------------------------------------------------------------------

func (p *Parser) visitModule(module *ast.Module) {
	// Reset to module scope for visiting
	p.scope = module.Scope
	p.scopeIndex = 0

	// Initialize purity context for this module
	p.purityCtx = ast.NewPurityContext(p.symbols)

	// Visit all declarations
	for _, decl := range module.Declarations {
		p.visitDecl(decl)
	}
}

func (p *Parser) visitDecl(d ast.Decl) {
	switch decl := d.(type) {
	case *ast.ConstDecl:
		p.visitType(decl.Type)
		decl.Initializer = p.visitExpr(decl.Initializer)
		// Track constant value for propagation
		if val := p.evaluateConstExpr(decl.Initializer); val.Kind != ConstNone {
			p.constValues[decl.Name] = val
		}

	case *ast.OverrideDecl:
		p.visitType(decl.Type)
		if decl.Initializer != nil {
			decl.Initializer = p.visitExpr(decl.Initializer)
		}

	case *ast.VarDecl:
		p.visitType(decl.Type)
		if decl.Initializer != nil {
			decl.Initializer = p.visitExpr(decl.Initializer)
		}

	case *ast.LetDecl:
		p.visitType(decl.Type)
		decl.Initializer = p.visitExpr(decl.Initializer)

	case *ast.FunctionDecl:
		p.visitFunctionDecl(decl)

	case *ast.StructDecl:
		// Visit struct member types (they may reference other structs/aliases)
		for i := range decl.Members {
			p.visitType(decl.Members[i].Type)
		}

	case *ast.AliasDecl:
		// Visit the aliased type
		p.visitType(decl.Type)

	case *ast.ConstAssertDecl:
		decl.Expr = p.visitExpr(decl.Expr)
	}
}

func (p *Parser) visitFunctionDecl(decl *ast.FunctionDecl) {
	// Visit parameter types (in module scope, before entering function scope)
	for i := range decl.Parameters {
		p.visitType(decl.Parameters[i].Type)
	}

	// Visit return type
	p.visitType(decl.ReturnType)

	// Enter function scope (recorded during parse)
	p.enterNextScope()

	// Visit body
	p.visitCompoundStmt(decl.Body)

	// Exit function scope
	p.exitScope()
}

func (p *Parser) visitStmt(s ast.Stmt) {
	switch stmt := s.(type) {
	case *ast.CompoundStmt:
		p.visitCompoundStmt(stmt)

	case *ast.ReturnStmt:
		if stmt.Value != nil {
			stmt.Value = p.visitExpr(stmt.Value)
		}

	case *ast.IfStmt:
		stmt.Condition = p.visitExpr(stmt.Condition)
		p.visitCompoundStmt(stmt.Body)
		if stmt.Else != nil {
			p.visitStmt(stmt.Else)
		}

	case *ast.SwitchStmt:
		stmt.Expr = p.visitExpr(stmt.Expr)
		for i := range stmt.Cases {
			for j := range stmt.Cases[i].Selectors {
				stmt.Cases[i].Selectors[j] = p.visitExpr(stmt.Cases[i].Selectors[j])
			}
			p.visitCompoundStmt(stmt.Cases[i].Body)
		}

	case *ast.ForStmt:
		p.enterNextScope()
		if stmt.Init != nil {
			p.visitStmt(stmt.Init)
		}
		if stmt.Condition != nil {
			stmt.Condition = p.visitExpr(stmt.Condition)
		}
		if stmt.Update != nil {
			p.visitStmt(stmt.Update)
		}
		p.visitCompoundStmt(stmt.Body)
		p.exitScope()

	case *ast.WhileStmt:
		stmt.Condition = p.visitExpr(stmt.Condition)
		p.visitCompoundStmt(stmt.Body)

	case *ast.LoopStmt:
		p.visitCompoundStmt(stmt.Body)
		if stmt.Continuing != nil {
			p.visitCompoundStmt(stmt.Continuing)
		}

	case *ast.BreakIfStmt:
		stmt.Condition = p.visitExpr(stmt.Condition)

	case *ast.AssignStmt:
		stmt.Left = p.visitExpr(stmt.Left)
		stmt.Right = p.visitExpr(stmt.Right)

	case *ast.IncrDecrStmt:
		stmt.Expr = p.visitExpr(stmt.Expr)

	case *ast.CallStmt:
		stmt.Call = p.visitExpr(stmt.Call).(*ast.CallExpr)

	case *ast.DeclStmt:
		p.visitDecl(stmt.Decl)
	}
}

func (p *Parser) visitCompoundStmt(stmt *ast.CompoundStmt) {
	// Compound statements create a new scope (already tracked during parse)
	p.enterNextScope()
	for _, s := range stmt.Stmts {
		p.visitStmt(s)
	}
	p.exitScope()
}

func (p *Parser) visitExpr(e ast.Expr) ast.Expr {
	if e == nil {
		return nil
	}

	switch expr := e.(type) {
	case *ast.IdentExpr:
		// Set current location for text-order scoping
		p.currentLoc = int(expr.Loc.Start)
		// Bind identifier to symbol and increment use count
		if ref, ok := p.lookupSymbol(expr.Name); ok {
			expr.Ref = ref
			// Increment use count
			if ref.IsValid() && int(ref.InnerIndex) < len(p.symbols) {
				p.symbols[ref.InnerIndex].UseCount++
			}
		}
		// Mark purity
		if p.purityCtx != nil {
			p.purityCtx.MarkExprPurity(expr)
		}
		return expr

	case *ast.LiteralExpr:
		// Literals are always pure and constant
		expr.Flags |= ast.ExprFlagCanBeRemovedIfUnused | ast.ExprFlagIsConstant
		return expr

	case *ast.BinaryExpr:
		expr.Left = p.visitExpr(expr.Left)
		expr.Right = p.visitExpr(expr.Right)
		// Mark purity based on operands
		if p.purityCtx != nil && p.purityCtx.ExprCanBeRemovedIfUnused(expr.Left) &&
			p.purityCtx.ExprCanBeRemovedIfUnused(expr.Right) {
			expr.Flags |= ast.ExprFlagCanBeRemovedIfUnused
		}
		return expr

	case *ast.UnaryExpr:
		expr.Operand = p.visitExpr(expr.Operand)
		// Mark purity based on operand
		if p.purityCtx != nil && p.purityCtx.ExprCanBeRemovedIfUnused(expr.Operand) {
			expr.Flags |= ast.ExprFlagCanBeRemovedIfUnused
		}
		return expr

	case *ast.CallExpr:
		if expr.Func != nil {
			expr.Func = p.visitExpr(expr.Func)
		}
		// Visit template type to bind user-defined types in templated constructors
		if expr.TemplateType != nil {
			p.visitType(expr.TemplateType)
		}
		for i := range expr.Args {
			expr.Args[i] = p.visitExpr(expr.Args[i])
		}
		// Mark purity - check if it's a pure built-in call
		if p.purityCtx != nil {
			if ident, ok := expr.Func.(*ast.IdentExpr); ok {
				if p.purityCtx.PureCalls[ident.Name] {
					expr.Flags |= ast.ExprFlagFromPureFunction
					// Check if all args are pure
					allPure := true
					for _, arg := range expr.Args {
						if !p.purityCtx.ExprCanBeRemovedIfUnused(arg) {
							allPure = false
							break
						}
					}
					if allPure {
						expr.Flags |= ast.ExprFlagCanBeRemovedIfUnused
					}
				}
			}
		}
		return expr

	case *ast.IndexExpr:
		expr.Base = p.visitExpr(expr.Base)
		expr.Index = p.visitExpr(expr.Index)
		// Mark purity
		if p.purityCtx != nil && p.purityCtx.ExprCanBeRemovedIfUnused(expr.Base) &&
			p.purityCtx.ExprCanBeRemovedIfUnused(expr.Index) {
			expr.Flags |= ast.ExprFlagCanBeRemovedIfUnused
		}
		return expr

	case *ast.MemberExpr:
		expr.Base = p.visitExpr(expr.Base)
		// Mark purity
		if p.purityCtx != nil && p.purityCtx.ExprCanBeRemovedIfUnused(expr.Base) {
			expr.Flags |= ast.ExprFlagCanBeRemovedIfUnused
		}
		return expr

	case *ast.ParenExpr:
		expr.Expr = p.visitExpr(expr.Expr)
		// Inherit purity from inner expression
		if p.purityCtx != nil && p.purityCtx.ExprCanBeRemovedIfUnused(expr.Expr) {
			expr.Flags |= ast.ExprFlagCanBeRemovedIfUnused
		}
		return expr

	default:
		// All expression types are handled above - this is unreachable
		// but we return e for safety in case new expression types are added
		return e
	}
}

// visitType binds type references (IdentType) to their symbol definitions.
// This enables consistent renaming of user-defined type names (structs, aliases).
func (p *Parser) visitType(t ast.Type) {
	if t == nil {
		return
	}

	switch typ := t.(type) {
	case *ast.IdentType:
		// Set current location for text-order scoping
		p.currentLoc = int(typ.Loc.Start)
		// Bind identifier to symbol (struct or type alias)
		if ref, ok := p.lookupSymbol(typ.Name); ok {
			typ.Ref = ref
			// Increment use count for type references
			if ref.IsValid() && int(ref.InnerIndex) < len(p.symbols) {
				p.symbols[ref.InnerIndex].UseCount++
			}
		}

	case *ast.VecType:
		p.visitType(typ.ElemType)

	case *ast.MatType:
		p.visitType(typ.ElemType)

	case *ast.ArrayType:
		p.visitType(typ.ElemType)
		if typ.Size != nil {
			p.visitExpr(typ.Size)
		}

	case *ast.PtrType:
		p.visitType(typ.ElemType)

	case *ast.AtomicType:
		p.visitType(typ.ElemType)

	case *ast.TextureType:
		p.visitType(typ.SampledType)
	}
}

// enterNextScope moves to the next scope in parse order during visit pass.
func (p *Parser) enterNextScope() {
	if p.scopeIndex < len(p.scopesInOrder) {
		p.scope = p.scopesInOrder[p.scopeIndex]
		p.scopeIndex++
	}
}

// exitScope returns to parent scope during visit pass.
func (p *Parser) exitScope() {
	if p.scope.Parent != nil {
		p.scope = p.scope.Parent
	}
}

// evaluateConstExpr attempts to evaluate a constant expression.
// This enables constant propagation during the visit pass.
func (p *Parser) evaluateConstExpr(e ast.Expr) ConstValue {
	if e == nil {
		return ConstValue{}
	}

	switch expr := e.(type) {
	case *ast.LiteralExpr:
		switch expr.Kind {
		case lexer.TokIntLiteral:
			var val int64
			fmt.Sscanf(expr.Value, "%d", &val)
			return ConstValue{Kind: ConstInt, Int: val}
		case lexer.TokFloatLiteral:
			var val float64
			fmt.Sscanf(expr.Value, "%f", &val)
			return ConstValue{Kind: ConstFloat, Float: val}
		case lexer.TokTrue:
			return ConstValue{Kind: ConstBool, Bool: true}
		case lexer.TokFalse:
			return ConstValue{Kind: ConstBool, Bool: false}
		}

	case *ast.IdentExpr:
		// Look up constant value
		if val, ok := p.constValues[expr.Ref]; ok {
			return val
		}

	case *ast.UnaryExpr:
		operand := p.evaluateConstExpr(expr.Operand)
		if operand.Kind == ConstNone {
			return ConstValue{}
		}
		switch expr.Op {
		case ast.UnaryOpNeg:
			if operand.Kind == ConstInt {
				return ConstValue{Kind: ConstInt, Int: -operand.Int}
			}
			if operand.Kind == ConstFloat {
				return ConstValue{Kind: ConstFloat, Float: -operand.Float}
			}
		case ast.UnaryOpNot:
			if operand.Kind == ConstBool {
				return ConstValue{Kind: ConstBool, Bool: !operand.Bool}
			}
		}

	case *ast.BinaryExpr:
		left := p.evaluateConstExpr(expr.Left)
		right := p.evaluateConstExpr(expr.Right)
		if left.Kind == ConstNone || right.Kind == ConstNone {
			return ConstValue{}
		}

		// Integer operations
		if left.Kind == ConstInt && right.Kind == ConstInt {
			switch expr.Op {
			case ast.BinOpAdd:
				return ConstValue{Kind: ConstInt, Int: left.Int + right.Int}
			case ast.BinOpSub:
				return ConstValue{Kind: ConstInt, Int: left.Int - right.Int}
			case ast.BinOpMul:
				return ConstValue{Kind: ConstInt, Int: left.Int * right.Int}
			case ast.BinOpDiv:
				if right.Int != 0 {
					return ConstValue{Kind: ConstInt, Int: left.Int / right.Int}
				}
			case ast.BinOpMod:
				if right.Int != 0 {
					return ConstValue{Kind: ConstInt, Int: left.Int % right.Int}
				}
			case ast.BinOpAnd:
				return ConstValue{Kind: ConstInt, Int: left.Int & right.Int}
			case ast.BinOpOr:
				return ConstValue{Kind: ConstInt, Int: left.Int | right.Int}
			case ast.BinOpXor:
				return ConstValue{Kind: ConstInt, Int: left.Int ^ right.Int}
			case ast.BinOpShl:
				return ConstValue{Kind: ConstInt, Int: left.Int << uint(right.Int)}
			case ast.BinOpShr:
				return ConstValue{Kind: ConstInt, Int: left.Int >> uint(right.Int)}
			case ast.BinOpEq:
				return ConstValue{Kind: ConstBool, Bool: left.Int == right.Int}
			case ast.BinOpNe:
				return ConstValue{Kind: ConstBool, Bool: left.Int != right.Int}
			case ast.BinOpLt:
				return ConstValue{Kind: ConstBool, Bool: left.Int < right.Int}
			case ast.BinOpLe:
				return ConstValue{Kind: ConstBool, Bool: left.Int <= right.Int}
			case ast.BinOpGt:
				return ConstValue{Kind: ConstBool, Bool: left.Int > right.Int}
			case ast.BinOpGe:
				return ConstValue{Kind: ConstBool, Bool: left.Int >= right.Int}
			}
		}

		// Float operations
		if left.Kind == ConstFloat && right.Kind == ConstFloat {
			switch expr.Op {
			case ast.BinOpAdd:
				return ConstValue{Kind: ConstFloat, Float: left.Float + right.Float}
			case ast.BinOpSub:
				return ConstValue{Kind: ConstFloat, Float: left.Float - right.Float}
			case ast.BinOpMul:
				return ConstValue{Kind: ConstFloat, Float: left.Float * right.Float}
			case ast.BinOpDiv:
				return ConstValue{Kind: ConstFloat, Float: left.Float / right.Float}
			case ast.BinOpEq:
				return ConstValue{Kind: ConstBool, Bool: left.Float == right.Float}
			case ast.BinOpNe:
				return ConstValue{Kind: ConstBool, Bool: left.Float != right.Float}
			case ast.BinOpLt:
				return ConstValue{Kind: ConstBool, Bool: left.Float < right.Float}
			case ast.BinOpLe:
				return ConstValue{Kind: ConstBool, Bool: left.Float <= right.Float}
			case ast.BinOpGt:
				return ConstValue{Kind: ConstBool, Bool: left.Float > right.Float}
			case ast.BinOpGe:
				return ConstValue{Kind: ConstBool, Bool: left.Float >= right.Float}
			}
		}

		// Boolean operations
		if left.Kind == ConstBool && right.Kind == ConstBool {
			switch expr.Op {
			case ast.BinOpLogicalAnd:
				return ConstValue{Kind: ConstBool, Bool: left.Bool && right.Bool}
			case ast.BinOpLogicalOr:
				return ConstValue{Kind: ConstBool, Bool: left.Bool || right.Bool}
			case ast.BinOpEq:
				return ConstValue{Kind: ConstBool, Bool: left.Bool == right.Bool}
			case ast.BinOpNe:
				return ConstValue{Kind: ConstBool, Bool: left.Bool != right.Bool}
			}
		}

	case *ast.ParenExpr:
		return p.evaluateConstExpr(expr.Expr)
	}

	return ConstValue{}
}

// GetConstValue returns the constant value for a symbol if known.
func (p *Parser) GetConstValue(ref ast.Ref) (ConstValue, bool) {
	val, ok := p.constValues[ref]
	return val, ok
}

// ----------------------------------------------------------------------------
// Pass 1: Parse - Grammar Rules
// ----------------------------------------------------------------------------

func (p *Parser) parseTranslationUnit(module *ast.Module) {
	// Parse directives
	for {
		switch p.current().Kind {
		case lexer.TokEnable:
			module.Directives = append(module.Directives, p.parseEnableDirective())
		case lexer.TokRequires:
			module.Directives = append(module.Directives, p.parseRequiresDirective())
		case lexer.TokDiagnostic:
			module.Directives = append(module.Directives, p.parseDiagnosticDirective())
		default:
			goto parseDecls
		}
	}

parseDecls:
	// Parse declarations
	for p.current().Kind != lexer.TokEOF {
		decl := p.parseDeclaration()
		if decl != nil {
			module.Declarations = append(module.Declarations, decl)
		} else {
			// Error recovery: skip to next likely declaration start
			p.advance()
		}
	}
}

func (p *Parser) parseEnableDirective() *ast.EnableDirective {
	p.expect(lexer.TokEnable)
	dir := &ast.EnableDirective{}

	// Parse feature list
	for {
		startPos := p.pos
		if tok, ok := p.expect(lexer.TokIdent); ok {
			dir.Features = append(dir.Features, tok.Value)
		}
		if !p.match(lexer.TokComma) {
			break
		}
		// Safety check: if we didn't advance (besides the comma), something is wrong
		if p.pos == startPos+1 && p.current().Kind != lexer.TokIdent {
			p.error("expected feature name")
			break
		}
	}

	p.expect(lexer.TokSemicolon)
	return dir
}

func (p *Parser) parseRequiresDirective() *ast.RequiresDirective {
	p.expect(lexer.TokRequires)
	dir := &ast.RequiresDirective{}

	for {
		if tok, ok := p.expect(lexer.TokIdent); ok {
			dir.Features = append(dir.Features, tok.Value)
		}
		if !p.match(lexer.TokComma) {
			break
		}
	}

	p.expect(lexer.TokSemicolon)
	return dir
}

func (p *Parser) parseDiagnosticDirective() *ast.DiagnosticDirective {
	p.expect(lexer.TokDiagnostic)
	p.expect(lexer.TokLParen)

	dir := &ast.DiagnosticDirective{}

	if tok, ok := p.expect(lexer.TokIdent); ok {
		dir.Severity = tok.Value
	}
	p.expect(lexer.TokComma)
	if tok, ok := p.expect(lexer.TokIdent); ok {
		dir.Rule = tok.Value
	}

	p.expect(lexer.TokRParen)
	p.expect(lexer.TokSemicolon)
	return dir
}

func (p *Parser) parseDeclaration() ast.Decl {
	// Parse attributes
	attrs := p.parseAttributes()

	switch p.current().Kind {
	case lexer.TokConst:
		if p.peek(1).Kind == lexer.TokIdent {
			return p.parseConstDecl()
		}
		// const_assert
		return p.parseConstAssert()

	case lexer.TokConstAssert:
		return p.parseConstAssert()

	case lexer.TokOverride:
		return p.parseOverrideDecl(attrs)

	case lexer.TokVar:
		return p.parseVarDecl(attrs)

	case lexer.TokLet:
		return p.parseLetDecl()

	case lexer.TokFn:
		return p.parseFunctionDecl(attrs)

	case lexer.TokStruct:
		return p.parseStructDecl()

	case lexer.TokAlias:
		return p.parseAliasDecl()

	default:
		if len(attrs) > 0 {
			p.error("unexpected attributes")
		}
		return nil
	}
}

func (p *Parser) parseAttributes() []ast.Attribute {
	var attrs []ast.Attribute

	for p.current().Kind == lexer.TokAt {
		p.advance() // @

		attr := ast.Attribute{}
		if tok, ok := p.expect(lexer.TokIdent); ok {
			attr.Name = tok.Value
		}

		if p.match(lexer.TokLParen) {
			attr.Args = p.parseExpressionList()
			p.expect(lexer.TokRParen)
		}

		attrs = append(attrs, attr)
	}

	return attrs
}

func (p *Parser) parseConstDecl() *ast.ConstDecl {
	p.expect(lexer.TokConst)
	decl := &ast.ConstDecl{}

	if tok, ok := p.expect(lexer.TokIdent); ok {
		decl.Name = p.declareSymbolAt(tok.Value, ast.SymbolConst, 0, tok.Start)
	}

	if p.match(lexer.TokColon) {
		decl.Type = p.parseType()
	}

	p.expect(lexer.TokEq)
	decl.Initializer = p.parseExpression()
	p.expect(lexer.TokSemicolon)

	return decl
}

func (p *Parser) parseOverrideDecl(attrs []ast.Attribute) *ast.OverrideDecl {
	p.expect(lexer.TokOverride)
	decl := &ast.OverrideDecl{Attributes: attrs}

	if tok, ok := p.expect(lexer.TokIdent); ok {
		decl.Name = p.declareSymbolAt(tok.Value, ast.SymbolOverride, 0, tok.Start)
	}

	if p.match(lexer.TokColon) {
		decl.Type = p.parseType()
	}

	if p.match(lexer.TokEq) {
		decl.Initializer = p.parseExpression()
	}

	p.expect(lexer.TokSemicolon)
	return decl
}

func (p *Parser) parseVarDecl(attrs []ast.Attribute) *ast.VarDecl {
	p.expect(lexer.TokVar)
	decl := &ast.VarDecl{Attributes: attrs}

	// Parse optional <address_space, access_mode>
	if p.match(lexer.TokLt) {
		decl.AddressSpace = p.parseAddressSpace()
		if p.match(lexer.TokComma) {
			decl.AccessMode = p.parseAccessMode()
		}
		p.expect(lexer.TokGt)
	}

	// Determine symbol flags - uniform/storage bindings are external
	var flags ast.SymbolFlags
	if decl.AddressSpace == ast.AddressSpaceUniform || decl.AddressSpace == ast.AddressSpaceStorage {
		flags |= ast.IsExternalBinding
	}

	if tok, ok := p.expect(lexer.TokIdent); ok {
		decl.Name = p.declareSymbolAt(tok.Value, ast.SymbolVar, flags, tok.Start)
	}

	if p.match(lexer.TokColon) {
		decl.Type = p.parseType()
	}

	if p.match(lexer.TokEq) {
		decl.Initializer = p.parseExpression()
	}

	p.expect(lexer.TokSemicolon)
	return decl
}

func (p *Parser) parseLetDecl() *ast.LetDecl {
	p.expect(lexer.TokLet)
	decl := &ast.LetDecl{}

	if tok, ok := p.expect(lexer.TokIdent); ok {
		decl.Name = p.declareSymbolAt(tok.Value, ast.SymbolLet, 0, tok.Start)
	}

	if p.match(lexer.TokColon) {
		decl.Type = p.parseType()
	}

	p.expect(lexer.TokEq)
	decl.Initializer = p.parseExpression()
	p.expect(lexer.TokSemicolon)

	return decl
}

func (p *Parser) parseFunctionDecl(attrs []ast.Attribute) *ast.FunctionDecl {
	p.expect(lexer.TokFn)
	decl := &ast.FunctionDecl{Attributes: attrs}

	// Check for entry point
	var isEntryPoint bool
	for _, attr := range attrs {
		if attr.Name == "vertex" || attr.Name == "fragment" || attr.Name == "compute" {
			isEntryPoint = true
			break
		}
	}

	flags := ast.SymbolFlags(0)
	if isEntryPoint {
		flags |= ast.IsEntryPoint | ast.MustNotBeRenamed
	}

	if tok, ok := p.expect(lexer.TokIdent); ok {
		decl.Name = p.declareSymbolAt(tok.Value, ast.SymbolFunction, flags, tok.Start)
	}

	p.pushScope()

	// Parameters
	p.expect(lexer.TokLParen)
	if p.current().Kind != lexer.TokRParen {
		decl.Parameters = p.parseParameters()
	}
	p.expect(lexer.TokRParen)

	// Return type
	if p.match(lexer.TokArrow) {
		decl.ReturnAttr = p.parseAttributes()
		decl.ReturnType = p.parseType()
	}

	// Body
	decl.Body = p.parseCompoundStmt()

	p.popScope()
	return decl
}

func (p *Parser) parseParameters() []ast.Parameter {
	var params []ast.Parameter

	for {
		param := ast.Parameter{}
		param.Attributes = p.parseAttributes()

		if tok, ok := p.expect(lexer.TokIdent); ok {
			param.Name = p.declareSymbolAt(tok.Value, ast.SymbolParameter, 0, tok.Start)
		}

		p.expect(lexer.TokColon)
		param.Type = p.parseType()

		params = append(params, param)

		if !p.match(lexer.TokComma) {
			break
		}
		// Handle trailing comma: if next token is ), stop parsing parameters
		if p.current().Kind == lexer.TokRParen {
			break
		}
	}

	return params
}

func (p *Parser) parseStructDecl() *ast.StructDecl {
	p.expect(lexer.TokStruct)
	decl := &ast.StructDecl{}

	if tok, ok := p.expect(lexer.TokIdent); ok {
		decl.Name = p.declareSymbolAt(tok.Value, ast.SymbolStruct, 0, tok.Start)
	}

	p.expect(lexer.TokLBrace)

	for p.current().Kind != lexer.TokRBrace && p.current().Kind != lexer.TokEOF {
		member := ast.StructMember{}
		member.Attributes = p.parseAttributes()

		if tok, ok := p.expect(lexer.TokIdent); ok {
			member.Name = p.declareSymbolAt(tok.Value, ast.SymbolMember, 0, tok.Start)
		}

		p.expect(lexer.TokColon)
		member.Type = p.parseType()

		decl.Members = append(decl.Members, member)

		// Optional trailing comma
		p.match(lexer.TokComma)
	}

	p.expect(lexer.TokRBrace)
	return decl
}

func (p *Parser) parseAliasDecl() *ast.AliasDecl {
	p.expect(lexer.TokAlias)
	decl := &ast.AliasDecl{}

	if tok, ok := p.expect(lexer.TokIdent); ok {
		decl.Name = p.declareSymbolAt(tok.Value, ast.SymbolAlias, 0, tok.Start)
	}

	p.expect(lexer.TokEq)
	decl.Type = p.parseType()
	p.expect(lexer.TokSemicolon)

	return decl
}

func (p *Parser) parseConstAssert() *ast.ConstAssertDecl {
	if p.current().Kind == lexer.TokConst {
		p.advance()
	}
	p.expect(lexer.TokConstAssert)
	decl := &ast.ConstAssertDecl{}
	decl.Expr = p.parseExpression()
	p.expect(lexer.TokSemicolon)
	return decl
}

// ----------------------------------------------------------------------------
// Types
// ----------------------------------------------------------------------------

func (p *Parser) parseType() ast.Type {
	tok := p.current()

	switch tok.Kind {
	case lexer.TokIdent:
		p.advance()
		name := tok.Value

		// Check for template arguments
		if p.current().Kind == lexer.TokLt {
			return p.parseTemplatedType(name)
		}

		return &ast.IdentType{Name: name, Ref: ast.InvalidRef()}

	default:
		p.error("expected type, got " + tok.Value)
		p.advance() // Skip the unexpected token to avoid infinite loop
		return &ast.IdentType{Name: "error", Ref: ast.InvalidRef()}
	}
}

func (p *Parser) parseTemplatedType(name string) ast.Type {
	p.expect(lexer.TokLt)

	switch {
	case name == "vec2" || name == "vec3" || name == "vec4":
		size := uint8(name[3] - '0')
		elemType := p.parseType()
		p.expect(lexer.TokGt)
		return &ast.VecType{Size: size, ElemType: elemType}

	case len(name) == 6 && name[:3] == "mat" && name[4] == 'x':
		cols := uint8(name[3] - '0')
		rows := uint8(name[5] - '0')
		elemType := p.parseType()
		p.expect(lexer.TokGt)
		return &ast.MatType{Cols: cols, Rows: rows, ElemType: elemType}

	case name == "array":
		elemType := p.parseType()
		var size ast.Expr
		if p.match(lexer.TokComma) {
			// Parse size expression - could be literal, identifier, or expression
			// Use parseTemplateArgExpr to handle > properly
			size = p.parseTemplateArgExpr()
		}
		p.expect(lexer.TokGt)
		return &ast.ArrayType{ElemType: elemType, Size: size}

	case name == "ptr":
		addrSpace := p.parseAddressSpace()
		p.expect(lexer.TokComma)
		elemType := p.parseType()
		var accessMode ast.AccessMode
		if p.match(lexer.TokComma) {
			accessMode = p.parseAccessMode()
		}
		p.expect(lexer.TokGt)
		return &ast.PtrType{AddressSpace: addrSpace, ElemType: elemType, AccessMode: accessMode}

	case name == "atomic":
		elemType := p.parseType()
		p.expect(lexer.TokGt)
		return &ast.AtomicType{ElemType: elemType}

	default:
		// Check if it's a texture type
		if texType := p.parseTextureType(name); texType != nil {
			return texType
		}
		// Generic templated type - just return the base type
		p.parseType() // Consume template arg
		for p.match(lexer.TokComma) {
			p.parseType()
		}
		p.expect(lexer.TokGt)
		return &ast.IdentType{Name: name, Ref: ast.InvalidRef()}
	}
}

// parseTextureType parses a texture type if the name matches a known texture type.
// Returns nil if not a texture type.
func (p *Parser) parseTextureType(name string) ast.Type {
	var kind ast.TextureKind
	var dim ast.TextureDimension
	var isTexture bool

	// Parse sampled textures: texture_1d, texture_2d, texture_2d_array, texture_3d, texture_cube, texture_cube_array
	switch name {
	case "texture_1d":
		kind, dim, isTexture = ast.TextureSampled, ast.Texture1D, true
	case "texture_2d":
		kind, dim, isTexture = ast.TextureSampled, ast.Texture2D, true
	case "texture_2d_array":
		kind, dim, isTexture = ast.TextureSampled, ast.Texture2DArray, true
	case "texture_3d":
		kind, dim, isTexture = ast.TextureSampled, ast.Texture3D, true
	case "texture_cube":
		kind, dim, isTexture = ast.TextureSampled, ast.TextureCube, true
	case "texture_cube_array":
		kind, dim, isTexture = ast.TextureSampled, ast.TextureCubeArray, true

	// Multisampled textures
	case "texture_multisampled_2d":
		kind, dim, isTexture = ast.TextureMultisampled, ast.Texture2D, true

	// Storage textures
	case "texture_storage_1d":
		kind, dim, isTexture = ast.TextureStorage, ast.Texture1D, true
	case "texture_storage_2d":
		kind, dim, isTexture = ast.TextureStorage, ast.Texture2D, true
	case "texture_storage_2d_array":
		kind, dim, isTexture = ast.TextureStorage, ast.Texture2DArray, true
	case "texture_storage_3d":
		kind, dim, isTexture = ast.TextureStorage, ast.Texture3D, true

	// Depth textures
	case "texture_depth_2d":
		kind, dim, isTexture = ast.TextureDepth, ast.Texture2D, true
	case "texture_depth_2d_array":
		kind, dim, isTexture = ast.TextureDepth, ast.Texture2DArray, true
	case "texture_depth_cube":
		kind, dim, isTexture = ast.TextureDepth, ast.TextureCube, true
	case "texture_depth_cube_array":
		kind, dim, isTexture = ast.TextureDepth, ast.TextureCubeArray, true
	case "texture_depth_multisampled_2d":
		kind, dim, isTexture = ast.TextureDepthMultisampled, ast.Texture2D, true
	}

	if !isTexture {
		return nil
	}

	texType := &ast.TextureType{
		Kind:      kind,
		Dimension: dim,
	}

	// Parse template arguments
	if kind == ast.TextureStorage {
		// Storage textures: texture_storage_2d<format, access>
		if tok, ok := p.expect(lexer.TokIdent); ok {
			texType.TexelFormat = tok.Value
		}
		if p.match(lexer.TokComma) {
			texType.AccessMode = p.parseAccessMode()
		}
	} else if kind != ast.TextureDepth && kind != ast.TextureDepthMultisampled {
		// Sampled and multisampled textures have a sampled type
		texType.SampledType = p.parseType()
	}

	p.expect(lexer.TokGt)
	return texType
}

// parseTemplateArgExpr parses an expression that appears as a template argument.
// This is a restricted version of parseExpression that stops at > or , since
// those delimit template arguments. It does not parse relational operators
// involving > to avoid ambiguity with the closing >.
func (p *Parser) parseTemplateArgExpr() ast.Expr {
	return p.parseTemplateAdditiveExpr()
}

func (p *Parser) parseTemplateAdditiveExpr() ast.Expr {
	left := p.parseTemplateMultiplicativeExpr()

	for {
		var op ast.BinaryOp
		switch p.current().Kind {
		case lexer.TokPlus:
			op = ast.BinOpAdd
		case lexer.TokMinus:
			op = ast.BinOpSub
		default:
			return left
		}
		p.advance()
		right := p.parseTemplateMultiplicativeExpr()
		left = &ast.BinaryExpr{Op: op, Left: left, Right: right}
	}
}

func (p *Parser) parseTemplateMultiplicativeExpr() ast.Expr {
	left := p.parseTemplateUnaryExpr()

	for {
		var op ast.BinaryOp
		switch p.current().Kind {
		case lexer.TokStar:
			op = ast.BinOpMul
		case lexer.TokSlash:
			op = ast.BinOpDiv
		case lexer.TokPercent:
			op = ast.BinOpMod
		default:
			return left
		}
		p.advance()
		right := p.parseTemplateUnaryExpr()
		left = &ast.BinaryExpr{Op: op, Left: left, Right: right}
	}
}

func (p *Parser) parseTemplateUnaryExpr() ast.Expr {
	var op ast.UnaryOp
	hasOp := true

	switch p.current().Kind {
	case lexer.TokMinus:
		op = ast.UnaryOpNeg
	case lexer.TokBang:
		op = ast.UnaryOpNot
	case lexer.TokTilde:
		op = ast.UnaryOpBitNot
	default:
		hasOp = false
	}

	if hasOp {
		p.advance()
		operand := p.parseTemplateUnaryExpr()
		return &ast.UnaryExpr{Op: op, Operand: operand}
	}

	return p.parseTemplatePrimaryExpr()
}

func (p *Parser) parseTemplatePrimaryExpr() ast.Expr {
	tok := p.current()

	switch tok.Kind {
	case lexer.TokIntLiteral, lexer.TokFloatLiteral:
		p.advance()
		return &ast.LiteralExpr{Kind: tok.Kind, Value: tok.Value}

	case lexer.TokTrue, lexer.TokFalse:
		p.advance()
		return &ast.LiteralExpr{Kind: tok.Kind, Value: tok.Value}

	case lexer.TokIdent:
		p.advance()
		return &ast.IdentExpr{Loc: ast.Loc{Start: int32(tok.Start)}, Name: tok.Value, Ref: ast.InvalidRef()}

	case lexer.TokLParen:
		p.advance()
		expr := p.parseTemplateArgExpr()
		p.expect(lexer.TokRParen)
		return &ast.ParenExpr{Expr: expr}

	default:
		p.error("expected expression")
		p.advance()
		return nil
	}
}

func (p *Parser) parseAddressSpace() ast.AddressSpace {
	tok := p.current()
	if tok.Kind == lexer.TokIdent {
		p.advance()
		switch tok.Value {
		case "function":
			return ast.AddressSpaceFunction
		case "private":
			return ast.AddressSpacePrivate
		case "workgroup":
			return ast.AddressSpaceWorkgroup
		case "uniform":
			return ast.AddressSpaceUniform
		case "storage":
			return ast.AddressSpaceStorage
		}
	}
	return ast.AddressSpaceNone
}

func (p *Parser) parseAccessMode() ast.AccessMode {
	tok := p.current()
	if tok.Kind == lexer.TokIdent {
		p.advance()
		switch tok.Value {
		case "read":
			return ast.AccessModeRead
		case "write":
			return ast.AccessModeWrite
		case "read_write":
			return ast.AccessModeReadWrite
		}
	}
	return ast.AccessModeNone
}

// ----------------------------------------------------------------------------
// Expressions
// ----------------------------------------------------------------------------

func (p *Parser) parseExpression() ast.Expr {
	return p.parseLogicalOrExpr()
}

func (p *Parser) parseLogicalOrExpr() ast.Expr {
	left := p.parseLogicalAndExpr()

	for p.current().Kind == lexer.TokPipePipe {
		p.advance()
		right := p.parseLogicalAndExpr()
		left = &ast.BinaryExpr{Op: ast.BinOpLogicalOr, Left: left, Right: right}
	}

	return left
}

func (p *Parser) parseLogicalAndExpr() ast.Expr {
	left := p.parseBitwiseOrExpr()

	for p.current().Kind == lexer.TokAmpAmp {
		p.advance()
		right := p.parseBitwiseOrExpr()
		left = &ast.BinaryExpr{Op: ast.BinOpLogicalAnd, Left: left, Right: right}
	}

	return left
}

func (p *Parser) parseBitwiseOrExpr() ast.Expr {
	left := p.parseBitwiseXorExpr()

	for p.current().Kind == lexer.TokPipe {
		p.advance()
		right := p.parseBitwiseXorExpr()
		left = &ast.BinaryExpr{Op: ast.BinOpOr, Left: left, Right: right}
	}

	return left
}

func (p *Parser) parseBitwiseXorExpr() ast.Expr {
	left := p.parseBitwiseAndExpr()

	for p.current().Kind == lexer.TokCaret {
		p.advance()
		right := p.parseBitwiseAndExpr()
		left = &ast.BinaryExpr{Op: ast.BinOpXor, Left: left, Right: right}
	}

	return left
}

func (p *Parser) parseBitwiseAndExpr() ast.Expr {
	left := p.parseEqualityExpr()

	for p.current().Kind == lexer.TokAmp {
		p.advance()
		right := p.parseEqualityExpr()
		left = &ast.BinaryExpr{Op: ast.BinOpAnd, Left: left, Right: right}
	}

	return left
}

func (p *Parser) parseEqualityExpr() ast.Expr {
	left := p.parseRelationalExpr()

	for {
		var op ast.BinaryOp
		switch p.current().Kind {
		case lexer.TokEqEq:
			op = ast.BinOpEq
		case lexer.TokBangEq:
			op = ast.BinOpNe
		default:
			return left
		}
		p.advance()
		right := p.parseRelationalExpr()
		left = &ast.BinaryExpr{Op: op, Left: left, Right: right}
	}
}

func (p *Parser) parseRelationalExpr() ast.Expr {
	left := p.parseShiftExpr()

	for {
		var op ast.BinaryOp
		switch p.current().Kind {
		case lexer.TokLt:
			op = ast.BinOpLt
		case lexer.TokLtEq:
			op = ast.BinOpLe
		case lexer.TokGt:
			op = ast.BinOpGt
		case lexer.TokGtEq:
			op = ast.BinOpGe
		default:
			return left
		}
		p.advance()
		right := p.parseShiftExpr()
		left = &ast.BinaryExpr{Op: op, Left: left, Right: right}
	}
}

func (p *Parser) parseShiftExpr() ast.Expr {
	left := p.parseAdditiveExpr()

	for {
		var op ast.BinaryOp
		switch p.current().Kind {
		case lexer.TokLtLt:
			op = ast.BinOpShl
		case lexer.TokGtGt:
			op = ast.BinOpShr
		default:
			return left
		}
		p.advance()
		right := p.parseAdditiveExpr()
		left = &ast.BinaryExpr{Op: op, Left: left, Right: right}
	}
}

func (p *Parser) parseAdditiveExpr() ast.Expr {
	left := p.parseMultiplicativeExpr()

	for {
		var op ast.BinaryOp
		switch p.current().Kind {
		case lexer.TokPlus:
			op = ast.BinOpAdd
		case lexer.TokMinus:
			op = ast.BinOpSub
		default:
			return left
		}
		p.advance()
		right := p.parseMultiplicativeExpr()
		left = &ast.BinaryExpr{Op: op, Left: left, Right: right}
	}
}

func (p *Parser) parseMultiplicativeExpr() ast.Expr {
	left := p.parseUnaryExpr()

	for {
		var op ast.BinaryOp
		switch p.current().Kind {
		case lexer.TokStar:
			op = ast.BinOpMul
		case lexer.TokSlash:
			op = ast.BinOpDiv
		case lexer.TokPercent:
			op = ast.BinOpMod
		default:
			return left
		}
		p.advance()
		right := p.parseUnaryExpr()
		left = &ast.BinaryExpr{Op: op, Left: left, Right: right}
	}
}

func (p *Parser) parseUnaryExpr() ast.Expr {
	var op ast.UnaryOp
	hasOp := true

	switch p.current().Kind {
	case lexer.TokMinus:
		op = ast.UnaryOpNeg
	case lexer.TokBang:
		op = ast.UnaryOpNot
	case lexer.TokTilde:
		op = ast.UnaryOpBitNot
	case lexer.TokStar:
		op = ast.UnaryOpDeref
	case lexer.TokAmp:
		op = ast.UnaryOpAddr
	default:
		hasOp = false
	}

	if hasOp {
		p.advance()
		operand := p.parseUnaryExpr()
		return &ast.UnaryExpr{Op: op, Operand: operand}
	}

	return p.parsePostfixExpr()
}

func (p *Parser) parsePostfixExpr() ast.Expr {
	left := p.parsePrimaryExpr()

	for {
		switch p.current().Kind {
		case lexer.TokDot:
			p.advance()
			if tok, ok := p.expect(lexer.TokIdent); ok {
				left = &ast.MemberExpr{Base: left, Member: tok.Value}
			}

		case lexer.TokLBracket:
			p.advance()
			index := p.parseExpression()
			p.expect(lexer.TokRBracket)
			left = &ast.IndexExpr{Base: left, Index: index}

		case lexer.TokLParen:
			p.advance()
			args := p.parseExpressionList()
			p.expect(lexer.TokRParen)
			left = &ast.CallExpr{Func: left, Args: args}

		default:
			return left
		}
	}
}

func (p *Parser) parsePrimaryExpr() ast.Expr {
	tok := p.current()

	switch tok.Kind {
	case lexer.TokIntLiteral, lexer.TokFloatLiteral:
		p.advance()
		return &ast.LiteralExpr{Kind: tok.Kind, Value: tok.Value}

	case lexer.TokTrue, lexer.TokFalse:
		p.advance()
		return &ast.LiteralExpr{Kind: tok.Kind, Value: tok.Value}

	case lexer.TokIdent:
		p.advance()
		name := tok.Value
		loc := ast.Loc{Start: int32(tok.Start)}

		// Check for templated type constructor: array<T, N>(...) or vec2<f32>(...)
		// Only consider this if the identifier looks like a type constructor
		// (array, vec2, vec3, vec4, mat*, etc.)
		if p.current().Kind == lexer.TokLt && isTemplatedTypeName(name) {
			return p.parseTemplatedConstructor(name)
		}

		// Note: ref binding happens in visit pass
		return &ast.IdentExpr{Loc: loc, Name: name, Ref: ast.InvalidRef()}

	case lexer.TokLParen:
		p.advance()
		expr := p.parseExpression()
		p.expect(lexer.TokRParen)
		return &ast.ParenExpr{Expr: expr}

	default:
		p.error("expected expression")
		p.advance()
		return nil
	}
}

// isTemplatedTypeName returns true if the identifier is a known type that takes template arguments.
// This helps distinguish between templated type constructors (array<T, N>(...)) and
// less-than comparisons (i < 10).
func isTemplatedTypeName(name string) bool {
	switch name {
	case "array", "vec2", "vec3", "vec4", "mat2x2", "mat2x3", "mat2x4",
		"mat3x2", "mat3x3", "mat3x4", "mat4x2", "mat4x3", "mat4x4",
		"ptr", "atomic", "texture_1d", "texture_2d", "texture_2d_array",
		"texture_3d", "texture_cube", "texture_cube_array", "texture_multisampled_2d",
		"texture_storage_1d", "texture_storage_2d", "texture_storage_2d_array",
		"texture_storage_3d", "sampler", "sampler_comparison",
		"texture_depth_2d", "texture_depth_2d_array", "texture_depth_cube",
		"texture_depth_cube_array", "texture_depth_multisampled_2d":
		return true
	}
	return false
}

// parseTemplatedConstructor parses a templated type constructor like array<T, N>(...) or vec2<f32>(...)
func (p *Parser) parseTemplatedConstructor(name string) ast.Expr {
	// Parse the templated type using existing infrastructure
	// parseTemplatedType expects '<' to not be consumed yet
	templatedType := p.parseTemplatedType(name)

	// Now expect the constructor call
	if p.current().Kind != lexer.TokLParen {
		// Not a constructor, just return as identifier
		// (This handles things like array<f32, N> as a type, not a call)
		return &ast.IdentExpr{Name: name, Ref: ast.InvalidRef()}
	}

	p.advance() // consume (
	args := p.parseExpressionList()
	p.expect(lexer.TokRParen)

	// Create a call expression with the parsed template type
	return &ast.CallExpr{
		TemplateType: templatedType,
		Args:         args,
	}
}

func (p *Parser) parseExpressionList() []ast.Expr {
	var exprs []ast.Expr

	if p.current().Kind == lexer.TokRParen {
		return exprs
	}

	exprs = append(exprs, p.parseExpression())
	for p.match(lexer.TokComma) {
		// Handle trailing comma - stop if we see the closing paren
		if p.current().Kind == lexer.TokRParen {
			break
		}
		exprs = append(exprs, p.parseExpression())
	}

	return exprs
}

// ----------------------------------------------------------------------------
// Statements
// ----------------------------------------------------------------------------

func (p *Parser) parseStatement() ast.Stmt {
	switch p.current().Kind {
	case lexer.TokLBrace:
		return p.parseCompoundStmt()

	case lexer.TokReturn:
		return p.parseReturnStmt()

	case lexer.TokIf:
		return p.parseIfStmt()

	case lexer.TokSwitch:
		return p.parseSwitchStmt()

	case lexer.TokFor:
		return p.parseForStmt()

	case lexer.TokWhile:
		return p.parseWhileStmt()

	case lexer.TokLoop:
		return p.parseLoopStmt()

	case lexer.TokBreak:
		p.advance()
		if p.match(lexer.TokIf) {
			cond := p.parseExpression()
			p.expect(lexer.TokSemicolon)
			return &ast.BreakIfStmt{Condition: cond}
		}
		p.expect(lexer.TokSemicolon)
		return &ast.BreakStmt{}

	case lexer.TokContinue:
		p.advance()
		p.expect(lexer.TokSemicolon)
		return &ast.ContinueStmt{}

	case lexer.TokDiscard:
		p.advance()
		p.expect(lexer.TokSemicolon)
		return &ast.DiscardStmt{}

	case lexer.TokConst, lexer.TokLet, lexer.TokVar:
		decl := p.parseDeclaration()
		return &ast.DeclStmt{Decl: decl}

	default:
		// Expression statement or assignment
		return p.parseExpressionOrAssignment()
	}
}

func (p *Parser) parseCompoundStmt() *ast.CompoundStmt {
	p.expect(lexer.TokLBrace)
	p.pushScope()

	stmt := &ast.CompoundStmt{}
	for p.current().Kind != lexer.TokRBrace && p.current().Kind != lexer.TokEOF {
		s := p.parseStatement()
		if s != nil {
			stmt.Stmts = append(stmt.Stmts, s)
		}
	}

	p.popScope()
	p.expect(lexer.TokRBrace)
	return stmt
}

func (p *Parser) parseReturnStmt() *ast.ReturnStmt {
	p.expect(lexer.TokReturn)
	stmt := &ast.ReturnStmt{}

	if p.current().Kind != lexer.TokSemicolon {
		stmt.Value = p.parseExpression()
	}

	p.expect(lexer.TokSemicolon)
	return stmt
}

func (p *Parser) parseIfStmt() *ast.IfStmt {
	p.expect(lexer.TokIf)
	stmt := &ast.IfStmt{}

	stmt.Condition = p.parseExpression()
	stmt.Body = p.parseCompoundStmt()

	if p.match(lexer.TokElse) {
		if p.current().Kind == lexer.TokIf {
			stmt.Else = p.parseIfStmt()
		} else {
			stmt.Else = p.parseCompoundStmt()
		}
	}

	return stmt
}

func (p *Parser) parseSwitchStmt() *ast.SwitchStmt {
	p.expect(lexer.TokSwitch)
	stmt := &ast.SwitchStmt{}

	stmt.Expr = p.parseExpression()
	p.expect(lexer.TokLBrace)

	for p.current().Kind != lexer.TokRBrace && p.current().Kind != lexer.TokEOF {
		c := ast.SwitchCase{}

		if p.match(lexer.TokDefault) {
			// default case
		} else {
			p.expect(lexer.TokCase)
			// Parse selectors
			c.Selectors = append(c.Selectors, p.parseExpression())
			for p.match(lexer.TokComma) {
				c.Selectors = append(c.Selectors, p.parseExpression())
			}
		}

		p.expect(lexer.TokColon)
		c.Body = p.parseCompoundStmt()
		stmt.Cases = append(stmt.Cases, c)
	}

	p.expect(lexer.TokRBrace)
	return stmt
}

func (p *Parser) parseForStmt() *ast.ForStmt {
	p.expect(lexer.TokFor)
	p.expect(lexer.TokLParen)
	p.pushScope()

	stmt := &ast.ForStmt{}

	// Init
	if p.current().Kind != lexer.TokSemicolon {
		switch p.current().Kind {
		case lexer.TokVar, lexer.TokLet:
			stmt.Init = &ast.DeclStmt{Decl: p.parseDeclaration()}
		default:
			stmt.Init = p.parseExpressionOrAssignment()
		}
	} else {
		p.advance() // ;
	}

	// Condition
	if p.current().Kind != lexer.TokSemicolon {
		stmt.Condition = p.parseExpression()
	}
	p.expect(lexer.TokSemicolon)

	// Update - note: no semicolon after update in for loop
	if p.current().Kind != lexer.TokRParen {
		stmt.Update = p.parseForUpdateStmt()
	}

	p.expect(lexer.TokRParen)
	stmt.Body = p.parseCompoundStmt()

	p.popScope()
	return stmt
}

// parseForUpdateStmt parses the update statement in a for loop.
// Unlike regular statements, for loop updates don't end with a semicolon.
func (p *Parser) parseForUpdateStmt() ast.Stmt {
	left := p.parseExpression()

	// Check for assignment
	var op ast.AssignOp
	hasAssign := true

	switch p.current().Kind {
	case lexer.TokEq:
		op = ast.AssignOpSimple
	case lexer.TokPlusEq:
		op = ast.AssignOpAdd
	case lexer.TokMinusEq:
		op = ast.AssignOpSub
	case lexer.TokStarEq:
		op = ast.AssignOpMul
	case lexer.TokSlashEq:
		op = ast.AssignOpDiv
	case lexer.TokPercentEq:
		op = ast.AssignOpMod
	case lexer.TokAmpEq:
		op = ast.AssignOpAnd
	case lexer.TokPipeEq:
		op = ast.AssignOpOr
	case lexer.TokCaretEq:
		op = ast.AssignOpXor
	case lexer.TokLtLtEq:
		op = ast.AssignOpShl
	case lexer.TokGtGtEq:
		op = ast.AssignOpShr
	case lexer.TokPlusPlus:
		p.advance()
		return &ast.IncrDecrStmt{Expr: left, Increment: true}
	case lexer.TokMinusMinus:
		p.advance()
		return &ast.IncrDecrStmt{Expr: left, Increment: false}
	default:
		hasAssign = false
	}

	if hasAssign {
		p.advance()
		right := p.parseExpression()
		return &ast.AssignStmt{Op: op, Left: left, Right: right}
	}

	// Call expression without semicolon
	if call, ok := left.(*ast.CallExpr); ok {
		return &ast.CallStmt{Call: call}
	}

	p.error("expected for loop update statement")
	return nil
}

func (p *Parser) parseWhileStmt() *ast.WhileStmt {
	p.expect(lexer.TokWhile)
	stmt := &ast.WhileStmt{}
	stmt.Condition = p.parseExpression()
	stmt.Body = p.parseCompoundStmt()
	return stmt
}

func (p *Parser) parseLoopStmt() *ast.LoopStmt {
	p.expect(lexer.TokLoop)
	stmt := &ast.LoopStmt{}
	stmt.Body = p.parseCompoundStmt()

	if p.match(lexer.TokContinuing) {
		stmt.Continuing = p.parseCompoundStmt()
	}

	return stmt
}

func (p *Parser) parseExpressionOrAssignment() ast.Stmt {
	left := p.parseExpression()

	// Check for assignment
	var op ast.AssignOp
	hasAssign := true

	switch p.current().Kind {
	case lexer.TokEq:
		op = ast.AssignOpSimple
	case lexer.TokPlusEq:
		op = ast.AssignOpAdd
	case lexer.TokMinusEq:
		op = ast.AssignOpSub
	case lexer.TokStarEq:
		op = ast.AssignOpMul
	case lexer.TokSlashEq:
		op = ast.AssignOpDiv
	case lexer.TokPercentEq:
		op = ast.AssignOpMod
	case lexer.TokAmpEq:
		op = ast.AssignOpAnd
	case lexer.TokPipeEq:
		op = ast.AssignOpOr
	case lexer.TokCaretEq:
		op = ast.AssignOpXor
	case lexer.TokLtLtEq:
		op = ast.AssignOpShl
	case lexer.TokGtGtEq:
		op = ast.AssignOpShr
	case lexer.TokPlusPlus:
		p.advance()
		p.expect(lexer.TokSemicolon)
		return &ast.IncrDecrStmt{Expr: left, Increment: true}
	case lexer.TokMinusMinus:
		p.advance()
		p.expect(lexer.TokSemicolon)
		return &ast.IncrDecrStmt{Expr: left, Increment: false}
	default:
		hasAssign = false
	}

	if hasAssign {
		p.advance()
		right := p.parseExpression()
		p.expect(lexer.TokSemicolon)
		return &ast.AssignStmt{Op: op, Left: left, Right: right}
	}

	// Call statement
	p.expect(lexer.TokSemicolon)
	if call, ok := left.(*ast.CallExpr); ok {
		return &ast.CallStmt{Call: call}
	}

	p.error("expected statement")
	return nil
}
