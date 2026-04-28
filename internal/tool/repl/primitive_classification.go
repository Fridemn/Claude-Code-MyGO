package repl

import (
	"strings"

	"claude-go/internal/tool"
	"claude-go/internal/tool/agent"
	"claude-go/internal/tool/bash"
	"claude-go/internal/tool/file"
	"claude-go/internal/tool/notebook"
	"claude-go/internal/tool/search"
)

// ClassifyPrimitiveTool returns search/read collapse classification for REPL
// primitive tools even when they are hidden from the active registry.
func ClassifyPrimitiveTool(name string, input tool.Input) (tool.SearchOrReadResult, bool) {
	normalized := normalizePrimitiveToolName(strings.TrimSpace(name))
	switch normalized {
	case file.FileReadToolName:
		return file.FileReadTool{}.IsSearchOrReadCommand(input), true
	case file.FileWriteToolName, file.FileEditToolName:
		filePath, _ := input["file_path"].(string)
		if isAutoManagedMemoryFilePath(filePath) {
			return tool.SearchOrReadResult{
				IsCollapsible: true,
				IsMemoryWrite: true,
			}, true
		}
		return tool.SearchOrReadResult{}, true
	case "Grep":
		return search.GrepTool{}.IsSearchOrReadCommand(input), true
	case "Glob":
		return search.GlobTool{}.IsSearchOrReadCommand(input), true
	case "Bash":
		return bash.BashTool{}.IsSearchOrReadCommand(input), true
	case notebook.NotebookEditToolName:
		// Notebook edits are not read/search collapsible.
		return tool.SearchOrReadResult{}, true
	case agent.AgentToolName:
		// Agent operations are REPL primitives but not read/search collapsible.
		return tool.SearchOrReadResult{}, true
	default:
		return tool.SearchOrReadResult{}, false
	}
}

func isAutoManagedMemoryFilePath(path string) bool {
	p := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(path), "\\", "/"))
	if p == "" {
		return false
	}
	return strings.Contains(p, "/session-memory/") ||
		strings.Contains(p, "/agent-memory/") ||
		strings.Contains(p, "/agent-memory-local/") ||
		strings.Contains(p, "/.claude/memory/") ||
		strings.Contains(p, "/.claude/memory")
}
