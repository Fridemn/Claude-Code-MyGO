package tool

import "context"

type Input map[string]any

type Result struct {
	Content      any            `json:"content"`
	Meta         map[string]any `json:"meta,omitempty"`
	Error        string         `json:"error,omitempty"`
	SkillContent string         `json:"skill_content,omitempty"` // Inline skill expansion content
}

// PermissionResult represents the result of a permission check
type PermissionResult struct {
	Decision    PermissionDecision `json:"decision"`    // allow, deny, ask
	Message     string             `json:"message"`     // Reason for the decision (shown to user)
	Reason      string             `json:"reason"`      // Internal reason for logging
	RuleMatched string             `json:"rule_matched"` // The rule that triggered this decision, if any
}

// PermissionDecision represents the permission decision
type PermissionDecision string

const (
	PermissionAllow PermissionDecision = "allow"
	PermissionDeny  PermissionDecision = "deny"
	PermissionAsk   PermissionDecision = "ask"
)

// PermissionChecker is an interface for checking permissions before tool execution
type PermissionChecker interface {
	CheckPermissions(ctx context.Context, input Input) PermissionResult
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