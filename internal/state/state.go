package state

import (
	"time"

	"claude-code-go/internal/types"
)

type AppState struct {
	SessionID          string          `json:"session_id"`
	CurrentModel       string          `json:"current_model"`
	PermissionMode     string          `json:"permission_mode,omitempty"`
	IsLoading          bool            `json:"is_loading"`
	IsThinking         bool            `json:"is_thinking"`
	ToolCallInProgress bool            `json:"tool_call_in_progress"`
	Messages           []types.Message `json:"messages,omitempty"`
	UpdatedAt          time.Time       `json:"updated_at,omitempty"`
}
