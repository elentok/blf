package launcher

import (
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/elentok/blf/internal/cleanurl"
	"github.com/elentok/blf/internal/config"
	"github.com/elentok/blf/internal/fuzzyfinder"
	"github.com/elentok/blf/internal/launcher/apps"
	"github.com/elentok/blf/internal/launcher/commands"
)

// ReloadDoneMsg is emitted when the "reload" command finishes refreshing all
// reloadable sources (apps and snippets).
type ReloadDoneMsg struct {
	AppsIndex   *apps.Index
	AppsErr     error
	Snippets    []config.SnippetConfig
	SnippetsErr error
}

// ReloadCmd reindexes apps and reloads snippets from disk.
func ReloadCmd(homeDir, appsCachePath string, readFile func(string) ([]byte, error)) tea.Cmd {
	return func() tea.Msg {
		idx, appsErr := apps.Reindex(homeDir)
		if appsErr == nil && appsCachePath != "" {
			_ = apps.Save(appsCachePath, idx)
		}
		snippets, snippetsErr := config.LoadSnippets(readFile, homeDir)
		return ReloadDoneMsg{
			AppsIndex:   idx,
			AppsErr:     appsErr,
			Snippets:    snippets,
			SnippetsErr: snippetsErr,
		}
	}
}

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

// AIPromptKind identifies which ai-prompt-mode command entered the mode.
type AIPromptKind string

const (
	AIPromptKindAI      AIPromptKind = "ai"
	AIPromptKindImprove AIPromptKind = "improve"
)

// EnterAIPromptModeMsg flips the launcher into ai prompt mode for Kind. A
// command's Run returns a tea.Cmd and cannot mutate the model directly, so
// the ai/improve commands emit this message and Update performs the
// transition — the same shape ReloadCmd/CleanURLCmd already use.
type EnterAIPromptModeMsg struct {
	Kind AIPromptKind
}

// AIPromptCmd enters ai prompt mode for kind.
func AIPromptCmd(kind AIPromptKind) tea.Cmd {
	return func() tea.Msg { return EnterAIPromptModeMsg{Kind: kind} }
}

// NewBuiltinCommands returns the built-in commands wired to their live
// implementations; homeDir/cachePath/readFile feed the reload command's
// ReloadCmd.
func NewBuiltinCommands(homeDir, cachePath string, readFile func(string) ([]byte, error)) []commands.Command {
	return commands.NewBuiltins(
		func() tea.Cmd { return ReloadCmd(homeDir, cachePath, readFile) },
		CleanURLCmd,
		func() tea.Cmd { return AIPromptCmd(AIPromptKindAI) },
		func() tea.Cmd { return AIPromptCmd(AIPromptKindImprove) },
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
