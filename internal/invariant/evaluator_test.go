package invariant

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

func TestEvaluate_SimpleComparison(t *testing.T) {
	tests := []struct {
		name       string
		inv        Invariant
		ctx        EvalContext
		wantPassed bool
	}{
		{
			name: "equality passes when values match",
			inv: Invariant{
				Name: "test",
				Rule: `db.env == "prod"`,
				Expr: Comparison{
					Left:     ConfigRef{Path: "db.env"},
					Right:    StringLiteral{Value: "prod"},
					Operator: OpEqual,
				},
			},
			ctx: EvalContext{
				ConfigValues: map[string]string{"db.env": "prod"},
			},
			wantPassed: true,
		},
		{
			name: "equality fails when values differ",
			inv: Invariant{
				Name: "test",
				Rule: `db.env == "prod"`,
				Expr: Comparison{
					Left:     ConfigRef{Path: "db.env"},
					Right:    StringLiteral{Value: "prod"},
					Operator: OpEqual,
				},
			},
			ctx: EvalContext{
				ConfigValues: map[string]string{"db.env": "staging"},
			},
			wantPassed: false,
		},
		{
			name: "inequality passes when values differ",
			inv: Invariant{
				Name: "test",
				Rule: `db.env != "prod"`,
				Expr: Comparison{
					Left:     ConfigRef{Path: "db.env"},
					Right:    StringLiteral{Value: "prod"},
					Operator: OpNotEqual,
				},
			},
			ctx: EvalContext{
				ConfigValues: map[string]string{"db.env": "staging"},
			},
			wantPassed: true,
		},
		{
			name: "inequality fails when values match",
			inv: Invariant{
				Name: "test",
				Rule: `db.env != "prod"`,
				Expr: Comparison{
					Left:     ConfigRef{Path: "db.env"},
					Right:    StringLiteral{Value: "prod"},
					Operator: OpNotEqual,
				},
			},
			ctx: EvalContext{
				ConfigValues: map[string]string{"db.env": "prod"},
			},
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Evaluate(tt.inv, tt.ctx)
			if result.Passed != tt.wantPassed {
				t.Errorf("Evaluate() passed = %v, want %v", result.Passed, tt.wantPassed)
			}
		})
	}
}

func TestEvaluate_ExecutionEnv(t *testing.T) {
	inv := Invariant{
		Name: "test",
		Rule: `execution.env == "prod"`,
		Expr: Comparison{
			Left:     ExecutionEnv{},
			Right:    StringLiteral{Value: "prod"},
			Operator: OpEqual,
		},
	}

	tests := []struct {
		name       string
		execEnv    string
		wantPassed bool
	}{
		{
			name:       "matches execution env",
			execEnv:    "prod",
			wantPassed: true,
		},
		{
			name:       "does not match execution env",
			execEnv:    "staging",
			wantPassed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := EvalContext{
				ConfigValues: map[string]string{},
				ExecutionEnv: tt.execEnv,
			}
			result := Evaluate(inv, ctx)
			if result.Passed != tt.wantPassed {
				t.Errorf("Evaluate() passed = %v, want %v", result.Passed, tt.wantPassed)
			}
		})
	}
}

func TestEvaluate_Implication(t *testing.T) {
	// Test: execution.env == "prod" => db.env == "prod"
	inv := Invariant{
		Name: "prod-db-guard",
		Rule: `execution.env == "prod" => db.env == "prod"`,
		Expr: Implication{
			Antecedent: Comparison{
				Left:     ExecutionEnv{},
				Right:    StringLiteral{Value: "prod"},
				Operator: OpEqual,
			},
			Consequent: Comparison{
				Left:     ConfigRef{Path: "db.env"},
				Right:    StringLiteral{Value: "prod"},
				Operator: OpEqual,
			},
		},
	}

	tests := []struct {
		name       string
		execEnv    string
		dbEnv      string
		wantPassed bool
	}{
		{
			name:       "F => F = T (not prod, not prod db)",
			execEnv:    "staging",
			dbEnv:      "staging",
			wantPassed: true,
		},
		{
			name:       "F => T = T (not prod, prod db)",
			execEnv:    "staging",
			dbEnv:      "prod",
			wantPassed: true,
		},
		{
			name:       "T => F = F (prod, not prod db) - VIOLATION",
			execEnv:    "prod",
			dbEnv:      "staging",
			wantPassed: false,
		},
		{
			name:       "T => T = T (prod, prod db)",
			execEnv:    "prod",
			dbEnv:      "prod",
			wantPassed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := EvalContext{
				ConfigValues: map[string]string{"db.env": tt.dbEnv},
				ExecutionEnv: tt.execEnv,
			}
			result := Evaluate(inv, ctx)
			if result.Passed != tt.wantPassed {
				t.Errorf("Evaluate() passed = %v, want %v", result.Passed, tt.wantPassed)
			}
		})
	}
}

func TestEvaluateAll(t *testing.T) {
	invariants := []Invariant{
		{
			Name: "inv1",
			Rule: `db.env == "prod"`,
			Expr: Comparison{
				Left:     ConfigRef{Path: "db.env"},
				Right:    StringLiteral{Value: "prod"},
				Operator: OpEqual,
			},
		},
		{
			Name: "inv2",
			Rule: `cache.env == "prod"`,
			Expr: Comparison{
				Left:     ConfigRef{Path: "cache.env"},
				Right:    StringLiteral{Value: "prod"},
				Operator: OpEqual,
			},
		},
	}

	ctx := EvalContext{
		ConfigValues: map[string]string{
			"db.env":    "prod",
			"cache.env": "staging",
		},
	}

	results := EvaluateAll(invariants, ctx)

	if len(results) != 2 {
		t.Fatalf("EvaluateAll() returned %d results, want 2", len(results))
	}

	if !results[0].Passed {
		t.Errorf("results[0].Passed = false, want true")
	}
	if results[1].Passed {
		t.Errorf("results[1].Passed = true, want false")
	}
}

// Feature: admit-v2-invariants, Property 5: Implication Evaluation Semantics
// For any implication expression A => B and evaluation context, the result
// SHALL be true if and only if A evaluates to false OR B evaluates to true
// (logical implication truth table).
// **Validates: Requirements 3.2**
func TestProperty5_ImplicationEvaluationSemantics(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for boolean pairs representing (antecedent, consequent) truth values
	genBoolPair := gopter.CombineGens(
		gen.Bool(),
		gen.Bool(),
	).Map(func(vals []interface{}) struct {
		antTrue bool
		conTrue bool
	} {
		return struct {
			antTrue bool
			conTrue bool
		}{
			antTrue: vals[0].(bool),
			conTrue: vals[1].(bool),
		}
	})

	properties.Property("implication follows truth table: A => B = !A || B", prop.ForAll(
		func(data struct {
			antTrue bool
			conTrue bool
		}) bool {
			// Create an implication where we control the truth values
			// We use string comparisons where we control both sides
			antValue := "false"
			if data.antTrue {
				antValue = "true"
			}
			conValue := "false"
			if data.conTrue {
				conValue = "true"
			}

			inv := Invariant{
				Name: "test-impl",
				Rule: `ant.val == "true" => con.val == "true"`,
				Expr: Implication{
					Antecedent: Comparison{
						Left:     ConfigRef{Path: "ant.val"},
						Right:    StringLiteral{Value: "true"},
						Operator: OpEqual,
					},
					Consequent: Comparison{
						Left:     ConfigRef{Path: "con.val"},
						Right:    StringLiteral{Value: "true"},
						Operator: OpEqual,
					},
				},
			}

			ctx := EvalContext{
				ConfigValues: map[string]string{
					"ant.val": antValue,
					"con.val": conValue,
				},
			}

			result := Evaluate(inv, ctx)

			// Expected: A => B is equivalent to !A || B
			expected := !data.antTrue || data.conTrue

			if result.Passed != expected {
				t.Logf("Implication truth table violation: A=%v, B=%v, got %v, want %v",
					data.antTrue, data.conTrue, result.Passed, expected)
				return false
			}

			return true
		},
		genBoolPair,
	))

	properties.TestingRun(t)
}


// Feature: admit-v2-invariants, Property 6: Equality/Inequality Evaluation Semantics
// For any comparison expression and evaluation context:
// - A == B SHALL return true if and only if A and B evaluate to the same string value
// - A != B SHALL return true if and only if A and B evaluate to different string values
// **Validates: Requirements 3.3, 3.4**
func TestProperty6_EqualityInequalityEvaluationSemantics(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100

	properties := gopter.NewProperties(parameters)

	// Generator for simple alphanumeric string values
	genStringValue := gen.AnyString().Map(func(s string) string {
		if len(s) == 0 {
			return "value"
		}
		result := make([]byte, 0, len(s))
		for i := 0; i < len(s) && len(result) < 20; i++ {
			ch := s[i]
			if (ch >= 'a' && ch <= 'z') || (ch >= '0' && ch <= '9') {
				result = append(result, ch)
			}
		}
		if len(result) == 0 {
			return "value"
		}
		return string(result)
	})

	// Generator for pairs of string values
	genValuePair := gopter.CombineGens(
		genStringValue,
		genStringValue,
	).Map(func(vals []interface{}) struct {
		leftVal  string
		rightVal string
	} {
		return struct {
			leftVal  string
			rightVal string
		}{
			leftVal:  vals[0].(string),
			rightVal: vals[1].(string),
		}
	})

	properties.Property("equality returns true iff values are equal", prop.ForAll(
		func(data struct {
			leftVal  string
			rightVal string
		}) bool {
			inv := Invariant{
				Name: "test-eq",
				Rule: `left.val == right.val`,
				Expr: Comparison{
					Left:     ConfigRef{Path: "left.val"},
					Right:    ConfigRef{Path: "right.val"},
					Operator: OpEqual,
				},
			}

			ctx := EvalContext{
				ConfigValues: map[string]string{
					"left.val":  data.leftVal,
					"right.val": data.rightVal,
				},
			}

			result := Evaluate(inv, ctx)

			// Expected: A == B is true iff A equals B
			expected := data.leftVal == data.rightVal

			if result.Passed != expected {
				t.Logf("Equality semantics violation: left=%q, right=%q, got %v, want %v",
					data.leftVal, data.rightVal, result.Passed, expected)
				return false
			}

			return true
		},
		genValuePair,
	))

	properties.Property("inequality returns true iff values are different", prop.ForAll(
		func(data struct {
			leftVal  string
			rightVal string
		}) bool {
			inv := Invariant{
				Name: "test-neq",
				Rule: `left.val != right.val`,
				Expr: Comparison{
					Left:     ConfigRef{Path: "left.val"},
					Right:    ConfigRef{Path: "right.val"},
					Operator: OpNotEqual,
				},
			}

			ctx := EvalContext{
				ConfigValues: map[string]string{
					"left.val":  data.leftVal,
					"right.val": data.rightVal,
				},
			}

			result := Evaluate(inv, ctx)

			// Expected: A != B is true iff A does not equal B
			expected := data.leftVal != data.rightVal

			if result.Passed != expected {
				t.Logf("Inequality semantics violation: left=%q, right=%q, got %v, want %v",
					data.leftVal, data.rightVal, result.Passed, expected)
				return false
			}

			return true
		},
		genValuePair,
	))

	properties.TestingRun(t)
}
