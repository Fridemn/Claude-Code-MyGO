package dialogs

import (
	"fmt"

	"claude-code-go/internal/ui/components"
)

// PermissionType represents different types of permission requests
type PermissionType int

const (
	PermissionBashCommand PermissionType = iota
	PermissionFileWrite
	PermissionFileRead
	PermissionMCPTool
	PermissionAgentSpawn
	PermissionNetwork
)

// PermissionRequest represents a permission request
type PermissionRequest struct {
	Type        PermissionType
	ToolName    string
	Description string
	Details     []string // Additional details to show
	Path        string   // File path if applicable
	Command     string   // Command if applicable
	IsDangerous bool     // Highlight as potentially dangerous
}

// Theme colors
var (
	colorPermission = components.RGB{177, 185, 249}
	colorError      = components.RGB{255, 107, 128}
	colorWarning    = components.RGB{255, 193, 7}
	colorMuted      = components.RGB{134, 145, 160}
	colorText       = components.RGB{255, 255, 255}
	colorSuccess    = components.RGB{78, 186, 101}
)

// RenderPermissionDialog renders a permission request dialog
// Matches src/components/TrustDialog and permissions components
func RenderPermissionDialog(req PermissionRequest, width int) string {
	if width <= 0 {
		width = 70
	}

	titleColor := colorPermission
	if req.IsDangerous {
		titleColor = colorWarning
	}

	// Build dialog content
	var content []string

	// Tool/action description
	if req.Description != "" {
		content = append(content, req.Description)
	}

	// Show path or command
	if req.Path != "" {
		content = append(content, "")
		content = append(content, fmt.Sprintf("Path: %s", req.Path))
	}
	if req.Command != "" {
		content = append(content, "")
		content = append(content, fmt.Sprintf("Command: %s", req.Command))
	}

	// Additional details
	if len(req.Details) > 0 {
		content = append(content, "")
		for _, detail := range req.Details {
			content = append(content, "  • "+detail)
		}
	}

	// Warning for dangerous operations
	if req.IsDangerous {
		content = append(content, "")
		content = append(content, renderColored(colorWarning, "⚠ This operation may modify your system"))
	}

	cfg := components.DialogConfig{
		Title:     fmt.Sprintf("Permission required: %s", req.ToolName),
		Content:   content,
		Color:     titleColor,
		Width:     width,
		InputHint: "y to allow · n to deny · a to always allow",
	}

	return components.RenderDialog(cfg)
}

// RenderTrustDialog renders the initial trust dialog
// Matches src/components/TrustDialog/TrustDialog.tsx
func RenderTrustDialog(projectPath string, width int) string {
	if width <= 0 {
		width = 70
	}

	content := []string{
		"Claude Code wants to operate in this directory:",
		"",
		"  " + projectPath,
		"",
		"This will allow Claude to:",
		"  • Read files in this directory",
		"  • Execute commands",
		"  • Make changes to files (with your approval)",
		"",
		"Would you like to trust this directory?",
	}

	cfg := components.DialogConfig{
		Title:     "Trust Project Directory",
		Content:   content,
		Color:     colorPermission,
		Width:     width,
		InputHint: "y to trust · n to cancel",
	}

	return components.RenderDialog(cfg)
}

// PermissionMode represents the current permission mode
type PermissionMode string

const (
	PermissionModeDefault PermissionMode = "default" // Ask for each
	PermissionModeAuto    PermissionMode = "auto"    // Auto-approve safe
	PermissionModePlan    PermissionMode = "plan"    // Plan mode
	PermissionModeBypass  PermissionMode = "bypass"  // Bypass all
)

// RenderAutoModeDialog renders the auto-mode opt-in dialog
// Matches src/components/AutoModeOptInDialog.tsx
func RenderAutoModeDialog(width int) string {
	if width <= 0 {
		width = 70
	}

	content := []string{
		"Auto mode allows Claude to automatically approve safe operations",
		"like reading files and running non-destructive commands.",
		"",
		"You will still be asked to approve:",
		"  • File modifications",
		"  • Potentially dangerous commands",
		"  • New tool installations",
		"",
		"Enable auto mode for this session?",
	}

	cfg := components.DialogConfig{
		Title:     "Enable Auto Mode?",
		Content:   content,
		Color:     colorPermission,
		Width:     width,
		InputHint: "y to enable · n to keep asking",
	}

	return components.RenderDialog(cfg)
}

// RenderBypassModeWarning renders a warning for bypass mode
// Matches src/components/BypassPermissionsModeDialog.tsx
func RenderBypassModeWarning(width int) string {
	if width <= 0 {
		width = 70
	}

	content := []string{
		"DANGER: Bypass mode will auto-approve ALL operations including:",
		"",
		renderColored(colorError, "  • File deletions"),
		renderColored(colorError, "  • System modifications"),
		renderColored(colorError, "  • Arbitrary command execution"),
		"",
		"This mode should only be used in sandboxed environments.",
		"",
		renderColored(colorWarning, "Are you absolutely sure?"),
	}

	cfg := components.DialogConfig{
		Title:     "⚠ BYPASS MODE WARNING",
		Content:   content,
		Color:     colorError,
		Width:     width,
		InputHint: "Type 'yes' to confirm · any other key to cancel",
	}

	return components.RenderDialog(cfg)
}

// MCPServerApproval represents an MCP server approval request
type MCPServerApproval struct {
	ServerName  string
	Description string
	Tools       []string
	IsNew       bool
}

// RenderMCPApprovalDialog renders an MCP server approval dialog
// Matches src/components/MCPServerApprovalDialog.tsx
func RenderMCPApprovalDialog(approval MCPServerApproval, width int) string {
	if width <= 0 {
		width = 70
	}

	status := "update"
	if approval.IsNew {
		status = "new"
	}

	content := []string{
		fmt.Sprintf("Server: %s (%s)", approval.ServerName, status),
	}

	if approval.Description != "" {
		content = append(content, "")
		content = append(content, approval.Description)
	}

	if len(approval.Tools) > 0 {
		content = append(content, "")
		content = append(content, "Available tools:")
		for _, tool := range approval.Tools {
			content = append(content, "  • "+tool)
		}
	}

	cfg := components.DialogConfig{
		Title:     "Approve MCP Server?",
		Content:   content,
		Color:     colorPermission,
		Width:     width,
		InputHint: "y to approve · n to deny",
	}

	return components.RenderDialog(cfg)
}

// RenderCostThresholdDialog renders a cost threshold warning
// Matches src/components/CostThresholdDialog.tsx
func RenderCostThresholdDialog(currentCost, threshold float64, width int) string {
	if width <= 0 {
		width = 60
	}

	content := []string{
		fmt.Sprintf("Current session cost: $%.2f", currentCost),
		fmt.Sprintf("Threshold: $%.2f", threshold),
		"",
		"Would you like to continue?",
	}

	cfg := components.DialogConfig{
		Title:     "Cost Threshold Reached",
		Content:   content,
		Color:     colorWarning,
		Width:     width,
		InputHint: "y to continue · n to stop",
	}

	return components.RenderDialog(cfg)
}

// RenderExportDialog renders an export options dialog
// Matches src/components/ExportDialog.tsx
func RenderExportDialog(sessionName string, formats []string, width int) string {
	if width <= 0 {
		width = 60
	}

	content := []string{
		fmt.Sprintf("Session: %s", sessionName),
		"",
		"Export format:",
	}

	for i, format := range formats {
		content = append(content, fmt.Sprintf("  %d. %s", i+1, format))
	}

	cfg := components.DialogConfig{
		Title:     "Export Session",
		Content:   content,
		Color:     colorPermission,
		Width:     width,
		InputHint: "Enter number to select · Esc to cancel",
	}

	return components.RenderDialog(cfg)
}

// Helper functions

func renderColored(color components.RGB, text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
}

func renderBold(text string) string {
	return "\033[1m" + text + "\033[0m"
}
