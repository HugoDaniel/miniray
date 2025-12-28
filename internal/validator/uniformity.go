// Package validator provides uniformity analysis for WGSL shaders.
//
// This implements the uniformity analysis as defined in WGSL spec section 15,
// detecting non-uniform control flow violations for derivative, texture sampling,
// synchronization, and subgroup operations.
package validator

import (
	"github.com/HugoDaniel/miniray/internal/ast"
	"github.com/HugoDaniel/miniray/internal/builtins"
	"github.com/HugoDaniel/miniray/internal/diagnostic"
)

// UniformityState tracks the uniformity status of control flow.
type UniformityState uint8

const (
	// Uniform means the control flow is uniform across all invocations.
	Uniform UniformityState = iota
	// MayBeNonUniform means the control flow might be non-uniform.
	MayBeNonUniform
	// NonUniform means the control flow is definitely non-uniform.
	NonUniform
)

func (s UniformityState) String() string {
	switch s {
	case Uniform:
		return "uniform"
	case MayBeNonUniform:
		return "may be non-uniform"
	case NonUniform:
		return "non-uniform"
	default:
		return "unknown"
	}
}

// UniformityAnalyzer performs uniformity analysis on WGSL functions.
type UniformityAnalyzer struct {
	module  *ast.Module
	diags   *diagnostic.DiagnosticList
	filters *diagnostic.DiagnosticFilter

	// Current function context
	currentFunc  *ast.FunctionDecl
	currentStage ShaderStage

	// Uniformity state at current point
	state UniformityState

	// Track sources of non-uniformity
	nonUniformSources []nonUniformSource
}

type nonUniformSource struct {
	loc     int
	reason  string
	builtin string
}

// NewUniformityAnalyzer creates a new uniformity analyzer.
func NewUniformityAnalyzer(module *ast.Module, diags *diagnostic.DiagnosticList, filters *diagnostic.DiagnosticFilter) *UniformityAnalyzer {
	return &UniformityAnalyzer{
		module:  module,
		diags:   diags,
		filters: filters,
	}
}

// Analyze performs uniformity analysis on all functions.
func (ua *UniformityAnalyzer) Analyze() {
	for _, decl := range ua.module.Declarations {
		if fn, ok := decl.(*ast.FunctionDecl); ok {
			ua.analyzeFunction(fn)
		}
	}
}

func (ua *UniformityAnalyzer) analyzeFunction(fn *ast.FunctionDecl) {
	ua.currentFunc = fn
	ua.state = Uniform
	ua.nonUniformSources = nil

	// Determine shader stage
	ua.currentStage = StageNone
	for _, attr := range fn.Attributes {
		switch attr.Name {
		case "vertex":
			ua.currentStage = StageVertex
		case "fragment":
			ua.currentStage = StageFragment
		case "compute":
			ua.currentStage = StageCompute
		}
	}

	// Parameters may introduce non-uniformity
	ua.analyzeParameters(fn.Parameters)

	// Analyze function body
	if fn.Body != nil {
		ua.analyzeStmt(fn.Body)
	}

	ua.currentFunc = nil
}

func (ua *UniformityAnalyzer) analyzeParameters(params []ast.Parameter) {
	for _, param := range params {
		// Check for builtin inputs that are non-uniform
		for _, attr := range param.Attributes {
			if attr.Name == "builtin" && len(attr.Args) > 0 {
				if ident, ok := attr.Args[0].(*ast.IdentExpr); ok {
					if ua.isNonUniformBuiltin(ident.Name) {
						ua.nonUniformSources = append(ua.nonUniformSources, nonUniformSource{
							loc:     int(param.Loc.Start),
							reason:  "builtin input is non-uniform",
							builtin: ident.Name,
						})
					}
				}
			}
		}
	}
}

func (ua *UniformityAnalyzer) isNonUniformBuiltin(name string) bool {
	// Most builtin inputs are non-uniform
	nonUniformBuiltins := map[string]bool{
		"vertex_index":           true,
		"instance_index":         true,
		"position":               true, // In fragment shader
		"front_facing":           true,
		"sample_index":           true,
		"sample_mask":            true,
		"local_invocation_id":    true,
		"local_invocation_index": true,
		"global_invocation_id":   true,
		"workgroup_id":           false, // Uniform within workgroup
		"num_workgroups":         false, // Uniform
	}
	return nonUniformBuiltins[name]
}

func (ua *UniformityAnalyzer) analyzeStmt(stmt ast.Stmt) {
	if stmt == nil {
		return
	}

	switch s := stmt.(type) {
	case *ast.CompoundStmt:
		for _, st := range s.Stmts {
			ua.analyzeStmt(st)
		}

	case *ast.IfStmt:
		ua.analyzeIfStmt(s)

	case *ast.SwitchStmt:
		ua.analyzeSwitchStmt(s)

	case *ast.LoopStmt:
		ua.analyzeLoopStmt(s)

	case *ast.WhileStmt:
		ua.analyzeWhileStmt(s)

	case *ast.ForStmt:
		ua.analyzeForStmt(s)

	case *ast.ReturnStmt:
		if s.Value != nil {
			ua.analyzeExpr(s.Value)
		}

	case *ast.AssignStmt:
		ua.analyzeExpr(s.Left)
		ua.analyzeExpr(s.Right)

	case *ast.CallStmt:
		ua.analyzeExpr(s.Call)

	case *ast.DeclStmt:
		switch d := s.Decl.(type) {
		case *ast.VarDecl:
			if d.Initializer != nil {
				ua.analyzeExpr(d.Initializer)
			}
		case *ast.LetDecl:
			if d.Initializer != nil {
				ua.analyzeExpr(d.Initializer)
			}
		case *ast.ConstDecl:
			if d.Initializer != nil {
				ua.analyzeExpr(d.Initializer)
			}
		}

	case *ast.IncrDecrStmt:
		ua.analyzeExpr(s.Expr)
	}
}

func (ua *UniformityAnalyzer) analyzeIfStmt(s *ast.IfStmt) {
	// Analyze condition
	condNonUniform := ua.analyzeExprUniformity(s.Condition)

	// If condition is non-uniform, the body executes in non-uniform control flow
	prevState := ua.state
	if condNonUniform {
		ua.state = NonUniform
	}

	ua.analyzeStmt(s.Body)

	if s.Else != nil {
		ua.analyzeStmt(s.Else)
	}

	// Restore state after if (simplified - actual analysis is more complex)
	ua.state = prevState
}

func (ua *UniformityAnalyzer) analyzeSwitchStmt(s *ast.SwitchStmt) {
	condNonUniform := ua.analyzeExprUniformity(s.Expr)

	prevState := ua.state
	if condNonUniform {
		ua.state = NonUniform
	}

	for _, clause := range s.Cases {
		for _, sel := range clause.Selectors {
			if sel != nil {
				ua.analyzeExpr(sel)
			}
		}
		ua.analyzeStmt(clause.Body)
	}

	ua.state = prevState
}

func (ua *UniformityAnalyzer) analyzeLoopStmt(s *ast.LoopStmt) {
	// Loop body may be non-uniform if there's a break/continue dependent on non-uniform value
	prevState := ua.state

	ua.analyzeStmt(s.Body)
	if s.Continuing != nil {
		ua.analyzeStmt(s.Continuing)
	}

	ua.state = prevState
}

func (ua *UniformityAnalyzer) analyzeWhileStmt(s *ast.WhileStmt) {
	condNonUniform := ua.analyzeExprUniformity(s.Condition)

	prevState := ua.state
	if condNonUniform {
		ua.state = NonUniform
	}

	ua.analyzeStmt(s.Body)
	ua.state = prevState
}

func (ua *UniformityAnalyzer) analyzeForStmt(s *ast.ForStmt) {
	if s.Init != nil {
		ua.analyzeStmt(s.Init)
	}

	condNonUniform := false
	if s.Condition != nil {
		condNonUniform = ua.analyzeExprUniformity(s.Condition)
	}

	prevState := ua.state
	if condNonUniform {
		ua.state = NonUniform
	}

	ua.analyzeStmt(s.Body)

	if s.Update != nil {
		ua.analyzeStmt(s.Update)
	}

	ua.state = prevState
}

func (ua *UniformityAnalyzer) analyzeExpr(expr ast.Expr) {
	if expr == nil {
		return
	}

	switch e := expr.(type) {
	case *ast.CallExpr:
		ua.analyzeCallExpr(e)

	case *ast.BinaryExpr:
		ua.analyzeExpr(e.Left)
		ua.analyzeExpr(e.Right)

	case *ast.UnaryExpr:
		ua.analyzeExpr(e.Operand)

	case *ast.IndexExpr:
		ua.analyzeExpr(e.Base)
		ua.analyzeExpr(e.Index)

	case *ast.MemberExpr:
		ua.analyzeExpr(e.Base)

	case *ast.ParenExpr:
		ua.analyzeExpr(e.Expr)
	}
}

func (ua *UniformityAnalyzer) analyzeCallExpr(e *ast.CallExpr) {
	// Get function name
	var calleeName string
	if ident, ok := e.Func.(*ast.IdentExpr); ok {
		calleeName = ident.Name
	}

	// Check arguments
	for _, arg := range e.Args {
		ua.analyzeExpr(arg)
	}

	// Check if this is a builtin that requires uniform control flow
	if builtin := builtins.Lookup(calleeName); builtin != nil {
		if builtin.RequiresUniform() && ua.state != Uniform {
			ua.reportUniformityError(e, calleeName, builtin.Kind)
		}
	}
}

func (ua *UniformityAnalyzer) analyzeExprUniformity(expr ast.Expr) bool {
	if expr == nil {
		return false
	}

	switch e := expr.(type) {
	case *ast.IdentExpr:
		// Check if identifier refers to non-uniform source
		for _, src := range ua.nonUniformSources {
			if src.builtin == e.Name {
				return true
			}
		}
		// Check if it's a builtin that produces non-uniform values
		if ua.isNonUniformBuiltin(e.Name) {
			return true
		}
		return false

	case *ast.CallExpr:
		// Some builtin functions produce non-uniform results
		var calleeName string
		if ident, ok := e.Func.(*ast.IdentExpr); ok {
			calleeName = ident.Name
		}
		// Most texture sampling results are non-uniform between invocations
		if builtin := builtins.Lookup(calleeName); builtin != nil {
			if builtin.Kind == builtins.BuiltinTexture {
				return true // Simplified - actual check is more nuanced
			}
		}
		// Check if any argument is non-uniform
		for _, arg := range e.Args {
			if ua.analyzeExprUniformity(arg) {
				return true
			}
		}
		return false

	case *ast.BinaryExpr:
		return ua.analyzeExprUniformity(e.Left) || ua.analyzeExprUniformity(e.Right)

	case *ast.UnaryExpr:
		return ua.analyzeExprUniformity(e.Operand)

	case *ast.IndexExpr:
		return ua.analyzeExprUniformity(e.Base) || ua.analyzeExprUniformity(e.Index)

	case *ast.MemberExpr:
		return ua.analyzeExprUniformity(e.Base)

	case *ast.ParenExpr:
		return ua.analyzeExprUniformity(e.Expr)

	case *ast.LiteralExpr:
		return false // Literals are always uniform
	}

	return false
}

func (ua *UniformityAnalyzer) reportUniformityError(e *ast.CallExpr, funcName string, kind builtins.BuiltinKind) {
	var loc int
	if ident, ok := e.Func.(*ast.IdentExpr); ok {
		loc = int(ident.Loc.Start)
	} else {
		loc = int(e.Loc.Start)
	}

	// Determine which diagnostic rule to use
	var rule string
	var code diagnostic.DiagnosticCode

	switch kind {
	case builtins.BuiltinDerivative:
		rule = diagnostic.RuleDerivativeUniformity
		code = diagnostic.CodeNonUniformDerivative
	case builtins.BuiltinSynchronization:
		rule = "" // Always an error, cannot be filtered
		code = diagnostic.CodeNonUniformBarrier
	case builtins.BuiltinTexture:
		rule = diagnostic.RuleDerivativeUniformity // Texture sampling that uses implicit LOD
		code = diagnostic.CodeNonUniformTexture
	case builtins.BuiltinSubgroup:
		rule = diagnostic.RuleSubgroupUniformity
		code = diagnostic.CodeNonUniformSubgroup
	default:
		return
	}

	// Check if this rule is filtered
	if rule != "" && ua.filters != nil && ua.filters.IsDisabled(rule) {
		return
	}

	// Determine severity
	severity := diagnostic.Error
	if rule != "" && ua.filters != nil {
		severity = ua.filters.GetSeverity(rule, diagnostic.Error)
	}

	// Build error message
	var message string
	switch kind {
	case builtins.BuiltinDerivative:
		message = "'" + funcName + "' must only be called from uniform control flow"
	case builtins.BuiltinSynchronization:
		message = "'" + funcName + "' must only be called from uniform control flow"
	case builtins.BuiltinTexture:
		message = "'" + funcName + "' with implicit level-of-detail must only be called from uniform control flow"
	case builtins.BuiltinSubgroup:
		message = "'" + funcName + "' requires uniform control flow"
	}

	// Add context about non-uniformity source
	if len(ua.nonUniformSources) > 0 {
		src := ua.nonUniformSources[0]
		message += " (control flow depends on '" + src.builtin + "' which is " + src.reason + ")"
	}

	ua.diags.Add(diagnostic.Diagnostic{
		Severity: severity,
		Code:     string(code),
		Message:  message,
		Range:    ua.diags.MakeRange(loc, loc+len(funcName)),
		SpecRef:  "15", // WGSL spec section 15: Uniformity
	})
}
