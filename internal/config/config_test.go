package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	cfg, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("load empty config: %v", err)
	}
	if len(cfg.Servers) != 0 {
		t.Errorf("expected 0 servers, got %d", len(cfg.Servers))
	}
}

func TestSaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cfg, _ := Load(tmpDir)

	cfg.AddStdioServer("@notion/mcp", "npx", []string{"-y", "@notionhq/notion-mcp-server"}, map[string]string{
		"NOTION_TOKEN": "${auth.token}",
	})
	cfg.AddHTTPServer("@github/mcp-server", "https://mcp.github.com", map[string]string{
		"Authorization": "Bearer ${auth.token}",
	})

	if err := cfg.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	cfg2, err := Load(tmpDir)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}

	if len(cfg2.Servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(cfg2.Servers))
	}

	notion, ok := cfg2.GetServer("@notion/mcp")
	if !ok {
		t.Fatal("notion server not found")
	}
	if notion.Transport != TransportStdio {
		t.Errorf("expected stdio transport, got %s", notion.Transport)
	}
	if notion.Command != "npx" {
		t.Errorf("expected npx command, got %s", notion.Command)
	}

	github, ok := cfg2.GetServer("@github/mcp-server")
	if !ok {
		t.Fatal("github server not found")
	}
	if github.Transport != TransportHTTP {
		t.Errorf("expected http transport, got %s", github.Transport)
	}
}

func TestResolveAuthVars(t *testing.T) {
	auth := &Auth{
		Data: map[string]string{
			"token":  "secret123",
			"apikey": "key456",
		},
	}

	srv := &ServerConfig{
		Env: map[string]string{
			"NOTION_TOKEN": "${auth.token}",
			"API_KEY":      "${auth.apikey}",
			"STATIC":       "unchanged",
		},
		Headers: map[string]string{
			"Authorization": "Bearer ${auth.token}",
		},
	}

	env := srv.ResolveEnv(auth)
	if env["NOTION_TOKEN"] != "secret123" {
		t.Errorf("expected secret123, got %s", env["NOTION_TOKEN"])
	}
	if env["API_KEY"] != "key456" {
		t.Errorf("expected key456, got %s", env["API_KEY"])
	}
	if env["STATIC"] != "unchanged" {
		t.Errorf("expected unchanged, got %s", env["STATIC"])
	}

	headers := srv.ResolveHeaders(auth)
	if headers["Authorization"] != "Bearer secret123" {
		t.Errorf("expected Bearer secret123, got %s", headers["Authorization"])
	}
}

func TestParseServerName(t *testing.T) {
	tests := []struct {
		input       string
		wantScope   string
		wantServer  string
	}{
		{"@notion/mcp", "@notion", "mcp"},
		{"notion/mcp", "@notion", "mcp"},
		{"mcp-server", "", "mcp-server"},
		{"@github/mcp-server", "@github", "mcp-server"},
	}

	for _, tt := range tests {
		scope, server := ParseServerName(tt.input)
		if scope != tt.wantScope || server != tt.wantServer {
			t.Errorf("ParseServerName(%q) = (%q, %q), want (%q, %q)",
				tt.input, scope, server, tt.wantScope, tt.wantServer)
		}
	}
}

func TestAuthSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()

	if err := SaveToken(tmpDir, "@notion/mcp", "my-secret-token"); err != nil {
		t.Fatalf("save token: %v", err)
	}

	authFile := filepath.Join(tmpDir, "auth", "notion_mcp.json")
	info, err := os.Stat(authFile)
	if err != nil {
		t.Fatalf("stat auth file: %v", err)
	}
	if info.Mode().Perm() != 0600 {
		t.Errorf("auth file permissions: got %o, want 0600", info.Mode().Perm())
	}

	auth, err := LoadAuth(tmpDir, "@notion/mcp")
	if err != nil {
		t.Fatalf("load auth: %v", err)
	}
	if auth.Data["token"] != "my-secret-token" {
		t.Errorf("expected my-secret-token, got %s", auth.Data["token"])
	}
}

func TestAuthNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	auth, err := LoadAuth(tmpDir, "nonexistent")
	if err != nil {
		t.Fatalf("load nonexistent auth: %v", err)
	}
	if len(auth.Data) != 0 {
		t.Errorf("expected empty auth data, got %v", auth.Data)
	}
}
