package session

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

type configTab int

const (
	configTabStatus configTab = iota
	configTabConfig
	configTabUsage
)

type configModel struct {
	rt     command.Runtime
	tab    configTab
	lines  []string
	width  int
	height int
	offset int
}

func loadConfigModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	m := configModel{
		rt:  rt,
		tab: configTabConfig, // TS /config defaults to Config tab.
	}
	m.refresh()
	return m, nil
}

func (m *configModel) refresh() {
	switch m.tab {
	case configTabStatus:
		m.lines = renderConfigStatusLines(m.rt)
	case configTabUsage:
		m.lines = renderConfigUsageLines(m.rt)
	default:
		m.lines = strings.Split(renderConfigSummary(m.rt), "\n")
	}
	if m.offset > configMaxOffset(len(m.lines), m.bodyHeight()) {
		m.offset = configMaxOffset(len(m.lines), m.bodyHeight())
	}
}

func (m configModel) Init() tea.Cmd { return nil }

func (m configModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.offset > configMaxOffset(len(m.lines), m.bodyHeight()) {
			m.offset = configMaxOffset(len(m.lines), m.bodyHeight())
		}
		return m, nil
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if m.offset > 0 {
				m.offset--
			}
			return m, nil
		case tea.KeyDown:
			if m.offset < configMaxOffset(len(m.lines), m.bodyHeight()) {
				m.offset++
			}
			return m, nil
		case tea.KeyLeft:
			m.selectPrevTab()
			return m, nil
		case tea.KeyRight:
			m.selectNextTab()
			return m, nil
		case tea.KeyEsc, tea.KeyCtrlC:
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
		switch strings.ToLower(strings.TrimSpace(msg.String())) {
		case "j":
			if m.offset < configMaxOffset(len(m.lines), m.bodyHeight()) {
				m.offset++
			}
		case "k":
			if m.offset > 0 {
				m.offset--
			}
		case "h":
			m.selectPrevTab()
		case "l":
			m.selectNextTab()
		case "1":
			m.selectTab(configTabStatus)
		case "2":
			m.selectTab(configTabConfig)
		case "3":
			m.selectTab(configTabUsage)
		case "r":
			m.refresh()
		case "q":
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

func (m configModel) View() string {
	if len(m.lines) == 0 {
		return "Settings\n\nNo data.\n\nEsc close"
	}
	bodyH := m.bodyHeight()
	start := m.offset
	if start < 0 {
		start = 0
	}
	end := start + bodyH
	if end > len(m.lines) {
		end = len(m.lines)
	}
	if bodyH <= 0 {
		start = 0
		end = len(m.lines)
	}

	tabLine := renderConfigTabs(m.tab)
	visible := strings.Join(m.lines[start:end], "\n")
	footer := "1/2/3 tab · h/l switch · j/k scroll · r refresh · Esc close"
	return tabLine + "\n\n" + strings.TrimRight(visible, "\n") + "\n\n" + footer
}

func (m configModel) bodyHeight() int {
	if m.height <= 0 {
		return len(m.lines)
	}
	h := m.height - 4
	if h < 6 {
		return 6
	}
	return h
}

func (m *configModel) selectPrevTab() {
	switch m.tab {
	case configTabStatus:
		m.selectTab(configTabUsage)
	case configTabConfig:
		m.selectTab(configTabStatus)
	default:
		m.selectTab(configTabConfig)
	}
}

func (m *configModel) selectNextTab() {
	switch m.tab {
	case configTabStatus:
		m.selectTab(configTabConfig)
	case configTabConfig:
		m.selectTab(configTabUsage)
	default:
		m.selectTab(configTabStatus)
	}
}

func (m *configModel) selectTab(tab configTab) {
	if m.tab == tab {
		return
	}
	m.tab = tab
	m.offset = 0
	m.refresh()
}

func configMaxOffset(total, body int) int {
	if total <= body {
		return 0
	}
	return total - body
}

func renderConfigTabs(tab configTab) string {
	status := " Status "
	config := " Config "
	usage := " Usage "
	switch tab {
	case configTabStatus:
		status = "[Status]"
	case configTabConfig:
		config = "[Config]"
	case configTabUsage:
		usage = "[Usage]"
	}
	return "Settings " + status + " " + config + " " + usage
}

func renderConfigSummary(runtime command.Runtime) string {
	if runtime.Engine == nil {
		return "engine is not configured"
	}
	msgs := runtime.Engine.Messages()
	lines := []string{
		"Config",
		fmt.Sprintf("app=%s", command.EmptyDash(runtime.Config.AppName)),
		fmt.Sprintf("session=%s", command.EmptyDash(runtime.Engine.SessionID())),
		fmt.Sprintf("model=%s", command.EmptyDash(runtime.Config.Model)),
		fmt.Sprintf("base_url=%s", command.EmptyDash(runtime.Config.BaseURL)),
		fmt.Sprintf("max_turns=%d", runtime.Config.MaxTurns),
		fmt.Sprintf("session_dir=%s", command.EmptyDash(runtime.Config.SessionDir)),
		fmt.Sprintf("messages=%d", len(msgs)),
	}
	return strings.Join(lines, "\n")
}

func renderConfigStatusLines(runtime command.Runtime) []string {
	lines := []string{
		"Status",
		fmt.Sprintf("skills=%s", maybeStatus(runtime.SkillStatus)),
		fmt.Sprintf("hooks=%s", maybeStatus(runtime.HookStatus)),
		fmt.Sprintf("plugins=%s", maybeStatus(runtime.PluginStatus)),
		fmt.Sprintf("mcp=%s", maybeStatus(runtime.MCPStatus)),
	}
	if runtime.State != nil {
		state := runtime.State.Snapshot()
		lines = append(lines,
			fmt.Sprintf("cwd=%s", command.EmptyDash(state.CWD)),
			fmt.Sprintf("project_root=%s", command.EmptyDash(state.ProjectRoot)),
			fmt.Sprintf("session=%s", command.EmptyDash(state.SessionID)),
		)
	}
	return lines
}

func renderConfigUsageLines(runtime command.Runtime) []string {
	if runtime.State == nil {
		return []string{"Usage", "state store is not configured"}
	}
	state := runtime.State.Snapshot()
	lines := []string{
		"Usage",
		fmt.Sprintf("turns=%d", state.TurnCount),
		fmt.Sprintf("tool_calls=%d", state.ToolCallCount),
		fmt.Sprintf("api_duration=%s", state.TotalAPIDuration),
		fmt.Sprintf("tool_duration=%s", state.TotalToolDuration),
		fmt.Sprintf("total_cost_usd=%.6f", state.TotalCostUSD),
	}
	keys := make([]string, 0, len(state.ModelUsage))
	for model := range state.ModelUsage {
		keys = append(keys, model)
	}
	sort.Strings(keys)
	if len(keys) > 0 {
		lines = append(lines, "", "models:")
		for _, model := range keys {
			lines = append(lines, fmt.Sprintf("- %s: %d", model, state.ModelUsage[model]))
		}
	}
	return lines
}

func maybeStatus(provider func() string) string {
	if provider == nil {
		return "not configured"
	}
	status := strings.TrimSpace(provider())
	if status == "" {
		return "configured"
	}
	return strings.ReplaceAll(status, "\n", " | ")
}
