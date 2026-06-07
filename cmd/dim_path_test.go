package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestDimPath(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantDimmed string
		wantPlain  string
	}{
		{
			name:       "absolute path",
			input:      "/path/to/my/file\n",
			wantDimmed: "/path/to/my/",
			wantPlain:  "file",
		},
		{
			name:       "relative path",
			input:      "path/to/file\n",
			wantDimmed: "path/to/",
			wantPlain:  "file",
		},
		{
			name:       "no slash",
			input:      "Makefile\n",
			wantDimmed: "",
			wantPlain:  "Makefile",
		},
		{
			name:       "single slash prefix",
			input:      "/file\n",
			wantDimmed: "/",
			wantPlain:  "file",
		},
		{
			name:       "slash suffix",
			input:      "/path/to/dir/\n",
			wantDimmed: "/path/to/",
			wantPlain:  "dir/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			d := deps{
				stdin:  strings.NewReader(tt.input),
				stdout: &out,
			}
			if err := runDimPath(d); err != nil {
				t.Fatalf("runDimPath: %v", err)
			}

			got := out.String()
			plain := ansi.Strip(got)
			plain = strings.TrimSuffix(plain, "\n")

			want := tt.wantDimmed + tt.wantPlain
			if plain != want {
				t.Errorf("plain output = %q, want %q", plain, want)
			}

			if tt.wantDimmed != "" {
				start := strings.Index(got, "\x1b[2m")
				if start < 0 {
					t.Fatalf("expected faint ANSI code in output, got %q", got)
				}
				start += len("\x1b[2m")
				end := strings.Index(got[start:], "\x1b[m")
				if end < 0 {
					t.Fatalf("expected faint reset ANSI code in output, got %q", got)
				}
				dimmed := got[start : start+end]
				rest := got[start+end+len("\x1b[m"):]
				if dimmed != tt.wantDimmed {
					t.Errorf("dimmed prefix = %q, want %q", dimmed, tt.wantDimmed)
				}
				if rest != tt.wantPlain+"\n" {
					t.Errorf("plain suffix = %q, want %q", rest, tt.wantPlain+"\n")
				}
			}
		})
	}
}

func TestDimPathMultipleLines(t *testing.T) {
	input := "/a/b/c\n/x/y/z\n"
	var out bytes.Buffer
	d := deps{
		stdin:  strings.NewReader(input),
		stdout: &out,
	}
	if err := runDimPath(d); err != nil {
		t.Fatalf("runDimPath: %v", err)
	}

	lines := strings.Split(strings.TrimSuffix(ansi.Strip(out.String()), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "/a/b/c" {
		t.Errorf("line 0 = %q, want %q", lines[0], "/a/b/c")
	}
	if lines[1] != "/x/y/z" {
		t.Errorf("line 1 = %q, want %q", lines[1], "/x/y/z")
	}
}
