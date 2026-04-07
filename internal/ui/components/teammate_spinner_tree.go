package components

import (
	"fmt"
	"strconv"
	"strings"
)

type TeammateTask struct {
	Name           string
	Activity       string
	ToolUseCount   int
	TokenCount     int
	IsIdle         bool
	IsForegrounded bool
}

type TeammateSpinnerTreeConfig struct {
	Width           int
	SelectedIndex   int // -1 = leader, [0..n-1] = teammate, n = hide row
	SelectionMode   bool
	AllIdle         bool
	LeaderVerb      string
	LeaderTokenCount int
	LeaderIdleText  string
	Teammates       []TeammateTask
}

func RenderTeammateSpinnerTree(cfg TeammateSpinnerTreeConfig) string {
	width := cfg.Width
	if width <= 0 {
		width = 80
	}

	lines := make([]string, 0, len(cfg.Teammates)+2)
	lines = append(lines, renderLeaderLine(cfg, width))
	for i := range cfg.Teammates {
		lines = append(lines, renderTeammateLine(cfg, i, width))
	}
	if cfg.SelectionMode {
		lines = append(lines, renderHideLine(cfg, width))
	}
	return strings.Join(lines, "\n")
}

func renderLeaderLine(cfg TeammateSpinnerTreeConfig, width int) string {
	selected := cfg.SelectionMode && cfg.SelectedIndex == -1
	tree := "┌─"
	if selected {
		tree = "╭═"
	}
	prefix := renderIndicator(selected, cfg.SelectionMode) + " " + tree + " team-lead"

	activity := cfg.LeaderIdleText
	if activity == "" {
		activity = "idle"
	}
	if cfg.LeaderVerb != "" {
		activity = cfg.LeaderVerb + "…"
	}

	segments := []string{activity}
	if cfg.LeaderTokenCount > 0 {
		segments = append(segments, formatNumber(cfg.LeaderTokenCount)+" tokens")
	}
	if cfg.SelectionMode {
		segments = append(segments, "enter to view")
	}
	return fitSegments(prefix, segments, width)
}

func renderTeammateLine(cfg TeammateSpinnerTreeConfig, index, width int) string {
	selected := cfg.SelectionMode && cfg.SelectedIndex == index
	last := index == len(cfg.Teammates)-1 && !cfg.SelectionMode
	if cfg.SelectionMode {
		last = false
	}

	tree := "├─"
	if last {
		tree = "└─"
	}
	if selected {
		if index == len(cfg.Teammates)-1 {
			tree = "╘═"
		} else {
			tree = "╞═"
		}
	}

	node := cfg.Teammates[index]
	showName := width >= 60
	prefix := renderIndicator(selected, cfg.SelectionMode) + " " + tree
	if showName && strings.TrimSpace(node.Name) != "" {
		prefix += " @" + strings.TrimSpace(node.Name)
	}

	activity := strings.TrimSpace(node.Activity)
	if activity == "" {
		if node.IsIdle {
			activity = "idle"
		} else {
			activity = "working…"
		}
	}

	segments := []string{activity}
	if node.ToolUseCount > 0 {
		segments = append(segments, strconv.Itoa(node.ToolUseCount)+" use")
	}
	if node.TokenCount > 0 {
		segments = append(segments, formatNumber(node.TokenCount)+" tokens")
	}
	if cfg.SelectionMode {
		segments = append(segments, "enter to view")
	}
	return fitSegments(prefix, segments, width)
}

func renderHideLine(cfg TeammateSpinnerTreeConfig, width int) string {
	selected := cfg.SelectedIndex == len(cfg.Teammates)
	tree := "└─"
	if selected {
		tree = "╘═"
	}
	prefix := renderIndicator(selected, true) + " " + tree + " hide"
	return fitSegments(prefix, []string{"enter to collapse"}, width)
}

func renderIndicator(selected bool, selectionMode bool) string {
	if !selectionMode {
		return " "
	}
	if selected {
		return "❯"
	}
	return " "
}

func fitSegments(prefix string, segments []string, width int) string {
	if width <= 0 {
		width = 80
	}
	parts := append([]string{prefix}, segments...)
	line := strings.Join(parts, " · ")
	for treeVisibleWidth(line) > width && len(parts) > 2 {
		parts = parts[:len(parts)-1]
		line = strings.Join(parts, " · ")
	}
	if treeVisibleWidth(line) > width {
		line = treeTruncateVisible(line, width)
	}
	return line
}

func formatNumber(n int) string {
	s := strconv.Itoa(n)
	if len(s) <= 3 {
		return s
	}
	neg := false
	if n < 0 {
		neg = true
		s = s[1:]
	}
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	rem := len(s) % 3
	if rem > 0 {
		b.WriteString(s[:rem])
		if len(s) > rem {
			b.WriteByte(',')
		}
	}
	for i := rem; i < len(s); i += 3 {
		b.WriteString(s[i : i+3])
		if i+3 < len(s) {
			b.WriteByte(',')
		}
	}
	return b.String()
}

func treeVisibleWidth(s string) int {
	return len([]rune(s))
}

func treeTruncateVisible(s string, width int) string {
	if width <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= width {
		return s
	}
	if width == 1 {
		return string(r[:1])
	}
	return string(r[:width-1]) + "…"
}

func (t TeammateTask) String() string {
	return fmt.Sprintf("%s:%s", t.Name, t.Activity)
}
