package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportHTTP  Transport = "http"
)

type ServerConfig struct {
	Transport Transport         `json:"transport"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
}

type Config struct {
	Servers map[string]*ServerConfig `json:"servers"`
	dir     string
}

func DefaultConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".mcp", ".config")
}

func Load(configDir string) (*Config, error) {
	if configDir == "" {
		configDir = DefaultConfigDir()
	}

	cfg := &Config{
		Servers: make(map[string]*ServerConfig),
		dir:     configDir,
	}

	serversPath := filepath.Join(configDir, "servers.json")
	data, err := os.ReadFile(serversPath)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read servers.json: %w", err)
	}

	if err := json.Unmarshal(data, &cfg.Servers); err != nil {
		return nil, fmt.Errorf("parse servers.json: %w", err)
	}

	return cfg, nil
}

func (c *Config) Save() error {
	if err := os.MkdirAll(c.dir, 0755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	serversPath := filepath.Join(c.dir, "servers.json")
	data, err := json.MarshalIndent(c.Servers, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal servers: %w", err)
	}

	if err := os.WriteFile(serversPath, data, 0644); err != nil {
		return fmt.Errorf("write servers.json: %w", err)
	}

	return nil
}

func (c *Config) AddStdioServer(name, command string, args []string, env map[string]string) {
	c.Servers[name] = &ServerConfig{
		Transport: TransportStdio,
		Command:   command,
		Args:      args,
		Env:       env,
	}
}

func (c *Config) AddHTTPServer(name, url string, headers map[string]string) {
	c.Servers[name] = &ServerConfig{
		Transport: TransportHTTP,
		URL:       url,
		Headers:   headers,
	}
}

func (c *Config) GetServer(name string) (*ServerConfig, bool) {
	srv, ok := c.Servers[name]
	return srv, ok
}

func (c *Config) Dir() string {
	return c.dir
}

var authVarPattern = regexp.MustCompile(`\$\{auth\.(\w+)\}`)

func (s *ServerConfig) ResolveEnv(auth *Auth) map[string]string {
	resolved := make(map[string]string)
	for k, v := range s.Env {
		resolved[k] = resolveAuthVars(v, auth)
	}
	return resolved
}

func (s *ServerConfig) ResolveHeaders(auth *Auth) map[string]string {
	resolved := make(map[string]string)
	for k, v := range s.Headers {
		resolved[k] = resolveAuthVars(v, auth)
	}
	return resolved
}

func resolveAuthVars(s string, auth *Auth) string {
	if auth == nil {
		return s
	}
	return authVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		parts := authVarPattern.FindStringSubmatch(match)
		if len(parts) < 2 {
			return match
		}
		key := parts[1]
		if val, ok := auth.Data[key]; ok {
			return val
		}
		return match
	})
}

func ParseServerName(name string) (scope, server string) {
	name = strings.TrimPrefix(name, "@")
	parts := strings.SplitN(name, "/", 2)
	if len(parts) == 2 {
		return "@" + parts[0], parts[1]
	}
	return "", name
}
