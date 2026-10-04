package cmd

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/elentok/blf/internal/herdr"
	"github.com/elentok/blf/internal/herdrplugin"
	"github.com/elentok/blf/internal/herdrsync"
	"github.com/elentok/blf/internal/tabtitle"
	"github.com/spf13/cobra"
)

// errTabsFailed signals that runHerdrSyncAllTabs already printed one stderr
// line per failed tab; callers should exit 1 without printing anything else.
var errTabsFailed = errors.New("")

func newHerdrCmd(d deps) *cobra.Command {
	herdrCmd := &cobra.Command{
		Use:   "herdr",
		Short: "Herdr terminal multiplexer utilities",
	}

	herdrCmd.AddCommand(
		newHerdrSyncTabTitleCmd(d),
		newInstallPluginCmd(d),
		newUninstallPluginCmd(d),
		newFindWorkspaceCmd(d),
		newGotoWorkspaceCmd(d),
		newHerdrDebugCmd(d),
	)

	return herdrCmd
}

func newHerdrSyncTabTitleCmd(d deps) *cobra.Command {
	var cmdFlag string
	var promptFlag bool

	syncTabTitleCmd := &cobra.Command{
		Use:   "sync-tab-title",
		Short: "Sync the herdr tab label from its active pane",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmdChanged := cmd.Flags().Changed("cmd")
			promptChanged := cmd.Flags().Changed("prompt")
			if cmdChanged && promptChanged {
				return fmt.Errorf("herdr sync-tab-title: --cmd and --prompt are mutually exclusive")
			}
			if cmdChanged || promptChanged {
				return runHerdrSyncTabTitleFish(d, cmdFlag)
			}
			return runHerdrSyncTabTitle(d)
		},
	}

	syncTabTitleCmd.Flags().StringVar(&cmdFlag, "cmd", "", "fish mode: the command about to run (preexec)")
	syncTabTitleCmd.Flags().BoolVar(&promptFlag, "prompt", false, "fish mode: the prompt returned")

	return syncTabTitleCmd
}

// runHerdrSyncTabTitle implements event mode. Run by a herdr hook with
// HERDR_TAB_ID set, it syncs that one tab's label from its active pane. With
// no HERDR_TAB_ID (pane.closed, [[startup]]), it syncs every tab in the
// session from one session.snapshot call.
func runHerdrSyncTabTitle(d deps) error {
	herdrEnv, _ := d.lookupEnv("HERDR_ENV")
	socketPath, _ := d.lookupEnv("HERDR_SOCKET_PATH")
	if herdrEnv == "" || socketPath == "" {
		return nil
	}

	home, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("herdr sync-tab-title: get home dir: %w", err)
	}

	client := d.newHerdrClient(socketPath)

	tabID, _ := d.lookupEnv("HERDR_TAB_ID")
	if tabID == "" {
		return runHerdrSyncAllTabs(d, client, home)
	}

	paneID, _ := d.lookupEnv("HERDR_PANE_ID")

	layout, err := client.PaneLayout(paneID)
	if err != nil {
		logHerdrSync(d, "event", tabID, "", "", err.Error())
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	activePane, err := client.PaneGet(layout.FocusedPaneID)
	if err != nil {
		logHerdrSync(d, "event", tabID, "", "", err.Error())
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	label := d.herdrLabel(activePane, home)
	if label == "" {
		logHerdrSync(d, "event", tabID, "", "", "empty label")
		return nil
	}

	tab, err := client.TabGet(tabID)
	if err != nil {
		logHerdrSync(d, "event", tabID, "", label, err.Error())
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	if tab.Label == label {
		logHerdrSync(d, "event", tabID, tab.Label, label, "unchanged")
		return nil
	}

	if err := client.TabRename(tabID, label); err != nil {
		logHerdrSync(d, "event", tabID, tab.Label, label, err.Error())
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	logHerdrSync(d, "event", tabID, tab.Label, label, "")

	return nil
}

func newInstallPluginCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "install-plugin",
		Short: "Write and link blf's herdr plugin manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInstallPlugin(d)
		},
	}
}

func newUninstallPluginCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "uninstall-plugin",
		Short: "Unlink and delete blf's herdr plugin manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUninstallPlugin(d)
		},
	}
}

// runInstallPlugin writes the generated manifest and links it with herdr,
// always relinking (a re-run is the upgrade path). After linking, it runs
// one full-session sync inside herdr, because [[startup]] doesn't fire on
// link; outside herdr it skips that sync silently. (T04, R F04, F21)
func runInstallPlugin(d deps) error {
	if _, err := d.lookPath("herdr"); err != nil {
		return fmt.Errorf("install-plugin: herdr not found on PATH")
	}

	blfPath, err := resolveBlfPath(d)
	if err != nil {
		return fmt.Errorf("install-plugin: %w", err)
	}

	home, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("install-plugin: get home dir: %w", err)
	}

	if err := d.mkdirAll(herdrplugin.Dir(home), 0o755); err != nil {
		return fmt.Errorf("install-plugin: create plugin dir: %w", err)
	}

	manifest := herdrplugin.GenerateManifest(blfPath)
	if err := d.writeFile(herdrplugin.ManifestPath(home), []byte(manifest), 0o644); err != nil {
		return fmt.Errorf("install-plugin: write manifest: %w", err)
	}

	if _, err := d.runCommand("herdr", "plugin", "link", herdrplugin.Dir(home)); err != nil {
		fmt.Fprintln(d.stderr, commandStderr(err))
		return fmt.Errorf("install-plugin: link plugin")
	}

	socketPath, _ := d.lookupEnv("HERDR_SOCKET_PATH")
	if socketPath == "" {
		return nil
	}

	client := d.newHerdrClient(socketPath)
	return runHerdrSyncAllTabs(d, client, home)
}

// runUninstallPlugin unlinks plugin blf and deletes the plugin folder.
// `herdr plugin unlink` already succeeds when the plugin isn't linked, and
// removing an already-absent folder is a no-op, so no special-casing is
// needed for "already uninstalled". (T04, R F27)
func runUninstallPlugin(d deps) error {
	if _, err := d.lookPath("herdr"); err != nil {
		return fmt.Errorf("uninstall-plugin: herdr not found on PATH")
	}

	if _, err := d.runCommand("herdr", "plugin", "unlink", herdrplugin.ID); err != nil {
		fmt.Fprintln(d.stderr, commandStderr(err))
		return fmt.Errorf("uninstall-plugin: unlink plugin")
	}

	home, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("uninstall-plugin: get home dir: %w", err)
	}

	if err := d.removeAll(herdrplugin.Dir(home)); err != nil {
		return fmt.Errorf("uninstall-plugin: remove plugin dir: %w", err)
	}

	return nil
}

// resolveBlfPath implements the <blf> path rule: the PATH entry when its
// resolved target matches the running binary (written unresolved), else the
// running binary itself. (T06, R F16)
func resolveBlfPath(d deps) (string, error) {
	runningBinary, err := d.executablePath()
	if err != nil {
		return "", fmt.Errorf("get running binary path: %w", err)
	}

	pathEntry, err := d.lookPath("blf")
	if err != nil {
		return herdrplugin.ResolveBlfPath("", "", runningBinary), nil
	}

	resolved, err := d.evalSymlinks(pathEntry)
	if err != nil {
		return "", fmt.Errorf("resolve symlinks for %q: %w", pathEntry, err)
	}

	return herdrplugin.ResolveBlfPath(pathEntry, resolved, runningBinary), nil
}

// commandStderr returns the real stderr text from a failed deps.runCommand
// call when it's available (populated by exec.Command.Output on an
// *exec.ExitError), falling back to the error's own message otherwise, e.g.
// a fake runner in tests.
func commandStderr(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && len(exitErr.Stderr) > 0 {
		return string(exitErr.Stderr)
	}
	return err.Error()
}

func newFindWorkspaceCmd(d deps) *cobra.Command {
	return &cobra.Command{
		Use:   "find-workspace <label>",
		Short: "Print the id of the workspace whose label matches (read-only)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFindWorkspace(args[0], d)
		},
	}
}

func newGotoWorkspaceCmd(d deps) *cobra.Command {
	var create bool
	var cwd string

	cmd := &cobra.Command{
		Use:   "goto-workspace <label>",
		Short: "Focus the workspace whose label matches, optionally creating it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGotoWorkspace(args[0], create, cwd, d)
		},
	}
	cmd.Flags().BoolVar(&create, "create", false, "Create the workspace when none matches")
	cmd.Flags().StringVar(&cwd, "cwd", "", "Starting directory for a created workspace (requires --create)")
	return cmd
}

// herdrClientFromDeps checks HERDR_SOCKET_PATH (never HERDR_ENV: a key-bound
// `type = "shell"` command doesn't get it) and builds a client over it.
func herdrClientFromDeps(d deps) (herdr.Client, error) {
	socketPath, ok := d.lookupEnv("HERDR_SOCKET_PATH")
	if !ok || socketPath == "" {
		return nil, fmt.Errorf("not running inside herdr")
	}
	return d.newHerdrClient(socketPath), nil
}

// findWorkspaceByLabel matches the whole label, ignoring case, against
// workspace.list's UI order; the first match wins. found == false with a nil
// error means no match, distinct from a transport/decoding error.
func findWorkspaceByLabel(client herdr.Client, label string) (herdr.WorkspaceInfo, bool, error) {
	workspaces, err := client.WorkspaceList()
	if err != nil {
		return herdr.WorkspaceInfo{}, false, err
	}
	for _, ws := range workspaces {
		if strings.EqualFold(ws.Label, label) {
			return ws, true, nil
		}
	}
	return herdr.WorkspaceInfo{}, false, nil
}

func runFindWorkspace(label string, d deps) error {
	if label == "" {
		return fmt.Errorf("find-workspace: label is required")
	}

	client, err := herdrClientFromDeps(d)
	if err != nil {
		return err
	}

	ws, found, err := findWorkspaceByLabel(client, label)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("no workspace labelled %q", label)
	}

	fmt.Fprintln(d.stdout, ws.WorkspaceID)
	return nil
}

func runGotoWorkspace(label string, create bool, cwd string, d deps) error {
	if label == "" {
		return fmt.Errorf("goto-workspace: label is required")
	}
	if cwd != "" && !create {
		return fmt.Errorf("goto-workspace: --cwd requires --create")
	}

	client, err := herdrClientFromDeps(d)
	if err != nil {
		return err
	}

	ws, found, err := findWorkspaceByLabel(client, label)
	if err != nil {
		return err
	}

	if !found {
		if !create {
			return fmt.Errorf("no workspace labelled %q", label)
		}

		// herdr silently opens $HOME for a cwd that doesn't exist; check first
		// so a typo fails loudly instead of creating the wrong workspace.
		if cwd != "" {
			exists, err := d.dirExists(cwd)
			if err != nil {
				return fmt.Errorf("goto-workspace: check cwd %q: %w", cwd, err)
			}
			if !exists {
				return fmt.Errorf("goto-workspace: cwd %q is not an existing directory", cwd)
			}
		}

		_, err := client.WorkspaceCreate(label, cwd)
		return err
	}

	return client.WorkspaceFocus(ws.WorkspaceID)
}

// runHerdrSyncAllTabs syncs every tab in the session from one
// session.snapshot call, using herdrsync.PlanTabs so the per-tab plan stays
// the same one doctor's dry-run table (ticket 10) reuses. One failed tab
// does not stop the others: each failure prints one stderr line, and the
// run exits 1 if any tab failed.
func runHerdrSyncAllTabs(d deps, client herdr.Client, home string) error {
	snapshot, err := client.SessionSnapshot()
	if err != nil {
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	plans, planErrs := herdrsync.PlanTabs(snapshot, d.herdrLabel, home)

	failed := false
	for _, planErr := range planErrs {
		fmt.Fprintln(d.stderr, "herdr sync-tab-title:", planErr)
		failed = true
	}

	for _, plan := range plans {
		if plan.NewLabel == "" || plan.NewLabel == plan.Label {
			continue
		}
		if err := client.TabRename(plan.TabID, plan.NewLabel); err != nil {
			fmt.Fprintf(d.stderr, "herdr sync-tab-title: tab %s: %v\n", plan.TabID, err)
			failed = true
		}
	}

	if failed {
		return errTabsFailed
	}
	return nil
}

// runHerdrSyncTabTitleFish implements fish mode: fired as a backgrounded
// fish hook on preexec (--cmd) or prompt (--prompt, cmdArg ""), it computes
// the tab title in-process instead of waiting for herdr's own (lagging)
// terminal title.
func runHerdrSyncTabTitleFish(d deps, cmdArg string) error {
	herdrEnv, _ := d.lookupEnv("HERDR_ENV")
	socketPath, _ := d.lookupEnv("HERDR_SOCKET_PATH")
	if herdrEnv == "" || socketPath == "" {
		return nil
	}

	tabID, _ := d.lookupEnv("HERDR_TAB_ID")
	paneID, _ := d.lookupEnv("HERDR_PANE_ID")
	if tabID == "" || paneID == "" {
		return nil
	}

	if vimruntime, _ := d.lookupEnv("VIMRUNTIME"); vimruntime != "" {
		return nil
	}

	client := d.newHerdrClient(socketPath)

	layout, err := client.PaneLayout(paneID)
	if err != nil {
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	if layout.FocusedPaneID != paneID {
		return nil
	}

	pwd, err := d.getwd()
	if err != nil {
		return fmt.Errorf("herdr sync-tab-title: get cwd: %w", err)
	}

	home, err := d.userHomeDir()
	if err != nil {
		return fmt.Errorf("herdr sync-tab-title: get home dir: %w", err)
	}

	label := tabtitle.Title(cmdArg, pwd, home)
	if label == "" {
		return nil
	}

	tab, err := client.TabGet(tabID)
	if err != nil {
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	if tab.Label == label {
		return nil
	}

	if err := client.TabRename(tabID, label); err != nil {
		return fmt.Errorf("herdr sync-tab-title: %w", err)
	}

	return nil
}
