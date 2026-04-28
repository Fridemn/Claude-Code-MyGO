package session

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"claude-go/internal/command"
	"claude-go/internal/task"
	"claude-go/internal/types"

	sessmgr "claude-go/internal/session"
)

// Runtime accessor functions
type sessionManagerGetter func() *sessmgr.Manager
type transcriptManagerGetter func() *sessmgr.EnhancedManager

var (
	getSessionManager    sessionManagerGetter
	getTranscriptManager transcriptManagerGetter
)

// RegisterSessionManagers sets the session manager accessors
func RegisterSessionManagers(sm *sessmgr.Manager, tm *sessmgr.EnhancedManager) {
	getSessionManager = func() *sessmgr.Manager { return sm }
	getTranscriptManager = func() *sessmgr.EnhancedManager { return tm }
}

func registerSessionCommands(r *command.Registry) {
	// /history - Dump current session history
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "history",
		Description: "dump current session history",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			return formatTaskTranscript(&task.AgentTask{Messages: runtime.Engine.Messages()}), nil
		},
	})

	// /compact - Compact conversation with LLM summarization
	r.Register(command.LocalCommand{
		CommandBase: command.CommandBase{
			Name:                   "compact",
			Description:            "Clear conversation history but keep a summary in context",
			ArgumentHint:           "<optional custom summarization instructions>",
			Source:                 "builtin",
		},
		SupportsNonInteractive: true,
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (command.CommandResult, error) {
			if runtime.CompactSession == nil {
				return command.CommandResult{}, fmt.Errorf("session compaction is not configured")
			}
			before, after := runtime.CompactSession(12)
			lines := []string{
				"session compacted",
				fmt.Sprintf("messages_before=%d", before),
				fmt.Sprintf("messages_after=%d", after),
				fmt.Sprintf("messages_removed=%d", before-after),
			}
			return command.CommandResult{
				Type:  command.ResultTypeCompact,
				Value: strings.Join(lines, "\n"),
			}, nil
		},
	})

	// /prompt - Show current system prompt
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "prompt",
		Description: "show current system prompt",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			msgs := runtime.Engine.Messages()
			if len(msgs) == 0 || msgs[0].Role != types.RoleSystem {
				return "", nil
			}
			return strings.TrimSpace(msgs[0].Content), nil
		},
	})

	// /clear - Clear conversation history
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "clear",
		Description: "Clear conversation history and free up context",
		Aliases:     []string{"reset", "new"},
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.Engine != nil {
				runtime.Engine.ReplaceMessages(nil)
			}
			if runtime.OnClear != nil {
				runtime.OnClear()
			}
			return "", nil
		},
	})

	// /rewind - Rewind to a previous message
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "rewind",
		Description:  "Rewind conversation to a previous message",
		Aliases:      []string{"checkpoint"},
		ArgumentHint: "[message index]",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			messages := runtime.Engine.Messages()
			if len(messages) <= 1 {
				return "No messages to rewind to", nil
			}

			// If no index provided, show available messages
			if len(args) == 0 || args[0] == "" {
				var lines []string
				lines = append(lines, "Available messages to rewind to:")
				lines = append(lines, "Usage: /rewind <index>")
				for i, msg := range messages {
					preview := msg.Content
					if len(preview) > 60 {
						preview = preview[:60] + "..."
					}
					lines = append(lines, fmt.Sprintf("[%d] %s: %s", i, msg.Role, preview))
				}
				return strings.Join(lines, "\n"), nil
			}

			// Parse the index
			var index int
			if _, err := fmt.Sscanf(args[0], "%d", &index); err != nil {
				return fmt.Sprintf("Invalid index: %s. Usage: /rewind <index>", args[0]), nil
			}

			if index < 0 || index >= len(messages) {
				return fmt.Sprintf("Index out of range. Available: 0-%d", len(messages)-1), nil
			}

			// Perform rewind if RewindMessages is available
			if runtime.RewindMessages != nil {
				if err := runtime.RewindMessages(index); err != nil {
					return "", fmt.Errorf("failed to rewind: %w", err)
				}
				return fmt.Sprintf("Rewound to message [%d]. Removed %d messages.", index, len(messages)-index), nil
			}

			// Fallback: just show the preview
			var lines []string
			for i, msg := range messages {
				if i > index {
					break
				}
				preview := msg.Content
				if len(preview) > 60 {
					preview = preview[:60] + "..."
				}
				lines = append(lines, fmt.Sprintf("[%d] %s: %s", i, msg.Role, preview))
			}
			return strings.Join(lines, "\n"), nil
		},
	})

	// /export - Export conversation to file
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocalJSX,
		Name:         "export",
		Description:  "Export conversation history to a file",
		ArgumentHint: "<filename>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			messages := runtime.Engine.Messages()
			content := formatMessagesToPlainText(messages)

			if len(args) == 0 || args[0] == "" {
				firstPrompt := extractFirstPrompt(messages)
				if firstPrompt == "" {
					firstPrompt = "conversation"
				}
				timestamp := time.Now().Format("2006-01-02-150405")
				filename := fmt.Sprintf("%s-%s.txt", sanitizeFilename(firstPrompt), timestamp)
				return fmt.Sprintf("export %s\n\n%s", filename, content), nil
			}

			filename := args[0]
			if !strings.HasSuffix(filename, ".txt") {
				filename = filename + ".txt"
			}
			cwd, _ := os.Getwd()
			path := filepath.Join(cwd, filename)
			if err := os.WriteFile(path, []byte(content), 0644); err != nil {
				return "", fmt.Errorf("failed to write file: %w", err)
			}
			return fmt.Sprintf("Conversation exported to: %s", path), nil
		},
	})

	// /resume - Resume a previous session
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocalJSX,
		Name:         "resume",
		Description:  "Resume a previous conversation session",
		ArgumentHint: "[conversation id or search term]",
		Aliases:      []string{"continue"},
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			tm := getTranscriptManager()
			if tm == nil {
				return "", fmt.Errorf("session manager not available")
			}

			sessions, err := tm.ListSessions()
			if err != nil {
				return "", fmt.Errorf("failed to list sessions: %w", err)
			}

			if len(sessions) == 0 {
				return "No previous sessions found", nil
			}

			currentID := runtime.Engine.SessionID()
			var filtered []sessmgr.LogOption
			for _, s := range sessions {
				if s.SessionID == currentID || s.IsSidechain {
					continue
				}
				filtered = append(filtered, s)
			}

			if len(filtered) == 0 {
				return "No previous sessions found", nil
			}

			sort.Slice(filtered, func(i, j int) bool {
				return filtered[i].Modified.After(filtered[j].Modified)
			})

			if len(args) > 0 && args[0] != "" {
				search := strings.ToLower(args[0])
				for _, s := range filtered {
					if strings.Contains(strings.ToLower(s.SessionID), search) ||
						strings.Contains(strings.ToLower(s.FirstPrompt), search) ||
						strings.Contains(strings.ToLower(s.CustomTitle), search) {
						return loadAndDisplaySession(&s)
					}
				}
				return fmt.Sprintf("Session not found: %s", args[0]), nil
			}

			var lines []string
			for i, s := range filtered {
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
				date := s.Modified.Format("2006-01-02 15:04")
				lines = append(lines, fmt.Sprintf("  %s  %s  [%s]", s.SessionID[:8], date, title))
			}
			return "Recent sessions (use /resume <session-id> to resume):\n" + strings.Join(lines, "\n"), nil
		},
	})

	// /rename - Rename current session
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "rename",
		Description:  "Rename the current session",
		ArgumentHint: "<new name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) == 0 || args[0] == "" {
				return "Usage: /rename <new-name>", nil
			}
			newName := strings.Join(args, " ")

			// Save the custom title to transcript
			if runtime.SaveSessionTitle != nil {
				sessionID := runtime.Engine.SessionID()
				if err := runtime.SaveSessionTitle(sessionID, newName); err != nil {
					return "", fmt.Errorf("failed to save session title: %w", err)
				}
			}

			return fmt.Sprintf("Session renamed to: %s", newName), nil
		},
	})

	// /exit - Quit the session
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "exit",
		Description: "quit the session",
		Hidden:      true,
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return "", nil
		},
	})
}

// formatMessagesToPlainText converts session messages to plain text for export
func formatMessagesToPlainText(messages []types.Message) string {
	if len(messages) == 0 {
		return "(empty conversation)"
	}
	var lines []string
	for _, msg := range messages {
		role := strings.ToUpper(string(msg.Role))
		content := msg.Content
		if len(content) > 5000 {
			content = content[:5000] + "\n... [truncated]"
		}
		lines = append(lines, fmt.Sprintf("[%s]\n%s\n", role, content))
	}
	return strings.Join(lines, "\n")
}

// sanitizeFilename converts text to a safe filename
func sanitizeFilename(text string) string {
	text = strings.ToLower(text)
	text = strings.ReplaceAll(text, " ", "-")
	var result []rune
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result = append(result, r)
		} else if r == '_' {
			result = append(result, '-')
		}
	}
	return strings.Trim(string(result), "-")
}

// extractFirstPrompt gets the first user message from session
func extractFirstPrompt(messages []types.Message) string {
	for _, msg := range messages {
		if msg.Role == types.RoleUser {
			lines := strings.SplitN(msg.Content, "\n", 2)
			if len(lines[0]) > 50 {
				return lines[0][:50]
			}
			return lines[0]
		}
	}
	return ""
}

func loadAndDisplaySession(s *sessmgr.LogOption) (string, error) {
	tm := getTranscriptManager()
	if tm == nil {
		return "", fmt.Errorf("session manager not available")
	}

	loaded, err := tm.LoadSession(s.SessionID)
	if err != nil {
		return "", fmt.Errorf("failed to load session: %w", err)
	}

	var lines []string
	lines = append(lines, fmt.Sprintf("Session: %s", s.SessionID))
	if s.CustomTitle != "" {
		lines = append(lines, fmt.Sprintf("Title: %s", s.CustomTitle))
	}
	lines = append(lines, fmt.Sprintf("Messages: %d", len(loaded.Messages)))
	lines = append(lines, fmt.Sprintf("Created: %s", loaded.CreatedAt.Format("2006-01-02 15:04")))

	if len(loaded.Messages) > 0 {
		lines = append(lines, "")
		lines = append(lines, "--- Conversation Preview ---")
		count := 5
		if len(loaded.Messages) < count {
			count = len(loaded.Messages)
		}
		for i := 0; i < count; i++ {
			msg := loaded.Messages[i]
			preview := msg.Content
			if len(preview) > 80 {
				preview = preview[:80] + "..."
			}
			lines = append(lines, fmt.Sprintf("[%d] %s: %s", i, msg.Role, preview))
		}
		if len(loaded.Messages) > count {
			lines = append(lines, fmt.Sprintf("... and %d more messages", len(loaded.Messages)-count))
		}
	}

	return strings.Join(lines, "\n"), nil
}

func formatTaskTranscript(taskState *task.AgentTask) string {
	if len(taskState.Messages) == 0 {
		return "no transcript recorded"
	}
	lines := make([]string, 0, len(taskState.Messages)*3)
	for i, msg := range taskState.Messages {
		lines = append(lines, fmt.Sprintf("[%02d] %s", i+1, strings.ToUpper(msg.Role)))
		lines = append(lines, msg.Content)
		if i < len(taskState.Messages)-1 {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

// SessionInfo holds session information for display
type SessionInfo struct {
	SessionID    string    `json:"sessionId"`
	CustomTitle  string    `json:"customTitle"`
	FirstPrompt  string    `json:"firstPrompt"`
	MessageCount int       `json:"messageCount"`
	Modified     time.Time `json:"modified"`
}

// MarshalJSON serializes session info
func (s *SessionInfo) MarshalJSON() ([]byte, error) {
	type Alias SessionInfo
	return json.Marshal(&struct {
		*Alias
		Modified string `json:"modified"`
	}{
		Alias:    (*Alias)(s),
		Modified: s.Modified.Format(time.RFC3339),
	})
}
