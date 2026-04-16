package targets

import (
	"regexp"
	"sort"
	"strings"
)

// TargetKind identifies the type of a matched target.
type TargetKind int

const (
	KindURL TargetKind = iota
	KindResumeCommand
	KindFileRef
	KindFilePath
	KindCommit
	KindEmail
	KindHostPort
	KindUUID
	KindIssue
	KindBranchOrTag
	KindBareDomain
)

// Target is a detected selectable span within a line of text.
type Target struct {
	Line       int
	Start      int
	End        int
	Kind       TargetKind
	Text       string
	Openable   bool
	OpenTarget string
}

type candidate struct {
	line       int
	start      int
	end        int
	kind       TargetKind
	text       string
	openable   bool
	openTarget string
}

type patternDef struct {
	kind     TargetKind
	re       *regexp.Regexp
	openable bool
	norm     func(string) string
}

var patterns = []patternDef{
	{kind: KindURL, re: regexp.MustCompile(`https?://[^\s<>")\]}]+`), openable: true, norm: identity},
	{kind: KindResumeCommand, re: regexp.MustCompile(`\b(?:codex resume|opencode -s|claude --resume|agent --resume|cursor-agent --resume) [A-Za-z0-9_-]+\b`), norm: identity},
	{kind: KindFileRef, re: regexp.MustCompile(`(?:~(?:/)?|\.{1,2}/|/)?[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)+:\d+(?::\d+)?`), norm: identity},
	{kind: KindFilePath, re: regexp.MustCompile(`(?:~(?:/)?|\.{1,2}/|/)?[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)+`), norm: identity},
	{kind: KindCommit, re: regexp.MustCompile(`\b[0-9a-f]{7,40}\b`), norm: identity},
	{kind: KindEmail, re: regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`), norm: identity},
	{kind: KindHostPort, re: regexp.MustCompile(`\b(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}:\d{2,5}\b`), openable: true, norm: withHTTPS},
	{kind: KindHostPort, re: regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}:\d{2,5}\b`), openable: true, norm: withHTTP},
	{kind: KindUUID, re: regexp.MustCompile(`\b[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}\b`), norm: identity},
	{kind: KindIssue, re: regexp.MustCompile(`\B#\d+\b`), norm: identity},
	{kind: KindBranchOrTag, re: regexp.MustCompile(`\b(?:[A-Za-z0-9._-]+/[A-Za-z0-9._-]+|v\d+\.\d+\.\d+)\b`), norm: identity},
	{kind: KindBareDomain, re: regexp.MustCompile(`\b(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}/[^\s<>")\]}]*`), openable: true, norm: withHTTPS},
}

func identity(s string) string  { return s }
func withHTTPS(s string) string { return "https://" + s }
func withHTTP(s string) string  { return "http://" + s }

// DetectTargets scans lines and returns all unique selectable targets in
// reading order, with overlapping spans resolved by position and precedence.
func DetectTargets(lines []string) []Target {
	if len(lines) == 0 {
		return nil
	}

	all := make([]Target, 0)
	seen := map[string]struct{}{}
	for lineIndex, line := range lines {
		lineTargets := detectTargetsInLine(lineIndex, line)
		for _, t := range lineTargets {
			if _, exists := seen[t.Text]; exists {
				continue
			}
			seen[t.Text] = struct{}{}
			all = append(all, t)
		}
	}
	return all
}

func detectTargetsInLine(lineIndex int, line string) []Target {
	if line == "" {
		return nil
	}

	cands := make([]candidate, 0)
	for _, def := range patterns {
		matches := def.re.FindAllStringIndex(line, -1)
		for _, m := range matches {
			text := strings.TrimRight(line[m[0]:m[1]], ").,;:]}\"")
			if text == "" {
				continue
			}
			adjEnd := m[0] + len(text)
			cands = append(cands, candidate{
				line:       lineIndex,
				start:      m[0],
				end:        adjEnd,
				kind:       def.kind,
				text:       text,
				openable:   def.openable,
				openTarget: def.norm(text),
			})
		}
	}

	if len(cands) == 0 {
		return nil
	}

	sort.Slice(cands, func(i, j int) bool {
		a := cands[i]
		b := cands[j]
		if a.start != b.start {
			return a.start < b.start
		}
		alen := a.end - a.start
		blen := b.end - b.start
		if alen != blen {
			return alen > blen
		}
		return a.kind < b.kind
	})

	accepted := make([]Target, 0, len(cands))
	for _, c := range cands {
		if len(accepted) > 0 {
			last := accepted[len(accepted)-1]
			if c.start < last.End {
				continue
			}
		}
		accepted = append(accepted, Target{
			Line:       c.line,
			Start:      c.start,
			End:        c.end,
			Kind:       c.kind,
			Text:       c.text,
			Openable:   c.openable,
			OpenTarget: c.openTarget,
		})
	}

	return accepted
}
