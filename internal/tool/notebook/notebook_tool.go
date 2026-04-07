package notebook

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-code-go/internal/tool"
)

// NotebookEditToolName is the name of the notebook edit tool
const NotebookEditToolName = "NotebookEdit"

// NotebookEditDescription describes the notebook edit tool
const NotebookEditDescription = `Replace the contents of a specific cell in a Jupyter notebook (.ipynb file) with new source. Jupyter notebooks are interactive documents that combine code, text, and visualizations, commonly used for data analysis and scientific computing. The notebook_path parameter must be an absolute path, not a relative path. The cell_number is 0-indexed. Use edit_mode=insert to add a new cell at the index specified by cell_number. Use edit_mode=delete to delete the cell at the index specified by cell_number.`

// Edit modes
const (
	EditModeReplace = "replace"
	EditModeInsert  = "insert"
	EditModeDelete  = "delete"
)

// Cell types
const (
	CellTypeCode     = "code"
	CellTypeMarkdown = "markdown"
)

// NotebookContent represents the structure of a Jupyter notebook
type NotebookContent struct {
	NBFormat      int              `json:"nbformat"`
	NBFormatMinor int              `json:"nbformat_minor"`
	Metadata      NotebookMetadata `json:"metadata"`
	Cells         []NotebookCell   `json:"cells"`
}

// NotebookMetadata represents notebook metadata
type NotebookMetadata struct {
	LanguageInfo *LanguageInfo `json:"language_info,omitempty"`
	Kernelspec   *KernelSpec   `json:"kernelspec,omitempty"`
}

// LanguageInfo represents language information
type LanguageInfo struct {
	Name string `json:"name"`
}

// KernelSpec represents kernel specification
type KernelSpec struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Language    string `json:"language"`
}

// NotebookCell represents a cell in a notebook
type NotebookCell struct {
	CellType       string         `json:"cell_type"`
	ID             string         `json:"id,omitempty"`
	Source         any            `json:"source"` // Can be string or []string
	Metadata       map[string]any `json:"metadata"`
	ExecutionCount *int           `json:"execution_count,omitempty"`
	Outputs        []any          `json:"outputs,omitempty"`
}

// NotebookEditTool implements the notebook edit tool
type NotebookEditTool struct{}

// Name returns the tool name
func (NotebookEditTool) Name() string { return NotebookEditToolName }

// Description returns the tool description
func (NotebookEditTool) Description() string { return NotebookEditDescription }

// IsReadOnly returns false as this tool modifies files
func (NotebookEditTool) IsReadOnly(tool.Input) bool { return false }

// ParametersSchema returns the JSON schema for the tool parameters
func (NotebookEditTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"notebook_path": tool.SchemaString("The absolute path to the Jupyter notebook file to edit (must be absolute, not relative)"),
		"cell_id":       tool.SchemaString("The ID of the cell to edit. When inserting a new cell, the new cell will be inserted after the cell with this ID, or at the beginning if not specified."),
		"new_source":    tool.SchemaString("The new source for the cell"),
		"cell_type":     tool.SchemaEnumString("The type of the cell (code or markdown). If not specified, it defaults to the current cell type. If using edit_mode=insert, this is required.", CellTypeCode, CellTypeMarkdown),
		"edit_mode":     tool.SchemaEnumString("The type of edit to make (replace, insert, delete). Defaults to replace.", EditModeReplace, EditModeInsert, EditModeDelete),
	}, "notebook_path", "new_source")
}

// Call executes the notebook edit tool
func (t NotebookEditTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	notebookPath := getString(in, "notebook_path")
	cellID := getString(in, "cell_id")
	newSource := getString(in, "new_source")
	cellType := getString(in, "cell_type")
	editMode := getString(in, "edit_mode")

	// Default edit mode is replace
	if editMode == "" {
		editMode = EditModeReplace
	}

	// Default cell type is code
	if cellType == "" {
		cellType = CellTypeCode
	}

	// Validate
	if notebookPath == "" {
		return tool.Result{}, fmt.Errorf("notebook_path is required")
	}
	if editMode != EditModeDelete && newSource == "" {
		return tool.Result{}, fmt.Errorf("new_source is required")
	}

	// Get absolute path
	absPath := notebookPath
	if !filepath.IsAbs(notebookPath) {
		if runtime.Store != nil {
			absPath = filepath.Join(runtime.Store.GetCWD(), notebookPath)
		} else {
			absPath, _ = filepath.Abs(notebookPath)
		}
	}

	// Check file extension
	if filepath.Ext(absPath) != ".ipynb" {
		return tool.Result{Error: "File must be a Jupyter notebook (.ipynb file). For editing other file types, use the FileEdit tool."}, nil
	}

	// Read notebook file
	data, err := os.ReadFile(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			return tool.Result{Error: "Notebook file does not exist."}, nil
		}
		return tool.Result{}, fmt.Errorf("failed to read notebook: %w", err)
	}

	// Parse notebook
	var notebook NotebookContent
	if err := json.Unmarshal(data, &notebook); err != nil {
		return tool.Result{Error: "Notebook is not valid JSON."}, nil
	}

	// Find cell index
	cellIndex := -1
	if cellID != "" {
		for i, cell := range notebook.Cells {
			if cell.ID == cellID {
				cellIndex = i
				break
			}
		}
		// Try parsing as numeric index (cell-N format)
		if cellIndex == -1 && strings.HasPrefix(cellID, "cell-") {
			var idx int
			if _, err := fmt.Sscanf(cellID, "cell-%d", &idx); err == nil {
				if idx >= 0 && idx < len(notebook.Cells) {
					cellIndex = idx
				}
			}
		}
	} else if editMode == EditModeInsert {
		cellIndex = 0 // Insert at beginning
	}

	// Handle different edit modes
	switch editMode {
	case EditModeDelete:
		if cellIndex == -1 {
			return tool.Result{Error: fmt.Sprintf("Cell with ID '%s' not found in notebook.", cellID)}, nil
		}
		// Delete the cell
		notebook.Cells = append(notebook.Cells[:cellIndex], notebook.Cells[cellIndex+1:]...)

	case EditModeInsert:
		if cellIndex >= 0 && cellIndex < len(notebook.Cells) {
			cellIndex++ // Insert after the specified cell
		}
		// Create new cell
		newCell := NotebookCell{
			CellType: cellType,
			Source:   newSource,
			Metadata: make(map[string]any),
		}
		if cellType == CellTypeCode {
			newCell.ExecutionCount = nil
			newCell.Outputs = []any{}
		}
		// Generate ID for nbformat >= 4.5
		if notebook.NBFormat > 4 || (notebook.NBFormat == 4 && notebook.NBFormatMinor >= 5) {
			newCell.ID = generateCellID()
		}
		// Insert the cell
		notebook.Cells = append(
			notebook.Cells[:cellIndex],
			append([]NotebookCell{newCell}, notebook.Cells[cellIndex:]...)...,
		)

	case EditModeReplace:
		if cellIndex == -1 {
			// Check if trying to replace at the end (convert to insert)
			if cellID != "" {
				return tool.Result{Error: fmt.Sprintf("Cell with ID '%s' not found in notebook.", cellID)}, nil
			}
			return tool.Result{Error: "Cell ID must be specified for replace mode."}, nil
		}
		// Update the cell
		notebook.Cells[cellIndex].Source = newSource
		if cellType != "" && cellType != notebook.Cells[cellIndex].CellType {
			notebook.Cells[cellIndex].CellType = cellType
		}
		if notebook.Cells[cellIndex].CellType == CellTypeCode {
			notebook.Cells[cellIndex].ExecutionCount = nil
			notebook.Cells[cellIndex].Outputs = []any{}
		}
	}

	// Write back to file
	output, err := json.MarshalIndent(notebook, "", " ")
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to marshal notebook: %w", err)
	}
	if err := os.WriteFile(absPath, output, 0644); err != nil {
		return tool.Result{}, fmt.Errorf("failed to write notebook: %w", err)
	}

	var resultMsg string
	switch editMode {
	case EditModeDelete:
		resultMsg = fmt.Sprintf("Deleted cell %s", cellID)
	case EditModeInsert:
		resultMsg = "Inserted cell with new source"
	case EditModeReplace:
		resultMsg = fmt.Sprintf("Updated cell %s", cellID)
	}

	return tool.Result{
		Content: resultMsg,
		Meta: map[string]any{
			"edit_mode":     editMode,
			"cell_id":       cellID,
			"cell_type":     cellType,
			"notebook_path": absPath,
		},
	}, nil
}

// generateCellID generates a random cell ID
func generateCellID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 10)
	for i := range b {
		b[i] = chars[i%len(chars)]
	}
	return string(b)
}

// RegisterNotebookTools registers notebook tools to the registry
func RegisterNotebookTools(r *tool.Registry) {
	r.Register(NotebookEditTool{})
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