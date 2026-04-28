package utils

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
)

// ReadFileRangeResult contains the result of reading a file range.
type ReadFileRangeResult struct {
	Content         string
	LineCount       int
	TotalLines      int
	TotalBytes      int64
	ReadBytes       int
	MtimeMs         int64
	TruncatedByBytes bool
}

// FastPathMaxSize is the threshold for using the fast in-memory path.
const FastPathMaxSize = 10 * 1024 * 1024 // 10 MB

// ReadFileInRange reads a range of lines from a file.
// For small files (< 10 MB), reads the entire file into memory.
// For large files, streams the file to avoid memory issues.
func ReadFileInRange(ctx context.Context, filePath string, offset, maxLines int, maxBytes int64, truncateOnByteLimit bool) (*ReadFileRangeResult, error) {
	// Check if context is cancelled
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	// Stat the file to determine path
	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, NewClaudeError(fmt.Sprintf("EISDIR: illegal operation on a directory, read '%s'", filePath))
	}

	// Fast path for small files
	if stat.Size() < FastPathMaxSize {
		return readFileInRangeFast(filePath, stat, offset, maxLines, maxBytes, truncateOnByteLimit)
	}

	// Streaming path for large files
	return readFileInRangeStreaming(ctx, filePath, offset, maxLines, maxBytes, truncateOnByteLimit)
}

// readFileInRangeFast reads a small file entirely into memory.
func readFileInRangeFast(filePath string, stat os.FileInfo, offset, maxLines int, maxBytes int64, truncateOnByteLimit bool) (*ReadFileRangeResult, error) {
	// Check size limit
	if !truncateOnByteLimit && maxBytes > 0 && stat.Size() > maxBytes {
		return nil, NewFileTooLargeError(stat.Size(), maxBytes)
	}

	// Read entire file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	mtimeMs := stat.ModTime().UnixMilli()

	// Strip UTF-8 BOM if present (EF BB BF)
	if len(content) >= 3 && content[0] == 0xEF && content[1] == 0xBB && content[2] == 0xBF {
		content = content[3:]
	}

	text := string(content)

	// Split lines
	lines := strings.Split(text, "\n")

	// Calculate range
	endLine := len(lines)
	if maxLines > 0 && offset+maxLines < endLine {
		endLine = offset + maxLines
	}

	// Select lines
	selectedLines := make([]string, 0, endLine-offset)
	selectedBytes := 0
	truncatedByBytes := false

	for i := offset; i < endLine && i < len(lines); i++ {
		line := strings.TrimSuffix(lines[i], "\r")

		if truncateOnByteLimit && maxBytes > 0 {
			sep := 0
			if len(selectedLines) > 0 {
				sep = 1 // newline
			}
			if selectedBytes+sep+len(line) > int(maxBytes) {
				truncatedByBytes = true
				break
			}
			selectedBytes += sep + len(line)
		}

		selectedLines = append(selectedLines, line)
	}

	result := &ReadFileRangeResult{
		Content:          strings.Join(selectedLines, "\n"),
		LineCount:        len(selectedLines),
		TotalLines:       len(lines),
		TotalBytes:       int64(len(text)),
		ReadBytes:        len(strings.Join(selectedLines, "\n")),
		MtimeMs:          mtimeMs,
		TruncatedByBytes: truncatedByBytes,
	}

	return result, nil
}

// readFileInRangeStreaming reads a large file using streaming.
func readFileInRangeStreaming(ctx context.Context, filePath string, offset, maxLines int, maxBytes int64, truncateOnByteLimit bool) (*ReadFileRangeResult, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return nil, err
	}
	mtimeMs := stat.ModTime().UnixMilli()

	reader := bufio.NewReader(file)
	scanner := bufio.NewScanner(reader)
	// Increase buffer size for large lines
	scanner.Buffer(make([]byte, 512*1024), 1024*1024)

	selectedLines := make([]string, 0)
	currentLine := 0
	endLine := -1 // unlimited
	if maxLines > 0 {
		endLine = offset + maxLines
	}

	totalBytesRead := int64(0)
	selectedBytes := 0
	truncatedByBytes := false

	for scanner.Scan() {
		line := scanner.Text()

		// Strip carriage return
		line = strings.TrimSuffix(line, "\r")

		totalBytesRead += int64(len(line) + 1) // +1 for newline

		// Check byte limit
		if !truncateOnByteLimit && maxBytes > 0 && totalBytesRead > maxBytes {
			return nil, NewFileTooLargeError(totalBytesRead, maxBytes)
		}

		// Select lines in range
		if currentLine >= offset && (endLine < 0 || currentLine < endLine) {
			if truncateOnByteLimit && maxBytes > 0 {
				sep := 0
				if len(selectedLines) > 0 {
					sep = 1
				}
				if selectedBytes+sep+len(line) > int(maxBytes) {
					truncatedByBytes = true
					break
				}
				selectedBytes += sep + len(line)
			}
			selectedLines = append(selectedLines, line)
		}

		currentLine++

		// Check context
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	result := &ReadFileRangeResult{
		Content:          strings.Join(selectedLines, "\n"),
		LineCount:        len(selectedLines),
		TotalLines:       currentLine,
		TotalBytes:       totalBytesRead,
		ReadBytes:        len(strings.Join(selectedLines, "\n")),
		MtimeMs:          mtimeMs,
		TruncatedByBytes: truncatedByBytes,
	}

	return result, nil
}

// ReadFileLines reads specific lines from a file with token budget.
func ReadFileLines(ctx context.Context, filePath string, offset, limit int, maxTokens int) (string, int, error) {
	// Convert tokens to approximate bytes (4 chars/token, base64 overhead)
	maxBytes := int64(maxTokens * 3) // conservative estimate

	result, err := ReadFileInRange(ctx, filePath, offset, limit, maxBytes, true)
	if err != nil {
		return "", 0, err
	}

	return result.Content, result.TotalLines, nil
}