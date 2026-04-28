package config

import (
	"strconv"
	"strings"

	"claude-go/internal/constants"
)

const (
	defaultBaseURL = "" // Must be set via CLAUDE_CODE_BASE_URL in .env
	defaultModel   = "gpt-4.1"
	// SessionDir is computed dynamically based on cwd, not a fixed path
	// Like TS: ~/.claude/projects/<sanitized-cwd>
	defaultSessionDir = "" // Empty means compute from cwd
)

var (
	builtinAppName           = constants.AppName
	builtinMaxTurns          = ""
	builtinSessionDir        = defaultSessionDir
	builtinMCPConfigPath     = ".claude-go/mcp.json"
	builtinPluginsConfigPath = ".claude-go/plugins.json"
	builtinHooksConfigPath   = ".claude-go/hooks.json"
	builtinContextWindow     = ""
)

func defaults() Config {
	cfg := Config{
		BaseURL:           defaultBaseURL,
		Model:             defaultModel,
		AppName:           builtinAppName,
		MaxTurns:          intFromBuildDefault(builtinMaxTurns, constants.DefaultMaxTurns),
		SessionDir:        builtinSessionDir,
		MCPConfigPath:     builtinMCPConfigPath,
		PluginsConfigPath: builtinPluginsConfigPath,
		HooksConfigPath:   builtinHooksConfigPath,
	}
	if strings.TrimSpace(builtinMCPConfigPath) != "" {
		cfg.MCPConfigPath = builtinMCPConfigPath
	}
	if strings.TrimSpace(builtinPluginsConfigPath) != "" {
		cfg.PluginsConfigPath = builtinPluginsConfigPath
	}
	if strings.TrimSpace(builtinHooksConfigPath) != "" {
		cfg.HooksConfigPath = builtinHooksConfigPath
	}
	cfg.ContextWindowOverride = intFromBuildDefault(builtinContextWindow, 0)
	return cfg
}

func intFromBuildDefault(raw string, fallback int) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
