# PRD: `blf copy-ref` — copy file references to the clipboard

## Problem Statement

When working in the terminal I often have a file (a screenshot, a PDF, an export) whose path I
know, and I want to paste it into a GUI app — attach it in Mail, drop it into Slack, place it in
Finder. Today I can copy the *path text* with `blf copy`, but pasting that into a GUI app just
pastes the string, not the file. There is no terminal command that puts the **file itself** on the
clipboard so a GUI paste produces the actual file.

## Solution

A new command, `blf copy-ref <file>...`, that copies one or more **file references** to the system
clipboard. A file reference is a clipboard entry that GUI apps interpret as actual files, so pasting
drops/attaches the files themselves rather than their path text. I can type relative paths, absolute
paths, or `~`-prefixed paths, list several at once, and reference directories as well as files.

```sh
blf copy-ref file1.png /path/to/file2.png ~/Downloads/report.pdf
```

On success it prints a short confirmation (`copied 3 file references to clipboard`) so I know the
otherwise-invisible clipboard write happened.

## User Stories

1. As a terminal user, I want to copy a single file as a file reference, so that I can paste the
   actual file into a GUI app.
2. As a terminal user, I want to copy multiple files in one command, so that I can attach several
   files at once in a GUI app.
3. As a terminal user, I want to pass relative paths, so that I don't have to type the full path for
   files in my current directory.
4. As a terminal user, I want relative paths resolved against my current working directory, so that
   the clipboard gets a path the GUI app can actually open.
5. As a terminal user, I want to use `~` and `~/...` paths, so that I can reference files in my home
   directory without typing the full home path.
6. As a terminal user, I want to copy a directory as a file reference, so that I can drag a folder
   into a GUI app.
7. As a terminal user, I want the command to confirm how many references it copied, so that I have
   feedback for an action I can't otherwise see.
8. As a terminal user, I want correct singular/plural wording in the confirmation, so that the
   output reads naturally for one file vs. many.
9. As a terminal user, I want the command to fail clearly if I name a file that doesn't exist, so
   that I don't silently paste nothing into a GUI app.
10. As a terminal user, I want validation to be all-or-nothing, so that a typo in one of several
    paths never leaves a partial set of files on the clipboard.
11. As a terminal user, I want a clear usage error when I run `copy-ref` with no arguments, so that I
    know the command needs at least one path.
12. As a terminal user, I want filenames containing spaces or quotes to be copied correctly, so that
    awkwardly-named files still paste as the right files.
13. As a macOS user, I want file references to paste into native apps (Mail, Finder, Slack), so that
    the feature works with the apps I actually use.
14. As a Linux/Wayland user, I want file references copied via the system clipboard, so that the
    feature works in my GUI apps too.
15. As a Linux user without the required clipboard tool installed, I want an actionable error telling
    me what to install, so that I can fix the problem myself.
16. As a user on an unsupported OS, I want a clear "unsupported on this OS" error, so that I'm not
    confused by a cryptic failure.
17. As a user who passes a symlink, I want the path I typed copied as-is, so that the reference
    matches my intent rather than the resolved target.
18. As a user reading `blf help`, I want `copy-ref` listed in the usage text, so that I can discover
    it.
19. As a reader of the README and CHANGELOG, I want the new command documented, so that I understand
    what it does and when it was added.

## Implementation Decisions

**Command name & vocabulary.** The command is `copy-ref` (not `copy-files`, which reads like a
filesystem copy of bytes). The canonical term for what lands on the clipboard is a **file
reference** — distinct from `blf copy`, which copies **content** (a string). Both terms are recorded
in `CONTEXT.md`. The informal "file ref" used in the `tmux-targets` feature (a `path:line:col`
location token inside text) is a different concept and is flagged in `CONTEXT.md`; left unchanged.

**Dispatch & wiring.** Add a `case "copy-ref":` to the existing flat `switch` in the command
dispatcher, routing to a new `runCopyRef(args, deps)` orchestrator. The feature reuses the existing
injectable `deps` (`runCommand`, `lookPath`, `userHomeDir`, `stdout`); **no new `deps` fields** are
introduced.

**Platform layer — branch in the command layer, not via build tags ("Option C").** There is no Go
library that copies file references portably (`atotto/clipboard` is text-only), so the command must
shell out to OS-specific tools. To keep the OS-specific construction testable from a single machine,
the OS is a **parameter**, not a hardcoded `runtime.GOOS` call. A pure builder produces the command
to run; the orchestrator passes `runtime.GOOS` in production and tests pass `"darwin"`/`"linux"`/
other explicitly.

- Builder shape (decision, from the grilling session — not final code):
  `buildCopyRefCommand(goos string, absPaths []string) (name string, args []string, err error)`.
- **macOS:** `osascript -e <script>` where the script is
  `set the clipboard to {POSIX file "...", POSIX file "..."}` for multiple files, and the singular
  `set the clipboard to POSIX file "..."` for one file. Each path is embedded in a double-quoted
  AppleScript literal with `\` and `"` backslash-escaped. Because execution goes through the
  `runCommand` dep (`exec.Command`, no shell), only AppleScript-level escaping is needed — no shell
  quoting.
- **Linux (Wayland):** `wl-copy --type text/uri-list <payload>` where `<payload>` is a **single**
  trailing argument containing the newline-joined `file://` URIs. URIs are built with `net/url` so
  paths are correctly percent-encoded (spaces → `%20`, etc.). Passing the payload as an argument
  (rather than via stdin) keeps the existing `runCommand` dep signature unchanged.
- **Unsupported OS:** the builder returns an error `unsupported on this OS (<goos>)` for any `goos`
  that is neither `darwin` nor `linux`.

**Linux tool preflight.** Before invoking on Linux, check `wl-copy` is on `PATH` via the `lookPath`
dep. If missing, fail with `copy-ref: wl-copy not found (install wl-clipboard)`. No preflight on
macOS (`osascript` is always present; its absence indicates a broken OS, not a fixable user error).
X11 (`xclip`/`xsel`) support is out of scope for v1.

**Path normalization pipeline (per argument, in order):**
1. **Tilde expansion** — expand `~` (exactly) and `~/...` only, using the `userHomeDir` dep. Do
   **not** expand `~user` (other users' homes) or a `~` that appears mid-path. Leave all other paths
   untouched.
2. **Absolute resolution** — `filepath.Abs` against the current working directory.
3. **Existence validation** — `os.Stat` success. Accepts directories as well as files; follows
   symlinks for the existence check, so a dangling symlink correctly fails.

**Symlinks** are copied **as-is** (the typed-then-absolutized path), not resolved to their target.

**Atomicity.** All arguments are expanded, resolved, and validated **before** any clipboard write.
If any path is missing, the command errors (`copy-ref: file not found: <path>`) and writes nothing —
the clipboard is never left in a partial state.

**Argument rules.** At least one path is required. With zero arguments, print
`usage: blf copy-ref <file>...` and return an error (matching the existing `copy` command's pattern).

**Success output.** On success, print `copied N file reference[s] to clipboard` to **stdout** (via
the `stdout` dep), with correct singular (`reference`) / plural (`references`) wording, count only —
no path list. Errors are returned as `error` and surfaced by the existing top-level error handling.
The macOS `copy`/`open` commands are silent on success; `copy-ref` deliberately differs because a
clipboard write is invisible and benefits from confirmation.

**Documentation surface.** Update the dispatcher's `printUsage` help text, the README command list,
and `CHANGELOG.md`.

**ADR.** None warranted — "Option C" is a local, easily-reversible structuring choice, not a
hard-to-reverse / surprising / lock-in decision.

## Testing Decisions

Good tests here assert **external behavior** through stable seams, not internal wiring: what command
gets built for a given OS and set of paths, and how the orchestrator behaves given fake deps. The
existing `cmd` test suite (`cmd_test.go`) is the prior art — it drives the dispatcher through
`execute(args, deps)` with fake function fields (`copyText`, `openURL`, `runCommand`, `lookPath`,
`fileExists`, `userHomeDir`, `stdout`/`stderr` builders) and asserts on captured arguments and
output. New tests follow that style. All three modules are tested (per developer decision):

1. **`buildCopyRefCommand` (the deep module)** — table-driven, runnable on any OS since the OS is a
   parameter:
   - macOS single file → `osascript -e 'set the clipboard to POSIX file "..."'`.
   - macOS multiple files → the `{POSIX file "...", ...}` list form.
   - macOS path containing a space, a `"`, and a `\` → correct backslash escaping.
   - Linux single/multiple → `wl-copy --type text/uri-list` with one trailing arg of newline-joined
     `file://` URIs.
   - Linux path with a space/special chars → correct `net/url` percent-encoding.
   - Unsupported `goos` → returns the "unsupported on this OS" error.

2. **`expandTilde` helper** — using a fake home-dir function:
   - `~` alone → home dir.
   - `~/sub/file` → joined under home.
   - `~user/...` → left unchanged (not expanded).
   - `/a/~/b` (mid-path tilde) → left unchanged.
   - a plain relative/absolute path → unchanged.

3. **`runCopyRef` orchestrator** — through `execute`/`deps` fakes:
   - Zero args → usage error, no `runCommand` call.
   - One missing path → `file not found` error, **no** `runCommand` call (atomicity).
   - Multiple paths where one is missing → error, no `runCommand` call.
   - Linux with `wl-copy` absent (fake `lookPath` returns error) → actionable install error.
   - Success path → `runCommand` invoked with the expected name/args, and stdout shows the correct
     singular/plural confirmation with the right count.
   - Tilde input is expanded before validation/build (fake `userHomeDir`).

The actual `exec` of `osascript`/`wl-copy` (whether a file truly lands on the clipboard) is verified
**manually**, not in unit tests — it sits behind the `runCommand` dep by design.

## Out of Scope

- X11 clipboard support (`xclip`/`xsel`); Wayland (`wl-copy`) only for v1.
- Windows or any non-darwin/non-linux clipboard backend (explicit "unsupported" error instead).
- `~user` (other users') home expansion.
- Resolving symlinks to their targets.
- Reading paths from stdin, or a "copy current directory" default.
- A verbose/`-v` flag that lists the copied paths.
- Copying file **contents/bytes** (this command copies references, not data).
- Stdin-piped payload to `wl-copy` (the argument form is used instead).

## Further Notes

- The split between a pure, OS-parameterized builder and a thin orchestrator that performs the
  side-effecting `exec` is what makes both OS code paths testable from a single (macOS) developer
  machine — the main motivation for choosing the command-layer branch over build-tagged platform
  files.
- `wl-copy` accepts the clipboard payload as positional arguments and treats a single argument
  containing newlines as the literal payload, which is why the whole URI list is passed as one arg
  to preserve the `\n` separators.
- Relevant repo conventions captured during design: `CONTEXT.md` (vocabulary), `.ai/workflow.md`
  (plan/verify expectations), `.ai/commit-message.md` (scoped, imperative, lowercase commit
  subjects).
