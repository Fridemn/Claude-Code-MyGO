package ide

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"claude-go/internal/command"

	tea "github.com/charmbracelet/bubbletea"
)

// TS src/commands/ide/ide.tsx
// Simplified for personal users - detect and connect to IDEs

// IdeType represents supported IDE types
// TS src/utils/ide.ts:IdeType
type IdeType string

const (
	IdeVSCode      IdeType = "vscode"
	IdeCursor      IdeType = "cursor"
	IdeWindsurf    IdeType = "windsurf"
	IdeIntelliJ    IdeType = "intellij"
	IdePyCharm     IdeType = "pycharm"
	IdeWebStorm    IdeType = "webstorm"
	IdeGoLand      IdeType = "goland"
	IdeCLion       IdeType = "clion"
	IdeAndroidStudio IdeType = "androidstudio"
)

// IdeConfig represents IDE configuration for detection
// TS src/utils/ide.ts:IdeConfig
type IdeConfig struct {
	IdeKind         string   // "vscode" or "jetbrains"
	DisplayName     string   // Display name
	ProcessKeywords []string // Process name keywords for detection
}

// Supported IDE configurations
// TS src/utils/ide.ts:supportedIdeConfigs
var supportedIdeConfigs = map[IdeType]IdeConfig{
	IdeVSCode: {
		IdeKind:     "vscode",
		DisplayName: "VS Code",
		ProcessKeywords: []string{"code", "Visual Studio Code", "Code Helper"},
	},
	IdeCursor: {
		IdeKind:     "vscode",
		DisplayName: "Cursor",
		ProcessKeywords: []string{"cursor", "Cursor Helper", "Cursor.app"},
	},
	IdeWindsurf: {
		IdeKind:     "vscode",
		DisplayName: "Windsurf",
		ProcessKeywords: []string{"windsurf", "Windsurf Helper", "Windsurf.app"},
	},
	IdeIntelliJ: {
		IdeKind:     "jetbrains",
		DisplayName: "IntelliJ IDEA",
		ProcessKeywords: []string{"idea", "IntelliJ IDEA", "idea64"},
	},
	IdePyCharm: {
		IdeKind:     "jetbrains",
		DisplayName: "PyCharm",
		ProcessKeywords: []string{"pycharm", "PyCharm", "pycharm64"},
	},
	IdeWebStorm: {
		IdeKind:     "jetbrains",
		DisplayName: "WebStorm",
		ProcessKeywords: []string{"webstorm", "WebStorm", "webstorm64"},
	},
	IdeGoLand: {
		IdeKind:     "jetbrains",
		DisplayName: "GoLand",
		ProcessKeywords: []string{"goland", "GoLand", "goland64"},
	},
	IdeCLion: {
		IdeKind:     "jetbrains",
		DisplayName: "CLion",
		ProcessKeywords: []string{"clion", "CLion", "clion64"},
	},
	IdeAndroidStudio: {
		IdeKind:     "jetbrains",
		DisplayName: "Android Studio",
		ProcessKeywords: []string{"android-studio", "Android Studio", "studio64"},
	},
}

// DetectedIDEInfo represents a detected IDE
// TS src/utils/ide.ts:DetectedIDEInfo
type DetectedIDEInfo struct {
	Name            string
	Port            int
	WorkspaceFolders []string
	URL             string
	IsValid         bool
	AuthToken       string
	IdeRunningInWindows bool
}

// Lockfile content structure
// TS src/utils/ide.ts:LockfileJsonContent
type LockfileContent struct {
	WorkspaceFolders []string `json:"workspaceFolders,omitempty"`
	Pid              int      `json:"pid,omitempty"`
	IdeName          string   `json:"ideName,omitempty"`
	Transport        string   `json:"transport,omitempty"` // "ws" or "sse"
	RunningInWindows bool     `json:"runningInWindows,omitempty"`
	AuthToken        string   `json:"authToken,omitempty"`
}

func Register(r *command.Registry) {
	registerIDE(r)
}

// toIDEDisplayName converts IDE type to display name
// TS src/utils/ide.ts:toIDEDisplayName
func toIDEDisplayName(ide IdeType) string {
	config, ok := supportedIdeConfigs[ide]
	if ok {
		return config.DisplayName
	}
	return string(ide)
}

// isVSCodeIde checks if IDE is VS Code-based
// TS src/utils/ide.ts:isVSCodeIde
func isVSCodeIde(ide IdeType) bool {
	config, ok := supportedIdeConfigs[ide]
	if !ok {
		return false
	}
	return config.IdeKind == "vscode"
}

// isJetBrainsIde checks if IDE is JetBrains-based
// TS src/utils/ide.ts:isJetBrainsIde
func isJetBrainsIde(ide IdeType) bool {
	config, ok := supportedIdeConfigs[ide]
	if !ok {
		return false
	}
	return config.IdeKind == "jetbrains"
}

// getIdeLockfilesPath returns the path to IDE lockfiles directory
// TS src/utils/ide.ts:getIdeLockfilesPaths
func getIdeLockfilesPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "ide")
}

// detectRunningIDEs detects running IDEs by checking processes
// TS src/utils/ide.ts:detectRunningIDEs
func detectRunningIDEs() []IdeType {
	runningIDEs := []IdeType{}

	// Get running processes based on platform
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("ps", "aux")
	case "linux":
		cmd = exec.Command("ps", "aux")
	case "windows":
		// Use tasklist on Windows
		cmd = exec.Command("tasklist")
	default:
		return runningIDEs
	}

	output, err := cmd.Output()
	if err != nil {
		return runningIDEs
	}

	outputLower := strings.ToLower(string(output))

	// Check each IDE's process keywords
	for ideType, config := range supportedIdeConfigs {
		for _, keyword := range config.ProcessKeywords {
			if strings.Contains(outputLower, strings.ToLower(keyword)) {
				runningIDEs = append(runningIDEs, ideType)
				break
			}
		}
	}

	return runningIDEs
}

// isProcessRunning checks if a process with given PID is running
// TS src/utils/ide.ts:isProcessRunning
func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, FindProcess always succeeds; need to send signal 0 to check
	err = process.Signal(os.Signal(nil))
	return err == nil
}

// readIdeLockfile reads an IDE lockfile and extracts info
// TS src/utils/ide.ts:readIdeLockfile
func readIdeLockfile(lockfilePath string) (*DetectedIDEInfo, error) {
	// Extract port from filename (e.g., 12345.lock -> 12345)
	filename := filepath.Base(lockfilePath)
	portStr := strings.TrimSuffix(filename, ".lock")

	port := 0
	for _, c := range portStr {
		if c >= '0' && c <= '9' {
			port = port*10 + int(c-'0')
		} else {
			break
		}
	}

	if port == 0 {
		return nil, fmt.Errorf("invalid port from lockfile: %s", filename)
	}

	// Read lockfile content
	content, err := os.ReadFile(lockfilePath)
	if err != nil {
		return nil, err
	}

	// Parse content (simple format: lines of workspace folders or JSON)
	lines := strings.Split(string(content), "\n")
	workspaceFolders := []string{}
	ideName := ""
	transport := "sse" // default
	runningInWindows := false
	authToken := ""

	// Try to parse as simple lines first
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Check if it looks like a path
		if strings.Contains(line, "/") || strings.Contains(line, "\\") {
			workspaceFolders = append(workspaceFolders, line)
		}
	}

	// If first line is JSON-like, try parsing key fields
	if len(lines) > 0 && strings.HasPrefix(lines[0], "{") {
		// Simple JSON field extraction (avoid full JSON parsing for simplicity)
		for _, line := range lines {
			if strings.Contains(line, "\"ideName\"") {
				// Extract ideName value
				start := strings.Index(line, ":")
				if start > 0 {
					value := strings.TrimSpace(line[start+1:])
					value = strings.Trim(value, "\" ,")
					ideName = value
				}
			}
			if strings.Contains(line, "\"transport\"") {
				start := strings.Index(line, ":")
				if start > 0 {
					value := strings.TrimSpace(line[start+1:])
					value = strings.Trim(value, "\" ,")
					if value == "ws" {
						transport = "ws"
					}
				}
			}
			if strings.Contains(line, "\"authToken\"") {
				start := strings.Index(line, ":")
				if start > 0 {
					value := strings.TrimSpace(line[start+1:])
					value = strings.Trim(value, "\" ,")
					authToken = value
				}
			}
			if strings.Contains(line, "\"runningInWindows\"") {
				if strings.Contains(line, "true") {
					runningInWindows = true
				}
			}
		}
	}

	// Determine URL based on transport
	host := "127.0.0.1"
	var url string
	if transport == "ws" {
		url = fmt.Sprintf("ws://%s:%d", host, port)
	} else {
		url = fmt.Sprintf("http://%s:%d/sse", host, port)
	}

	// Use first workspace folder or cwd for validation check
	cwd, _ := os.Getwd()
	isValid := len(workspaceFolders) > 0
	if isValid {
		// Check if cwd is within workspace
		for _, folder := range workspaceFolders {
			if strings.HasPrefix(cwd, folder) {
				isValid = true
				break
			}
		}
	}

	if ideName == "" {
		// Try to detect IDE name from running processes
		running := detectRunningIDEs()
		if len(running) > 0 {
			ideName = toIDEDisplayName(running[0])
		} else {
			ideName = "IDE"
		}
	}

	return &DetectedIDEInfo{
		Name:             ideName,
		Port:             port,
		WorkspaceFolders: workspaceFolders,
		URL:              url,
		IsValid:          isValid,
		AuthToken:        authToken,
		IdeRunningInWindows: runningInWindows,
	}, nil
}

// detectIDEs detects available IDEs by reading lockfiles
// TS src/utils/ide.ts:detectIDEs
func detectIDEs(includeInvalid bool) []DetectedIDEInfo {
	detected := []DetectedIDEInfo{}

	lockfilesPath := getIdeLockfilesPath()
	if lockfilesPath == "" {
		return detected
	}

	// Check if directory exists
	info, err := os.Stat(lockfilesPath)
	if err != nil || !info.IsDir() {
		return detected
	}

	// Read all lockfiles
	entries, err := os.ReadDir(lockfilesPath)
	if err != nil {
		return detected
	}

	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".lock") {
			continue
		}

		lockfilePath := filepath.Join(lockfilesPath, entry.Name())
		ideInfo, err := readIdeLockfile(lockfilePath)
		if err != nil {
			continue
		}

		if includeInvalid || ideInfo.IsValid {
			detected = append(detected, *ideInfo)
		}
	}

	return detected
}

// formatWorkspaceFolders formats workspace folders for display
// TS src/commands/ide/ide.tsx:formatWorkspaceFolders
func formatWorkspaceFolders(folders []string) string {
	if len(folders) == 0 {
		return ""
	}

	// Show first 2 folders
	maxFolders := 2
	if len(folders) < maxFolders {
		maxFolders = len(folders)
	}

	result := []string{}
	cwd, _ := os.Getwd()

	for i := 0; i < maxFolders; i++ {
		folder := folders[i]
		// Strip cwd prefix if present
		if strings.HasPrefix(folder, cwd) {
			rel := strings.TrimPrefix(folder, cwd)
			rel = strings.TrimPrefix(rel, "/")
			rel = strings.TrimPrefix(rel, "\\")
			if rel != "" {
				folder = rel
			}
		}
		// Truncate long paths
		if len(folder) > 50 {
			folder = "..." + folder[len(folder)-47:]
		}
		result = append(result, folder)
	}

	formatted := strings.Join(result, ", ")
	if len(folders) > maxFolders {
		formatted += ", ..."
	}

	return formatted
}

func registerIDE(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "ide",
		Description: "connect to an IDE for integrated development features",
		Load:        loadIDEModel,
		Handler: func(ctx context.Context, rt command.Runtime, args []string) (string, error) {
			return handleIDECommand(args, rt), nil
		},
	})
}

func handleIDECommand(args []string, rt command.Runtime) string {
	// Handle 'open' argument
	if len(args) > 0 && strings.TrimSpace(args[0]) == "open" {
		return handleIDEOpen(rt)
	}

	// Detect IDEs
	detected := detectIDEs(true)
	available := []DetectedIDEInfo{}
	unavailable := []DetectedIDEInfo{}

	for _, ide := range detected {
		if ide.IsValid {
			available = append(available, ide)
		} else {
			unavailable = append(unavailable, ide)
		}
	}

	// Build output
	var lines []string
	lines = append(lines, "IDE Detection Results")
	lines = append(lines, "")

	if len(available) == 0 {
		lines = append(lines, "No available IDEs detected.")
		lines = append(lines, "Make sure your IDE has the Claude Code extension installed and is running.")
		lines = append(lines, "")
		lines = append(lines, "Supported IDEs:")
		for ideType, config := range supportedIdeConfigs {
			lines = append(lines, fmt.Sprintf("  - %s (%s)", config.DisplayName, ideType))
		}
	} else {
		lines = append(lines, fmt.Sprintf("Available IDEs (%d):", len(available)))
		for _, ide := range available {
			workspaces := formatWorkspaceFolders(ide.WorkspaceFolders)
			if workspaces != "" {
				lines = append(lines, fmt.Sprintf("  - %s (port %d): %s", ide.Name, ide.Port, workspaces))
			} else {
				lines = append(lines, fmt.Sprintf("  - %s (port %d)", ide.Name, ide.Port))
			}
		}
	}

	if len(unavailable) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Unavailable IDEs (%d) - workspace mismatch:", len(unavailable)))
		for _, ide := range unavailable {
			workspaces := formatWorkspaceFolders(ide.WorkspaceFolders)
			lines = append(lines, fmt.Sprintf("  - %s (port %d): %s", ide.Name, ide.Port, workspaces))
		}
	}

	// Show running IDEs (detected by process)
	running := detectRunningIDEs()
	if len(running) > 0 {
		lines = append(lines, "")
		lines = append(lines, fmt.Sprintf("Running IDE processes (%d):", len(running)))
		for _, ide := range running {
			lines = append(lines, fmt.Sprintf("  - %s", toIDEDisplayName(ide)))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "Tip: Use /ide <port> to connect to a specific IDE")

	return strings.Join(lines, "\n")
}

func handleIDEOpen(rt command.Runtime) string {
	cwd, _ := os.Getwd()

	// Detect available IDEs
	detected := detectIDEs(true)
	available := []DetectedIDEInfo{}
	for _, ide := range detected {
		if ide.IsValid {
			available = append(available, ide)
		}
	}

	if len(available) == 0 {
		return "No IDEs with Claude Code extension detected."
	}

	// Find VS Code-based IDE for opening
	for _, ide := range available {
		if strings.Contains(strings.ToLower(ide.Name), "vscode") ||
		   strings.Contains(strings.ToLower(ide.Name), "cursor") ||
		   strings.Contains(strings.ToLower(ide.Name), "windsurf") {
			// Try to open with 'code' command
			cmd := exec.Command("code", cwd)
			err := cmd.Start()
			if err == nil {
				return fmt.Sprintf("Opened project in %s", ide.Name)
			}
		}
	}

	// Fallback: show manual instruction
	if len(available) > 0 {
		return fmt.Sprintf("Please open the project manually in %s: %s", available[0].Name, cwd)
	}

	return "No suitable IDE found for opening."
}

type ideModel struct {
	rt           command.Runtime
	available    []DetectedIDEInfo
	unavailable  []DetectedIDEInfo
	running      []IdeType
	selectedPort int
	message      string
}

func loadIDEModel(_ context.Context, rt command.Runtime, args []string) (tea.Model, error) {
	m := ideModel{rt: rt}

	// Handle 'open' argument
	if len(args) > 0 && strings.TrimSpace(args[0]) == "open" {
		m.message = handleIDEOpen(rt)
		return m, nil
	}

	// Detect IDEs
	detected := detectIDEs(true)
	for _, ide := range detected {
		if ide.IsValid {
			m.available = append(m.available, ide)
		} else {
			m.unavailable = append(m.unavailable, ide)
		}
	}
	m.running = detectRunningIDEs()

	// Build message
	m.message = buildIDEDisplay(m)

	return m, nil
}

func buildIDEDisplay(m ideModel) string {
	var lines []string
	lines = append(lines, "IDE Connection")
	lines = append(lines, "")

	if len(m.available) == 0 {
		lines = append(lines, "No available IDEs detected.")
		lines = append(lines, "")
		lines = append(lines, "Install Claude Code extension in your IDE:")
		lines = append(lines, "  VS Code/Cursor/Windsurf: Install 'anthropic.claude-code'")
		lines = append(lines, "  JetBrains: Install Claude Code plugin from marketplace")
	} else {
		lines = append(lines, "Available IDEs:")
		for _, ide := range m.available {
			workspaces := formatWorkspaceFolders(ide.WorkspaceFolders)
			if workspaces != "" {
				lines = append(lines, fmt.Sprintf("  [%d] %s: %s", ide.Port, ide.Name, workspaces))
			} else {
				lines = append(lines, fmt.Sprintf("  [%d] %s", ide.Port, ide.Name))
			}
		}
		lines = append(lines, "")
		lines = append(lines, "Select an IDE by port number or press Esc to cancel")
	}

	return strings.Join(lines, "\n")
}

func (m ideModel) Init() tea.Cmd { return nil }

func (m ideModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyEsc, tea.KeyCtrlC:
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		case tea.KeyEnter:
			if m.rt.OnExit != nil {
				m.rt.OnExit()
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m ideModel) View() string {
	return m.message + "\n\nPress Enter or Esc to close"
}