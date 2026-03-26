package tmuxtargets

import (
	"regexp"
	"sort"
	"strings"
)

type targetKind int

const (
	kindURL targetKind = iota
	kindFileRef
	kindFilePath
	kindCommit
	kindEmail
	kindHostPort
	kindUUID
	kindIssue
	kindBranchOrTag
	kindBareDomain
)

type candidate struct {
	line       int
	start      int
	end        int
	kind       targetKind
	text       string
	openable   bool
	openTarget string
}

type target struct {
	line       int
	start      int
	end        int
	text       string
	openable   bool
	openTarget string
}

type patternDef struct {
	kind     targetKind
	re       *regexp.Regexp
	openable bool
	norm     func(string) string
}

var patterns = []patternDef{
	{kind: kindURL, re: regexp.MustCompile(`https?://[^\s<>")\]}]+`), openable: true, norm: identity},
	{kind: kindFileRef, re: regexp.MustCompile(`(?:~(?:/)?|\.{1,2}/|/)?[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)+:\d+(?::\d+)?`), norm: identity},
	{kind: kindFilePath, re: regexp.MustCompile(`(?:~(?:/)?|\.{1,2}/|/)?[A-Za-z0-9._-]+(?:/[A-Za-z0-9._-]+)+`), norm: identity},
	{kind: kindCommit, re: regexp.MustCompile(`\b[0-9a-f]{7,40}\b`), norm: identity},
	{kind: kindEmail, re: regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`), norm: identity},
	{kind: kindHostPort, re: regexp.MustCompile(`\b(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}:\d{2,5}\b`), openable: true, norm: withHTTPS},
	{kind: kindHostPort, re: regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}:\d{2,5}\b`), openable: true, norm: withHTTP},
	{kind: kindUUID, re: regexp.MustCompile(`\b[0-9a-fA-F]{8}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{4}\-[0-9a-fA-F]{12}\b`), norm: identity},
	{kind: kindIssue, re: regexp.MustCompile(`\B#\d+\b`), norm: identity},
	{kind: kindBranchOrTag, re: regexp.MustCompile(`\b(?:[A-Za-z0-9._-]+/[A-Za-z0-9._-]+|v\d+\.\d+\.\d+)\b`), norm: identity},
	{kind: kindBareDomain, re: regexp.MustCompile(`\b(?:[A-Za-z0-9-]+\.)+[A-Za-z]{2,}/[^\s<>")\]}]*`), openable: true, norm: withHTTPS},
}

func identity(s string) string { return s }

func withHTTPS(s string) string { return "https://" + s }

func withHTTP(s string) string { return "http://" + s }

func detectTargets(lines []string) []target {
	if len(lines) == 0 {
		return nil
	}

	all := make([]target, 0)
	seen := map[string]struct{}{}
	for lineIndex, line := range lines {
		lineTargets := detectTargetsInLine(lineIndex, line)
		for _, t := range lineTargets {
			if _, exists := seen[t.text]; exists {
				continue
			}
			seen[t.text] = struct{}{}
			all = append(all, t)
		}
	}
	return all
}

func detectTargetsInLine(lineIndex int, line string) []target {
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

	accepted := make([]target, 0, len(cands))
	for _, c := range cands {
		if len(accepted) > 0 {
			last := accepted[len(accepted)-1]
			if c.start < last.end {
				continue
			}
		}
		accepted = append(accepted, target{
			line:       c.line,
			start:      c.start,
			end:        c.end,
			text:       c.text,
			openable:   c.openable,
			openTarget: c.openTarget,
		})
	}

	return accepted
}
