# Tmux targets search

I want to add a search functionality to tmux-targets where "/" goes into search
mode:

- When you type it filters the available targets (and jumps to the
  first match)
- When you press <esc> it resets the search
- When you press <enter> it "locks" the search (the highlighted are only the
  ones that match the filter)

Before we start there are some questions:

1. Filtering should be fuzzy, please research what's the best way to do that with go,
   is there a library for that?
2. We need to indicate somehow that we're in "filtered" state,
   do you have a recommendation? (perhaps some sort of dynamic positioning
   that won't cover existing targets).
3. Any questions for me?
