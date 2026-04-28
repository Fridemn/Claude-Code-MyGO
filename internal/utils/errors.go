package utils

import (
	"errors"
	"fmt"
	"runtime/debug"
	"strings"
)

// ClaudeError is the base error type for Claude Code.
type ClaudeError struct {
	Message string
}

func (e *ClaudeError) Error() string {
	return e.Message
}

// NewClaudeError creates a new ClaudeError.
func NewClaudeError(message string) *ClaudeError {
	return &ClaudeError{Message: message}
}

// AbortError represents an aborted operation.
type AbortError struct {
	Message string
}

func (e *AbortError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "operation aborted"
}

// IsAbortError checks if an error is an abort error.
func IsAbortError(err error) bool {
	var abortErr *AbortError
	return errors.As(err, &abortErr) || errors.Is(err, contextCanceled)
}

var contextCanceled = errors.New("context canceled")

// ConfigParseError represents a configuration parsing error.
type ConfigParseError struct {
	Message      string
	FilePath     string
	DefaultConfig interface{}
}

func (e *ConfigParseError) Error() string {
	return e.Message
}

// NewConfigParseError creates a new ConfigParseError.
func NewConfigParseError(message, filePath string, defaultConfig interface{}) *ConfigParseError {
	return &ConfigParseError{
		Message:      message,
		FilePath:     filePath,
		DefaultConfig: defaultConfig,
	}
}

// ShellError represents a shell command failure.
type ShellError struct {
	Stdout      string
	Stderr      string
	Code        int
	Interrupted bool
}

func (e *ShellError) Error() string {
	return fmt.Sprintf("shell command failed with code %d", e.Code)
}

// NewShellError creates a new ShellError.
func NewShellError(stdout, stderr string, code int, interrupted bool) *ShellError {
	return &ShellError{
		Stdout:      stdout,
		Stderr:      stderr,
		Code:        code,
		Interrupted: interrupted,
	}
}

// FileTooLargeError represents a file that exceeds size limits.
type FileTooLargeError struct {
	SizeInBytes  int64
	MaxSizeBytes int64
}

func (e *FileTooLargeError) Error() string {
	return fmt.Sprintf(
		"File content (%s) exceeds maximum allowed size (%s). Use offset and limit parameters to read specific portions of the file, or search for specific content instead of reading the whole file.",
		FormatFileSize(e.SizeInBytes),
		FormatFileSize(e.MaxSizeBytes),
	)
}

// NewFileTooLargeError creates a new FileTooLargeError.
func NewFileTooLargeError(sizeInBytes, maxSizeBytes int64) *FileTooLargeError {
	return &FileTooLargeError{
		SizeInBytes:  sizeInBytes,
		MaxSizeBytes: maxSizeBytes,
	}
}

// ImageResizeError represents an image resizing failure.
type ImageResizeError struct {
	Message string
}

func (e *ImageResizeError) Error() string {
	return e.Message
}

// NewImageResizeError creates a new ImageResizeError.
func NewImageResizeError(message string) *ImageResizeError {
	return &ImageResizeError{Message: message}
}

// MalformedCommandError represents a malformed command.
type MalformedCommandError struct {
	Message string
}

func (e *MalformedCommandError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return "malformed command"
}

// ToError converts any value to an error.
func ToError(e interface{}) error {
	if e == nil {
		return nil
	}
	switch v := e.(type) {
	case error:
		return v
	case string:
		return errors.New(v)
	default:
		return fmt.Errorf("%v", e)
	}
}

// ErrorMessage extracts the message from an error.
func ErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// Wrap wraps an error with context.
func Wrap(scope string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", scope, err)
}

// Wrapf wraps an error with formatted context.
func Wrapf(err error, format string, args ...interface{}) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", fmt.Sprintf(format, args...), err)
}

// IsENOENT checks if the error is ENOENT (file not found).
func IsENOENT(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no such file or directory")
}

// IsEACCES checks if the error is EACCES (permission denied).
func IsEACCES(err error) bool {
	return err != nil && strings.Contains(err.Error(), "permission denied")
}

// IsFSInaccessible checks if the error indicates an inaccessible filesystem path.
func IsFSInaccessible(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "no such file") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "operation not permitted") ||
		strings.Contains(errStr, "not a directory") ||
		strings.Contains(errStr, "too many levels of symbolic links")
}

// ShortErrorStack returns a truncated error stack trace.
// Useful when the error flows to the model - full stack traces waste context tokens.
func ShortErrorStack(err error, maxFrames int) string {
	if err == nil {
		return ""
	}

	stack := string(debug.Stack())
	if stack == "" {
		return err.Error()
	}

	lines := strings.Split(stack, "\n")
	if len(lines) <= maxFrames+1 {
		return stack
	}

	// Keep header and first maxFrames lines
	result := make([]string, 0, maxFrames+1)
	result = append(result, lines[0]) // Error message

	frameCount := 0
	for i := 1; i < len(lines) && frameCount < maxFrames; i++ {
		if strings.Contains(lines[i], "at ") || strings.Contains(lines[i], "\t") {
			result = append(result, lines[i])
			frameCount++
		}
	}

	return strings.Join(result, "\n")
}

// HasErrorMessage checks if an error has an exact message.
func HasErrorMessage(err error, message string) bool {
	return err != nil && err.Error() == message
}

// GetErrnoCode extracts errno code from an error.
func GetErrnoCode(err error) string {
	if err == nil {
		return ""
	}
	errStr := err.Error()

	// Common errno patterns
	if strings.Contains(errStr, "no such file") {
		return "ENOENT"
	}
	if strings.Contains(errStr, "permission denied") {
		return "EACCES"
	}
	if strings.Contains(errStr, "operation not permitted") {
		return "EPERM"
	}
	if strings.Contains(errStr, "not a directory") {
		return "ENOTDIR"
	}
	if strings.Contains(errStr, "too many levels") {
		return "ELOOP"
	}
	if strings.Contains(errStr, "connection refused") {
		return "ECONNREFUSED"
	}
	if strings.Contains(errStr, "broken pipe") {
		return "EPIPE"
	}
	if strings.Contains(errStr, "timed out") || strings.Contains(errStr, "timeout") {
		return "ETIMEDOUT"
	}

	return ""
}

// FormatFileSize formats a file size in bytes to a human-readable string.
func FormatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}