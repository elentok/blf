# blf herdr

Sources: every statement below traces to `.scratch/blf-herdr/map.md` or one of its six closed
tickets. Tags name the source: `(C)` = charting decisions in the map's Notes, `(T01)`..`(T06)` = the
ticket with that number, `(R)` = the design review (`.scratch/blf-herdr/review-findings.md`, finding
number given) and the herdr facts verified for it (`.scratch/blf-herdr/research-socket-facts.md`).
Where two sources disagree, the later one wins.

## Problem Statement

- herdr does not name tabs after what runs in them. An unnamed tab is labelled with its position
  in the workspace (`1`, `2`, ...), and herdr never changes a label to follow the title, agent or
  cwd. (R F09)
- The logic that computes a good **tab title** (icon or short command, plus directory) exists only
  as fish functions in the dotfiles (`tab-title.fish`), with no tests. herdr already receives its
  output as the pane's **terminal title**, but the **tab label** does not follow it.
- herdr has no plugin event for "terminal title changed" or "foreground command changed", so a
  plugin alone cannot keep a **tab label** live. (T01)
- Jumping to a workspace by its **workspace label** is done by a script in the dotfiles
  (`extra/scripts/herdr-goto-workspace`), outside blf. (T06)

## Solution

herdr support inside blf, as a simplified take on `qu8n/herdr-automatic-rename`. (map Destination)

- `blf tab-title [cmd]` prints the **tab title**. The fish logic moves to Go; `tab-title.fish`
  becomes thin wrappers around it. (C, T02)
- `blf herdr sync-tab-title` keeps each herdr **tab label** in step with the **terminal title** of
  the tab's active pane. It runs from herdr plugin hooks (event mode) and from fish prompt/preexec
  hooks (fish mode). (C, T03)
- A tiny herdr plugin (a generated manifest, nothing else) calls `blf herdr sync-tab-title`.
  `blf herdr install-plugin` / `uninstall-plugin` manage it. (C, T04)
- `blf herdr find-workspace <label>` and `blf herdr goto-workspace <label>` look a workspace up by
  its **workspace label**. (T05)
- `blf herdr doctor` and `blf herdr debug on|off` make failures visible. (T03)
- The dotfiles changes are not part of this epic's code. This spec carries a **Dotfiles handoff**
  section, and the last implementation ticket publishes a separate `blf-herdr-dotfiles` epic. (T06)

## User Stories

Actors: "herdr user" = a person running fish inside herdr with blf installed. "Shell user" = a
person using `blf tab-title` with or without herdr.

### Tab title (`blf tab-title`)

1. As a shell user, I want `blf tab-title` with no argument to print the directory form only, so
   that my prompt title is the directory. (T02)
2. As a shell user, I want `blf tab-title "<cmd>"` to print the command part, then U+2800, then the
   directory, so that the title matches what fish `_tab_title` prints today. (T02)
3. As a shell user, I want the directory to be `~` only when the cwd equals `$HOME`, otherwise the
   basename, so that titles stay short. (T02)
4. As a shell user, I want a basename of `main` or `master` to print as `parent/basename`, so that
   worktree-style folders stay distinguishable. No other branch-like names get this rule. (T02)
5. As a shell user, I want a cwd of `/` to print `/`. (T02)
6. As a shell user, I want a directory longer than 25 runes cut with the ellipsis rule, and a
   command part longer than 10 runes cut the same way, so that titles have a bounded width. (T02)
7. As a shell user, I want commands starting with `nvim`, `lazyg` or `gx` to show only their icon
   plus one space, so that long editor command lines do not fill the title. (T02)
8. As a shell user, I want any command starting with `npm run test` (including `npm run test:unit`
   and `npm run test -- foo`) to show the test icon only, so that test runs are recognisable. This
   is the one deliberate change from fish. (T02)
9. As a shell user, I want commands starting with `npm run ` or `npx vitest ` to show their icon
   followed by the rest of the command, cut to 10 runes with the icon counted inside the 10. Bare
   `npm run` and bare `npx vitest` (no trailing space) print as is, with no icon and no `:`, as fish
   does today. (T02, R F12)
10. As a shell user, I want any other command cut to 10 runes and followed by `:`. (T02)
11. As a shell user, I want the icon match to be a raw prefix match on the whole command string,
    first match wins, so that `gxfoo` and `nvimdiff` match while `sudo nvim` and `FOO=1 nvim` do
    not. (T02)
12. As a shell user running a shell inside Neovim, I want `blf tab-title` to print nothing and exit 0
    when `$VIMRUNTIME` is non-empty, so that processes inside Neovim do not change the title. (T02,
    R F12)

### Tab label sync, event mode

13. As a herdr user, I want the **tab label** to follow the **terminal title** of the tab's active
    pane when I focus a pane or switch tabs (`pane.focused`), so that tabs name themselves. (C, T03)
14. As a herdr user, I want a new tab labelled when it is created (`tab.created`), including a tab
    created in the background, so that it never keeps a default name. (T01, T03)
15. As a herdr user, I want the label refreshed on `pane.agent_detected` and
    `pane.agent_status_changed`, so that an agent tab shows the agent's task description. (C, T03)
16. As a herdr user, I want every tab in the session re-synced when a pane closes (`pane.closed`),
    so that the tab is renamed after the pane that took over. herdr gives no tab id for this
    event. (T01, T03)
17. As a herdr user, I want every tab in the session synced when the herdr server starts
    (`[[startup]]`). (T03)
18. As a herdr user, I want an event from a non-active pane to sync the label from the tab's active
    pane, not from the pane that raised the event, so that a background pane never names the
    tab. (T03)
19. As a herdr user, I want an agent pane's label to be its task description with the leading
    spinner, `✳` and spaces removed, cut to 25 runes, so that a frozen spinner frame never lies
    about the status. (C, T03)
20. As a herdr user, I want an agent pane with an empty title to be labelled `<agent>: <dir>` (for
    example `claude: blf`), where `<dir>` is the directory form of the pane's `foreground_cwd`
    (`cwd` when that is missing), cut at 25 like any directory. The whole string is not cut again.
    (C, T03, R F10, F23)
21. As a herdr user, I want a shell pane's label to be its **terminal title** copied as is, cut to
    40 runes when longer, so that titles blf did not compute (programs that set their own title) are
    shown too. blf never adds a host or branch itself. (T03, R F05)
22. As a herdr user, I want a shell pane with an empty **terminal title** to be labelled with the
    directory form of the pane's `cwd`. (T03, R F10)
23. As a herdr user, I want one tab that fails to sync not to stop the other tabs in a full-session
    sync. Each failed tab prints one line on stderr, and the run exits 1 if any tab failed. (T03,
    R F13)
24. As a herdr user, I want a label I set by hand to be overwritten at the next trigger, with no
    state file and no reset action. (C, T03)

### Tab label sync, fish mode

25. As a herdr user, I want the label to change as soon as I run a command
    (`sync-tab-title --cmd "<command>"`), computed in-process from `$PWD` and the command, so that
    it does not wait for herdr's own title, which lags inside fish hooks. (C, T01, T03)
26. As a herdr user, I want the label to return to the directory form when the prompt comes back
    (`sync-tab-title --prompt`). (C, T03)
27. As a herdr user with split panes, I want a command in a pane that is not the active pane of its
    tab to leave the label alone. (C, T03)
28. As a herdr user running a shell inside Neovim, I want fish mode to do nothing when `$VIMRUNTIME`
    is non-empty. Event mode ignores `$VIMRUNTIME`: a hook runs with the herdr server's env, so the
    variable says nothing about the pane. (T03, R F18)
29. As a herdr user, I want `claude` typed in fish to show `claude:⠀blf` until
    `pane.agent_detected` replaces it; fish mode makes no agent check. (T03)
29a. As a herdr user, I want fish mode to exit 0 silently when `HERDR_TAB_ID` or `HERDR_PANE_ID` is
    unset, to treat `--cmd ""` as `--prompt`, and to reject `--cmd` and `--prompt` together as a
    usage error. (R F22)

### Tab label sync, all modes

30. As a herdr user, I want no rename sent when the label is already correct, so that blf does not
    fire needless `tab.renamed` events and session saves. (T01, T03)
31. As a user outside herdr, I want `sync-tab-title` to exit 0 with no output when `HERDR_ENV` or
    `HERDR_SOCKET_PATH` is unset. (T03)
32. As a herdr user, I want a sync that cannot reach herdr, or gets an API error (for example the
    tab closed meanwhile), to print one line on stderr and exit 1, with no retry and a 200 ms
    timeout per socket request, so that a prompt is never held up. (T03, R F13)
33. As a herdr user after a herdr update, I want a reply that lacks a field herdr always sends to
    fail with
    `herdr socket reply not understood (blf tested with herdr 0.9.3); run blf herdr doctor`, so that
    a protocol change is visible instead of silently wrong. Fields herdr omits when unset (`agent`,
    `terminal_title`, `terminal_title_stripped`, `cwd`, `foreground_cwd`) are optional and read as
    empty. (T03, ADR 0010, R F30)
33a. As a herdr user, I want blf never to send an empty label (an empty computed label means skip)
    and always to send explicit ids, because herdr stores an empty label and acts on the focused
    object when an id is missing. (R F33)
34. As a herdr user on a newer herdr, I want no version warning on each sync. (T03, ADR 0010)
35. As a herdr user with named sessions, I want each sync to talk to the session named by
    `$HERDR_SOCKET_PATH`. (T01, T03)

### Plugin install

36. As a herdr user, I want `blf herdr install-plugin` to write the manifest into blf's data dir and
    link it with herdr, so that one command sets the plugin up. (T04)
37. As a herdr user, I want to run `install-plugin` again safely, and for a second run to be the
    upgrade path: it exits 0, overwrites the manifest (also a hand-edited one), links again, and
    runs the full sync again inside herdr. (T04, R F04)
38. As a herdr user, I want the manifest to call `blf` by absolute path, so that hooks work with the
    `PATH` the herdr server started with. (T04)
39. As a Homebrew user, I want that path to be the `blf` found on `PATH` when it points to the
    running binary, written unresolved, so that `brew upgrade` does not break the hooks. (T06,
    R F16)
40. As a herdr user, I want `install-plugin` to exit 1 and write nothing when `herdr` is not on
    `PATH`. (T04)
40a. As a herdr user, I want `install-plugin` to print herdr's stderr and exit 1 when
    `herdr plugin link` fails, leaving the written manifest in place. (R F21)
41. As a herdr user, I want `install-plugin` run inside herdr to sync every tab once after linking,
    and to skip that sync silently outside herdr. When that sync fails, it prints the error and
    exits 1; the plugin stays installed. (T04, R F21)
42. As a herdr user, I want `blf herdr uninstall-plugin` to unlink plugin `blf` and delete the
    plugin folder, with no error when it is already uninstalled. When `herdr` is not on `PATH` it
    exits 1 and changes nothing. (T04, R F27)

### Doctor and debug log

43. As a herdr user, I want `blf herdr doctor` to print one pass/fail line per check and exit 1 when
    any check fails, without ever renaming a tab. Outside herdr (`HERDR_SOCKET_PATH` unset) it
    prints one failing line, skips the socket checks and exits 1. (T03, R F24, F06)
44. As a herdr user, I want `blf herdr doctor --json` to print the same report as JSON, in the
    shape given under "`blf herdr doctor`". (T03, R F25)
45. As a herdr user, I want doctor to show a dry-run table of every tab (tab id, active pane, agent,
    terminal title, current label, label blf would set, `ok` / `would rename`), where `would rename`
    is not a failure. (T03)
46. As a herdr user after a blf upgrade, I want doctor to fail the manifest check when the manifest
    on disk differs from the one blf would generate (the `blf` path excluded), and to tell me to run
    `blf herdr install-plugin`. (T04, R F14)
47. As a herdr user, I want doctor to fail a check when the `blf` path in the manifest does not
    exist or is not executable. A dev build running doctor against a Homebrew install passes.
    (T04, T06, R F14)
48. As a herdr user, I want doctor to list failed hook runs among the last 10 entries of herdr's
    plugin log. This is information only, never a failure. (T03, R F24)
49. As a herdr user, I want `blf herdr debug on` to start a fresh log (also when already on) and
    `blf herdr debug off` to stop logging and keep the log, so that I can see what each sync did.
    (T03, R F26)
50. As a herdr user, I want `blf herdr debug` with no argument to print the state and the log
    path. (T03)

### Workspace commands

51. As a herdr user, I want `blf herdr find-workspace <label>` to print the bare workspace id and a
    newline, so that `herdr workspace focus (blf herdr find-workspace notes)` works. (T05)
52. As a herdr user, I want the match to be on the whole **workspace label**, ignoring case, with no
    substring or fuzzy match. (T05)
53. As a herdr user with two workspaces of the same label, I want the first one in herdr's order to
    win, with no error. (T05)
54. As a herdr user, I want `blf herdr goto-workspace <label>` to focus the matching workspace and
    print nothing, and to do nothing when it is already focused. (T05)
55. As a herdr user, I want `goto-workspace <label> --create` to create and focus the workspace when
    none matches, with the label exactly as typed. (T05)
56. As a herdr user, I want `--cwd <path>` to set the starting directory of a created workspace, and
    to be ignored when the workspace already exists. When a workspace would be created and the path
    is not an existing directory, blf prints an error on stderr, exits 1 and creates nothing,
    because herdr would silently open `$HOME`. (T05, R F31)
57. As a herdr user, I want `--cwd` without `--create`, and an empty label, to be usage errors
    (stderr, exit 1). (T05)
58. As a herdr user, I want a missing workspace to print `no workspace labelled "notes"` on stderr
    and exit 1, with nothing on stdout, in both commands. (T05)
59. As a user outside herdr (`HERDR_SOCKET_PATH` unset), I want both commands to print
    `not running inside herdr` on stderr and exit 1, because a person or a key runs them. They do
    not look at `HERDR_ENV`: a key-bound `type = "shell"` command does not get it. (T05, R F06,
    F29)

### Docs and handoff

60. As a blf user without these dotfiles, I want the README to document the fish snippet, the
    `goto-workspace` binding and the required herdr setting (`prompt_new_tab_name = false`). (T06)

Publishing the dotfiles epic is a ticket instruction, not a story; see "Epics and tickets". (R F03)

## Implementation Decisions

### Shape

- Everything lives inside blf, not a new repo. The plugin is a tiny manifest calling
  `blf herdr ...`. (C)
- Only **tab labels** are renamed, never workspaces. (C)
- A tab is named after its active pane. (C, T03)
- New commands: `blf tab-title [cmd]`, and under a `blf herdr` group: `sync-tab-title`,
  `install-plugin`, `uninstall-plugin`, `doctor`, `debug`, `find-workspace`, `goto-workspace`.
- The command keeps the name `sync-tab-title` although it writes a **tab label**. (T03)

### `blf tab-title [cmd]` (T02)

- Prints exactly what fish `_tab_title` prints today for every reachable input. One deliberate
  change: the `npm run test` row.
- Inputs: one optional argument (the command). Directory from the process cwd, home from `$HOME`. No
  flags. An empty or missing argument gives the prompt form: directory only.
- Icon table, hard-coded in Go. Raw prefix match on the whole command string, first match wins, in
  this order:

  | Prefix         | Icon   | Rest of command             |
  | -------------- | ------ | --------------------------- |
  | `nvim`         | U+F36F | dropped                     |
  | `lazyg`        | U+EA84 | dropped                     |
  | `gx`           | U+EA84 | dropped                     |
  | `npm run test` | U+E8D9 | dropped                     |
  | `npm run`      | U+ED0D | kept, cut to 10             |
  | `npx vitest`   | U+E8D9 | kept, cut to 10             |
  | anything else  | none   | command cut to 10, then `:` |

  - Icon-only rows print the icon plus one space.
  - "Kept" rows match the prefix with no trailing space, as fish does. They replace the first
    `npm run ` / `npx vitest ` (with the space) by icon + space, then apply the 10-rune ellipsis to
    the whole string, so the icon counts inside the 10. With no such substring (bare `npm run`,
    `npm runx`) the command is cut to 10 unchanged: no icon, no `:`. (R F12)
  - `lazyg` and `gx` share one glyph on purpose.
- Ellipsis: when the text is longer than `n` runes, print the first `round(n/2)` runes, `…`
  (U+2026), then the last `n - round(n/2)` runes. The result is `n+1` runes. Counted in runes, not
  bytes. `n` is 10 for the command part and 25 for the directory.
- Separator: U+2800 between the command part and the directory. None when there is no command.
- Directory: `~` only when cwd equals `$HOME`; otherwise the basename; when the basename is `main`
  or `master`, `parent/basename`. `/` gives `/`.
- `$VIMRUNTIME` guard lives in the command layer: when set, print nothing and exit 0. Callers treat
  empty output as "do nothing". The pure function knows nothing about env.
- Module: package `internal/tabtitle` with `Title(cmd, pwd, home string) string` and
  `Ellipsis(s string, n int) string`. The sync rules reuse `Ellipsis` and the directory rule.
- No config. The icon table is not user-extendable.

### `blf herdr sync-tab-title`: event mode (no args) (T03)

- Run by herdr plugin hooks.
- Target tab: when the hook env has `HERDR_TAB_ID`, sync only that tab. When it has none
  (`pane.closed`, startup), sync every tab in the session. One failed tab does not stop the others;
  each failed tab prints one stderr line, and the run exits 1 if any tab failed. (R F13)
- The pane that names the tab is always the tab's active pane (`focused_pane_id` of the tab's
  layout). Never `HERDR_PANE_ID` itself, never the global `focused` flag.
- Agent pane (herdr's `agent` field is set):
  - Label = `terminal_title_stripped` with leading braille chars (U+2800–U+28FF), `✳` and spaces
    stripped, then `Ellipsis(s, 25)`.
  - Empty after stripping: `<agent>: <dir>`. `<dir>` is `Title("", foreground_cwd, home)` (so cut at
    25), with `cwd` when `foreground_cwd` is missing. The whole string is not cut again. (R F10, F23)
  - No transcript reading. Mid-task title changes may lag until the next status or focus change. No
    polling. (C)
- Shell pane:
  - Copy `terminal_title_stripped` as is; apply `Ellipsis(s, 40)` when longer than 40 runes.
  - Empty title: `Title("", cwd, home)`. (R F10)
- `$VIMRUNTIME` is ignored in event mode. (R F18)

### `blf herdr sync-tab-title`: fish mode (`--cmd "<command>"` / `--prompt`) (C, T03)

- Run in the background by fish hooks.
- Computes the **tab title** in-process from `$PWD` + command and renames `$HERDR_TAB_ID`.
- Skips when `$HERDR_PANE_ID` is not the active pane of its tab (`pane.layout`).
- Skips when `$VIMRUNTIME` is non-empty (empty title = do nothing).
- No agent check.
- `HERDR_TAB_ID` or `HERDR_PANE_ID` unset: exit 0, no output. `--cmd ""` is the same as `--prompt`.
  `--cmd` with `--prompt` is a usage error. (R F22)

### `blf herdr sync-tab-title`: all modes (T03)

- Rename only when the new label differs from the current one. The current label is always read from
  the socket. `HERDR_PLUGIN_CONTEXT_JSON` is not used.
- Transport: the herdr socket at `$HERDR_SOCKET_PATH`, through a small `internal/herdr` client.
  Not the herdr CLI. See ADR 0010. Methods (verified on herdr 0.9.3, R F19):
  - Single-tab sync: `pane.layout {pane_id}` (`focused_pane_id`), `pane.get {pane_id}`,
    `tab.get {tab_id}` (`label`), `tab.rename`.
  - Full-session sync and doctor: one `session.snapshot {}`, which returns `workspaces`, `tabs`,
    `panes`, `layouts[]` (one per tab, with `focused_pane_id`), `version` and `protocol`.
- Strict decoding: a reply missing a field herdr always sends (ids, `label`, `focused_pane_id`, ...)
  is an error with the message
  `herdr socket reply not understood (blf tested with herdr 0.9.3); run blf herdr doctor`. Fields
  herdr omits when unset are optional and read as empty: `agent`, `terminal_title`,
  `terminal_title_stripped`, `cwd`, `foreground_cwd`. (R F30)
- Every request sends explicit ids; a missing id makes herdr act on the focused object. A computed
  empty label is never sent (herdr would store it); it means skip. (R F33)
- No per-sync version warning.
- Outside herdr (`HERDR_ENV` or `HERDR_SOCKET_PATH` unset): exit 0, no output.
- Errors (herdr unreachable, API error): one line on stderr, exit 1, no retry. Timeout 200 ms per
  socket request. (R F13)
- `tab.renamed` is never hooked or reacted to (loop risk: blf's own renames fire it). (T01, T03)
- The sync never checks or rewrites the manifest. (T04)

### Known behaviour, no special rule (T03)

- Named sessions: covered by `$HERDR_SOCKET_PATH`.
- nvim terminals: covered by the `$VIMRUNTIME` guard.
- tmux inside a herdr pane: `HERDR_*` env is inherited, so the tab is named after the inner shell's
  command. With several tmux panes the last one to run a command wins.
- ssh / remote panes: fish hooks do not run for the remote shell. The label is the **terminal
  title** herdr sees, at the next event. blf never adds the host itself.
- Fish race: `--cmd` and `--prompt` run as separate background processes. For a very fast command
  the `--cmd` rename can land last, so the label shows the finished command until the next prompt
  or focus change. Accepted; no waiting, no state. (R F08)

### Plugin (T04, T06)

- Folder: `$XDG_DATA_HOME/blf/herdr-plugin/` (default `~/.local/share/blf/herdr-plugin/`). It holds
  only `herdr-plugin.toml`. herdr stores the path as absolute, so it must not move.
- Identity: id `blf`, name `blf tab titles`, `platforms = ["macos", "linux"]`,
  `min_herdr_version = "0.9.3"`.
- Manifest `version`: its own number as a TOML string, starting at `"1"` and raised by one each
  time the manifest changes. Not the blf version. herdr accepts any non-empty string and refuses a
  bare number. (R F28)
- Hooks: `[[events]]` for `pane.focused`, `tab.created`, `pane.agent_detected`,
  `pane.agent_status_changed`, `pane.closed`, plus `[[startup]]`. Each runs
  `<blf> herdr sync-tab-title`. `tab.focused` is not hooked (it always comes with `pane.focused`).
- No `[[actions]]`.
- The manifest is generated from an embedded template at install time, not shipped as a static file.
- `<blf>` path rule (T06 amends T04, R F16): look up `blf` on `PATH` and resolve symlinks only to
  compare it with the running binary. When they match, write the `PATH` entry unresolved.
  Otherwise write the running binary's path as the OS reports it.
- If the binary moves, the fix is to run install again.

### `blf herdr install-plugin` (T04)

- Writes the generated manifest to the folder, then runs `herdr plugin link <folder>`. It always
  links, also on a re-run.
- `herdr` not on `PATH`: exit 1, nothing written.
- `herdr plugin link` fails: print herdr's stderr, exit 1, leave the written manifest. (R F21)
- After linking, inside herdr: one full sync (every tab), because `[[startup]]` does not run on
  link. Outside herdr: skip the sync silently. A failed sync: print the error, exit 1; the plugin
  stays installed. (R F21)
- A re-run exits 0, overwrites the manifest (also a hand-edited one), links again and syncs again.
  (R F04)

### `blf herdr uninstall-plugin` (T04)

- Runs `herdr plugin unlink blf` and deletes the plugin folder.
- herdr's own config/state dirs for the plugin stay. The debug log and marker stay.
- No error when already uninstalled.
- `herdr` not on `PATH`: exit 1, change nothing. (R F27)

### `blf herdr doctor` (T03, T04)

- Read-only. Never renames. One pass/fail line per check. Exit 1 when any check fails.
- Outside herdr (`HERDR_SOCKET_PATH` unset): one failing line, socket checks skipped, exit 1.
  (R F24, F06)
- Checks:
  - Socket reachable; herdr version from `ping` against the tested one (informational).
  - Server version from `ping` at least the manifest's `min_herdr_version` (0.9.3). Fails
    otherwise and says: restart the herdr server. The running server, not the `herdr` binary,
    checks this at link time, so an updated but not restarted server refuses the link. (R F32)
  - Reply fields present for the snapshot (layouts, tabs, panes) and for `workspace.list`.
    (R F15)
  - Environment: `HERDR_ENV`, `HERDR_SOCKET_PATH`, `HERDR_TAB_ID`, `HERDR_PANE_ID`, `VIMRUNTIME`
    (set or not), `blf` path and version, debug log state. Information only, except
    `HERDR_SOCKET_PATH`. (R F24)
  - Plugin linked: `plugin.list {"plugin_id":"blf"}` returns an entry whose `plugin_root` is the
    folder above (an empty array = not linked). (R F19)
  - Manifest: doctor builds the expected manifest and compares it with the file on disk, the `blf`
    path excluded. On a difference the check fails and says: run `blf herdr install-plugin`. This
    replaces the looser "hooks exactly the expected events" check from T03. (R F14)
  - `blf` path in the manifest: fails when it does not exist or is not executable. (R F14)
  - Dry-run table: per tab, tab id, active pane, agent, terminal title, current label, label blf
    would set, `ok` / `would rename`. `would rename` is not a failure.
  - Failed hook runs among the last 10 entries of `herdr plugin log list`. Information only. This
    spawns the herdr CLI, which is fine off the hot path. (R F24)
- `--json` prints the same report as one object, a stable contract (R F25):

  ```json
  {"ok": true,
   "checks": [{"name": "socket", "status": "pass|fail|info", "detail": "..."}],
   "tabs": [{"tab_id": "", "active_pane_id": "", "agent": "", "terminal_title": "",
             "label": "", "new_label": "", "action": "ok|would rename"}],
   "failed_hooks": [{"time": "", "event": "", "detail": ""}]}
  ```
- Not checked: fish hooks (a Go binary cannot see fish functions), `prompt_new_tab_name` (T06),
  duplicate workspace labels (T05).

### Debug log (T03)

- Files in blf's state dir (`$XDG_STATE_HOME/blf`): marker `herdr-debug.on`, log
  `herdr-debug.log`. (R F26)
- `blf herdr debug on` creates the marker and truncates the log, also when already on.
- `blf herdr debug off` removes the marker and leaves the log.
- No argument: prints the state and the log path.
- Each sync does one `stat` on the marker. When present it appends one tab-separated line: RFC 3339
  timestamp, mode, tab, old label, new label, skip reason or error.
- No env var switch. No rotation.

### Workspace commands (T05)

- Both take exactly one `<label>` argument. A label with spaces must be quoted. No trimming. An
  empty label is a usage error (stderr, exit 1).
- Match: the whole label, ignoring case with Unicode case folding (`strings.EqualFold`).
- Duplicates: the first workspace in herdr's order (lowest number) wins. No error.
- `find-workspace <label>`: prints the bare workspace id plus a newline on stdout, exit 0.
  Read-only. No `--json`, no `--create`.
- `goto-workspace <label> [--create [--cwd <path>]]`:
  - Focuses the matching workspace. Already focused: does nothing.
  - Prints nothing on success, exit 0 (focused, already focused, or created).
  - `--create`: when no workspace matches, creates one and focuses it, label exactly as typed.
  - `--cwd <path>`: starting directory of a created workspace. Without it blf passes no cwd and
    herdr picks its default. Ignored when the workspace already exists (no directory check, no
    warning). `--cwd` without `--create` is a usage error (stderr, exit 1).
  - When a workspace would be created and `--cwd` is not an existing directory: stderr, exit 1,
    nothing created. herdr itself gives no error and opens `$HOME`. (R F31)
- Errors:
  - Not found and no `--create`: nothing on stdout, `no workspace labelled "<label>"` on stderr,
    exit 1.
  - Outside herdr (`HERDR_SOCKET_PATH` unset; `HERDR_ENV` is not checked, because a key-bound
    `type = "shell"` command does not get it): `not running inside herdr` on stderr, exit 1.
    (R F06, F29)
  - Any other error (socket down, bad reply): stderr, exit 1.
- Transport: the socket with strict decoding, as in ADR 0010 (R F19):
  - `workspace.list {}`: `workspace_id`, `number`, `label`, `focused`, `active_tab_id`. Array
    order is the UI order and `number` is index + 1, so "first" = first in the array.
  - `workspace.focus {workspace_id}`: a no-op success when already focused.
  - `workspace.create {label, cwd?, focus: true}`. `focus` defaults to false, so blf always sends
    `true`.
- `goto-workspace` is bound in herdr with `type = "shell"`, not as a plugin action (actions take no
  argument). (C, T04)

### Docs, glossary, ADR

- README: documents the fish snippet, the `goto-workspace` binding and the required herdr setting
  `prompt_new_tab_name = false`. It also says the herdr server must run 0.9.3 or later: restart it
  after a herdr update. (T06, R F32)
- Glossary terms already recorded in `GLOSSARY.md`: **terminal title**, **tab title**, **tab
  label**, **workspace label**. Use them exactly. (T03, T05, R F17)
- ADR 0010 is already written. No further ADR. (T05, T06)

### Epics and tickets (T06)

- The implementation epic is `blf-herdr-impl`. No ticket in it edits `~/.dotfiles` files.
- Its last ticket is "publish the dotfiles epic": it writes ticket files into
  `~/.dotfiles/.scratch/blf-herdr-dotfiles/issues/` from the Dotfiles handoff section below. Ticket
  files only, no dotfiles code. It is written last so the tickets match what was really built.
  Done when: three ticket files exist with `status: open`, tickets 2 and 3 are not blocked by
  ticket 1, and their content comes from the Dotfiles handoff section. (R F03)

## Dotfiles handoff

This section is self-contained. It is the source for the `blf-herdr-dotfiles` epic, which lives in
the dotfiles repo's own tracker (`~/.dotfiles/.scratch/`) and is run with gx from that repo. (T06)

### Epic shape

- Three tickets. Tickets 2 and 3 are not blocked by ticket 1.
  1. `tab-title.fish` rewrite (wrappers, herdr hooks, fallback).
  2. `prefix+n` binding swap and deleting `extra/scripts/herdr-goto-workspace`.
  3. `install-plugin` lines in `extra/herdr/install` and `core/setup`.
- Tickets are written `status: open`, not `draft`. The user starts the epic by hand, after
  `brew upgrade blf`.
- The first dotfiles ticket checks that the new blf is installed (`blf herdr doctor --help` exits
  0), and stops with `needs-answer` when it is not. It does not need doctor to pass: a gx agent may
  run outside herdr. (R F11)

### New `tab-title.fish`

- `fish_title` prints `blf tab-title "$argv[1]"`.
- The tmux wrappers stay and call `blf tab-title` too.
- `_pretty_cmd`, `_tab_title` and `string-ellipsis` are deleted (no other users in the dotfiles).
- Accepted cost: about 4 ms per title instead of about 1.6 ms.
- Herdr hooks, defined only when `HERDR_ENV` is set at shell start:
  - `fish_preexec` runs `blf herdr sync-tab-title --cmd "$argv[1]" &; disown`.
  - `fish_prompt` runs the same with `--prompt`.
  - stderr goes to `/dev/null`. Errors are seen only through the debug log.
  - Accepted cost: two `blf` spawns per prompt inside herdr (title + backgrounded sync). No merged
    command.
- Fallback when `blf` is missing or too old:
  - One check at shell start: `blf tab-title >/dev/null 2>&1`.
  - On failure `fish_title` prints the plain directory name and no herdr hooks are defined.
  - No copy of the old fish logic is kept.
  - Accepted cost: one spawn (about 4 ms) per shell start.

### Binding and herdr config

- `prefix+n` becomes `type = "shell"`, `command = "blf herdr goto-workspace notes"`. No `--create`.
  Plain `blf`, not an absolute path.
- `extra/scripts/herdr-goto-workspace` is deleted.
- Behaviour change: the match now ignores case.
- `prompt_new_tab_name = false` is already set in the dotfiles. No change.

### Where `install-plugin` runs

- `blf herdr install-plugin` is added to `extra/herdr/install`, and to `core/setup` after the blf
  install step, so every setup run refreshes the manifest.

### Rollout order

1. Release blf with all new commands; `brew upgrade blf`. Make sure the herdr server runs 0.9.3 or
   later (restart it after a herdr update). (R F32)
2. `blf herdr install-plugin`, then `blf herdr doctor`.
3. Switch `tab-title.fish` (safe at any time thanks to the fallback).
4. Swap the `prefix+n` binding, reload the herdr config, delete the old script.

Rollback: revert the dotfiles commit(s) and run `blf herdr uninstall-plugin`.

## Testing Decisions

- A good test checks external behaviour only (output for a given input), not internals.
- Decided seam: `internal/tabtitle`. (T02)
  - `Title(cmd, pwd, home)` is table-tested against recorded fish `_tab_title` outputs.
  - `Ellipsis(s, n)` is tested alone.
  - No `tab-title` test goes through cobra or env.
- Prior art in the codebase: table tests of pure functions (for example the clean-url and calendar
  tests), and command tests that inject a `deps` struct with fake env lookup, file access and command
  runners.
- Seams for the rest (R F20):
  - A pure label function: pane data in (agent, stripped title, cwd fields, home), label out.
    Table-tested, no socket, no env.
  - Command tests for `sync-tab-title`, `doctor`, `install-plugin`, `uninstall-plugin`, `debug`,
    `find-workspace` and `goto-workspace`, with a fake herdr client, fake env, fake files and a fake
    command runner injected through the existing `deps` struct pattern (`cmd/cmd.go`,
    `internal/kitty/deps.go`).
  - A fake Unix socket server only for the `internal/herdr` client itself: strict decoding,
    optional fields, the 200 ms timeout.

## Out of Scope

From the map (hard boundary):

- Number prefixes on workspaces, tabs and agents.
- Git branch and ssh host in the label.
- A user config file for naming rules.
- Reading Claude transcripts for a task title.
- Respecting hand-renamed tabs.
- Renaming workspaces.
- Shell completion of workspace labels.
- Toggling back to the previous workspace from `goto-workspace`.

Also ruled out by tickets:

- Polling, a daemon, or a socket subscription to `pane.updated` for live agent titles. (C, R F02)
  T01 notes the subscription as the upgrade path if live agent titles are wanted later.
- A state file or `reset` action for hand renames. (C)
- Plugin `[[actions]]`. (T04)
- Hooking `tab.renamed` or `tab.focused`. (T03)
- Automatic manifest refresh after a blf upgrade. (T04)
- Doctor checks for fish hooks, `prompt_new_tab_name` and duplicate workspace labels. (T03, T05, T06)
- `--json` or `--create` on `find-workspace`; substring or fuzzy label matching. (T05)
- A `--dir` flag on `blf tab-title`. (T02)
- A merged "title + sync" command to save a spawn. (T06)
- Rotation of the debug log; an env var to switch it on. (T03)
- Editing `~/.dotfiles` from the `blf-herdr-impl` epic. (T06)

## Further Notes

### Facts the design rests on (T01)

Checked against herdr 0.9.3, protocol 22.

- A plugin is a folder with `herdr-plugin.toml`. Hook commands run with the manifest argv, no shell,
  cwd = plugin root.
- 22 hookable events, none for title or command change.
- Hooks get `HERDR_WORKSPACE_ID` / `HERDR_TAB_ID` / `HERDR_PANE_ID` for the event's subject.
  `pane.closed` has no tab id; startup has none of the three. Verified for
  `pane.agent_detected` and `pane.agent_status_changed` too; there `HERDR_PANE_ID` is the event's
  pane, which may not be the tab's active pane.
- `terminal_title_stripped` is `terminal_title` with whitespace trimmed at both ends and one
  leading status glyph dropped (braille U+2800–U+28FF or one of `·✢✳✶✻✽◐◓◑◒`, only when followed by
  whitespace or nothing). Private-use icons and an inner U+2800 are untouched, so every title
  `blf tab-title` prints comes back identical: fish mode and event mode agree on the label.
- One focus change fires `pane.focused` and `tab.focused` together, so `pane.focused` covers tab
  switches.
- Closing the focused pane of a tab fires only `pane.closed`, no `pane.focused` for the pane that
  takes over.
- Every `tab.rename` fires `tab.renamed`, also for blf's own rename and for a same-label rename.
- `pane.layout` answers "which pane is active in this tab", also for background tabs. The `focused`
  flag on a pane is global focus.
- herdr CLI call: about 4 ms. Same request on the socket: 0.05–0.2 ms.
- Inside fish `preexec` / `prompt` handlers, the title herdr reports is the previous one.
- herdr re-reads the manifest before every hookable event. `[[startup]]` runs only on server start.
- `herdr plugin unlink` takes the plugin id and removes the registry entry only.
- Unverified: that the plugin registry survives `herdr update` (read from source only).

### More facts verified for the review (R)

Checked on a throwaway herdr 0.9.3 server. The user's server was 0.9.1 during the research and has since been restarted on 0.9.3.

- A key-bound `type = "shell"` command runs as `/bin/sh -lc "<command>"` with `HERDR_SOCKET_PATH`,
  `HERDR_BIN_PATH` and `HERDR_ACTIVE_*` set, but not `HERDR_ENV`, `HERDR_TAB_ID` or
  `HERDR_PANE_ID`. `PATH` is the server's, so plain `blf` resolves. Only read from source for 0.9.1.
- `cwd` is the pane shell's directory; `foreground_cwd` is the foreground program's. They differ only
  when that program changed directory itself.
- An unnamed tab's label is its position, computed on read; it changes when the tab moves, with no
  `tab.renamed` event.

### Not decided

Open findings still being settled in the review: see `.scratch/blf-herdr/review-findings.md`.
