// Package builtins defines WGSL built-in functions and their type signatures.
//
// This implements the builtin function table as defined in WGSL spec section 17,
// supporting overload resolution and validation of builtin function calls.
package builtins

import (
	"github.com/HugoDaniel/miniray/internal/types"
)

// BuiltinKind identifies categories of builtin functions.
type BuiltinKind uint8

const (
	BuiltinConstructor BuiltinKind = iota // Type constructors
	BuiltinConversion                     // Bit reinterpretation
	BuiltinLogical                        // Logical operations
	BuiltinArray                          // Array operations
	BuiltinNumeric                        // Math functions
	BuiltinDerivative                     // Derivative functions (require uniform flow)
	BuiltinTexture                        // Texture sampling
	BuiltinAtomic                         // Atomic operations
	BuiltinPacking                        // Data packing/unpacking
	BuiltinSynchronization                // Barriers (require uniform flow)
	BuiltinSubgroup                       // Subgroup operations (require uniform flow)
)

// EvalStage indicates when a function can be evaluated.
type EvalStage uint8

const (
	// StageRuntime means the function can only be called at runtime.
	StageRuntime EvalStage = iota
	// StageConstEval means the function can be evaluated at compile time.
	StageConstEval
	// StageOverride means the function can be evaluated at pipeline creation.
	StageOverride
)

// UniformityRequirement indicates uniformity constraints.
type UniformityRequirement uint8

const (
	// NoUniformityReq means no uniformity requirement.
	NoUniformityReq UniformityRequirement = iota
	// RequiresUniformFlow means the call must be in uniform control flow.
	RequiresUniformFlow
	// RequiresUniformArgs means certain arguments must be uniform.
	RequiresUniformArgs
)

// Overload represents a single function overload.
type Overload struct {
	// Parameter types. nil means "any" for that position.
	Params []types.Type
	// Return type. nil means void.
	Return types.Type
	// Matcher for parametric overloads (e.g., vec<N, T> -> vec<N, T>).
	// If non-nil, this is called instead of direct type matching.
	Matcher func(args []types.Type) (types.Type, bool)
	// ConstEval indicates this overload can be const-evaluated.
	ConstEval bool
}

// Builtin represents a built-in function.
type Builtin struct {
	Name       string
	Kind       BuiltinKind
	Overloads  []Overload
	Stage      EvalStage
	Uniformity UniformityRequirement
}

// Table maps builtin function names to their definitions.
var Table = make(map[string]*Builtin)

func init() {
	// Register all builtin functions
	registerConstructors()
	registerConversions()
	registerLogical()
	registerArray()
	registerNumeric()
	registerDerivative()
	registerTexture()
	registerAtomic()
	registerPacking()
	registerSynchronization()
	registerSubgroup()
}

// Lookup returns the builtin function with the given name, or nil.
func Lookup(name string) *Builtin {
	return Table[name]
}

// IsBuiltin returns true if the name is a builtin function.
func IsBuiltin(name string) bool {
	return Table[name] != nil
}

// ResolveOverload finds the matching overload for the given arguments.
// Returns the return type and true if a match was found.
func ResolveOverload(b *Builtin, args []types.Type) (types.Type, bool) {
	for _, overload := range b.Overloads {
		// Try matcher first
		if overload.Matcher != nil {
			if ret, ok := overload.Matcher(args); ok {
				return ret, true
			}
			continue
		}

		// Direct parameter matching
		if len(args) != len(overload.Params) {
			continue
		}

		match := true
		for i, param := range overload.Params {
			if param == nil {
				continue // nil means any type
			}
			if !args[i].Equals(param) && !types.CanConvertTo(args[i], param) {
				match = false
				break
			}
		}

		if match {
			return overload.Return, true
		}
	}
	return nil, false
}

// RequiresUniform returns true if this builtin requires uniform control flow.
func (b *Builtin) RequiresUniform() bool {
	return b.Uniformity == RequiresUniformFlow
}

// IsConstEval returns true if this builtin can be evaluated at compile time.
func (b *Builtin) IsConstEval() bool {
	return b.Stage == StageConstEval
}

// register adds a builtin to the table.
func register(b *Builtin) {
	Table[b.Name] = b
}

// ----------------------------------------------------------------------------
// Constructor Builtins (Section 17.1)
// ----------------------------------------------------------------------------

func registerConstructors() {
	// Type constructors are handled specially by the type checker
	// They're not registered here but checked during type resolution
}

// ----------------------------------------------------------------------------
// Conversion/Reinterpretation Builtins (Section 17.2)
// ----------------------------------------------------------------------------

func registerConversions() {
	// bitcast<T>(e) - reinterpret bits as different type
	register(&Builtin{
		Name:  "bitcast",
		Kind:  BuiltinConversion,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchBitcast, ConstEval: true},
		},
	})
}

func matchBitcast(args []types.Type) (types.Type, bool) {
	// bitcast requires the template parameter to be specified
	// This is handled during type resolution
	return nil, false
}

// ----------------------------------------------------------------------------
// Logical Builtins (Section 17.3)
// ----------------------------------------------------------------------------

func registerLogical() {
	// all(e) - returns true if all components are true
	register(&Builtin{
		Name:  "all",
		Kind:  BuiltinLogical,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Params: []types.Type{types.Bool}, Return: types.Bool, ConstEval: true},
			{Matcher: matchAllAny, ConstEval: true},
		},
	})

	// any(e) - returns true if any component is true
	register(&Builtin{
		Name:  "any",
		Kind:  BuiltinLogical,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Params: []types.Type{types.Bool}, Return: types.Bool, ConstEval: true},
			{Matcher: matchAllAny, ConstEval: true},
		},
	})

	// select(f, t, cond) - component-wise select
	register(&Builtin{
		Name:  "select",
		Kind:  BuiltinLogical,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchSelect, ConstEval: true},
		},
	})
}

func matchAllAny(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	if v, ok := args[0].(*types.Vector); ok {
		if v.Element.Kind == types.ScalarBool {
			return types.Bool, true
		}
	}
	return nil, false
}

func matchSelect(args []types.Type) (types.Type, bool) {
	if len(args) != 3 {
		return nil, false
	}
	// f and t must have same type, cond must be bool or vecN<bool>
	if !args[0].Equals(args[1]) && !types.CanConvertTo(args[0], args[1]) {
		return nil, false
	}
	// Simple case: scalar select
	if args[2].Equals(types.Bool) {
		return args[1], true
	}
	// Vector case: vecN<T>, vecN<T>, vecN<bool>
	if v, ok := args[0].(*types.Vector); ok {
		if condVec, ok := args[2].(*types.Vector); ok {
			if condVec.Element.Kind == types.ScalarBool && condVec.Width == v.Width {
				return args[1], true
			}
		}
	}
	return nil, false
}

// ----------------------------------------------------------------------------
// Array Builtins (Section 17.4)
// ----------------------------------------------------------------------------

func registerArray() {
	// arrayLength(p) - returns the number of elements in a runtime-sized array
	register(&Builtin{
		Name:  "arrayLength",
		Kind:  BuiltinArray,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchArrayLength},
		},
	})
}

func matchArrayLength(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	// Argument must be a pointer to a runtime-sized array
	if ptr, ok := args[0].(*types.Pointer); ok {
		if arr, ok := ptr.Element.(*types.Array); ok && arr.IsRuntimeSized() {
			return types.U32, true
		}
	}
	return nil, false
}

// ----------------------------------------------------------------------------
// Numeric Builtins (Section 17.5)
// ----------------------------------------------------------------------------

func registerNumeric() {
	// Trigonometric functions
	for _, name := range []string{"sin", "cos", "tan", "asin", "acos", "atan", "sinh", "cosh", "tanh", "asinh", "acosh", "atanh"} {
		register(&Builtin{
			Name:  name,
			Kind:  BuiltinNumeric,
			Stage: StageConstEval,
			Overloads: []Overload{
				{Matcher: matchFloatUnary, ConstEval: true},
			},
		})
	}

	// atan2(y, x)
	register(&Builtin{
		Name:  "atan2",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchFloatBinary, ConstEval: true},
		},
	})

	// Exponential functions
	for _, name := range []string{"exp", "exp2", "log", "log2"} {
		register(&Builtin{
			Name:  name,
			Kind:  BuiltinNumeric,
			Stage: StageConstEval,
			Overloads: []Overload{
				{Matcher: matchFloatUnary, ConstEval: true},
			},
		})
	}

	// pow(base, exp)
	register(&Builtin{
		Name:  "pow",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchFloatBinary, ConstEval: true},
		},
	})

	// sqrt, inverseSqrt
	for _, name := range []string{"sqrt", "inverseSqrt"} {
		register(&Builtin{
			Name:  name,
			Kind:  BuiltinNumeric,
			Stage: StageConstEval,
			Overloads: []Overload{
				{Matcher: matchFloatUnary, ConstEval: true},
			},
		})
	}

	// abs - works on signed integers and floats
	register(&Builtin{
		Name:  "abs",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchSignedOrFloat, ConstEval: true},
		},
	})

	// sign
	register(&Builtin{
		Name:  "sign",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchSignedOrFloat, ConstEval: true},
		},
	})

	// floor, ceil, round, trunc, fract
	for _, name := range []string{"floor", "ceil", "round", "trunc", "fract"} {
		register(&Builtin{
			Name:  name,
			Kind:  BuiltinNumeric,
			Stage: StageConstEval,
			Overloads: []Overload{
				{Matcher: matchFloatUnary, ConstEval: true},
			},
		})
	}

	// min, max - work on all numeric types
	for _, name := range []string{"min", "max"} {
		register(&Builtin{
			Name:  name,
			Kind:  BuiltinNumeric,
			Stage: StageConstEval,
			Overloads: []Overload{
				{Matcher: matchNumericBinary, ConstEval: true},
			},
		})
	}

	// clamp(e, low, high)
	register(&Builtin{
		Name:  "clamp",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchClamp, ConstEval: true},
		},
	})

	// saturate(e) - clamp to [0, 1]
	register(&Builtin{
		Name:  "saturate",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchFloatUnary, ConstEval: true},
		},
	})

	// mix(x, y, a) - linear interpolation
	register(&Builtin{
		Name:  "mix",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchMix, ConstEval: true},
		},
	})

	// step(edge, x)
	register(&Builtin{
		Name:  "step",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchFloatBinary, ConstEval: true},
		},
	})

	// smoothstep(low, high, x)
	register(&Builtin{
		Name:  "smoothstep",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchSmoothstep, ConstEval: true},
		},
	})

	// fma(a, b, c) - fused multiply-add
	register(&Builtin{
		Name:  "fma",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchFma, ConstEval: true},
		},
	})

	// Vector operations
	register(&Builtin{
		Name:  "dot",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchDot, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "cross",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchCross, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "length",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchLength, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "distance",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchDistance, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "normalize",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchNormalize, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "reflect",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchReflect, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "refract",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchRefract, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "faceForward",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchFaceForward, ConstEval: true},
		},
	})

	// Bit operations
	for _, name := range []string{"countOneBits", "countLeadingZeros", "countTrailingZeros", "reverseBits"} {
		register(&Builtin{
			Name:  name,
			Kind:  BuiltinNumeric,
			Stage: StageConstEval,
			Overloads: []Overload{
				{Matcher: matchIntegerUnary, ConstEval: true},
			},
		})
	}

	register(&Builtin{
		Name:  "firstLeadingBit",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchIntegerUnary, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "firstTrailingBit",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchIntegerUnary, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "extractBits",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchExtractBits, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "insertBits",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchInsertBits, ConstEval: true},
		},
	})

	// Matrix operations
	register(&Builtin{
		Name:  "transpose",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchTranspose, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "determinant",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchDeterminant, ConstEval: true},
		},
	})

	// ldexp, frexp, modf
	register(&Builtin{
		Name:  "ldexp",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchLdexp, ConstEval: true},
		},
	})

	register(&Builtin{
		Name:  "frexp",
		Kind:  BuiltinNumeric,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchFrexp},
		},
	})

	register(&Builtin{
		Name:  "modf",
		Kind:  BuiltinNumeric,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchModf},
		},
	})

	// quantizeToF16
	register(&Builtin{
		Name:  "quantizeToF16",
		Kind:  BuiltinNumeric,
		Stage: StageConstEval,
		Overloads: []Overload{
			{Matcher: matchQuantizeToF16, ConstEval: true},
		},
	})
}

// Matcher functions for numeric builtins

func matchFloatUnary(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	if types.IsFloat(args[0]) {
		return args[0], true
	}
	return nil, false
}

func matchFloatBinary(args []types.Type) (types.Type, bool) {
	if len(args) != 2 {
		return nil, false
	}
	if !types.IsFloat(args[0]) || !types.IsFloat(args[1]) {
		return nil, false
	}
	common := types.CommonType(args[0], args[1])
	if common != nil {
		return common, true
	}
	return nil, false
}

func matchNumericBinary(args []types.Type) (types.Type, bool) {
	if len(args) != 2 {
		return nil, false
	}
	if !types.IsNumeric(args[0]) || !types.IsNumeric(args[1]) {
		return nil, false
	}
	common := types.CommonType(args[0], args[1])
	if common != nil {
		return common, true
	}
	return nil, false
}

func matchSignedOrFloat(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	if types.IsFloat(args[0]) {
		return args[0], true
	}
	// Check for signed integer
	if s, ok := args[0].(*types.Scalar); ok && s.Kind == types.ScalarI32 {
		return args[0], true
	}
	if v, ok := args[0].(*types.Vector); ok && v.Element.Kind == types.ScalarI32 {
		return args[0], true
	}
	return nil, false
}

func matchIntegerUnary(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	if types.IsInteger(args[0]) {
		return args[0], true
	}
	return nil, false
}

func matchClamp(args []types.Type) (types.Type, bool) {
	if len(args) != 3 {
		return nil, false
	}
	if !types.IsNumeric(args[0]) {
		return nil, false
	}
	// All three must be compatible numeric types
	common := types.CommonType(args[0], args[1])
	if common == nil {
		return nil, false
	}
	common = types.CommonType(common, args[2])
	if common != nil {
		return common, true
	}
	return nil, false
}

func matchMix(args []types.Type) (types.Type, bool) {
	if len(args) != 3 {
		return nil, false
	}
	if !types.IsFloat(args[0]) || !types.IsFloat(args[1]) || !types.IsFloat(args[2]) {
		return nil, false
	}
	common := types.CommonType(args[0], args[1])
	if common == nil {
		return nil, false
	}
	// Third argument can be scalar or same vector type
	return common, true
}

func matchSmoothstep(args []types.Type) (types.Type, bool) {
	if len(args) != 3 {
		return nil, false
	}
	if !types.IsFloat(args[0]) || !types.IsFloat(args[1]) || !types.IsFloat(args[2]) {
		return nil, false
	}
	return args[2], true
}

func matchFma(args []types.Type) (types.Type, bool) {
	if len(args) != 3 {
		return nil, false
	}
	if !types.IsFloat(args[0]) || !types.IsFloat(args[1]) || !types.IsFloat(args[2]) {
		return nil, false
	}
	common := types.CommonType(args[0], args[1])
	if common == nil {
		return nil, false
	}
	common = types.CommonType(common, args[2])
	return common, common != nil
}

func matchDot(args []types.Type) (types.Type, bool) {
	if len(args) != 2 {
		return nil, false
	}
	v1, ok1 := args[0].(*types.Vector)
	v2, ok2 := args[1].(*types.Vector)
	if !ok1 || !ok2 {
		return nil, false
	}
	if v1.Width != v2.Width {
		return nil, false
	}
	if !v1.Element.IsNumeric() || !v2.Element.IsNumeric() {
		return nil, false
	}
	common := types.CommonType(v1.Element, v2.Element)
	return common, common != nil
}

func matchCross(args []types.Type) (types.Type, bool) {
	if len(args) != 2 {
		return nil, false
	}
	v1, ok1 := args[0].(*types.Vector)
	v2, ok2 := args[1].(*types.Vector)
	if !ok1 || !ok2 {
		return nil, false
	}
	// Cross product only works on vec3
	if v1.Width != 3 || v2.Width != 3 {
		return nil, false
	}
	if !v1.Element.IsFloat() || !v2.Element.IsFloat() {
		return nil, false
	}
	common := types.CommonType(v1.Element, v2.Element)
	if common == nil {
		return nil, false
	}
	return &types.Vector{Width: 3, Element: common.(*types.Scalar)}, true
}

func matchLength(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	// Works on scalar float or vector of floats
	if s, ok := args[0].(*types.Scalar); ok && s.IsFloat() {
		return args[0], true
	}
	if v, ok := args[0].(*types.Vector); ok && v.Element.IsFloat() {
		return v.Element, true
	}
	return nil, false
}

func matchDistance(args []types.Type) (types.Type, bool) {
	if len(args) != 2 {
		return nil, false
	}
	// Both must be same float type (scalar or vector)
	if !args[0].Equals(args[1]) {
		return nil, false
	}
	if s, ok := args[0].(*types.Scalar); ok && s.IsFloat() {
		return args[0], true
	}
	if v, ok := args[0].(*types.Vector); ok && v.Element.IsFloat() {
		return v.Element, true
	}
	return nil, false
}

func matchNormalize(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	if v, ok := args[0].(*types.Vector); ok && v.Element.IsFloat() {
		return args[0], true
	}
	return nil, false
}

func matchReflect(args []types.Type) (types.Type, bool) {
	if len(args) != 2 {
		return nil, false
	}
	v1, ok1 := args[0].(*types.Vector)
	v2, ok2 := args[1].(*types.Vector)
	if !ok1 || !ok2 {
		return nil, false
	}
	if v1.Width != v2.Width || !v1.Element.IsFloat() || !v2.Element.IsFloat() {
		return nil, false
	}
	common := types.CommonType(v1.Element, v2.Element)
	if common == nil {
		return nil, false
	}
	return &types.Vector{Width: v1.Width, Element: common.(*types.Scalar)}, true
}

func matchRefract(args []types.Type) (types.Type, bool) {
	if len(args) != 3 {
		return nil, false
	}
	v1, ok1 := args[0].(*types.Vector)
	v2, ok2 := args[1].(*types.Vector)
	if !ok1 || !ok2 {
		return nil, false
	}
	if v1.Width != v2.Width || !v1.Element.IsFloat() || !v2.Element.IsFloat() {
		return nil, false
	}
	// Third argument is the ratio (scalar float)
	if !types.IsFloat(args[2]) || types.IsVector(args[2]) {
		return nil, false
	}
	common := types.CommonType(v1.Element, v2.Element)
	if common == nil {
		return nil, false
	}
	return &types.Vector{Width: v1.Width, Element: common.(*types.Scalar)}, true
}

func matchFaceForward(args []types.Type) (types.Type, bool) {
	if len(args) != 3 {
		return nil, false
	}
	v1, ok1 := args[0].(*types.Vector)
	v2, ok2 := args[1].(*types.Vector)
	v3, ok3 := args[2].(*types.Vector)
	if !ok1 || !ok2 || !ok3 {
		return nil, false
	}
	if v1.Width != v2.Width || v1.Width != v3.Width {
		return nil, false
	}
	if !v1.Element.IsFloat() || !v2.Element.IsFloat() || !v3.Element.IsFloat() {
		return nil, false
	}
	return args[0], true
}

func matchExtractBits(args []types.Type) (types.Type, bool) {
	if len(args) != 3 {
		return nil, false
	}
	if !types.IsInteger(args[0]) {
		return nil, false
	}
	// offset and count must be u32
	if !args[1].Equals(types.U32) || !args[2].Equals(types.U32) {
		return nil, false
	}
	return args[0], true
}

func matchInsertBits(args []types.Type) (types.Type, bool) {
	if len(args) != 4 {
		return nil, false
	}
	if !types.IsInteger(args[0]) || !types.IsInteger(args[1]) {
		return nil, false
	}
	if !args[0].Equals(args[1]) {
		return nil, false
	}
	// offset and count must be u32
	if !args[2].Equals(types.U32) || !args[3].Equals(types.U32) {
		return nil, false
	}
	return args[0], true
}

func matchTranspose(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	if m, ok := args[0].(*types.Matrix); ok {
		return &types.Matrix{Cols: m.Rows, Rows: m.Cols, Element: m.Element}, true
	}
	return nil, false
}

func matchDeterminant(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	if m, ok := args[0].(*types.Matrix); ok && m.Cols == m.Rows {
		return m.Element, true
	}
	return nil, false
}

func matchLdexp(args []types.Type) (types.Type, bool) {
	if len(args) != 2 {
		return nil, false
	}
	if !types.IsFloat(args[0]) {
		return nil, false
	}
	// Second arg must be i32 or vecN<i32> matching first arg
	if s, ok := args[0].(*types.Scalar); ok {
		if args[1].Equals(types.I32) {
			return s, true
		}
	}
	if v, ok := args[0].(*types.Vector); ok {
		if v2, ok := args[1].(*types.Vector); ok {
			if v.Width == v2.Width && v2.Element.Kind == types.ScalarI32 {
				return v, true
			}
		}
	}
	return nil, false
}

func matchFrexp(args []types.Type) (types.Type, bool) {
	// Returns a struct with fract and exp fields - simplified for now
	if len(args) != 1 {
		return nil, false
	}
	if types.IsFloat(args[0]) {
		return args[0], true // Simplified - actually returns __frexp_result
	}
	return nil, false
}

func matchModf(args []types.Type) (types.Type, bool) {
	// Returns a struct with fract and whole fields - simplified for now
	if len(args) != 1 {
		return nil, false
	}
	if types.IsFloat(args[0]) {
		return args[0], true // Simplified - actually returns __modf_result
	}
	return nil, false
}

func matchQuantizeToF16(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	// Input must be f32 or vecN<f32>
	if s, ok := args[0].(*types.Scalar); ok && s.Kind == types.ScalarF32 {
		return args[0], true
	}
	if v, ok := args[0].(*types.Vector); ok && v.Element.Kind == types.ScalarF32 {
		return args[0], true
	}
	return nil, false
}

// ----------------------------------------------------------------------------
// Derivative Builtins (Section 17.6) - REQUIRE UNIFORM CONTROL FLOW
// ----------------------------------------------------------------------------

func registerDerivative() {
	for _, name := range []string{"dpdx", "dpdy", "fwidth", "dpdxCoarse", "dpdyCoarse", "fwidthCoarse", "dpdxFine", "dpdyFine", "fwidthFine"} {
		register(&Builtin{
			Name:       name,
			Kind:       BuiltinDerivative,
			Stage:      StageRuntime,
			Uniformity: RequiresUniformFlow,
			Overloads: []Overload{
				{Matcher: matchDerivative},
			},
		})
	}
}

func matchDerivative(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	// Must be f32 or vecN<f32>
	if s, ok := args[0].(*types.Scalar); ok && s.Kind == types.ScalarF32 {
		return args[0], true
	}
	if v, ok := args[0].(*types.Vector); ok && v.Element.Kind == types.ScalarF32 {
		return args[0], true
	}
	return nil, false
}

// ----------------------------------------------------------------------------
// Texture Builtins (Section 17.7)
// ----------------------------------------------------------------------------

func registerTexture() {
	// textureSample - requires uniform control flow
	register(&Builtin{
		Name:       "textureSample",
		Kind:       BuiltinTexture,
		Stage:      StageRuntime,
		Uniformity: RequiresUniformFlow,
		Overloads: []Overload{
			{Matcher: matchTextureSample},
		},
	})

	// textureSampleBias - requires uniform control flow
	register(&Builtin{
		Name:       "textureSampleBias",
		Kind:       BuiltinTexture,
		Stage:      StageRuntime,
		Uniformity: RequiresUniformFlow,
		Overloads: []Overload{
			{Matcher: matchTextureSampleBias},
		},
	})

	// textureSampleCompare - requires uniform control flow
	register(&Builtin{
		Name:       "textureSampleCompare",
		Kind:       BuiltinTexture,
		Stage:      StageRuntime,
		Uniformity: RequiresUniformFlow,
		Overloads: []Overload{
			{Matcher: matchTextureSampleCompare},
		},
	})

	// textureSampleCompareLevel - does NOT require uniform control flow
	register(&Builtin{
		Name:  "textureSampleCompareLevel",
		Kind:  BuiltinTexture,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchTextureSampleCompareLevel},
		},
	})

	// textureSampleLevel - does NOT require uniform control flow
	register(&Builtin{
		Name:  "textureSampleLevel",
		Kind:  BuiltinTexture,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchTextureSampleLevel},
		},
	})

	// textureSampleGrad - does NOT require uniform control flow
	register(&Builtin{
		Name:  "textureSampleGrad",
		Kind:  BuiltinTexture,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchTextureSampleGrad},
		},
	})

	// textureLoad
	register(&Builtin{
		Name:  "textureLoad",
		Kind:  BuiltinTexture,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchTextureLoad},
		},
	})

	// textureStore
	register(&Builtin{
		Name:  "textureStore",
		Kind:  BuiltinTexture,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchTextureStore},
		},
	})

	// textureDimensions
	register(&Builtin{
		Name:  "textureDimensions",
		Kind:  BuiltinTexture,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchTextureDimensions},
		},
	})

	// textureNumLayers
	register(&Builtin{
		Name:  "textureNumLayers",
		Kind:  BuiltinTexture,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchTextureNumLayers},
		},
	})

	// textureNumLevels
	register(&Builtin{
		Name:  "textureNumLevels",
		Kind:  BuiltinTexture,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchTextureNumLevels},
		},
	})

	// textureNumSamples
	register(&Builtin{
		Name:  "textureNumSamples",
		Kind:  BuiltinTexture,
		Stage: StageRuntime,
		Overloads: []Overload{
			{Matcher: matchTextureNumSamples},
		},
	})

	// textureGather and textureGatherCompare
	register(&Builtin{
		Name:       "textureGather",
		Kind:       BuiltinTexture,
		Stage:      StageRuntime,
		Uniformity: RequiresUniformFlow,
		Overloads: []Overload{
			{Matcher: matchTextureGather},
		},
	})

	register(&Builtin{
		Name:       "textureGatherCompare",
		Kind:       BuiltinTexture,
		Stage:      StageRuntime,
		Uniformity: RequiresUniformFlow,
		Overloads: []Overload{
			{Matcher: matchTextureGatherCompare},
		},
	})
}

// Simplified texture matchers - full implementation would check texture types more precisely
func matchTextureSample(args []types.Type) (types.Type, bool) {
	if len(args) < 2 {
		return nil, false
	}
	if _, ok := args[0].(*types.Texture); !ok {
		return nil, false
	}
	if _, ok := args[1].(*types.Sampler); !ok {
		return nil, false
	}
	return types.Vec(4, types.F32), true
}

func matchTextureSampleBias(args []types.Type) (types.Type, bool) {
	if len(args) < 3 {
		return nil, false
	}
	return types.Vec(4, types.F32), true
}

func matchTextureSampleCompare(args []types.Type) (types.Type, bool) {
	if len(args) < 3 {
		return nil, false
	}
	return types.F32, true
}

func matchTextureSampleCompareLevel(args []types.Type) (types.Type, bool) {
	if len(args) < 4 {
		return nil, false
	}
	return types.F32, true
}

func matchTextureSampleLevel(args []types.Type) (types.Type, bool) {
	if len(args) < 3 {
		return nil, false
	}
	return types.Vec(4, types.F32), true
}

func matchTextureSampleGrad(args []types.Type) (types.Type, bool) {
	if len(args) < 4 {
		return nil, false
	}
	return types.Vec(4, types.F32), true
}

func matchTextureLoad(args []types.Type) (types.Type, bool) {
	if len(args) < 2 {
		return nil, false
	}
	return types.Vec(4, types.F32), true
}

func matchTextureStore(args []types.Type) (types.Type, bool) {
	if len(args) < 3 {
		return nil, false
	}
	return nil, true // void return
}

func matchTextureDimensions(args []types.Type) (types.Type, bool) {
	if len(args) < 1 {
		return nil, false
	}
	// Returns u32 for 1D, vec2<u32> for 2D, vec3<u32> for 3D
	return types.Vec(2, types.U32), true // Simplified
}

func matchTextureNumLayers(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	return types.U32, true
}

func matchTextureNumLevels(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	return types.U32, true
}

func matchTextureNumSamples(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	return types.U32, true
}

func matchTextureGather(args []types.Type) (types.Type, bool) {
	if len(args) < 3 {
		return nil, false
	}
	return types.Vec(4, types.F32), true
}

func matchTextureGatherCompare(args []types.Type) (types.Type, bool) {
	if len(args) < 4 {
		return nil, false
	}
	return types.Vec(4, types.F32), true
}

// ----------------------------------------------------------------------------
// Atomic Builtins (Section 17.8)
// ----------------------------------------------------------------------------

func registerAtomic() {
	atomicOps := []string{
		"atomicLoad", "atomicStore", "atomicAdd", "atomicSub",
		"atomicMax", "atomicMin", "atomicAnd", "atomicOr", "atomicXor",
		"atomicExchange", "atomicCompareExchangeWeak",
	}

	for _, name := range atomicOps {
		register(&Builtin{
			Name:  name,
			Kind:  BuiltinAtomic,
			Stage: StageRuntime,
			Overloads: []Overload{
				{Matcher: matchAtomic},
			},
		})
	}
}

func matchAtomic(args []types.Type) (types.Type, bool) {
	if len(args) < 1 {
		return nil, false
	}
	// First arg must be pointer to atomic
	if ptr, ok := args[0].(*types.Pointer); ok {
		if atomic, ok := ptr.Element.(*types.Atomic); ok {
			return atomic.Element, true
		}
	}
	return nil, false
}

// ----------------------------------------------------------------------------
// Data Packing Builtins (Section 17.9-17.10)
// ----------------------------------------------------------------------------

func registerPacking() {
	// Packing functions
	packFuncs := []string{"pack4x8snorm", "pack4x8unorm", "pack2x16snorm", "pack2x16unorm", "pack2x16float", "pack4xI8", "pack4xU8", "pack4xI8Clamp", "pack4xU8Clamp"}
	for _, name := range packFuncs {
		register(&Builtin{
			Name:  name,
			Kind:  BuiltinPacking,
			Stage: StageConstEval,
			Overloads: []Overload{
				{Matcher: matchPack, ConstEval: true},
			},
		})
	}

	// Unpacking functions
	unpackFuncs := []string{"unpack4x8snorm", "unpack4x8unorm", "unpack2x16snorm", "unpack2x16unorm", "unpack2x16float", "unpack4xI8", "unpack4xU8"}
	for _, name := range unpackFuncs {
		register(&Builtin{
			Name:  name,
			Kind:  BuiltinPacking,
			Stage: StageConstEval,
			Overloads: []Overload{
				{Matcher: matchUnpack, ConstEval: true},
			},
		})
	}
}

func matchPack(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	return types.U32, true
}

func matchUnpack(args []types.Type) (types.Type, bool) {
	if len(args) != 1 {
		return nil, false
	}
	if !args[0].Equals(types.U32) {
		return nil, false
	}
	return types.Vec(4, types.F32), true // Simplified - actual return type varies
}

// ----------------------------------------------------------------------------
// Synchronization Builtins (Section 17.11) - REQUIRE UNIFORM CONTROL FLOW
// ----------------------------------------------------------------------------

func registerSynchronization() {
	barriers := []string{"workgroupBarrier", "storageBarrier", "textureBarrier", "workgroupUniformLoad"}

	for _, name := range barriers {
		register(&Builtin{
			Name:       name,
			Kind:       BuiltinSynchronization,
			Stage:      StageRuntime,
			Uniformity: RequiresUniformFlow,
			Overloads: []Overload{
				{Return: nil}, // void return
			},
		})
	}
}

// ----------------------------------------------------------------------------
// Subgroup Builtins (Section 17.12) - REQUIRE UNIFORM CONTROL FLOW
// ----------------------------------------------------------------------------

func registerSubgroup() {
	// Subgroup operations all require uniform control flow
	subgroupOps := []string{
		"subgroupBallot", "subgroupBroadcast", "subgroupBroadcastFirst",
		"subgroupShuffle", "subgroupShuffleDown", "subgroupShuffleUp", "subgroupShuffleXor",
		"subgroupAdd", "subgroupMul", "subgroupAnd", "subgroupOr", "subgroupXor",
		"subgroupMin", "subgroupMax",
		"subgroupInclusiveAdd", "subgroupInclusiveMul",
		"subgroupExclusiveAdd", "subgroupExclusiveMul",
		"subgroupAll", "subgroupAny", "subgroupElect",
	}

	for _, name := range subgroupOps {
		register(&Builtin{
			Name:       name,
			Kind:       BuiltinSubgroup,
			Stage:      StageRuntime,
			Uniformity: RequiresUniformFlow,
			Overloads: []Overload{
				{Matcher: matchSubgroupOp},
			},
		})
	}
}

func matchSubgroupOp(args []types.Type) (types.Type, bool) {
	// Simplified - actual signatures vary by operation
	if len(args) >= 1 {
		return args[0], true
	}
	return types.Bool, true
}
