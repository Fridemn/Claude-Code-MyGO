package prompt

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

// blockPattern matches ```!` ... ``` blocks (multi-line shell commands)
var blockPattern = regexp.MustCompile("(?s)```!\\s*\\n?([\\s\\S]*?)\\n?```")

// inlinePattern matches !`...` inline shell commands
var inlinePattern = regexp.MustCompile("(?:^|\\s)!`([^`]+)`")

// shellTimeout is the maximum time a shell command can run
const shellTimeout = 30 * time.Second

// ExecuteShellCommandsInPrompt finds shell command patterns in prompt text,
// executes them, and replaces the patterns with command output.
//
// Supported patterns:
//   - !`command` - inline command, replaced with output
//   - ```!\ncommand\n``` - block command, replaced with output
//
// This matches the TS implementation in src/utils/promptShellExecution.ts
func ExecuteShellCommandsInPrompt(ctx context.Context, text string) (string, error) {
	result := text

	// Process block patterns first (```! ... ```)
	blockMatches := blockPattern.FindAllStringSubmatchIndex(text, -1)
	for i := len(blockMatches) - 1; i >= 0; i-- {
		match := blockMatches[i]
		fullMatch := text[match[0]:match[1]]
		command := strings.TrimSpace(text[match[2]:match[3]])

		if command == "" {
			continue
		}

		output, err := executeShellCommand(ctx, command)
		if err != nil {
			return "", fmt.Errorf("shell command failed: %s: %w", command, err)
		}

		result = strings.Replace(result, fullMatch, output, 1)
	}

	// Process inline patterns (!`command`)
	inlineMatches := inlinePattern.FindAllStringSubmatchIndex(result, -1)
	for i := len(inlineMatches) - 1; i >= 0; i-- {
		match := inlineMatches[i]
		fullMatch := result[match[0]:match[1]]
		command := strings.TrimSpace(result[match[2]:match[3]])

		if command == "" {
			continue
		}

		output, err := executeShellCommand(ctx, command)
		if err != nil {
			return "", fmt.Errorf("shell command failed: %s: %w", command, err)
		}

		result = strings.Replace(result, fullMatch, output, 1)
	}

	return result, nil
}

// executeShellCommand runs a shell command and returns its combined output
func executeShellCommand(ctx context.Context, command string) (string, error) {
	// Apply timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, shellTimeout)
	defer cancel()

	cmd := exec.CommandContext(timeoutCtx, "sh", "-c", command)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := strings.TrimSpace(stdout.String())
	errOutput := strings.TrimSpace(stderr.String())

	// Combine stderr with stdout if present
	if errOutput != "" {
		if output != "" {
			output = output + "\n" + errOutput
		} else {
			output = errOutput
		}
	}

	if err != nil {
		if output != "" {
			return output, nil // Return partial output even on error
		}
		return "", fmt.Errorf("command failed: %w", err)
	}

	return output, nil
}
