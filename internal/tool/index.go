package tool

// RegisterBuiltins registers all built-in tools.
// Note: Individual sub-packages (bash, file, agent, etc.) need to be registered
// separately from the caller to avoid import cycles.
//
// Example usage from main:
//
//	import (
//	    "claude-go/internal/tool"
//	    "claude-go/internal/tool/bash"
//	    "claude-go/internal/tool/file"
//	    "claude-go/internal/tool/agent"
//	)
//
//	func main() {
//	    r := tool.EmptyRegistry()
//	    bash.RegisterBashTool(r)
//	    file.RegisterFileTools(r)
//	    // etc.
//	}
func RegisterBuiltins(r *Registry) {
	// Sub-packages need to be registered from the caller to avoid import cycles
	// This is a design constraint in Go to prevent circular dependencies
}