package bash

import (
	"strings"
)

// Exit code interpretation for PowerShell commands

// interpretPowerShellReturnCode interprets the exit code of a PowerShell command
func interpretPowerShellReturnCode(exitCode int, command string) string {
	// PowerShell native cmdlets don't use exit codes meaningfully
	// They exit 0 regardless of result (errors go to error stream)
	// External executables may have semantic exit codes

	// Extract the base command to check for known semantics
	baseCommand := extractPowerShellBaseCommandForSemantics(command)
	canonical := resolvePowerShellCanonical(baseCommand)

	// Check for commands with semantic exit codes
	switch canonical {
	case "grep", "rg", "findstr":
		// grep/ripgrep/findstr: 0 = matches found, 1 = no matches, 2+ = error
		if exitCode == 1 {
			return "No matches found"
		}
		if exitCode >= 2 {
			return ""
		}

	case "robocopy":
		// robocopy: exit codes are a bitfield
		// 0-7 = success, 8+ = error
		if exitCode >= 8 {
			return ""
		}
		switch exitCode {
		case 0:
			return "No files copied (already in sync)"
		case 1:
			return "Files copied successfully"
		case 2, 4, 7:
			return "Robocopy completed (no errors)"
		}
	}

	// Default interpretation
	if exitCode != 0 {
		return ""
	}

	return ""
}

// extractPowerShellBaseCommandForSemantics extracts the base command for exit code semantics
func extractPowerShellBaseCommandForSemantics(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}

	// Split by pipeline and get the last segment (determines exit code)
	segments := splitByPipeline(trimmed)
	if len(segments) == 0 {
		return ""
	}

	lastSegment := segments[len(segments)-1]
	lastSegment = strings.TrimSpace(lastSegment)

	// Handle call operator syntax
	if strings.HasPrefix(lastSegment, "& ") || strings.HasPrefix(lastSegment, ". ") {
		lastSegment = strings.TrimPrefix(lastSegment, "& ")
		lastSegment = strings.TrimPrefix(lastSegment, ". ")
		lastSegment = strings.TrimSpace(lastSegment)
	}

	// Remove quotes
	lastSegment = strings.Trim(lastSegment, "\"'")

	// Get first token
	tokens := strings.Fields(lastSegment)
	if len(tokens) == 0 {
		return ""
	}

	cmdName := tokens[0]

	// Extract basename from path
	if strings.Contains(cmdName, "\\") {
		parts := strings.Split(cmdName, "\\")
		cmdName = parts[len(parts)-1]
	}
	if strings.Contains(cmdName, "/") {
		parts := strings.Split(cmdName, "/")
		cmdName = parts[len(parts)-1]
	}

	// Remove .exe suffix
	cmdName = strings.TrimSuffix(strings.ToLower(cmdName), ".exe")

	return cmdName
}

// splitByPipeline splits a PowerShell command by pipeline operator
func splitByPipeline(command string) []string {
	var segments []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false

	for i := 0; i < len(command); i++ {
		c := command[i]

		// Handle quotes
		if c == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			current.WriteByte(c)
			continue
		}
		if c == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			current.WriteByte(c)
			continue
		}

		// Check for pipeline operator only outside quotes
		if c == '|' && !inSingleQuote && !inDoubleQuote {
			if current.Len() > 0 {
				segments = append(segments, current.String())
				current.Reset()
			}
			continue
		}

		current.WriteByte(c)
	}

	if current.Len() > 0 {
		segments = append(segments, current.String())
	}

	return segments
}

// Command semantics map for external executables
// PowerShell native cmdlets don't need exit-code semantics
var psCommandSemantics = map[string]func(exitCode int) string{
	"grep":    grepSemantic,
	"rg":      grepSemantic,
	"findstr": grepSemantic,
}

// grepSemantic interprets grep/ripgrep exit codes
func grepSemantic(exitCode int) string {
	if exitCode == 0 {
		return "" // Matches found - normal success
	}
	if exitCode == 1 {
		return "No matches found"
	}
	return "" // exitCode >= 2 is error
}

// robocopySemantic interprets robocopy exit codes
func robocopySemantic(exitCode int) string {
	if exitCode >= 8 {
		return "" // Error
	}
	if exitCode == 0 {
		return "No files copied (already in sync)"
	}
	if exitCode&1 != 0 {
		return "Files copied successfully"
	}
	return "Robocopy completed (no errors)"
}
