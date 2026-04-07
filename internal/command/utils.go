package command

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/tool"
)

// FormatCommandUsage returns a usage string for a command
func FormatCommandUsage(name, hint string) string {
	if hint == "" {
		return fmt.Sprintf("usage: /%s", name)
	}
	return fmt.Sprintf("usage: /%s %s", name, hint)
}

// EmptyDash returns a dash if the string is empty or whitespace only
func EmptyDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "-"
	}
	return s
}

// ParseToggle parses a string as a boolean toggle
func ParseToggle(s string) (bool, error) {
	switch s {
	case "on", "true", "1", "yes", "enable":
		return true, nil
	case "off", "false", "0", "no", "disable":
		return false, nil
	default:
		return false, fmt.Errorf("invalid toggle value: %s (use on/off)", s)
	}
}

// ShellQuote quotes a string for shell usage
func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// CallNamedTool calls a tool by name with the given input
func CallNamedTool(ctx context.Context, tools *tool.Registry, name string, input tool.Input) (tool.Result, error) {
	if tools == nil {
		return tool.Result{}, fmt.Errorf("tool registry is not configured")
	}
	definition, ok := tools.Get(name)
	if !ok {
		return tool.Result{}, fmt.Errorf("tool not found: %s", name)
	}
	return definition.Call(ctx, input, tool.Runtime{})
}

// StringifyToolContent converts tool content to a string
func StringifyToolContent(content any) string {
	switch value := content.(type) {
	case string:
		return value
	case []string:
		return strings.Join(value, "\n")
	default:
		return fmt.Sprintf("%v", value)
	}
}

// formatCommandUsage is an alias for backward compatibility
func formatCommandUsage(name, hint string) string { return FormatCommandUsage(name, hint) }

// emptyDash is an alias for backward compatibility
func emptyDash(s string) string { return EmptyDash(s) }

// parseToggle is an alias for backward compatibility
func parseToggle(s string) (bool, error) { return ParseToggle(s) }

// shellQuote is an alias for backward compatibility
func shellQuote(value string) string { return ShellQuote(value) }

// callNamedTool is an alias for backward compatibility
func callNamedTool(ctx context.Context, tools *tool.Registry, name string, input tool.Input) (tool.Result, error) {
	return CallNamedTool(ctx, tools, name, input)
}

// stringifyToolContent is an alias for backward compatibility
func stringifyToolContent(content any) string { return StringifyToolContent(content) }
