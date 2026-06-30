package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// Conversation holds cheap metadata extracted from a single .jsonl transcript.
type Conversation struct {
	Path         string
	SessionID    string
	Title        string
	LastAccessed time.Time
}

// ConversationMeta parses path cheaply to extract title and last-accessed time.
// It does not count turns or tokens.
func ConversationMeta(path string) (Conversation, error) {
	f, err := os.Open(path)
	if err != nil {
		return Conversation{}, err
	}
	defer f.Close()

	var (
		aiTitle       string
		lastPrompt    string
		firstUserText string
		sessionID     string
		lastAccessed  time.Time
	)

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		var rec transcriptLine
		if err := json.Unmarshal(scanner.Bytes(), &rec); err != nil {
			continue
		}

		if rec.SessionID != "" && sessionID == "" {
			sessionID = rec.SessionID
		}

		if rec.Timestamp != "" {
			if t, err := time.Parse(time.RFC3339Nano, rec.Timestamp); err == nil {
				lastAccessed = t
			}
		}

		switch rec.Type {
		case "ai-title":
			if aiTitle == "" {
				aiTitle = rec.AITitle
			}
		case "last-prompt":
			if rec.LastPrompt != "" {
				lastPrompt = rec.LastPrompt
			}
		case "user":
			if firstUserText == "" && !rec.IsSidechain && rec.ParentUUID == "" {
				firstUserText = extractUserMessageText(rec.RawMessage)
			}
		}
	}

	return Conversation{
		Path:         path,
		SessionID:    sessionID,
		Title:        buildConversationTitle(aiTitle, lastPrompt, firstUserText, sessionID),
		LastAccessed: lastAccessed,
	}, scanner.Err()
}

// ListConversations returns all conversations in dir (a project directory),
// sorted most-recently-accessed first. Metadata is parsed in parallel.
func ListConversations(dir string) ([]Conversation, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var paths []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			paths = append(paths, filepath.Join(dir, e.Name()))
		}
	}

	if len(paths) == 0 {
		return nil, nil
	}

	type result struct {
		conv Conversation
		err  error
	}
	results := make([]result, len(paths))
	var wg sync.WaitGroup
	for i, p := range paths {
		wg.Add(1)
		go func(i int, p string) {
			defer wg.Done()
			c, err := ConversationMeta(p)
			results[i] = result{conv: c, err: err}
		}(i, p)
	}
	wg.Wait()

	var convs []Conversation
	for _, r := range results {
		if r.err == nil && (r.conv.SessionID != "" || r.conv.Title != "") {
			convs = append(convs, r.conv)
		}
	}

	sort.Slice(convs, func(i, j int) bool {
		return convs[i].LastAccessed.After(convs[j].LastAccessed)
	})

	return convs, nil
}

type transcriptLine struct {
	Type        string          `json:"type"`
	SessionID   string          `json:"sessionId"`
	Timestamp   string          `json:"timestamp"`
	AITitle     string          `json:"aiTitle"`
	LastPrompt  string          `json:"lastPrompt"`
	ParentUUID  string          `json:"parentUuid"`
	IsSidechain bool            `json:"isSidechain"`
	RawMessage  json.RawMessage `json:"message"`
}

func extractUserMessageText(rawMsg json.RawMessage) string {
	if len(rawMsg) == 0 {
		return ""
	}
	var msg struct {
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(rawMsg, &msg); err != nil || len(msg.Content) == 0 {
		return ""
	}

	var s string
	if err := json.Unmarshal(msg.Content, &s); err == nil {
		return cleanUserText(s)
	}

	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msg.Content, &blocks); err == nil {
		for _, b := range blocks {
			if b.Type == "text" && b.Text != "" {
				return cleanUserText(b.Text)
			}
		}
	}
	return ""
}

// leadingXMLRE matches a single XML-like tag at the start of a string.
// Used to strip slash-command metadata noise like <command-message>...</command-message>.
var leadingXMLRE = regexp.MustCompile(`^<[^>]+>[^<]*</[^>]+>\s*`)

func cleanUserText(s string) string {
	s = strings.TrimSpace(s)
	for leadingXMLRE.MatchString(s) {
		s = strings.TrimSpace(leadingXMLRE.ReplaceAllLiteralString(s, ""))
	}
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}

func buildConversationTitle(aiTitle, lastPrompt, firstUser, sessionID string) string {
	for _, candidate := range []string{aiTitle, lastPrompt, firstUser} {
		if t := stripLeadingSlashToken(candidate); t != "" {
			return t
		}
	}
	return sessionID
}

// stripLeadingSlashToken strips a leading /command token so that
// "/grill generate a logo" → "generate a logo" and "/clear" → "".
func stripLeadingSlashToken(s string) string {
	s = strings.TrimSpace(s)
	for strings.HasPrefix(s, "/") {
		rest := s[1:]
		idx := strings.IndexAny(rest, " \t\n")
		if idx < 0 {
			return ""
		}
		s = strings.TrimSpace(rest[idx:])
	}
	return s
}
