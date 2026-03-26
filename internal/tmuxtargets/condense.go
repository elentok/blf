package tmuxtargets

func condenseViewport(lines []string, targets []target, context int) ([]string, []target) {
	if len(lines) == 0 || len(targets) == 0 {
		return lines, targets
	}
	if context < 0 {
		context = 0
	}

	keep := make([]bool, len(lines))
	for _, t := range targets {
		start := t.line - context
		if start < 0 {
			start = 0
		}
		end := t.line + context
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

	newLines := make([]string, 0, len(lines))
	seenKept := false
	for i := 0; i < len(lines); {
		if keep[i] {
			seenKept = true
			oldToNew[i] = len(newLines)
			newLines = append(newLines, lines[i])
			i++
			continue
		}

		j := i
		for j < len(lines) && !keep[j] {
			j++
		}
		if seenKept && j < len(lines) {
			newLines = append(newLines, "...")
		}
		i = j
	}

	newTargets := make([]target, 0, len(targets))
	for _, t := range targets {
		if t.line < 0 || t.line >= len(oldToNew) {
			continue
		}
		mapped := oldToNew[t.line]
		if mapped < 0 {
			continue
		}
		t.line = mapped
		newTargets = append(newTargets, t)
	}

	return newLines, newTargets
}
