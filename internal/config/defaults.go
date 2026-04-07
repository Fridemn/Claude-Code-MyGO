package config

import "claude-code-go/internal/constants"

const (
	defaultBaseURL           = "" // Must be set via CLAUDE_CODE_BASE_URL in .env
	defaultModel             = "gpt-4.1"
	defaultAppName           = constants.AppName
	defaultMaxTurns          = constants.DefaultMaxTurns
	defaultSessionDir        = ".claude-code-go/sessions"
	defaultMCPConfigPath     = ".claude-code-go/mcp.json"
	defaultPluginsConfigPath = ".claude-code-go/plugins.json"
	defaultHooksConfigPath   = ".claude-code-go/hooks.json"
)

func defaults() Config {
	return Config{
		BaseURL:           defaultBaseURL,
		Model:             defaultModel,
		AppName:           defaultAppName,
		MaxTurns:          defaultMaxTurns,
		SessionDir:        defaultSessionDir,
		MCPConfigPath:     defaultMCPConfigPath,
		PluginsConfigPath: defaultPluginsConfigPath,
		HooksConfigPath:   defaultHooksConfigPath,
	}
}
