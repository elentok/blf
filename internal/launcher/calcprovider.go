package launcher

import (
	"github.com/elentok/blf/internal/launcher/calc"
)

// CalcProvider is a Provider that evaluates mathematical expressions.
// For Computational queries it returns a single result with the formatted
// value as the title. For LargeBareNumber queries it returns a
// comma-formatted row. For NameLike queries it returns nothing.
type CalcProvider struct{}

var _ Provider = CalcProvider{}

func (CalcProvider) Query(input string) []Result {
	qt := Classify(input)
	switch qt {
	case LargeBareNumber:
		// Show the comma-formatted number as a result
		v, err := calc.Evaluate(input)
		if err != nil {
			return nil
		}
		return []Result{{
			Title:  FormatNumber(v),
			Icon:   IconRoleCalc,
			Source: "calc",
			Weight: 2.0,
			Action: Action{Type: ActionCopy, Target: FormatNumber(v)},
		}}

	case Computational:
		v, err := calc.Evaluate(input)
		if err != nil {
			// Unit/currency inputs (e.g. "10$", "5km") look Computational but
			// are handled by UnitsProvider — don't show a calc error for them.
			if matchesNumberUnit(input) {
				return nil
			}
			return []Result{{
				Title: "Invalid math expression",
				Icon:  IconRoleCalc,
			}}
		}
		formatted := FormatNumber(v)
		return []Result{{
			Title:  formatted,
			Icon:   IconRoleCalc,
			Source: "calc",
			Weight: 2.0,
			Action: Action{Type: ActionCopy, Target: formatted},
		}}

	default:
		return nil
	}
}

// Hint returns "= <value>" for a real math expression, or "" otherwise.
// Unit/currency inputs (handled by UnitsProvider) and bare numbers get no hint.
func (CalcProvider) Hint(query string) string {
	if Classify(query) != Computational || matchesNumberUnit(query) {
		return ""
	}
	v, err := calc.Evaluate(query)
	if err != nil {
		return ""
	}
	return "= " + FormatNumber(v)
}
