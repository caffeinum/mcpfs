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
	var useCwd bool

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
		case "--cwd", "-w":
			useCwd = true
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

	if useCwd {
		cwd, err := os.Getwd()
		if err != nil {
			fatal("getwd: %v", err)
		}
		if opencode {
			path, err := findOpencodeByCwd(cwd)
			if err != nil {
				fatal("%v", err)
			}
			sessionID = filepath.Base(strings.TrimSuffix(path, ".json"))
		} else {
			path, err := findClaudeByCwd(cwd)
			if err != nil {
				fatal("%v", err)
			}
			sessionID = strings.TrimSuffix(filepath.Base(path), ".jsonl")
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
  --cwd, -w        find most recent session for current directory
  --opencode, -o   read from opencode format (~/.local/share/opencode)
  --help, -h       show this help

examples:
  claude-session d68288c5-4553-4ea9-a6fa-846015331b5b
  claude-session --cwd | jq 'select(.type=="user")'
  claude-session --opencode --cwd | grep TODO
`)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}

func findClaudeByCwd(cwd string) (string, error) {
	home, _ := os.UserHomeDir()
	encoded := strings.ReplaceAll(cwd, "/", "-")
	projectDir := filepath.Join(home, ".claude", "projects", encoded)

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return "", fmt.Errorf("no sessions for %s", cwd)
	}

	type sessionFile struct {
		path  string
		mtime int64
	}
	var sessions []sessionFile

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}
		if entry.Name() == "sessions-index.json" {
			continue
		}
		path := filepath.Join(projectDir, entry.Name())
		info, err := entry.Info()
		if err != nil {
			continue
		}
		sessions = append(sessions, sessionFile{path: path, mtime: info.ModTime().UnixNano()})
	}

	if len(sessions) == 0 {
		return "", fmt.Errorf("no sessions for %s", cwd)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].mtime > sessions[j].mtime
	})

	return sessions[0].path, nil
}

func findOpencodeByCwd(cwd string) (string, error) {
	home, _ := os.UserHomeDir()
	sessionDir := filepath.Join(home, ".local", "share", "opencode", "storage", "session")

	projectDirs, err := os.ReadDir(sessionDir)
	if err != nil {
		return "", fmt.Errorf("no opencode sessions")
	}

	type sessionFile struct {
		path  string
		mtime int64
	}
	var matches []sessionFile

	for _, projectEntry := range projectDirs {
		if !projectEntry.IsDir() {
			continue
		}
		projectPath := filepath.Join(sessionDir, projectEntry.Name())
		sessions, err := os.ReadDir(projectPath)
		if err != nil {
			continue
		}
		for _, entry := range sessions {
			if !strings.HasSuffix(entry.Name(), ".json") {
				continue
			}
			path := filepath.Join(projectPath, entry.Name())
			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var meta struct {
				Directory string `json:"directory"`
			}
			if err := json.Unmarshal(data, &meta); err != nil {
				continue
			}
			if meta.Directory != cwd {
				continue
			}
			info, err := entry.Info()
			if err != nil {
				continue
			}
			matches = append(matches, sessionFile{path: path, mtime: info.ModTime().UnixNano()})
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no opencode sessions for %s", cwd)
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].mtime > matches[j].mtime
	})

	return matches[0].path, nil
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
