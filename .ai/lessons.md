# Lessons Learned

## Go

### Avoid import cycles by putting shared types in the lower-level package

When two packages need to share types, put those types in the package that is lower in the dependency tree. In this project, `app` imports `ui`, so types needed by both (like `EditMode`, field constants) live in `ui`. `app` then re-exports them as type aliases.

**Why:** Go's import cycle detection is strict. If `ui/editor.go` imports `app` for a type, and `app/view.go` imports `ui`, you get a cycle and the build fails.

**How to apply:** Before adding an import, ask which package is the "leaf" (imported by others). Put shared types there.

---

## Lip Gloss layout

### Never manually pad ANSI-styled strings — use Width() instead

When right-aligning or filling a row that contains Lip Gloss-rendered strings, do NOT calculate padding by measuring string lengths. ANSI escape codes inflate byte length invisibly, so `len(styledString)` is far larger than the visual width. Any space calculation based on it will be wrong.

**Why:** In `ui/outputs.go`, manually computing `strings.Repeat(" ", spaces)` based on string lengths caused the key hint `[yr]` to overflow to the next line because the styled label (`Width(6)`) contributed more bytes than its 3-char raw value.

**How to apply:** Set `Width(n)` on the value column (or any fill column) and let Lip Gloss pad/clip to exactly `n` visual chars. The line then has a predictable visual width without any manual arithmetic.

---

### Pass the correct content width to renderers — account for panel borders

A Lip Gloss panel with `.Border(NormalBorder()).Width(n)` has a **content area** of `n` chars. The border itself adds 1 col per side (2 total), so the **total rendered width** is `n + 2`.

**Why:** `RenderOutputs` was receiving `innerW` but rendered inside a panel with `.Width(innerW - 2)`. Lines were 2 chars too wide for the content area → Lip Gloss wrapped them, putting `[yr]` on its own line.

**How to apply:** When calling a renderer, pass the panel's `.Width(n)` value, not the outer width. Trace: `outerW → innerW = outerW - 2 → panelContentW = innerW - 2 → pass panelContentW to renderer`.

---

### Derive panel heights from actual fixed costs — never use a magic constant

Compute available height for variable-size panels by subtracting the exact fixed heights of all other regions:

```
fixedH = outer_border(2) + header(3) + outputs_panel(5) + footer(1) = 11
splitPanelH = m.height - fixedH
splitContentH = splitPanelH - 2  // border takes 2 rows
```

Also pin the outer box height explicitly with `.Height(m.height - 2)` so it always fills the terminal even if content falls short.

**Why:** The original `usedH = headerH + outputsH + footerH + 8` used a guessed constant of 8 for "borders + padding". The actual border cost was different, leaving the box 3–4 rows too short.

**How to apply:** For each layout region, count its rows explicitly (content + border). Sum them. Subtract from `m.height`. Never guess.
