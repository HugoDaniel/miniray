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
)

// ExternalAlias represents an alias for an external binding.
type ExternalAlias struct {
	OriginalRef  ast.Ref
	OriginalName string
	AliasName    string
}

// Options controls printer output.
type Options struct {
	// MinifyWhitespace removes unnecessary whitespace
	MinifyWhitespace bool

	// MinifyIdentifiers uses short names for identifiers
	MinifyIdentifiers bool

	// MinifySyntax applies syntax-level optimizations
	MinifySyntax bool

	// Renamer provides minified names (nil for no renaming)
	Renamer Renamer

	// ExternalAliases maps external binding refs to their short aliases
	ExternalAliases []ExternalAlias
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

	// External binding alias lookup (ref -> alias name)
	externalAliasMap map[uint32]string
}

// New creates a new printer.
func New(options Options, symbols []ast.Symbol) *Printer {
	p := &Printer{
		options:          options,
		symbols:          symbols,
		externalAliasMap: make(map[uint32]string),
	}

	// Build alias lookup
	for _, alias := range options.ExternalAliases {
		p.externalAliasMap[alias.OriginalRef.InnerIndex] = alias.AliasName
	}

	return p
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
	p.needsSpace = false
}

func (p *Printer) printSpace() {
	if !p.options.MinifyWhitespace {
		p.buf.WriteByte(' ')
	} else if p.needsSpace {
		p.buf.WriteByte(' ')
	}
	p.needsSpace = false
}

func (p *Printer) printNewline() {
	if !p.options.MinifyWhitespace {
		p.buf.WriteByte('\n')
		for i := 0; i < p.indent; i++ {
			p.buf.WriteString("    ")
		}
	}
	p.needsSpace = false
}

func (p *Printer) printSemicolon() {
	p.print(";")
	p.printNewline()
}

func (p *Printer) printName(ref ast.Ref) {
	// Check if this is an external binding with an alias
	if ref.IsValid() {
		if aliasName, ok := p.externalAliasMap[ref.InnerIndex]; ok {
			p.print(aliasName)
			return
		}
	}

	if p.options.MinifyIdentifiers && p.options.Renamer != nil {
		p.print(p.options.Renamer.NameForSymbol(ref))
	} else if ref.IsValid() && int(ref.InnerIndex) < len(p.symbols) {
		p.print(p.symbols[ref.InnerIndex].OriginalName)
	}
}

// printOriginalName prints the original symbol name (ignoring aliases).
// Used for declarations of external bindings.
func (p *Printer) printOriginalName(ref ast.Ref) {
	if ref.IsValid() && int(ref.InnerIndex) < len(p.symbols) {
		p.print(p.symbols[ref.InnerIndex].OriginalName)
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

	// Declarations - track when to inject aliases
	aliasesInjected := false
	for i, decl := range m.Declarations {
		p.printDecl(decl)

		// After printing all external bindings, inject their aliases
		if !aliasesInjected && len(p.options.ExternalAliases) > 0 {
			// Check if next declaration is not an external binding var
			nextIsExternalBinding := false
			if i+1 < len(m.Declarations) {
				if varDecl, ok := m.Declarations[i+1].(*ast.VarDecl); ok {
					if varDecl.AddressSpace == ast.AddressSpaceUniform ||
						varDecl.AddressSpace == ast.AddressSpaceStorage {
						nextIsExternalBinding = true
					}
				}
			}

			// If current is external binding and next is not, inject aliases
			if varDecl, ok := decl.(*ast.VarDecl); ok {
				isCurrentExternal := varDecl.AddressSpace == ast.AddressSpaceUniform ||
					varDecl.AddressSpace == ast.AddressSpaceStorage
				if isCurrentExternal && !nextIsExternalBinding {
					p.printExternalAliases()
					aliasesInjected = true
				}
			}
		}

		if i < len(m.Declarations)-1 && !p.options.MinifyWhitespace {
			p.printNewline()
		}
	}
}

// printExternalAliases prints let declarations for external binding aliases.
func (p *Printer) printExternalAliases() {
	for _, alias := range p.options.ExternalAliases {
		p.print("let ")
		p.print(alias.AliasName)
		p.print("=")
		p.print(alias.OriginalName)
		p.print(";")
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
		// External bindings keep their original names in declarations only when using aliases
		// (i.e., when externalAliasMap is populated). If not using aliases, mangle normally.
		isExternal := decl.AddressSpace == ast.AddressSpaceUniform || decl.AddressSpace == ast.AddressSpaceStorage
		if isExternal && len(p.externalAliasMap) > 0 {
			p.printOriginalName(decl.Name)
		} else {
			p.printName(decl.Name)
		}
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

	case *ast.AtomicType:
		p.print("atomic<")
		p.printType(typ.ElemType)
		p.print(">")

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
	} else if t.TexelFormat != "" {
		p.print("<")
		p.print(t.TexelFormat)
		p.print(",")
		p.printSpace()
		p.print(t.AccessMode.String())
		p.print(">")
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
		p.printExpr(expr.Func)
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

func (p *Printer) printStmt(s ast.Stmt) {
	switch stmt := s.(type) {
	case *ast.CompoundStmt:
		p.printCompoundStmt(stmt)

	case *ast.ReturnStmt:
		p.print("return")
		if stmt.Value != nil {
			p.print(" ")
			p.printExpr(stmt.Value)
		}
		p.printSemicolon()

	case *ast.IfStmt:
		p.printIfStmt(stmt)

	case *ast.SwitchStmt:
		p.printSwitchStmt(stmt)

	case *ast.ForStmt:
		p.printForStmt(stmt)

	case *ast.WhileStmt:
		p.print("while ")
		p.printExpr(stmt.Condition)
		p.printSpace()
		p.printCompoundStmt(stmt.Body)

	case *ast.LoopStmt:
		p.print("loop")
		p.printSpace()
		p.printCompoundStmt(stmt.Body)
		if stmt.Continuing != nil {
			p.print("continuing")
			p.printCompoundStmt(stmt.Continuing)
		}

	case *ast.BreakStmt:
		p.print("break")
		p.printSemicolon()

	case *ast.BreakIfStmt:
		p.print("break if ")
		p.printExpr(stmt.Condition)
		p.printSemicolon()

	case *ast.ContinueStmt:
		p.print("continue")
		p.printSemicolon()

	case *ast.DiscardStmt:
		p.print("discard")
		p.printSemicolon()

	case *ast.AssignStmt:
		p.printExpr(stmt.Left)
		p.printSpace()
		p.print(assignOpString(stmt.Op))
		p.printSpace()
		p.printExpr(stmt.Right)
		p.printSemicolon()

	case *ast.IncrDecrStmt:
		p.printExpr(stmt.Expr)
		if stmt.Increment {
			p.print("++")
		} else {
			p.print("--")
		}
		p.printSemicolon()

	case *ast.CallStmt:
		p.printExpr(stmt.Call)
		p.printSemicolon()

	case *ast.DeclStmt:
		p.printDecl(stmt.Decl)
	}
}

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
			p.print(" else")
			p.printSpace()
			p.printStmtNoTrailingNewline(stmt.Else)
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

func (p *Printer) printIfStmt(stmt *ast.IfStmt) {
	p.print("if ")
	p.printExpr(stmt.Condition)
	p.printSpace()
	p.printCompoundStmt(stmt.Body)
	if stmt.Else != nil {
		p.print("else")
		p.printSpace()
		p.printStmt(stmt.Else)
	}
}

func (p *Printer) printSwitchStmt(stmt *ast.SwitchStmt) {
	p.print("switch ")
	p.printExpr(stmt.Expr)
	p.printSpace()
	p.print("{")
	p.indent++
	p.printNewline()
	for _, c := range stmt.Cases {
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
		p.printCompoundStmt(c.Body)
	}
	p.indent--
	p.print("}")
	p.printNewline()
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
