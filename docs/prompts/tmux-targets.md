# Tmux targets feature

I want to implement the `blf tmux-targets` command that will be used as a tmux bindings:

```
bind-key t run 'blf tmux-targets'
```

It should take the entire viewport of the current tmux pane and open a tmux popup
in the same size with the same content but with everything in one color except for
"targets" (strings-of-interest), e.g.

- URLs (https://hello.com, hello.com, hello.com/world)
- File paths
- Commit hashes (short and long)
- Custom patterns (e.g. id of a jira ticket) - we'll deal with this later, just
  keep it in mind

Only one target can be highlighted at the same time, the user can jump between
them with vim bindings (both arrows and hjkl, gg to the first, G to the last).

- Pressing "y" will yank and exit
- Pressing "enter" or "o" will open it (if it's openable like a URL) and exit
- Pressing "q" will exit

Before you start:

1. Do you have any questions?
2. Can you think of other patterns that could be useful?

## Decisions

- Command name is `blf tmux-targets` (`bfl` was a typo).
- If user presses `enter`/`o` on a non-openable target, do a no-op and show a `tmux display-message`.
- Scope is visible viewport only (not scrollback).
- File targets should include absolute and relative paths, including `path:line` and `path:line:col`.
- Include these target patterns in v1:
  - URLs (`https://...`, `http://...`, bare domains, domain paths)
  - File paths
  - Commit hashes (short and long)
  - `file:line` and `file:line:col`
  - Email addresses
  - IP addresses and `host:port`
  - `#123` style issue references
  - UUIDs
  - Branch/tag-like tokens
  - PR/issue URLs
