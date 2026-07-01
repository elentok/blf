package directories

import "strings"

// Directory is a named filesystem location the launcher can open.
type Directory struct {
	Name string
	Path string // may contain "~", expanded by the caller before use
}

// Builtins are the directories shipped with blf. User config adds or
// overrides them.
var Builtins = []Directory{
	{"Home", "~"},
	{"Desktop", "~/Desktop"},
	{"Downloads", "~/Downloads"},
	{"Documents", "~/Documents"},
	{"iCloud", "~/Library/Mobile Documents/com~apple~CloudDocs"},
}

// Merge returns the built-in directories with user overrides applied.
// User directories with a name matching a built-in replace the built-in;
// new names are appended.
func Merge(builtins, user []Directory) []Directory {
	result := make([]Directory, len(builtins))
	copy(result, builtins)
	for _, u := range user {
		found := false
		for i, b := range result {
			if strings.EqualFold(b.Name, u.Name) {
				result[i] = u
				found = true
				break
			}
		}
		if !found {
			result = append(result, u)
		}
	}
	return result
}
