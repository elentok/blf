package launcher

import (
	"testing"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		input string
		want  QueryType
	}{
		// empty
		{"", NameLike},

		// plain names and short bare numbers → NameLike
		{"hello", NameLike},
		{"1Password", NameLike},
		{"1", NameLike},
		{"42", NameLike},
		{"123", NameLike},
		{"-5", NameLike},
		{"3.14", NameLike},

		// large bare numbers (≥4 digits) → LargeBareNumber
		{"1000", LargeBareNumber},
		{"9999", LargeBareNumber},
		{"1000000", LargeBareNumber},
		{"1234", LargeBareNumber},

		// operators make it Computational
		{"1+2", Computational},
		{"200-10", Computational},
		{"3*4", Computational},
		{"10/2", Computational},
		{"2^8", Computational},
		{"10%", Computational},
		{"200+10%", Computational},
		{"-1+2", Computational},  // unary minus then binary plus

		// <number><unit> → Computational
		{"10cm", Computational},
		{"123$", Computational},
		{"5.5kg", Computational},
		{"100m", Computational},
		{"72°F", Computational},

		// function call → Computational
		{"sqrt(2)", Computational},
		{"sin(30)", Computational},
		{"log(100)", Computational},
		{"SQRT(2)", Computational},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := Classify(tt.input)
			if got != tt.want {
				t.Errorf("Classify(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestRankOrdering(t *testing.T) {
	exact := Result{Title: "test", IsExactMatch: true, Weight: 1.0, FuzzyScore: 50}
	prefix := Result{Title: "testing", IsPrefixMatch: true, Weight: 1.0, FuzzyScore: 80}
	highWeight := Result{Title: "weighted", Weight: 2.0, FuzzyScore: 30}
	lowWeight := Result{Title: "light", Weight: 1.0, FuzzyScore: 30}
	highFuzzy := Result{Title: "fuzzy-high", Weight: 1.0, FuzzyScore: 90}
	lowFuzzy := Result{Title: "fuzzy-low", Weight: 1.0, FuzzyScore: 10}

	tests := []struct {
		name     string
		input    []Result
		wantTitles []string
	}{
		{
			name:       "exact before prefix",
			input:      []Result{prefix, exact},
			wantTitles: []string{"test", "testing"},
		},
		{
			name:       "prefix before weight",
			input:      []Result{highWeight, prefix},
			wantTitles: []string{"testing", "weighted"},
		},
		{
			name:       "higher weight before lower weight",
			input:      []Result{lowWeight, highWeight},
			wantTitles: []string{"weighted", "light"},
		},
		{
			name:       "higher fuzzy score before lower",
			input:      []Result{lowFuzzy, highFuzzy},
			wantTitles: []string{"fuzzy-high", "fuzzy-low"},
		},
		{
			name:       "full ordering: exact > prefix > weight > fuzzy",
			input:      []Result{lowFuzzy, highWeight, prefix, exact},
			wantTitles: []string{"test", "testing", "weighted", "fuzzy-low"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Rank(tt.input, nil)
			if len(got) != len(tt.wantTitles) {
				t.Fatalf("Rank() returned %d results, want %d", len(got), len(tt.wantTitles))
			}
			for i, title := range tt.wantTitles {
				if got[i].Title != title {
					actual := make([]string, len(got))
					for j, r := range got {
						actual[j] = r.Title
					}
					t.Errorf("Rank()[%d] = %q, want %q (full order: %v)", i, got[i].Title, title, actual)
					break
				}
			}
		})
	}
}

func TestRankLearnedRankOrdering(t *testing.T) {
	exact := Result{Title: "test", IsExactMatch: true, Weight: 1.0, FuzzyScore: 50, Action: Action{Type: ActionLaunch, Target: "test"}}
	learned := Result{Title: "learned", Weight: 1.0, FuzzyScore: 10, Action: Action{Type: ActionLaunch, Target: "learned"}}
	learnedHigh := Result{Title: "learned-high", Weight: 1.0, FuzzyScore: 10, Action: Action{Type: ActionLaunch, Target: "learned-high"}}
	learnedLow := Result{Title: "learned-low", Weight: 1.0, FuzzyScore: 10, Action: Action{Type: ActionLaunch, Target: "learned-low"}}
	prefix := Result{Title: "testing", IsPrefixMatch: true, Weight: 1.0, FuzzyScore: 80, Action: Action{Type: ActionLaunch, Target: "testing"}}
	highWeight := Result{Title: "weighted", Weight: 2.0, FuzzyScore: 30, Action: Action{Type: ActionLaunch, Target: "weighted"}}
	tiedA := Result{Title: "tied-a", IsPrefixMatch: true, Weight: 1.0, FuzzyScore: 10, Action: Action{Type: ActionLaunch, Target: "tied-a"}}
	tiedB := Result{Title: "tied-b", Weight: 1.0, FuzzyScore: 10, Action: Action{Type: ActionLaunch, Target: "tied-b"}}

	tests := []struct {
		name         string
		input        []Result
		learnedRanks map[string]int
		wantTitles   []string
	}{
		{
			name:         "nonzero learned-rank outranks exact match with zero count",
			input:        []Result{exact, learned},
			learnedRanks: map[string]int{learned.Action.Key(): 1},
			wantTitles:   []string{"learned", "test"},
		},
		{
			name:         "higher learned-rank count sorts first among learned results",
			input:        []Result{learnedLow, learnedHigh},
			learnedRanks: map[string]int{learnedHigh.Action.Key(): 5, learnedLow.Action.Key(): 2},
			wantTitles:   []string{"learned-high", "learned-low"},
		},
		{
			name:         "all zero counts falls back to exact > prefix > weight > fuzzy",
			input:        []Result{highWeight, prefix, exact},
			learnedRanks: map[string]int{},
			wantTitles:   []string{"test", "testing", "weighted"},
		},
		{
			name:         "nil learnedRanks behaves like all zero counts",
			input:        []Result{highWeight, prefix, exact},
			learnedRanks: nil,
			wantTitles:   []string{"test", "testing", "weighted"},
		},
		{
			name:         "tied nonzero learned-rank counts fall through to existing tiers",
			input:        []Result{tiedB, tiedA},
			learnedRanks: map[string]int{tiedA.Action.Key(): 3, tiedB.Action.Key(): 3},
			wantTitles:   []string{"tied-a", "tied-b"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Rank(tt.input, tt.learnedRanks)
			if len(got) != len(tt.wantTitles) {
				t.Fatalf("Rank() returned %d results, want %d", len(got), len(tt.wantTitles))
			}
			for i, title := range tt.wantTitles {
				if got[i].Title != title {
					actual := make([]string, len(got))
					for j, r := range got {
						actual[j] = r.Title
					}
					t.Errorf("Rank()[%d] = %q, want %q (full order: %v)", i, got[i].Title, title, actual)
					break
				}
			}
		})
	}
}

func TestFormatNumber(t *testing.T) {
	tests := []struct {
		input float64
		want  string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1,000"},
		{1234567, "1,234,567"},
		{-1000, "-1,000"},
		{1000000000, "1,000,000,000"},
	}

	for _, tt := range tests {
		got := FormatNumber(tt.input)
		if got != tt.want {
			t.Errorf("FormatNumber(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
