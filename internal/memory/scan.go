package memory

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"claude-code-go/internal/types"
)

// ScanMemoryFiles scans a memory directory for .md files and returns
// header information sorted newest-first, capped at MaxMemoryFiles.
func ScanMemoryFiles(ctx context.Context, memoryDir string) ([]types.MemoryHeader, error) {
	entries, err := os.ReadDir(memoryDir)
	if err != nil {
		return nil, nil
	}

	var mdFiles []string
	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively find .md files in subdirectories
			subFiles, _ := findMdFilesRecursive(filepath.Join(memoryDir, entry.Name()))
			mdFiles = append(mdFiles, subFiles...)
		} else if strings.HasSuffix(entry.Name(), ".md") && entry.Name() != EntrypointName {
			mdFiles = append(mdFiles, filepath.Join(memoryDir, entry.Name()))
		}
	}

	var headers []types.MemoryHeader
	for _, filePath := range mdFiles {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		header, err := scanMemoryFileHeader(filePath)
		if err != nil {
			continue
		}
		if header != nil {
			headers = append(headers, *header)
		}
	}

	// Sort by mtime descending (newest first)
	sort.Slice(headers, func(i, j int) bool {
		return headers[i].MtimeMs > headers[j].MtimeMs
	})

	// Cap at MaxMemoryFiles
	if len(headers) > MaxMemoryFiles {
		headers = headers[:MaxMemoryFiles]
	}

	return headers, nil
}

func findMdFilesRecursive(dir string) ([]string, error) {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			subFiles, _ := findMdFilesRecursive(fullPath)
			files = append(files, subFiles...)
		} else if strings.HasSuffix(entry.Name(), ".md") && entry.Name() != EntrypointName {
			files = append(files, fullPath)
		}
	}

	return files, nil
}

func scanMemoryFileHeader(filePath string) (*types.MemoryHeader, error) {
	// Get file info for mtime
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	// Read first N lines for frontmatter
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read up to FrontmatterMaxLines
	var lines []string
	scanner := CreateLimitScanner(file, FrontmatterMaxLines)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	content := strings.Join(lines, "\n")
	fm, _ := ParseFrontmatter(content)

	return &types.MemoryHeader{
		Filename:    filepath.Base(filePath),
		FilePath:    filePath,
		MtimeMs:     info.ModTime().UnixMilli(),
		Description: fm.Description,
		Type:        types.ParseMemoryType(fm.Type),
	}, nil
}

// FormatMemoryManifest formats memory headers as a text manifest.
// One line per file: [type] filename (timestamp): description
func FormatMemoryManifest(memories []types.MemoryHeader) string {
	var lines []string
	for _, m := range memories {
		var tag string
		if m.Type != "" {
			tag = "[" + string(m.Type) + "] "
		}
		ts := time.UnixMilli(m.MtimeMs).UTC().Format(time.RFC3339)

		var line string
		if m.Description != "" {
			line = "- " + tag + m.Filename + " (" + ts + "): " + m.Description
		} else {
			line = "- " + tag + m.Filename + " (" + ts + ")"
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

// LimitScanner wraps a scanner to limit the number of lines read.
type LimitScanner struct {
	scanner   Scanner
	maxLines  int
	lineCount int
}

// Scanner is an interface matching bufio.Scanner for testing.
type Scanner interface {
	Scan() bool
	Text() string
	Err() error
}

// LimitScanner creates a scanner that limits the number of lines.
func CreateLimitScanner(file *os.File, maxLines int) *LimitScanner {
	return &LimitScanner{
		scanner:  bufio.NewScanner(file),
		maxLines: maxLines,
	}
}

// Scan advances to the next line, respecting the line limit.
func (ls *LimitScanner) Scan() bool {
	if ls.lineCount >= ls.maxLines {
		return false
	}
	if !ls.scanner.Scan() {
		return false
	}
	ls.lineCount++
	return true
}

// Text returns the current line.
func (ls *LimitScanner) Text() string {
	return ls.scanner.Text()
}

// Err returns any error encountered during scanning.
func (ls *LimitScanner) Err() error {
	return ls.scanner.Err()
}
