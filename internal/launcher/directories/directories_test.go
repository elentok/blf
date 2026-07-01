package directories

import "testing"

func TestMerge_override(t *testing.T) {
	builtins := []Directory{
		{"Home", "~"},
		{"Desktop", "~/Desktop"},
	}
	user := []Directory{
		{"desktop", "~/OtherDesktop"}, // case-insensitive override
		{"Projects", "~/dev"},         // new entry
	}

	result := Merge(builtins, user)

	want := []Directory{
		{"Home", "~"},
		{"desktop", "~/OtherDesktop"},
		{"Projects", "~/dev"},
	}
	if len(result) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(result), len(want), result)
	}
	for i := range want {
		if result[i] != want[i] {
			t.Errorf("entry %d: got %+v, want %+v", i, result[i], want[i])
		}
	}
}
