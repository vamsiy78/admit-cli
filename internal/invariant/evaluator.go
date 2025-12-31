package invariant

import (
	"fmt"
)

// EvalContext provides values for invariant evaluation
type EvalContext struct {
	ConfigValues map[string]string // Resolved config values
	ExecutionEnv string            // ADMIT_ENV value
}

// Evaluate evaluates a single invariant against the context
// Returns an InvariantResult with the evaluation outcome
func Evaluate(inv Invariant, ctx EvalContext) InvariantResult {
	result := InvariantResult{
		Name:   inv.Name,
		Rule:   inv.Rule,
		Passed: true,
	}

	passed, leftVal, rightVal, msg := evalExpr(inv.Expr, ctx)
	result.Passed = passed
	result.LeftValue = leftVal
	result.RightValue = rightVal
	result.Message = msg

	return result
}

// EvaluateAll evaluates all invariants against the context
// Returns results for all invariants (both passing and failing)
func EvaluateAll(invariants []Invariant, ctx EvalContext) []InvariantResult {
	results := make([]InvariantResult, 0, len(invariants))
	for _, inv := range invariants {
		results = append(results, Evaluate(inv, ctx))
	}
	return results
}

// evalExpr evaluates a rule expression and returns:
// - passed: whether the expression evaluated to true
// - leftVal: the evaluated left operand value (for reporting)
// - rightVal: the evaluated right operand value (for reporting)
// - message: human-readable explanation
func evalExpr(expr RuleExpr, ctx EvalContext) (passed bool, leftVal, rightVal, message string) {
	switch e := expr.(type) {
	case Implication:
		return evalImplication(e, ctx)
	case Comparison:
		return evalComparison(e, ctx)
	case ConfigRef:
		val := resolveValue(e, ctx)
		return val != "", val, "", ""
	case ExecutionEnv:
		val := ctx.ExecutionEnv
		return val != "", val, "", ""
	case StringLiteral:
		return true, e.Value, "", ""
	default:
		return false, "", "", "unknown expression type"
	}
}

// evalImplication evaluates an implication expression: A => B
// Returns true if A is false OR B is true (logical implication)
func evalImplication(impl Implication, ctx EvalContext) (passed bool, leftVal, rightVal, message string) {
	// Evaluate antecedent (left side)
	antPassed, antLeft, antRight, _ := evalExpr(impl.Antecedent, ctx)

	// Evaluate consequent (right side)
	conPassed, conLeft, conRight, _ := evalExpr(impl.Consequent, ctx)

	// Implication truth table: A => B is true if A is false OR B is true
	// (F,F) -> T, (F,T) -> T, (T,F) -> F, (T,T) -> T
	passed = !antPassed || conPassed

	// For reporting, we want to show the values from both sides
	// Left value comes from antecedent, right value from consequent
	if antLeft != "" {
		leftVal = antLeft
	} else {
		leftVal = antRight
	}
	if conLeft != "" {
		rightVal = conLeft
	} else {
		rightVal = conRight
	}

	if !passed {
		message = fmt.Sprintf("condition '%s' is true but '%s' is false",
			FormatRule(impl.Antecedent), FormatRule(impl.Consequent))
	}

	return passed, leftVal, rightVal, message
}

// evalComparison evaluates a comparison expression: A == B or A != B
func evalComparison(comp Comparison, ctx EvalContext) (passed bool, leftVal, rightVal, message string) {
	leftVal = resolveValue(comp.Left, ctx)
	rightVal = resolveValue(comp.Right, ctx)

	switch comp.Operator {
	case OpEqual:
		passed = leftVal == rightVal
		if !passed {
			message = fmt.Sprintf("'%s' != '%s'", leftVal, rightVal)
		}
	case OpNotEqual:
		passed = leftVal != rightVal
		if !passed {
			message = fmt.Sprintf("'%s' == '%s'", leftVal, rightVal)
		}
	default:
		passed = false
		message = fmt.Sprintf("unknown operator: %s", comp.Operator)
	}

	return passed, leftVal, rightVal, message
}

// resolveValue resolves a rule expression to its string value
func resolveValue(expr RuleExpr, ctx EvalContext) string {
	switch e := expr.(type) {
	case ConfigRef:
		if val, ok := ctx.ConfigValues[e.Path]; ok {
			return val
		}
		return ""
	case ExecutionEnv:
		return ctx.ExecutionEnv
	case StringLiteral:
		return e.Value
	case Comparison:
		// For nested comparisons, evaluate and return "true" or "false"
		passed, _, _, _ := evalComparison(e, ctx)
		if passed {
			return "true"
		}
		return "false"
	case Implication:
		// For nested implications, evaluate and return "true" or "false"
		passed, _, _, _ := evalImplication(e, ctx)
		if passed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}
