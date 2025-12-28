// Package validator provides semantic validation for WGSL shaders.
//
// The validator performs type checking, symbol resolution validation,
// control flow analysis, and uniformity analysis to ensure shaders
// conform to the WGSL specification.
package validator

import (
	"fmt"
	"strings"

	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/builtins"
	"github.com/HugoDaniel/miniray/internal/diagnostic"
	"github.com/HugoDaniel/miniray/internal/types"
)

// ShaderStage represents the shader pipeline stage.
type ShaderStage uint8

const (
	StageNone ShaderStage = iota
	StageVertex
	StageFragment
	StageCompute
)

func (s ShaderStage) String() string {
	switch s {
	case StageVertex:
		return "vertex"
	case StageFragment:
		return "fragment"
	case StageCompute:
		return "compute"
	default:
		return "none"
	}
}

// Options controls validation behavior.
type Options struct {
	// StrictMode treats warnings as errors.
	StrictMode bool
	// DiagnosticFilters control which diagnostics are reported.
	DiagnosticFilters *diagnostic.DiagnosticFilter
}

// Result contains validation results.
type Result struct {
	// Valid is true if no errors were found.
	Valid bool
	// Diagnostics contains all validation messages.
	Diagnostics *diagnostic.DiagnosticList
	// TypeInfo contains resolved type information (for tooling).
	TypeInfo *TypeInfo
}

// TypeInfo stores resolved type information for expressions.
type TypeInfo struct {
	// ExprTypes maps expression locations to their resolved types.
	ExprTypes map[int]types.Type
	// SymbolTypes maps symbol references to their types.
	SymbolTypes map[ast.Ref]types.Type
	// Structs maps struct names to their resolved types.
	Structs map[string]*types.Struct
}

// Validator performs semantic validation on a parsed WGSL module.
type Validator struct {
	module      *ast.Module
	diags       *diagnostic.DiagnosticList
	typeInfo    *TypeInfo
	options     Options

	// Current context
	currentFunc      *ast.FunctionDecl
	currentStage     ShaderStage
	inLoop           bool
	inSwitch         bool
	returnType       types.Type
	hasReturn        bool

	// Symbol type cache
	symbolTypes map[ast.Ref]types.Type

	// Struct type cache
	structTypes map[string]*types.Struct

	// Alias resolution cache
	aliasTypes map[string]types.Type

	// Uniformity tracking
	uniformityAnalyzer *UniformityAnalyzer
}

// Validate performs semantic validation on the given module.
func Validate(module *ast.Module, options Options) *Result {
	v := &Validator{
		module:      module,
		diags:       diagnostic.NewDiagnosticList(module.Source),
		options:     options,
		symbolTypes: make(map[ast.Ref]types.Type),
		structTypes: make(map[string]*types.Struct),
		aliasTypes:  make(map[string]types.Type),
		typeInfo: &TypeInfo{
			ExprTypes:   make(map[int]types.Type),
			SymbolTypes: make(map[ast.Ref]types.Type),
			Structs:     make(map[string]*types.Struct),
		},
	}

	if options.DiagnosticFilters == nil {
		v.options.DiagnosticFilters = diagnostic.NewDiagnosticFilter()
	}

	// Phase 1: Collect type declarations (structs, aliases)
	v.collectTypeDeclarations()

	// Phase 2: Resolve struct layouts
	v.resolveStructLayouts()

	// Phase 3: Validate declarations
	v.validateDeclarations()

	// Phase 4: Validate functions and statements
	v.validateFunctions()

	// Phase 5: Uniformity analysis
	v.analyzeUniformity()

	// Copy type info
	v.typeInfo.SymbolTypes = v.symbolTypes
	v.typeInfo.Structs = v.structTypes

	return &Result{
		Valid:       !v.diags.HasErrors(),
		Diagnostics: v.diags,
		TypeInfo:    v.typeInfo,
	}
}

// ----------------------------------------------------------------------------
// Phase 1: Collect Type Declarations
// ----------------------------------------------------------------------------

func (v *Validator) collectTypeDeclarations() {
	for _, decl := range v.module.Declarations {
		switch d := decl.(type) {
		case *ast.StructDecl:
			name := v.symbolName(d.Name)
			if name == "" {
				continue
			}
			// Create struct type placeholder
			st := &types.Struct{Name: name}
			v.structTypes[name] = st

		case *ast.AliasDecl:
			name := v.symbolName(d.Name)
			if name == "" {
				continue
			}
			// Resolve alias type later
			v.aliasTypes[name] = nil // Placeholder
		}
	}
}

// ----------------------------------------------------------------------------
// Phase 2: Resolve Struct Layouts
// ----------------------------------------------------------------------------

func (v *Validator) resolveStructLayouts() {
	for _, decl := range v.module.Declarations {
		switch d := decl.(type) {
		case *ast.StructDecl:
			name := v.symbolName(d.Name)
			st := v.structTypes[name]
			if st == nil {
				continue
			}

			// Resolve member types
			for _, member := range d.Members {
				memberName := v.symbolName(member.Name)
				memberType := v.resolveType(member.Type)
				if memberType == nil {
					v.error(int(member.Loc.Start), "cannot resolve type for struct member '%s'", memberName)
					continue
				}
				st.Fields = append(st.Fields, types.StructField{
					Name: memberName,
					Type: memberType,
				})
			}

			// Compute layout
			st.ComputeLayout()

		case *ast.AliasDecl:
			name := v.symbolName(d.Name)
			aliasType := v.resolveType(d.Type)
			if aliasType == nil {
				v.error(int(d.Loc.Start), "cannot resolve type alias '%s'", name)
				continue
			}
			v.aliasTypes[name] = aliasType
		}
	}
}

// ----------------------------------------------------------------------------
// Phase 3: Validate Declarations
// ----------------------------------------------------------------------------

func (v *Validator) validateDeclarations() {
	for _, decl := range v.module.Declarations {
		switch d := decl.(type) {
		case *ast.ConstDecl:
			v.validateConstDecl(d)
		case *ast.OverrideDecl:
			v.validateOverrideDecl(d)
		case *ast.VarDecl:
			v.validateVarDecl(d)
		case *ast.LetDecl:
			v.validateLetDecl(d)
		}
	}
}

func (v *Validator) validateConstDecl(d *ast.ConstDecl) {
	name := v.symbolName(d.Name)

	// const must have an initializer
	if d.Initializer == nil {
		v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeMissingInitializer),
			"const declaration '%s' requires an initializer", name)
		return
	}

	// Infer or check type
	initType := v.checkExpr(d.Initializer)
	if initType == nil {
		return
	}

	var declType types.Type
	if d.Type != nil {
		declType = v.resolveType(d.Type)
		if declType != nil && !types.CanConvertTo(initType, declType) {
			v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeTypeMismatch),
				"cannot initialize '%s' of type '%s' with value of type '%s'",
				name, declType.String(), initType.String())
			return
		}
	} else {
		declType = initType
	}

	// const must have constructible type
	if declType != nil && !declType.IsConstructible() {
		v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeInvalidConstExpr),
			"const '%s' has non-constructible type '%s'", name, declType.String())
		return
	}

	v.symbolTypes[d.Name] = declType
}

func (v *Validator) validateOverrideDecl(d *ast.OverrideDecl) {
	name := v.symbolName(d.Name)

	// override must be concrete scalar type
	var declType types.Type
	if d.Type != nil {
		declType = v.resolveType(d.Type)
	} else if d.Initializer != nil {
		declType = v.checkExpr(d.Initializer)
	}

	if declType == nil {
		v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeInvalidOverride),
			"cannot determine type for override '%s'", name)
		return
	}

	// Must be concrete scalar
	scalar, ok := declType.(*types.Scalar)
	if !ok || !scalar.IsConcrete() {
		v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeInvalidOverride),
			"override '%s' must have concrete scalar type, got '%s'", name, declType.String())
		return
	}

	if d.Initializer != nil {
		initType := v.checkExpr(d.Initializer)
		if initType != nil && !types.CanConvertTo(initType, declType) {
			v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeTypeMismatch),
				"cannot initialize override '%s' of type '%s' with '%s'",
				name, declType.String(), initType.String())
		}
	}

	v.symbolTypes[d.Name] = declType
}

func (v *Validator) validateVarDecl(d *ast.VarDecl) {
	name := v.symbolName(d.Name)

	// Determine type
	var declType types.Type
	if d.Type != nil {
		declType = v.resolveType(d.Type)
	} else if d.Initializer != nil {
		declType = v.checkExpr(d.Initializer)
	}

	if declType == nil {
		v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeTypeMismatch),
			"cannot determine type for var '%s'", name)
		return
	}

	// Validate address space constraints
	v.validateAddressSpace(d, declType)

	// Check initializer compatibility
	if d.Initializer != nil {
		initType := v.checkExpr(d.Initializer)
		if initType != nil && !types.CanConvertTo(initType, declType) {
			v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeTypeMismatch),
				"cannot initialize var '%s' of type '%s' with '%s'",
				name, declType.String(), initType.String())
		}
	}

	// Check for required @group/@binding
	if d.AddressSpace == ast.AddressSpaceUniform || d.AddressSpace == ast.AddressSpaceStorage {
		hasGroup := false
		hasBinding := false
		for _, attr := range d.Attributes {
			if attr.Name == "group" {
				hasGroup = true
			}
			if attr.Name == "binding" {
				hasBinding = true
			}
		}
		if !hasGroup || !hasBinding {
			v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeMissingBinding),
				"var '%s' with address space '%s' requires @group and @binding attributes",
				name, d.AddressSpace.String())
		}
	}

	v.symbolTypes[d.Name] = declType
}

func (v *Validator) validateLetDecl(d *ast.LetDecl) {
	name := v.symbolName(d.Name)

	// let must have an initializer
	if d.Initializer == nil {
		v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeMissingInitializer),
			"let declaration '%s' requires an initializer", name)
		return
	}

	initType := v.checkExpr(d.Initializer)
	if initType == nil {
		return
	}

	var declType types.Type
	if d.Type != nil {
		declType = v.resolveType(d.Type)
		if declType != nil && !types.CanConvertTo(initType, declType) {
			v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeTypeMismatch),
				"cannot initialize let '%s' of type '%s' with '%s'",
				name, declType.String(), initType.String())
			return
		}
	} else {
		// Infer type from initializer, converting abstract to concrete
		declType = types.ConcreteType(initType)
	}

	v.symbolTypes[d.Name] = declType
}

func (v *Validator) validateAddressSpace(d *ast.VarDecl, varType types.Type) {
	name := v.symbolName(d.Name)

	switch d.AddressSpace {
	case ast.AddressSpaceWorkgroup:
		// workgroup only allowed at module scope in compute shaders
		// This is checked during function validation

		// Must be plain type with fixed footprint
		if !varType.IsStorable() {
			v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeInvalidWorkgroupVar),
				"workgroup var '%s' must have storable type", name)
		}

	case ast.AddressSpaceUniform:
		// Must be host-shareable
		if !varType.IsHostShareable() {
			v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeInvalidUniformVar),
				"uniform var '%s' must have host-shareable type", name)
		}
		// No initializer allowed
		if d.Initializer != nil {
			v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeInvalidInitializer),
				"uniform var '%s' cannot have an initializer", name)
		}

	case ast.AddressSpaceStorage:
		// Must be host-shareable
		if !varType.IsHostShareable() {
			v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeInvalidStorageVar),
				"storage var '%s' must have host-shareable type", name)
		}
		// No initializer allowed
		if d.Initializer != nil {
			v.errorWithCode(int(d.Loc.Start), string(diagnostic.CodeInvalidInitializer),
				"storage var '%s' cannot have an initializer", name)
		}
	}
}

// ----------------------------------------------------------------------------
// Phase 4: Validate Functions
// ----------------------------------------------------------------------------

func (v *Validator) validateFunctions() {
	for _, decl := range v.module.Declarations {
		if fn, ok := decl.(*ast.FunctionDecl); ok {
			v.validateFunction(fn)
		}
	}
}

func (v *Validator) validateFunction(fn *ast.FunctionDecl) {
	v.currentFunc = fn
	v.inLoop = false
	v.inSwitch = false
	v.hasReturn = false

	// Determine shader stage
	v.currentStage = StageNone
	for _, attr := range fn.Attributes {
		switch attr.Name {
		case "vertex":
			v.currentStage = StageVertex
		case "fragment":
			v.currentStage = StageFragment
		case "compute":
			v.currentStage = StageCompute
		}
	}

	// Resolve return type
	if fn.ReturnType != nil {
		v.returnType = v.resolveType(fn.ReturnType)
	} else {
		v.returnType = nil
	}

	// Validate parameters
	for _, param := range fn.Parameters {
		paramType := v.resolveType(param.Type)
		if paramType != nil {
			v.symbolTypes[param.Name] = paramType
		}

		// Validate parameter attributes
		v.validateParameterAttributes(param)
	}

	// Validate entry point requirements
	if v.currentStage != StageNone {
		v.validateEntryPoint(fn)
	}

	// Validate function body
	if fn.Body != nil {
		v.validateCompoundStmt(fn.Body)
	}

	// Check for missing return
	if v.returnType != nil && !v.hasReturn {
		v.errorWithCode(int(fn.Loc.Start), string(diagnostic.CodeMissingReturn),
			"function '%s' must return a value of type '%s'",
			v.symbolName(fn.Name), v.returnType.String())
	}

	v.currentFunc = nil
	v.returnType = nil
}

func (v *Validator) validateParameterAttributes(param ast.Parameter) {
	for _, attr := range param.Attributes {
		switch attr.Name {
		case "location":
			// Only valid for entry point I/O
			if v.currentStage == StageNone {
				v.errorWithCode(int(attr.Loc.Start), string(diagnostic.CodeInvalidAttribute),
					"@location is only valid on entry point parameters")
			}
		case "builtin":
			// Validate builtin is valid for this stage
			if len(attr.Args) > 0 {
				if ident, ok := attr.Args[0].(*ast.IdentExpr); ok {
					v.validateBuiltinForStage(int(attr.Loc.Start), ident.Name, true)
				}
			}
		}
	}
}

func (v *Validator) validateEntryPoint(fn *ast.FunctionDecl) {
	name := v.symbolName(fn.Name)

	switch v.currentStage {
	case StageVertex:
		// Must return @builtin(position) vec4<f32>
		hasPosition := false
		for _, attr := range fn.ReturnAttr {
			if attr.Name == "builtin" && len(attr.Args) > 0 {
				if ident, ok := attr.Args[0].(*ast.IdentExpr); ok && ident.Name == "position" {
					hasPosition = true
				}
			}
		}
		if fn.ReturnType != nil && !hasPosition {
			// Check if return type is struct with @builtin(position)
			// For now, just warn if no position builtin found
		}

	case StageFragment:
		// Return type must not be void... actually it can be void with outputs via parameters
		// This is a simplified check

	case StageCompute:
		// Must have @workgroup_size
		hasWorkgroupSize := false
		for _, attr := range fn.Attributes {
			if attr.Name == "workgroup_size" {
				hasWorkgroupSize = true
				// Validate workgroup_size arguments
				if len(attr.Args) == 0 {
					v.errorWithCode(int(attr.Loc.Start), string(diagnostic.CodeInvalidAttribute),
						"@workgroup_size requires at least one argument")
				}
			}
		}
		if !hasWorkgroupSize {
			v.errorWithCode(int(fn.Loc.Start), string(diagnostic.CodeMissingAttribute),
				"compute entry point '%s' requires @workgroup_size attribute", name)
		}

		// Must not return a value
		if fn.ReturnType != nil {
			v.errorWithCode(int(fn.Loc.Start), string(diagnostic.CodeInvalidEntryPoint),
				"compute entry point '%s' must not return a value", name)
		}
	}
}

func (v *Validator) validateBuiltinForStage(loc int, builtin string, isInput bool) {
	// Map of valid builtins per stage and direction
	vertexInputs := map[string]bool{
		"vertex_index": true, "instance_index": true,
	}
	vertexOutputs := map[string]bool{
		"position": true,
	}
	fragmentInputs := map[string]bool{
		"position": true, "front_facing": true, "sample_index": true, "sample_mask": true,
	}
	fragmentOutputs := map[string]bool{
		"frag_depth": true, "sample_mask": true,
	}
	computeInputs := map[string]bool{
		"local_invocation_id": true, "local_invocation_index": true,
		"global_invocation_id": true, "workgroup_id": true, "num_workgroups": true,
	}

	valid := false
	switch v.currentStage {
	case StageVertex:
		if isInput {
			valid = vertexInputs[builtin]
		} else {
			valid = vertexOutputs[builtin]
		}
	case StageFragment:
		if isInput {
			valid = fragmentInputs[builtin]
		} else {
			valid = fragmentOutputs[builtin]
		}
	case StageCompute:
		if isInput {
			valid = computeInputs[builtin]
		}
	}

	if !valid {
		dir := "output"
		if isInput {
			dir = "input"
		}
		v.errorWithCode(loc, string(diagnostic.CodeInvalidBuiltin),
			"@builtin(%s) is not valid as %s %s for %s shader",
			builtin, v.currentStage.String(), dir, v.currentStage.String())
	}
}

// ----------------------------------------------------------------------------
// Statement Validation
// ----------------------------------------------------------------------------

func (v *Validator) validateStmt(stmt ast.Stmt) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.CompoundStmt:
		v.validateCompoundStmt(s)
	case *ast.ReturnStmt:
		v.validateReturnStmt(s)
	case *ast.IfStmt:
		v.validateIfStmt(s)
	case *ast.SwitchStmt:
		v.validateSwitchStmt(s)
	case *ast.LoopStmt:
		v.validateLoopStmt(s)
	case *ast.WhileStmt:
		v.validateWhileStmt(s)
	case *ast.ForStmt:
		v.validateForStmt(s)
	case *ast.BreakStmt:
		v.validateBreakStmt(s)
	case *ast.ContinueStmt:
		v.validateContinueStmt(s)
	case *ast.DiscardStmt:
		v.validateDiscardStmt(s)
	case *ast.AssignStmt:
		v.validateAssignStmt(s)
	case *ast.DeclStmt:
		v.validateDeclStmt(s)
	case *ast.IncrDecrStmt:
		v.validateIncrDecrStmt(s)
	case *ast.CallStmt:
		v.validateCallStmt(s)
	}
}

func (v *Validator) validateCompoundStmt(s *ast.CompoundStmt) {
	for _, stmt := range s.Stmts {
		v.validateStmt(stmt)
	}
}

func (v *Validator) validateReturnStmt(s *ast.ReturnStmt) {
	v.hasReturn = true

	if s.Value == nil {
		if v.returnType != nil {
			v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeMissingReturn),
				"return statement must return a value of type '%s'", v.returnType.String())
		}
		return
	}

	exprType := v.checkExpr(s.Value)
	if exprType == nil {
		return
	}

	if v.returnType == nil {
		v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeInvalidReturn),
			"cannot return a value from a void function")
		return
	}

	if !types.CanConvertTo(exprType, v.returnType) {
		v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeTypeMismatch),
			"cannot return '%s' from function returning '%s'",
			exprType.String(), v.returnType.String())
	}
}

func (v *Validator) validateIfStmt(s *ast.IfStmt) {
	condType := v.checkExpr(s.Condition)
	if condType != nil && !condType.Equals(types.Bool) {
		v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeTypeMismatch),
			"if condition must be bool, got '%s'", condType.String())
	}

	v.validateStmt(s.Body)
	if s.Else != nil {
		v.validateStmt(s.Else)
	}
}

func (v *Validator) validateSwitchStmt(s *ast.SwitchStmt) {
	selectorType := v.checkExpr(s.Expr)
	if selectorType != nil && !types.IsInteger(selectorType) {
		v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeTypeMismatch),
			"switch selector must be integer type, got '%s'", selectorType.String())
	}

	prevInSwitch := v.inSwitch
	v.inSwitch = true

	for _, clause := range s.Cases {
		for _, sel := range clause.Selectors {
			if sel != nil { // nil means default
				selType := v.checkExpr(sel)
				if selType != nil && selectorType != nil && !types.CanConvertTo(selType, selectorType) {
					v.errorWithCode(int(clause.Loc.Start), string(diagnostic.CodeTypeMismatch),
						"case selector type '%s' doesn't match switch selector type '%s'",
						selType.String(), selectorType.String())
				}
			}
		}
		v.validateStmt(clause.Body)
	}

	v.inSwitch = prevInSwitch
}

func (v *Validator) validateLoopStmt(s *ast.LoopStmt) {
	prevInLoop := v.inLoop
	v.inLoop = true

	v.validateStmt(s.Body)
	if s.Continuing != nil {
		v.validateStmt(s.Continuing)
	}

	v.inLoop = prevInLoop
}

func (v *Validator) validateWhileStmt(s *ast.WhileStmt) {
	condType := v.checkExpr(s.Condition)
	if condType != nil && !condType.Equals(types.Bool) {
		v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeTypeMismatch),
			"while condition must be bool, got '%s'", condType.String())
	}

	prevInLoop := v.inLoop
	v.inLoop = true
	v.validateStmt(s.Body)
	v.inLoop = prevInLoop
}

func (v *Validator) validateForStmt(s *ast.ForStmt) {
	if s.Init != nil {
		v.validateStmt(s.Init)
	}
	if s.Condition != nil {
		condType := v.checkExpr(s.Condition)
		if condType != nil && !condType.Equals(types.Bool) {
			v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeTypeMismatch),
				"for condition must be bool, got '%s'", condType.String())
		}
	}
	if s.Update != nil {
		v.validateStmt(s.Update)
	}

	prevInLoop := v.inLoop
	v.inLoop = true
	v.validateStmt(s.Body)
	v.inLoop = prevInLoop
}

func (v *Validator) validateBreakStmt(s *ast.BreakStmt) {
	if !v.inLoop && !v.inSwitch {
		v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeBreakOutsideLoop),
			"break statement must be inside a loop or switch")
	}
}

func (v *Validator) validateContinueStmt(s *ast.ContinueStmt) {
	if !v.inLoop {
		v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeContinueOutsideLoop),
			"continue statement must be inside a loop")
	}
}

func (v *Validator) validateDiscardStmt(s *ast.DiscardStmt) {
	if v.currentStage != StageFragment {
		v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeDiscardOutsideFragment),
			"discard statement is only valid in fragment shaders")
	}
}

func (v *Validator) validateAssignStmt(s *ast.AssignStmt) {
	lhsType := v.checkExpr(s.Left)
	rhsType := v.checkExpr(s.Right)

	if lhsType == nil || rhsType == nil {
		return
	}

	// Check LHS is assignable (reference type)
	// For now, just check type compatibility
	if !types.CanConvertTo(rhsType, lhsType) {
		v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeTypeMismatch),
			"cannot assign '%s' to '%s'", rhsType.String(), lhsType.String())
	}
}

func (v *Validator) validateIncrDecrStmt(s *ast.IncrDecrStmt) {
	exprType := v.checkExpr(s.Expr)
	if exprType == nil {
		return
	}

	if !types.IsInteger(exprType) {
		v.errorWithCode(int(s.Loc.Start), string(diagnostic.CodeTypeMismatch),
			"increment/decrement requires integer type, got '%s'", exprType.String())
	}
}

func (v *Validator) validateCallStmt(s *ast.CallStmt) {
	// Just check the call expression
	v.checkExpr(s.Call)
}

func (v *Validator) validateDeclStmt(s *ast.DeclStmt) {
	// Validate the wrapped declaration
	switch d := s.Decl.(type) {
	case *ast.ConstDecl:
		v.validateConstDecl(d)
	case *ast.LetDecl:
		v.validateLetDecl(d)
	case *ast.VarDecl:
		v.validateVarDecl(d)
	}
}

// ----------------------------------------------------------------------------
// Expression Type Checking
// ----------------------------------------------------------------------------

func (v *Validator) checkExpr(expr ast.Expr) types.Type {
	if expr == nil {
		return nil
	}

	var t types.Type

	switch e := expr.(type) {
	case *ast.LiteralExpr:
		t = v.checkLiteral(e)
	case *ast.IdentExpr:
		t = v.checkIdent(e)
	case *ast.BinaryExpr:
		t = v.checkBinary(e)
	case *ast.UnaryExpr:
		t = v.checkUnary(e)
	case *ast.CallExpr:
		t = v.checkCall(e)
	case *ast.IndexExpr:
		t = v.checkIndex(e)
	case *ast.MemberExpr:
		t = v.checkMember(e)
	case *ast.ParenExpr:
		t = v.checkExpr(e.Expr)
	}

	// Store type info
	if t != nil && expr != nil {
		// Use expression location as key
		switch e := expr.(type) {
		case *ast.LiteralExpr:
			v.typeInfo.ExprTypes[int(e.Loc.Start)] = t
		case *ast.IdentExpr:
			v.typeInfo.ExprTypes[int(e.Loc.Start)] = t
		case *ast.BinaryExpr:
			v.typeInfo.ExprTypes[int(e.Loc.Start)] = t
		case *ast.UnaryExpr:
			v.typeInfo.ExprTypes[int(e.Loc.Start)] = t
		case *ast.CallExpr:
			v.typeInfo.ExprTypes[int(e.Loc.Start)] = t
		case *ast.IndexExpr:
			v.typeInfo.ExprTypes[int(e.Loc.Start)] = t
		case *ast.MemberExpr:
			v.typeInfo.ExprTypes[int(e.Loc.Start)] = t
		}
	}

	return t
}

func (v *Validator) checkLiteral(e *ast.LiteralExpr) types.Type {
	switch {
	case e.Value == "true" || e.Value == "false":
		return types.Bool
	case strings.ContainsAny(e.Value, ".eE") || strings.HasSuffix(e.Value, "f") || strings.HasSuffix(e.Value, "h"):
		if strings.HasSuffix(e.Value, "h") {
			return types.F16
		}
		if strings.HasSuffix(e.Value, "f") {
			return types.F32
		}
		return types.AbstractFloat
	case strings.HasPrefix(e.Value, "0x") || strings.HasPrefix(e.Value, "0X"):
		if strings.HasSuffix(e.Value, "u") {
			return types.U32
		}
		if strings.HasSuffix(e.Value, "i") {
			return types.I32
		}
		return types.AbstractInt
	default:
		if strings.HasSuffix(e.Value, "u") {
			return types.U32
		}
		if strings.HasSuffix(e.Value, "i") {
			return types.I32
		}
		return types.AbstractInt
	}
}

func (v *Validator) checkIdent(e *ast.IdentExpr) types.Type {
	// Check if it's a type name being used as expression (constructor)
	if t := v.lookupType(e.Name); t != nil {
		return t
	}

	// Check symbol table
	if e.Ref.IsValid() {
		if t, ok := v.symbolTypes[e.Ref]; ok {
			return t
		}
	}

	// Check if it's a builtin function
	if builtins.IsBuiltin(e.Name) {
		// Return nil - actual type comes from call resolution
		return nil
	}

	v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeUndefinedSymbol),
		"undefined identifier '%s'", e.Name)
	return nil
}

func (v *Validator) checkBinary(e *ast.BinaryExpr) types.Type {
	leftType := v.checkExpr(e.Left)
	rightType := v.checkExpr(e.Right)

	if leftType == nil || rightType == nil {
		return nil
	}

	// Determine operation category
	switch e.Op {
	case ast.BinOpLogicalAnd, ast.BinOpLogicalOr:
		// Both must be bool
		if !leftType.Equals(types.Bool) || !rightType.Equals(types.Bool) {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"logical operator requires bool operands, got '%s' and '%s'",
				leftType.String(), rightType.String())
			return nil
		}
		return types.Bool

	case ast.BinOpEq, ast.BinOpNe:
		// Comparison - result is bool
		if !leftType.Equals(rightType) && !types.CanConvertTo(leftType, rightType) && !types.CanConvertTo(rightType, leftType) {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"comparison requires compatible types, got '%s' and '%s'",
				leftType.String(), rightType.String())
			return nil
		}
		return types.Bool

	case ast.BinOpLt, ast.BinOpLe, ast.BinOpGt, ast.BinOpGe:
		// Relational comparison - operands must be numeric
		if !types.IsNumeric(leftType) || !types.IsNumeric(rightType) {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"relational operator requires numeric operands, got '%s' and '%s'",
				leftType.String(), rightType.String())
			return nil
		}
		return types.Bool

	case ast.BinOpAdd, ast.BinOpSub:
		// Addition/Subtraction
		result := types.AddSubResultType(leftType, rightType)
		if result == nil {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"arithmetic operator requires compatible numeric types, got '%s' and '%s'",
				leftType.String(), rightType.String())
			return nil
		}
		return result

	case ast.BinOpMul:
		// Multiplication - handles mat*vec, vec*mat, scalar*vec, etc.
		result := types.MultiplyResultType(leftType, rightType)
		if result == nil {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"multiplication requires compatible types, got '%s' and '%s'",
				leftType.String(), rightType.String())
			return nil
		}
		return result

	case ast.BinOpDiv:
		// Division
		result := types.DivResultType(leftType, rightType)
		if result == nil {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"division requires compatible numeric types, got '%s' and '%s'",
				leftType.String(), rightType.String())
			return nil
		}
		return result

	case ast.BinOpMod:
		// Modulo - requires integer types
		if !types.IsInteger(leftType) || !types.IsInteger(rightType) {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"modulo operator requires integer operands, got '%s' and '%s'",
				leftType.String(), rightType.String())
			return nil
		}
		return types.CommonType(leftType, rightType)

	case ast.BinOpAnd, ast.BinOpOr, ast.BinOpXor:
		// Bitwise - requires integer or bool
		if leftType.Equals(types.Bool) && rightType.Equals(types.Bool) {
			return types.Bool
		}
		if types.IsInteger(leftType) && types.IsInteger(rightType) {
			return types.CommonType(leftType, rightType)
		}
		v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
			"bitwise operator requires integer or bool operands, got '%s' and '%s'",
			leftType.String(), rightType.String())
		return nil

	case ast.BinOpShl, ast.BinOpShr:
		// Shift - left must be integer, right must be u32
		if !types.IsInteger(leftType) {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"shift operator requires integer left operand, got '%s'", leftType.String())
			return nil
		}
		if !rightType.Equals(types.U32) && !types.CanConvertTo(rightType, types.U32) {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"shift amount must be u32, got '%s'", rightType.String())
			return nil
		}
		return leftType
	}

	return nil
}

func (v *Validator) checkUnary(e *ast.UnaryExpr) types.Type {
	operandType := v.checkExpr(e.Operand)
	if operandType == nil {
		return nil
	}

	switch e.Op {
	case ast.UnaryOpNeg:
		if !types.IsNumeric(operandType) {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"negation requires numeric type, got '%s'", operandType.String())
			return nil
		}
		return operandType

	case ast.UnaryOpNot:
		if !operandType.Equals(types.Bool) {
			// Also allow vector<bool>
			if vec, ok := operandType.(*types.Vector); ok && vec.Element.Kind == types.ScalarBool {
				return operandType
			}
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"logical not requires bool, got '%s'", operandType.String())
			return nil
		}
		return types.Bool

	case ast.UnaryOpBitNot:
		if !types.IsInteger(operandType) {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
				"bitwise not requires integer type, got '%s'", operandType.String())
			return nil
		}
		return operandType

	case ast.UnaryOpDeref:
		if ptr, ok := operandType.(*types.Pointer); ok {
			return ptr.Element
		}
		if ref, ok := operandType.(*types.Reference); ok {
			return ref.Element
		}
		v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidOperand),
			"dereference requires pointer type, got '%s'", operandType.String())
		return nil

	case ast.UnaryOpAddr:
		// Creates a pointer to the operand
		// Simplified - actual address space detection is complex
		return types.Ptr(types.AddressSpaceFunction, operandType, types.AccessModeReadWrite)
	}

	return nil
}

func (v *Validator) checkCall(e *ast.CallExpr) types.Type {
	// Get function/type name
	var calleeName string

	// First check if it's a template type constructor
	if e.TemplateType != nil {
		return v.resolveType(e.TemplateType)
	}

	switch c := e.Func.(type) {
	case *ast.IdentExpr:
		calleeName = c.Name
	case *ast.MemberExpr:
		// Method call on type (e.g., vec3f(1.0))
		calleeName = v.memberExprToString(c)
	default:
		v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeNotCallable),
			"expression is not callable")
		return nil
	}

	// Check argument types
	var argTypes []types.Type
	for _, arg := range e.Args {
		argType := v.checkExpr(arg)
		argTypes = append(argTypes, argType)
	}

	// Check if it's a builtin function
	if builtin := builtins.Lookup(calleeName); builtin != nil {
		// Check uniformity requirement
		if builtin.RequiresUniform() {
			// Will be checked in uniformity analysis phase
		}

		// Resolve overload
		retType, ok := builtins.ResolveOverload(builtin, argTypes)
		if !ok {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidArgType),
				"no matching overload for '%s' with argument types (%s)",
				calleeName, v.formatTypes(argTypes))
			return nil
		}
		return retType
	}

	// Check if it's a type constructor
	if t := v.lookupType(calleeName); t != nil {
		return v.checkTypeConstructor(e, t, argTypes)
	}

	// Check if it's a user-defined function
	if ident, ok := e.Func.(*ast.IdentExpr); ok && ident.Ref.IsValid() {
		if funcType, ok := v.symbolTypes[ident.Ref]; ok {
			if fn, ok := funcType.(*types.Function); ok {
				// Check argument count
				if len(argTypes) != len(fn.Parameters) {
					v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidArgCount),
						"function '%s' expects %d arguments, got %d",
						calleeName, len(fn.Parameters), len(argTypes))
					return nil
				}
				// Check argument types
				for i, paramType := range fn.Parameters {
					if argTypes[i] != nil && !types.CanConvertTo(argTypes[i], paramType) {
						v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidArgType),
							"argument %d of '%s': cannot convert '%s' to '%s'",
							i+1, calleeName, argTypes[i].String(), paramType.String())
					}
				}
				return fn.ReturnType
			}
		}
	}

	v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeNotCallable),
		"'%s' is not a function or type constructor", calleeName)
	return nil
}

func (v *Validator) checkTypeConstructor(e *ast.CallExpr, t types.Type, argTypes []types.Type) types.Type {
	// Type constructors create values of the type
	switch ty := t.(type) {
	case *types.Scalar:
		// Scalar constructors take 1 argument convertible to the scalar type
		if len(argTypes) != 1 {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidArgCount),
				"scalar constructor expects 1 argument, got %d", len(argTypes))
			return nil
		}
		if argTypes[0] != nil && !types.CanConvertTo(argTypes[0], t) {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidArgType),
				"cannot construct '%s' from '%s'", t.String(), argTypes[0].String())
			return nil
		}
		return t

	case *types.Vector:
		// Vector constructors can take various forms
		// Simplified check: total components must match
		return t

	case *types.Matrix:
		// Matrix constructors can take columns or scalar
		return t

	case *types.Struct:
		// Struct constructors take one arg per field
		if len(argTypes) != len(ty.Fields) {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidArgCount),
				"struct constructor for '%s' expects %d arguments, got %d",
				ty.Name, len(ty.Fields), len(argTypes))
			return nil
		}
		for i, field := range ty.Fields {
			if argTypes[i] != nil && !types.CanConvertTo(argTypes[i], field.Type) {
				v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeInvalidArgType),
					"struct '%s' field '%s': cannot convert '%s' to '%s'",
					ty.Name, field.Name, argTypes[i].String(), field.Type.String())
			}
		}
		return t

	case *types.Array:
		// Array constructors
		return t
	}

	return t
}

func (v *Validator) checkIndex(e *ast.IndexExpr) types.Type {
	baseType := v.checkExpr(e.Base)
	indexType := v.checkExpr(e.Index)

	if baseType == nil {
		return nil
	}

	// Check index type
	if indexType != nil && !types.IsInteger(indexType) {
		v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeTypeMismatch),
			"array index must be integer type, got '%s'", indexType.String())
	}

	// Get element type
	switch t := baseType.(type) {
	case *types.Array:
		return t.Element
	case *types.Vector:
		return t.Element
	case *types.Matrix:
		return &types.Vector{Width: t.Rows, Element: t.Element}
	case *types.Pointer:
		// Indexing through pointer to array
		if arr, ok := t.Element.(*types.Array); ok {
			return arr.Element
		}
	case *types.Reference:
		// Indexing through reference to array
		if arr, ok := t.Element.(*types.Array); ok {
			return arr.Element
		}
	}

	v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeNotIndexable),
		"type '%s' is not indexable", baseType.String())
	return nil
}

func (v *Validator) checkMember(e *ast.MemberExpr) types.Type {
	baseType := v.checkExpr(e.Base)
	if baseType == nil {
		return nil
	}

	// Handle pointer/reference dereferencing
	for {
		if ptr, ok := baseType.(*types.Pointer); ok {
			baseType = ptr.Element
		} else if ref, ok := baseType.(*types.Reference); ok {
			baseType = ref.Element
		} else {
			break
		}
	}

	switch t := baseType.(type) {
	case *types.Struct:
		field := t.GetField(e.Member)
		if field == nil {
			v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeNoSuchMember),
				"struct '%s' has no member '%s'", t.Name, e.Member)
			return nil
		}
		return field.Type

	case *types.Vector:
		// Swizzle access
		if len(e.Member) == 1 {
			return t.Element
		}
		// Multi-component swizzle
		if len(e.Member) <= 4 {
			return &types.Vector{Width: len(e.Member), Element: t.Element}
		}
		v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeNoSuchMember),
			"invalid swizzle '%s'", e.Member)
		return nil
	}

	v.errorWithCode(int(e.Loc.Start), string(diagnostic.CodeNoSuchMember),
		"type '%s' has no member '%s'", baseType.String(), e.Member)
	return nil
}

// ----------------------------------------------------------------------------
// Phase 5: Uniformity Analysis
// ----------------------------------------------------------------------------

func (v *Validator) analyzeUniformity() {
	v.uniformityAnalyzer = NewUniformityAnalyzer(v.module, v.diags, v.options.DiagnosticFilters)
	v.uniformityAnalyzer.Analyze()
}

// ----------------------------------------------------------------------------
// Helper Functions
// ----------------------------------------------------------------------------

func (v *Validator) resolveType(t ast.Type) types.Type {
	if t == nil {
		return nil
	}

	switch ty := t.(type) {
	case *ast.IdentType:
		return v.lookupType(ty.Name)

	case *ast.VecType:
		var elemType *types.Scalar
		if ty.ElemType != nil {
			et := v.resolveType(ty.ElemType)
			if s, ok := et.(*types.Scalar); ok {
				elemType = s
			}
		} else if ty.Shorthand != "" {
			// Handle shorthand like vec3f
			elemType = v.shorthandElement(ty.Shorthand)
		}
		if elemType == nil {
			elemType = types.F32 // Default
		}
		return &types.Vector{Width: int(ty.Size), Element: elemType}

	case *ast.MatType:
		var elemType *types.Scalar
		if ty.ElemType != nil {
			et := v.resolveType(ty.ElemType)
			if s, ok := et.(*types.Scalar); ok {
				elemType = s
			}
		} else if ty.Shorthand != "" {
			elemType = v.shorthandElement(ty.Shorthand)
		}
		if elemType == nil {
			elemType = types.F32
		}
		return &types.Matrix{Cols: int(ty.Cols), Rows: int(ty.Rows), Element: elemType}

	case *ast.ArrayType:
		elemType := v.resolveType(ty.ElemType)
		if elemType == nil {
			return nil
		}
		count := 0
		if ty.Size != nil {
			// TODO: Evaluate constant expression for array size
			if lit, ok := ty.Size.(*ast.LiteralExpr); ok {
				fmt.Sscanf(lit.Value, "%d", &count)
			}
		}
		return &types.Array{Element: elemType, Count: count}

	case *ast.PtrType:
		elemType := v.resolveType(ty.ElemType)
		if elemType == nil {
			return nil
		}
		return &types.Pointer{
			AddressSpace: types.AddressSpace(ty.AddressSpace),
			Element:      elemType,
			AccessMode:   types.AccessMode(ty.AccessMode),
		}

	case *ast.AtomicType:
		elemType := v.resolveType(ty.ElemType)
		if s, ok := elemType.(*types.Scalar); ok {
			return &types.Atomic{Element: s}
		}
		return nil

	case *ast.SamplerType:
		return &types.Sampler{Comparison: ty.Comparison}

	case *ast.TextureType:
		var sampledType *types.Scalar
		if ty.SampledType != nil {
			if st := v.resolveType(ty.SampledType); st != nil {
				if s, ok := st.(*types.Scalar); ok {
					sampledType = s
				}
			}
		}
		return &types.Texture{
			Kind:        types.TextureKind(ty.Kind),
			Dimension:   types.TextureDimension(ty.Dimension),
			SampledType: sampledType,
			TexelFormat: ty.TexelFormat,
			AccessMode:  types.AccessMode(ty.AccessMode),
		}
	}

	return nil
}

func (v *Validator) lookupType(name string) types.Type {
	// Check builtin types
	switch name {
	case "bool":
		return types.Bool
	case "i32":
		return types.I32
	case "u32":
		return types.U32
	case "f32":
		return types.F32
	case "f16":
		return types.F16
	case "sampler":
		return &types.Sampler{Comparison: false}
	case "sampler_comparison":
		return &types.Sampler{Comparison: true}
	}

	// Check for vector shorthand
	if strings.HasPrefix(name, "vec") {
		return v.parseVectorShorthand(name)
	}

	// Check for matrix shorthand
	if strings.HasPrefix(name, "mat") {
		return v.parseMatrixShorthand(name)
	}

	// Check struct types
	if st, ok := v.structTypes[name]; ok {
		return st
	}

	// Check type aliases
	if t, ok := v.aliasTypes[name]; ok {
		return t
	}

	return nil
}

func (v *Validator) parseVectorShorthand(name string) types.Type {
	// vec2i, vec3f, vec4u, vec2h, vec3<f32>, etc.
	if len(name) < 4 {
		return nil
	}

	size := 0
	switch name[3] {
	case '2':
		size = 2
	case '3':
		size = 3
	case '4':
		size = 4
	default:
		return nil
	}

	if len(name) == 4 {
		return &types.Vector{Width: size, Element: types.F32} // Default to f32
	}

	var elem *types.Scalar
	switch name[4] {
	case 'i':
		elem = types.I32
	case 'u':
		elem = types.U32
	case 'f':
		elem = types.F32
	case 'h':
		elem = types.F16
	default:
		return nil
	}

	return &types.Vector{Width: size, Element: elem}
}

func (v *Validator) parseMatrixShorthand(name string) types.Type {
	// mat2x2f, mat3x3f, mat4x4f, etc.
	if len(name) < 6 {
		return nil
	}

	cols := int(name[3] - '0')
	rows := int(name[5] - '0')

	if cols < 2 || cols > 4 || rows < 2 || rows > 4 {
		return nil
	}

	elem := types.F32
	if len(name) > 6 {
		switch name[6] {
		case 'f':
			elem = types.F32
		case 'h':
			elem = types.F16
		}
	}

	return &types.Matrix{Cols: cols, Rows: rows, Element: elem}
}

func (v *Validator) shorthandElement(shorthand string) *types.Scalar {
	if len(shorthand) == 0 {
		return types.F32
	}
	switch shorthand[len(shorthand)-1] {
	case 'i':
		return types.I32
	case 'u':
		return types.U32
	case 'f':
		return types.F32
	case 'h':
		return types.F16
	default:
		return types.F32
	}
}

func (v *Validator) symbolName(ref ast.Ref) string {
	if !ref.IsValid() {
		return ""
	}
	if int(ref.InnerIndex) < len(v.module.Symbols) {
		return v.module.Symbols[ref.InnerIndex].OriginalName
	}
	return ""
}

func (v *Validator) formatTypes(types []types.Type) string {
	var parts []string
	for _, t := range types {
		if t != nil {
			parts = append(parts, t.String())
		} else {
			parts = append(parts, "?")
		}
	}
	return strings.Join(parts, ", ")
}

func (v *Validator) memberExprToString(e *ast.MemberExpr) string {
	// Convert member expression to string (for nested type names)
	var parts []string
	current := ast.Expr(e)
	for current != nil {
		switch c := current.(type) {
		case *ast.MemberExpr:
			parts = append([]string{c.Member}, parts...)
			current = c.Base
		case *ast.IdentExpr:
			parts = append([]string{c.Name}, parts...)
			current = nil
		default:
			current = nil
		}
	}
	return strings.Join(parts, ".")
}

func (v *Validator) error(loc int, format string, args ...interface{}) {
	v.diags.AddError(loc, fmt.Sprintf(format, args...))
}

func (v *Validator) errorWithCode(loc int, code string, format string, args ...interface{}) {
	v.diags.AddErrorWithCode(loc, code, fmt.Sprintf(format, args...))
}

func (v *Validator) warning(loc int, format string, args ...interface{}) {
	if v.options.StrictMode {
		v.diags.AddError(loc, fmt.Sprintf(format, args...))
	} else {
		v.diags.AddWarning(loc, fmt.Sprintf(format, args...))
	}
}
