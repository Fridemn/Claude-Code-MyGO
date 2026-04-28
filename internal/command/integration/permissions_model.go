package integration

import (
	"context"
	"fmt"
	"strings"

	"claude-go/internal/command"
	bashperm "claude-go/internal/tool/bash"

	tea "github.com/charmbracelet/bubbletea"
)

type permissionsModel struct {
	rt     command.Runtime
	lines  []string
	width  int
	height int
	offset int
}

func loadPermissionsModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	m := permissionsModel{rt: rt}
	m.refresh()
	return m, nil
}

func (m *permissionsModel) refresh() {
	checker := bashperm.GetPermissionChecker()
	mode := checker.GetMode()
	rules := checker.RulesSnapshot()

	lines := []string{
		"Permissions",
		fmt.Sprintf("mode=%s", mode),
		fmt.Sprintf("rules=%d", len(rules)),
		"",
		"Mode shortcuts:",
		"1 ask",
		"2 acceptEdits",
		"3 bypassPermissions",
		"",
		"Rules:",
	}
	if len(rules) == 0 {
		lines = append(lines, "- (no custom rules)")
	}
	for _, rule := range rules {
		lines = append(lines, fmt.Sprintf("- [%s] %s  source=%s", permissionBehaviorLabel(rule.Behavior), rule.Pattern, emptyDash(rule.Source)))
	}

	if m.rt.State != nil {
		state := m.rt.State.Snapshot()
		lines = append(lines, "", "Session:", "cwd="+emptyDash(state.CWD))
	}

	m.lines = lines
	if m.offset > maxOffset(len(m.lines), m.bodyHeight()) {
		m.offset = maxOffset(len(m.lines), m.bodyHeight())
	}
}

func (m permissionsModel) Init() tea.Cmd { return nil }

func (m permissionsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.offset > maxOffset(len(m.lines), m.bodyHeight()) {
			m.offset = maxOffset(len(m.lines), m.bodyHeight())
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
			if m.offset < maxOffset(len(m.lines), m.bodyHeight()) {
				m.offset++
			}
			return m, nil
		case tea.KeyEsc, tea.KeyCtrlC:
			notifyPermissionsDone(m.rt, "", command.LocalJSXDoneOptions{Display: "skip"})
			return m, tea.Quit
		}
		key := strings.ToLower(strings.TrimSpace(msg.String()))
		switch key {
		case "j":
			if m.offset < maxOffset(len(m.lines), m.bodyHeight()) {
				m.offset++
			}
		case "k":
			if m.offset > 0 {
				m.offset--
			}
		case "r":
			m.refresh()
		case "1":
			bashperm.SetGlobalPermissionMode(bashperm.PermissionModeAsk)
			m.refresh()
		case "2":
			bashperm.SetGlobalPermissionMode(bashperm.PermissionModeAcceptEdits)
			m.refresh()
		case "3":
			bashperm.SetGlobalPermissionMode(bashperm.PermissionModeBypassPermissions)
			m.refresh()
		case "q":
			notifyPermissionsDone(m.rt, "", command.LocalJSXDoneOptions{Display: "skip"})
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

func (m permissionsModel) View() string {
	if len(m.lines) == 0 {
		return "Permissions\n\nNo data.\n\nEsc close"
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

	visible := strings.Join(m.lines[start:end], "\n")
	footer := "j/k scroll · r refresh · 1/2/3 mode · Esc close"
	return strings.TrimRight(visible, "\n") + "\n\n" + footer
}

func (m permissionsModel) bodyHeight() int {
	if m.height <= 0 {
		return len(m.lines)
	}
	h := m.height - 2
	if h < 6 {
		return 6
	}
	return h
}

func maxOffset(total, body int) int {
	if total <= body {
		return 0
	}
	return total - body
}

func permissionBehaviorLabel(v bashperm.PermissionBehavior) string {
	switch v {
	case bashperm.BehaviorAllow:
		return "allow"
	case bashperm.BehaviorDeny:
		return "deny"
	default:
		return "ask"
	}
}

func notifyPermissionsDone(rt command.Runtime, result string, options command.LocalJSXDoneOptions) {
	if rt.OnLocalJSXDone != nil {
		rt.OnLocalJSXDone(result, options)
		return
	}
	if rt.OnExit != nil {
		rt.OnExit()
	}
}

func emptyDash(v string) string {
	if strings.TrimSpace(v) == "" {
		return "-"
	}
	return v
}
