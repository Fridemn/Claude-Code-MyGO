package file

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-code-go/internal/tool"
)

// FileReadTool implements the Read tool from TypeScript
type FileReadTool struct{}

func (FileReadTool) Name() string { return FileReadToolName }

func (FileReadTool) Description() string {
	return `Reads a file from the local filesystem. You can access any file directly by using this tool.
Assume this tool is able to read all files on the machine. If the User provides a path to a file assume that path is valid. It is okay to read a file that does not exist; an error will be returned.

Usage:
- The file_path parameter must be an absolute path, not a relative path
- By default, it reads up to 2000 lines starting from the beginning of the file
- You can optionally specify a line offset and limit (especially handy for long files), but it's recommended to read the whole file by not providing these parameters
- Results are returned using cat -n format, with line numbers starting at 1
- This tool allows Claude Code to read images (eg PNG, JPG, etc). When reading an image file the contents are presented visually as Claude Code is a multimodal LLM.
- This tool can read PDF files (.pdf). For large PDFs (more than 10 pages), you MUST provide the pages parameter to read specific page ranges (e.g., pages: "1-5"). Reading a large PDF without the pages parameter will fail. Maximum 20 pages per request.
- This tool can read Jupyter notebooks (.ipynb files) and returns all cells with their outputs, combining code, text, and visualizations.
- This tool can only read files, not directories. To read a directory, use an ls command via the Bash tool.
- You will regularly be asked to read screenshots. If the user provides a path to a screenshot, ALWAYS use this tool to view the file at the path. This tool will work with all temporary file paths.
- If you read a file that exists but has empty contents you will receive a system reminder warning in place of file contents.`
}

func (FileReadTool) IsReadOnly(tool.Input) bool { return true }

// IsSearchOrReadCommand indicates that Read is always a collapsible read operation
func (FileReadTool) IsSearchOrReadCommand(in tool.Input) tool.SearchOrReadResult {
	return tool.SearchOrReadResult{
		IsCollapsible: true,
		IsRead:        true,
	}
}

func (FileReadTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"file_path": tool.SchemaString("The absolute path to the file to read"),
		"offset":    tool.SchemaInteger("The line number to start reading from. Only provide if the file is too large to read at once"),
		"limit":     tool.SchemaInteger("The number of lines to read. Only provide if the file is too large to read at once."),
		"pages":     tool.SchemaString("Page range for PDF files (e.g., \"1-5\", \"3\", \"10-20\"). Only applicable to PDF files. Maximum 20 pages per request."),
	}, "file_path")
}

// FileReadResult represents the result of reading a file
type FileReadResult struct {
	Type string `json:"type"`
	File any    `json:"file"`
}

// TextFileResult represents a text file read result
type TextFileResult struct {
	FilePath   string `json:"filePath"`
	Content    string `json:"content"`
	NumLines   int    `json:"numLines"`
	StartLine  int    `json:"startLine"`
	TotalLines int    `json:"totalLines"`
}

// ImageFileResult represents an image file read result
type ImageFileResult struct {
	Base64       string `json:"base64"`
	Type         string `json:"type"`
	OriginalSize int    `json:"originalSize"`
}

// NotebookFileResult represents a notebook file read result
type NotebookFileResult struct {
	FilePath string `json:"filePath"`
	Cells    []any  `json:"cells"`
}

func (t FileReadTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	filePath := tool.GetString(in, "file_path")
	if filePath == "" {
		return tool.Result{}, fmt.Errorf("file_path is required")
	}

	// Expand and clean the path
	fullFilePath := expandPath(filePath)

	// Check if it's a blocked device path
	if IsBlockedDevicePath(fullFilePath) {
		return tool.Result{}, fmt.Errorf("cannot read '%s': this device file would block or produce infinite output", filePath)
	}

	offset := tool.GetInt(in, "offset", 1)
	limit := tool.GetInt(in, "limit", MaxLinesToRead)

	// Get file extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fullFilePath), "."))

	// Check for image files
	if IsImageExtension(ext) {
		return t.readImage(fullFilePath, filePath)
	}

	// Check for notebook files
	if ext == "ipynb" {
		return t.readNotebook(fullFilePath, filePath)
	}

	// Read text file
	return t.readTextFile(ctx, fullFilePath, filePath, offset, limit)
}

func (t FileReadTool) readTextFile(ctx context.Context, fullFilePath, filePath string, offset, limit int) (tool.Result, error) {
	// Context is available for cancellation if needed in future
	_ = ctx // Mark as intentionally unused for now

	// Check file exists and get info
	info, err := os.Stat(fullFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Try to find similar file
			similar := findSimilarFile(fullFilePath)
			msg := fmt.Sprintf("File does not exist. Current working directory: %s", getCWD())
			if similar != "" {
				msg += fmt.Sprintf(" Did you mean %s?", similar)
			}
			return tool.Result{}, fmt.Errorf(msg)
		}
		return tool.Result{}, err
	}

	// Check if it's a directory
	if info.IsDir() {
		return tool.Result{}, fmt.Errorf("This tool can only read files, not directories. To read a directory, use an ls command via the Bash tool.")
	}

	// Check file size (256KB limit for text files)
	maxSize := int64(256 * 1024)
	if info.Size() > maxSize {
		return tool.Result{}, fmt.Errorf("File content (%d bytes) exceeds maximum allowed size (%d bytes). Use offset and limit parameters to read specific portions of the file", info.Size(), maxSize)
	}

	// Open file
	file, err := os.Open(fullFilePath)
	if err != nil {
		return tool.Result{}, err
	}
	defer file.Close()

	// Count total lines and read content
	scanner := bufio.NewScanner(file)
	// Increase buffer size for larger lines
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	totalLines := 0
	lines := make([]string, 0, limit)

	// Adjust offset (TS uses 1-indexed, Go uses 0-indexed)
	lineOffset := offset - 1
	if lineOffset < 0 {
		lineOffset = 0
	}

	for scanner.Scan() {
		totalLines++
		if totalLines <= lineOffset {
			continue
		}
		if len(lines) >= limit {
			break
		}
		// Format with line numbers (cat -n style)
		lines = append(lines, fmt.Sprintf("%d\t%s", totalLines, scanner.Text()))
	}

	if err := scanner.Err(); err != nil {
		return tool.Result{}, err
	}

	content := strings.Join(lines, "\n")
	numLines := len(lines)

	// Handle edge cases
	if numLines == 0 {
		if totalLines == 0 {
			return tool.Result{
				Content: "<system-reminder>Warning: the file exists but the contents are empty.</system-reminder>",
			}, nil
		}
		return tool.Result{
			Content: fmt.Sprintf("<system-reminder>Warning: the file exists but is shorter than the provided offset (%d). The file has %d lines.</system-reminder>", offset, totalLines),
		}, nil
	}

	result := FileReadResult{
		Type: "text",
		File: TextFileResult{
			FilePath:   filePath,
			Content:    content,
			NumLines:   numLines,
			StartLine:  offset,
			TotalLines: totalLines,
		},
	}

	return tool.Result{Content: result}, nil
}

func (t FileReadTool) readImage(fullFilePath, filePath string) (tool.Result, error) {
	// Read image file
	data, err := os.ReadFile(fullFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return tool.Result{}, fmt.Errorf("File does not exist: %s", filePath)
		}
		return tool.Result{}, err
	}

	if len(data) == 0 {
		return tool.Result{}, fmt.Errorf("Image file is empty: %s", filePath)
	}

	// Detect image type from content
	imgType := detectImageType(data)

	result := FileReadResult{
		Type: "image",
		File: ImageFileResult{
			Base64:       base64.StdEncoding.EncodeToString(data),
			Type:         imgType,
			OriginalSize: len(data),
		},
	}

	return tool.Result{Content: result}, nil
}

func (t FileReadTool) readNotebook(fullFilePath, filePath string) (tool.Result, error) {
	// Read notebook file
	data, err := os.ReadFile(fullFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return tool.Result{}, fmt.Errorf("File does not exist: %s", filePath)
		}
		return tool.Result{}, err
	}

	// Return raw content for notebooks - parsing would be done by caller
	result := FileReadResult{
		Type: "notebook",
		File: NotebookFileResult{
			FilePath: filePath,
			Cells:    []any{string(data)}, // Simplified representation
		},
	}

	return tool.Result{Content: result}, nil
}