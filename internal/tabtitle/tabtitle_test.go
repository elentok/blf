package tabtitle

import "testing"

func TestTitle(t *testing.T) {
	const home = "/home/david"

	tests := []struct {
		name string
		cmd  string
		pwd  string
		want string
	}{
		{"no command, home dir", "", home, "~"},
		{"no command, basename dir", "", "/home/david/projects/blf", "blf"},
		{"no command, root dir", "", "/", "/"},
		{"no command, main worktree", "", "/home/david/projects/foo/main", "foo/main"},
		{"no command, master worktree", "", "/home/david/projects/foo/master", "foo/master"},
		{"no command, non-branch-like basename untouched", "", "/home/david/projects/mainframe", "mainframe"},
		{"no command, long dir cut at 25", "", "/home/david/projects/a-very-long-directory-name-indeed", Ellipsis("a-very-long-directory-name-indeed", 25)},

		{"nvim drops rest", "nvim foo.go", home, iconNvim + " " + "⠀~"},
		{"nvim bare prefix match", "nvimdiff", home, iconNvim + " " + "⠀~"},
		{"sudo nvim does not match", "sudo nvim foo.go", home, Ellipsis("sudo nvim foo.go", 10) + ":" + "⠀~"},
		{"env assignment nvim does not match", "FOO=1 nvim", home, Ellipsis("FOO=1 nvim", 10) + ":" + "⠀~"},

		{"lazyg drops rest", "lazygit status", home, iconGxLazyg + " " + "⠀~"},
		{"gx drops rest", "gx tickets ls", home, iconGxLazyg + " " + "⠀~"},
		{"gxfoo matches gx prefix", "gxfoo", home, iconGxLazyg + " " + "⠀~"},

		{"npm run test drops rest", "npm run test", home, iconTest + " " + "⠀~"},
		{"npm run test:unit drops rest", "npm run test:unit", home, iconTest + " " + "⠀~"},
		{"npm run test -- foo drops rest", "npm run test -- foo", home, iconTest + " " + "⠀~"},

		{"npm run with trailing space keeps rest cut to 10", "npm run build:watch", home, Ellipsis(iconNpmRun+" build:watch", 10) + "⠀~"},
		{"bare npm run, no icon no colon", "npm run", home, "npm run" + "⠀~"},
		{"npm runx, no icon no colon", "npm runx", home, "npm runx" + "⠀~"},

		{"npx vitest with trailing space keeps rest cut to 10", "npx vitest run watch", home, Ellipsis(iconTest+" run watch", 10) + "⠀~"},
		{"bare npx vitest, no icon no colon", "npx vitest", home, "npx vitest" + "⠀~"},

		{"catch-all command gets colon", "ls -la", home, Ellipsis("ls -la", 10) + ":" + "⠀~"},
		{"catch-all command cut to 10 with colon", "go test ./... -run TestFoo", home, Ellipsis("go test ./... -run TestFoo", 10) + ":" + "⠀~"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Title(tt.cmd, tt.pwd, home)
			if got != tt.want {
				t.Errorf("Title(%q, %q, %q) = %q, want %q", tt.cmd, tt.pwd, home, got, tt.want)
			}
		})
	}
}

func TestEllipsis(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{"shorter than n is unchanged", "abc", 10, "abc"},
		{"exact length is unchanged", "0123456789", 10, "0123456789"},
		{"even n cuts evenly", "01234567890", 10, "01234…67890"},
		{"odd n rounds start up", "abcdefghijklmnopqrstuvwxyz0", 25, "abcdefghijklm…pqrstuvwxyz0"},
		{"multi-byte runes counted as runes not bytes", "一二三四五六七八九十甲乙", 10, "一二三四五…八九十甲乙"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Ellipsis(tt.s, tt.n)
			if got != tt.want {
				t.Errorf("Ellipsis(%q, %d) = %q, want %q", tt.s, tt.n, got, tt.want)
			}
		})
	}
}
