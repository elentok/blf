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
			Title:    FormatNumber(v),
			Subtitle: input,
			Icon:     IconRoleCalc,
			Source:   "calc",
			Weight:   2.0,
			Action:   Action{Type: ActionCopy, Target: FormatNumber(v)},
		}}

	case Computational:
		v, err := calc.Evaluate(input)
		if err != nil {
			return []Result{{
				Title: "Invalid math expression",
				Icon:  IconRoleCalc,
			}}
		}
		formatted := FormatNumber(v)
		return []Result{{
			Title:    formatted,
			Subtitle: input,
			Icon:     IconRoleCalc,
			Source:   "calc",
			Weight:   2.0,
			Action:   Action{Type: ActionCopy, Target: formatted},
		}}

	default:
		return nil
	}
}
