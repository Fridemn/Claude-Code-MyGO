package agent

type Definition struct {
	AgentType     string   `json:"agent_type"`
	WhenToUse     string   `json:"when_to_use"`
	Tools         []string `json:"tools,omitempty"`
	Disallowed    []string `json:"disallowed_tools,omitempty"`
	Source        string   `json:"source"`
	BaseDir       string   `json:"base_dir"`
	Model         string   `json:"model,omitempty"`
	Color         string   `json:"color,omitempty"`
	Background    bool     `json:"background,omitempty"`
	SystemPrompt  string   `json:"system_prompt"`
	ReadOnly      bool     `json:"read_only,omitempty"`
	OmitClaudeMd  bool     `json:"omit_claude_md,omitempty"`
	MaxTurns      int      `json:"max_turns,omitempty"`
	InitialPrompt string   `json:"initial_prompt,omitempty"`
}
