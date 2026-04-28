package config

type Config struct {
	APIKey            string `json:"api_key"`
	BaseURL           string `json:"base_url"`
	Model             string `json:"model"`
	AppName           string `json:"app_name"`
	MaxTurns          int    `json:"max_turns"`
	SessionDir        string `json:"session_dir"`
	SystemPrompt      string `json:"system_prompt"`
	MCPConfigPath     string `json:"mcp_config_path"`
	PluginsConfigPath string `json:"plugins_config_path"`
	HooksConfigPath   string `json:"hooks_config_path"`

	// SummaryModel is the model to use for summary/compact operations
	// If empty, falls back to Model
	SummaryModel      string `json:"summary_model,omitempty"`

	// AutoCompactEnabled controls whether auto-compact triggers
	AutoCompactEnabled bool `json:"auto_compact_enabled,omitempty"`

	// ContextWindowOverride allows overriding the context window size
	ContextWindowOverride int `json:"context_window_override,omitempty"`

	// EditorMode controls keyboard mode ("normal" or "vim")
	// Ported from src/utils/config.ts:editorMode
	EditorMode        string `json:"editor_mode,omitempty"`

	// AgentColor is the color assigned to this agent session
	// Ported from src/utils/sessionStorage.ts:agentColor
	AgentColor        string `json:"agent_color,omitempty"`
}

// Editor mode constants
const (
	EditorModeNormal = "normal"
	EditorModeVim    = "vim"
)

// Agent color constants
// Ported from src/tools/AgentTool/agentColorManager.ts:AGENT_COLORS
var AgentColors = []string{
	"blue",
	"green",
	"yellow",
	"purple",
	"red",
	"orange",
	"pink",
	"cyan",
}
