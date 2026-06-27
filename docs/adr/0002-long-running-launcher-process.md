# ADR 0002: Launcher is a single long-running process inside Kitty's quick terminal

**Status**: Accepted

## Context

`blf launcher` is a launcher shown via Kitty's quick-access terminal
(`kitten quick-access-terminal --instance-group quick`, bound to Cmd+2). The defining
requirement is that it open **blazingly fast** — pressing Cmd+2 must feel instant.

The obvious model is to spawn `blf launcher` fresh each time the quick terminal opens.
That pays Go runtime startup, config parsing, app-index loading, and currency-cache reads
on every single open, and (since `quick-access-terminal` startup-program wiring is awkward)
didn't even work cleanly in practice.

## Decision

The launcher is **one long-lived process** that lives inside the persistent `quick`
instance-group terminal. Cmd+2 only **toggles the terminal's visibility** (same
`--instance-group quick` invocation acts as the toggle); the process never exits between
uses. The first launch is started manually; thereafter it stays resident.

### Consequences (the shape this forces)

- **Never exit on action.** After running a result (launch app / run script / copy a calc),
  the launcher resets to empty input and **hides the quick terminal** (re-invoking
  `kitten quick-access-terminal --instance-group quick`) rather than quitting.
- **In-process caches.** App index, currency rates, and history live in memory and are
  loaded once; the hot path touches no filesystem or network.
- **Self-refresh instead of cron.** Because the process is always up, freshness is managed
  in-process via TTL ticks (`tea.Tick`) + an on-show staleness check + manual Ctrl+R —
  no crontab/launchd entry is needed for the app index or currency rates.
- **Reset-on-dismiss.** Esc/focus-loss clears the input so the next open is always clean.

## Alternatives considered

- **Spawn `blf launcher` per open.** Rejected — pays startup + cache-load cost on every Cmd+2,
  defeating the instant-open premise, and `quick-access-terminal` startup-program wiring was
  unreliable.
- **Keep a shell in the quick terminal, type to start.** Rejected — extra keystrokes, not
  launcher-like.
- **External cron/launchd to refresh caches.** Rejected as redundant once the process is
  always resident; the launcher refreshes its own caches.
