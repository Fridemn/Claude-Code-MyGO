package hooks

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"claude-go/internal/command"
)

// TS src/commands/hooks/hooks.tsx
// Hooks configuration command for personal users

func Register(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "hooks",
		Description: "View and manage hooks configuration",
		Handler:     handleHooksCommand,
	})
}

func handleHooksCommand(ctx context.Context, rt command.Runtime, args []string) (string, error) {
	// TS hooks.tsx: logEvent('tengu_hooks_command', {})

	// Build hooks info
	// TS hooks.tsx: return <HooksConfigMenu toolNames={toolNames} onExit={onDone} />
	return buildHooksInfo(rt), nil
}

// buildHooksInfo returns hooks configuration info
// TS hooks.tsx: HooksConfigMenu component
func buildHooksInfo(rt command.Runtime) string {
	lines := []string{
		"Hooks Configuration",
		"",
	}

	// Get hooks config path
	// TS hooks.tsx: uses context for appState
	configPath := getHooksConfigPath(rt)
	lines = append(lines, fmt.Sprintf("Config file: %s", configPath))
	lines = append(lines, "")

	// Check if config exists
	configExists := false
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
	}

	if configExists {
		// Read and display hooks config
		hooks := readHooksConfig(configPath)
		if len(hooks) > 0 {
			lines = append(lines, fmt.Sprintf("Defined hooks (%d):", len(hooks)))
			for name, hook := range hooks {
				lines = append(lines, fmt.Sprintf("  - %s: %s", name, hook))
			}
		} else {
			lines = append(lines, "No hooks defined in config file.")
		}
	} else {
		lines = append(lines, "Hooks config file does not exist.")
		lines = append(lines, "Create it at: ~/.claude/hooks.json")
	}

	// Show available tool names (TS: toolNames = getTools(permissionContext).map(tool => tool.name))
	lines = append(lines, "")
	lines = append(lines, "Hook triggers available:")
	lines = append(lines, "  - PreToolUse: Before tool execution")
	lines = append(lines, "  - PostToolUse: After tool execution")
	lines = append(lines, "  - Notification: When notification is received")
	lines = append(lines, "  - Stop: When session stops")
	lines = append(lines, "")

	// Show example hook config
	lines = append(lines, "Example hooks.json:")
	example := `{
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": ["echo 'Running Bash command'"]
      }
    ],
    "PostToolUse": [
      {
        "matcher": "Edit",
        "hooks": ["echo 'File edited'"]
      }
    ]
  }
}`
	for _, line := range strings.Split(example, "\n") {
		lines = append(lines, fmt.Sprintf("  %s", line))
	}

	return strings.Join(lines, "\n")
}

// getHooksConfigPath returns the hooks config file path
// TS hooks.tsx: getHooksConfigPath from config
func getHooksConfigPath(rt command.Runtime) string {
	// Check config for hooks path
	if rt.Config.HooksConfigPath != "" {
		return rt.Config.HooksConfigPath
	}

	// Default path: ~/.claude/hooks.json
	home, err := os.UserHomeDir()
	if err != nil {
		return "hooks.json"
	}
	return filepath.Join(home, ".claude", "hooks.json")
}

// readHooksConfig reads hooks configuration
func readHooksConfig(path string) map[string]string {
	hooks := make(map[string]string)

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return hooks
	}

	// Parse as JSON
	// Standard hooks config format: { "hooks": { "PreToolUse": [...], ... } }
	var rawConfig map[string]interface{}
	if err := parseJSON(data, &rawConfig); err != nil {
		return hooks
	}

	// Get hooks section
	hooksSection, ok := rawConfig["hooks"].(map[string]interface{})
	if !ok {
		return hooks
	}

	// Parse each hook type
	for hookType, hookData := range hooksSection {
		// Hook data is usually an array
		if arr, ok := hookData.([]interface{}); ok {
			count := len(arr)
			hooks[hookType] = fmt.Sprintf("%d hook(s) defined", count)
		} else {
			hooks[hookType] = "configured"
		}
	}

	return hooks
}

// parseJSON is a simple JSON parser helper
func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}