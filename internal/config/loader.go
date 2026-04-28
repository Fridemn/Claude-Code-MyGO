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
	cfg.SessionDir = normalizeLegacyConfigPath(cfg.SessionDir)
	cfg.MCPConfigPath = normalizeLegacyConfigPath(cfg.MCPConfigPath)
	cfg.PluginsConfigPath = normalizeLegacyConfigPath(cfg.PluginsConfigPath)
	cfg.HooksConfigPath = normalizeLegacyConfigPath(cfg.HooksConfigPath)

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

	if profiledCfg, _, err := ApplyActiveAPIProfile(cfg); err != nil {
		return Config{}, err
	} else {
		cfg = profiledCfg
	}

	// Compute SessionDir if not explicitly set
	// Uses same path pattern as TS: ~/.claude/projects/<sanitized-cwd>
	// Go CLI uses: ~/.claude-go/projects/<sanitized-cwd>
	if cfg.SessionDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return Config{}, fmt.Errorf("get home dir: %w", err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		sanitized := sanitizePathForConfig(cwd)
		cfg.SessionDir = filepath.Join(home, ".claude-go", "projects", sanitized)
	} else if !filepath.IsAbs(cfg.SessionDir) {
		// Make relative path absolute based on current working directory
		absPath, err := filepath.Abs(cfg.SessionDir)
		if err != nil {
			return Config{}, fmt.Errorf("resolve session dir path: %w", err)
		}
		cfg.SessionDir = absPath
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

func normalizeLegacyConfigPath(path string) string {
	if path == "" {
		return path
	}
	normalized := strings.ReplaceAll(path, ".claude-code-go", ".claude-go")
	normalized = strings.ReplaceAll(normalized, ".Claude-Go", ".claude-go")
	return normalized
}

// sanitizePathForConfig converts a path to a safe directory name
// Replaces non-alphanumeric chars with '-' to create valid directory names
func sanitizePathForConfig(path string) string {
	var result []rune
	for _, r := range path {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result = append(result, r)
		} else {
			result = append(result, '-')
		}
	}
	s := string(result)
	// Limit length to prevent very long directory names
	if len(s) > 64 {
		s = s[:64]
	}
	return s
}
