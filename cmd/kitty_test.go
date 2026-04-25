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
