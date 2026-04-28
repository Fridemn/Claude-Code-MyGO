package bash

import (
	"claude-go/internal/tool"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// PowerShellTool implements the PowerShell execution tool for Windows.
// On non-Windows platforms, it returns an error message.
type PowerShellTool struct{}

func (PowerShellTool) Name() string { return "PowerShell" }

func (PowerShellTool) Description() string {
	return "Execute PowerShell commands with proper timeout and security controls (Windows only)"
}

func (PowerShellTool) ParametersSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The PowerShell command to execute.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Clear, concise description of what this command does in active voice.",
			},
			"timeout": map[string]any{
				"type":        "integer",
				"description": "Optional timeout in milliseconds (max 600000).",
			},
			"run_in_background": map[string]any{
				"type":        "boolean",
				"description": "Set to true to run this command in the background. Use Read to read the output later.",
			},
			"dangerouslyDisableSandbox": map[string]any{
				"type":        "boolean",
				"description": "Set this to true to dangerously override sandbox mode and run commands without sandboxing.",
			},
		},
		"required": []string{"command"},
	}
}

// IsReadOnly determines if a PowerShell command is read-only (no side effects)
func (PowerShellTool) IsReadOnly(in tool.Input) bool {
	command, _ := in["command"].(string)
	return isReadOnlyPowerShellCommand(command)
}

// Call executes the PowerShell command
func (PowerShellTool) Call(ctx context.Context, in tool.Input, rt tool.Runtime) (tool.Result, error) {
	// Check platform - PowerShell tool is only available on Windows
	if runtime.GOOS != "windows" {
		return tool.Result{
			Content: PowerShellOutput{
				Stdout:      "",
				Stderr:      "PowerShell tool is only available on Windows. On Unix-like systems, use the Bash tool instead.",
				Interrupted: false,
			},
		}, nil
	}

	// Extract input parameters
	command := tool.GetString(in, "command")
	description := tool.GetString(in, "description")
	timeoutMs := tool.GetInt(in, "timeout", 0)
	runInBackground := tool.GetBool(in, "run_in_background")
	disableSandbox := tool.GetBool(in, "dangerouslyDisableSandbox")

	if strings.TrimSpace(command) == "" {
		return tool.Result{}, fmt.Errorf("command is required")
	}

	// Set timeout
	timeout := timeoutMs
	if timeout <= 0 {
		timeout = 120000 // 2 minutes default
	}
	if timeout > 600000 { // 10 minutes max
		timeout = 600000
	}

	// Get working directory
	cwd := "."
	if rt.Store != nil {
		cwd = rt.Store.GetCWD()
	}

	// Validate command for security issues
	if securityErr := validatePowerShellSecurity(command); securityErr != nil {
		errMsg := securityErr.Error()
		return tool.Result{
			Error: errMsg,
			Content: PowerShellOutput{
				Stderr:      errMsg,
				Interrupted: false,
			},
		}, nil
	}

	// Check for blocked sleep patterns (PowerShell equivalent)
	if blockedPattern := detectBlockedPowerShellSleepPattern(command); blockedPattern != "" {
		errMsg := fmt.Sprintf("Blocked: %s. Run blocking commands in the background with run_in_background: true.", blockedPattern)
		return tool.Result{
			Error: errMsg,
			Content: PowerShellOutput{
				Stderr:      errMsg,
				Interrupted: false,
			},
		}, nil
	}

	// Check permissions
	permResult := checkPowerShellPermission(command, description)
	if !permResult.Allowed {
		errMsg := fmt.Sprintf("permission denied: %s", permResult.Reason)
		return tool.Result{
			Error: errMsg,
			Content: PowerShellOutput{
				Stderr:      errMsg,
				Interrupted: false,
			},
		}, nil
	}

	// Handle background execution
	if runInBackground {
		return executePowerShellBackground(ctx, command, description, cwd, rt)
	}

	// Execute synchronously
	return executePowerShellSync(ctx, command, cwd, timeout, disableSandbox, rt)
}

// PowerShellOutput represents the output of the PowerShell tool
type PowerShellOutput struct {
	Stdout                    string `json:"stdout"`
	Stderr                    string `json:"stderr"`
	Interrupted               bool   `json:"interrupted"`
	ReturnCodeInterpretation  string `json:"returnCodeInterpretation,omitempty"`
	IsImage                   bool   `json:"isImage,omitempty"`
	PersistedOutputPath       string `json:"persistedOutputPath,omitempty"`
	PersistedOutputSize       int64  `json:"persistedOutputSize,omitempty"`
	BackgroundTaskID          string `json:"backgroundTaskId,omitempty"`
	BackgroundedByUser        bool   `json:"backgroundedByUser,omitempty"`
	AssistantAutoBackgrounded bool   `json:"assistantAutoBackgrounded,omitempty"`
}

// executePowerShellSync executes a PowerShell command synchronously
func executePowerShellSync(ctx context.Context, command, cwd string, timeout int, disableSandbox bool, rt tool.Runtime) (tool.Result, error) {
	// Create context with timeout
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Millisecond)
	defer cancel()

	// Build PowerShell command
	cmd := buildPowerShellCommand(command, cwd, disableSandbox)

	// Set up output capture
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Start the command
	startTime := time.Now()
	err := cmd.Start()
	if err != nil {
		return tool.Result{
			Content: PowerShellOutput{
				Stderr:      fmt.Sprintf("Failed to start PowerShell command: %v", err),
				Interrupted: false,
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
	returnCodeInterpretation := interpretPowerShellReturnCode(returnCode, command)

	// Strip empty lines from output
	stdoutStr = stripEmptyLines(stdoutStr)
	stderrStr = stripEmptyLines(stderrStr)

	// Check for image output
	isImage := strings.HasPrefix(stdoutStr, "data:image/")

	// Handle large output
	var persistedOutputSize int64
	if len(stdoutStr) > 30000 {
		persistedOutputSize = int64(len(stdoutStr))
	}

	return tool.Result{
		Content: PowerShellOutput{
			Stdout:                   stdoutStr,
			Stderr:                   stderrStr,
			Interrupted:              interrupted,
			ReturnCodeInterpretation: returnCodeInterpretation,
			IsImage:                  isImage,
			PersistedOutputSize:      persistedOutputSize,
		},
	}, nil
}

// executePowerShellBackground executes a PowerShell command in the background
func executePowerShellBackground(ctx context.Context, command, description, cwd string, rt tool.Runtime) (tool.Result, error) {
	// Create a background task
	if rt.Tasks == nil {
		return tool.Result{}, fmt.Errorf("task store not available for background execution")
	}

	// Generate task ID
	taskID := fmt.Sprintf("ps-task-%d", time.Now().UnixNano())

	// Start the command in a goroutine
	go func() {
		cmd := buildPowerShellCommand(command, cwd, false)
		cmd.Run()
		// tool.Result would be stored in task system
	}()

	return tool.Result{
		Content: PowerShellOutput{
			Stdout:           "",
			Stderr:           "",
			Interrupted:      false,
			BackgroundTaskID: taskID,
		},
	}, nil
}

// buildPowerShellCommand creates an exec.Cmd for PowerShell execution
func buildPowerShellCommand(command, cwd string, disableSandbox bool) *exec.Cmd {
	// Use PowerShell with -NonInteractive -NoProfile -Command
	// This matches the TypeScript implementation
	cmd := exec.Command("powershell.exe", "-NonInteractive", "-NoProfile", "-Command", command)
	cmd.Dir = cwd

	// Set environment
	cmd.Env = os.Environ()

	// TODO: Add sandbox support when available
	_ = disableSandbox

	return cmd
}

// stripEmptyLines removes leading and trailing empty lines
func stripEmptyLines(content string) string {
	lines := strings.Split(content, "\n")

	// Find first non-empty line
	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}

	// Find last non-empty line
	end := len(lines) - 1
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}

	if start > end {
		return ""
	}

	return strings.Join(lines[start:end+1], "\n")
}

// RegisterPowerShellTool registers the PowerShell tool
func RegisterPowerShellTool(r *tool.Registry) {
	r.Register(PowerShellTool{})
}
