package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func Load(envFile string) (Config, error) {
	cfg := defaults()

	if err := loadEnvFile(envFile); err != nil {
		return Config{}, err
	}

	cfg.APIKey = strings.TrimSpace(os.Getenv("CLAUDE_CODE_API_KEY"))
	cfg.BaseURL = valueOrDefault("CLAUDE_CODE_BASE_URL", cfg.BaseURL)
	cfg.Model = valueOrDefault("CLAUDE_CODE_MODEL", cfg.Model)
	cfg.AppName = valueOrDefault("CLAUDE_CODE_APP_NAME", cfg.AppName)
	cfg.SessionDir = valueOrDefault("CLAUDE_CODE_SESSION_DIR", cfg.SessionDir)
	cfg.SystemPrompt = strings.TrimSpace(os.Getenv("CLAUDE_CODE_SYSTEM_PROMPT"))
	cfg.MCPConfigPath = valueOrDefault("CLAUDE_CODE_MCP_CONFIG", cfg.MCPConfigPath)
	cfg.PluginsConfigPath = valueOrDefault("CLAUDE_CODE_PLUGINS_CONFIG", cfg.PluginsConfigPath)
	cfg.HooksConfigPath = valueOrDefault("CLAUDE_CODE_HOOKS_CONFIG", cfg.HooksConfigPath)

	// Summary/compact model (optional, defaults to Model)
	cfg.SummaryModel = strings.TrimSpace(os.Getenv("CLAUDE_CODE_SUMMARY_MODEL"))

	// Auto-compact enabled (defaults to true)
	cfg.AutoCompactEnabled = true
	if raw := strings.TrimSpace(os.Getenv("CLAUDE_CODE_AUTO_COMPACT")); raw != "" {
		cfg.AutoCompactEnabled = strings.EqualFold(raw, "true") || raw == "1"
	}

	// Context window override (optional)
	if raw := strings.TrimSpace(os.Getenv("CLAUDE_CODE_CONTEXT_WINDOW")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return Config{}, fmt.Errorf("invalid CLAUDE_CODE_CONTEXT_WINDOW: %q", raw)
		}
		cfg.ContextWindowOverride = n
	}

	if raw := strings.TrimSpace(os.Getenv("CLAUDE_CODE_MAX_TURNS")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			return Config{}, fmt.Errorf("invalid CLAUDE_CODE_MAX_TURNS: %q", raw)
		}
		cfg.MaxTurns = n
	}

	if cfg.APIKey == "" {
		return Config{}, fmt.Errorf("CLAUDE_CODE_API_KEY is required")
	}

	if cfg.BaseURL == "" {
		return Config{}, fmt.Errorf("CLAUDE_CODE_BASE_URL is required")
	}

	if !filepath.IsAbs(cfg.SessionDir) {
		cfg.SessionDir = filepath.Clean(cfg.SessionDir)
	}
	if !filepath.IsAbs(cfg.MCPConfigPath) {
		cfg.MCPConfigPath = filepath.Clean(cfg.MCPConfigPath)
	}
	if !filepath.IsAbs(cfg.PluginsConfigPath) {
		cfg.PluginsConfigPath = filepath.Clean(cfg.PluginsConfigPath)
	}
	if !filepath.IsAbs(cfg.HooksConfigPath) {
		cfg.HooksConfigPath = filepath.Clean(cfg.HooksConfigPath)
	}

	return cfg, nil
}

func loadEnvFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid env line: %q", line)
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			return fmt.Errorf("invalid env key in line: %q", line)
		}
		if _, exists := os.LookupEnv(key); !exists {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}
	return scanner.Err()
}

func valueOrDefault(key, fallback string) string {
	if val := strings.TrimSpace(os.Getenv(key)); val != "" {
		return val
	}
	return fallback
}
