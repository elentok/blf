package targets

// CondenseViewport collapses lines that contain no targets, keeping `context`
// lines of surrounding context around each target line and inserting "..."
// separators in place of collapsed ranges.
func CondenseViewport(lines []string, tgts []Target, context int) ([]string, []Target) {
	if len(lines) == 0 || len(tgts) == 0 {
		return lines, tgts
	}
	if context < 0 {
		context = 0
	}

	keep := make([]bool, len(lines))
	for _, t := range tgts {
		start := t.Line - context
		if start < 0 {
			start = 0
		}
		end := t.Line + context
		if end >= len(lines) {
			end = len(lines) - 1
		}
		for i := start; i <= end; i++ {
			keep[i] = true
		}
	}

	oldToNew := make([]int, len(lines))
	for i := range oldToNew {
		oldToNew[i] = -1
	}

	firstKept := -1
	lastKept := -1
	for i, v := range keep {
		if !v {
			continue
		}
		if firstKept == -1 {
			firstKept = i
		}
		lastKept = i
	}
	if firstKept == -1 {
		return lines, tgts
	}

	newLines := make([]string, 0, len(lines))
	if firstKept > 0 {
		newLines = append(newLines, "...")
	}

	for i := firstKept; i <= lastKept; {
		if keep[i] {
			oldToNew[i] = len(newLines)
			newLines = append(newLines, lines[i])
			i++
			continue
		}

		j := i
		for j <= lastKept && !keep[j] {
			j++
		}
		newLines = append(newLines, "...")
		i = j
	}

	if lastKept < len(lines)-1 {
		newLines = append(newLines, "...")
	}

	newTargets := make([]Target, 0, len(tgts))
	for _, t := range tgts {
		if t.Line < 0 || t.Line >= len(oldToNew) {
			continue
		}
		mapped := oldToNew[t.Line]
		if mapped < 0 {
			continue
		}
		t.Line = mapped
		newTargets = append(newTargets, t)
	}

	return newLines, newTargets
}
