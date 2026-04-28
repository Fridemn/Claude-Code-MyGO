package file

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/tool"
	"claude-go/internal/tool/bash"
)

// CheckPermissions checks if the file edit operation is permitted
// This implements tool.PermissionChecker interface
func (t FileEditTool) CheckPermissions(ctx context.Context, input tool.Input) tool.PermissionResult {
	filePath := tool.GetString(input, "file_path")
	oldString := tool.GetString(input, "old_string")
	newString := tool.GetString(input, "new_string")

	// Basic validation
	if filePath == "" {
		return tool.PermissionResult{
			Decision: tool.PermissionDeny,
			Message:  "File path is required",
			Reason:   "missing_file_path",
		}
	}

	// Check if old_string and new_string are different
	if oldString == newString {
		return tool.PermissionResult{
			Decision: tool.PermissionDeny,
			Message:  "No changes to make: old_string and new_string are exactly the same",
			Reason:   "no_change",
		}
	}

	// Check permission mode - if AcceptEdits or BypassPermissions, auto-allow
	permMode := bash.GetPermissionChecker().GetMode()
	if permMode == bash.PermissionModeBypassPermissions {
		return tool.PermissionResult{
			Decision:    tool.PermissionAllow,
			Message:     "Bypass permissions mode enabled",
			Reason:      "bypass_permissions",
			RuleMatched: "mode:bypassPermissions",
		}
	}

	if permMode == bash.PermissionModeAcceptEdits {
		// In AcceptEdits mode, auto-approve edits in the working directory
		// (This matches TS behavior: acceptEdits only applies within working directory)
		return tool.PermissionResult{
			Decision:    tool.PermissionAllow,
			Message:     "Accept edits mode - auto-approved",
			Reason:      "accept_edits_mode",
			RuleMatched: "mode:acceptEdits",
		}
	}

	// Determine operation type for message
	opType := "update"
	if oldString == "" {
		opType = "create"
	}

	// Build permission request message with file path info
	displayPath := filePath
	if len(filePath) > 60 {
		// Truncate long paths
		parts := strings.Split(filePath, "/")
		if len(parts) > 3 {
			displayPath = "..." + strings.Join(parts[len(parts)-3:], "/")
		}
	}

	message := fmt.Sprintf("Edit file %s (%s)", displayPath, opType)

	// Add preview info if strings are short enough
	if len(oldString) < 100 && len(newString) < 100 {
		if oldString != "" {
			message += fmt.Sprintf("\nOld: %s", truncatePreview(oldString, 50))
		}
		if newString != "" {
			message += fmt.Sprintf("\nNew: %s", truncatePreview(newString, 50))
		}
	}

	return tool.PermissionResult{
		Decision: tool.PermissionAsk,
		Message:  message,
		Reason:   "file_edit_permission",
	}
}

// CheckPermissions for FileWriteTool
func (t FileWriteTool) CheckPermissions(ctx context.Context, input tool.Input) tool.PermissionResult {
	filePath := tool.GetString(input, "file_path")

	if filePath == "" {
		return tool.PermissionResult{
			Decision: tool.PermissionDeny,
			Message:  "File path is required",
			Reason:   "missing_file_path",
		}
	}

	// Check permission mode - if AcceptEdits or BypassPermissions, auto-allow
	permMode := bash.GetPermissionChecker().GetMode()
	if permMode == bash.PermissionModeBypassPermissions {
		return tool.PermissionResult{
			Decision:    tool.PermissionAllow,
			Message:     "Bypass permissions mode enabled",
			Reason:      "bypass_permissions",
			RuleMatched: "mode:bypassPermissions",
		}
	}

	if permMode == bash.PermissionModeAcceptEdits {
		// In AcceptEdits mode, auto-approve writes in the working directory
		return tool.PermissionResult{
			Decision:    tool.PermissionAllow,
			Message:     "Accept edits mode - auto-approved",
			Reason:      "accept_edits_mode",
			RuleMatched: "mode:acceptEdits",
		}
	}

	// Build message
	displayPath := filePath
	if len(filePath) > 60 {
		parts := strings.Split(filePath, "/")
		if len(parts) > 3 {
			displayPath = "..." + strings.Join(parts[len(parts)-3:], "/")
		}
	}

	message := fmt.Sprintf("Write to file %s", displayPath)

	return tool.PermissionResult{
		Decision: tool.PermissionAsk,
		Message:  message,
		Reason:   "file_write_permission",
	}
}

// truncatePreview truncates a string for preview display
func truncatePreview(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "…"
}