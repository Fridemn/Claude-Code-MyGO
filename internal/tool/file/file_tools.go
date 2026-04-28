package file

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"claude-go/internal/tool"
)

// ListFilesTool lists files in a directory (kept for backward compatibility)
type ListFilesTool struct{}

func (ListFilesTool) Name() string        { return "list_files" }
func (ListFilesTool) Description() string { return "list files under a directory" }
func (ListFilesTool) IsReadOnly(tool.Input) bool { return true }

// IsSearchOrReadCommand indicates that list_files is a collapsible list operation
func (ListFilesTool) IsSearchOrReadCommand(tool.Input) tool.SearchOrReadResult {
	return tool.SearchOrReadResult{
		IsCollapsible: true,
		IsList:        true,
	}
}
func (ListFilesTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"path":        tool.SchemaString("Directory path to walk. Defaults to current directory when empty."),
		"max_results": tool.SchemaInteger("Maximum number of paths to return."),
	})
}
func (ListFilesTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	return callListFiles(in, runtime)
}

func callListFiles(in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	root, _ := in["path"].(string)
	root = strings.TrimSpace(root)
	if root == "" {
		if runtime.Store != nil {
			root = runtime.Store.GetCWD()
		} else {
			root = "."
		}
	} else if runtime.Store != nil && !filepath.IsAbs(root) {
		root = filepath.Join(runtime.Store.GetCWD(), root)
	}
	root = filepath.Clean(root)
	maxResults := intFromInput(in, "max_results", 500)
	var files []string
	skippedDirs := map[string]bool{
		".git":         true,
		".svn":         true,
		".hg":          true,
		".bzr":         true,
		".jj":          true,
		".sl":          true,
		".cache":       true,
		"node_modules": true,
	}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && skippedDirs[d.Name()] {
			return filepath.SkipDir
		}
		relPath := path
		if rel, relErr := filepath.Rel(root, path); relErr == nil {
			if rel == "." {
				relPath = "."
			} else {
				relPath = rel
			}
		}
		files = append(files, relPath)
		if len(files) >= maxResults {
			return filepath.SkipAll
		}
		return nil
	})
	return tool.Result{Content: files}, err
}

// RegisterFileTools registers all file tools including the comprehensive TS-compatible versions
func RegisterFileTools(r *tool.Registry) {
	// Legacy tools for backward compatibility
	r.Register(ListFilesTool{})

	// Comprehensive TS-compatible tools
	r.Register(FileReadTool{})
	r.Register(FileWriteTool{})
	r.Register(FileEditTool{})

	// MCP resource tools
	r.Register(ListMcpResourcesTool{})
	r.Register(ReadMcpResourceTool{})
}

func intFromInput(in tool.Input, key string, fallback int) int {
	value, ok := in[key]
	if !ok {
		return fallback
	}
	switch typed := value.(type) {
	case int:
		return typed
	case float64:
		return int(typed)
	default:
		return fallback
	}
}
