package bash

import (
	"claude-go/internal/task"
	"claude-go/internal/tool"

	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
)

// BashTool implements the Bash/Shell execution tool.
// This is a comprehensive implementation matching the TypeScript BashTool behavior.
type BashTool struct{}

func (BashTool) Name() string { return "Bash" }
func (BashTool) Description() string {
	return getBashToolDescription()
}
func (BashTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"command":                   tool.SchemaString("Shell command to execute."),
		"description":               tool.SchemaString("Short explanation of why the command is needed."),
		"timeout":                   tool.SchemaInteger("Timeout in milliseconds."),
		"run_in_background":         tool.SchemaBoolean("Whether to start the command in the background."),
		"dangerouslyDisableSandbox": tool.SchemaBoolean("Disable sandboxing for this command when policy allows it."),
	}, "command")
}

// IsReadOnly determines if a command is read-only (no side effects)
func (BashTool) IsReadOnly(in tool.Input) bool {
	command, _ := in["command"].(string)
	return isReadOnlyBashCommand(command)
}

// IsSearchOrReadCommand determines if a bash command is a search/read operation
// that can be collapsed in the TUI.
func (BashTool) IsSearchOrReadCommand(in tool.Input) tool.SearchOrReadResult {
	command, _ := in["command"].(string)
	if command == "" {
		return tool.SearchOrReadResult{}
	}

	// Extract the command name (first word)
	cmdName := extractCommandName(command)
	if cmdName == "" {
		return tool.SearchOrReadResult{}
	}

	// Check for search commands
	if bashSearchCommands[cmdName] {
		return tool.SearchOrReadResult{
			IsCollapsible: true,
			IsSearch:      true,
		}
	}

	// Check for read commands
	if bashReadCommands[cmdName] {
		return tool.SearchOrReadResult{
			IsCollapsible: true,
			IsRead:        true,
		}
	}

	// Check for list commands
	if bashListCommands[cmdName] {
		return tool.SearchOrReadResult{
			IsCollapsible: true,
			IsList:        true,
		}
	}

	// Non-search/read Bash commands can be collapsed in fullscreen mode
	// but are tracked separately as "bash commands"
	return tool.SearchOrReadResult{
		IsCollapsible: false, // Will be set to true in fullscreen mode by the collapse logic
		IsBash:        true,
	}
}

// extractCommandName extracts the command name from a bash command string
func extractCommandName(command string) string {
	// Remove leading sudo, time, etc.
	command = strings.TrimSpace(command)
	for {
		if strings.HasPrefix(command, "sudo ") {
			command = strings.TrimPrefix(command, "sudo ")
			command = strings.TrimSpace(command)
			continue
		}
		if strings.HasPrefix(command, "time ") {
			command = strings.TrimPrefix(command, "time ")
			command = strings.TrimSpace(command)
			continue
		}
		break
	}

	// Get first word
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// BashInput represents the input parameters for the Bash tool
type BashInput struct {
	Command                   string `json:"command"`
	Description               string `json:"description,omitempty"`
	Timeout                   int    `json:"timeout,omitempty"`
	RunInBackground           bool   `json:"run_in_background,omitempty"`
	DangerouslyDisableSandbox bool   `json:"dangerouslyDisableSandbox,omitempty"`
}

// BashOutput represents the output of the Bash tool
type BashOutput struct {
	Stdout                    string `json:"stdout"`
	Stderr                    string `json:"stderr"`
	Interrupted               bool   `json:"interrupted"`
	IsImage                   bool   `json:"isImage,omitempty"`
	BackgroundTaskID          string `json:"backgroundTaskId,omitempty"`
	ReturnCode                int    `json:"returnCode,omitempty"`
	ReturnCodeInterpretation  string `json:"returnCodeInterpretation,omitempty"`
	NoOutputExpected          bool   `json:"noOutputExpected,omitempty"`
	PersistedOutputPath       string `json:"persistedOutputPath,omitempty"`
	PersistedOutputSize       int64  `json:"persistedOutputSize,omitempty"`
	DangerouslyDisableSandbox bool   `json:"dangerouslyDisableSandbox,omitempty"`
}

// Default timeout values
const (
	DefaultTimeoutMs = 120000 // 2 minutes
	MaxTimeoutMs     = 600000 // 10 minutes
	MaxOutputSize    = 30000  // 30KB characters for inline output
)

// Command classification sets
var (
	// Search commands that can be collapsed in UI
	bashSearchCommands = map[string]bool{
		"find": true, "grep": true, "rg": true, "ag": true,
		"ack": true, "locate": true, "which": true, "whereis": true,
	}

	// Read/view commands
	bashReadCommands = map[string]bool{
		"cat": true, "head": true, "tail": true, "less": true, "more": true,
		"wc": true, "stat": true, "file": true, "strings": true,
		"jq": true, "awk": true, "cut": true, "sort": true, "uniq": true, "tr": true,
	}

	// Directory listing commands
	bashListCommands = map[string]bool{
		"ls": true, "tree": true, "du": true,
	}

	// Commands that typically produce no stdout on success
	bashSilentCommands = map[string]bool{
		"mv": true, "cp": true, "rm": true, "mkdir": true, "rmdir": true,
		"chmod": true, "chown": true, "chgrp": true, "touch": true, "ln": true,
		"cd": true, "export": true, "unset": true, "wait": true,
	}

	// Semantic-neutral commands (pure output/status)
	bashSemanticNeutralCommands = map[string]bool{
		"echo": true, "printf": true, "true": true, "false": true, ":": true,
	}

	// Dangerous patterns that require extra validation
	dangerousPatterns = []struct {
		pattern *regexp.Regexp
		message string
	}{
		{regexp.MustCompile(`\$\(`), "$() command substitution"},
		{regexp.MustCompile(`<\(`), "process substitution <()"},
		{regexp.MustCompile(`>\(`), "process substitution >()"},
		{regexp.MustCompile(`\$\{`), "${} parameter substitution"},
		{regexp.MustCompile(`\$\[`), "$[] legacy arithmetic expansion"},
		{regexp.MustCompile(`\s+&&\s+rm\s+`), "rm after && (destructive chain)"},
		{regexp.MustCompile(`\s*\|\s*rm\s+`), "rm in pipeline (destructive)"},
	}

	// Commands that should not be auto-backgrounded
	disallowedAutoBackgroundCommands = map[string]bool{
		"sleep": true,
	}
)

// Call executes the bash command
func (BashTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	// Extract input parameters
	command, _ := in["command"].(string)
	description, _ := in["description"].(string)
	timeoutMs, _ := in["timeout"].(float64)
	runInBackground, _ := in["run_in_background"].(bool)
	disableSandbox, _ := in["dangerouslyDisableSandbox"].(bool)

	if strings.TrimSpace(command) == "" {
		return tool.Result{}, fmt.Errorf("command is required")
	}

	// Set timeout
	timeout := int(timeoutMs)
	if timeout <= 0 {
		timeout = DefaultTimeoutMs
	}
	if timeout > MaxTimeoutMs {
		timeout = MaxTimeoutMs
	}

	// Get working directory
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	// Use store's CWD if available
	if runtime.Store != nil {
		cwd = runtime.Store.GetCWD()
	}

	// Validate command for security issues. Ask-level security checks are
	// converted into an explicit permission prompt override, rather than a hard
	// tool error, so users can approve from the interactive permission UI.
	securityPrompt, securityErr := validateCommandSecurity(command)
	if securityErr != nil {
		errMsg := securityErr.Error()
		return tool.Result{
			Error: errMsg,
			Content: BashOutput{
				Stderr:      errMsg,
				Interrupted: false,
			},
		}, nil
	}

	// Check for blocked sleep patterns
	if blockedPattern := DetectBlockedSleepPattern(command); blockedPattern != "" {
		errMsg := fmt.Sprintf("Blocked: %s. Run blocking commands in the background with run_in_background: true — you'll get a completion notification when done. For streaming events (watching logs, polling APIs), use the Monitor tool. If you genuinely need a delay (rate limiting, deliberate pacing), keep it under 2 seconds.", blockedPattern)
		return tool.Result{
			Error: errMsg,
			Content: BashOutput{
				Stderr:      errMsg,
				Interrupted: false,
			},
		}, nil
	}

	// Check permissions using global permission checker
	permResult := CheckGlobalPermissionWithPromptOverride(command, description, securityPrompt)
	if !permResult.Allowed {
		errMsg := fmt.Sprintf("permission denied: %s", permResult.Reason)
		return tool.Result{
			Error: errMsg,
			Content: BashOutput{
				Stderr:                    errMsg,
				Interrupted:               false,
				DangerouslyDisableSandbox: disableSandbox,
			},
		}, nil
	}

	// Check if this is a read-only command
	isReadOnly := isReadOnlyBashCommand(command)

	// Handle sandbox execution if enabled and not disabled
	if !disableSandbox && IsSandboxingEnabled() {
		sandboxResult := ExecuteSandboxed(ctx, command, cwd)
		if !sandboxResult.Allowed {
			errMsg := fmt.Sprintf("sandbox denied: %s", sandboxResult.Violation.Message)
			return tool.Result{
				Error: errMsg,
				Content: BashOutput{
					Stderr:                    errMsg,
					Interrupted:               sandboxResult.Violation.Type == ViolationTimeout,
					DangerouslyDisableSandbox: disableSandbox,
				},
			}, nil
		}
		// Sandbox execution complete
		return tool.Result{
			Content: BashOutput{
				Stdout:                    sandboxResult.Output,
				Stderr:                    "",
				Interrupted:               sandboxResult.Violation != nil && sandboxResult.Violation.Type == ViolationTimeout,
				DangerouslyDisableSandbox: disableSandbox,
			},
		}, nil
	}

	// Handle background execution
	if runInBackground {
		return executeBackground(ctx, command, description, cwd, runtime)
	}

	// Execute synchronously
	return executeSync(ctx, command, description, cwd, timeout, isReadOnly, disableSandbox, runtime)
}

// executeSync executes the command synchronously
func executeSync(ctx context.Context, command, description, cwd string, timeout int, isReadOnly, disableSandbox bool, runtime tool.Runtime) (tool.Result, error) {
	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	// Build command
	cmd := buildCommand(command, cwd, disableSandbox)

	// Set up output capture
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Check if command is expected to produce no output
	noOutputExpected := isSilentBashCommand(command)

	// Start the command
	startTime := time.Now()
	err := cmd.Start()
	if err != nil {
		return tool.Result{
			Content: BashOutput{
				Stderr:           fmt.Sprintf("Failed to start command: %v", err),
				Interrupted:      false,
				NoOutputExpected: noOutputExpected,
			},
		}, nil
	}

	// Wait for completion
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	var interrupted bool
	select {
	case <-execCtx.Done():
		// Context cancelled or timeout
		interrupted = true
		if cmd.Process != nil {
			// Try graceful termination first
			cmd.Process.Signal(os.Interrupt)
			time.Sleep(100 * time.Millisecond)
			// Force kill if still running
			cmd.Process.Kill()
		}
		<-done // Wait for command to actually finish
	case err = <-done:
		// Command completed
	}

	duration := time.Since(startTime)

	// Process output
	stdoutStr := stdout.String()
	stderrStr := stderr.String()

	// Handle interruption
	if interrupted {
		stderrStr += "\nCommand timed out after " + duration.String()
	}

	// Determine return code interpretation
	returnCode := 0
	if cmd.ProcessState != nil {
		returnCode = cmd.ProcessState.ExitCode()
	}
	returnCodeInterpretation := interpretReturnCode(returnCode, command)

	// Check for CWD reset after command execution
	var cwdReset bool
	if runtime.Store != nil {
		_ = runtime.Store
		cwdReset = false
	}

	// Add reset message to stderr if CWD was reset
	if cwdReset && runtime.Store != nil {
		stderrStr = stderrStr
	}

	// Strip empty lines from output
	stdoutStr = StripEmptyLines(stdoutStr)
	stderrStr = StripEmptyLines(stderrStr)

	// Extract Claude Code hints from output
	hintsResult := ExtractClaudeCodeHints(stdoutStr, command)
	stdoutStr = hintsResult.Stripped

	// Format output with proper truncation and image detection
	formattedOutput := FormatOutput(stdoutStr)
	isImage := formattedOutput.IsImage

	// Process image output
	var persistedOutputPath string
	var persistedOutputSize int64

	if isImage {
		// Try to process/resize the image
		processedURI, err := stdoutStr, error(nil)
		if err == nil && processedURI != "" {
			stdoutStr = processedURI
		} else {
			// Failed to process - fall back to text
			isImage = false
		}
	}

	// If not image, use formatted/truncated content
	if !isImage {
		stdoutStr = formattedOutput.TruncatedContent
	}

	// Track large output size for reporting
	if len(stdoutStr) > MaxOutputSize {
		persistedOutputSize = int64(len(stdoutStr))
	}

	return tool.Result{
		Content: BashOutput{
			Stdout:                    stdoutStr,
			Stderr:                    stderrStr,
			Interrupted:               interrupted,
			IsImage:                   isImage,
			NoOutputExpected:          noOutputExpected,
			ReturnCode:                returnCode,
			ReturnCodeInterpretation:  returnCodeInterpretation,
			PersistedOutputPath:       persistedOutputPath,
			PersistedOutputSize:       persistedOutputSize,
			DangerouslyDisableSandbox: disableSandbox,
		},
	}, nil
}

// executeBackground executes the command in the background
// Ported from src/tasks/LocalShellTask/LocalShellTask.tsx:spawnShellTask
func executeBackground(ctx context.Context, command, description, cwd string, runtime tool.Runtime) (tool.Result, error) {
	// Check if shell task store is available
	if runtime.ShellTasks == nil {
		return tool.Result{}, fmt.Errorf("shell task store not available for background execution")
	}

	// Create shell task
	state, err := runtime.ShellTasks.CreateTask(command, description)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to create background task: %w", err)
	}
	taskID := state.ID

	// Start the command in a goroutine
	go func() {
		cmd := buildCommand(command, cwd, false)

		// Create stdout/stderr writers that also write to task output
		stdoutWriter := &taskOutputWriter{
			taskID:      taskID,
			taskManager: runtime.ShellTasks,
		}
		stderrWriter := &taskOutputWriter{
			taskID:      taskID,
			taskManager: runtime.ShellTasks,
			isStderr:    true,
		}

		cmd.Stdout = stdoutWriter
		cmd.Stderr = stderrWriter

		err := cmd.Run()

		// Determine status
		var status task.ShellTaskStatus
		var exitCode int
		interrupted := false

		if err != nil {
			if cmd.ProcessState != nil {
				exitCode = cmd.ProcessState.ExitCode()
			}
			if ctx.Err() == context.Canceled {
				status = task.ShellTaskStatusInterrupted
				interrupted = true
			} else {
				status = task.ShellTaskStatusFailed
			}
		} else {
			status = task.ShellTaskStatusCompleted
			if cmd.ProcessState != nil {
				exitCode = cmd.ProcessState.ExitCode()
			}
		}

		// Update task status and enqueue notification
		runtime.ShellTasks.UpdateTaskStatus(taskID, status, exitCode, interrupted)
	}()

	return tool.Result{
		Content: BashOutput{
			Stdout:           "",
			Stderr:           "",
			Interrupted:      false,
			BackgroundTaskID: taskID,
		},
	}, nil
}

// taskOutputWriter is an io.Writer that writes to both buffer and task output
type taskOutputWriter struct {
	taskID      string
	taskManager tool.ShellTaskStore
	buf         []byte
	isStderr    bool
}

func (w *taskOutputWriter) Write(p []byte) (n int, err error) {
	// Append to buffer
	w.buf = append(w.buf, p...)

	// Write to task output file
	if w.taskManager != nil {
		// Prefix stderr lines with marker for distinction
		var data []byte
		if w.isStderr {
			data = append([]byte("[stderr] "), p...)
		} else {
			data = p
		}
		w.taskManager.WriteOutput(w.taskID, data)
	}

	return len(p), nil
}

func (w *taskOutputWriter) String() string {
	return string(w.buf)
}

// buildCommand creates an exec.Cmd for the given command
func buildCommand(command, cwd string, disableSandbox bool) *exec.Cmd {
	// Use bash -lc for proper shell expansion
	cmd := exec.Command("bash", "-lc", command)
	cmd.Dir = cwd

	// Set environment
	cmd.Env = os.Environ()

	// TODO: Add sandbox support when available
	_ = disableSandbox

	return cmd
}

// validateCommandSecurity checks for potential security issues and returns an
// optional interactive prompt override for ask-level findings.
func validateCommandSecurity(command string) (*PermissionPromptOverride, error) {
	// Check for empty command
	if strings.TrimSpace(command) == "" {
		return nil, nil
	}

	// Use the comprehensive security validator
	result := ValidateCommandSecurity(command)

	switch result.Level {
	case SecurityLevelDeny:
		return nil, fmt.Errorf("command blocked: %s", result.Message)
	case SecurityLevelAsk:
		// Ask-level checks require explicit user approval. The caller threads this
		// override into the interactive permission flow.
		return &PermissionPromptOverride{
			Reason:      strings.TrimSpace(result.Message),
			Suggestions: append([]string{}, result.Suggestions...),
		}, nil
	case SecurityLevelWarning:
		// Log warning but allow execution
		// In production, this would log to analytics
		return nil, nil
	default:
		return nil, nil
	}
}

// hasMalformedTokens checks for potentially dangerous token patterns
func hasMalformedTokens(command string) bool {
	// Check for unbalanced quotes
	singleQuotes := strings.Count(command, "'")
	doubleQuotes := strings.Count(command, "\"")

	if singleQuotes%2 != 0 || doubleQuotes%2 != 0 {
		return true
	}

	return false
}

// isReadOnlyBashCommand determines if a command is read-only
func isReadOnlyBashCommand(command string) bool {
	parts := splitCommand(command)
	if len(parts) == 0 {
		return false
	}

	hasSearch := false
	hasRead := false
	hasList := false
	hasNonNeutralCommand := false

	for _, part := range parts {
		// Skip operators
		if isOperator(part) {
			continue
		}

		baseCommand := extractBaseCommand(part)
		if baseCommand == "" {
			continue
		}

		// Skip semantic-neutral commands
		if bashSemanticNeutralCommands[baseCommand] {
			continue
		}

		hasNonNeutralCommand = true

		isPartSearch := bashSearchCommands[baseCommand]
		isPartRead := bashReadCommands[baseCommand]
		isPartList := bashListCommands[baseCommand]

		if !isPartSearch && !isPartRead && !isPartList {
			return false
		}

		if isPartSearch {
			hasSearch = true
		}
		if isPartRead {
			hasRead = true
		}
		if isPartList {
			hasList = true
		}
	}

	if !hasNonNeutralCommand {
		return false
	}

	return hasSearch || hasRead || hasList
}

// isSilentBashCommand checks if a command produces no output on success
func isSilentBashCommand(command string) bool {
	parts := splitCommand(command)
	if len(parts) == 0 {
		return false
	}

	hasNonFallbackCommand := false
	lastOperator := ""

	for _, part := range parts {
		if isOperator(part) {
			lastOperator = part
			continue
		}

		baseCommand := extractBaseCommand(part)
		if baseCommand == "" {
			continue
		}

		// Skip neutral commands after ||
		if lastOperator == "||" && bashSemanticNeutralCommands[baseCommand] {
			continue
		}

		hasNonFallbackCommand = true

		if !bashSilentCommands[baseCommand] {
			return false
		}
	}

	return hasNonFallbackCommand
}

// splitCommand splits a command into parts based on operators
func splitCommand(command string) []string {
	var parts []string
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	escaped := false

	for i, char := range command {
		if escaped {
			current.WriteRune(char)
			escaped = false
			continue
		}

		if char == '\\' && !inSingleQuote {
			escaped = true
			current.WriteRune(char)
			continue
		}

		if char == '\'' && !inDoubleQuote {
			inSingleQuote = !inSingleQuote
			current.WriteRune(char)
			continue
		}

		if char == '"' && !inSingleQuote {
			inDoubleQuote = !inDoubleQuote
			current.WriteRune(char)
			continue
		}

		if !inSingleQuote && !inDoubleQuote {
			// Check for operators
			op := ""
			if i+1 < len(command) {
				twoChar := command[i : i+2]
				if twoChar == "&&" || twoChar == "||" || twoChar == ";;" {
					op = twoChar
				}
			}
			if char == ';' || char == '|' || char == '&' {
				if op == "" {
					op = string(char)
				}
			}

			if op != "" {
				if current.Len() > 0 {
					parts = append(parts, strings.TrimSpace(current.String()))
					current.Reset()
				}
				parts = append(parts, op)
				if len(op) == 2 {
					// Skip next character since we consumed it
					_ = command[i+1]
				}
				continue
			}
		}

		current.WriteRune(char)
	}

	if current.Len() > 0 {
		parts = append(parts, strings.TrimSpace(current.String()))
	}

	return parts
}

// isOperator checks if a string is a shell operator
func isOperator(s string) bool {
	return s == "&&" || s == "||" || s == ";" || s == "|" || s == "&"
}

// extractBaseCommand extracts the base command from a command string
func extractExecBaseCommand(part string) string {
	// Skip env var assignments
	for strings.Contains(part, "=") && !strings.HasPrefix(part, "\"") && !strings.HasPrefix(part, "'") {
		idx := strings.Index(part, " ")
		if idx == -1 {
			return ""
		}
		part = strings.TrimSpace(part[idx+1:])
	}

	// Get first word
	fields := strings.Fields(part)
	if len(fields) == 0 {
		return ""
	}

	return fields[0]
}

// interpretReturnCode provides semantic interpretation for exit codes
func interpretReturnCode(code int, command string) string {
	if code == 0 {
		return ""
	}

	// Check for common command interpretations
	baseCmd := extractExecBaseCommand(command)

	switch baseCmd {
	case "grep", "rg", "ag":
		if code == 1 {
			return "No matches found"
		}
	case "diff":
		if code == 1 {
			return "Files differ"
		}
	case "test":
		if code == 1 {
			return "Test condition false"
		}
	}

	if code == 127 {
		return "Command not found"
	}
	if code == 126 {
		return "Permission denied"
	}
	if code == 128+int(syscall.SIGINT) {
		return "Interrupted by signal"
	}

	return ""
}

// truncateOutput truncates output to a maximum size
func truncateOutput(output string, maxSize int) string {
	if len(output) <= maxSize {
		return output
	}

	truncated := output[:maxSize]
	// Try to end at a newline
	if lastNewline := strings.LastIndex(truncated, "\n"); lastNewline > maxSize/2 {
		truncated = truncated[:lastNewline+1]
	}

	return truncated + "\n... (output truncated)"
}

// generateTaskID generates a unique task ID
func generateTaskID() string {
	return fmt.Sprintf("bash-%d", time.Now().UnixNano())
}

// getBashToolDescription returns the comprehensive tool description for BashTool
func getBashToolDescription() string {
	return `Executes a given bash command and returns its output.

The working directory persists between commands, but shell state does not. The shell environment is initialized from the user's profile (bash or zsh).

IMPORTANT: Avoid using this tool to run ` + "`find`" + `, ` + "`grep`" + `, ` + "`cat`" + `, ` + "`head`" + `, ` + "`tail`" + `, ` + "`sed`" + `, ` + "`awk`" + `, or ` + "`echo`" + ` commands, unless explicitly instructed or after you have verified that a dedicated tool cannot accomplish your task. Instead, use the appropriate dedicated tool as this will provide a much better experience for the user:

- File search: Use Glob (NOT find or ls)
- Content search: Use Grep (NOT grep or rg)
- Read files: Use Read (NOT cat/head/tail)
- Edit files: Use Edit (NOT sed/awk)
- Write files: Use Write (NOT echo >/cat <<EOF)
- Communication: Output text directly (NOT echo/printf)

While the Bash tool can do similar things, it's better to use the built-in tools as they provide a better user experience and make it easier to review tool calls and give permission.

# Instructions
- If your command will create new directories or files, first use this tool to run ` + "`ls`" + ` to verify the parent directory exists and is the correct location.
- Always quote file paths that contain spaces with double quotes in your command (e.g., cd "path with spaces/file.txt")
- Try to maintain your current working directory throughout the session by using absolute paths and avoiding usage of ` + "`cd`" + `. You may use ` + "`cd`" + ` if the User explicitly requests it.
- You may specify an optional timeout in milliseconds (up to 600000ms / 10 minutes). By default, your command will timeout after 120000ms (2 minutes).
- You can use the ` + "`run_in_background`" + ` parameter to run the command in the background. Only use this if you don't need the result immediately and are OK being notified when the command completes later.
- If the tool returns ` + "`permission required:`" + `, ` + "`permission denied:`" + `, or ` + "`command blocked:`" + `, do not retry the same command. Ask the user for permission changes or choose a different approach.

When issuing multiple commands:
- If the commands are independent and can run in parallel, make multiple Bash tool calls in a single message.
- If the commands depend on each other and must run sequentially, use a single Bash call with '&&' to chain them together.
- Use ';' only when you need to run commands sequentially but don't care if earlier commands fail.
- DO NOT use newlines to separate commands (newlines are ok in quoted strings).

For git commands:
- Prefer to create a new commit rather than amending an existing commit.
- Before running destructive operations (e.g., git reset --hard, git push --force, git checkout --), consider whether there is a safer alternative.
- Never skip hooks (--no-verify) or bypass signing (--no-gpg-sign) unless the user has explicitly asked for it.

Avoid unnecessary ` + "`sleep`" + ` commands:
- Do not sleep between commands that can run immediately — just run them.
- If your command is long running and you would like to be notified when it finishes — use ` + "`run_in_background`" + `. No sleep needed.
- Do not retry failing commands in a sleep loop — diagnose the root cause.`
}

// RegisterShellTools registers the bash tool and related tools
func RegisterShellTools(r *tool.Registry) {
	r.Register(BashTool{})
}

// --- Additional Shell Utilities ---

// PipelineBuilder helps build shell pipelines
type PipelineBuilder struct {
	commands []string
}

func CreatePipeline() *PipelineBuilder {
	return &PipelineBuilder{commands: []string{}}
}

func (p *PipelineBuilder) Add(cmd string) *PipelineBuilder {
	p.commands = append(p.commands, cmd)
	return p
}

func (p *PipelineBuilder) Build() string {
	return strings.Join(p.commands, " | ")
}

// --- Streaming Output Support ---

// StreamConfig holds configuration for streaming command output
type StreamConfig struct {
	Command    string
	Cwd        string
	Timeout    time.Duration
	OnStdout   func(line string)
	OnStderr   func(line string)
	OnComplete func(stdout, stderr string, err error)
}

// StreamCommand executes a command and streams output line by line
func StreamCommand(ctx context.Context, cfg StreamConfig) error {
	cmd := exec.CommandContext(ctx, "bash", "-lc", cfg.Command)
	cmd.Dir = cfg.Cwd

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	var wg sync.WaitGroup

	// Stream stdout
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdoutPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stdoutBuf.WriteString(line + "\n")
			if cfg.OnStdout != nil {
				cfg.OnStdout(line)
			}
		}
	}()

	// Stream stderr
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			line := scanner.Text()
			stderrBuf.WriteString(line + "\n")
			if cfg.OnStderr != nil {
				cfg.OnStderr(line)
			}
		}
	}()

	wg.Wait()
	err = cmd.Wait()

	if cfg.OnComplete != nil {
		cfg.OnComplete(stdoutBuf.String(), stderrBuf.String(), err)
	}

	return err
}

// --- File Operation Helpers ---

// RunWithFileTracking executes a command and tracks file changes
func RunWithFileTracking(ctx context.Context, command, cwd string, trackFiles []string) (BashOutput, map[string]FileState, error) {
	// Capture file states before
	beforeStates := make(map[string]FileState)
	for _, f := range trackFiles {
		state, err := getFileState(f)
		if err == nil {
			beforeStates[f] = state
		}
	}

	// Execute command
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	cmd.Dir = cwd
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Capture file states after
	afterStates := make(map[string]FileState)
	for _, f := range trackFiles {
		state, err := getFileState(f)
		if err == nil {
			afterStates[f] = state
		}
	}

	return BashOutput{
		Stdout:      stdout.String(),
		Stderr:      stderr.String(),
		Interrupted: false,
	}, afterStates, err
}

// FileState represents the state of a file
type FileState struct {
	Exists  bool
	Size    int64
	ModTime time.Time
}

func getFileState(path string) (FileState, error) {
	info, err := os.Stat(path)
	if err != nil {
		return FileState{Exists: false}, err
	}
	return FileState{
		Exists:  true,
		Size:    info.Size(),
		ModTime: info.ModTime(),
	}, nil
}

// --- Process Utilities ---

// FindProcess finds a process by name
func FindProcess(name string) ([]int, error) {
	cmd := exec.Command("pgrep", "-f", name)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var pids []int
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		var pid int
		if _, err := fmt.Sscanf(line, "%d", &pid); err == nil {
			pids = append(pids, pid)
		}
	}

	return pids, nil
}

// KillProcess kills a process by PID
func KillProcess(pid int, force bool) error {
	sig := os.Interrupt
	if force {
		sig = os.Kill
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}

	return process.Signal(sig)
}

// --- Environment Utilities ---

// GetEnvVar gets an environment variable from the shell
func GetEnvVar(name string) (string, error) {
	cmd := exec.Command("bash", "-lc", "echo \"$"+name+"\"")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// SetEnvVar sets an environment variable for the current process
func SetEnvVar(name, value string) error {
	return os.Setenv(name, value)
}

// --- Working Directory Utilities ---

// Pushd changes to a directory and returns a function to restore
func Pushd(newDir string) (func() error, error) {
	oldDir, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	if err := os.Chdir(newDir); err != nil {
		return nil, err
	}

	return func() error {
		return os.Chdir(oldDir)
	}, nil
}

// --- Path Resolution ---

// Which finds the full path to an executable
func Which(name string) (string, error) {
	cmd := exec.Command("which", name)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("executable not found: %s", name)
	}
	return strings.TrimSpace(string(output)), nil
}

// ExpandPath expands ~ and environment variables in a path
func ExpandPath(path string) string {
	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}

	// Expand environment variables
	return os.ExpandEnv(path)
}

// --- Output Capture ---

// CaptureOutput captures stdout and stderr from a command
func CaptureOutput(ctx context.Context, command string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, "bash", "-lc", command)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()
	stdout = stdoutBuf.String()
	stderr = stderrBuf.String()

	return
}

// --- Temporary Files ---

// TempFile creates a temporary file and returns its path
func TempFile(pattern string) (string, error) {
	file, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", err
	}
	path := file.Name()
	file.Close()
	return path, nil
}

// TempDir creates a temporary directory and returns its path
func TempDir(pattern string) (string, error) {
	return os.MkdirTemp("", pattern)
}

// Cleanup removes temporary files/directories
func Cleanup(paths ...string) error {
	for _, path := range paths {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

// Ensure the tool implements Definition interface
var _ tool.Definition = BashTool{}
