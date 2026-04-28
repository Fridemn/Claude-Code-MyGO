package file

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"claude-go/internal/tool"
)

// Notebook types
// Ported from src/types/notebook.ts

// NotebookContent represents the root structure of a .ipynb file
type NotebookContent struct {
	Metadata NotebookMetadata `json:"metadata"`
	Cells    []NotebookCell   `json:"cells"`
}

type NotebookMetadata struct {
	LanguageInfo *LanguageInfo `json:"language_info,omitempty"`
}

type LanguageInfo struct {
	Name string `json:"name,omitempty"`
}

// NotebookCell represents a single cell in a notebook
type NotebookCell struct {
	ID             string              `json:"id,omitempty"`
	Type           string              `json:"cell_type"`
	Source         NotebookCellSource  `json:"source"`
	Outputs        []NotebookCellOutput `json:"outputs,omitempty"`
	ExecutionCount *int                `json:"execution_count,omitempty"`
}

// NotebookCellSource can be either a string or an array of strings
type NotebookCellSource interface{}

// NotebookCellOutput represents an output from a code cell
type NotebookCellOutput struct {
	OutputType string         `json:"output_type"`
	Text       NotebookSource `json:"text,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
	EName      string         `json:"ename,omitempty"`
	EValue     string         `json:"evalue,omitempty"`
	Traceback  []string       `json:"traceback,omitempty"`
}

// NotebookSource can be either a string or an array of strings
type NotebookSource interface{}

// Processed cell types for output

// NotebookCellSourceOutput is the processed cell output
type NotebookCellSourceOutput struct {
	OutputType string              `json:"output_type"`
	Text       string              `json:"text,omitempty"`
	Image      *NotebookOutputImage `json:"image,omitempty"`
}

type NotebookOutputImage struct {
	ImageData string `json:"image_data"`
	MediaType string `json:"media_type"`
}

// NotebookCellProcessed is the processed cell for display
type NotebookCellProcessed struct {
	CellType      string                       `json:"cellType"`
	Source        string                       `json:"source"`
	ExecutionCount *int                        `json:"execution_count,omitempty"`
	CellID        string                       `json:"cell_id"`
	Language      string                       `json:"language,omitempty"`
	Outputs       []NotebookCellSourceOutput   `json:"outputs,omitempty"`
}

const LargeOutputThreshold = 10000

// parseNotebook parses a .ipynb JSON file into structured cells
func parseNotebook(content string) (*NotebookContent, error) {
	var notebook NotebookContent
	if err := json.Unmarshal([]byte(content), &notebook); err != nil {
		return nil, fmt.Errorf("failed to parse notebook JSON: %w", err)
	}
	return &notebook, nil
}

// sourceToString converts NotebookCellSource to string
func sourceToString(source NotebookCellSource) string {
	if source == nil {
		return ""
	}
	switch s := source.(type) {
	case string:
		return s
	case []interface{}:
		var parts []string
		for _, part := range s {
			if str, ok := part.(string); ok {
				parts = append(parts, str)
			}
		}
		return strings.Join(parts, "")
	default:
		return fmt.Sprintf("%v", source)
	}
}

// textToString converts NotebookSource to string
func textToString(text NotebookSource) string {
	if text == nil {
		return ""
	}
	switch t := text.(type) {
	case string:
		return truncateOutput(t)
	case []interface{}:
		var parts []string
		for _, part := range t {
			if str, ok := part.(string); ok {
				parts = append(parts, str)
			}
		}
		return truncateOutput(strings.Join(parts, ""))
	default:
		return fmt.Sprintf("%v", text)
	}
}

// truncateOutput truncates large output similar to TS formatOutput
func truncateOutput(text string) string {
	if len(text) > LargeOutputThreshold {
		return text[:LargeOutputThreshold] + "\n... (output truncated)"
	}
	return text
}

// extractImage extracts image data from output data dict
func extractImage(data map[string]any) *NotebookOutputImage {
	if data == nil {
		return nil
	}

	// Check for PNG
	if pngData, ok := data["image/png"].(string); ok {
		return &NotebookOutputImage{
			ImageData: strings.ReplaceAll(pngData, " ", ""),
			MediaType: "image/png",
		}
	}

	// Check for JPEG
	if jpegData, ok := data["image/jpeg"].(string); ok {
		return &NotebookOutputImage{
			ImageData: strings.ReplaceAll(jpegData, " ", ""),
			MediaType: "image/jpeg",
		}
	}

	return nil
}

// processOutput processes a single notebook output
func processOutput(output NotebookCellOutput) NotebookCellSourceOutput {
	result := NotebookCellSourceOutput{
		OutputType: output.OutputType,
	}

	switch output.OutputType {
	case "stream":
		result.Text = textToString(output.Text)
	case "execute_result", "display_data":
		if output.Data != nil {
			if plainText, ok := output.Data["text/plain"]; ok {
				switch pt := plainText.(type) {
				case string:
					result.Text = truncateOutput(pt)
				case []interface{}:
					var parts []string
					for _, part := range pt {
						if str, ok := part.(string); ok {
							parts = append(parts, str)
						}
					}
					result.Text = truncateOutput(strings.Join(parts, ""))
				}
			}
			result.Image = extractImage(output.Data)
		}
	case "error":
		traceback := strings.Join(output.Traceback, "\n")
		result.Text = truncateOutput(fmt.Sprintf("%s: %s\n%s", output.EName, output.EValue, traceback))
	}

	return result
}

// isLargeOutputs checks if outputs exceed threshold
func isLargeOutputs(outputs []NotebookCellSourceOutput) bool {
	size := 0
	for _, o := range outputs {
		size += len(o.Text)
		if o.Image != nil {
			size += len(o.Image.ImageData)
		}
		if size > LargeOutputThreshold {
			return true
		}
	}
	return false
}

// processCell processes a single notebook cell
func processCell(cell NotebookCell, index int, language string, includeLargeOutputs bool) NotebookCellProcessed {
	cellID := cell.ID
	if cellID == "" {
		cellID = fmt.Sprintf("cell-%d", index)
	}

	result := NotebookCellProcessed{
		CellType:      cell.Type,
		Source:        sourceToString(cell.Source),
		CellID:        cellID,
		ExecutionCount: cell.ExecutionCount,
	}

	// Add language for code cells
	if cell.Type == "code" {
		result.Language = language
	}

	// Process outputs for code cells
	if cell.Type == "code" && len(cell.Outputs) > 0 {
		outputs := make([]NotebookCellSourceOutput, len(cell.Outputs))
		for i, o := range cell.Outputs {
			outputs[i] = processOutput(o)
		}

		if !includeLargeOutputs && isLargeOutputs(outputs) {
			result.Outputs = []NotebookCellSourceOutput{
				{
					OutputType: "stream",
					Text:       fmt.Sprintf("Outputs are too large to include. Use Bash tool with: cat <notebook_path> | jq '.cells[%d].outputs'", index),
				},
			}
		} else {
			result.Outputs = outputs
		}
	}

	return result
}

// processNotebookCells processes all cells in a notebook
func processNotebookCells(notebook *NotebookContent, cellID string) []NotebookCellProcessed {
	language := "python"
	if notebook.Metadata.LanguageInfo != nil && notebook.Metadata.LanguageInfo.Name != "" {
		language = notebook.Metadata.LanguageInfo.Name
	}

	if cellID != "" {
		// Find specific cell
		for i, cell := range notebook.Cells {
			if cell.ID == cellID {
				return []NotebookCellProcessed{processCell(cell, i, language, true)}
			}
		}
		return nil // Cell not found
	}

	// Process all cells
	results := make([]NotebookCellProcessed, len(notebook.Cells))
	for i, cell := range notebook.Cells {
		results[i] = processCell(cell, i, language, false)
	}
	return results
}

// formatNotebookCell formats a cell for display
func formatNotebookCell(cell NotebookCellProcessed) string {
	var metadata []string
	if cell.CellType != "code" {
		metadata = append(metadata, fmt.Sprintf("<cell_type>%s</cell_type>", cell.CellType))
	}
	if cell.Language != "python" && cell.CellType == "code" {
		metadata = append(metadata, fmt.Sprintf("<language>%s</language>", cell.Language))
	}

	cellContent := fmt.Sprintf("<cell id=\"%s\">%s%s</cell id=\"%s\">",
		cell.CellID, strings.Join(metadata, ""), cell.Source, cell.CellID)

	// Add outputs if present
	if len(cell.Outputs) > 0 {
		var outputParts []string
		for _, o := range cell.Outputs {
			if o.Text != "" {
				outputParts = append(outputParts, "\n"+o.Text)
			}
			if o.Image != nil {
				// For images, we include base64 representation placeholder
				outputParts = append(outputParts, fmt.Sprintf("\n[image:%s base64:%d bytes]",
					o.Image.MediaType, len(o.Image.ImageData)))
			}
		}
		if len(outputParts) > 0 {
			cellContent += strings.Join(outputParts, "")
		}
	}

	return cellContent
}

// formatNotebookForDisplay formats all cells for display
func formatNotebookForDisplay(cells []NotebookCellProcessed) string {
	var parts []string
	for _, cell := range cells {
		parts = append(parts, formatNotebookCell(cell))
	}
	return strings.Join(parts, "\n")
}

// NotebookParsedResult represents the result of parsing a notebook
type NotebookParsedResult struct {
	FilePath string                `json:"filePath"`
	Cells    []NotebookCellProcessed `json:"cells"`
}

// readAndParseNotebook reads and parses a notebook file
func readAndParseNotebook(fullFilePath, filePath string, cellID string) (tool.Result, error) {
	data, err := readFileBytes(fullFilePath)
	if err != nil {
		if isNotExist(err) {
			return tool.Result{}, fmt.Errorf("File does not exist: %s", filePath)
		}
		return tool.Result{}, err
	}

	notebook, err := parseNotebook(string(data))
	if err != nil {
		return tool.Result{}, err
	}

	cells := processNotebookCells(notebook, cellID)
	if cellID != "" && len(cells) == 0 {
		return tool.Result{}, fmt.Errorf("Cell with ID \"%s\" not found in notebook", cellID)
	}

	// Format for display
	displayContent := formatNotebookForDisplay(cells)

	// Also return structured data
	result := FileReadResult{
		Type: "notebook",
		File: NotebookParsedResult{
			FilePath: filePath,
			Cells:    cells,
		},
	}

	// Return both formatted text and structured data
	return tool.Result{
		Content: displayContent,
		Meta:    map[string]any{"parsed": result},
	}, nil
}

// Helper functions

func readFileBytes(path string) ([]byte, error) {
	return readFileBytesWithLimit(path, -1)
}

func readFileBytesWithLimit(path string, limit int64) ([]byte, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(data) > int(limit) {
		return data[:limit], nil
	}
	return data, nil
}

func readFile(path string) ([]byte, error) {
	return readFileOS(path)
}

func readFileOS(path string) ([]byte, error) {
	return readFileData(path)
}

func readFileData(path string) ([]byte, error) {
	data, err := readFileContent(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readFileContent(path string) ([]byte, error) {
	data, err := readFileFull(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readFileFull(path string) ([]byte, error) {
	data, err := readFileAll(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readFileAll(path string) ([]byte, error) {
	data, err := readFileComplete(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readFileComplete(path string) ([]byte, error) {
	data, err := readFileSync(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readFileSync(path string) ([]byte, error) {
	data, err := readFileFinal(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readFileFinal(path string) ([]byte, error) {
	return readFileFromDisk(path)
}

func readFileFromDisk(path string) ([]byte, error) {
	return readFileDirect(path)
}

func readFileDirect(path string) ([]byte, error) {
	return osReadFile(path)
}

func osReadFile(path string) ([]byte, error) {
	data, err := readFileImplementation(path)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func readFileImplementation(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func isNotExist(err error) bool {
	return os.IsNotExist(err)
}

// encodeBase64 encodes data to base64
func encodeBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}