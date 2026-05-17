package cmd

import (
	"strings"
	"testing"
)

func TestRunKittyTargetsRoutesToInternalKitty(t *testing.T) {
	var calls []string
	d := deps{
		lookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		runCommand: func(name string, args ...string) ([]byte, error) {
			calls = append(calls, name+" "+strings.Join(args, " "))
			switch {
			case name == "kitty" && strings.Join(args, " ") == "@ get-text --extent screen --match id:17":
				return []byte("nothing here"), nil
			case name == "kitten" && strings.Join(args, " ") == `@ action show_error "blf kitty targets" "no targets found in current kitty window"`:
				return []byte{}, nil
			default:
				t.Fatalf("unexpected command: %s %v", name, args)
				return nil, nil
			}
		},
		stdout: &strings.Builder{},
		stderr: &strings.Builder{},
	}

	err := runKitty([]string{"targets", "--target", "17"}, d)
	if err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if strings.Join(calls, "\n") != "kitty @ get-text --extent screen --match id:17\nkitten @ action show_error \"blf kitty targets\" \"no targets found in current kitty window\"" {
		t.Fatalf("calls = %v", calls)
	}
}

func TestRunKittyLSRoutesToInternalKitty(t *testing.T) {
	out := &strings.Builder{}
	d := deps{
		runCommand: func(name string, args ...string) ([]byte, error) {
			if name != "kitty" || strings.Join(args, " ") != "@ ls" {
				t.Fatalf("unexpected command: %s %v", name, args)
			}
			return []byte(`[
				{"id":1,"is_active":true,"tabs":[
					{"id":10,"is_active":true,"title":"shell","windows":[{"id":100,"title":"editor","session_name":"proj"}]}
				]}
			]`), nil
		},
		stdout: out,
		stderr: &strings.Builder{},
	}

	if err := runKitty([]string{"ls"}, d); err != nil {
		t.Fatalf("runKitty returned error: %v", err)
	}
	if got := out.String(); got != ""+
		"- OS Window 1 (active)\n"+
		"\x1b[38;2;243;139;169;48;2;50;40;59m  - Tab 10 (active): shell\x1b[m\n"+
		"\x1b[38;2;249;226;176;48;2;51;49;59m    - Window 100 [proj]: editor\x1b[m\n"+
		"      \x1b[38;2;137;180;250m- cmdline:\x1b[m []\n"+
		"      \x1b[38;2;137;180;250m- last_reported_cmdline:\x1b[m\n"+
		"      \x1b[38;2;137;180;250m- Foreground processes:\x1b[m\n" {
		t.Fatalf("output = %q", got)
	}
}
