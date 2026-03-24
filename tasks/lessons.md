# Lessons

## Command Design

When a helper flow could also be broadly useful, prefer first-class user commands over hidden internal flags.

- Applied here by adding `blf open <url>` and `blf copy <text>` and reusing them from `tmux-links` menu callbacks.
- This keeps behavior discoverable and easier to script directly.
