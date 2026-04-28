package memory

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"claude-go/internal/command"
	memorypkg "claude-go/internal/memory"
	"claude-go/internal/types"

	tea "github.com/charmbracelet/bubbletea"
)

type memoryCandidate struct {
	Label       string
	Path        string
	Description string
	Exists      bool
}

type memoryEditorFinishedMsg struct {
	path         string
	editorSource string
	editorValue  string
	err          error
}

type memoryModel struct {
	rt         command.Runtime
	candidates []memoryCandidate
	index      int
	lastError  string
	cwd        string
	home       string
	opening    bool
}

func loadMemoryModel(_ context.Context, rt command.Runtime, _ []string) (tea.Model, error) {
	cwd, home := resolvePaths(rt)
	return memoryModel{
		rt:         rt,
		cwd:        cwd,
		home:       home,
		candidates: discoverMemoryCandidates(cwd, home),
	}, nil
}

func (m memoryModel) Init() tea.Cmd { return nil }

func (m memoryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyUp:
			if m.index > 0 {
				m.index--
			}
			return m, nil
		case tea.KeyDown:
			if m.index < len(m.candidates)-1 {
				m.index++
			}
			return m, nil
		case tea.KeyEnter:
			return m.openSelectedMemory()
		case tea.KeyEsc, tea.KeyCtrlC:
			m.notifyDismiss()
			return m, tea.Quit
		}
		switch strings.ToLower(strings.TrimSpace(msg.String())) {
		case "k":
			if m.index > 0 {
				m.index--
			}
		case "j":
			if m.index < len(m.candidates)-1 {
				m.index++
			}
		case "o":
			return m.openSelectedMemory()
		case "r":
			m.candidates = discoverMemoryCandidates(m.cwd, m.home)
			if m.index >= len(m.candidates) {
				m.index = len(m.candidates) - 1
				if m.index < 0 {
					m.index = 0
				}
			}
			m.lastError = ""
		case "q":
			m.notifyDismiss()
			return m, tea.Quit
		}
		return m, nil
	case memoryEditorFinishedMsg:
		m.opening = false
		if msg.err != nil {
			m.lastError = "Error opening memory file: " + msg.err.Error()
			return m, nil
		}
		editorHint := buildEditorHint(msg.editorSource, msg.editorValue)
		result := fmt.Sprintf("Opened memory file at %s\n\n%s", getRelativeMemoryPath(msg.path, m.cwd, m.home), editorHint)
		if m.rt.OnLocalJSXDone != nil {
			m.rt.OnLocalJSXDone(result, command.LocalJSXDoneOptions{Display: "system"})
		} else if m.rt.OnExit != nil {
			m.rt.OnExit()
		}
		return m, tea.Quit
	}
	return m, nil
}

func (m memoryModel) View() string {
	var b strings.Builder
	b.WriteString("Memory\n")
	b.WriteString("Select a memory file to edit in your terminal editor.\n\n")

	if len(m.candidates) == 0 {
		b.WriteString("No memory files found.\n")
	} else {
		for i, c := range m.candidates {
			cursor := "  "
			if i == m.index {
				cursor = "> "
			}
			label := c.Label
			if !c.Exists {
				label += " (new)"
			}
			b.WriteString(cursor + label + "\n")
			if strings.TrimSpace(c.Description) != "" {
				b.WriteString("    " + c.Description + "\n")
			}
		}
	}

	if m.opening {
		b.WriteString("\nOpening editor...\n")
	}
	if strings.TrimSpace(m.lastError) != "" {
		b.WriteString("\n" + m.lastError + "\n")
	}

	snapshot := renderMemorySnapshot(m.rt)
	if strings.TrimSpace(snapshot) != "" {
		b.WriteString("\nRecent memory snapshot:\n")
		for _, line := range strings.Split(snapshot, "\n") {
			b.WriteString("  " + line + "\n")
		}
	}

	b.WriteString("\nEnter open · j/k navigate · r refresh · Esc cancel")
	return strings.TrimRight(b.String(), "\n")
}

func (m memoryModel) openSelectedMemory() (tea.Model, tea.Cmd) {
	if len(m.candidates) == 0 {
		return m, nil
	}
	c := m.candidates[m.index]
	if err := ensureMemoryFile(c.Path); err != nil {
		m.lastError = "Error preparing memory file: " + err.Error()
		return m, nil
	}
	cmd, editorSource, editorValue, err := buildEditorCommand(c.Path)
	if err != nil {
		m.lastError = "Error opening editor: " + err.Error()
		return m, nil
	}
	m.opening = true
	m.lastError = ""
	return m, tea.ExecProcess(cmd, func(execErr error) tea.Msg {
		return memoryEditorFinishedMsg{
			path:         c.Path,
			editorSource: editorSource,
			editorValue:  editorValue,
			err:          execErr,
		}
	})
}

func (m memoryModel) notifyDismiss() {
	if m.rt.OnLocalJSXDone != nil {
		m.rt.OnLocalJSXDone("Cancelled memory editing", command.LocalJSXDoneOptions{
			Display: "system",
		})
		return
	}
	if m.rt.OnExit != nil {
		m.rt.OnExit()
	}
}

func resolvePaths(rt command.Runtime) (string, string) {
	cwd := ""
	if rt.State != nil {
		cwd = strings.TrimSpace(rt.State.Snapshot().CWD)
	}
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	home, _ := os.UserHomeDir()
	return cwd, home
}

func discoverMemoryCandidates(cwd, home string) []memoryCandidate {
	configHome := filepath.Join(home, ".claude")
	loader := memorypkg.CreateClaudeMdLoader(memorypkg.LoaderOptions{
		OriginalCwd:            cwd,
		HomeDir:                home,
		ConfigHomeDir:          configHome,
		UserSettingsEnabled:    true,
		ProjectSettingsEnabled: true,
		LocalSettingsEnabled:   true,
		AutoMemEnabled:         true,
	})

	existing := map[string]types.MemoryFileInfo{}
	if files, err := loader.GetMemoryFiles(context.Background()); err == nil {
		for _, file := range files {
			existing[normalizeCandidatePath(file.Path)] = file
		}
	}

	type seed struct {
		label string
		path  string
		desc  string
	}
	seeds := []seed{
		{
			label: "User memory",
			path:  filepath.Join(configHome, "CLAUDE.md"),
			desc:  "Saved in ~/.claude/CLAUDE.md",
		},
		{
			label: "Project memory",
			path:  filepath.Join(cwd, "CLAUDE.md"),
			desc:  "Saved in ./CLAUDE.md",
		},
		{
			label: "Local project memory",
			path:  filepath.Join(cwd, "CLAUDE.local.md"),
			desc:  "Private project instructions",
		},
		{
			label: "Project .claude memory",
			path:  filepath.Join(cwd, ".claude", "CLAUDE.md"),
			desc:  "Saved in ./.claude/CLAUDE.md",
		},
		{
			label: "Auto memory index",
			path:  filepath.Join(loader.GetAutoMemPath(), memorypkg.EntrypointName),
			desc:  "Auto-memory entrypoint",
		},
	}

	candidates := make([]memoryCandidate, 0, len(seeds)+len(existing))
	seen := map[string]bool{}
	for _, seed := range seeds {
		key := normalizeCandidatePath(seed.path)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		_, exists := existing[key]
		if !exists {
			_, statErr := os.Stat(seed.path)
			exists = statErr == nil
		}
		candidates = append(candidates, memoryCandidate{
			Label:       seed.label,
			Path:        seed.path,
			Description: seed.desc,
			Exists:      exists,
		})
	}

	existingExtras := make([]types.MemoryFileInfo, 0, len(existing))
	for key, file := range existing {
		if seen[key] {
			continue
		}
		existingExtras = append(existingExtras, file)
	}
	sort.Slice(existingExtras, func(i, j int) bool {
		return strings.ToLower(existingExtras[i].Path) < strings.ToLower(existingExtras[j].Path)
	})
	for _, file := range existingExtras {
		candidates = append(candidates, memoryCandidate{
			Label:       getRelativeMemoryPath(file.Path, cwd, home),
			Path:        file.Path,
			Description: "type=" + file.Type.String(),
			Exists:      true,
		})
	}

	return candidates
}

func ensureMemoryFile(path string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("empty memory path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	return f.Close()
}

func buildEditorCommand(path string) (*exec.Cmd, string, string, error) {
	editorSource := "default"
	editorValue := ""
	editor := strings.TrimSpace(os.Getenv("VISUAL"))
	if editor != "" {
		editorSource = "$VISUAL"
		editorValue = editor
	} else {
		editor = strings.TrimSpace(os.Getenv("EDITOR"))
		if editor != "" {
			editorSource = "$EDITOR"
			editorValue = editor
		}
	}
	if editor == "" {
		editor = "vi"
	}

	args := strings.Fields(editor)
	if len(args) == 0 {
		return nil, editorSource, editorValue, fmt.Errorf("invalid editor command")
	}
	cmd := exec.Command(args[0], append(args[1:], path)...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd, editorSource, editorValue, nil
}

func buildEditorHint(source, value string) string {
	if source != "default" && strings.TrimSpace(value) != "" {
		return fmt.Sprintf("Using %s=%q. To change editor, set $EDITOR or $VISUAL environment variable.", source, value)
	}
	return "To use a different editor, set the $EDITOR or $VISUAL environment variable."
}

func getRelativeMemoryPath(path, cwd, home string) string {
	path = filepath.Clean(path)
	if strings.TrimSpace(cwd) != "" {
		if rel, err := filepath.Rel(cwd, path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return "./" + filepath.ToSlash(rel)
		}
	}
	if strings.TrimSpace(home) != "" {
		if rel, err := filepath.Rel(home, path); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return path
}

func normalizeCandidatePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	return strings.ToLower(filepath.Clean(path))
}

func renderMemorySnapshot(runtime command.Runtime) string {
	if runtime.Engine == nil {
		return "engine is not configured"
	}
	messages := runtime.Engine.Messages()
	if len(messages) == 0 {
		return "no session memory"
	}

	lines := []string{
		fmt.Sprintf("session=%s", runtime.Engine.SessionID()),
		fmt.Sprintf("messages=%d", len(messages)),
		"recent_memory:",
	}
	start := len(messages) - 8
	if start < 0 {
		start = 0
	}
	for _, msg := range messages[start:] {
		lines = append(lines, fmt.Sprintf("- %s: %s", msg.Role, summarizeMessage(msg)))
	}
	return strings.Join(lines, "\n")
}
