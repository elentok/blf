# PRD: blf launcher

A launcher in the terminal, shown via Kitty's quick-access terminal.

Related: `CONTEXT.md` (glossary), `docs/adr/0002-long-running-launcher-process.md`,
`docs/adr/0003-currency-rate-source.md`, `docs/plans/launcher.md`.

## Problem Statement

I want a launcher in the terminal. When I press Cmd+2 (already bound to Kitty's
quick-access terminal), I want a single box where I can type anything — a calculation, a unit or
currency conversion, an app name, or a named script — and get the right answer or action instantly.
Today I have to context-switch between a calculator, a converter, Spotlight, and a shell to do these
things, and none of them live where my terminal is. It has to feel blazingly fast: the moment the
quick terminal appears, the launcher is already there and responsive.

## Solution

A single long-running `blf launcher` TUI lives inside the persistent `quick` instance-group
terminal; Cmd+2 just toggles it into view. It shows one input box over a single ranked result list
fed by multiple **launcher sources**:

- Type a **math expression** (`1234*2`, `sqrt(2)*pi`, `200+10%`) → the result appears immediately,
  comma-formatted; Enter copies it.
- Type a **`<number><unit>`** (`10cm`, `123$`) → conversions to every other unit in that group
  appear immediately; Enter copies the selected one.
- Type a **name** → fuzzy matches against installed applications and configured scripts, ranked
  into one list with matched characters highlighted; Enter launches the app or runs the script.

Computational input suppresses the fuzzy list; name-like input shows it. When the box is empty, it
shows recent history so I can recall a previous query without typing. The launcher never exits — it
resets and hides after an action, staying warm for the next Cmd+2.

## User Stories

1. As a launcher user, I want the launcher already running when I press Cmd+2, so that it opens
   instantly with no spawn or load delay.
2. As a launcher user, I want Cmd+2 to toggle the quick terminal's visibility, so that one keystroke
   both summons and dismisses it.
3. As a launcher user, I want the input cleared when I dismiss (Esc or focus-loss), so that I never
   return to a stale half-typed query.
4. As a launcher user, I want the launcher to hide itself after I run a script, so that I don't have
   to manually dismiss it when focus didn't move to another app.
5. As a launcher user, I want a single ranked result list rather than mode switches, so that I can
   just type and let the launcher figure out what I mean.
6. As a launcher user, I want a confident math/unit query to hide app/script results, so that
   `1+2` shows me the answer cleanly without noise.
7. As a launcher user, I want a bare small number like `1` to search apps (e.g. 1Password), so that
   numeric-leading names are still findable.
8. As a launcher user, I want a large bare number like `1000000` to also show a comma-formatted
   row, so that I can quickly read/copy a grouped number.
9. As a calculator user, I want results to update as I type, so that I see the answer without
   pressing enter.
10. As a calculator user, I want `+ - * / % ^` with `^` meaning power and `/` meaning float
    division, so that the math behaves like a calculator, not a programming language.
11. As a calculator user, I want constants (`pi`, `e`, `tau`, `phi`), so that I can use them in
    expressions.
12. As a calculator user, I want functions (`sqrt`, `cbrt`, `abs`, `round`, `floor`, `ceil`, `ln`,
    `log`, `log2`, `exp`, `pow`, `min`, `max`), so that I can do richer calculations.
13. As a calculator user, I want trig functions (`sin`, `cos`, `tan`, `asin`, `acos`, `atan`) to
    work in degrees by default, with a `rad()` escape, so that `sin(30)` gives 0.5.
14. As a calculator user, I want inverse trig to return degrees, so that `asin(0.5)` gives 30.
15. As a calculator user, I want percent math (`200+10%`=220, `200-10%`=180, `200*10%`=20,
    `10%`=0.1), so that everyday percentage calculations just work.
16. As a calculator user, I want results comma-formatted (`1,234,567`), so that large numbers are
    readable.
17. As a converter user, I want to type `10cm` and see mm/m/km/inch/ft/mile instantly, so that I
    can convert without choosing a target.
18. As a converter user, I want currency conversions (`123$`) against fresh rates, so that I can
    convert money.
19. As a converter user, I want temperature conversions (°C/°F/K) to be correct despite the
    offset, so that affine units convert accurately.
20. As a converter user, I want a sensible default when a symbol is ambiguous (`$`→USD, `m`→meters,
    `min`→minutes), so that common inputs resolve predictably.
21. As a converter user, I want to define my own unit groups in config, so that I can add units the
    built-ins don't cover.
22. As a converter user, I want currency rates cached for ~12h and refreshed in the background, so
    that conversions are instant and rates stay reasonably fresh.
23. As a converter user, I want stale cached rates used when the network is down, so that
    conversion still works offline.
24. As an app launcher, I want to fuzzy-find installed applications on mac and linux, so that I can
    launch anything by typing part of its name.
25. As an app launcher, I want matched characters highlighted, so that I can see why a result
    matched.
26. As an app launcher, I want Enter to launch the selected app and the launcher to get out of the
    way, so that launching is one gesture.
27. As an app launcher, I want a prebuilt app index so that searching never scans the filesystem on
    the hot path.
28. As an app launcher, I want the index refreshed automatically in the background and on demand
    (Ctrl+R), so that newly installed apps appear without restarting the launcher.
29. As an app launcher, I want a `blf launcher reindex` command, so that I can build/refresh the
    index manually or on first run.
30. As a power user, I want to define named scripts (bash or osascript) in config, so that I can
    run common actions like Spotify play/pause by name.
31. As a power user, I want each script to declare its platform, so that mac scripts are ignored on
    linux and vice versa.
32. As a power user, I want some scripts to call blf itself (e.g. `cleanurl` → `blf clean-url
    --clipboard`), so that the launcher composes with the rest of blf.
33. As a power user, I want scripts to run only on Enter, so that I don't trigger actions while
    typing.
34. As a power user, I want a script's output handling configurable (ignore / show / copy), so that
    fire-and-forget and value-producing scripts both work.
35. As a power user, I want a failing script to keep the launcher open and show its error, so that
    failures aren't silently swallowed.
36. As a power user, I want scripts to run without freezing the UI, so that a slow script doesn't
    lock the launcher.
37. As a launcher user, I want one ranked list where an exact or prefix match wins regardless of
    source, so that the thing I clearly meant is on top.
38. As a launcher user, I want scripts weighted slightly above apps (configurable), so that my
    hand-curated verbs aren't buried under dozens of apps.
39. As a launcher user, I want Enter on a calc/unit result to copy it to the clipboard, so that I
    can paste the answer elsewhere.
40. As a launcher user, I want every Enter action recorded in history, so that I can recall what I
    did.
41. As a launcher user, I want Ctrl+P/Ctrl+N to recall previous/next history entries, so that I can
    re-run or edit a past query.
42. As a launcher user, I want recalling a history item to populate the input and recompute (not
    blindly re-fire), so that I can tweak it and avoid accidental side-effects.
43. As a launcher user, I want Ctrl+S to save the current input to history without acting on it, so
    that I can park a half-built expression and come back to it.
44. As a launcher user, I want a transient "saved" confirmation on Ctrl+S, so that I know it worked
    when nothing else visibly happens.
45. As a launcher user, I want recent history shown when the input is empty, so that the launcher is
    useful the instant it appears.
46. As a launcher user, I want history to survive restarts and dedup repeated entries, so that it
    stays useful and tidy.
47. As a launcher user, I want a clean full-screen layout (outer border, input on top, separator,
    results below) with nerdfont icons, so that it looks and feels like a real launcher.
48. As a launcher user, I want a help footer hidden by default and toggled with `?`, so that the
    everyday view stays clean but bindings are discoverable.
49. As a launcher user, I want the launcher to still open with built-in defaults if my config is
    malformed, plus a visible non-blocking error, so that I'm never locked out.
50. As a launcher user, I want built-in scripts and unit groups out of the box, with my config
    adding/overriding them, so that it's useful before I configure anything.

## Implementation Decisions

### Architecture

- **Long-running single-instance process** (ADR 0002). The launcher never exits on action; it
  resets the input and hides the quick terminal by re-invoking
  `kitten quick-access-terminal --instance-group quick`. Freshness (app index, currency rates) is
  managed in-process via TTL ticks + on-show staleness checks — no cron.
- **Provider model.** Each launcher source implements a synchronous `Provider.Query(input) []Result`.
  `Result` carries title, subtitle, icon role, match-highlight ranges, source/weight, and an
  `Action` (launch app / run script / copy value). Async edges (app scan, currency fetch) load data
  via `tea.Cmd` into in-memory state that providers read synchronously. No per-keystroke debounce —
  everything except app-scan/network is instant.
- **Routing (`router` module).** `Classify(input)` distinguishes a *computational query* (math
  expression containing an operator/function, or `<number><unit>`) from a *name-like query*. A
  confident computational parse suppresses the fuzzy app/script list. A bare number is name-like
  (falls through to fuzzy); a large bare number (≥4 digits) additionally contributes a
  comma-formatted row.
- **Ranking (`router` module).** Single ranked list: exact > prefix > source-weight > raw
  `sahilm/fuzzy` score. Source weights configurable (default scripts slightly above apps).

### Modules

- **`calc`** (deep, pure) — `Evaluate(expr string) (float64, error)`. Hand-rolled tokenizer +
  Pratt/shunting-yard evaluator (ADR-style rationale: calculator semantics differ from a general
  expr lib — `^`=power, `/`=float, degrees-default trig). Operators `+ - * / % ^`, unary minus,
  constants, function table, percent semantics (`+`/`-` nodes special-case a percent right operand;
  `*`/`/` and bare treat `%` as ÷100). Comma formatting helper (may borrow `sum.go`'s number
  formatting).
- **`units`** (deep, pure) — unit-group registry where each unit has a factor relative to the
  group's base plus an optional `offset` for affine units (temperature). `Convert(value, unit)
  []Conversion` routes through the base. `<number><ws?><unit>` parser, lowercased token → one
  `(group, unit)` via a fixed precedence table (`$`→USD, `m`→meters, `min`→minutes). Currency is
  injected as just another group whose factors come from the rate cache. User `unit_group`s merged
  from config.
- **`currency`** (deep, IO behind interface) — `Rates() (map[string]float64, error)`. open.er-api
  primary, fawazahmed0 fallback (ADR 0003). USD base + locally-derived cross-rates. Cache to
  `~/.cache/blf/currency.json`; TTL driven by open.er-api `time_next_update_unix` else 12h; stale
  cache on failure. Fetcher and clock injected for testability.
- **`history`** (deep) — load/append/dedup(move-to-recent)/cap(~500)/persist to
  `~/.local/state/blf/launcher-history`. Records executed (Enter) or Ctrl+S-saved queries only.
- **`apps`** (platform glue) — `Reindex()`, `Load()`, `Launch(app)`. Index persisted to
  `~/.cache/blf/apps.json` (name, path, platform); built inline if missing on startup; refreshed via
  in-process TTL tick + on-show mtime check + Ctrl+R, all calling `apps.Reindex()` directly (no
  shelling out). Launch: mac `open -a <path>`; linux `gio launch <desktop>` (`.desktop`-based).
  `blf launcher reindex` CLI command for bootstrap/manual. Built from `/Applications`,
  `~/Applications`, `/System/Applications` (mac) and XDG `.desktop` dirs (linux). Uses existing
  `internal/platform` for OS branching.
- **`scripts`** (glue) — model from config (`name`, `icon`, `type` bash|osascript, `platform`
  mac|linux|"", `body`, `output` ignore|show|clipboard default ignore). Platform filtering. Async
  `Run(script)` via `tea.Cmd` with a brief "running…" state. Errors → stay open, show stderr row.
  Built-in defaults in-binary (`playpause`, `cleanurl`, …), config adds/overrides.
- **`config`** (shared) — TOML loader at `~/.config/blf/config.toml`, `[launcher]` section,
  defaults merge. Malformed → defaults + non-blocking error row. New shared loader since blf has no
  config infra today.
- **`launcher` model** (shallow glue) — bubbletea v2 orchestrator wiring providers → ranked list →
  UI. Owns key bindings: Up/Down select, Enter act, Esc dismiss+reset, Ctrl+P/N history, Ctrl+S
  save, Ctrl+R reindex, `?` help. Empty input shows recent history.
- **`styles` / `icons`** (launcher-local UI) — named lipgloss vars (no blf-wide design system yet);
  semantic icon map keyed by role (app, script, calc, unit/currency, error/loading) with nerdfont +
  ASCII fallback; scripts may name an icon.

### UI

- Full-screen within the quick terminal: outer rounded border, top `bubbles/textinput`, horizontal
  separator rule, scrollable results viewport (cap ~200, following the `internal/targets` viewport
  pattern). Result row: `[icon] Title  subtitle  [source hint]`. Calc/unit rows show the **result as
  the title**, expression as subtitle, one row per target unit. Matched characters highlighted via
  `sahilm/fuzzy` positions. Help footer hidden by default, `?` toggles a binding-derived bar.

### Enter semantics

- Enter on the selected result performs its action — launch app, run script, or copy computed value
  to clipboard — records a history entry, and on success hides the quick terminal. Every source has
  a meaningful Enter.

### Paths

- Config `~/.config/blf/config.toml`; cache `~/.cache/blf/{apps.json,currency.json}`; state
  `~/.local/state/blf/launcher-history`.

## Testing Decisions

Good tests here assert **external behavior through a module's public interface**, not internal
representation: feed an input, assert the computed/converted/ranked output. The four deep modules
are pure or have their IO behind injected boundaries, so they test without a running TUI.

- **`calc`** — operator precedence, float division, power (`^`), unary minus, constants,
  degrees-default trig + `rad()` escape, inverse-trig-returns-degrees, the four percent cases, and
  comma formatting. Pure; prior art `cmd/sum_test.go`.
- **`units` + `currency`** — linear conversion, affine (temperature) conversion through the base,
  currency cross-rates derived from a USD base, symbol-collision resolution (`$`/`m`/`min`), the
  `<number><unit>` parser, and cache TTL/stale-fallback behavior with an **injected fetcher + clock**
  (no real network). Pure logic with mocked boundaries.
- **`router`** — `Classify` (computational vs name-like, incl. bare-number and large-bare-number
  edge cases) and `Rank` ordering (exact > prefix > source-weight > fuzzy score). Pure.
- **`history`** — dedup/move-to-recent, cap enforcement, and that recall **populates** rather than
  re-fires. Pure list logic with a thin file boundary.

Lighter coverage (not in the deep-test set): `apps` launch-command construction and `scripts` exec
dispatch can have focused tests on the command they build per platform/type, following the
deps-injection pattern used across `cmd/`. The `launcher` bubbletea model can get a smoke/model test
in the style of `internal/targets/model_test.go` if it proves valuable, but the logic-heavy behavior
lives in the deep modules by design.

## Out of Scope

- A shared blf-wide `ui`/design-system package (launcher-local styling for now; extract later if a
  second TUI needs it).
- Crypto/metals currency rates (fawazahmed0 covers them, but not needed).
- Per-symbol unit disambiguation beyond the `$`/`m`/`min` defaults.
- Periodic app rescans beyond the TTL tick / on-show check / Ctrl+R.
- Rendering real application icons (terminal shows generic nerdfont role icons).
- A distinct "waiting/blocked" state or any agent-status concepts (unrelated feature).
- Automatic first-time start of the launcher process (started manually the first time; Cmd+2
  toggles thereafter).

## Further Notes

- The first launcher instance is started manually; subsequent Cmd+2 presses only toggle visibility.
- open.er-api requests attribution on its free tier — honor it where appropriate.
- The exact open.er-api / fawazahmed0 endpoint paths should be verified at implementation time
  (community URLs have moved historically).
- Build order recommendation: Foundation (`config`, `router`, provider interface) + UI shell +
  `calc` first yields a working calculator launcher end-to-end; other providers slot into the same
  interface afterward. Prefer vertical slices for feedback.
