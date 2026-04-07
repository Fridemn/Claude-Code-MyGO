package engine

import "claude-code-go/internal/types"

type Request struct {
	Model    string
	Messages []types.Message
	Tools    []types.ToolDefinition
}

type Response struct {
	Text      string
	ToolCalls []types.ToolCall
}
