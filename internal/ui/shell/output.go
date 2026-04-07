package shell

import (
	"fmt"
	"strings"
	"time"

	"claude-code-go/internal/ui/components"
)

// OutputConfig holds configuration for shell output rendering
type OutputConfig struct {
	Content       string
	ExitCode      int
	StartTime     time.Time
	EndTime       time.Time
	IsExpanded    bool
	IsActive      bool
	MaxLines      int // Max lines to show when collapsed
	Width         int
	ShowTimestamp bool
}

// Theme colors for shell output
var (
	colorMuted   = components.RGB{134, 145, 160}
	colorSuccess = components.RGB{78, 186, 101}
	colorError   = components.RGB{255, 107, 128}
	colorText    = components.RGB{255, 255, 255}
	colorClaude  = components.RGB{215, 119, 87}
)

// RenderShellOutput renders shell command output
// Matches src/components/shell/OutputLine.tsx and ShellProgressMessage.tsx
func RenderShellOutput(cfg OutputConfig) string {
	if cfg.Width <= 0 {
		cfg.Width = 80
	}
	if cfg.MaxLines <= 0 {
		cfg.MaxLines = 10
	}

	lines := strings.Split(cfg.Content, "\n")
	totalLines := len(lines)

	// Determine how many lines to show
	showLines := lines
	isTruncated := false
	if !cfg.IsExpanded && totalLines > cfg.MaxLines {
		showLines = lines[:cfg.MaxLines]
		isTruncated = true
	}

	var output []string

	// Render each line
	for _, line := range showLines {
		// Wrap long lines
		if len(line) > cfg.Width {
			wrapped := wrapLine(line, cfg.Width)
			for _, w := range wrapped {
				output = append(output, renderOutputLine(w, cfg.ExitCode != 0))
			}
		} else {
			output = append(output, renderOutputLine(line, cfg.ExitCode != 0))
		}
	}

	// Add truncation indicator
	if isTruncated {
		hiddenCount := totalLines - cfg.MaxLines
		truncMsg := fmt.Sprintf("  … %d more lines (Ctrl+O to expand)", hiddenCount)
		output = append(output, renderMuted(truncMsg))
	}

	// Add timing info if requested
	if cfg.ShowTimestamp && !cfg.EndTime.IsZero() {
		elapsed := cfg.EndTime.Sub(cfg.StartTime)
		timeStr := formatDuration(elapsed)
		exitStr := ""
		if cfg.ExitCode != 0 {
			exitStr = fmt.Sprintf(" (exit code %d)", cfg.ExitCode)
		}
		output = append(output, renderMuted(fmt.Sprintf("  [%s%s]", timeStr, exitStr)))
	}

	return strings.Join(output, "\n")
}

// renderOutputLine renders a single output line with appropriate styling
func renderOutputLine(line string, isError bool) string {
	prefix := "  "
	if isError {
		return prefix + renderColored(colorError, line)
	}
	return prefix + line
}

// ShellProgressState holds state for shell progress display
type ShellProgressState struct {
	Command   string
	StartTime time.Time
	IsActive  bool
	Output    strings.Builder
	ExitCode  int
}

// ShellProgressState creates a new shell progress state
func NewShellProgressState(command string) *ShellProgressState {
	return ShellProgressStateFor(command)
}

func ShellProgressStateFor(command string) *ShellProgressState {
	return &ShellProgressState{
		Command:   command,
		StartTime: time.Now(),
		IsActive:  true,
	}
}

// AppendOutput appends output to the shell progress
func (s *ShellProgressState) AppendOutput(text string) {
	s.Output.WriteString(text)
}

// Complete marks the shell progress as complete
func (s *ShellProgressState) Complete(exitCode int) {
	s.IsActive = false
	s.ExitCode = exitCode
}

// RenderProgress renders the shell progress
func (s *ShellProgressState) RenderProgress(width int) string {
	var lines []string

	// Command header
	indicator := components.BlackCircle
	if s.IsActive {
		lines = append(lines, renderColored(colorClaude, indicator)+" "+renderBold(s.Command))
	} else if s.ExitCode != 0 {
		lines = append(lines, renderColored(colorError, indicator)+" "+s.Command)
	} else {
		lines = append(lines, renderColored(colorSuccess, indicator)+" "+s.Command)
	}

	// Output
	output := s.Output.String()
	if output != "" {
		cfg := OutputConfig{
			Content:   output,
			ExitCode:  s.ExitCode,
			StartTime: s.StartTime,
			EndTime:   time.Now(),
			IsActive:  s.IsActive,
			Width:     width - 2,
			MaxLines:  10,
		}
		lines = append(lines, RenderShellOutput(cfg))
	}

	// Elapsed time for active commands
	if s.IsActive {
		elapsed := time.Since(s.StartTime)
		timeStr := formatDuration(elapsed)
		lines = append(lines, renderMuted(fmt.Sprintf("  Running... %s", timeStr)))
	}

	return strings.Join(lines, "\n")
}

// RenderTimeDisplay renders an elapsed time display
// Matches src/components/shell/ShellTimeDisplay.tsx
func RenderTimeDisplay(elapsed time.Duration, isActive bool) string {
	timeStr := formatDuration(elapsed)
	if isActive {
		return renderMuted(timeStr)
	}
	return timeStr
}

// formatDuration formats a duration for display
// Matches formatDuration in src/utils/format.ts
func formatDuration(d time.Duration) string {
	ms := d.Milliseconds()
	if ms == 0 {
		return "0s"
	}
	if ms < 60000 {
		seconds := ms / 1000
		if seconds == 0 {
			return "0s"
		}
		return fmt.Sprintf("%ds", seconds)
	}
	// >= 60 seconds
	minutes := ms / 60000
	seconds := (ms % 60000) / 1000
	if seconds == 60 {
		seconds = 0
		minutes++
	}
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// wrapLine wraps a long line to fit within width
func wrapLine(line string, width int) []string {
	if width <= 0 || len(line) <= width {
		return []string{line}
	}

	var result []string
	for len(line) > width {
		result = append(result, line[:width])
		line = line[width:]
	}
	if len(line) > 0 {
		result = append(result, line)
	}
	return result
}

// Helper functions

func renderColored(color components.RGB, text string) string {
	return fmt.Sprintf("\033[38;2;%d;%d;%dm%s\033[0m", color.R, color.G, color.B, text)
}

func renderBold(text string) string {
	return "\033[1m" + text + "\033[0m"
}

func renderMuted(text string) string {
	return renderColored(colorMuted, text)
}
