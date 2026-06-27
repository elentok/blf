package launcher

// ActionType defines what to do when a result is selected.
type ActionType int

const (
	ActionCopy   ActionType = iota // copy a computed value to clipboard
	ActionLaunch                   // launch an application
	ActionRun                      // run a script
	ActionRecall                   // populate input from history (no side-effects)
	ActionOpen                     // open a file/URL via `open` (no -a flag)
)

// Action describes the operation performed when a result is selected.
type Action struct {
	Type   ActionType
	Target string // value to copy, app path to launch, or script name to run
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
}

// Provider contributes results to the launcher's ranked list.
// Query is called synchronously on every keystroke; providers must not block.
type Provider interface {
	Query(input string) []Result
}
