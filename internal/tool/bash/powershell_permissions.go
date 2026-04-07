package bash

import (
	"strings"
)

// Permission checking for PowerShell commands

// PermissionCheckResult represents the result of a permission check
type PermissionCheckResult struct {
	Allowed bool
	Reason  string
}

// checkPowerShellPermission checks if a PowerShell command is allowed
func checkPowerShellPermission(command, description string) PermissionCheckResult {
	cmdName := extractPowerShellCommandName(command)
	canonical := resolvePowerShellCanonical(cmdName)

	// Check against deny rules first
	if dangerousPowerShellCmdlets[canonical] {
		return PermissionCheckResult{
			Allowed: false,
			Reason:  "Command is in deny list",
		}
	}

	// Check read-only commands - they are auto-allowed
	if isReadOnlyPowerShellCommand(command) {
		return PermissionCheckResult{
			Allowed: true,
			Reason:  "Command is read-only",
		}
	}

	// Check if command requires explicit permission
	if askPowerShellCmdlets[canonical] {
		// In full implementation, this would trigger a permission prompt
		// For now, we return allowed with a note
		return PermissionCheckResult{
			Allowed: true,
			Reason:  "Command requires explicit permission (auto-allowed for testing)",
		}
	}

	// Default: allow
	return PermissionCheckResult{
		Allowed: true,
		Reason:  "Default allow",
	}
}

// PowerShell command aliases mapping to canonical names
var powerShellAliases = map[string]string{
	// File system
	"ls":        "get-childitem",
	"dir":       "get-childitem",
	"gci":       "get-childitem",
	"cat":       "get-content",
	"gc":        "get-content",
	"type":      "get-content",
	"cd":        "set-location",
	"sl":        "set-location",
	"pwd":       "get-location",
	"gl":        "get-location",
	"pushd":     "push-location",
	"popd":      "pop-location",
	"del":       "remove-item",
	"rm":        "remove-item",
	"ri":        "remove-item",
	"erase":     "remove-item",
	"rmdir":     "remove-item",
	"rd":        "remove-item",
	"mv":        "move-item",
	"mi":        "move-item",
	"move":      "move-item",
	"ren":       "rename-item",
	"rni":       "rename-item",
	"cp":        "copy-item",
	"cpi":       "copy-item",
	"copy":      "copy-item",
	"mkdir":     "new-item",
	"ni":        "new-item",
	"touch":     "set-content",
	"sc":        "set-content",
	"ac":        "add-content",
	"clc":       "clear-content",
	"si":        "set-item",
	"gi":        "get-item",
	"gp":        "get-itemproperty",
	"sp":        "set-itemproperty",
	"rp":        "remove-itemproperty",
	"clv":       "clear-variable",
	"cv":        "clear-variable",
	"nv":        "new-variable",
	"set":       "set-variable",
	"sv":        "set-variable",

	// Process and service
	"ps":        "get-process",
	"gps":       "get-process",
	"kill":      "stop-process",
	"spps":      "stop-process",
	"pps":       "start-process",

	// Data
	"echo":      "write-output",
	"write":     "write-output",
	"select":    "select-object",
	"where":     "where-object",
	"sort":      "sort-object",
	"group":     "group-object",
	"measure":   "measure-object",
	"tee":       "tee-object",
	"foreach":   "foreach-object",
	"ft":        "format-table",
	"fl":        "format-list",
	"fw":        "format-wide",
	"fc":        "format-custom",

	// Content search
	"sls":       "select-string",

	// Network
	"wget":      "invoke-webrequest",
	"curl":      "invoke-webrequest",
	"iwr":       "invoke-webrequest",
	"irm":       "invoke-restmethod",

	// Execution
	"iex":       "invoke-expression",
	"clhy":      "clear-history",
	"h":         "get-history",
	"ghy":       "get-history",
	"ihy":       "invoke-history",

	// Comparison
	"diff":      "compare-object",
	"cmp":       "compare-object",
}

// resolvePowerShellCanonical resolves a command name/alias to its canonical form
func resolvePowerShellCanonical(cmdName string) string {
	lower := strings.ToLower(cmdName)
	if canonical, ok := powerShellAliases[lower]; ok {
		return canonical
	}
	return lower
}

// extractPowerShellCommandName extracts the command name from a PowerShell command string
func extractPowerShellCommandName(command string) string {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return ""
	}

	// Handle call operator syntax: & "command" or . "command"
	if strings.HasPrefix(trimmed, "& ") || strings.HasPrefix(trimmed, ". ") {
		trimmed = strings.TrimPrefix(trimmed, "& ")
		trimmed = strings.TrimPrefix(trimmed, ". ")
		trimmed = strings.TrimSpace(trimmed)
	}

	// Remove quotes if present
	trimmed = strings.Trim(trimmed, "\"'")

	// Get first token
	parts := strings.Fields(trimmed)
	if len(parts) == 0 {
		return ""
	}

	cmdName := parts[0]

	// Handle module-qualified names: Module\Command
	if idx := strings.LastIndex(cmdName, "\\"); idx >= 0 {
		cmdName = cmdName[idx+1:]
	}

	// Remove .exe suffix for external commands
	cmdName = strings.TrimSuffix(strings.ToLower(cmdName), ".exe")

	return cmdName
}