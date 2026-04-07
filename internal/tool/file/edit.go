package file

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-code-go/internal/tool"
)

// FileEditTool implements the Edit tool from TypeScript
type FileEditTool struct{}

func (FileEditTool) Name() string { return FileEditToolName }

func (FileEditTool) Description() string {
	return `Performs exact string replacements in files.

Usage:
- You must use your Read tool at least once in the conversation before editing. This tool will error if you attempt an edit without reading the file.
- When editing text from Read tool output, ensure you preserve the exact indentation (tabs/spaces) as it appears AFTER the line number prefix. The line number prefix format is: line number + tab. Everything after that is the actual file content to match. Never include any part of the line number prefix in the old_string or new_string.
- ALWAYS prefer editing existing files in the codebase. NEVER write new files unless explicitly required.
- Only use emojis if the user explicitly requests it. Avoid adding emojis to files unless asked.
- The edit will FAIL if old_string is not unique in the file. Either provide a larger string with more surrounding context to make it unique or use replace_all to change every instance of old_string.
- Use replace_all for replacing and renaming strings across the file. This parameter is useful if you want to rename a variable for instance.`
}

func (FileEditTool) IsReadOnly(tool.Input) bool { return false }

func (FileEditTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"file_path":   tool.SchemaString("The absolute path to the file to modify"),
		"old_string":  tool.SchemaString("The text to replace"),
		"new_string":  tool.SchemaString("The text to replace it with (must be different from old_string)"),
		"replace_all": tool.SchemaBoolean("Replace all occurrences of old_string (default false)"),
	}, "file_path", "old_string", "new_string")
}

// FileEditResult represents the result of editing a file
type FileEditResult struct {
	FilePath        string           `json:"filePath"`
	OldString       string           `json:"oldString"`
	NewString       string           `json:"newString"`
	OriginalFile    string           `json:"originalFile"`
	StructuredPatch []StructuredHunk `json:"structuredPatch"`
	UserModified    bool             `json:"userModified"`
	ReplaceAll      bool             `json:"replaceAll"`
}

func (t FileEditTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	filePath := tool.GetString(in, "file_path")
	oldString := tool.GetString(in, "old_string")
	newString := tool.GetString(in, "new_string")
	replaceAll := tool.GetBool(in, "replace_all")

	if filePath == "" {
		return tool.Result{}, fmt.Errorf("file_path is required")
	}

	// Check that old_string and new_string are different
	if oldString == newString {
		return tool.Result{}, fmt.Errorf("No changes to make: old_string and new_string are exactly the same")
	}

	// Expand and clean the path
	fullFilePath := expandPath(filePath)

	// Read existing file
	originalContent, fileExists, err := readFileIfExists(fullFilePath)
	if err != nil {
		return tool.Result{}, err
	}

	// Handle file not existing
	if !fileExists {
		if oldString == "" {
			// Creating new file with empty old_string is valid
			// Ensure parent directory exists
			dir := filepath.Dir(fullFilePath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return tool.Result{}, fmt.Errorf("failed to create parent directory: %w", err)
			}
			if err := os.WriteFile(fullFilePath, []byte(newString), 0644); err != nil {
				return tool.Result{}, fmt.Errorf("failed to write file: %w", err)
			}

			result := FileEditResult{
				FilePath:        filePath,
				OldString:       "",
				NewString:       newString,
				OriginalFile:    "",
				StructuredPatch: []StructuredHunk{},
				UserModified:    false,
				ReplaceAll:      false,
			}
			return tool.Result{Content: result}, nil
		}

		// Try to find similar file
		similar := findSimilarFile(fullFilePath)
		msg := fmt.Sprintf("File does not exist. Current working directory: %s", getCWD())
		if similar != "" {
			msg += fmt.Sprintf(" Did you mean %s?", similar)
		}
		return tool.Result{}, fmt.Errorf(msg)
	}

	// Handle empty old_string for existing files
	if oldString == "" {
		if strings.TrimSpace(originalContent) != "" {
			return tool.Result{}, fmt.Errorf("Cannot create new file - file already exists")
		}
		// Empty file with empty old_string is valid - we're replacing empty with content
	}

	// Handle notebook files
	if filepath.Ext(fullFilePath) == ".ipynb" {
		return tool.Result{}, fmt.Errorf("File is a Jupyter Notebook. Use the NotebookEditTool to edit this file")
	}

	// Find the actual string in file (handle quote normalization)
	actualOldString := findActualString(originalContent, oldString)
	if actualOldString == "" {
		return tool.Result{}, fmt.Errorf("String to replace not found in file.\nString: %s", oldString)
	}

	// Count matches
	matches := countMatches(originalContent, actualOldString)

	// Check if we have multiple matches but replace_all is false
	if matches > 1 && !replaceAll {
		return tool.Result{}, fmt.Errorf("Found %d matches of the string to replace, but replace_all is false. To replace all occurrences, set replace_all to true. To replace only one occurrence, please provide more context to uniquely identify the instance.\nString: %s", matches, oldString)
	}

	// Perform the replacement
	var updatedContent string
	if replaceAll {
		updatedContent = strings.ReplaceAll(originalContent, actualOldString, newString)
	} else {
		updatedContent = strings.Replace(originalContent, actualOldString, newString, 1)
	}

	// Check if replacement happened
	if updatedContent == originalContent {
		return tool.Result{}, fmt.Errorf("String not found in file. Failed to apply edit")
	}

	// Write the updated content
	if err := os.WriteFile(fullFilePath, []byte(updatedContent), 0644); err != nil {
		return tool.Result{}, fmt.Errorf("failed to write file: %w", err)
	}

	// Generate patch
	patch := generateEditPatch(originalContent, updatedContent, actualOldString)

	result := FileEditResult{
		FilePath:        filePath,
		OldString:       actualOldString,
		NewString:       newString,
		OriginalFile:    originalContent,
		StructuredPatch: patch,
		UserModified:    false,
		ReplaceAll:      replaceAll,
	}

	return tool.Result{Content: result}, nil
}

// generateEditPatch creates a patch representation for an edit
func generateEditPatch(oldContent, newContent, oldString string) []StructuredHunk {
	// Find where the change occurred
	oldLines := strings.Split(oldContent, "\n")
	newLines := strings.Split(newContent, "\n")

	// Simple approach: find the first changed position
	changeStart := 0
	for i := 0; i < len(oldLines) && i < len(newLines); i++ {
		if oldLines[i] != newLines[i] {
			changeStart = i
			break
		}
	}

	// Find where changes end
	changeEndOld := len(oldLines)
	changeEndNew := len(newLines)
	for i := changeStart; i < len(oldLines) && i < len(newLines); i++ {
		if oldLines[i] == newLines[i] {
			changeEndOld = i
			changeEndNew = i
			break
		}
	}

	// Build the hunk
	lines := make([]string, 0)

	// Add context (up to 4 lines before)
	contextStart := changeStart - 4
	if contextStart < 0 {
		contextStart = 0
	}

	for i := contextStart; i < changeStart; i++ {
		lines = append(lines, " "+oldLines[i])
	}

	// Add old lines (deleted)
	for i := changeStart; i < changeEndOld && i < len(oldLines); i++ {
		lines = append(lines, "-"+oldLines[i])
	}

	// Add new lines (added)
	for i := changeStart; i < changeEndNew && i < len(newLines); i++ {
		lines = append(lines, "+"+newLines[i])
	}

	// Add context (up to 4 lines after)
	contextEnd := changeEndOld + 4
	if contextEnd > len(newLines) {
		contextEnd = len(newLines)
	}
	for i := changeEndNew; i < contextEnd && i < len(newLines); i++ {
		lines = append(lines, " "+newLines[i])
	}

	return []StructuredHunk{
		{
			OldStart: contextStart + 1, // 1-indexed
			OldLines: changeEndOld - contextStart,
			NewStart: contextStart + 1,
			NewLines: changeEndNew - contextStart,
			Lines:    lines,
		},
	}
}