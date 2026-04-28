package services

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"claude-go/internal/types"
)

// Hook execution constants
const (
	DefaultHookTimeoutMs       = 60000
	ToolHookExecutionTimeoutMs = 30000
	SessionEndHookTimeoutMs    = 5000
)

// HookExecutor handles hook execution logic.
type HookExecutor struct {
	service        *HooksService
	cwd            string
	sessionID      string
	transcriptPath string
}

// NewHookExecutor creates a new hook executor.
func NewHookExecutor(service *HooksService, cwd, sessionID, transcriptPath string) *HookExecutor {
	return &HookExecutor{
		service:        service,
		cwd:            cwd,
		sessionID:      sessionID,
		transcriptPath: transcriptPath,
	}
}

// ExecuteHooks executes all matching hooks for the given input.
func (e *HookExecutor) ExecuteHooks(ctx context.Context, hookInput types.HookInput, matchQuery string, timeoutMs int) ([]types.HookResult, error) {
	eventName := hookInput.GetHookEventName()
	matchedHooks := e.service.GetMatchingHooks(eventName, matchQuery)

	if len(matchedHooks) == 0 {
		return nil, nil
	}

	var results []types.HookResult
	for _, hook := range matchedHooks {
		if !hook.Enabled {
			continue
		}

		result, err := e.executeSingleHook(ctx, hook, hookInput, timeoutMs)
		if err != nil && hook.Blocking {
			return results, err
		}
		results = append(results, result)
	}

	return results, nil
}

// executeSingleHook executes a single hook and returns its result.
func (e *HookExecutor) executeSingleHook(ctx context.Context, hook Hook, input types.HookInput, timeoutMs int) (types.HookResult, error) {
	if timeoutMs <= 0 {
		timeoutMs = hook.TimeoutMs
		if timeoutMs <= 0 {
			timeoutMs = DefaultHookTimeoutMs
		}
	}

	timeout := time.Duration(timeoutMs) * time.Millisecond
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	startTime := time.Now()

	// Prepare input JSON
	inputJSON, err := json.Marshal(input)
	if err != nil {
		return types.HookResult{
			HookName:  hook.Event,
			HookEvent: input.GetHookEventName(),
			Continue:  true,
			Error:     err.Error(),
		}, err
	}

	// Execute hook command
	output, err := e.runHookCommand(runCtx, hook, string(inputJSON))

	duration := time.Since(startTime)

	result := types.HookResult{
		HookName:   hook.Event,
		HookEvent:  input.GetHookEventName(),
		Output:     strings.TrimSpace(output),
		Continue:   true,
		DurationMs: int(duration.Milliseconds()),
	}

	if err != nil {
		result.Error = err.Error()
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = fmt.Sprintf("hook timed out after %dms", timeoutMs)
		}
	}

	// Parse JSON output if present
	if output != "" {
		parsed := e.parseHookJSONOutput(output)
		if parsed != nil {
			result.Continue = parsed.Sync != nil && parsed.Sync.Continue
			if parsed.Sync != nil {
				result.Decision = parsed.Sync.Decision
				if parsed.Sync.StopReason != "" {
					result.Error = parsed.Sync.StopReason
					result.IsBlocking = true
				}
			}
		}
	}

	// Update hook stats
	e.service.UpdateHookStats(hook.Event, result)

	return result, nil
}

// runHookCommand executes a hook shell command.
func (e *HookExecutor) runHookCommand(ctx context.Context, hook Hook, inputJSON string) (string, error) {
	command := strings.TrimSpace(hook.Command)
	if command == "" {
		return "", fmt.Errorf("empty hook command")
	}

	shell := strings.TrimSpace(hook.Shell)
	if shell == "" {
		shell = "bash"
	}

	cmd := exec.CommandContext(ctx, shell, "-lc", command)
	cmd.Dir = e.cwd

	// Build environment variables
	env := os.Environ()
	env = append(env,
		buildHookEnvVar("CLAUDE_CODE_SESSION_ID", e.sessionID),
		buildHookEnvVar("CLAUDE_CODE_HOOK_EVENT_NAME", hook.Event),
		buildHookEnvVar("CLAUDE_CODE_TRANSCRIPT_PATH", e.transcriptPath),
		buildHookEnvVar("CLAUDE_CODE_CWD", e.cwd),
	)

	// Add hook input as JSON environment variable
	env = append(env, buildHookEnvVar("CLAUDE_CODE_HOOK_INPUT", inputJSON))

	cmd.Env = env

	// Capture stdout and stderr separately
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// parseHookJSONOutput parses JSON output from a hook.
func (e *HookExecutor) parseHookJSONOutput(output string) *types.HookJSONOutput {
	// Find JSON in output (hooks may output other text before/after JSON)
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Try to parse as JSON
		if strings.HasPrefix(line, "{") && strings.HasSuffix(line, "}") {
			var syncOutput types.SyncHookOutput
			if err := json.Unmarshal([]byte(line), &syncOutput); err == nil {
				return &types.HookJSONOutput{Sync: &syncOutput}
			}

			var asyncOutput types.AsyncHookOutput
			if err := json.Unmarshal([]byte(line), &asyncOutput); err == nil && asyncOutput.Async {
				return &types.HookJSONOutput{Async: &asyncOutput}
			}
		}
	}

	return nil
}

// ExecutePreToolHooks executes PreToolUse hooks.
func (e *HookExecutor) ExecutePreToolHooks(ctx context.Context, toolName, toolUseID string, toolInput interface{}) ([]types.HookResult, error) {
	input := types.PreToolUseHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "PreToolUse",
		ToolName:      toolName,
		ToolInput:     toolInput,
		ToolUseID:     toolUseID,
	}

	return e.ExecuteHooks(ctx, input, toolName, ToolHookExecutionTimeoutMs)
}

// ExecutePostToolHooks executes PostToolUse hooks.
func (e *HookExecutor) ExecutePostToolHooks(ctx context.Context, toolName, toolUseID string, toolInput, toolResponse interface{}) ([]types.HookResult, error) {
	input := types.PostToolUseHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "PostToolUse",
		ToolName:      toolName,
		ToolInput:     toolInput,
		ToolResponse:  toolResponse,
		ToolUseID:     toolUseID,
	}

	return e.ExecuteHooks(ctx, input, toolName, ToolHookExecutionTimeoutMs)
}

// ExecutePostToolUseFailureHooks executes PostToolUseFailure hooks.
func (e *HookExecutor) ExecutePostToolUseFailureHooks(ctx context.Context, toolName, toolUseID string, toolInput interface{}, errMsg string) ([]types.HookResult, error) {
	input := types.PostToolUseFailureHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "PostToolUseFailure",
		ToolName:      toolName,
		ToolInput:     toolInput,
		Error:         errMsg,
		ToolUseID:     toolUseID,
	}

	return e.ExecuteHooks(ctx, input, toolName, ToolHookExecutionTimeoutMs)
}

// ExecuteSessionStartHooks executes SessionStart hooks.
func (e *HookExecutor) ExecuteSessionStartHooks(ctx context.Context, source string) ([]types.HookResult, error) {
	input := types.SessionStartHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "SessionStart",
		Source:        source,
	}

	return e.ExecuteHooks(ctx, input, source, DefaultHookTimeoutMs)
}

// ExecuteSessionEndHooks executes SessionEnd hooks.
func (e *HookExecutor) ExecuteSessionEndHooks(ctx context.Context, reason string) ([]types.HookResult, error) {
	input := types.SessionEndHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "SessionEnd",
		Reason:        reason,
	}

	return e.ExecuteHooks(ctx, input, reason, SessionEndHookTimeoutMs)
}

// ExecuteStopHooks executes Stop hooks.
func (e *HookExecutor) ExecuteStopHooks(ctx context.Context, reason string) ([]types.HookResult, error) {
	input := types.StopHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "Stop",
		Reason:        reason,
	}

	return e.ExecuteHooks(ctx, input, "", DefaultHookTimeoutMs)
}

// ExecuteNotificationHooks executes Notification hooks.
func (e *HookExecutor) ExecuteNotificationHooks(ctx context.Context, notificationType, title, message string) ([]types.HookResult, error) {
	input := types.NotificationHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName:    "Notification",
		NotificationType: notificationType,
		Title:            title,
		Message:          message,
	}

	return e.ExecuteHooks(ctx, input, notificationType, DefaultHookTimeoutMs)
}

// ExecuteUserPromptSubmitHooks executes UserPromptSubmit hooks.
func (e *HookExecutor) ExecuteUserPromptSubmitHooks(ctx context.Context, prompt string) ([]types.HookResult, error) {
	input := types.UserPromptSubmitHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "UserPromptSubmit",
		Prompt:        prompt,
	}

	return e.ExecuteHooks(ctx, input, "", DefaultHookTimeoutMs)
}

// ExecuteSubagentStartHooks executes SubagentStart hooks.
func (e *HookExecutor) ExecuteSubagentStartHooks(ctx context.Context, agentType, taskID, description string) ([]types.HookResult, error) {
	input := types.SubagentStartHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "SubagentStart",
		AgentType:     agentType,
		TaskID:        taskID,
		Description:   description,
	}

	return e.ExecuteHooks(ctx, input, agentType, DefaultHookTimeoutMs)
}

// ExecuteSubagentStopHooks executes SubagentStop hooks.
func (e *HookExecutor) ExecuteSubagentStopHooks(ctx context.Context, agentType, taskID, result string) ([]types.HookResult, error) {
	input := types.SubagentStopHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "SubagentStop",
		AgentType:     agentType,
		TaskID:        taskID,
		Result:        result,
	}

	return e.ExecuteHooks(ctx, input, agentType, DefaultHookTimeoutMs)
}

// ExecutePreCompactHooks executes PreCompact hooks.
func (e *HookExecutor) ExecutePreCompactHooks(ctx context.Context, trigger string) ([]types.HookResult, error) {
	input := types.PreCompactHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "PreCompact",
		Trigger:       trigger,
	}

	return e.ExecuteHooks(ctx, input, trigger, DefaultHookTimeoutMs)
}

// ExecutePostCompactHooks executes PostCompact hooks.
func (e *HookExecutor) ExecutePostCompactHooks(ctx context.Context, trigger string, wasCompacted bool, tokensBefore, tokensAfter int) ([]types.HookResult, error) {
	input := types.PostCompactHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "PostCompact",
		Trigger:       trigger,
		WasCompacted:  wasCompacted,
		TokensBefore:  tokensBefore,
		TokensAfter:   tokensAfter,
	}

	return e.ExecuteHooks(ctx, input, trigger, DefaultHookTimeoutMs)
}

// ExecutePermissionDeniedHooks executes PermissionDenied hooks.
func (e *HookExecutor) ExecutePermissionDeniedHooks(ctx context.Context, toolName, toolUseID string, toolInput interface{}, reason string) ([]types.HookResult, error) {
	input := types.PermissionDeniedHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "PermissionDenied",
		ToolName:      toolName,
		ToolInput:     toolInput,
		ToolUseID:     toolUseID,
		Reason:        reason,
	}

	return e.ExecuteHooks(ctx, input, toolName, ToolHookExecutionTimeoutMs)
}

// ExecuteTaskCreatedHooks executes TaskCreated hooks.
func (e *HookExecutor) ExecuteTaskCreatedHooks(ctx context.Context, taskID, taskType string) ([]types.HookResult, error) {
	input := types.TaskCreatedHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "TaskCreated",
		TaskID:        taskID,
		TaskType:      taskType,
	}

	return e.ExecuteHooks(ctx, input, "", DefaultHookTimeoutMs)
}

// ExecuteTaskCompletedHooks executes TaskCompleted hooks.
func (e *HookExecutor) ExecuteTaskCompletedHooks(ctx context.Context, taskID, taskType, status string) ([]types.HookResult, error) {
	input := types.TaskCompletedHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "TaskCompleted",
		TaskID:        taskID,
		TaskType:      taskType,
		Status:        status,
	}

	return e.ExecuteHooks(ctx, input, "", DefaultHookTimeoutMs)
}

// ExecuteConfigChangeHooks executes ConfigChange hooks.
func (e *HookExecutor) ExecuteConfigChangeHooks(ctx context.Context, source string, changes map[string]interface{}) ([]types.HookResult, error) {
	input := types.ConfigChangeHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "ConfigChange",
		Source:        source,
		Changes:       changes,
	}

	return e.ExecuteHooks(ctx, input, source, DefaultHookTimeoutMs)
}

// ExecuteCwdChangedHooks executes CwdChanged hooks.
func (e *HookExecutor) ExecuteCwdChangedHooks(ctx context.Context, oldCwd, newCwd string) ([]types.HookResult, error) {
	input := types.CwdChangedHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            newCwd,
		},
		HookEventName: "CwdChanged",
		OldCWD:        oldCwd,
		NewCWD:        newCwd,
	}

	return e.ExecuteHooks(ctx, input, "", DefaultHookTimeoutMs)
}

// ExecuteFileChangedHooks executes FileChanged hooks.
func (e *HookExecutor) ExecuteFileChangedHooks(ctx context.Context, filePath, changeType string) ([]types.HookResult, error) {
	input := types.FileChangedHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "FileChanged",
		FilePath:      filePath,
		ChangeType:    changeType,
	}

	// Use file basename as match query
	matchQuery := filepath.Base(filePath)
	return e.ExecuteHooks(ctx, input, matchQuery, DefaultHookTimeoutMs)
}

// ExecuteSetupHooks executes Setup hooks.
func (e *HookExecutor) ExecuteSetupHooks(ctx context.Context, trigger string) ([]types.HookResult, error) {
	input := types.SetupHookInput{
		BaseHookInput: types.BaseHookInput{
			SessionID:      e.sessionID,
			TranscriptPath: e.transcriptPath,
			CWD:            e.cwd,
		},
		HookEventName: "Setup",
		Trigger:       trigger,
	}

	return e.ExecuteHooks(ctx, input, trigger, DefaultHookTimeoutMs)
}

// HasBlockingResult checks if any result is blocking.
func HasBlockingResult(results []types.HookResult) bool {
	for _, result := range results {
		if result.IsBlocking || !result.Continue {
			return true
		}
		if result.Decision == "block" {
			return true
		}
	}
	return false
}

// GetBlockingMessage returns the blocking message from results.
func GetBlockingMessage(results []types.HookResult) string {
	for _, result := range results {
		if result.IsBlocking || !result.Continue {
			return result.Error
		}
		if result.Decision == "block" && result.Error != "" {
			return result.Error
		}
	}
	return ""
}

// GetPreToolHookBlockingMessage returns a formatted blocking message for PreToolUse.
func GetPreToolHookBlockingMessage(toolName string, results []types.HookResult) string {
	msg := GetBlockingMessage(results)
	if msg == "" {
		return fmt.Sprintf("Hook blocked tool execution: %s", toolName)
	}
	return msg
}

// GetStopHookMessage returns a formatted message for Stop hooks.
func GetStopHookMessage(results []types.HookResult) string {
	msg := GetBlockingMessage(results)
	if msg == "" {
		return "Hook requested stop"
	}
	return msg
}

// GetAdditionalContext extracts additional context from hook results.
func GetAdditionalContext(results []types.HookResult) string {
	var contexts []string
	for _, result := range results {
		if result.Output != "" {
			// Try to parse JSON output for additional context
			lines := strings.Split(result.Output, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "{") {
					var sync types.SyncHookOutput
					if json.Unmarshal([]byte(line), &sync) == nil && sync.AdditionalContext != "" {
						contexts = append(contexts, sync.AdditionalContext)
					}
				}
			}
		}
	}
	return strings.Join(contexts, "\n")
}
