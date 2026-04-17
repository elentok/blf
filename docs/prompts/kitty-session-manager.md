# Kitty session manager

## Intro

I want to add a kitty session manager.

Please read https://sw.kovidgoyal.net/kitty/sessions to see how sessions in Kitty work and write a summary to .ai/kitty-sessions.md.

- I'm planning to use the method described in the "Managing multi tab sessions
  in a single OS Window" section by setting `tab_bar_filter session:~` setting
  so kitty only shows tabs from the active session.
- A session is just a `.kitty-session` file that describes where the session is running and how to open it.
- Session files will be stored in ~/.local/share/kitty/sessions/.

## Commands

### blf kitty new-session

- Ask the user for the name (if there's a kitty API for this use that, otherwise open an overlay)
- Create a new `${name}.kitty-session` file in the sessions directory
- Run `kitten @ action goto_session ~/.local/share/kitty/sessions/${name}.kitty-session`
- Write an example in the readme on how to map this to `kitty_mod+e>n` in kitty.conf

### blf kitty sessions

- Open as an overlay over the current window
- Find all of the active sessions and use fzf to let the user pick one
- Run `kitten @ action goto_session ~/.local/share/kitty/sessions/${name}.kitty-session`
- Write an example in the readme on how to map this to `kitty_mod+e>j` in kitty.conf

## Questions

1. How can we find all of the active sessions? we can look in the
   kitty/sessions dir, but what happens when a session ends? is there a kitty
   API to list sessions?
2. Can we show the amount of tabs in a sessions?

Any questions for me before we begin?
