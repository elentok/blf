# ADR 0006: Launcher history records the picked item and direct-fires on recall

**Status**: Accepted

## Context

**Launcher history** recorded the raw query text you executed (e.g. `"kit"`), not what it
resolved to. Recalling a row populated the input with that text and recomputed results —
deliberately never re-firing the original action, per ADR-era design in `CONTEXT.md`.

In practice this made history rows low-value for `ActionLaunch`/`ActionRun`/`ActionOpen`:
seeing `"kit"` in the history list doesn't tell you it launched Kitty, and two different
abbreviations for the same app (`"kit"`, `"kitty"`) produced two separate, redundant
entries instead of coalescing into one.

## Decision

For `ActionLaunch`, `ActionRun`, and `ActionOpen`, history now stores the **picked
result's label and action** (e.g. `{label: "Kitty", type: launch, target:
/Applications/kitty.app}`) instead of the query text. `ActionCopy` (calc/unit/currency)
is unchanged — those results have no stable identity independent of the query, so they
keep recording the query text.

Selecting a history row from the empty-input list now **direct-fires** the stored action
on Enter (e.g. picking "Kitty" launches Kitty immediately), reversing the previous
"never blindly re-fire" rule for this one entry point. Ctrl+R/Ctrl+F (shell-history-style
cycling into the input for further editing) keep the old populate-and-recompute behavior
unchanged — they are a text-recall shortcut, not an execution shortcut.

Ctrl+X (delete entry) matches on the stored `(action type, target)` pair rather than
label text, so it can't misfire if two entries ever share a label.

### On-disk format

The history file moves from plain-text-one-query-per-line to JSON-lines (one JSON object
per entry: label + action type + target). A flat string can't represent an entry with a
label distinct from its re-fire target. Old-format files fail to parse and `Load()` falls
back to empty history, same as a missing file — the 30-entry cap makes this a disposable
convenience cache, not something worth writing one-time migration code for.

## Alternatives considered

**Keep populate-and-recompute for history rows too, only change the displayed label.**
Rejected — the whole point of the change is that picking "Kitty" from history should be
as fast as picking it from a fresh search; forcing a second keystroke through recompute
defeats that.

**Apply item-based history + direct-fire to `ActionCopy` too.** Rejected — computed
values (e.g. `10+20` → `30`) have no identity separate from the query; storing `"= 30"`
as the entry loses the ability to recall *how* you got there.

**Best-effort migrate old plain-text history entries into the new format.** Rejected —
they'd have no action to re-fire (only a query string), so they'd need a distinct
partial-entry shape just to avoid data loss on a 30-entry cache. Not worth the code.
