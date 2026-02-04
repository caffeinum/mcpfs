package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	var sessionID string
	var opencode bool

	args := os.Args[1:]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--current", "-c":
			sessionID = os.Getenv("CLAUDE_SESSION_ID")
			if sessionID == "" {
				sessionID = os.Getenv("OPENCODE_SESSION_ID")
			}
			if sessionID == "" {
				fatal("CLAUDE_SESSION_ID or OPENCODE_SESSION_ID not set")
			}
		case "--opencode", "-o":
			opencode = true
		case "--help", "-h":
			usage()
			os.Exit(0)
		default:
			if strings.HasPrefix(args[i], "-") {
				fatal("unknown flag: %s", args[i])
			}
			sessionID = args[i]
		}
	}

	if sessionID == "" {
		usage()
		os.Exit(1)
	}

	if opencode {
		if err := outputOpencode(sessionID); err != nil {
			fatal("opencode: %v", err)
		}
	} else {
		if err := outputClaude(sessionID); err != nil {
			fatal("claude: %v", err)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `usage: claude-session [flags] <session-id>

outputs session history as jsonl to stdout.

flags:
  --current, -c    use $CLAUDE_SESSION_ID or $OPENCODE_SESSION_ID
  --opencode, -o   read from opencode format (~/.local/share/opencode)
  --help, -h       show this help

examples:
  claude-session d68288c5-4553-4ea9-a6fa-846015331b5b
  claude-session --current | jq 'select(.type=="user")'
  claude-session --opencode ses_abc123 | grep TODO
`)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func outputClaude(sessionID string) error {
	path, err := findClaudeSession(sessionID)
	if err != nil {
		return err
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(os.Stdout, f)
	return err
}

func findClaudeSession(sessionID string) (string, error) {
	home, _ := os.UserHomeDir()

	paths := []string{
		filepath.Join(home, ".claude", "transcripts", "ses_"+sessionID+".jsonl"),
		filepath.Join(home, ".claude", "transcripts", sessionID+".jsonl"),
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	projectsDir := filepath.Join(home, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			sessionFile := filepath.Join(projectsDir, entry.Name(), sessionID+".jsonl")
			if _, err := os.Stat(sessionFile); err == nil {
				return sessionFile, nil
			}
		}
	}

	return "", fmt.Errorf("session not found: %s", sessionID)
}

func outputOpencode(sessionID string) error {
	home, _ := os.UserHomeDir()
	baseDir := filepath.Join(home, ".local", "share", "opencode", "storage")

	if !strings.HasPrefix(sessionID, "ses_") {
		sessionID = "ses_" + sessionID
	}

	messageDir := filepath.Join(baseDir, "message", sessionID)
	if _, err := os.Stat(messageDir); err != nil {
		return fmt.Errorf("session not found: %s", sessionID)
	}

	entries, err := os.ReadDir(messageDir)
	if err != nil {
		return err
	}

	type messageWithTime struct {
		data    map[string]any
		created int64
	}

	var messages []messageWithTime

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		msgPath := filepath.Join(messageDir, entry.Name())
		data, err := os.ReadFile(msgPath)
		if err != nil {
			continue
		}

		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}

		var created int64
		if timeObj, ok := msg["time"].(map[string]any); ok {
			if c, ok := timeObj["created"].(float64); ok {
				created = int64(c)
			}
		}

		msgID := strings.TrimSuffix(entry.Name(), ".json")
		partsDir := filepath.Join(baseDir, "part", msgID)
		if parts, err := loadParts(partsDir); err == nil && len(parts) > 0 {
			msg["parts"] = parts
		}

		messages = append(messages, messageWithTime{data: msg, created: created})
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].created < messages[j].created
	})

	w := bufio.NewWriter(os.Stdout)
	enc := json.NewEncoder(w)
	for _, msg := range messages {
		enc.Encode(msg.data)
	}
	w.Flush()

	return nil
}

func loadParts(partsDir string) ([]map[string]any, error) {
	entries, err := os.ReadDir(partsDir)
	if err != nil {
		return nil, err
	}

	var parts []map[string]any
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(partsDir, entry.Name()))
		if err != nil {
			continue
		}

		var part map[string]any
		if err := json.Unmarshal(data, &part); err != nil {
			continue
		}
		parts = append(parts, part)
	}

	return parts, nil
}
