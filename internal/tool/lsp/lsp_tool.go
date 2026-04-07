package lsp

import (
	"context"
	"fmt"

	"claude-code-go/internal/tool"
)

// LSPToolName is the name of the LSP tool
const LSPToolName = "LSP"

// LSPDescription describes the LSP tool functionality
const LSPDescription = `Interact with Language Server Protocol (LSP) servers to get code intelligence features.

Supported operations:
- goToDefinition: Find where a symbol is defined
- findReferences: Find all references to a symbol
- hover: Get hover information (documentation, type info) for a symbol
- documentSymbol: Get all symbols (functions, classes, variables) in a document
- workspaceSymbol: Search for symbols across the entire workspace
- goToImplementation: Find implementations of an interface or abstract method
- prepareCallHierarchy: Get call hierarchy item at a position (functions/methods)
- incomingCalls: Find all functions/methods that call the function at a position
- outgoingCalls: Find all functions/methods called by the function at a position

All operations require:
- filePath: The file to operate on
- line: The line number (1-based, as shown in editors)
- character: The character offset (1-based, as shown in editors)

Note: LSP servers must be configured for the file type. If no server is available, an error will be returned.`

// LSP operations
const (
	OpGoToDefinition       = "goToDefinition"
	OpFindReferences       = "findReferences"
	OpHover                = "hover"
	OpDocumentSymbol       = "documentSymbol"
	OpWorkspaceSymbol      = "workspaceSymbol"
	OpGoToImplementation   = "goToImplementation"
	OpPrepareCallHierarchy = "prepareCallHierarchy"
	OpIncomingCalls        = "incomingCalls"
	OpOutgoingCalls        = "outgoingCalls"
)

// LSPTool implements the LSP tool for code intelligence
type LSPTool struct{}

// Name returns the tool name
func (LSPTool) Name() string { return LSPToolName }

// Description returns the tool description
func (LSPTool) Description() string { return LSPDescription }

// IsReadOnly returns true as LSP operations are read-only
func (LSPTool) IsReadOnly(tool.Input) bool { return true }

// ParametersSchema returns the JSON schema for the tool parameters
func (LSPTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"operation": tool.SchemaEnumString("The LSP operation to perform",
			OpGoToDefinition,
			OpFindReferences,
			OpHover,
			OpDocumentSymbol,
			OpWorkspaceSymbol,
			OpGoToImplementation,
			OpPrepareCallHierarchy,
			OpIncomingCalls,
			OpOutgoingCalls,
		),
		"filePath":  tool.SchemaString("The absolute or relative path to the file"),
		"line":      tool.SchemaInteger("The line number (1-based, as shown in editors)"),
		"character": tool.SchemaInteger("The character offset (1-based, as shown in editors)"),
	}, "operation", "filePath", "line", "character")
}

// Call executes the LSP tool
func (t LSPTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	operation := getString(in, "operation")
	filePath := getString(in, "filePath")
	line := getInt(in, "line", 1)
	character := getInt(in, "character", 1)

	if operation == "" {
		return tool.Result{}, fmt.Errorf("operation is required")
	}
	if filePath == "" {
		return tool.Result{}, fmt.Errorf("filePath is required")
	}

	// Validate operation
	validOps := map[string]bool{
		OpGoToDefinition:       true,
		OpFindReferences:       true,
		OpHover:                true,
		OpDocumentSymbol:       true,
		OpWorkspaceSymbol:      true,
		OpGoToImplementation:   true,
		OpPrepareCallHierarchy: true,
		OpIncomingCalls:        true,
		OpOutgoingCalls:        true,
	}
	if !validOps[operation] {
		return tool.Result{}, fmt.Errorf("invalid operation: %s", operation)
	}

	// TODO: Implement actual LSP communication
	// This requires integration with an LSP client/manager
	// For now, return a placeholder result
	result := fmt.Sprintf("LSP operation '%s' on %s at line %d, character %d not yet implemented - requires LSP server integration",
		operation, filePath, line, character)

	return tool.Result{
		Content: result,
		Meta: map[string]any{
			"operation": operation,
			"filePath":  filePath,
			"line":      line,
			"character": character,
		},
	}, nil
}

// RegisterLSPTools registers LSP tools to the registry
func RegisterLSPTools(r *tool.Registry) {
	r.Register(LSPTool{})
}

// Helper functions for extracting values from Input
func getString(in tool.Input, key string) string {
	if v, ok := in[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(in tool.Input, key string, def int) int {
	if v, ok := in[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return def
}