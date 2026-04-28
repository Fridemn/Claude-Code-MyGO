package skills

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

type skillsModel struct {
	rt     command.Runtime
	lines  []string
	width  int
	height int
	offset int
}

func loadSkillsModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	m := skillsModel{rt: rt}
	m.refresh()
	return m, nil
}

func (m *skillsModel) refresh() {
	content := strings.TrimSpace(renderSkillsOverview(m.rt))
	if content == "" {
		m.lines = []string{"Skills", "", "No data."}
	} else {
		m.lines = strings.Split(content, "\n")
	}
	if m.offset > skillsMaxOffset(len(m.lines), m.bodyHeight()) {
		m.offset = skillsMaxOffset(len(m.lines), m.bodyHeight())
	}
}

func (m skillsModel) Init() tea.Cmd { return nil }

func (m skillsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		if m.offset > skillsMaxOffset(len(m.lines), m.bodyHeight()) {
			m.offset = skillsMaxOffset(len(m.lines), m.bodyHeight())
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
			if m.offset < skillsMaxOffset(len(m.lines), m.bodyHeight()) {
				m.offset++
			}
			return m, nil
		case tea.KeyEsc, tea.KeyCtrlC:
			if m.rt.OnLocalJSXDone != nil {
				m.rt.OnLocalJSXDone("Skills dialog dismissed", command.LocalJSXDoneOptions{
					Display: "system",
				})
			} else if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
		switch strings.ToLower(strings.TrimSpace(msg.String())) {
		case "j":
			if m.offset < skillsMaxOffset(len(m.lines), m.bodyHeight()) {
				m.offset++
			}
		case "k":
			if m.offset > 0 {
				m.offset--
			}
		case "r":
			m.refresh()
		case "q":
			if m.rt.OnLocalJSXDone != nil {
				m.rt.OnLocalJSXDone("Skills dialog dismissed", command.LocalJSXDoneOptions{
					Display: "system",
				})
			} else if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
		return m, nil
	}
	return m, nil
}

func (m skillsModel) View() string {
	if len(m.lines) == 0 {
		return "Skills\n\nNo data.\n\nEsc close"
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
	footer := "j/k scroll · r refresh · Esc close"
	return strings.TrimRight(visible, "\n") + "\n\n" + footer
}

func (m skillsModel) bodyHeight() int {
	if m.height <= 0 {
		return len(m.lines)
	}
	h := m.height - 2
	if h < 6 {
		return 6
	}
	return h
}

func skillsMaxOffset(total, body int) int {
	if total <= body {
		return 0
	}
	return total - body
}

func renderSkillsOverview(runtime command.Runtime) string {
	lines := []string{
		"Skills",
		"registry=skills",
	}
	skills := []command.SkillInfo(nil)
	if runtime.SkillList != nil {
		skills = runtime.SkillList()
	}
	lines = append(lines, fmt.Sprintf("entries=%d", len(skills)))

	if len(skills) > 0 {
		sorted := append([]command.SkillInfo(nil), skills...)
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Source == sorted[j].Source {
				return strings.ToLower(sorted[i].Name) < strings.ToLower(sorted[j].Name)
			}
			return sourceRank(sorted[i].Source) < sourceRank(sorted[j].Source)
		})
		currentSource := ""
		for _, skill := range sorted {
			if skill.Source != currentSource {
				currentSource = skill.Source
				lines = append(lines, "", sourceTitle(currentSource)+":")
			}
			name := skill.Name
			if strings.TrimSpace(skill.DisplayName) != "" && skill.DisplayName != skill.Name {
				name = skill.DisplayName + " (" + skill.Name + ")"
			}
			meta := []string{}
			if strings.TrimSpace(skill.DisplayName) != "" {
				meta = append(meta, "display_name="+skill.DisplayName)
			}
			if !skill.UserInvocable {
				meta = append(meta, "hidden=true")
			}
			if len(skill.Aliases) > 0 {
				meta = append(meta, "aliases="+strings.Join(skill.Aliases, ","))
			}
			if strings.TrimSpace(skill.WhenToUse) != "" {
				meta = append(meta, "when_to_use="+skill.WhenToUse)
			}
			if strings.TrimSpace(skill.Path) != "" {
				meta = append(meta, "path="+skill.Path)
			}
			line := "- " + name
			if len(meta) > 0 {
				line += "  " + strings.Join(meta, "  ")
			}
			lines = append(lines, line)
			if strings.TrimSpace(skill.Description) != "" {
				lines = append(lines, "  "+skill.Description)
			}
		}
	}

	if runtime.SkillStatus != nil {
		status := strings.TrimSpace(runtime.SkillStatus())
		if status != "" {
			lines = append(lines, "", "status:", status)
		}
	}
	lines = append(lines, "", "actions:", "- /skills", "- /reload-plugins")
	return strings.Join(lines, "\n")
}

func sourceTitle(source string) string {
	switch strings.TrimSpace(source) {
	case "projectSettings":
		return "Project skills"
	case "userSettings":
		return "User skills"
	case "policySettings":
		return "Policy skills"
	case "plugin":
		return "Plugin skills"
	case "mcp":
		return "MCP skills"
	case "bundled":
		return "Bundled skills"
	default:
		if strings.TrimSpace(source) == "" {
			return "Skills"
		}
		return source + " skills"
	}
}

func sourceRank(source string) int {
	switch strings.TrimSpace(source) {
	case "projectSettings":
		return 0
	case "userSettings":
		return 1
	case "policySettings":
		return 2
	case "plugin":
		return 3
	case "mcp":
		return 4
	case "bundled":
		return 5
	default:
		return 100
	}
}
