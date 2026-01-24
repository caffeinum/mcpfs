package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Auth struct {
	Data map[string]string `json:"data"`
}

func LoadAuth(configDir, serverName string) (*Auth, error) {
	if configDir == "" {
		configDir = DefaultConfigDir()
	}

	safeName := strings.ReplaceAll(serverName, "/", "_")
	safeName = strings.TrimPrefix(safeName, "@")
	authPath := filepath.Join(configDir, "auth", safeName+".json")

	data, err := os.ReadFile(authPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &Auth{Data: make(map[string]string)}, nil
		}
		return nil, fmt.Errorf("read auth: %w", err)
	}

	auth := &Auth{Data: make(map[string]string)}
	if err := json.Unmarshal(data, &auth.Data); err != nil {
		return nil, fmt.Errorf("parse auth: %w", err)
	}

	return auth, nil
}

func SaveAuth(configDir, serverName string, auth *Auth) error {
	if configDir == "" {
		configDir = DefaultConfigDir()
	}

	authDir := filepath.Join(configDir, "auth")
	if err := os.MkdirAll(authDir, 0700); err != nil {
		return fmt.Errorf("create auth dir: %w", err)
	}

	safeName := strings.ReplaceAll(serverName, "/", "_")
	safeName = strings.TrimPrefix(safeName, "@")
	authPath := filepath.Join(authDir, safeName+".json")

	data, err := json.MarshalIndent(auth.Data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal auth: %w", err)
	}

	if err := os.WriteFile(authPath, data, 0600); err != nil {
		return fmt.Errorf("write auth: %w", err)
	}

	return nil
}

func SaveToken(configDir, serverName, token string) error {
	auth := &Auth{
		Data: map[string]string{
			"token": token,
		},
	}
	return SaveAuth(configDir, serverName, auth)
}
