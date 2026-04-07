package bash

import (
	"strings"
)

// Read-only command validation for PowerShell

// PowerShell search commands that can be collapsed in UI
var psSearchCommands = map[string]bool{
	"select-string":  true,
	"get-childitem":  true, // with -Recurse
	"findstr":        true,
	"where.exe":      true,
}

// PowerShell read/view commands
var psReadCommands = map[string]bool{
	"get-content":    true,
	"get-item":       true,
	"test-path":      true,
	"resolve-path":   true,
	"get-process":    true,
	"get-service":    true,
	"get-location":   true,
	"get-filehash":   true,
	"get-acl":        true,
	"format-hex":     true,
	"get-childitem":  true,
}

// PowerShell semantic-neutral commands (pure output/status)
var psSemanticNeutralCommands = map[string]bool{
	"write-output": true,
	"write-host":   true,
}

// isReadOnlyPowerShellCommand determines if a PowerShell command is read-only
func isReadOnlyPowerShellCommand(command string) bool {
	if strings.TrimSpace(command) == "" {
		return false
	}

	// Check for sync security concerns first
	if hasSyncSecurityConcerns(command) {
		return false
	}

	// Split by PowerShell statement separators: ; |
	parts := splitPowerShellCommand(command)
	if len(parts) == 0 {
		return false
	}

	hasSearch := false
	hasRead := false
	hasNonNeutralCommand := false

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}

		// Get base command
		baseCommand := extractPowerShellBaseCommand(trimmed)
		if baseCommand == "" {
			continue
		}

		canonical := resolvePowerShellCanonical(baseCommand)

		// Skip semantic-neutral commands
		if psSemanticNeutralCommands[canonical] {
			continue
		}

		hasNonNeutralCommand = true

		isPartSearch := psSearchCommands[canonical]
		isPartRead := psReadCommands[canonical]

		if !isPartSearch && !isPartRead {
			return false
		}

		if isPartSearch {
			hasSearch = true
		}
		if isPartRead {
			hasRead = true
		}
	}

	if !hasNonNeutralCommand {
		return false
	}

	return hasSearch || hasRead
}

// hasSyncSecurityConcerns checks for security concerns that can be detected synchronously
func hasSyncSecurityConcerns(command string) bool {
	// Check for subexpressions
	if strings.Contains(command, "$(") {
		return true
	}

	// Check for splatting
	if strings.Contains(command, "@") {
		return true
	}

	// Check for member invocations (method calls)
	if strings.Contains(command, "().") {
		return true
	}

	// Check for script blocks
	if strings.Contains(command, "{") && strings.Contains(command, "}") {
		return true
	}

	// Check for assignments
	assignmentPatterns := []string{"= ", "+= ", "-= ", "*= ", "/= "}
	for _, pattern := range assignmentPatterns {
		if strings.Contains(command, pattern) {
			// Allow variable assignment in certain contexts
			// This is a simplified check
			if strings.Contains(command, "$") {
				return true
			}
		}
	}

	return false
}

// splitPowerShellCommand splits a PowerShell command by statement separators
func splitPowerShellCommand(command string) []string {
	var parts []string
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

		// Check for separators only outside quotes
		if !inSingleQuote && !inDoubleQuote {
			// PowerShell statement separators: ; | (not && and ||)
			if c == ';' || c == '|' {
				if current.Len() > 0 {
					parts = append(parts, current.String())
					current.Reset()
				}
				continue
			}
		}

		current.WriteByte(c)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// extractPowerShellBaseCommand extracts the base command from a PowerShell command part
func extractPowerShellBaseCommand(part string) string {
	trimmed := strings.TrimSpace(part)
	if trimmed == "" {
		return ""
	}

	// Handle call operator
	if strings.HasPrefix(trimmed, "& ") || strings.HasPrefix(trimmed, ". ") {
		trimmed = strings.TrimPrefix(trimmed, "& ")
		trimmed = strings.TrimPrefix(trimmed, ". ")
		trimmed = strings.TrimSpace(trimmed)
	}

	// Get first token
	tokens := strings.Fields(trimmed)
	if len(tokens) == 0 {
		return ""
	}

	return tokens[0]
}

// Cmdlet allowlist for read-only operations with safe flags
// This is a simplified version - the full TS version has extensive flag validation
var readOnlyCmdletAllowlist = map[string]bool{
	// Filesystem (read-only)
	"get-childitem":   true,
	"get-content":     true,
	"get-item":        true,
	"get-itemproperty": true,
	"test-path":       true,
	"resolve-path":    true,
	"get-filehash":    true,
	"get-acl":         true,

	// Navigation
	"set-location":    true,
	"push-location":   true,
	"pop-location":    true,

	// Text searching
	"select-string":   true,

	// Data conversion (no side effects)
	"convertto-json":  true,
	"convertfrom-json": true,
	"convertto-csv":   true,
	"convertfrom-csv": true,
	"convertto-xml":   true,
	"convertto-html":  true,
	"format-hex":      true,

	// Object inspection
	"get-member":      true,
	"get-unique":      true,
	"compare-object":  true,
	"join-string":     true,
	"get-random":      true,

	// Path utilities
	"convert-path":    true,
	"join-path":       true,
	"split-path":      true,

	// System info
	"get-hotfix":      true,
	"get-itempropertyvalue": true,
	"get-psprovider":  true,
	"get-process":     true,
	"get-service":     true,
	"get-computerinfo": true,
	"get-host":        true,
	"get-date":        true,
	"get-location":    true,
	"get-psdrive":     true,
	"get-module":      true,
	"get-alias":       true,
	"get-history":     true,
	"get-culture":     true,
	"get-uiculture":   true,
	"get-timezone":    true,
	"get-uptime":      true,

	// Output (no side effects)
	"write-output":    true,
	"write-host":      true,
	"start-sleep":     true, // Only short sleeps are allowed

	// External read-only commands
	"hostname":        true,
	"whoami":          true,
	"where.exe":       true,
	"findstr":         true,
}

// isAllowlistedCmdlet checks if a cmdlet is in the read-only allowlist
func isAllowlistedCmdlet(cmdName string) bool {
	canonical := resolvePowerShellCanonical(cmdName)
	return readOnlyCmdletAllowlist[canonical]
}
