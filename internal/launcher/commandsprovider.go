package launcher

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/cleanurl"
	"github.com/elentok/blf/internal/fuzzyfinder"
	"github.com/elentok/blf/internal/launcher/commands"
)

// CleanURLDoneMsg is emitted when the cleanurl command finishes.
type CleanURLDoneMsg struct {
	Err error
}

// CleanURLCmd cleans the URL currently on the clipboard, in place, asynchronously.
func CleanURLCmd() tea.Cmd {
	return func() tea.Msg {
		err := cleanurl.RunClipboard()
		return CleanURLDoneMsg{Err: err}
	}
}

// NewBuiltinCommands returns the built-in commands wired to their live
// implementations; homeDir/cachePath feed the reload command's ReindexCmd.
func NewBuiltinCommands(homeDir, cachePath string) []commands.Command {
	return commands.NewBuiltins(
		func() tea.Cmd { return ReindexCmd(homeDir, cachePath) },
		CleanURLCmd,
	)
}

// CommandsProvider is a Provider that fuzzy-matches built-in commands.
type CommandsProvider struct {
	commands []commands.Command
	weight   float64
}

var _ Provider = (*CommandsProvider)(nil)

// NewCommandsProvider creates a CommandsProvider from a list of commands.
func NewCommandsProvider(cmds []commands.Command, weight float64) *CommandsProvider {
	return &CommandsProvider{commands: cmds, weight: weight}
}

// Find returns the command with the given name, or false if not found.
func (p *CommandsProvider) Find(name string) (commands.Command, bool) {
	for _, c := range p.commands {
		if strings.EqualFold(c.Name, name) {
			return c, true
		}
	}
	return commands.Command{}, false
}

func (p *CommandsProvider) Query(input string) []Result {
	if Classify(input) == Computational {
		return nil
	}
	q := strings.TrimSpace(input)
	if q == "" || len(p.commands) == 0 {
		return nil
	}

	names := make([]string, len(p.commands))
	for i, c := range p.commands {
		names[i] = c.Name
	}

	matches := fuzzyfinder.Find(q, names)
	results := make([]Result, 0, len(matches))
	for _, m := range matches {
		c := p.commands[m.Index]
		lowerName := strings.ToLower(c.Name)
		lowerQ := strings.ToLower(q)
		results = append(results, Result{
			Title:         c.Name,
			Icon:          IconRoleCommand,
			IconGlyph:     c.Icon,
			Source:        "commands",
			Weight:        p.weight,
			FuzzyScore:    m.Score,
			MatchRanges:   m.MatchedIndexes,
			IsExactMatch:  lowerName == lowerQ,
			IsPrefixMatch: strings.HasPrefix(lowerName, lowerQ),
			Action:        Action{Type: ActionCommand, Target: c.Name},
		})
	}
	return results
}

// LookupResult implements TargetLookupProvider by finding the command whose
// name matches action.Target.
func (p *CommandsProvider) LookupResult(action Action) (Result, bool) {
	if action.Type != ActionCommand {
		return Result{}, false
	}
	c, found := p.Find(action.Target)
	if !found {
		return Result{}, false
	}
	return Result{
		Title:     c.Name,
		Icon:      IconRoleCommand,
		IconGlyph: c.Icon,
		Action:    Action{Type: ActionCommand, Target: c.Name},
	}, true
}
