package cmd

import (
	"github.com/spf13/cobra"

	internalkitty "github.com/elentok/blf/internal/kitty"
)

func newKittyCmd(d deps) *cobra.Command {
	kitty := &cobra.Command{
		Use:   "kitty",
		Short: "Kitty terminal utilities",
	}

	kitty.AddCommand(
		newKittyLSCmd(d),
		newKittyListOSWindowsCmd(d),
		newKittyGotoOSWindowCmd(d),
		newKittyTargetsCmd(d),
		newKittyListAgentsCmd(d),
		newKittySetAgentStateCmd(d),
		newKittySetupClaudeCmd(d),
		newKittyGotoAgentCmd(d),
		newKittyPreviewAgentCmd(d),
		newKittyNewSessionCmd(d),
		newKittySessionsCmd(d),
		newKittyDeleteSessionCmd(d),
		newKittyDoctorCmd(d),
		newKittyPreviewSessionCmd(d),
		newKittyListSessionChoicesCmd(d),
		newKittyDeleteSessionFileCmd(d),
		newKittyEditSessionFileCmd(d),
	)

	return kitty
}

func newKittyLSCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   internalkitty.LSCmd,
		Short: "List kitty state",
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.LSCommand(kittyDepsFromCmd(d))
		},
	}
}

func newKittyListOSWindowsCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   internalkitty.ListOSWindowsCmd,
		Short: "List OS windows",
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.ListOSWindowsCommand(kittyDepsFromCmd(d))
		},
	}
}

func newKittyGotoOSWindowCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   internalkitty.GotoOSWindowCmd + " [id]",
		Short: "Go to an OS window",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := ""
			if len(args) > 0 {
				id = args[0]
			}
			return internalkitty.GotoOSWindow(id, kittyDepsFromCmd(d))
		},
	}
}

func newKittyTargetsCmd(d deps) *cobra.Command {
	var overlay bool
	var target string

	cmd := &cobra.Command{
		Use:   internalkitty.TargetsCmd,
		Short: "Show targets in current kitty window",
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.Targets(overlay, target, kittyDepsFromCmd(d))
		},
	}
	cmd.Flags().BoolVar(&overlay, "overlay", false, "Run in overlay mode")
	cmd.Flags().StringVar(&target, "target", "", "Target window ID")
	return cmd
}

func newKittyListAgentsCmd(d deps) *cobra.Command {
	var asJSON bool

	cmd := &cobra.Command{
		Use:   internalkitty.ListAgentsCmd,
		Short: "List open AI agent windows and their status",
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.ListAgentsCommand(asJSON, kittyDepsFromCmd(d))
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Output as JSON")
	return cmd
}

func newKittySetAgentStateCmd(d deps) *cobra.Command {
	var onlyIfWorking bool

	cmd := &cobra.Command{
		Use:   internalkitty.SetAgentStateCmd + " <working|waiting|idle>",
		Short: "Publish the calling window's agent status as a Kitty user var",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.SetAgentState(args[0], onlyIfWorking, kittyDepsFromCmd(d))
		},
	}
	cmd.Flags().BoolVar(&onlyIfWorking, "only-if-working", false,
		"Apply only when the window's current state is working (used by the Notification hook to ignore the ~60s idle nag)")
	return cmd
}

func newKittySetupClaudeCmd(d deps) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   internalkitty.SetupClaudeCmd,
		Short: "Install agent-state hooks into ~/.claude/settings.json (idempotent)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.SetupClaude(dryRun, kittyDepsFromCmd(d))
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the resulting changes without writing")
	return cmd
}

func newKittyGotoAgentCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   internalkitty.GotoAgentCmd,
		Short: "Pick an open AI agent window and focus it",
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.GotoAgent(kittyDepsFromCmd(d))
		},
	}
}

func newKittyPreviewAgentCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:    internalkitty.PreviewAgentCmd + " <id>",
		Short:  "Preview an agent window's screen",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.PreviewAgent(args[0], kittyDepsFromCmd(d))
		},
	}
}

func newKittyNewSessionCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   internalkitty.NewSessionCmd,
		Short: "Create a new kitty session",
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.NewSession(kittyDepsFromCmd(d))
		},
	}
}

func newKittySessionsCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   internalkitty.SessionsCmd,
		Short: "Open kitty sessions picker",
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.SessionsCommand(kittyDepsFromCmd(d))
		},
	}
}

func newKittyDeleteSessionCmd(d deps) *cobra.Command {
	var overlay bool

	cmd := &cobra.Command{
		Use:   internalkitty.DeleteSessionCmd,
		Short: "Delete a kitty session",
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.DeleteSession(overlay, kittyDepsFromCmd(d))
		},
	}
	cmd.Flags().BoolVar(&overlay, "overlay", false, "Run in overlay mode")
	return cmd
}

func newKittyDoctorCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   internalkitty.DoctorCmd,
		Short: "Run kitty diagnostics",
		// pass remaining args through to the internal function
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.Doctor(args, kittyDepsFromCmd(d))
		},
	}
}

func newKittyPreviewSessionCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:    internalkitty.PreviewSessionCmd + " <path>",
		Short:  "Preview a kitty session file",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.PreviewSession(args[0], kittyDepsFromCmd(d))
		},
	}
}

func newKittyListSessionChoicesCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:    internalkitty.ListSessionChoicesCmd,
		Short:  "List session choices for picker",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.ListSessionChoices(kittyDepsFromCmd(d))
		},
	}
}

func newKittyDeleteSessionFileCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:    internalkitty.DeleteSessionFileCmd + " <path>",
		Short:  "Delete a kitty session file",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.DeleteSessionFile(args[0], kittyDepsFromCmd(d))
		},
	}
}

func newKittyEditSessionFileCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:    internalkitty.EditSessionFileCmd + " <path>",
		Short:  "Edit a kitty session file",
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return internalkitty.EditSessionFile(args[0], kittyDepsFromCmd(d))
		},
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
