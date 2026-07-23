package cmd

import (
	"errors"
	"reflect"
	"testing"
)

func TestLauncherHideTerminal(t *testing.T) {
	tests := []struct {
		name       string
		ghostty    bool
		wantName   string
		wantArgs   []string
		commandErr error
	}{
		{
			name:     "Ghostty quick terminal",
			ghostty:  true,
			wantName: "osascript",
			wantArgs: []string{"-e", ghosttyToggleQuickTerminalScript},
		},
		{
			name:     "Kitty quick terminal",
			wantName: "kitten",
			wantArgs: []string{"quick-access-terminal", "--instance-group", "quick"},
		},
		{
			name:       "propagates command error",
			ghostty:    true,
			wantName:   "osascript",
			wantArgs:   []string{"-e", ghosttyToggleQuickTerminalScript},
			commandErr: errors.New("automation denied"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := deps{
				lookupEnv: func(name string) (string, bool) {
					if name != ghosttyQuickTerminalEnv {
						t.Fatalf("looked up unexpected environment variable %q", name)
					}
					return "1", tt.ghostty
				},
				runCommand: func(name string, args ...string) ([]byte, error) {
					if name != tt.wantName || !reflect.DeepEqual(args, tt.wantArgs) {
						t.Fatalf("command = %q %q, want %q %q", name, args, tt.wantName, tt.wantArgs)
					}
					return nil, tt.commandErr
				},
			}

			if err := launcherHideTerminal(d)(); !errors.Is(err, tt.commandErr) {
				t.Fatalf("launcherHideTerminal() error = %v, want %v", err, tt.commandErr)
			}
		})
	}
}
