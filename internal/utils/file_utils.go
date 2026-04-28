package utils

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
)

// LineEndingType represents the line ending style of a file
type LineEndingType string

const (
	LineEndingLF   LineEndingType = "LF"
	LineEndingCRLF LineEndingType = "CRLF"
)

// EncodingType represents file encoding
type EncodingType string

const (
	EncodingUTF8   EncodingType = "utf8"
	EncodingUTF16LE EncodingType = "utf16le"
	EncodingASCII  EncodingType = "ascii"
	EncodingLatin1 EncodingType = "latin1"
)

// DetectFileEncoding detects the encoding of a file by reading its BOM and content
// Returns utf8 as default for empty files or when no BOM is detected
func DetectFileEncoding(filePath string) EncodingType {
	f, err := os.Open(filePath)
	if err != nil {
		return EncodingUTF8
	}
	defer f.Close()

	// Read first 4KB to detect encoding
	buf := make([]byte, 4096)
	n, err := f.Read(buf)
	if err != nil && err != os.ErrClosed {
		return EncodingUTF8
	}
	buf = buf[:n]

	// Empty files default to utf8
	if n == 0 {
		return EncodingUTF8
	}

	// Check for BOM
	if n >= 2 {
		// UTF-16 LE BOM: FF FE
		if buf[0] == 0xFF && buf[1] == 0xFE {
			return EncodingUTF16LE
		}
	}

	if n >= 3 {
		// UTF-8 BOM: EF BB BF
		if buf[0] == 0xEF && buf[1] == 0xBB && buf[2] == 0xBF {
			return EncodingUTF8
		}
	}

	// Default to utf8 since it's a superset of ascii
	return EncodingUTF8
}

// DetectLineEndings detects the line ending style of a file
func DetectLineEndings(filePath string, encoding EncodingType) LineEndingType {
	f, err := os.Open(filePath)
	if err != nil {
		return LineEndingLF
	}
	defer f.Close()

	// Read first 4KB to detect line endings
	buf := make([]byte, 4096)
	n, err := f.Read(buf)
	if err != nil && err != os.ErrClosed {
		return LineEndingLF
	}
	buf = buf[:n]

	// Convert to string based on encoding
	var content string
	switch encoding {
	case EncodingUTF16LE:
		// Simple UTF-16 LE to string conversion for detection
		// For now, just check raw bytes for \r\n pattern
		return detectLineEndingsFromBytes(buf)
	default:
		content = string(buf)
	}

	return DetectLineEndingsForString(content)
}

// DetectLineEndingsForString detects line ending style from string content
func DetectLineEndingsForString(content string) LineEndingType {
	crlfCount := 0
	lfCount := 0

	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			if i > 0 && content[i-1] == '\r' {
				crlfCount++
			} else {
				lfCount++
			}
		}
	}

	if crlfCount > lfCount {
		return LineEndingCRLF
	}
	return LineEndingLF
}

// detectLineEndingsFromBytes detects line endings from raw bytes
func detectLineEndingsFromBytes(buf []byte) LineEndingType {
	crlfCount := bytes.Count(buf, []byte("\r\n"))
	lfOnlyCount := bytes.Count(buf, []byte("\n")) - crlfCount

	if crlfCount > lfOnlyCount {
		return LineEndingCRLF
	}
	return LineEndingLF
}

// WriteTextContent writes content to file with proper encoding and line endings
func WriteTextContent(filePath string, content string, encoding EncodingType, lineEndings LineEndingType) error {
	toWrite := content

	// Convert line endings if CRLF requested
	if lineEndings == LineEndingCRLF {
		// Normalize any existing CRLF to LF first
		toWrite = strings.ReplaceAll(toWrite, "\r\n", "\n")
		// Then convert LF to CRLF
		toWrite = strings.ReplaceAll(toWrite, "\n", "\r\n")
	}

	// Handle encoding
	var data []byte
	switch encoding {
	case EncodingUTF16LE:
		// Convert UTF-8 to UTF-16 LE
		data = utf8ToUTF16LE(toWrite)
	default:
		data = []byte(toWrite)
	}

	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0644)
}

// utf8ToUTF16LE converts UTF-8 string to UTF-16 LE bytes
func utf8ToUTF16LE(s string) []byte {
	// Simple conversion - each rune becomes 2 bytes (LE)
	// This is a simplified implementation
	runes := []rune(s)
	result := make([]byte, 0, len(runes)*2)
	for _, r := range runes {
		if r <= 0xFFFF {
			// BMP character - 2 bytes
			result = append(result, byte(r), byte(r>>8))
		} else {
			// Surrogate pair - simplified handling
			r -= 0x10000
			high := 0xD800 + (r >> 10)
			low := 0xDC00 + (r & 0x3FF)
			result = append(result, byte(high), byte(high>>8), byte(low), byte(low>>8))
		}
	}
	return result
}

// GetFileModificationTime returns the modification time of a file in milliseconds
func GetFileModificationTime(filePath string) int64 {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0
	}
	return info.ModTime().UnixMilli()
}

// ReadFileWithMetadata reads a file and returns content with detected encoding and line endings
func ReadFileWithMetadata(filePath string) (content string, encoding EncodingType, lineEndings LineEndingType, err error) {
	// Detect encoding first
	encoding = DetectFileEncoding(filePath)

	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", EncodingUTF8, LineEndingLF, err
	}

	// Convert to string based on encoding
	switch encoding {
	case EncodingUTF16LE:
		content = utf16LEToUTF8(data)
	default:
		content = string(data)
	}

	// Detect line endings from first 4KB
	sampleLen := 4096
	if len(content) < sampleLen {
		sampleLen = len(content)
	}
	lineEndings = DetectLineEndingsForString(content[:sampleLen])

	// Normalize line endings to LF for returned content
	content = strings.ReplaceAll(content, "\r\n", "\n")

	return content, encoding, lineEndings, nil
}

// utf16LEToUTF8 converts UTF-16 LE bytes to UTF-8 string
func utf16LEToUTF8(data []byte) string {
	// Skip BOM if present
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		data = data[2:]
	}

	// Convert pairs of bytes to runes
	runes := make([]rune, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		r := rune(data[i]) | (rune(data[i+1]) << 8)
		runes = append(runes, r)
	}

	return string(runes)
}

// PathExists checks if a path exists
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// GetConfigDir returns the Claude Code Go config directory.
// Uses XDG_CONFIG_HOME env var if set, otherwise ~/.claude-go.
// This matches the TS getClaudeConfigHomeDir() behavior.
// Ported from src/utils/envUtils.ts:getClaudeConfigHomeDir
func GetConfigDir() string {
	// Check XDG_CONFIG_HOME first (Linux/BSD standard)
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "claude-go")
	}

	// Fall back to ~/.config/claude-go on Unix-like systems
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "claude-go")
	}

	// Last resort: ~/.claude-go
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".claude-go")
	}

	return ".claude-go"
}
