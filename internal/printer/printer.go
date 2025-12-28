// Package printer outputs WGSL code from an AST.
//
// The printer can operate in two modes:
// - Pretty: Human-readable output with indentation
// - Minified: Minimal whitespace output
//
// Following the esbuild pattern, minification decisions are made
// during printing rather than as a separate AST transformation.
package printer

import (
	"strings"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/dce"
	"github.com/HugoDaniel/miniray/internal/sourcemap"
)

// Options controls printer output.
type Options struct {
	// MinifyWhitespace removes unnecessary whitespace
	MinifyWhitespace bool

	// MinifyIdentifiers uses short names for identifiers
	MinifyIdentifiers bool

	// MinifySyntax applies syntax-level optimizations
	MinifySyntax bool

	// TreeShaking enables dead code elimination (skip dead declarations)
	TreeShaking bool

	// Renamer provides minified names (nil for no renaming)
	Renamer Renamer

	// SourceMapGen is the source map generator (nil to disable)
	SourceMapGen *sourcemap.Generator
}

// Renamer provides minified names for symbols.
type Renamer interface {
	NameForSymbol(ref ast.Ref) string
}

// Printer outputs WGSL code.
type Printer struct {
	options Options
	symbols []ast.Symbol

	buf    strings.Builder
	indent int

	// Track if we need whitespace before next token
	needsSpace bool

	// Position tracking for source maps
	outputLine int
	outputCol  int
}

// New creates a new printer.
func New(options Options, symbols []ast.Symbol) *Printer {
	return &Printer{
		options: options,
		symbols: symbols,
	}
}

// Print outputs the module as a string.
func (p *Printer) Print(module *ast.Module) string {
	p.buf.Reset()
	p.printModule(module)
	return p.buf.String()
}

// ----------------------------------------------------------------------------
// Output Helpers
// ----------------------------------------------------------------------------

func (p *Printer) print(s string) {
	p.buf.WriteString(s)
	p.updatePosition(s)
	p.needsSpace = false
}

// updatePosition updates output line and column after printing a string.
func (p *Printer) updatePosition(s string) {
	for _, c := range s {
		if c == '\n' {
			p.outputLine++
			p.outputCol = 0
		} else {
			p.outputCol++
		}
	}
}

func (p *Printer) printSpace() {
	if !p.options.MinifyWhitespace {
		p.buf.WriteByte(' ')
		p.outputCol++
	} else if p.needsSpace {
		p.buf.WriteByte(' ')
		p.outputCol++
	}
	p.needsSpace = false
}

func (p *Printer) printNewline() {
	if !p.options.MinifyWhitespace {
		p.buf.WriteByte('\n')
		p.outputLine++
		p.outputCol = 0
		for i := 0; i < p.indent; i++ {
			p.buf.WriteString("    ")
			p.outputCol += 4
		}
	}
	p.needsSpace = false
}

func (p *Printer) printSemicolon() {
	p.print(";")
	p.printNewline()
}

func (p *Printer) printName(ref ast.Ref) {
	if !ref.IsValid() || int(ref.InnerIndex) >= len(p.symbols) {
		return
	}

	sym := &p.symbols[ref.InnerIndex]

	// Record source map mapping before printing
	if p.options.SourceMapGen != nil {
		// Determine if we need to record the original name
		originalName := ""
		if p.options.MinifyIdentifiers && p.options.Renamer != nil {
			renamedName := p.options.Renamer.NameForSymbol(ref)
			if renamedName != sym.OriginalName {
				originalName = sym.OriginalName
			}
		}
		p.options.SourceMapGen.AddMapping(p.outputLine, p.outputCol, int(sym.Loc.Start), originalName)
	}

	// Print the name
	if p.options.MinifyIdentifiers && p.options.Renamer != nil {
		p.print(p.options.Renamer.NameForSymbol(ref))
	} else {
		p.print(sym.OriginalName)
	}
}

// ----------------------------------------------------------------------------
// Module Printing
// ----------------------------------------------------------------------------

func (p *Printer) printModule(m *ast.Module) {
	// Directives
	for _, dir := range m.Directives {
		p.printDirective(dir)
	}

	if len(m.Directives) > 0 && len(m.Declarations) > 0 {
		p.printNewline()
	}

	// Filter declarations based on tree shaking
	var liveDecls []ast.Decl
	if p.options.TreeShaking {
		for _, decl := range m.Declarations {
			if dce.IsDeclarationLive(decl, p.symbols) {
				liveDecls = append(liveDecls, decl)
			}
		}
	} else {
		liveDecls = m.Declarations
	}

	// Declarations
	for i, decl := range liveDecls {
		p.printDecl(decl)

		if i < len(liveDecls)-1 && !p.options.MinifyWhitespace {
			p.printNewline()
		}
	}
}

func (p *Printer) printDirective(d ast.Directive) {
	switch dir := d.(type) {
	case *ast.EnableDirective:
		p.print("enable ")
		for i, feat := range dir.Features {
			if i > 0 {
				p.print(",")
				p.printSpace()
			}
			p.print(feat)
		}
		p.printSemicolon()

	case *ast.RequiresDirective:
		p.print("requires ")
		for i, feat := range dir.Features {
			if i > 0 {
				p.print(",")
				p.printSpace()
			}
			p.print(feat)
		}
		p.printSemicolon()

	case *ast.DiagnosticDirective:
		p.print("diagnostic(")
		p.print(dir.Severity)
		p.print(",")
		p.printSpace()
		p.print(dir.Rule)
		p.print(")")
		p.printSemicolon()
	}
}

// ----------------------------------------------------------------------------
// Declaration Printing
// ----------------------------------------------------------------------------

func (p *Printer) printDecl(d ast.Decl) {
	switch decl := d.(type) {
	case *ast.ConstDecl:
		p.print("const ")
		p.printName(decl.Name)
		if decl.Type != nil {
			p.print(":")
			p.printSpace()
			p.printType(decl.Type)
		}
		p.printSpace()
		p.print("=")
		p.printSpace()
		p.printExpr(decl.Initializer)
		p.printSemicolon()

	case *ast.OverrideDecl:
		p.printAttributes(decl.Attributes)
		p.print("override ")
		p.printName(decl.Name)
		if decl.Type != nil {
			p.print(":")
			p.printSpace()
			p.printType(decl.Type)
		}
		if decl.Initializer != nil {
			p.printSpace()
			p.print("=")
			p.printSpace()
			p.printExpr(decl.Initializer)
		}
		p.printSemicolon()

	case *ast.VarDecl:
		p.printAttributes(decl.Attributes)
		p.print("var")
		if decl.AddressSpace != ast.AddressSpaceNone {
			p.print("<")
			p.print(decl.AddressSpace.String())
			if decl.AccessMode != ast.AccessModeNone {
				p.print(",")
				p.printSpace()
				p.print(decl.AccessMode.String())
			}
			p.print(">")
		}
		p.print(" ")
		p.printName(decl.Name)
		if decl.Type != nil {
			p.print(":")
			p.printSpace()
			p.printType(decl.Type)
		}
		if decl.Initializer != nil {
			p.printSpace()
			p.print("=")
			p.printSpace()
			p.printExpr(decl.Initializer)
		}
		p.printSemicolon()

	case *ast.LetDecl:
		p.print("let ")
		p.printName(decl.Name)
		if decl.Type != nil {
			p.print(":")
			p.printSpace()
			p.printType(decl.Type)
		}
		p.printSpace()
		p.print("=")
		p.printSpace()
		p.printExpr(decl.Initializer)
		p.printSemicolon()

	case *ast.FunctionDecl:
		p.printAttributes(decl.Attributes)
		p.print("fn ")
		p.printName(decl.Name)
		p.print("(")
		for i, param := range decl.Parameters {
			if i > 0 {
				p.print(",")
				p.printSpace()
			}
			p.printAttributes(param.Attributes)
			p.printName(param.Name)
			p.print(":")
			p.printSpace()
			p.printType(param.Type)
		}
		p.print(")")
		if decl.ReturnType != nil {
			p.printSpace()
			p.print("->")
			p.printSpace()
			p.printAttributes(decl.ReturnAttr)
			p.printType(decl.ReturnType)
		}
		p.printSpace()
		p.printCompoundStmt(decl.Body)
		p.printNewline()

	case *ast.StructDecl:
		p.print("struct ")
		p.printName(decl.Name)
		p.printSpace()
		p.print("{")
		p.indent++
		for i, member := range decl.Members {
			p.printNewline()
			p.printAttributes(member.Attributes)
			p.printName(member.Name)
			p.print(":")
			p.printSpace()
			p.printType(member.Type)
			if i < len(decl.Members)-1 {
				p.print(",")
			}
		}
		p.indent--
		p.printNewline()
		p.print("}")
		p.printNewline()

	case *ast.AliasDecl:
		p.print("alias ")
		p.printName(decl.Name)
		p.printSpace()
		p.print("=")
		p.printSpace()
		p.printType(decl.Type)
		p.printSemicolon()

	case *ast.ConstAssertDecl:
		p.print("const_assert ")
		p.printExpr(decl.Expr)
		p.printSemicolon()
	}
}

// ----------------------------------------------------------------------------
// Attribute Printing
// ----------------------------------------------------------------------------

func (p *Printer) printAttributes(attrs []ast.Attribute) {
	for _, attr := range attrs {
		p.print("@")
		p.print(attr.Name)
		if len(attr.Args) > 0 {
			p.print("(")
			for i, arg := range attr.Args {
				if i > 0 {
					p.print(",")
					p.printSpace()
				}
				p.printExpr(arg)
			}
			p.print(")")
		}
		// After an attribute, we need a space before the next token
		// to avoid things like @vertexfn becoming one token
		p.needsSpace = true
		p.printSpace()
	}
}

// ----------------------------------------------------------------------------
// Type Printing
// ----------------------------------------------------------------------------

func (p *Printer) printType(t ast.Type) {
	switch typ := t.(type) {
	case *ast.IdentType:
		// Use renamed name if the type reference was bound to a symbol
		if typ.Ref.IsValid() {
			p.printName(typ.Ref)
		} else {
			p.print(typ.Name)
		}

	case *ast.VecType:
		if typ.Shorthand != "" {
			p.print(typ.Shorthand)
		} else {
			p.print("vec")
			p.print(string('0' + typ.Size))
			p.print("<")
			p.printType(typ.ElemType)
			p.print(">")
			p.needsSpace = true // Prevent >= from forming
		}

	case *ast.MatType:
		if typ.Shorthand != "" {
			p.print(typ.Shorthand)
		} else {
			p.print("mat")
			p.print(string('0' + typ.Cols))
			p.print("x")
			p.print(string('0' + typ.Rows))
			p.print("<")
			p.printType(typ.ElemType)
			p.print(">")
			p.needsSpace = true // Prevent >= from forming
		}

	case *ast.ArrayType:
		p.print("array<")
		p.printType(typ.ElemType)
		if typ.Size != nil {
			p.print(",")
			p.printSpace()
			p.printExpr(typ.Size)
		}
		p.print(">")
		p.needsSpace = true // Prevent >= from forming

	case *ast.PtrType:
		p.print("ptr<")
		p.print(typ.AddressSpace.String())
		p.print(",")
		p.printSpace()
		p.printType(typ.ElemType)
		if typ.AccessMode != ast.AccessModeNone {
			p.print(",")
			p.printSpace()
			p.print(typ.AccessMode.String())
		}
		p.print(">")
		p.needsSpace = true // Prevent >= from forming

	case *ast.AtomicType:
		p.print("atomic<")
		p.printType(typ.ElemType)
		p.print(">")
		p.needsSpace = true // Prevent >= from forming

	case *ast.SamplerType:
		if typ.Comparison {
			p.print("sampler_comparison")
		} else {
			p.print("sampler")
		}

	case *ast.TextureType:
		p.printTextureType(typ)
	}
}

func (p *Printer) printTextureType(t *ast.TextureType) {
	prefix := "texture_"
	switch t.Kind {
	case ast.TextureMultisampled:
		prefix = "texture_multisampled_"
	case ast.TextureStorage:
		prefix = "texture_storage_"
	case ast.TextureDepth:
		prefix = "texture_depth_"
	case ast.TextureDepthMultisampled:
		prefix = "texture_depth_multisampled_"
	case ast.TextureExternal:
		p.print("texture_external")
		return
	}

	p.print(prefix)
	switch t.Dimension {
	case ast.Texture1D:
		p.print("1d")
	case ast.Texture2D:
		p.print("2d")
	case ast.Texture2DArray:
		p.print("2d_array")
	case ast.Texture3D:
		p.print("3d")
	case ast.TextureCube:
		p.print("cube")
	case ast.TextureCubeArray:
		p.print("cube_array")
	}

	// Template arguments
	if t.SampledType != nil {
		p.print("<")
		p.printType(t.SampledType)
		p.print(">")
		p.needsSpace = true // Prevent >= from forming
	} else if t.TexelFormat != "" {
		p.print("<")
		p.print(t.TexelFormat)
		p.print(",")
		p.printSpace()
		p.print(t.AccessMode.String())
		p.print(">")
		p.needsSpace = true // Prevent >= from forming
	}
}

// ----------------------------------------------------------------------------
// Expression Printing
// ----------------------------------------------------------------------------

func (p *Printer) printExpr(e ast.Expr) {
	switch expr := e.(type) {
	case *ast.IdentExpr:
		if expr.Ref.IsValid() {
			p.printName(expr.Ref)
		} else {
			p.print(expr.Name)
		}

	case *ast.LiteralExpr:
		p.printLiteral(expr)

	case *ast.BinaryExpr:
		p.printBinaryExpr(expr)

	case *ast.UnaryExpr:
		p.printUnaryExpr(expr)

	case *ast.CallExpr:
		// Check for templated type constructor first
		if expr.TemplateType != nil {
			p.printType(expr.TemplateType)
		} else {
			p.printExpr(expr.Func)
		}
		p.print("(")
		for i, arg := range expr.Args {
			if i > 0 {
				p.print(",")
				p.printSpace()
			}
			p.printExpr(arg)
		}
		p.print(")")

	case *ast.IndexExpr:
		p.printExpr(expr.Base)
		p.print("[")
		p.printExpr(expr.Index)
		p.print("]")

	case *ast.MemberExpr:
		p.printExpr(expr.Base)
		p.print(".")
		p.print(expr.Member)

	case *ast.ParenExpr:
		p.print("(")
		p.printExpr(expr.Expr)
		p.print(")")
	}
}

func (p *Printer) printLiteral(lit *ast.LiteralExpr) {
	if p.options.MinifySyntax {
		// Apply numeric literal optimizations
		p.print(optimizeNumericLiteral(lit.Value))
	} else {
		p.print(lit.Value)
	}
}

func (p *Printer) printBinaryExpr(expr *ast.BinaryExpr) {
	// TODO: Parenthesization based on precedence
	p.printExpr(expr.Left)
	p.printSpace()
	p.print(binaryOpString(expr.Op))
	p.printSpace()
	p.printExpr(expr.Right)
}

func (p *Printer) printUnaryExpr(expr *ast.UnaryExpr) {
	p.print(unaryOpString(expr.Op))
	p.printExpr(expr.Operand)
}

func binaryOpString(op ast.BinaryOp) string {
	ops := [...]string{
		"+", "-", "*", "/", "%",
		"&", "|", "^", "<<", ">>",
		"&&", "||",
		"==", "!=", "<", "<=", ">", ">=",
	}
	if int(op) < len(ops) {
		return ops[op]
	}
	return "?"
}

func unaryOpString(op ast.UnaryOp) string {
	ops := [...]string{"-", "!", "~", "*", "&"}
	if int(op) < len(ops) {
		return ops[op]
	}
	return "?"
}

// ----------------------------------------------------------------------------
// Statement Printing
// ----------------------------------------------------------------------------

func (p *Printer) printCompoundStmt(stmt *ast.CompoundStmt) {
	p.print("{")
	p.indent++
	for _, s := range stmt.Stmts {
		p.printNewline()
		p.printStmtNoTrailingNewline(s)
	}
	p.indent--
	p.printNewline()
	p.print("}")
}

// printStmtNoTrailingNewline prints a statement but doesn't add trailing newlines.
// The compound statement handles the newlines between statements.
func (p *Printer) printStmtNoTrailingNewline(s ast.Stmt) {
	switch stmt := s.(type) {
	case *ast.CompoundStmt:
		p.print("{")
		p.indent++
		for _, sub := range stmt.Stmts {
			p.printNewline()
			p.printStmtNoTrailingNewline(sub)
		}
		p.indent--
		p.printNewline()
		p.print("}")

	case *ast.ReturnStmt:
		p.print("return")
		if stmt.Value != nil {
			p.print(" ")
			p.printExpr(stmt.Value)
		}
		p.print(";")

	case *ast.IfStmt:
		p.print("if ")
		p.printExpr(stmt.Condition)
		p.printSpace()
		p.print("{")
		p.indent++
		for _, sub := range stmt.Body.Stmts {
			p.printNewline()
			p.printStmtNoTrailingNewline(sub)
		}
		p.indent--
		p.printNewline()
		p.print("}")
		if stmt.Else != nil {
			// Check if else branch is another if statement (else if)
			if _, isElseIf := stmt.Else.(*ast.IfStmt); isElseIf {
				p.print(" else if ")
				elseIf := stmt.Else.(*ast.IfStmt)
				p.printExpr(elseIf.Condition)
				p.printSpace()
				p.print("{")
				p.indent++
				for _, sub := range elseIf.Body.Stmts {
					p.printNewline()
					p.printStmtNoTrailingNewline(sub)
				}
				p.indent--
				p.printNewline()
				p.print("}")
				if elseIf.Else != nil {
					p.printElseChainNoTrailing(elseIf.Else)
				}
			} else {
				p.print(" else")
				p.printSpace()
				p.printStmtNoTrailingNewline(stmt.Else)
			}
		}

	case *ast.SwitchStmt:
		p.print("switch ")
		p.printExpr(stmt.Expr)
		p.printSpace()
		p.print("{")
		p.indent++
		for _, c := range stmt.Cases {
			p.printNewline()
			if c.Selectors == nil {
				p.print("default")
			} else {
				p.print("case ")
				for i, sel := range c.Selectors {
					if i > 0 {
						p.print(",")
						p.printSpace()
					}
					p.printExpr(sel)
				}
			}
			p.print(":")
			p.printSpace()
			p.print("{")
			p.indent++
			for _, sub := range c.Body.Stmts {
				p.printNewline()
				p.printStmtNoTrailingNewline(sub)
			}
			p.indent--
			p.printNewline()
			p.print("}")
		}
		p.indent--
		p.printNewline()
		p.print("}")

	case *ast.ForStmt:
		p.printForStmt(stmt)

	case *ast.WhileStmt:
		p.print("while ")
		p.printExpr(stmt.Condition)
		p.printSpace()
		p.print("{")
		p.indent++
		for _, sub := range stmt.Body.Stmts {
			p.printNewline()
			p.printStmtNoTrailingNewline(sub)
		}
		p.indent--
		p.printNewline()
		p.print("}")

	case *ast.LoopStmt:
		p.print("loop")
		p.printSpace()
		p.print("{")
		p.indent++
		for _, sub := range stmt.Body.Stmts {
			p.printNewline()
			p.printStmtNoTrailingNewline(sub)
		}
		p.indent--
		p.printNewline()
		p.print("}")
		if stmt.Continuing != nil {
			p.print(" continuing")
			p.printSpace()
			p.print("{")
			p.indent++
			for _, sub := range stmt.Continuing.Stmts {
				p.printNewline()
				p.printStmtNoTrailingNewline(sub)
			}
			p.indent--
			p.printNewline()
			p.print("}")
		}

	case *ast.BreakStmt:
		p.print("break;")

	case *ast.BreakIfStmt:
		p.print("break if ")
		p.printExpr(stmt.Condition)
		p.print(";")

	case *ast.ContinueStmt:
		p.print("continue;")

	case *ast.DiscardStmt:
		p.print("discard;")

	case *ast.AssignStmt:
		p.printExpr(stmt.Left)
		p.printSpace()
		p.print(assignOpString(stmt.Op))
		p.printSpace()
		p.printExpr(stmt.Right)
		p.print(";")

	case *ast.IncrDecrStmt:
		p.printExpr(stmt.Expr)
		if stmt.Increment {
			p.print("++")
		} else {
			p.print("--")
		}
		p.print(";")

	case *ast.CallStmt:
		p.printExpr(stmt.Call)
		p.print(";")

	case *ast.DeclStmt:
		p.printDeclNoTrailingNewline(stmt.Decl)
	}
}

// printDeclNoTrailingNewline prints a declaration without trailing newline.
func (p *Printer) printDeclNoTrailingNewline(d ast.Decl) {
	switch decl := d.(type) {
	case *ast.ConstDecl:
		p.print("const ")
		p.printName(decl.Name)
		if decl.Type != nil {
			p.print(":")
			p.printSpace()
			p.printType(decl.Type)
		}
		p.printSpace()
		p.print("=")
		p.printSpace()
		p.printExpr(decl.Initializer)
		p.print(";")

	case *ast.LetDecl:
		p.print("let ")
		p.printName(decl.Name)
		if decl.Type != nil {
			p.print(":")
			p.printSpace()
			p.printType(decl.Type)
		}
		p.printSpace()
		p.print("=")
		p.printSpace()
		p.printExpr(decl.Initializer)
		p.print(";")

	case *ast.VarDecl:
		p.printAttributes(decl.Attributes)
		p.print("var")
		if decl.AddressSpace != ast.AddressSpaceNone {
			p.print("<")
			p.print(decl.AddressSpace.String())
			if decl.AccessMode != ast.AccessModeNone {
				p.print(",")
				p.printSpace()
				p.print(decl.AccessMode.String())
			}
			p.print(">")
		}
		p.print(" ")
		p.printName(decl.Name)
		if decl.Type != nil {
			p.print(":")
			p.printSpace()
			p.printType(decl.Type)
		}
		if decl.Initializer != nil {
			p.printSpace()
			p.print("=")
			p.printSpace()
			p.printExpr(decl.Initializer)
		}
		p.print(";")

	default:
		// For other declarations, use regular printing
		p.printDecl(d)
	}
}

func (p *Printer) printElseChainNoTrailing(stmt ast.Stmt) {
	if ifStmt, isIf := stmt.(*ast.IfStmt); isIf {
		p.print(" else if ")
		p.printExpr(ifStmt.Condition)
		p.printSpace()
		p.print("{")
		p.indent++
		for _, sub := range ifStmt.Body.Stmts {
			p.printNewline()
			p.printStmtNoTrailingNewline(sub)
		}
		p.indent--
		p.printNewline()
		p.print("}")
		if ifStmt.Else != nil {
			p.printElseChainNoTrailing(ifStmt.Else)
		}
	} else {
		p.print(" else")
		p.printSpace()
		p.printStmtNoTrailingNewline(stmt)
	}
}

func (p *Printer) printForStmt(stmt *ast.ForStmt) {
	p.print("for")
	p.printSpace()
	p.print("(")
	if stmt.Init != nil {
		// Print init without trailing semicolon
		p.printForInit(stmt.Init)
	}
	p.print(";")
	p.printSpace()
	if stmt.Condition != nil {
		p.printExpr(stmt.Condition)
	}
	p.print(";")
	p.printSpace()
	if stmt.Update != nil {
		// Print update without trailing semicolon
		p.printForUpdate(stmt.Update)
	}
	p.print(")")
	p.printSpace()
	p.printCompoundStmt(stmt.Body)
}

// printForInit prints the init statement in a for loop without trailing semicolon
func (p *Printer) printForInit(s ast.Stmt) {
	switch stmt := s.(type) {
	case *ast.DeclStmt:
		switch decl := stmt.Decl.(type) {
		case *ast.VarDecl:
			p.print("var ")
			p.printName(decl.Name)
			if decl.Type != nil {
				p.print(":")
				p.printSpace()
				p.printType(decl.Type)
			}
			if decl.Initializer != nil {
				p.printSpace()
				p.print("=")
				p.printSpace()
				p.printExpr(decl.Initializer)
			}
		case *ast.LetDecl:
			p.print("let ")
			p.printName(decl.Name)
			if decl.Type != nil {
				p.print(":")
				p.printSpace()
				p.printType(decl.Type)
			}
			p.printSpace()
			p.print("=")
			p.printSpace()
			p.printExpr(decl.Initializer)
		}
	case *ast.AssignStmt:
		p.printExpr(stmt.Left)
		p.printSpace()
		p.print(assignOpString(stmt.Op))
		p.printSpace()
		p.printExpr(stmt.Right)
	}
}

// printForUpdate prints the update statement in a for loop without trailing semicolon
func (p *Printer) printForUpdate(s ast.Stmt) {
	switch stmt := s.(type) {
	case *ast.IncrDecrStmt:
		p.printExpr(stmt.Expr)
		if stmt.Increment {
			p.print("++")
		} else {
			p.print("--")
		}
	case *ast.AssignStmt:
		p.printExpr(stmt.Left)
		p.printSpace()
		p.print(assignOpString(stmt.Op))
		p.printSpace()
		p.printExpr(stmt.Right)
	case *ast.CallStmt:
		p.printExpr(stmt.Call)
	}
}

func assignOpString(op ast.AssignOp) string {
	ops := [...]string{
		"=", "+=", "-=", "*=", "/=", "%=",
		"&=", "|=", "^=", "<<=", ">>=",
	}
	if int(op) < len(ops) {
		return ops[op]
	}
	return "="
}

// ----------------------------------------------------------------------------
// Minification Helpers
// ----------------------------------------------------------------------------

// optimizeNumericLiteral applies minification to numeric literals.
func optimizeNumericLiteral(value string) string {
	// TODO: Implement optimizations:
	// - 0.5 -> .5
	// - 1.0 -> 1.
	// - 1000000.0 -> 1e6
	// - Remove redundant type suffixes when type is clear from context
	return value
}
