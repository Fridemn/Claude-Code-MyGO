package lsp

import (
	"context"
	"errors"
	"path/filepath"

	"claude-go/internal/tool"
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

// globalManager is the global LSP manager instance
var globalManager *Manager

// SetManager sets the global LSP manager (called during app initialization)
func SetManager(m *Manager) {
	globalManager = m
}

// GetManager returns the global LSP manager
func GetManager() *Manager {
	return globalManager
}

// RegisterLSPTools registers LSP tools to the registry
func RegisterLSPTools(r *tool.Registry) {
	r.Register(LSPTool{})
}

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
		"filePath":  tool.SchemaString("The path to the file"),
		"line":      tool.SchemaInteger("The line number (1-based, as shown in editors)"),
		"character": tool.SchemaInteger("The character offset (1-based, as shown in editors)"),
		"query":     tool.SchemaString("For workspaceSymbol: the search query"),
		"symbolData": tool.SchemaObject(map[string]any{
			"name": tool.SchemaString("Symbol name"),
			"kind": tool.SchemaInteger("Symbol kind"),
			"uri":  tool.SchemaString("File URI"),
			"range": tool.SchemaObject(map[string]any{
				"start": tool.SchemaObject(map[string]any{
					"line":      tool.SchemaInteger("Start line"),
					"character": tool.SchemaInteger("Start character"),
				}),
				"end": tool.SchemaObject(map[string]any{
					"line":      tool.SchemaInteger("End line"),
					"character": tool.SchemaInteger("End character"),
				}),
			}),
		}, "Symbol data from prepareCallHierarchy"),
	}, "operation", "filePath")
}

// Call executes the LSP tool
func (t LSPTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	operation := getString(in, "operation")
	filePath := getString(in, "filePath")
	line := getUint32(in, "line", 1)
	character := getUint32(in, "character", 1)

	m := globalManager
	if m == nil {
		m = NewManager()
		globalManager = m
	}

	var content string
	var errStr string

	switch operation {
	case OpGoToDefinition:
		if filePath == "" {
			return tool.Result{}, errors.New("filePath is required")
		}
		result := GoToDefinition(ctx, m, filePath, line, character)
		content = result.Content
		errStr = result.Error

	case OpFindReferences:
		if filePath == "" {
			return tool.Result{}, errors.New("filePath is required")
		}
		result := FindReferences(ctx, m, filePath, line, character)
		content = result.Content
		errStr = result.Error

	case OpHover:
		if filePath == "" {
			return tool.Result{}, errors.New("filePath is required")
		}
		result := HoverOp(ctx, m, filePath, line, character)
		content = result.Content
		errStr = result.Error

	case OpDocumentSymbol:
		if filePath == "" {
			return tool.Result{}, errors.New("filePath is required")
		}
		result := DocumentSymbolOp(ctx, m, filePath)
		content = result.Content
		errStr = result.Error

	case OpWorkspaceSymbol:
		query := getString(in, "query")
		result := WorkspaceSymbol(ctx, m, query)
		content = result.Content
		errStr = result.Error

	case OpGoToImplementation:
		if filePath == "" {
			return tool.Result{}, errors.New("filePath is required")
		}
		result := GoToImplementation(ctx, m, filePath, line, character)
		content = result.Content
		errStr = result.Error

	case OpPrepareCallHierarchy:
		if filePath == "" {
			return tool.Result{}, errors.New("filePath is required")
		}
		result := PrepareCallHierarchy(ctx, m, filePath, line, character)
		content = result.Content
		errStr = result.Error

	case OpIncomingCalls:
		// Requires symbol data from prepareCallHierarchy
		item := extractCallHierarchyItem(in)
		if item.Name == "" {
			return tool.Result{}, errors.New("symbolData is required for incomingCalls (run prepareCallHierarchy first)")
		}
		result := IncomingCalls(ctx, m, item)
		content = result.Content
		errStr = result.Error

	case OpOutgoingCalls:
		// Requires symbol data from prepareCallHierarchy
		item := extractCallHierarchyItem(in)
		if item.Name == "" {
			return tool.Result{}, errors.New("symbolData is required for outgoingCalls (run prepareCallHierarchy first)")
		}
		result := OutgoingCalls(ctx, m, item)
		content = result.Content
		errStr = result.Error

	default:
		return tool.Result{}, errors.New("invalid operation: " + operation)
	}

	if errStr != "" {
		return tool.Result{
			Content: "LSP error: " + errStr,
			Meta: map[string]any{
				"operation": operation,
				"filePath":  filePath,
				"line":      line,
				"character": character,
			},
		}, nil
	}

	return tool.Result{
		Content: content,
		Meta: map[string]any{
			"operation": operation,
			"filePath":  filePath,
			"line":      line,
			"character": character,
		},
	}, nil
}

// extractCallHierarchyItem extracts a CallHierarchyItem from tool input
func extractCallHierarchyItem(in tool.Input) CallHierarchyItem {
	var item CallHierarchyItem
	if data, ok := in["symbolData"]; ok {
		if m, ok := data.(map[string]any); ok {
			if name, ok := m["name"].(string); ok {
				item.Name = name
			}
			if uri, ok := m["uri"].(string); ok {
				item.URI = URI(uri)
			}
			if kind, ok := m["kind"].(float64); ok {
				item.Kind = uint32(kind)
			}
			if r, ok := m["range"].(map[string]any); ok {
				if start, ok := r["start"].(map[string]any); ok {
					if line, ok := start["line"].(float64); ok {
						item.Range.Start.Line = uint32(line)
					}
					if char, ok := start["character"].(float64); ok {
						item.Range.Start.Character = uint32(char)
					}
				}
				if end, ok := r["end"].(map[string]any); ok {
					if line, ok := end["line"].(float64); ok {
						item.Range.End.Line = uint32(line)
					}
					if char, ok := end["character"].(float64); ok {
						item.Range.End.Character = uint32(char)
					}
				}
			}
		}
	}
	return item
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

func getUint32(in tool.Input, key string, def uint32) uint32 {
	if v, ok := in[key]; ok {
		switch n := v.(type) {
		case int:
			if n > 0 {
				return uint32(n)
			}
		case int64:
			if n > 0 {
				return uint32(n)
			}
		case float64:
			if n > 0 {
				return uint32(n)
			}
		}
	}
	return def
}

func getFileName(filePath string) string {
	if filePath == "" {
		return ""
	}
	return filepath.Base(filePath)
}
