package file

import (
	"os"
	"path/filepath"
	"strings"
)

// expandPath expands ~ and environment variables in a path
func expandPath(path string) string {
	// Trim whitespace
	path = strings.TrimSpace(path)

	// Handle home directory expansion
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}

	// Clean the path
	return filepath.Clean(path)
}

// getCWD returns the current working directory
func getCWD() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

// findSimilarFile tries to find a similar file with different extension
func findSimilarFile(path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(path)

	// Try common extensions
	commonExts := []string{".ts", ".tsx", ".js", ".jsx", ".go", ".py", ".json", ".yaml", ".yml", ".md"}
	for _, tryExt := range commonExts {
		if tryExt == ext {
			continue
		}
		tryPath := filepath.Join(dir, strings.TrimSuffix(base, ext)+tryExt)
		if _, err := os.Stat(tryPath); err == nil {
			return tryPath
		}
	}

	return ""
}

// detectImageType detects image MIME type from content
func detectImageType(data []byte) string {
	if len(data) < 4 {
		return "image/png" // default
	}

	// PNG: 89 50 4E 47
	if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E && data[3] == 0x47 {
		return "image/png"
	}

	// JPEG: FF D8 FF
	if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
		return "image/jpeg"
	}

	// GIF: 47 49 46 38
	if data[0] == 0x47 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x38 {
		return "image/gif"
	}

	// WebP: 52 49 46 46 ... 57 45 42 50
	if len(data) >= 12 && data[0] == 0x52 && data[1] == 0x49 && data[2] == 0x46 && data[3] == 0x46 &&
		data[8] == 0x57 && data[9] == 0x45 && data[10] == 0x42 && data[11] == 0x50 {
		return "image/webp"
	}

	return "image/png" // default
}

// readFileIfExists reads a file if it exists, returns content, exists flag, and error
func readFileIfExists(path string) (string, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", false, nil
		}
		return "", false, err
	}
	return string(data), true, nil
}

// findActualString finds the actual string in file content, handling quote normalization
func findActualString(fileContent, searchString string) string {
	// First try exact match
	if strings.Contains(fileContent, searchString) {
		return searchString
	}

	// Try with normalized quotes (curly to straight)
	normalizedSearch := normalizeQuotes(searchString)
	normalizedFile := normalizeQuotes(fileContent)

	// Find position in normalized content
	idx := strings.Index(normalizedFile, normalizedSearch)
	if idx == -1 {
		return ""
	}

	// Return the actual string from the file
	return fileContent[idx:idx+len(searchString)]
}

// normalizeQuotes converts curly quotes to straight quotes
func normalizeQuotes(str string) string {
	// Unicode curly quotes
	str = strings.ReplaceAll(str, "\u2018", "'") // Left single curly quote
	str = strings.ReplaceAll(str, "\u2019", "'") // Right single curly quote
	str = strings.ReplaceAll(str, "\u201C", "\"") // Left double curly quote
	str = strings.ReplaceAll(str, "\u201D", "\"") // Right double curly quote
	return str
}

// countMatches counts the number of occurrences of a string
func countMatches(content, search string) int {
	if search == "" {
		return 0
	}
	return strings.Count(content, search)
}