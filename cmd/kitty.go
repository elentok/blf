package cmd

import (
	"fmt"

	internalkitty "github.com/elentok/blf/internal/kitty"
)

func runKitty(args []string, d deps) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: blf kitty <ls|list-os-windows|goto-os-window|targets|new-session|sessions|delete-session|doctor> [id]")
	}

	switch args[0] {
	case internalkitty.LSCmd:
		return internalkitty.LSCommand(kittyDepsFromCmd(d))
	case internalkitty.ListOSWindowsCmd:
		return internalkitty.ListOSWindowsCommand(kittyDepsFromCmd(d))
	case internalkitty.GotoOSWindowCmd:
		return internalkitty.GotoOSWindow(args[1:], kittyDepsFromCmd(d))
	case internalkitty.TargetsCmd:
		return internalkitty.Targets(args[1:], kittyDepsFromCmd(d))
	case internalkitty.NewSessionCmd:
		return internalkitty.NewSession(args[1:], kittyDepsFromCmd(d))
	case internalkitty.SessionsCmd:
		return internalkitty.SessionsCommand(args[1:], kittyDepsFromCmd(d))
	case internalkitty.DeleteSessionCmd:
		return internalkitty.DeleteSession(args[1:], kittyDepsFromCmd(d))
	case internalkitty.DoctorCmd:
		return internalkitty.Doctor(args[1:], kittyDepsFromCmd(d))
	case internalkitty.PreviewSessionCmd:
		return internalkitty.PreviewSession(args[1:], kittyDepsFromCmd(d))
	case internalkitty.ListSessionChoicesCmd:
		return internalkitty.ListSessionChoices(args[1:], kittyDepsFromCmd(d))
	case internalkitty.DeleteSessionFileCmd:
		return internalkitty.DeleteSessionFile(args[1:], kittyDepsFromCmd(d))
	case internalkitty.EditSessionFileCmd:
		return internalkitty.EditSessionFile(args[1:], kittyDepsFromCmd(d))
	default:
		return fmt.Errorf("unknown kitty command %q", args[0])
	}
}

func kittyDepsFromCmd(d deps) internalkitty.Deps {
	return internalkitty.Deps{
		Stdin:          d.stdin,
		Stdout:         d.stdout,
		Stderr:         d.stderr,
		LookupEnv:      d.lookupEnv,
		LookPath:       d.lookPath,
		RunCommand:     d.runCommand,
		FileExists:     d.fileExists,
		RemoveFile:     d.removeFile,
		ReadFile:       d.readFile,
		ReadDir:        d.readDir,
		WriteFile:      d.writeFile,
		MkdirAll:       d.mkdirAll,
		ExecutablePath: d.executablePath,
		Getwd:          d.getwd,
		UserHomeDir:    d.userHomeDir,
	}
}
