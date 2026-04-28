package context

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

// TS src/commands/context/context.tsx
// Simplified for personal users - shows context usage summary

// Model context windows (approximate)
// TS src/utils/context.ts:getContextWindowForModel
var modelContextWindows = map[string]int{
	"claude-opus-4-6":     200000,
	"claude-sonnet-4-6":   200000,
	"claude-sonnet-4":     200000,
	"claude-haiku-4-5":    200000,
	"gpt-4o":              128000,
	"gpt-4-turbo":         128000,
	"gpt-3.5-turbo":       16385,
	"default":             128000,
}

func Register(r *command.Registry) {
	registerContext(r)
}

// estimateTokens provides rough token estimation
// 1 token ≈ 4 characters for English text (approximation)
func estimateTokens(text string) int {
	// Simple estimation: ~4 chars per token for English
	// This is a rough estimate; actual tokenization depends on model
	return len(text) / 4
}

// estimateMessageTokens estimates tokens for a message
func estimateMessageTokens(role, content string) int {
	// Base overhead for message structure (~4 tokens for role, formatting)
	baseOverhead := 4
	contentTokens := estimateTokens(content)
	return baseOverhead + contentTokens
}

// getContextWindowSize returns context window for model
func getContextWindowSize(model string) int {
	modelLower := strings.ToLower(model)
	for key, size := range modelContextWindows {
		if strings.Contains(modelLower, strings.ToLower(key)) {
			return size
		}
	}
	return modelContextWindows["default"]
}

func registerContext(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "context",
		Description: "show context usage summary",
		Load:        loadContextModel,
		Handler: func(ctx context.Context, rt command.Runtime, args []string) (string, error) {
			return buildContextSummary(rt), nil
		},
	})
}

func buildContextSummary(rt command.Runtime) string {
	var lines []string
	lines = append(lines, "Context Usage Summary")
	lines = append(lines, "")

	// Model info
	model := ""
	if rt.State != nil {
		model = rt.State.Snapshot().CurrentModel
	}
	if model == "" && rt.Config.Model != "" {
		model = rt.Config.Model
	}
	if model == "" {
		model = "unknown"
	}

	contextWindow := getContextWindowSize(model)
	lines = append(lines, fmt.Sprintf("Model: %s", model))
	lines = append(lines, fmt.Sprintf("Context Window: %d tokens", contextWindow))
	lines = append(lines, "")

	// System prompt
	sysPrompt := rt.Config.SystemPrompt
	if sysPrompt != "" {
		sysPromptTokens := estimateTokens(sysPrompt)
		lines = append(lines, fmt.Sprintf("System Prompt: ~%d tokens (%d chars)", sysPromptTokens, len(sysPrompt)))
	} else {
		lines = append(lines, "System Prompt: default")
	}
	lines = append(lines, "")

	// Tools
	if rt.Tools != nil {
		tools := rt.Tools.List()
		toolCount := len(tools)
		readOnlyCount := 0
		for _, t := range tools {
			if t.IsReadOnly(nil) {
				readOnlyCount++
			}
		}
		lines = append(lines, fmt.Sprintf("Tools: %d total (%d read-only)", toolCount, readOnlyCount))
	} else {
		lines = append(lines, "Tools: not configured")
	}
	lines = append(lines, "")

	// Additional directories
	if rt.State != nil {
		additionalDirs := rt.State.GetAdditionalDirectories()
		if len(additionalDirs) > 0 {
			lines = append(lines, fmt.Sprintf("Additional Dirs: %d", len(additionalDirs)))
			for _, dir := range additionalDirs {
				lines = append(lines, fmt.Sprintf("  - %s", dir))
			}
		} else {
			lines = append(lines, "Additional Dirs: none")
		}
	}
	lines = append(lines, "")

	// Session stats
	if rt.State != nil {
		state := rt.State.Snapshot()
		lines = append(lines, "Session Stats:")
		lines = append(lines, fmt.Sprintf("  Turns: %d", state.TurnCount))
		lines = append(lines, fmt.Sprintf("  Tool Calls: %d", state.ToolCallCount))
		if state.TotalCostUSD > 0 {
			lines = append(lines, fmt.Sprintf("  Cost: $%.4f", state.TotalCostUSD))
		}
	}

	// Usage hint
	lines = append(lines, "")
	lines = append(lines, "Tip: Use /compact to reduce context size if approaching limits")

	return strings.Join(lines, "\n")
}

type contextModel struct {
	rt     command.Runtime
	output string
}

func loadContextModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	m := contextModel{rt: rt}
	m.output = buildContextSummary(rt)
	return m, nil
}

func (m contextModel) Init() tea.Cmd { return nil }

func (m contextModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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

func (m contextModel) View() string {
	return m.output + "\n\nPress Enter or Esc to close"
}