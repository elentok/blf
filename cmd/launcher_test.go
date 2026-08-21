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

func TestLauncherHideTerminalFor(t *testing.T) {
	tests := []struct {
		name       string
		standalone bool
		wantNil    bool
	}{
		{name: "standalone", standalone: true, wantNil: true},
		{name: "not standalone", standalone: false, wantNil: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := deps{
				lookupEnv: func(name string) (string, bool) {
					return "", false
				},
				runCommand: func(name string, args ...string) ([]byte, error) {
					return nil, nil
				},
			}

			got := launcherHideTerminalFor(d, tt.standalone)
			if tt.wantNil {
				if got != nil {
					t.Fatalf("launcherHideTerminalFor(%v) = non-nil, want nil", tt.standalone)
				}
				return
			}

			if got == nil {
				t.Fatalf("launcherHideTerminalFor(%v) = nil, want non-nil", tt.standalone)
			}
			wantName, wantArgs := "kitten", []string{"quick-access-terminal", "--instance-group", "quick"}
			d.runCommand = func(name string, args ...string) ([]byte, error) {
				if name != wantName || !reflect.DeepEqual(args, wantArgs) {
					t.Fatalf("command = %q %q, want %q %q", name, args, wantName, wantArgs)
				}
				return nil, nil
			}
			if err := launcherHideTerminalFor(d, tt.standalone)(); err != nil {
				t.Fatalf("launcherHideTerminalFor() error = %v, want nil", err)
			}
		})
	}
}
