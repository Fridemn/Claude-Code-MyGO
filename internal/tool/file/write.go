package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-go/internal/tool"
)

// FileWriteTool implements the Write tool from TypeScript
type FileWriteTool struct{}

func (FileWriteTool) Name() string { return FileWriteToolName }

func (FileWriteTool) Description() string {
	return `Writes a file to the local filesystem.

Usage:
- This tool will overwrite the existing file if there is one at the provided path.
- If this is an existing file, you MUST use the Read tool first to read the file's contents. This tool will fail if you did not read the file first.
- Prefer the Edit tool for modifying existing files — it only sends the diff. Only use this tool to create new files or for complete rewrites.
- NEVER create documentation files (*.md) or README files unless explicitly requested by the User.
- Only use emojis if the user explicitly requests it. Avoid writing emojis to files unless asked.`
}

func (FileWriteTool) IsReadOnly(tool.Input) bool { return false }

func (FileWriteTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"file_path": tool.SchemaString("The absolute path to the file to write (must be absolute, not relative)"),
		"content":   tool.SchemaString("The content to write to the file"),
	}, "file_path", "content")
}

// FileWriteResult represents the result of writing a file
type FileWriteResult struct {
	Type            string           `json:"type"`
	FilePath        string           `json:"filePath"`
	Content         string           `json:"content"`
	StructuredPatch []StructuredHunk `json:"structuredPatch"`
	OriginalFile    *string          `json:"originalFile"`
}

// StructuredHunk represents a diff hunk
type StructuredHunk struct {
	OldStart int      `json:"oldStart"`
	OldLines int      `json:"oldLines"`
	NewStart int      `json:"newStart"`
	NewLines int      `json:"newLines"`
	Lines    []string `json:"lines"`
}

func (t FileWriteTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	filePath := tool.GetString(in, "file_path")
	content := tool.GetString(in, "content")

	if filePath == "" {
		return tool.Result{}, fmt.Errorf("file_path is required")
	}

	// Expand and clean the path
	fullFilePath := expandPath(filePath)

	// Check if file exists
	existingContent, fileExists, err := readFileIfExists(fullFilePath)
	if err != nil {
		return tool.Result{}, err
	}

	// Ensure parent directory exists
	dir := filepath.Dir(fullFilePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return tool.Result{}, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Write the content
	if err := os.WriteFile(fullFilePath, []byte(content), 0644); err != nil {
		return tool.Result{}, fmt.Errorf("failed to write file: %w", err)
	}

	// Determine if this was a create or update
	resultType := "create"
	var originalFile *string
	if fileExists {
		resultType = "update"
		originalFile = &existingContent
	}

	// Generate patch for display
	patch := generateWritePatch(existingContent, content, fileExists)

	result := FileWriteResult{
		Type:            resultType,
		FilePath:        filePath,
		Content:         content,
		StructuredPatch: patch,
		OriginalFile:    originalFile,
	}

	return tool.Result{Content: result}, nil
}

// generateWritePatch creates a simple patch representation for write operations
func generateWritePatch(oldContent, newContent string, fileExists bool) []StructuredHunk {
	if !fileExists || oldContent == newContent {
		return []StructuredHunk{}
	}

	// Simple line-based diff
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// For a complete replacement, create a single hunk
	lines := make([]string, 0, len(oldLines)+len(newLines)+1)

	// Add deletion markers for old content
	for _, line := range oldLines {
		lines = append(lines, "-"+line)
	}

	// Add insertion markers for new content
	for _, line := range newLines {
		lines = append(lines, "+"+line)
	}

	return []StructuredHunk{
		{
			OldStart: 1,
			OldLines: len(oldLines),
			NewStart: 1,
			NewLines: len(newLines),
			Lines:    lines,
		},
	}
}