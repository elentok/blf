package launcher

import (
	"strconv"

	"github.com/elentok/blf/internal/launcher/history"
)

// ActionType defines what to do when a result is selected.
type ActionType int

const (
	ActionCopy    ActionType = iota // copy a computed value to clipboard
	ActionLaunch                    // launch an application
	ActionRun                       // run a script
	ActionRecall                    // populate input from history (no side-effects)
	ActionOpen                      // open a file/URL via `open` (no -a flag)
	ActionCommand                   // run a built-in, hardcoded launcher command
	ActionAIRun                     // act on a stored ai run (Target is the run ID)
)

// Action describes the operation performed when a result is selected.
type Action struct {
	Type   ActionType
	Target string // value to copy, app path to launch, or script name to run
}

// Key returns a stable, collision-resistant identifier for the action,
// suitable for use as a learned-rank resultKey. It combines Type and Target
// so that identical Target strings under different Types never collide.
func (a Action) Key() string {
	return strconv.Itoa(int(a.Type)) + ":" + a.Target
}

// IconRole is a semantic icon category for a result row.
type IconRole int

const (
	IconRoleCalc IconRole = iota
	IconRoleUnit
	IconRoleCurrency
	IconRoleApp
	IconRoleScript
	IconRoleHistory
	IconRoleError
	IconRoleLoading
	IconRoleSettings
	IconRoleDirectory
	IconRoleCommand
	IconRoleSnippet
	IconRoleAI
	IconRoleImprove
)

// Result is one item in the launcher's ranked result list.
type Result struct {
	Title         string
	Subtitle      string
	Icon          IconRole
	IconGlyph     string  // optional: raw nerd-font glyph overrides Icon role
	MatchRanges   []int   // highlight positions in Title from sahilm/fuzzy
	Source        string  // provider name shown as source hint
	Weight        float64 // source weight for ranking; higher wins
	FuzzyScore    int     // raw sahilm/fuzzy score; higher wins
	IsExactMatch  bool    // Title exactly equals the trimmed query
	IsPrefixMatch bool    // Title starts with the trimmed query (case-insensitive)
	Action        Action
	HistoryEntry  *history.Entry // set only for rows synthesized from launcher history; nil otherwise
}

// Provider contributes results to the launcher's ranked list.
// Query is called synchronously on every keystroke; providers must not block.
type Provider interface {
	Query(input string) []Result
}

// HintProvider is an optional capability for Providers that can compute a
// dimmed-italic "history hint" for a stored query — the "= <result>" subtitle
// shown beside a launcher history row. Hint returns "" when the provider has no
// hint for the query (not its kind, fails to resolve, or data not yet loaded).
type HintProvider interface {
	Hint(query string) string
}

// TargetLookupProvider is an optional capability for Providers that can
// resolve the live Result (icon/subtitle) for a previously-recorded Action,
// used to re-derive display details for a launch/run/open launcher-history
// row instead of persisting them. Returns ok=false if this provider doesn't
// own action.Type or can no longer find action.Target (e.g. app was removed
// or renamed) — callers should fall back to a generic display.
type TargetLookupProvider interface {
	LookupResult(action Action) (Result, bool)
}
