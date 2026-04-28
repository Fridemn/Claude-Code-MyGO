package stats

import (
	"context"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

type effortModel struct {
	rt     command.Runtime
	result string
}

func loadEffortModel(_ context.Context, rt command.Runtime, args []string) (tea.Model, error) {
	m := effortModel{rt: rt}
	// Process args and get result
	argsStr := ""
	if len(args) > 0 {
		argsStr = args[0]
	}

	// Handle help args
	for _, helpArg := range []string{"help", "-h", "--help"} {
		if argsStr == helpArg {
			m.result = `Usage: /effort [low|medium|high|max|auto]

Effort levels:
- low: Quick, straightforward implementation with minimal overhead
- medium: Balanced approach with standard implementation and testing
- high: Comprehensive implementation with extensive testing and documentation
- max: Maximum capability with deepest reasoning (Opus 4.6 only)
- auto: Use the default effort level for your model`
			return m, nil
		}
	}

	// Handle current/status or no args
	if argsStr == "" || argsStr == "current" || argsStr == "status" {
		appStateEffort := getEffortFromState(rt)
		model := ""
		if rt.State != nil {
			model = rt.State.Snapshot().CurrentModel
		}
		m.result = showCurrentEffort(appStateEffort, model)
		return m, nil
	}

	m.result = executeEffort(argsStr, rt)
	return m, nil
}

func (m effortModel) Init() tea.Cmd { return nil }

func (m effortModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m effortModel) View() string {
	return m.result + "\n\nPress Enter or Esc to close"
}