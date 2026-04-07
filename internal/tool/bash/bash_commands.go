package bash

import (
	"regexp"
	"strings"
)

// splitCommandWithOperators splits command string into parts by shell operators
func splitCommandWithOperators(command string) []string {
	var parts []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i := 0; i < len(command); i++ {
		c := rune(command[i])

		if escaped {
			current.WriteRune(c)
			escaped = false
			continue
		}

		if c == '\\' && !inSingleQuote {
			current.WriteRune(c)
			escaped = true
			continue
		}

		if c == '\'' && !inDoubleQuote {
			current.WriteRune(c)
			inSingleQuote = !inSingleQuote
			continue
		}

		if c == '"' && !inSingleQuote {
			current.WriteRune(c)
			inDoubleQuote = !inDoubleQuote
			continue
		}

		if !inSingleQuote && !inDoubleQuote {
			// Check for two-character operators
			if i+1 < len(command) {
				twoChar := command[i : i+2]
				switch twoChar {
				case "&&", "||", ">>", ">&", ";<":
					if current.Len() > 0 {
						parts = append(parts, current.String())
						current.Reset()
					}
					parts = append(parts, twoChar)
					i++
					continue
				}
			}

			// Check for single-character operators
			switch c {
			case '|', ';', '(', ')', '<', '>':
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
				parts = append(parts, string(c))
				continue
			}
		}

		current.WriteRune(c)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// splitCommandDeprecated splits command by operators and filters redirections
// This provides behavior similar to TS splitCommand_DEPRECATED
func splitCommandDeprecated(command string) []string {
	parts := splitCommandWithOperators(command)
	return filterRedirections(parts)
}

// filterRedirections removes redirection operators from command parts
// Handles: 2>&1, 2>/dev/null, > file.txt, >> file.txt
func filterRedirections(parts []string) []string {
	var filtered []string

	for i := 0; i < len(parts); i++ {
		part := parts[i]
		if part == "" || part == undefined {
			continue
		}

		// Check for redirection operators
		switch part {
		case ">", ">>", "<", "<&":
			// Skip operator and target
			continue
		case "&>":
			// Skip operator and target
			continue
		case ">&":
			// Skip operator
			continue
		}

		// Check for &> pattern (operator already merged)
		if strings.HasPrefix(part, "&>") {
			continue
		}

		// Handle specific redirection patterns
		if isRedirectionPart(part, i, parts) {
			continue
		}

		filtered = append(filtered, part)
	}

	return filtered
}

// undefined placeholder for removed parts
const undefined = "\x00REMOVED\x00"

// isRedirectionPart checks if a part is a redirection target
func isRedirectionPart(part string, idx int, parts []string) bool {
	// Check for FD patterns like "2>&1" or "2>/dev/null"
	fdRegex := regexp.MustCompile(`^\d+>&?\d+$|^\d+>.*$`)
	if fdRegex.MatchString(part) {
		return true
	}

	// Check for ">&" preceded by FD
	if part == ">&" && idx > 0 {
		prev := parts[idx-1]
		if len(prev) > 0 && prev[len(prev)-1] >= '0' && prev[len(prev)-1] <= '9' {
			return true
		}
	}

	// Check if this part looks like a redirect target
	// (starts with > or >>)
	if strings.HasPrefix(part, ">") || strings.HasPrefix(part, ">>") {
		return true
	}

	return false
}

// isCommandList checks if command contains list operators (; && ||)
func isCommandList(command string) bool {
	parts := splitCommandWithOperators(command)
	for _, part := range parts {
		if part == ";" || part == "&&" || part == "||" {
			return true
		}
	}
	return false
}

// extractBaseCommandWithArgs extracts the base command and args from a command part
func extractBaseCommandWithArgs(part string) (base, args string) {
	parts := strings.Fields(part)
	if len(parts) == 0 {
		return "", ""
	}
	base = parts[0]
	if len(parts) > 1 {
		args = strings.Join(parts[1:], " ")
	}
	return base, args
}

// hasMultipleCommands checks if command string has multiple commands
func hasMultipleCommands(command string) bool {
	parts := splitCommandDeprecated(command)
	return len(parts) > 1 && !isCommandList(command)
}

// hasPipeline checks if command contains a pipeline
func hasPipeline(command string) bool {
	parts := splitCommandWithOperators(command)
	for _, part := range parts {
		if part == "|" {
			return true
		}
	}
	return false
}