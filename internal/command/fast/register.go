package fast

import (
	"context"
	"fmt"
	"os"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

// For personal users, fast mode is a simplified toggle that:
// 1. Tracks whether user wants faster responses
// 2. Can be used to switch to a configured "fast" model
//
// TS src/commands/fast/fast.tsx has complex OAuth/org-status logic
// which is not applicable for personal users.

// FastModeModelDisplay is the display name for fast mode model
// TS parity: src/utils/fastMode.ts:FAST_MODE_MODEL_DISPLAY
const FastModeModelDisplay = "fast model"

// isFastModeDisabled checks if fast mode is disabled via environment
// TS src/utils/fastMode.ts:isFastModeEnabled()
func isFastModeDisabled() bool {
	return os.Getenv("CLAUDE_CODE_DISABLE_FAST_MODE") == "1" ||
		os.Getenv("CLAUDE_CODE_DISABLE_FAST_MODE") == "true"
}

func Register(r *command.Registry) {
	registerFastMode(r)
}

func registerFastMode(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "fast",
		Description: "toggle fast mode for quicker responses",
		Load:        loadFastModel,
		Handler: func(ctx context.Context, rt command.Runtime, args []string) (string, error) {
			return handleFastCommand(args, rt), nil
		},
	})
}

func handleFastCommand(args []string, rt command.Runtime) string {
	// Check if fast mode is disabled via environment
	if isFastModeDisabled() {
		return "Fast mode is disabled via CLAUDE_CODE_DISABLE_FAST_MODE"
	}

	argsStr := ""
	if len(args) > 0 {
		argsStr = strings.Join(args, " ")
	}
	argsStr = strings.TrimSpace(strings.ToLower(argsStr))

	// Handle help args
	for _, helpArg := range []string{"help", "-h", "--help"} {
		if argsStr == helpArg {
			return `Fast mode

Usage: /fast [on|off]

Fast mode enables quicker responses by using optimized settings.

- on: Enable fast mode
- off: Disable fast mode
- (no args): Show current status and toggle

Note: For personal users using OpenAI-compatible APIs, fast mode
is a session-level preference. Configure your "fast model" in settings.`
		}
	}

	// Handle on/off args
	if argsStr == "on" {
		return enableFastMode(rt)
	}
	if argsStr == "off" {
		return disableFastMode(rt)
	}

	// No args: show current status
	return showFastModeStatus(rt)
}

func enableFastMode(rt command.Runtime) string {
	// Set fast mode in state
	if rt.State != nil {
		rt.State.SetSessionFlag("fast_mode", true)
	}

	// If model switch callback exists, use it
	if rt.OnModelChange != nil {
		fastModel := os.Getenv("CLAUDE_CODE_FAST_MODEL")
		if fastModel != "" {
			rt.OnModelChange(fastModel)
			return fmt.Sprintf("⚡ Fast mode ON · model set to %s", fastModel)
		}
	}

	return "⚡ Fast mode ON"
}

func disableFastMode(rt command.Runtime) string {
	// Clear fast mode in state
	if rt.State != nil {
		rt.State.SetSessionFlag("fast_mode", false)
	}

	return "Fast mode OFF"
}

func showFastModeStatus(rt command.Runtime) string {
	var status string
	if rt.State != nil && rt.State.GetSessionFlag("fast_mode") {
		status = "ON"
	} else {
		status = "OFF"
	}

	fastModel := os.Getenv("CLAUDE_CODE_FAST_MODEL")
	if fastModel != "" {
		return fmt.Sprintf("Fast mode: %s (fast model: %s)\n\nTab to toggle · Enter to confirm · Esc to cancel", status, fastModel)
	}

	return fmt.Sprintf("Fast mode: %s\n\nTab to toggle · Enter to confirm · Esc to cancel", status)
}

func getFastModeFromState(rt command.Runtime) bool {
	if rt.State == nil {
		return false
	}
	return rt.State.GetSessionFlag("fast_mode")
}

type fastModel struct {
	rt          command.Runtime
	enableFast  bool
	unavailable string
}

func loadFastModel(_ context.Context, rt command.Runtime, args []string) (tea.Model, error) {
	m := fastModel{rt: rt}

	// Check if disabled
	if isFastModeDisabled() {
		m.unavailable = "Fast mode is disabled via environment"
		return m, nil
	}

	// Initial state from stored preference
	m.enableFast = getFastModeFromState(rt)

	// Handle direct args (on/off) - return after setting
	argsStr := ""
	if len(args) > 0 {
		argsStr = strings.TrimSpace(strings.ToLower(args[0]))
	}
	if argsStr == "on" || argsStr == "off" {
		m.enableFast = argsStr == "on"
		if m.enableFast {
			if rt.State != nil {
				rt.State.SetSessionFlag("fast_mode", true)
			}
			if rt.OnModelChange != nil {
				fastModel := os.Getenv("CLAUDE_CODE_FAST_MODEL")
				if fastModel != "" {
					rt.OnModelChange(fastModel)
				}
			}
		} else {
			if rt.State != nil {
				rt.State.SetSessionFlag("fast_mode", false)
			}
		}
		return m, nil
	}

	return m, nil
}

func (m fastModel) Init() tea.Cmd { return nil }

func (m fastModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		case tea.KeyEnter:
			// Apply the toggle
			if m.unavailable != "" {
				if m.rt.OnExit != nil {
					m.rt.OnExit()
				}
				return m, tea.Quit
			}
			if m.enableFast {
				if m.rt.State != nil {
					m.rt.State.SetSessionFlag("fast_mode", true)
				}
				if m.rt.OnModelChange != nil {
					fastModel := os.Getenv("CLAUDE_CODE_FAST_MODEL")
					if fastModel != "" {
						m.rt.OnModelChange(fastModel)
					}
				}
			} else {
				if m.rt.State != nil {
					m.rt.State.SetSessionFlag("fast_mode", false)
				}
			}
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		case tea.KeyTab:
			// Toggle
			if m.unavailable == "" {
				m.enableFast = !m.enableFast
			}
			return m, nil
		}
	}
	return m, nil
}

func (m fastModel) View() string {
	if m.unavailable != "" {
		return fmt.Sprintf("Fast mode\n\n%s\n\nEsc to close", m.unavailable)
	}

	status := "OFF"
	if m.enableFast {
		status = "ON"
	}

	fastModelEnv := os.Getenv("CLAUDE_CODE_FAST_MODEL")
	modelInfo := ""
	if fastModelEnv != "" {
		modelInfo = fmt.Sprintf(" (fast model: %s)", fastModelEnv)
	}

	return fmt.Sprintf(`Fast mode (research preview)

High-speed mode for quicker responses%s

Fast mode: %s

Tab to toggle · Enter to confirm · Esc to close`, modelInfo, status)
}