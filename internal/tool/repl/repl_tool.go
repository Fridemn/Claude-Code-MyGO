package repl

import (
	"context"

	"claude-code-go/internal/tool"
)

// REPLToolName is the name of the REPL tool
const REPLToolName = "REPL"

// REPLOnlyTools that are only accessible via REPL when REPL mode is enabled
// When REPL mode is on, these tools are hidden from Claude's direct use,
// forcing Claude to use REPL for batch operations.
var REPLOnlyTools = []string{
	"Read",
	"Write",
	"Edit",
	"Glob",
	"Grep",
	"Bash",
	"NotebookEdit",
	"Agent",
}

// PrimitiveTools returns the list of primitive tools available in REPL mode
func PrimitiveTools() []string {
	return REPLOnlyTools
}

// IsREPLOnlyTool checks if a tool is a REPL-only tool
func IsREPLOnlyTool(name string) bool {
	for _, toolName := range REPLOnlyTools {
		if toolName == name {
			return true
		}
	}
	return false
}

// REPLTool implements the REPL tool concept
// This is primarily a marker/constant file - the actual REPL logic
// would be in the REPL runtime/bridge component
type REPLTool struct{}

// Name returns the tool name
func (REPLTool) Name() string { return REPLToolName }

// Description returns the tool description
func (REPLTool) Description() string {
	return "Interactive REPL for batch tool operations"
}

// IsReadOnly returns false as REPL can modify state
func (REPLTool) IsReadOnly(tool.Input) bool { return false }

// ParametersSchema returns the JSON schema for the tool parameters
func (REPLTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"script":     tool.SchemaString("JavaScript/TypeScript code to execute in the REPL context"),
		"background": tool.SchemaBoolean("Run in background mode (non-blocking)"),
	})
}

// Call executes the REPL tool
func (REPLTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	// TODO: Implement actual REPL execution
	// This requires integration with a JavaScript/TypeScript runtime
	return tool.Result{
		Content: "REPL tool not yet implemented - requires runtime integration",
	}, nil
}

// RegisterREPLTools registers REPL tools to the registry
func RegisterREPLTools(r *tool.Registry) {
	r.Register(REPLTool{})
}