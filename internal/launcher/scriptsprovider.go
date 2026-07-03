package launcher

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/fuzzyfinder"
	"github.com/elentok/blf/internal/launcher/scripts"
)

// ScriptRunMsg is emitted when an async script execution completes.
type ScriptRunMsg struct {
	ScriptName string
	Result     scripts.RunResult
	Output     scripts.OutputMode
}

// ScriptRunCmd executes s asynchronously and returns a ScriptRunMsg.
func ScriptRunCmd(s scripts.Script) tea.Cmd {
	return func() tea.Msg {
		result := scripts.Run(s)
		return ScriptRunMsg{ScriptName: s.Name, Result: result, Output: s.Output}
	}
}

// ScriptsProvider is a Provider that fuzzy-matches named scripts.
type ScriptsProvider struct {
	scripts []scripts.Script // platform-filtered
	weight  float64
}

var _ Provider = (*ScriptsProvider)(nil)

// NewScriptsProvider creates a ScriptsProvider from a platform-filtered list of scripts.
func NewScriptsProvider(ss []scripts.Script, weight float64) *ScriptsProvider {
	return &ScriptsProvider{scripts: ss, weight: weight}
}

// Find returns the script with the given name, or false if not found.
func (p *ScriptsProvider) Find(name string) (scripts.Script, bool) {
	for _, s := range p.scripts {
		if strings.EqualFold(s.Name, name) {
			return s, true
		}
	}
	return scripts.Script{}, false
}

func (p *ScriptsProvider) Query(input string) []Result {
	if Classify(input) == Computational {
		return nil
	}
	q := strings.TrimSpace(input)
	if q == "" || len(p.scripts) == 0 {
		return nil
	}

	names := make([]string, len(p.scripts))
	for i, s := range p.scripts {
		names[i] = s.Name
	}

	matches := fuzzyfinder.Find(q, names)
	results := make([]Result, 0, len(matches))
	for _, m := range matches {
		s := p.scripts[m.Index]
		lowerName := strings.ToLower(s.Name)
		lowerQ := strings.ToLower(q)
		results = append(results, Result{
			Title:         s.Name,
			Icon:          IconRoleScript,
			IconGlyph:     s.Icon,
			Source:        "scripts",
			Weight:        p.weight,
			FuzzyScore:    m.Score,
			MatchRanges:   m.MatchedIndexes,
			IsExactMatch:  lowerName == lowerQ,
			IsPrefixMatch: strings.HasPrefix(lowerName, lowerQ),
			Action:        Action{Type: ActionRun, Target: s.Name},
		})
	}
	return results
}

// LookupResult implements TargetLookupProvider by finding the script whose
// name matches action.Target.
func (p *ScriptsProvider) LookupResult(action Action) (Result, bool) {
	if action.Type != ActionRun {
		return Result{}, false
	}
	s, found := p.Find(action.Target)
	if !found {
		return Result{}, false
	}
	return Result{
		Title:     s.Name,
		Icon:      IconRoleScript,
		IconGlyph: s.Icon,
		Action:    Action{Type: ActionRun, Target: s.Name},
	}, true
}
