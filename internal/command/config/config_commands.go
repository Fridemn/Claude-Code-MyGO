package config

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/command"
	"claude-go/internal/config"
)

// registerVim registers the /vim command for toggling editor mode
// Ported from src/commands/vim/vim.ts
func registerVim(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "vim",
		Description: "Toggle between vim and normal editor mode",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			// Handle backward compatibility - treat 'emacs' as 'normal'
			currentMode := runtime.Config.EditorMode
			if currentMode == "" || currentMode == "emacs" {
				currentMode = config.EditorModeNormal
			}

			// Toggle mode
			newMode := config.EditorModeNormal
			if currentMode == config.EditorModeNormal {
				newMode = config.EditorModeVim
			}

			// Update config (via state if available)
			if runtime.State != nil {
				runtime.State.SetEditorMode(newMode)
			}

			// Build response message
			var hint string
			if newMode == config.EditorModeVim {
				hint = "Use Escape key to toggle between INSERT and NORMAL modes."
			} else {
				hint = "Using standard (readline) keyboard bindings."
			}

			return fmt.Sprintf("Editor mode set to %s. %s", newMode, hint), nil
		},
	})
}

// registerColor registers the /color command for setting agent color
// Ported from src/commands/color/color.ts
func registerColor(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "color",
		Description:  "Set the session color for multi-agent swarms",
		ArgumentHint: "<color>",
		Aliases:      []string{"agent-color"},
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
				colorList := strings.Join(config.AgentColors, ", ")
				return fmt.Sprintf("Please provide a color. Available colors: %s, default", colorList), nil
			}

			colorArg := strings.ToLower(strings.TrimSpace(args[0]))

			// Handle reset to default
			resetAliases := []string{"default", "reset", "none", "gray", "grey"}
			for _, alias := range resetAliases {
				if colorArg == alias {
					if runtime.State != nil {
						runtime.State.SetAgentColor("")
					}
					return "Session color reset to default", nil
				}
			}

			// Validate color
			validColor := false
			for _, c := range config.AgentColors {
				if c == colorArg {
					validColor = true
					break
				}
			}

			if !validColor {
				colorList := strings.Join(config.AgentColors, ", ")
				return fmt.Sprintf("Invalid color \"%s\". Available colors: %s, default", colorArg, colorList), nil
			}

			// Set color
			if runtime.State != nil {
				runtime.State.SetAgentColor(colorArg)
			}

			return fmt.Sprintf("Session color set to: %s", colorArg), nil
		},
	})
}

// registerKeybindings registers the /keybindings command
// Ported from src/commands/keybindings/keybindings.ts
func registerKeybindings(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "keybindings",
		Description:  "Open keybindings configuration file in editor",
		ArgumentHint: "[preview]",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			// Check if preview mode
			preview := len(args) > 0 && strings.ToLower(args[0]) == "preview"

			// Get keybindings path
			keybindingsPath := getKeybindingsPath(runtime)

			if preview {
				// Show current keybindings info
				return fmt.Sprintf("Keybindings file: %s\nEdit with: /keybindings", keybindingsPath), nil
			}

			// In Go version, we return a message directing user to edit manually
			// Full editor integration would require exec editor command
			return fmt.Sprintf("Keybindings configuration at: %s\nOpen this file in your editor to customize keybindings.", keybindingsPath), nil
		},
	})
}

func getKeybindingsPath(runtime command.Runtime) string {
	// Default path is ~/.claude/keybindings.json
	home, err := command.GetHomeDir()
	if err == nil && home != "" {
		return home + "/.claude/keybindings.json"
	}
	// Fallback to session dir if available
	if runtime.Config.SessionDir != "" {
		return runtime.Config.SessionDir + "/keybindings.json"
	}
	return "keybindings.json"
}