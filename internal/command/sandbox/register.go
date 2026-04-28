package sandbox

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

// TS src/commands/sandbox-toggle/sandbox-toggle.tsx
// For Go, sandboxing requires OS-level support. We create a stub that
// reports status and handles the exclude subcommand.

func Register(r *command.Registry) {
	registerSandboxToggle(r)
}

// isSupportedPlatform checks if sandbox is supported
// TS src/utils/sandbox/sandbox-adapter.ts:isSupportedPlatform()
func isSupportedPlatform() bool {
	goos := runtime.GOOS
	// macOS, Linux are supported. WSL detection requires checking for WSL
	// which is complex, so we simplify to just check OS.
	return goos == "darwin" || goos == "linux"
}

func registerSandboxToggle(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "sandbox-toggle",
		Description: "toggle sandbox settings",
		Load:        loadSandboxModel,
		Handler: func(ctx context.Context, runtime command.Runtime, args []string) (string, error) {
			return handleSandboxCommand(args, runtime), nil
		},
	})
}

func handleSandboxCommand(args []string, rt command.Runtime) string {
	// Check platform support
	if !isSupportedPlatform() {
		if isWSL() {
			return "Error: Sandboxing requires WSL2. WSL1 is not supported."
		}
		return fmt.Sprintf("Error: Sandboxing is currently only supported on macOS, Linux, and WSL2. Current platform: %s", runtime.GOOS)
	}

	argsStr := ""
	if len(args) > 0 {
		argsStr = strings.Join(args, " ")
	}
	argsStr = strings.TrimSpace(argsStr)

	// If no args, show current status
	if argsStr == "" {
		return buildSandboxStatus(rt)
	}

	// Handle exclude subcommand
	// TS src/commands/sandbox-toggle/sandbox-toggle.tsx:exclude handling
	parts := strings.Split(argsStr, " ")
	subcommand := parts[0]

	if subcommand == "exclude" {
		commandPattern := strings.TrimSpace(strings.TrimPrefix(argsStr, "exclude "))
		if commandPattern == "" {
			return "Error: Please provide a command pattern to exclude (e.g., /sandbox exclude \"npm run test:*\")"
		}

		// Remove quotes if present
		cleanPattern := strings.ReplaceAll(strings.ReplaceAll(commandPattern, "\"", ""), "'", "")

		// Store excluded command in state
		if rt.State != nil {
			rt.State.SetSessionFlag("sandbox_exclude_"+cleanPattern, true)
		}

		return fmt.Sprintf("Added \"%s\" to excluded commands", cleanPattern)
	}

	return fmt.Sprintf("Error: Unknown subcommand \"%s\". Available subcommand: exclude", subcommand)
}

func buildSandboxStatus(rt command.Runtime) string {
	var lines []string
	lines = append(lines, "Sandbox Status")

	// Platform info
	lines = append(lines, "", fmt.Sprintf("Platform: %s", runtime.GOOS))
	if isSupportedPlatform() {
		lines = append(lines, "Supported: yes")
	} else {
		lines = append(lines, "Supported: no (requires macOS, Linux, or WSL2)")
	}

	// Sandbox state
	lines = append(lines, "", "Settings:")
	if rt.State != nil {
		// Check for excluded commands
		state := rt.State.Snapshot()
		excludedCount := 0
		for key := range state.SessionFlags {
			if strings.HasPrefix(key, "sandbox_exclude_") {
				excludedCount++
			}
		}
		if excludedCount > 0 {
			lines = append(lines, fmt.Sprintf("  excluded_commands: %d patterns", excludedCount))
		} else {
			lines = append(lines, "  excluded_commands: none")
		}
	}

	// Note about implementation status
	lines = append(lines, "", "Note: Full sandbox isolation requires OS-level process isolation.")
	lines = append(lines, "Currently, excluded commands are tracked but not enforced in Go version.")

	return strings.Join(lines, "\n")
}

// isWSL checks if running under WSL
func isWSL() bool {
	// Check for WSL indicators in /proc/version or environment
	if _, err := os.Stat("/proc/version"); err == nil {
		data, err := os.ReadFile("/proc/version")
		if err == nil && strings.Contains(string(data), "WSL") {
			return true
		}
	}
	return os.Getenv("WSL_DISTRO_NAME") != ""
}

type sandboxModel struct {
	rt     command.Runtime
	result string
}

func loadSandboxModel(_ context.Context, rt command.Runtime, args []string) (tea.Model, error) {
	m := sandboxModel{rt: rt}
	m.result = handleSandboxCommand(args, rt)
	return m, nil
}

func (m sandboxModel) Init() tea.Cmd { return nil }

func (m sandboxModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC, tea.KeyEnter:
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m sandboxModel) View() string {
	return m.result + "\n\nPress Enter or Esc to close"
}