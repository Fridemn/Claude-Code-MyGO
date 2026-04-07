package tests

import (
	"os"
	"path/filepath"
	"testing"

	"claude-code-go/internal/utils"
)

func TestDetectFileEncoding(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "encoding-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test UTF-8 without BOM
	utf8File := filepath.Join(tmpDir, "utf8.txt")
	os.WriteFile(utf8File, []byte("hello world"), 0644)
	encoding := utils.DetectFileEncoding(utf8File)
	if encoding != utils.EncodingUTF8 {
		t.Errorf("Expected UTF-8 encoding, got %s", encoding)
	}

	// Test UTF-8 with BOM
	utf8BOMFile := filepath.Join(tmpDir, "utf8bom.txt")
	os.WriteFile(utf8BOMFile, []byte{0xEF, 0xBB, 0xBF, 'h', 'e', 'l', 'l', 'o'}, 0644)
	encoding = utils.DetectFileEncoding(utf8BOMFile)
	if encoding != utils.EncodingUTF8 {
		t.Errorf("Expected UTF-8 encoding for BOM file, got %s", encoding)
	}

	// Test UTF-16 LE
	utf16leFile := filepath.Join(tmpDir, "utf16le.txt")
	os.WriteFile(utf16leFile, []byte{0xFF, 0xFE, 'h', 0x00, 'i', 0x00}, 0644)
	encoding = utils.DetectFileEncoding(utf16leFile)
	if encoding != utils.EncodingUTF16LE {
		t.Errorf("Expected UTF-16 LE encoding, got %s", encoding)
	}

	// Test empty file
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	os.WriteFile(emptyFile, []byte{}, 0644)
	encoding = utils.DetectFileEncoding(emptyFile)
	if encoding != utils.EncodingUTF8 {
		t.Errorf("Expected UTF-8 for empty file, got %s", encoding)
	}
}

func TestDetectLineEndings(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "lineendings-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test LF
	lfFile := filepath.Join(tmpDir, "lf.txt")
	os.WriteFile(lfFile, []byte("line1\nline2\nline3"), 0644)
	lineEndings := utils.DetectLineEndings(lfFile, utils.EncodingUTF8)
	if lineEndings != utils.LineEndingLF {
		t.Errorf("Expected LF line endings, got %s", lineEndings)
	}

	// Test CRLF
	crlfFile := filepath.Join(tmpDir, "crlf.txt")
	os.WriteFile(crlfFile, []byte("line1\r\nline2\r\nline3"), 0644)
	lineEndings = utils.DetectLineEndings(crlfFile, utils.EncodingUTF8)
	if lineEndings != utils.LineEndingCRLF {
		t.Errorf("Expected CRLF line endings, got %s", lineEndings)
	}

	// Test mixed (should detect dominant)
	mixedFile := filepath.Join(tmpDir, "mixed.txt")
	os.WriteFile(mixedFile, []byte("line1\r\nline2\r\nline3\r\nline4\n"), 0644)
	lineEndings = utils.DetectLineEndings(mixedFile, utils.EncodingUTF8)
	if lineEndings != utils.LineEndingCRLF {
		t.Errorf("Expected CRLF for majority CRLF file, got %s", lineEndings)
	}
}

func TestDetectLineEndingsForString(t *testing.T) {
	tests := []struct {
		content  string
		expected utils.LineEndingType
	}{
		{"line1\nline2\nline3", utils.LineEndingLF},
		{"line1\r\nline2\r\nline3", utils.LineEndingCRLF},
		{"no newlines", utils.LineEndingLF},
		{"", utils.LineEndingLF},
		{"a\nb\nc\nd\ne\nf\ng\nh\n", utils.LineEndingLF},           // 8 LF
		{"a\r\nb\r\nc\r\nd\r\ne\r\nf\r\ng\r\nh\r\ni\r\n", utils.LineEndingCRLF}, // 9 CRLF
	}

	for _, tt := range tests {
		result := utils.DetectLineEndingsForString(tt.content)
		if result != tt.expected {
			t.Errorf("DetectLineEndingsForString(%q) = %s, want %s", tt.content, result, tt.expected)
		}
	}
}

func TestWriteTextContent(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "write-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test LF
	lfFile := filepath.Join(tmpDir, "lf.txt")
	content := "line1\nline2\nline3"
	err = utils.WriteTextContent(lfFile, content, utils.EncodingUTF8, utils.LineEndingLF)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(lfFile)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != content {
		t.Errorf("LF write: expected %q, got %q", content, string(data))
	}

	// Test CRLF conversion
	crlfFile := filepath.Join(tmpDir, "crlf.txt")
	err = utils.WriteTextContent(crlfFile, content, utils.EncodingUTF8, utils.LineEndingCRLF)
	if err != nil {
		t.Fatal(err)
	}

	data, err = os.ReadFile(crlfFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "line1\r\nline2\r\nline3"
	if string(data) != expected {
		t.Errorf("CRLF write: expected %q, got %q", expected, string(data))
	}
}

func TestReadFileWithMetadata(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "read-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test LF file
	lfFile := filepath.Join(tmpDir, "lf.txt")
	os.WriteFile(lfFile, []byte("line1\nline2\nline3"), 0644)

	content, encoding, lineEndings, err := utils.ReadFileWithMetadata(lfFile)
	if err != nil {
		t.Fatal(err)
	}

	if content != "line1\nline2\nline3" {
		t.Errorf("Content mismatch: got %q", content)
	}
	if encoding != utils.EncodingUTF8 {
		t.Errorf("Expected UTF-8, got %s", encoding)
	}
	if lineEndings != utils.LineEndingLF {
		t.Errorf("Expected LF, got %s", lineEndings)
	}

	// Test CRLF file
	crlfFile := filepath.Join(tmpDir, "crlf.txt")
	os.WriteFile(crlfFile, []byte("line1\r\nline2\r\nline3"), 0644)

	content, encoding, lineEndings, err = utils.ReadFileWithMetadata(crlfFile)
	if err != nil {
		t.Fatal(err)
	}

	// Content should be normalized to LF
	if content != "line1\nline2\nline3" {
		t.Errorf("Content should be normalized: got %q", content)
	}
	if lineEndings != utils.LineEndingCRLF {
		t.Errorf("Expected CRLF detection, got %s", lineEndings)
	}
}

func TestGetFileModificationTime(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "mtime-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	tmpFile.WriteString("test")
	tmpFile.Close()

	mtime := utils.GetFileModificationTime(tmpFile.Name())
	if mtime == 0 {
		t.Error("Expected non-zero modification time")
	}
}

func TestPathExists(t *testing.T) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "exists-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if !utils.PathExists(tmpFile.Name()) {
		t.Error("Expected path to exist")
	}

	if utils.PathExists("/nonexistent/path/12345") {
		t.Error("Expected path to not exist")
	}
}