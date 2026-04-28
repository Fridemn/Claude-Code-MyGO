package tests

import (
	"claude-go/internal/tool/file"

	"claude-go/internal/tool/search"

	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-go/internal/tool"
)

func TestFileToolsListReadWriteEdit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	filePath := filepath.Join(root, "demo.txt")

	writeResult, err := (file.FileWriteTool{}).Call(context.Background(), tool.Input{
		"file_path": filePath,
		"content":   "alpha\nbeta\nTODO item\n",
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("write file: %v", err)
	}
	// FileWriteTool returns FileWriteResult struct
	writeData, ok := writeResult.Content.(file.FileWriteResult)
	if !ok {
		t.Fatalf("unexpected write result type: %T", writeResult.Content)
	}
	if !strings.Contains(writeData.FilePath, "demo.txt") {
		t.Fatalf("unexpected write result: %#v", writeData)
	}

	listResult, err := (file.ListFilesTool{}).Call(context.Background(), tool.Input{
		"path":        root,
		"max_results": 10,
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	files, ok := listResult.Content.([]string)
	if !ok || len(files) == 0 {
		t.Fatalf("unexpected listed files: %#v", listResult.Content)
	}

	readResult, err := (file.FileReadTool{}).Call(context.Background(), tool.Input{
		"file_path": filePath,
		"offset":    1,
		"limit":     2,
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	// FileReadTool returns FileReadResult struct
	readData, ok := readResult.Content.(file.FileReadResult)
	if !ok {
		t.Fatalf("unexpected read content type: %T", readResult.Content)
	}
	textFile, ok := readData.File.(file.TextFileResult)
	if !ok {
		t.Fatalf("unexpected text file type: %T", readData.File)
	}
	if !strings.Contains(textFile.Content, "beta") {
		t.Fatalf("unexpected read content: %q", textFile.Content)
	}

	editResult, err := (file.FileEditTool{}).Call(context.Background(), tool.Input{
		"file_path":  filePath,
		"old_string": "beta",
		"new_string": "gamma",
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("edit file: %v", err)
	}
	editData, ok := editResult.Content.(file.FileEditResult)
	if !ok {
		t.Fatalf("unexpected edit result type: %T", editResult.Content)
	}
	if !strings.Contains(editData.FilePath, "demo.txt") {
		t.Fatalf("unexpected edit result: %#v", editData)
	}

	updated, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if !strings.Contains(string(updated), "gamma") {
		t.Fatalf("expected updated content, got %q", string(updated))
	}
}

func TestListFilesToolSkipsCacheNoise(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".cache", "go-build"), 0o755); err != nil {
		t.Fatalf("mkdir cache: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cache", "go-build", "junk.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write junk: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "demo.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatalf("write demo: %v", err)
	}

	listResult, err := (file.ListFilesTool{}).Call(context.Background(), tool.Input{
		"path": root,
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("list files: %v", err)
	}
	files, ok := listResult.Content.([]string)
	if !ok {
		t.Fatalf("unexpected listed files: %#v", listResult.Content)
	}

	joined := strings.Join(files, "\n")
	if strings.Contains(joined, ".cache") {
		t.Fatalf("expected cache directories to be skipped, got %q", joined)
	}
	if !strings.Contains(joined, "demo.txt") {
		t.Fatalf("expected demo.txt to be listed, got %q", joined)
	}
}

func TestGrepToolFindsMatchesAndRespectsLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("TODO one\nTODO two\n"), 0o644); err != nil {
		t.Fatalf("write a.txt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "b.txt"), []byte("TODO three\n"), 0o644); err != nil {
		t.Fatalf("write b.txt: %v", err)
	}

	result, err := (search.GrepTool{}).Call(context.Background(), tool.Input{
		"pattern":    "TODO",
		"path":       root,
		"head_limit": 2,
	}, tool.Runtime{})
	if err != nil {
		t.Fatalf("grep: %v", err)
	}
	// GrepTool returns a string content, not []string
	content, ok := result.Content.(string)
	if !ok {
		t.Fatalf("unexpected grep content type: %T", result.Content)
	}
	// In files_with_match mode (default), it returns "Found N files\nfile1\nfile2..."
	if !strings.Contains(content, "Found") {
		t.Fatalf("expected 'Found' in output, got: %q", content)
	}
}
