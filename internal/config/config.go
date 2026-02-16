package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	Port        int
	TmuxSession string
	Token       string
	ConfigPath  string
	PrintToken  bool
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:        8765,
		TmuxSession: "ai-coding",
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	cfg.ConfigPath = filepath.Join(homeDir, ".config", "agenterm", "config")

	if err := cfg.loadFromFile(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load config file: %w", err)
	}

	flag.IntVar(&cfg.Port, "port", cfg.Port, "server port (1-65535)")
	flag.StringVar(&cfg.TmuxSession, "session", cfg.TmuxSession, "tmux session name")
	flag.StringVar(&cfg.Token, "token", cfg.Token, "authentication token (auto-generated if empty)")
	flag.BoolVar(&cfg.PrintToken, "print-token", false, "print token to stdout (for local debugging)")
	flag.Parse()

	if cfg.Port < 1 || cfg.Port > 65535 {
		return nil, fmt.Errorf("invalid port %d: must be between 1 and 65535", cfg.Port)
	}

	if cfg.Token == "" {
		token, err := generateToken()
		if err != nil {
			return nil, fmt.Errorf("failed to generate token: %w", err)
		}
		cfg.Token = token
		if err := cfg.saveToFile(); err != nil {
			return nil, fmt.Errorf("failed to save config file: %w", err)
		}
	}

	return cfg, nil
}

func (c *Config) loadFromFile() error {
	data, err := os.ReadFile(c.ConfigPath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		switch key {
		case "Token":
			c.Token = value
		case "Port":
			var port int
			if _, err := fmt.Sscanf(value, "%d", &port); err != nil {
				return fmt.Errorf("invalid Port value %q: %w", value, err)
			}
			c.Port = port
		case "TmuxSession":
			c.TmuxSession = value
		}
	}
	return nil
}

func (c *Config) saveToFile() error {
	dir := filepath.Dir(c.ConfigPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data := fmt.Sprintf("Port=%d\nTmuxSession=%s\nToken=%s\n", c.Port, c.TmuxSession, c.Token)
	return os.WriteFile(c.ConfigPath, []byte(data), 0600)
}

func generateToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
