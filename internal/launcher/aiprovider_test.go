package launcher

import (
	"strings"
	"testing"

	"github.com/elentok/blf/internal/launcher/ai"
)

func runN(id string, kind ai.Kind, status ai.Status, input, response string) ai.Run {
	return ai.Run{ID: id, Kind: kind, Input: input, Response: response, Status: status}
}

func TestAIProvider_QueryEmptyInput_ReturnsNewestSuccessfulRuns(t *testing.T) {
	store := ai.NewStore()
	// Append oldest first so most recent ends up at index 0.
	for i := range 7 {
		store.Append(runN(string(rune('a'+i)), ai.KindAI, ai.StatusSuccess, "input", "response"))
	}
	provider := NewAIProvider(store)

	results := provider.Query("")

	if len(results) != aiRecentRunLimit {
		t.Fatalf("expected %d results, got %d", aiRecentRunLimit, len(results))
	}
	// Most recently appended run ("g") should be first.
	if results[0].Action.Target != "g" {
		t.Errorf("expected newest run first, got %q", results[0].Action.Target)
	}
}

func TestAIProvider_QueryEmptyInput_ExcludesFailedRuns(t *testing.T) {
	store := ai.NewStore()
	store.Append(runN("ok", ai.KindAI, ai.StatusSuccess, "input", "response"))
	store.Append(runN("bad", ai.KindAI, ai.StatusFailure, "input", "response"))
	provider := NewAIProvider(store)

	results := provider.Query("")

	for _, r := range results {
		if r.Action.Target == "bad" {
			t.Fatalf("failed run must not appear in results")
		}
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
}

func TestAIProvider_QueryNonEmptyInput_ReturnsNothing(t *testing.T) {
	store := ai.NewStore()
	store.Append(runN("a", ai.KindAI, ai.StatusSuccess, "input", "response"))
	provider := NewAIProvider(store)

	if results := provider.Query("something"); len(results) != 0 {
		t.Fatalf("expected no results for non-empty query, got %d", len(results))
	}
}

func TestAIProvider_TitleTruncatedAndSubtitleFirstLine(t *testing.T) {
	store := ai.NewStore()
	longInput := strings.Repeat("x", 100)
	store.Append(runN("a", ai.KindAI, ai.StatusSuccess, longInput, "first line\nsecond line"))
	provider := NewAIProvider(store)

	results := provider.Query("")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if len([]rune(results[0].Title)) > aiTitleMaxLen {
		t.Errorf("expected title truncated to ~%d chars, got %d", aiTitleMaxLen, len([]rune(results[0].Title)))
	}
	if results[0].Subtitle != "first line" {
		t.Errorf("expected subtitle to be first line, got %q", results[0].Subtitle)
	}
}

func TestAIProvider_IconsDifferByKind(t *testing.T) {
	store := ai.NewStore()
	store.Append(runN("ai", ai.KindAI, ai.StatusSuccess, "input", "response"))
	store.Append(runN("improve", ai.KindImprove, ai.StatusSuccess, "input", "response"))
	provider := NewAIProvider(store)

	results := provider.Query("")
	icons := map[string]IconRole{}
	for _, r := range results {
		icons[r.Action.Target] = r.Icon
	}
	if icons["ai"] != IconRoleAI {
		t.Errorf("expected IconRoleAI for kind ai, got %v", icons["ai"])
	}
	if icons["improve"] != IconRoleImprove {
		t.Errorf("expected IconRoleImprove for kind improve, got %v", icons["improve"])
	}
	if icons["ai"] == icons["improve"] {
		t.Errorf("expected distinct icons for ai and improve kinds")
	}
}

func TestAIProvider_EmptyStore_ReturnsNoResults(t *testing.T) {
	provider := NewAIProvider(ai.NewStore())
	if results := provider.Query(""); len(results) != 0 {
		t.Fatalf("expected no results for empty store, got %d", len(results))
	}
}
