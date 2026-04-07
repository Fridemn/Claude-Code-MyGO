package tool

import "context"

type Input map[string]any

type Result struct {
	Content any            `json:"content"`
	Meta    map[string]any `json:"meta,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type Definition interface {
	Name() string
	Description() string
	IsReadOnly(Input) bool
	Call(context.Context, Input, Runtime) (Result, error)
}

type SchemaProvider interface {
	ParametersSchema() map[string]any
}

// SearchOrReadResult indicates whether a tool call is a search or read operation
// that can be collapsed into a summary group in the TUI.
type SearchOrReadResult struct {
	IsCollapsible     bool   // True if this tool use can be collapsed
	IsSearch          bool   // True for search operations (Grep, Glob)
	IsRead            bool   // True for read operations (Read, file reads via Bash)
	IsList            bool   // True for list operations (ls, tree, du)
	IsMemoryWrite     bool   // True for memory file write/edit operations
	IsAbsorbedSilently bool   // True for operations that don't increment counts (REPL, Snip)
	MCPServerName     string // MCP server name if this is an MCP tool
	IsBash            bool   // True for non-search/read Bash commands in fullscreen mode
}

// CollapsibleTool is an optional interface that tools can implement to indicate
// whether their calls are collapsible search/read operations.
type CollapsibleTool interface {
	IsSearchOrReadCommand(input Input) SearchOrReadResult
}