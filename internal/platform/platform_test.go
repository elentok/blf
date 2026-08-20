package platform

import (
	"runtime"
	"testing"
)

func TestShowNotification(t *testing.T) {
	origRunCommand := runCommand
	defer func() { runCommand = origRunCommand }()

	var gotName string
	var gotArgs []string
	runCommand = func(name string, args ...string) error {
		gotName = name
		gotArgs = args
		return nil
	}

	err := ShowNotification("Title", "Message")

	switch runtime.GOOS {
	case "darwin":
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotName != "osascript" {
			t.Errorf("name = %q, want osascript", gotName)
		}
		if len(gotArgs) != 2 || gotArgs[0] != "-e" {
			t.Fatalf("args = %v, want [-e <script>]", gotArgs)
		}
		script := gotArgs[1]
		if script != `display notification "Message" with title "Title"` {
			t.Errorf("script = %q", script)
		}
	case "linux":
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if gotName != "notify-send" {
			t.Errorf("name = %q, want notify-send", gotName)
		}
		if len(gotArgs) != 2 || gotArgs[0] != "Title" || gotArgs[1] != "Message" {
			t.Errorf("args = %v, want [Title Message]", gotArgs)
		}
	default:
		if err == nil {
			t.Fatal("expected error on unsupported platform")
		}
	}
}

func TestShowNotification_escapesAppleScriptQuotes(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("mac-only test")
	}

	origRunCommand := runCommand
	defer func() { runCommand = origRunCommand }()

	var gotScript string
	runCommand = func(name string, args ...string) error {
		gotScript = args[1]
		return nil
	}

	if err := ShowNotification(`Say "hi"`, `back\slash`); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := `display notification "back\\slash" with title "Say \"hi\""`
	if gotScript != want {
		t.Errorf("script = %q, want %q", gotScript, want)
	}
}

func TestShowNotification_multilineBody(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("mac-only test")
	}

	origRunCommand := runCommand
	defer func() { runCommand = origRunCommand }()

	var gotScript string
	runCommand = func(name string, args ...string) error {
		gotScript = args[1]
		return nil
	}

	if err := ShowNotification("Title", "line one\nline two"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "display notification \"line one\nline two\" with title \"Title\""
	if gotScript != want {
		t.Errorf("script = %q, want %q", gotScript, want)
	}
}
