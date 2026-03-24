I have a lot of custom scripts in my dotfiles, I want to convert some of them
to Go so they'll run faster.

I want to create a Go CLI named "blf" (short for "Blazingly Fast"),
use ~/dev/colr or ~/dev/gx/main as an example.

It should support multiple commands, the first one to implement is "blf tmux-links <open|copy>":

- Capture the current tmux pane's last 10000 lines with:
  `tmux capture-pane -pS "-10000"`
- Extract the URLs from it
- Open a menu using the `tmux display-menu` command
- When the user picks a URL it will either open or copy it to the clipboard
  - Research what's the best cross-platform way to open/copy with Go (colr uses
    github.com/atotto/clipboard for copying)

Let's start by planning how to implement this in docs/initial-plan.md
