package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"claude-go/internal/app"
	"claude-go/internal/provider"
	"claude-go/internal/query"
	"claude-go/internal/session"

	tea "github.com/charmbracelet/bubbletea"
)

// Version information (set at build time)
var Version = "dev"

// CLIConfig holds CLI configuration
type CLIConfig struct {
	APIKey       string
	BaseURL      string
	Model        string
	Provider     string
	MaxTurns     int
	SessionID    string
	ResumeID     string
	ResumePicker bool   // true when -r/--resume used without value (show picker)
	Continue     bool
	PrintMode    bool
	OutputJSON   bool
	SystemPrompt string
	Prompt       string
	ExtraArgs    []string
	Version      bool
	Help         bool

	apiKeyExplicit       bool
	baseURLExplicit      bool
	modelExplicit        bool
	maxTurnsExplicit     bool
	systemPromptExplicit bool
	providerExplicit     bool
}

// RunCLI is the main CLI entry point
// Ported from src/entrypoints/cli.tsx and src/main.tsx
func RunCLI(ctx context.Context, stdout, stderr io.Writer, args []string) error {
	// Parse command line arguments
	cfg, err := parseArgs(args)
	if err != nil {
		return err
	}

	// Handle version flag
	if cfg.Version {
		fmt.Fprintf(stdout, "%s (Claude Code Go)\n", Version)
		return nil
	}

	// Handle help flag
	if cfg.Help {
		printHelp(stdout)
		return nil
	}

	// Handle print mode (non-interactive)
	if cfg.PrintMode {
		// Create LLM provider
		p, err := createProvider(cfg)
		if err != nil {
			return fmt.Errorf("create provider: %w", err)
		}

		// Create session manager
		sessionDir := getSessionDir()
		mgr, err := session.CreateEnhancedManager(sessionDir)
		if err != nil {
			return fmt.Errorf("create session storage: %w", err)
		}

		// Create query loop
		loop := query.NewQueryLoop(query.QueryConfig{
			MaxTurns:  cfg.MaxTurns,
			SessionID: cfg.SessionID,
		})
		loop.SetProvider(p)

		// Add system prompt if provided
		if cfg.SystemPrompt != "" {
			loop.AddSystemMessage(cfg.SystemPrompt)
		}

		// Resume session if specified
		if cfg.ResumeID != "" {
			sess, err := mgr.LoadSession(cfg.ResumeID)
			if err != nil {
				return fmt.Errorf("resume session: %w", err)
			}
			for _, msg := range sess.Messages {
				loop.AddUserMessage(msg.Content)
			}
		}

		return runPrintMode(ctx, loop, cfg, stdout, stderr)
	}

	// Handle resume picker mode (-r or --resume without value)
	if cfg.ResumePicker {
		return runResumePicker(ctx, cfg, stdout, stderr)
	}

	// Interactive mode now uses the Bubble Tea chat runner.
	return runInteractiveChat(ctx, cfg, stdout, stderr)
}

// parseArgs parses command line arguments
// Ported from src/main.tsx commander setup
func parseArgs(args []string) (*CLIConfig, error) {
	cfg := &CLIConfig{
		MaxTurns: 100,
		Provider: "openai",
		Model:    "gpt-4",
	}

	i := 0
	for i < len(args) {
		arg := args[i]

		switch {
		case arg == "-v" || arg == "--version" || arg == "-V":
			cfg.Version = true
			return cfg, nil

		case arg == "-h" || arg == "--help":
			cfg.Help = true
			return cfg, nil

		case arg == "-p" || arg == "--print":
			cfg.PrintMode = true

		case arg == "--json":
			cfg.OutputJSON = true

		case arg == "-c" || arg == "--continue":
			cfg.Continue = true

		case arg == "--api-key" && i+1 < len(args):
			cfg.APIKey = args[i+1]
			cfg.apiKeyExplicit = true
			i++

		case strings.HasPrefix(arg, "--api-key="):
			cfg.APIKey = strings.TrimPrefix(arg, "--api-key=")
			cfg.apiKeyExplicit = true

		case arg == "--base-url" && i+1 < len(args):
			cfg.BaseURL = args[i+1]
			cfg.baseURLExplicit = true
			i++

		case strings.HasPrefix(arg, "--base-url="):
			cfg.BaseURL = strings.TrimPrefix(arg, "--base-url=")
			cfg.baseURLExplicit = true

		case arg == "--model" && i+1 < len(args):
			cfg.Model = args[i+1]
			cfg.modelExplicit = true
			i++

		case strings.HasPrefix(arg, "--model="):
			cfg.Model = strings.TrimPrefix(arg, "--model=")
			cfg.modelExplicit = true

		case arg == "--provider" && i+1 < len(args):
			cfg.Provider = args[i+1]
			cfg.providerExplicit = true
			i++

		case strings.HasPrefix(arg, "--provider="):
			cfg.Provider = strings.TrimPrefix(arg, "--provider=")
			cfg.providerExplicit = true

		case arg == "--max-turns" && i+1 < len(args):
			fmt.Sscanf(args[i+1], "%d", &cfg.MaxTurns)
			cfg.maxTurnsExplicit = true
			i++

		case strings.HasPrefix(arg, "--max-turns="):
			fmt.Sscanf(strings.TrimPrefix(arg, "--max-turns="), "%d", &cfg.MaxTurns)
			cfg.maxTurnsExplicit = true

		case arg == "-r" || arg == "--resume":
			// Check if there's a value after -r/--resume
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				cfg.ResumeID = args[i+1]
				i++
			} else {
				// No value provided - show interactive picker
				cfg.ResumePicker = true
			}

		case strings.HasPrefix(arg, "--resume="):
			cfg.ResumeID = strings.TrimPrefix(arg, "--resume=")
			if cfg.ResumeID == "" {
				cfg.ResumePicker = true
			}

		case strings.HasPrefix(arg, "-r="):
			// Handle -r=<value> format
			cfg.ResumeID = strings.TrimPrefix(arg, "-r=")
			if cfg.ResumeID == "" {
				cfg.ResumePicker = true
			}

		case arg == "--system" && i+1 < len(args):
			cfg.SystemPrompt = args[i+1]
			cfg.systemPromptExplicit = true
			i++

		case strings.HasPrefix(arg, "--system="):
			cfg.SystemPrompt = strings.TrimPrefix(arg, "--system=")
			cfg.systemPromptExplicit = true

		case arg == "chat":
			// Default command, no-op

		case arg == "version":
			cfg.Version = true
			return cfg, nil

		case arg == "help":
			cfg.Help = true
			return cfg, nil

		case !strings.HasPrefix(arg, "-"):
			// Positional argument (prompt)
			cfg.Prompt = arg

		default:
			// Unknown flag, collect for later
			cfg.ExtraArgs = append(cfg.ExtraArgs, arg)
		}
		i++
	}

	// Load API key from environment if not provided
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("CLAUDE_API_KEY")
		if cfg.APIKey == "" {
			cfg.APIKey = os.Getenv("OPENAI_API_KEY")
		}
	}

	return cfg, nil
}

// createProvider creates an LLM provider based on config
func createProvider(cfg *CLIConfig) (provider.Provider, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("API key required (set --api-key or OPENAI_API_KEY env)")
	}

	providerType := provider.ProviderType(cfg.Provider)
	return provider.CreateProvider(providerType, cfg.APIKey, cfg.Model, cfg.BaseURL), nil
}

// runInteractiveChat runs Bubble Tea chat mode (TS-aligned primary path).
// If cfg.ResumeID is set, loads previous session content before starting.
func runInteractiveChat(ctx context.Context, cfg *CLIConfig, stdout, stderr io.Writer) error {
	restore, err := applyInteractiveCompatEnv(cfg)
	if err != nil {
		return err
	}
	defer restore()

	// If continuing last session, find the most recent session for this project
	if cfg.Continue && cfg.ResumeID == "" {
		sessionDir := getSessionDir()
		mgr, err := session.CreateEnhancedManager(sessionDir)
		if err == nil {
			sessions, err := mgr.ListSessions()
			if err == nil && len(sessions) > 0 {
				cfg.ResumeID = sessions[0].SessionID
			}
		}
	}

	// Pass resume session ID to engine so it loads the existing session
	// instead of creating a new one
	application, err := app.Create(ctx, cfg.ResumeID)
	if err != nil {
		return fmt.Errorf("create app: %w", err)
	}

	return RunChat(ctx, application, stdout, stderr)
}

// runResumePicker shows an interactive session picker for resume
// Matches TS behavior: -r/--resume without value opens interactive picker
// Uses Bubble Tea for interactive UI matching TS LogSelector/TreeSelect pattern
func runResumePicker(ctx context.Context, cfg *CLIConfig, stdout, stderr io.Writer) error {
	// Get all sessions from all projects
	sessions, err := listAllProjectSessions()
	if err != nil {
		return fmt.Errorf("list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Fprintln(stdout, "\n  No conversations found to resume")
		fmt.Fprintln(stdout, "\n  Sessions are stored in ~/.claude-go/projects/")
		fmt.Fprintln(stdout, "\n  Press Enter to start a new session...")
		var input string
		fmt.Fscanln(stdinReader(), &input)
		return runInteractiveChat(ctx, cfg, stdout, stderr)
	}

	// Convert to picker items
	pickerItems := make([]SessionPickItem, len(sessions))
	for i, s := range sessions {
		title := s.CustomTitle
		if title == "" {
			title = s.FirstPrompt
		}
		if title == "" {
			title = "(no prompt)"
		}
		projectName := s.ProjectName
		if projectName == "" {
			projectName = "unknown"
		}
		pickerItems[i] = SessionPickItem{
			SessionID:    s.SessionID,
			Title:        title,
			Date:         s.Modified.Format("2006-01-02 15:04"),
			ProjectName:  projectName,
			MessageCount: s.MessageCount,
		}
	}

	// Create and run the Bubble Tea picker
	picker := NewSessionPickerModel(pickerItems, 80, 24)
	p := tea.NewProgram(picker, tea.WithAltScreen())

	resultModel, err := p.Run()
	if err != nil {
		// Fallback to simple text input if Bubble Tea fails
		fmt.Fprintln(stderr, "Interactive picker unavailable, using simple mode")
		return runResumePickerFallback(ctx, cfg, stdout, stderr, sessions)
	}

	result := resultModel.(SessionPickerModel).GetResult()

	if result.Cancelled {
		// Start new session
		return runInteractiveChat(ctx, cfg, stdout, stderr)
	}

	// Resume selected session
	cfg.ResumeID = result.SessionID
	fmt.Fprintf(stdout, "\nResuming session: %s\n\n", result.SessionID[:8])

	return runInteractiveChat(ctx, cfg, stdout, stderr)
}

// runResumePickerFallback is a fallback for when Bubble Tea picker fails
func runResumePickerFallback(ctx context.Context, cfg *CLIConfig, stdout, stderr io.Writer, sessions []session.LogOption) error {
	fmt.Fprintln(stdout, "\nRecent sessions:")
	fmt.Fprintln(stdout, "  [0] Start new session")
	for i, s := range sessions {
		if i >= 20 {
			break
		}
		title := s.CustomTitle
		if title == "" {
			title = s.FirstPrompt
		}
		if title == "" {
			title = "(no prompt)"
		}
		if len(title) > 50 {
			title = title[:50] + "..."
		}
		projectName := s.ProjectName
		if len(projectName) > 20 {
			projectName = projectName[:20] + "..."
		}
		date := s.Modified.Format("2006-01-02 15:04")
		fmt.Fprintf(stdout, "  [%d] %s  %s  [%s] (%s)\n", i+1, s.SessionID[:8], date, title, projectName)
	}

	fmt.Fprintln(stdout, "\nEnter number to resume (or press Enter for new session):")

	var input string
	fmt.Fscanln(stdinReader(), &input)

	if input == "" || input == "0" {
		return runInteractiveChat(ctx, cfg, stdout, stderr)
	}

	idx, err := strconv.Atoi(input)
	if err != nil || idx < 1 || idx > len(sessions) {
		fmt.Fprintln(stderr, "Invalid selection")
		return nil
	}

	selected := sessions[idx-1]
	cfg.ResumeID = selected.SessionID
	fmt.Fprintf(stdout, "Resuming session: %s\n", selected.SessionID[:8])

	return runInteractiveChat(ctx, cfg, stdout, stderr)
}

// listAllProjectSessions lists all sessions from all projects in ~/.claude-go/projects/
// Uses Go CLI independent storage, not TS CLI history
func listAllProjectSessions() ([]session.LogOption, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home dir: %w", err)
	}

	// Use Go CLI specific projects directory
	projectsDir := filepath.Join(home, ".claude-go", "projects")

	// Check if projects directory exists
	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil // No sessions yet
	}

	// List all project directories
	projectDirs, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, fmt.Errorf("read projects dir: %w", err)
	}

	var allSessions []session.LogOption

	for _, projectDir := range projectDirs {
		if !projectDir.IsDir() {
			continue
		}

		projectPath := filepath.Join(projectsDir, projectDir.Name())
		mgr, err := session.CreateEnhancedManager(projectPath)
		if err != nil {
			continue // Skip problematic directories
		}

		sessions, err := mgr.ListSessions()
		if err != nil {
			continue
		}

		// Add project name to each session
		for _, s := range sessions {
			s.ProjectName = projectDir.Name()
			allSessions = append(allSessions, s)
		}
	}

	// Sort by modified time (most recent first)
	sortSessionsByModified(allSessions)

	return allSessions, nil
}

// sortSessionsByModified sorts sessions by modified time, most recent first
func sortSessionsByModified(sessions []session.LogOption) {
	// Simple sort by Modified time descending
	for i := 0; i < len(sessions); i++ {
		for j := i + 1; j < len(sessions); j++ {
			if sessions[j].Modified.After(sessions[i].Modified) {
				sessions[i], sessions[j] = sessions[j], sessions[i]
			}
		}
	}
}

// stdinReader returns os.Stdin or a fallback for testing
func stdinReader() io.Reader {
	if fi, err := os.Stdin.Stat(); err == nil && (fi.Mode()&os.ModeCharDevice) == 0 {
		return os.Stdin
	}
	return os.Stdin
}

func applyInteractiveCompatEnv(cfg *CLIConfig) (func(), error) {
	type snapshot struct {
		key     string
		value   string
		present bool
	}

	seen := map[string]bool{}
	snapshots := make([]snapshot, 0, 8)
	setEnv := func(key, value string) error {
		if !seen[key] {
			v, ok := os.LookupEnv(key)
			snapshots = append(snapshots, snapshot{key: key, value: v, present: ok})
			seen[key] = true
		}
		return os.Setenv(key, value)
	}

	claudeAPIKey := strings.TrimSpace(os.Getenv("CLAUDE_CODE_API_KEY"))
	apiKey := strings.TrimSpace(cfg.APIKey)
	switch {
	case cfg.apiKeyExplicit && apiKey != "":
		if err := setEnv("CLAUDE_CODE_API_KEY", apiKey); err != nil {
			return nil, err
		}
	case claudeAPIKey == "" && apiKey != "":
		if err := setEnv("CLAUDE_CODE_API_KEY", apiKey); err != nil {
			return nil, err
		}
	}

	baseURL := strings.TrimSpace(cfg.BaseURL)
	if baseURL != "" {
		if err := setEnv("CLAUDE_CODE_BASE_URL", baseURL); err != nil {
			return nil, err
		}
	}

	if cfg.modelExplicit && strings.TrimSpace(cfg.Model) != "" {
		if err := setEnv("CLAUDE_CODE_MODEL", strings.TrimSpace(cfg.Model)); err != nil {
			return nil, err
		}
	}
	if cfg.systemPromptExplicit {
		if err := setEnv("CLAUDE_CODE_SYSTEM_PROMPT", strings.TrimSpace(cfg.SystemPrompt)); err != nil {
			return nil, err
		}
	}
	if cfg.maxTurnsExplicit && cfg.MaxTurns > 0 {
		if err := setEnv("CLAUDE_CODE_MAX_TURNS", strconv.Itoa(cfg.MaxTurns)); err != nil {
			return nil, err
		}
	}

	restore := func() {
		for i := len(snapshots) - 1; i >= 0; i-- {
			item := snapshots[i]
			if item.present {
				_ = os.Setenv(item.key, item.value)
				continue
			}
			_ = os.Unsetenv(item.key)
		}
	}
	return restore, nil
}

// runPrintMode runs in non-interactive print mode
// Ported from src/main.tsx print mode logic
func runPrintMode(ctx context.Context, loop *query.QueryLoop, cfg *CLIConfig, stdout, stderr io.Writer) error {
	if cfg.Prompt == "" {
		return fmt.Errorf("prompt required in print mode")
	}

	result, err := loop.Query(ctx, cfg.Prompt)
	if err != nil {
		return err
	}

	if cfg.OutputJSON {
		// JSON output
		fmt.Fprintf(stdout, `{"response": "%s", "stop_reason": "%s", "turn": %d}`+"\n",
			escapeJSON(result.Response), result.StopReason, result.Turn)
	} else {
		// Plain text output
		fmt.Fprintln(stdout, result.Response)
	}

	return nil
}

// printHelp prints CLI help
func printHelp(w io.Writer) {
	fmt.Fprintf(w, `Claude Code Go - AI-powered coding assistant

Usage:
  claude [flags] [prompt]
  claude [command]

Commands:
  chat      Start an interactive chat session (default)
  version   Show version information
  help      Show this help message

Flags:
  -p, --print           Non-interactive print mode
  --json                Output in JSON format (with -p)
  -c, --continue        Continue last session
  -r, --resume [id]     Resume a session (interactive picker if no id)
  --api-key <key>       API key (or set OPENAI_API_KEY env)
  --base-url <url>      API base URL (reads from .env if not set)
  --model <name>        Model name (default: gpt-4)
  --max-turns <n>       Maximum turns per session
  --system <prompt>     System prompt

Examples:
  claude                          Start interactive session
  claude -p "explain this code"   Print mode
  claude --model gpt-4o           Use GPT-4o
  claude -r                       Show session picker
  claude -r sess_abc123           Resume specific session
  claude --resume sess_abc123     Resume session

Environment Variables:
  OPENAI_API_KEY      Default API key
  CLAUDE_API_KEY      Alternative API key

`)
}

// getSessionDir returns the session directory for the current project
// Uses independent Go CLI storage: ~/.claude-go/projects/<sanitized-cwd-path>
// This avoids reading/writing TS CLI history
func getSessionDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".claude-go"
	}

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	// Sanitize the cwd path to create a safe directory name
	sanitized := sanitizePathForSession(cwd)

	// Return the Go CLI specific directory
	return filepath.Join(home, ".claude-go", "projects", sanitized)
}

// sanitizePathForSession converts a path to a safe directory name
// Matches TS sanitizePath function: replaces non-alphanumeric chars with '-'
func sanitizePathForSession(path string) string {
	var result []rune
	for _, r := range path {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			result = append(result, r)
		} else {
			result = append(result, '-')
		}
	}
	s := string(result)
	// Limit length to prevent very long directory names
	if len(s) > 64 {
		// Keep first 64 chars
		s = s[:64]
	}
	return s
}

// escapeJSON escapes a string for JSON output
func escapeJSON(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, `"`, "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}
