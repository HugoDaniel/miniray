// Package ast defines the Abstract Syntax Tree types for WGSL.
//
// The AST is designed to be:
// - Complete: Represents all valid WGSL constructs
// - Efficient: Minimizes allocations and pointer chasing
// - Transformable: Supports in-place modifications for minification
package ast

import "github.com/HugoDaniel/miniray/internal/lexer"

// ----------------------------------------------------------------------------
// Side Effects and Purity Tracking
// ----------------------------------------------------------------------------

// SideEffects indicates whether an expression has side effects.
type SideEffects uint8

const (
	// CouldHaveSideEffects means we can't prove the expression is pure.
	CouldHaveSideEffects SideEffects = iota
	// NoSideEffects means the expression is definitely pure.
	NoSideEffects
)

// ExprFlags are bitflags for expression properties.
type ExprFlags uint8

const (
	// ExprFlagCanBeRemovedIfUnused marks an expression as removable if its
	// result is not used. This is set during the visit pass for pure expressions.
	ExprFlagCanBeRemovedIfUnused ExprFlags = 1 << iota

	// ExprFlagCallCanBeUnwrappedIfUnused marks a call expression where the
	// call target can be removed if unused, keeping only side-effecting args.
	ExprFlagCallCanBeUnwrappedIfUnused

	// ExprFlagIsConstant marks an expression that evaluates to a compile-time constant.
	ExprFlagIsConstant

	// ExprFlagFromPureFunction marks a call to a known pure built-in function.
	ExprFlagFromPureFunction
)

// Has returns true if the flag is set.
func (f ExprFlags) Has(flag ExprFlags) bool {
	return (f & flag) != 0
}

// ----------------------------------------------------------------------------
// Source Location
// ----------------------------------------------------------------------------

// Loc represents a location in source code.
type Loc struct {
	Start int32 // Byte offset of start
}

// Range represents a range in source code.
type Range struct {
	Loc Loc
	Len int32
}

// ----------------------------------------------------------------------------
// Symbols and References
// ----------------------------------------------------------------------------

// Ref is a reference to a symbol in the symbol table.
// Using a struct with two indices allows efficient symbol table lookups
// while supporting multiple source files (future extension).
type Ref struct {
	SourceIndex uint32
	InnerIndex  uint32
}

// InvalidRef returns an invalid reference.
func InvalidRef() Ref {
	return Ref{SourceIndex: ^uint32(0), InnerIndex: ^uint32(0)}
}

// IsValid returns true if this is a valid reference.
func (r Ref) IsValid() bool {
	return r.SourceIndex != ^uint32(0)
}

// Symbol represents a declared name in WGSL.
type Symbol struct {
	// The original name as written in source
	OriginalName string

	// Location of the declaration
	Loc Loc

	// What kind of symbol this is
	Kind SymbolKind

	// Flags for special handling
	Flags SymbolFlags

	// For minification: the assigned slot for this symbol
	NestedScopeSlot Index32

	// Usage count for frequency-based renaming
	UseCount uint32
}

// SymbolKind indicates what a symbol represents.
type SymbolKind uint8

const (
	SymbolUnbound    SymbolKind = iota // Not yet resolved
	SymbolConst                        // const declaration
	SymbolOverride                     // override declaration
	SymbolLet                          // let declaration
	SymbolVar                          // var declaration
	SymbolFunction                     // fn declaration
	SymbolStruct                       // struct declaration
	SymbolAlias                        // type alias
	SymbolParameter                    // function parameter
	SymbolBuiltin                      // built-in function/type
	SymbolMember                       // struct member
)

// SymbolFlags are bitflags for symbol properties.
type SymbolFlags uint16

const (
	// MustNotBeRenamed prevents minification of this symbol.
	// Used for entry points, API-facing names, etc.
	MustNotBeRenamed SymbolFlags = 1 << iota

	// IsEntryPoint marks entry point functions.
	IsEntryPoint

	// IsAPIFacing marks names visible to the WebGPU API.
	IsAPIFacing

	// IsBuiltin marks built-in functions and types.
	IsBuiltin

	// IsExternalBinding marks uniform/storage variables that are bound via @group/@binding.
	// These keep their original names in declarations but get aliased before use.
	IsExternalBinding

	// IsLive marks symbols that are reachable from entry points.
	// Used by dead code elimination to filter unused declarations.
	IsLive
)

// Has returns true if the flag is set.
func (f SymbolFlags) Has(flag SymbolFlags) bool {
	return (f & flag) != 0
}

// Index32 is an optional 32-bit index.
type Index32 struct {
	value uint32
	valid bool
}

// MakeIndex32 creates a valid index.
func MakeIndex32(i uint32) Index32 {
	return Index32{value: i, valid: true}
}

// IsValid returns true if this index is valid.
func (i Index32) IsValid() bool {
	return i.valid
}

// GetIndex returns the index value.
func (i Index32) GetIndex() uint32 {
	return i.value
}

// ----------------------------------------------------------------------------
// Module (Top Level)
// ----------------------------------------------------------------------------

// Module represents a complete WGSL module.
type Module struct {
	// Source information
	Source     string // Original source text
	SourcePath string // File path (for error messages)

	// Top-level declarations in order
	Directives  []Directive
	Declarations []Decl

	// Symbol table
	Symbols []Symbol

	// Module-level scope
	Scope *Scope
}

// ----------------------------------------------------------------------------
// Directives
// ----------------------------------------------------------------------------

// Directive represents a top-level directive (enable, requires, diagnostic).
type Directive interface {
	isDirective()
}

// EnableDirective represents: enable feature1, feature2;
type EnableDirective struct {
	Loc      Loc
	Features []string
}

func (*EnableDirective) isDirective() {}

// RequiresDirective represents: requires feature1, feature2;
type RequiresDirective struct {
	Loc      Loc
	Features []string
}

func (*RequiresDirective) isDirective() {}

// DiagnosticDirective represents: diagnostic(severity, rule);
type DiagnosticDirective struct {
	Loc      Loc
	Severity string
	Rule     string
}

func (*DiagnosticDirective) isDirective() {}

// ----------------------------------------------------------------------------
// Declarations
// ----------------------------------------------------------------------------

// Decl represents a top-level or local declaration.
type Decl interface {
	isDecl()
}

// ConstDecl represents: const name [: type] = expr;
type ConstDecl struct {
	Loc         Loc
	Name        Ref
	Type        Type   // nil if inferred
	Initializer Expr
}

func (*ConstDecl) isDecl() {}

// OverrideDecl represents: @id(n) override name [: type] [= expr];
type OverrideDecl struct {
	Loc         Loc
	Attributes  []Attribute
	Name        Ref
	Type        Type // nil if inferred
	Initializer Expr // nil if no default
}

func (*OverrideDecl) isDecl() {}

// VarDecl represents: @group(g) @binding(b) var<space, access> name [: type] [= expr];
type VarDecl struct {
	Loc          Loc
	Attributes   []Attribute
	AddressSpace AddressSpace
	AccessMode   AccessMode
	Name         Ref
	Type         Type
	Initializer  Expr // nil if no initializer
}

func (*VarDecl) isDecl() {}

// LetDecl represents: let name [: type] = expr;
type LetDecl struct {
	Loc         Loc
	Name        Ref
	Type        Type // nil if inferred
	Initializer Expr
}

func (*LetDecl) isDecl() {}

// FunctionDecl represents a function declaration.
type FunctionDecl struct {
	Loc        Loc
	Attributes []Attribute
	Name       Ref
	Parameters []Parameter
	ReturnType Type          // nil for void
	ReturnAttr []Attribute   // Return value attributes
	Body       *CompoundStmt
}

func (*FunctionDecl) isDecl() {}

// Parameter represents a function parameter.
type Parameter struct {
	Loc        Loc
	Attributes []Attribute
	Name       Ref
	Type       Type
}

// StructDecl represents: struct Name { members }
type StructDecl struct {
	Loc     Loc
	Name    Ref
	Members []StructMember
}

func (*StructDecl) isDecl() {}

// StructMember represents a struct field.
type StructMember struct {
	Loc        Loc
	Attributes []Attribute
	Name       Ref
	Type       Type
}

// AliasDecl represents: alias Name = Type;
type AliasDecl struct {
	Loc  Loc
	Name Ref
	Type Type
}

func (*AliasDecl) isDecl() {}

// ConstAssertDecl represents: const_assert expr;
type ConstAssertDecl struct {
	Loc  Loc
	Expr Expr
}

func (*ConstAssertDecl) isDecl() {}

// ----------------------------------------------------------------------------
// Address Spaces and Access Modes
// ----------------------------------------------------------------------------

// AddressSpace represents WGSL address spaces.
type AddressSpace uint8

const (
	AddressSpaceNone AddressSpace = iota
	AddressSpaceFunction
	AddressSpacePrivate
	AddressSpaceWorkgroup
	AddressSpaceUniform
	AddressSpaceStorage
	AddressSpaceHandle // For textures and samplers
)

func (a AddressSpace) String() string {
	switch a {
	case AddressSpaceFunction:
		return "function"
	case AddressSpacePrivate:
		return "private"
	case AddressSpaceWorkgroup:
		return "workgroup"
	case AddressSpaceUniform:
		return "uniform"
	case AddressSpaceStorage:
		return "storage"
	default:
		return ""
	}
}

// AccessMode represents WGSL access modes.
type AccessMode uint8

const (
	AccessModeNone AccessMode = iota
	AccessModeRead
	AccessModeWrite
	AccessModeReadWrite
)

func (a AccessMode) String() string {
	switch a {
	case AccessModeRead:
		return "read"
	case AccessModeWrite:
		return "write"
	case AccessModeReadWrite:
		return "read_write"
	default:
		return ""
	}
}

// ----------------------------------------------------------------------------
// Attributes
// ----------------------------------------------------------------------------

// Attribute represents a WGSL attribute (@name or @name(args)).
type Attribute struct {
	Loc  Loc
	Name string
	Args []Expr // nil for attributes without arguments
}

// ----------------------------------------------------------------------------
// Types
// ----------------------------------------------------------------------------

// Type represents a WGSL type.
type Type interface {
	isType()
}

// IdentType represents a type name (i32, f32, MyStruct, etc.)
type IdentType struct {
	Loc  Loc
	Name string
	Ref  Ref // Resolved reference (for user-defined types)
}

func (*IdentType) isType() {}

// VecType represents vec2<T>, vec3<T>, vec4<T> or vec2f, etc.
type VecType struct {
	Loc       Loc
	Size      uint8 // 2, 3, or 4
	ElemType  Type  // Element type (nil for shorthand like vec3f)
	Shorthand string // "vec3f", "vec4i", etc. (empty if using template)
}

func (*VecType) isType() {}

// MatType represents matCxR<T> or mat4x4f, etc.
type MatType struct {
	Loc       Loc
	Cols      uint8 // 2, 3, or 4
	Rows      uint8 // 2, 3, or 4
	ElemType  Type  // Element type (nil for shorthand)
	Shorthand string
}

func (*MatType) isType() {}

// ArrayType represents array<T, N> or array<T>.
type ArrayType struct {
	Loc      Loc
	ElemType Type
	Size     Expr // nil for runtime-sized arrays
}

func (*ArrayType) isType() {}

// PtrType represents ptr<space, T, access>.
type PtrType struct {
	Loc          Loc
	AddressSpace AddressSpace
	ElemType     Type
	AccessMode   AccessMode
}

func (*PtrType) isType() {}

// AtomicType represents atomic<T>.
type AtomicType struct {
	Loc      Loc
	ElemType Type
}

func (*AtomicType) isType() {}

// SamplerType represents sampler or sampler_comparison.
type SamplerType struct {
	Loc        Loc
	Comparison bool
}

func (*SamplerType) isType() {}

// TextureType represents texture types.
type TextureType struct {
	Loc         Loc
	Kind        TextureKind
	Dimension   TextureDimension
	SampledType Type   // For sampled textures
	TexelFormat string // For storage textures
	AccessMode  AccessMode
}

func (*TextureType) isType() {}

// TextureKind indicates the texture category.
type TextureKind uint8

const (
	TextureSampled TextureKind = iota
	TextureMultisampled
	TextureStorage
	TextureDepth
	TextureDepthMultisampled
	TextureExternal
)

// TextureDimension indicates texture dimensionality.
type TextureDimension uint8

const (
	Texture1D TextureDimension = iota
	Texture2D
	Texture2DArray
	Texture3D
	TextureCube
	TextureCubeArray
)

// ----------------------------------------------------------------------------
// Expressions
// ----------------------------------------------------------------------------

// Expr represents an expression.
type Expr interface {
	isExpr()
}

// IdentExpr represents an identifier reference.
type IdentExpr struct {
	Loc   Loc
	Name  string
	Ref   Ref       // Resolved symbol reference
	Flags ExprFlags // Purity flags
}

func (*IdentExpr) isExpr() {}

// LiteralExpr represents a literal value.
type LiteralExpr struct {
	Loc   Loc
	Kind  lexer.TokenKind
	Value string    // Raw literal text
	Flags ExprFlags // Purity flags (always pure)
}

func (*LiteralExpr) isExpr() {}

// BinaryExpr represents a binary operation.
type BinaryExpr struct {
	Loc   Loc
	Op    BinaryOp
	Left  Expr
	Right Expr
	Flags ExprFlags // Purity flags
}

func (*BinaryExpr) isExpr() {}

// BinaryOp represents binary operators.
type BinaryOp uint8

const (
	BinOpAdd BinaryOp = iota // +
	BinOpSub                 // -
	BinOpMul                 // *
	BinOpDiv                 // /
	BinOpMod                 // %
	BinOpAnd                 // &
	BinOpOr                  // |
	BinOpXor                 // ^
	BinOpShl                 // <<
	BinOpShr                 // >>
	BinOpLogicalAnd          // &&
	BinOpLogicalOr           // ||
	BinOpEq                  // ==
	BinOpNe                  // !=
	BinOpLt                  // <
	BinOpLe                  // <=
	BinOpGt                  // >
	BinOpGe                  // >=
)

// UnaryExpr represents a unary operation.
type UnaryExpr struct {
	Loc     Loc
	Op      UnaryOp
	Operand Expr
	Flags   ExprFlags // Purity flags
}

func (*UnaryExpr) isExpr() {}

// UnaryOp represents unary operators.
type UnaryOp uint8

const (
	UnaryOpNeg   UnaryOp = iota // -
	UnaryOpNot                  // !
	UnaryOpBitNot               // ~
	UnaryOpDeref                // *
	UnaryOpAddr                 // &
)

// CallExpr represents a function call or type constructor.
type CallExpr struct {
	Loc          Loc
	Func         Expr      // IdentExpr for function name (nil if TemplateType is set)
	TemplateType Type      // For templated constructors: array<T, N>, vec2<T>, etc.
	Args         []Expr
	Flags        ExprFlags // Purity flags
}

func (*CallExpr) isExpr() {}

// IndexExpr represents array/vector indexing: base[index]
type IndexExpr struct {
	Loc   Loc
	Base  Expr
	Index Expr
	Flags ExprFlags // Purity flags
}

func (*IndexExpr) isExpr() {}

// MemberExpr represents member access: base.member
type MemberExpr struct {
	Loc    Loc
	Base   Expr
	Member string
	Flags  ExprFlags // Purity flags
}

func (*MemberExpr) isExpr() {}

// ParenExpr represents a parenthesized expression.
type ParenExpr struct {
	Loc   Loc
	Expr  Expr
	Flags ExprFlags // Purity flags (inherits from inner expr)
}

func (*ParenExpr) isExpr() {}

// ----------------------------------------------------------------------------
// Statements
// ----------------------------------------------------------------------------

// Stmt represents a statement.
type Stmt interface {
	isStmt()
}

// CompoundStmt represents a block of statements: { stmts }
type CompoundStmt struct {
	Loc   Loc
	Stmts []Stmt
}

func (*CompoundStmt) isStmt() {}

// ReturnStmt represents: return [expr];
type ReturnStmt struct {
	Loc   Loc
	Value Expr // nil for bare return
}

func (*ReturnStmt) isStmt() {}

// IfStmt represents: if (cond) { } [else [if ...] { }]
type IfStmt struct {
	Loc       Loc
	Condition Expr
	Body      *CompoundStmt
	Else      Stmt // nil, *IfStmt, or *CompoundStmt
}

func (*IfStmt) isStmt() {}

// SwitchStmt represents: switch (expr) { cases }
type SwitchStmt struct {
	Loc   Loc
	Expr  Expr
	Cases []SwitchCase
}

func (*SwitchStmt) isStmt() {}

// SwitchCase represents a case clause in a switch.
type SwitchCase struct {
	Loc       Loc
	Selectors []Expr // nil for default
	Body      *CompoundStmt
}

// ForStmt represents: for (init; cond; update) { }
type ForStmt struct {
	Loc       Loc
	Init      Stmt // VarDecl, LetDecl, assignment, or nil
	Condition Expr // nil for infinite loop
	Update    Stmt // Assignment or call, or nil
	Body      *CompoundStmt
}

func (*ForStmt) isStmt() {}

// WhileStmt represents: while (cond) { }
type WhileStmt struct {
	Loc       Loc
	Condition Expr
	Body      *CompoundStmt
}

func (*WhileStmt) isStmt() {}

// LoopStmt represents: loop { [continuing { }] }
type LoopStmt struct {
	Loc        Loc
	Body       *CompoundStmt
	Continuing *CompoundStmt // nil if no continuing block
}

func (*LoopStmt) isStmt() {}

// BreakStmt represents: break;
type BreakStmt struct {
	Loc Loc
}

func (*BreakStmt) isStmt() {}

// BreakIfStmt represents: break if expr; (only in continuing block)
type BreakIfStmt struct {
	Loc       Loc
	Condition Expr
}

func (*BreakIfStmt) isStmt() {}

// ContinueStmt represents: continue;
type ContinueStmt struct {
	Loc Loc
}

func (*ContinueStmt) isStmt() {}

// DiscardStmt represents: discard; (fragment shader only)
type DiscardStmt struct {
	Loc Loc
}

func (*DiscardStmt) isStmt() {}

// AssignStmt represents: lhs = rhs; or lhs op= rhs;
type AssignStmt struct {
	Loc   Loc
	Op    AssignOp
	Left  Expr
	Right Expr
}

func (*AssignStmt) isStmt() {}

// AssignOp represents assignment operators.
type AssignOp uint8

const (
	AssignOpSimple AssignOp = iota // =
	AssignOpAdd                    // +=
	AssignOpSub                    // -=
	AssignOpMul                    // *=
	AssignOpDiv                    // /=
	AssignOpMod                    // %=
	AssignOpAnd                    // &=
	AssignOpOr                     // |=
	AssignOpXor                    // ^=
	AssignOpShl                    // <<=
	AssignOpShr                    // >>=
)

// IncrDecrStmt represents: expr++; or expr--;
type IncrDecrStmt struct {
	Loc       Loc
	Expr      Expr
	Increment bool // true for ++, false for --
}

func (*IncrDecrStmt) isStmt() {}

// CallStmt represents a function call as a statement.
type CallStmt struct {
	Loc  Loc
	Call *CallExpr
}

func (*CallStmt) isStmt() {}

// DeclStmt wraps a declaration as a statement (for local const/let/var).
type DeclStmt struct {
	Decl Decl
}

func (*DeclStmt) isStmt() {}

// ----------------------------------------------------------------------------
// Scope
// ----------------------------------------------------------------------------

// ScopeMember represents a symbol in a scope with its declaration location.
type ScopeMember struct {
	Ref Ref
	Loc int // Source position where declared (for text-order scoping)
}

// Scope represents a lexical scope.
type Scope struct {
	Parent   *Scope
	Children []*Scope
	Members  map[string]ScopeMember

	// For minification: whether this scope contains direct eval (always false in WGSL)
	// Kept for structural similarity with esbuild
	ContainsDirectEval bool
}

// NewScope creates a new scope with the given parent.
func NewScope(parent *Scope) *Scope {
	return &Scope{
		Parent:  parent,
		Members: make(map[string]ScopeMember),
	}
}

// ----------------------------------------------------------------------------
// Purity Analysis Helpers
// ----------------------------------------------------------------------------

// PurityContext provides context for purity analysis.
type PurityContext struct {
	Symbols     []Symbol
	PureCalls   map[string]bool // Known pure built-in functions
}

// NewPurityContext creates a purity context with known pure functions.
func NewPurityContext(symbols []Symbol) *PurityContext {
	return &PurityContext{
		Symbols:   symbols,
		PureCalls: ComputePureBuiltins(),
	}
}

// ComputePureBuiltins returns a set of known pure WGSL built-in functions.
// These functions have no side effects and return deterministic results.
func ComputePureBuiltins() map[string]bool {
	pure := make(map[string]bool)

	// Math functions (scalar and vector)
	mathFuncs := []string{
		"abs", "acos", "acosh", "asin", "asinh", "atan", "atanh", "atan2",
		"ceil", "clamp", "cos", "cosh", "cross", "degrees",
		"determinant", "distance", "dot", "exp", "exp2",
		"faceForward", "floor", "fma", "fract", "frexp",
		"inverseSqrt", "ldexp", "length", "log", "log2",
		"max", "min", "mix", "modf", "normalize",
		"pow", "quantizeToF16", "radians", "reflect", "refract",
		"round", "saturate", "sign", "sin", "sinh", "smoothstep",
		"sqrt", "step", "tan", "tanh", "transpose", "trunc",
	}
	for _, fn := range mathFuncs {
		pure[fn] = true
	}

	// Integer functions
	intFuncs := []string{
		"abs", "clamp", "countLeadingZeros", "countOneBits", "countTrailingZeros",
		"extractBits", "firstLeadingBit", "firstTrailingBit", "insertBits",
		"max", "min", "reverseBits",
	}
	for _, fn := range intFuncs {
		pure[fn] = true
	}

	// Logical/comparison functions
	logicFuncs := []string{
		"all", "any", "select",
	}
	for _, fn := range logicFuncs {
		pure[fn] = true
	}

	// Vector/matrix construction (type constructors are always pure)
	constructors := []string{
		"vec2", "vec3", "vec4", "vec2f", "vec3f", "vec4f",
		"vec2i", "vec3i", "vec4i", "vec2u", "vec3u", "vec4u",
		"vec2h", "vec3h", "vec4h",
		"mat2x2", "mat2x3", "mat2x4",
		"mat3x2", "mat3x3", "mat3x4",
		"mat4x2", "mat4x3", "mat4x4",
		"mat2x2f", "mat2x3f", "mat2x4f",
		"mat3x2f", "mat3x3f", "mat3x4f",
		"mat4x2f", "mat4x3f", "mat4x4f",
		"mat2x2h", "mat2x3h", "mat2x4h",
		"mat3x2h", "mat3x3h", "mat3x4h",
		"mat4x2h", "mat4x3h", "mat4x4h",
		"array", "bool", "i32", "u32", "f32", "f16",
	}
	for _, fn := range constructors {
		pure[fn] = true
	}

	// Pack/unpack functions
	packFuncs := []string{
		"pack2x16float", "pack2x16snorm", "pack2x16unorm",
		"pack4x8snorm", "pack4x8unorm", "pack4xI8", "pack4xU8",
		"pack4xI8Clamp", "pack4xU8Clamp",
		"unpack2x16float", "unpack2x16snorm", "unpack2x16unorm",
		"unpack4x8snorm", "unpack4x8unorm", "unpack4xI8", "unpack4xU8",
	}
	for _, fn := range packFuncs {
		pure[fn] = true
	}

	// Derivative functions (pure in the sense they don't modify state,
	// but depend on fragment position - still safe to remove if unused)
	derivFuncs := []string{
		"dpdx", "dpdxCoarse", "dpdxFine",
		"dpdy", "dpdyCoarse", "dpdyFine",
		"fwidth", "fwidthCoarse", "fwidthFine",
	}
	for _, fn := range derivFuncs {
		pure[fn] = true
	}

	return pure
}

// ExprCanBeRemovedIfUnused returns true if the expression can be safely
// removed when its result is not used (i.e., it has no side effects).
func (ctx *PurityContext) ExprCanBeRemovedIfUnused(expr Expr) bool {
	if expr == nil {
		return true
	}

	switch e := expr.(type) {
	case *LiteralExpr:
		// Literals are always pure
		return true

	case *IdentExpr:
		// Identifier references are pure (no side effects from reading)
		// unless they're flagged otherwise
		return e.Flags.Has(ExprFlagCanBeRemovedIfUnused) || !e.Ref.IsValid() || ctx.isSymbolPure(e.Ref)

	case *BinaryExpr:
		// Binary expressions are pure if both operands are pure
		// WGSL has no operator overloading, so built-in operators are always pure
		return ctx.ExprCanBeRemovedIfUnused(e.Left) && ctx.ExprCanBeRemovedIfUnused(e.Right)

	case *UnaryExpr:
		// Unary expressions are pure if the operand is pure
		// Exception: address-of (&) might have implications, but reading is pure
		return ctx.ExprCanBeRemovedIfUnused(e.Operand)

	case *CallExpr:
		// Calls are pure if:
		// 1. The function is a known pure built-in
		// 2. All arguments are pure
		if e.Flags.Has(ExprFlagCanBeRemovedIfUnused) || e.Flags.Has(ExprFlagFromPureFunction) {
			return true
		}

		// Check if it's a call to a pure built-in
		if ident, ok := e.Func.(*IdentExpr); ok {
			if ctx.PureCalls[ident.Name] {
				// Check all arguments are pure
				for _, arg := range e.Args {
					if !ctx.ExprCanBeRemovedIfUnused(arg) {
						return false
					}
				}
				return true
			}
		}

		// User-defined functions might have side effects
		return false

	case *IndexExpr:
		// Array indexing is pure if both base and index are pure
		return ctx.ExprCanBeRemovedIfUnused(e.Base) && ctx.ExprCanBeRemovedIfUnused(e.Index)

	case *MemberExpr:
		// Member access is pure if the base is pure
		return ctx.ExprCanBeRemovedIfUnused(e.Base)

	case *ParenExpr:
		// Parentheses just wrap another expression
		return ctx.ExprCanBeRemovedIfUnused(e.Expr)

	default:
		// Unknown expression types - assume they could have side effects
		return false
	}
}

// isSymbolPure returns true if reading the symbol has no side effects.
func (ctx *PurityContext) isSymbolPure(ref Ref) bool {
	if !ref.IsValid() {
		return false
	}
	if int(ref.InnerIndex) >= len(ctx.Symbols) {
		return false
	}

	sym := &ctx.Symbols[ref.InnerIndex]

	// Constants, parameters, and let bindings are always pure to read
	switch sym.Kind {
	case SymbolConst, SymbolLet, SymbolParameter, SymbolBuiltin:
		return true
	case SymbolVar, SymbolOverride:
		// Vars and overrides are still pure to read (no side effects)
		return true
	}

	return true
}

// StmtCanBeRemovedIfUnused returns true if the statement can be removed
// when none of its declared symbols are used.
func (ctx *PurityContext) StmtCanBeRemovedIfUnused(stmt Stmt) bool {
	if stmt == nil {
		return true
	}

	switch s := stmt.(type) {
	case *DeclStmt:
		return ctx.DeclCanBeRemovedIfUnused(s.Decl)

	case *ReturnStmt:
		if s.Value != nil {
			return ctx.ExprCanBeRemovedIfUnused(s.Value)
		}
		return true

	case *CallStmt:
		// Function call statements might have side effects
		return false

	case *AssignStmt:
		// Assignments have side effects
		return false

	case *IncrDecrStmt:
		// Increment/decrement has side effects
		return false

	case *IfStmt, *ForStmt, *WhileStmt, *LoopStmt, *SwitchStmt:
		// Control flow statements need deeper analysis
		return false

	case *BreakStmt, *ContinueStmt, *DiscardStmt:
		// Control flow statements have effects
		return false

	default:
		return false
	}
}

// DeclCanBeRemovedIfUnused returns true if the declaration can be removed
// when its symbol is not used.
func (ctx *PurityContext) DeclCanBeRemovedIfUnused(decl Decl) bool {
	if decl == nil {
		return true
	}

	switch d := decl.(type) {
	case *ConstDecl:
		// Const declarations can be removed if initializer is pure
		return ctx.ExprCanBeRemovedIfUnused(d.Initializer)

	case *LetDecl:
		// Let declarations can be removed if initializer is pure
		return ctx.ExprCanBeRemovedIfUnused(d.Initializer)

	case *VarDecl:
		// Var declarations can be removed if initializer is pure (or absent)
		if d.Initializer == nil {
			return true
		}
		return ctx.ExprCanBeRemovedIfUnused(d.Initializer)

	case *OverrideDecl:
		// Override declarations are API-facing, should not be removed
		return false

	case *FunctionDecl:
		// Functions can be removed if unused (dead code elimination)
		// but we don't analyze the body here
		return true

	case *StructDecl:
		// Structs can be removed if unused
		return true

	case *AliasDecl:
		// Type aliases can be removed if unused
		return true

	case *ConstAssertDecl:
		// const_assert should not be removed
		return false

	default:
		return false
	}
}

// MarkExprPurity sets the purity flags on an expression based on analysis.
func (ctx *PurityContext) MarkExprPurity(expr Expr) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *LiteralExpr:
		e.Flags |= ExprFlagCanBeRemovedIfUnused | ExprFlagIsConstant

	case *IdentExpr:
		if ctx.isSymbolPure(e.Ref) {
			e.Flags |= ExprFlagCanBeRemovedIfUnused
		}
		// Check if it's a constant
		if e.Ref.IsValid() && int(e.Ref.InnerIndex) < len(ctx.Symbols) {
			if ctx.Symbols[e.Ref.InnerIndex].Kind == SymbolConst {
				e.Flags |= ExprFlagIsConstant
			}
		}

	case *BinaryExpr:
		ctx.MarkExprPurity(e.Left)
		ctx.MarkExprPurity(e.Right)
		if ctx.ExprCanBeRemovedIfUnused(e.Left) && ctx.ExprCanBeRemovedIfUnused(e.Right) {
			e.Flags |= ExprFlagCanBeRemovedIfUnused
		}

	case *UnaryExpr:
		ctx.MarkExprPurity(e.Operand)
		if ctx.ExprCanBeRemovedIfUnused(e.Operand) {
			e.Flags |= ExprFlagCanBeRemovedIfUnused
		}

	case *CallExpr:
		for _, arg := range e.Args {
			ctx.MarkExprPurity(arg)
		}
		if ident, ok := e.Func.(*IdentExpr); ok {
			if ctx.PureCalls[ident.Name] {
				e.Flags |= ExprFlagFromPureFunction
				// Check if all args are pure
				allPure := true
				for _, arg := range e.Args {
					if !ctx.ExprCanBeRemovedIfUnused(arg) {
						allPure = false
						break
					}
				}
				if allPure {
					e.Flags |= ExprFlagCanBeRemovedIfUnused
				}
			}
		}

	case *IndexExpr:
		ctx.MarkExprPurity(e.Base)
		ctx.MarkExprPurity(e.Index)
		if ctx.ExprCanBeRemovedIfUnused(e.Base) && ctx.ExprCanBeRemovedIfUnused(e.Index) {
			e.Flags |= ExprFlagCanBeRemovedIfUnused
		}

	case *MemberExpr:
		ctx.MarkExprPurity(e.Base)
		if ctx.ExprCanBeRemovedIfUnused(e.Base) {
			e.Flags |= ExprFlagCanBeRemovedIfUnused
		}

	case *ParenExpr:
		ctx.MarkExprPurity(e.Expr)
		if ctx.ExprCanBeRemovedIfUnused(e.Expr) {
			e.Flags |= ExprFlagCanBeRemovedIfUnused
		}
	}
}
