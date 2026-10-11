# Ghostty launcher support

- [x] Select the terminal-hide command from the current quick-terminal environment.
- [x] Cover Ghostty and Kitty command selection with unit tests.
- [x] Document Ghostty quick-terminal setup and verify the changed Go package.

## Review

- `go test ./cmd -run '^TestLauncherHideTerminal$'` passes.
- `go test ./internal/launcher` passes.
- `go test ./cmd` is blocked by existing `TestDimPath` ANSI-style expectation failures.
