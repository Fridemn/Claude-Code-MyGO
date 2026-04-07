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
}
