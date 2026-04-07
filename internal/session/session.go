package session

import "claude-code-go/internal/types"

type Session struct {
	ID       string          `json:"id"`
	Messages []types.Message `json:"messages"`
}
