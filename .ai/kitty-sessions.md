# Kitty Sessions Notes

When reading this file please explicitly mention that you did.

Based on the official Kitty sessions docs at `https://sw.kovidgoyal.net/kitty/sessions/` and the `kitten @ ls` help in Kitty `0.46.2`.

## Core model

- A Kitty session is a text file that describes tabs, windows, layouts, working directories, and commands to launch.
- Sessions can be started with `kitty --session <path>` or switched at runtime with the `goto_session` action.
- `goto_session` can target a specific file, browse globally known sessions, browse a directory of session files, or jump backward in session history.
- When `goto_session` is pointed at a directory, Kitty scans for files ending in `.kitty-session`, `.kitty_session`, or `.session`.

## Session file behavior

- Relative paths in `cd` are resolved relative to the directory containing the session file.
- `new_tab [title]`, `layout`, `cd`, and `launch` are the main primitives we need for hand-written session files.
- `launch` inside a session file cannot create tabs or OS windows; tabs/OS windows must be declared with session keywords.
- `save_as_session` can capture the current Kitty state back into a session file, including `--match=session:.` to save only the active session.

## Session membership

- Kitty has first-class session membership for both windows and tabs.
- Remote-control matching supports `session` for windows and tabs.
- Special session match values:
- `session:.` means the currently active session.
- `session:~` means the active session, or the last active session when the current window/tab is not in a session.
- `session:^$` matches windows/tabs that do not belong to any session.
- New tabs/windows do not automatically join the active session unless the user uses `new_window_with_cwd`, `new_tab_with_cwd`, `new_os_window_with_cwd`, or `launch --add-to-session`.

## Multi-session single-window setup

- The docs explicitly support managing multiple sessions in one OS window.
- The recommended config is `tab_bar_filter session:~` or `tab_bar_filter session:^$`.
- With that filter, the tab bar only shows tabs from the active session plus tabs with no session; when no session is active, Kitty shows the most recently active session to preserve context.

## What Kitty exposes for discovery

- `kitten @ ls` can filter by `--match session:...` and `--match-tab session:...`.
- `kitten @ ls` can return JSON or session-format output.
- The documented `ls` JSON structure includes OS windows, tabs, and windows, but the help text does not document a dedicated "list sessions" API or a session-name field in the JSON response.
- Practical implication: if we want "active sessions" by file, the safe approach is to scan the configured sessions directory and check each session file against Kitty using session-based matching.

## Answers to the prompt questions

1. Finding active sessions:
   There does not appear to be a dedicated documented API that returns "all active sessions" directly. The reliable documented primitives are session-aware matching in `kitten @ ls` and scanning a directory of session files.

2. Showing tab counts:
   Yes. We can count matched tabs for a session by querying Kitty with a session-aware tab match and counting the returned tabs in the JSON response.
