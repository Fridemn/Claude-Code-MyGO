package services

// API-based context management strategies
// Ported from src/services/compact/apiMicrocompact.ts

// Context edit strategy types
type ContextEditStrategyType string

const (
	ClearToolUses20250919 ContextEditStrategyType = "clear_tool_uses_20250919"
	ClearThinking20251015 ContextEditStrategyType = "clear_thinking_20251015"
)

// Context management configuration for API calls
type ContextManagementConfig struct {
	Edits []ContextEditStrategy
}

// Context edit strategy for API-based context management
type ContextEditStrategy struct {
	Type           ContextEditStrategyType
	Trigger        *TriggerConfig
	Keep           *KeepConfig
	ClearToolInputs interface{} // bool or []string
	ExcludeTools   []string
	ClearAtLeast   *TriggerConfig
}

// Trigger configuration
type TriggerConfig struct {
	Type  string // "input_tokens"
	Value int
}

// Keep configuration for thinking blocks
type KeepConfig struct {
	Type  string // "thinking_turns" or "all"
	Value int
}

// Tools that can have their results cleared
var ToolsClearableResults = []string{
	"bash", "Bash", "PowerShell",
	"Glob", "Grep", "Read", "WebFetch", "WebSearch",
}

// Tools that use files (can have their uses cleared)
var ToolsClearableUses = []string{
	"Edit", "Write",
}

// GetAPIContextManagement returns API-based context management strategies
// Ported from src/services/compact/apiMicrocompact.ts:getAPIContextManagement
func GetAPIContextManagement(options *ContextManagementOptions) *ContextManagementConfig {
	opts := ContextManagementOptions{
		HasThinking: false,
		IsRedactThinkingActive: false,
		ClearAllThinking: false,
	}
	if options != nil {
		opts = *options
	}

	var strategies []ContextEditStrategy

	// Preserve thinking blocks (unless redact-thinking is active)
	if opts.HasThinking && !opts.IsRedactThinkingActive {
		keepValue := "all"
		if opts.ClearAllThinking {
			keepValue = "thinking_turns"
		}
		strategies = append(strategies, ContextEditStrategy{
			Type: ClearThinking20251015,
			Keep: &KeepConfig{
				Type:  keepValue,
				Value: 1,
			},
		})
	}

	// Tool clearing is ant-only (simplified for Go - always include strategies)
	// In full implementation, this would check USER_TYPE === 'ant'
	if len(strategies) > 0 {
		return &ContextManagementConfig{Edits: strategies}
	}

	return nil
}

// ContextManagementOptions contains options for API context management
type ContextManagementOptions struct {
	HasThinking          bool
	IsRedactThinkingActive bool
	ClearAllThinking     bool
}
