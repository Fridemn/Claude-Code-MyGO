package repl

import (
	"context"
	"fmt"
	"os"
	"strings"

	"claude-go/internal/tool"
	"claude-go/internal/tool/agent"
	"claude-go/internal/tool/bash"
	"claude-go/internal/tool/file"
	"claude-go/internal/tool/notebook"
	"claude-go/internal/tool/search"
)

// REPLToolName is the name of the REPL tool
const REPLToolName = "REPL"

// REPLOnlyTools that are only accessible via REPL when REPL mode is enabled
// When REPL mode is on, these tools are hidden from Claude's direct use,
// forcing Claude to use REPL for batch operations.
var REPLOnlyTools = []string{
	"Read",
	"Write",
	"Edit",
	"Glob",
	"Grep",
	"Bash",
	"NotebookEdit",
	"Agent",
}

// IsModeEnabled mirrors src/tools/REPLTool/constants.ts:isReplModeEnabled.
// This controls whether the REPL tool concept is active in the tool pool.
func IsModeEnabled() bool {
	// Explicit opt-out always wins.
	if isEnvDefinedFalsy(os.Getenv("CLAUDE_CODE_REPL")) {
		return false
	}
	// Legacy override still supported.
	if isEnvTruthy(os.Getenv("CLAUDE_REPL_MODE")) {
		return true
	}
	// Ant CLI default-on behavior.
	return strings.EqualFold(strings.TrimSpace(os.Getenv("USER_TYPE")), "ant") &&
		strings.EqualFold(strings.TrimSpace(os.Getenv("CLAUDE_CODE_ENTRYPOINT")), "cli")
}

// PrimitiveTools returns the list of primitive tools available in REPL mode
func PrimitiveTools() []string {
	return REPLOnlyTools
}

// IsREPLOnlyTool checks if a tool is a REPL-only tool
func IsREPLOnlyTool(name string) bool {
	for _, toolName := range REPLOnlyTools {
		if toolName == name {
			return true
		}
	}
	return false
}

// REPLTool implements the REPL tool concept
// This is primarily a marker/constant file - the actual REPL logic
// would be in the REPL runtime/bridge component
type REPLTool struct{}

// Name returns the tool name
func (REPLTool) Name() string { return REPLToolName }

// Description returns the tool description
func (REPLTool) Description() string {
	return "Interactive REPL for batch tool operations"
}

// IsReadOnly returns false as REPL can modify state
func (REPLTool) IsReadOnly(tool.Input) bool { return false }

// IsSearchOrReadCommand marks REPL wrapper calls as collapsible but absorbed.
// This matches TS collapse behavior where inner virtual tool calls contribute
// counts while the REPL wrapper itself does not break groups or increment them.
func (REPLTool) IsSearchOrReadCommand(tool.Input) tool.SearchOrReadResult {
	return tool.SearchOrReadResult{
		IsCollapsible:      true,
		IsAbsorbedSilently: true,
	}
}

// ParametersSchema returns the JSON schema for the tool parameters
func (REPLTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"script":     tool.SchemaString("JavaScript/TypeScript code to execute in the REPL context"),
		"background": tool.SchemaBoolean("Run in background mode (non-blocking)"),
	}, "script")
}

// Call executes the REPL tool
func (REPLTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if !IsModeEnabled() {
		return tool.Result{
			Error: "REPL mode is disabled for this runtime",
			Meta: map[string]any{
				"code": "mode_disabled",
			},
		}, nil
	}

	rawScript, ok := in["script"]
	if !ok {
		return tool.Result{Error: "missing required field: script"}, nil
	}
	script, ok := rawScript.(string)
	if !ok {
		return tool.Result{Error: fmt.Sprintf("invalid field type for script: %T", rawScript)}, nil
	}
	if strings.TrimSpace(script) == "" {
		return tool.Result{Error: "script must be a non-empty string"}, nil
	}

	background := false
	if rawBackground, exists := in["background"]; exists {
		parsed, ok := rawBackground.(bool)
		if !ok {
			return tool.Result{Error: fmt.Sprintf("invalid field type for background: %T", rawBackground)}, nil
		}
		background = parsed
	}

	plan, parsed := ExtractPrimitiveCalls(script)
	if !parsed || len(plan) == 0 {
		return tool.Result{
			Error: "REPL script must start with a supported primitive tool call",
			Meta: map[string]any{
				"code":          "unsupported_script",
				"background":    background,
				"script_length": len(script),
			},
		}, nil
	}

	wrappedNames := make([]string, 0, len(plan))
	wrappedSteps := make([]map[string]any, 0, len(plan))
	var lastResult tool.Result
	for idx, step := range plan {
		innerToolName := step.Name
		innerInput := cloneInput(step.Input)

		// TS parity direction: background is a REPL wrapper concern.
		// Thread it into primitive tools that support async execution.
		if background && supportsBackgroundFlag(innerToolName) {
			if _, exists := innerInput["run_in_background"]; !exists {
				innerInput["run_in_background"] = true
			}
		}

		emitREPLProgress(runtime, "start", innerToolName, innerInput, nil)

		stepResult, err := callPrimitiveTool(ctx, innerToolName, innerInput, runtime)
		if err != nil {
			emitREPLProgress(runtime, "end", innerToolName, innerInput, &tool.Result{Error: err.Error()})
			return tool.Result{
				Error: err.Error(),
				Meta: map[string]any{
					"code":               "inner_tool_error",
					"background":         background,
					"script_length":      len(script),
					"wrapped_tool_name":  innerToolName,
					"wrapped_tool_input": map[string]any(innerInput),
					"wrapped_tool_index": idx,
				},
			}, nil
		}
		emitREPLProgress(runtime, "end", innerToolName, innerInput, &stepResult)

		if strings.TrimSpace(stepResult.Error) != "" {
			if stepResult.Meta == nil {
				stepResult.Meta = map[string]any{}
			}
			stepResult.Meta["code"] = "inner_tool_error"
			stepResult.Meta["background"] = background
			stepResult.Meta["script_length"] = len(script)
			stepResult.Meta["wrapped_tool_name"] = innerToolName
			stepResult.Meta["wrapped_tool_input"] = map[string]any(innerInput)
			stepResult.Meta["wrapped_tool_index"] = idx
			stepResult.Meta["wrapped_by"] = REPLToolName
			return stepResult, nil
		}

		wrappedNames = append(wrappedNames, innerToolName)
		wrappedSteps = append(wrappedSteps, map[string]any{
			"tool_name":  innerToolName,
			"tool_input": map[string]any(innerInput),
		})
		lastResult = stepResult
	}

	result := lastResult
	if len(plan) > 1 {
		result = tool.Result{
			Content: map[string]any{
				"status":     "ok",
				"step_count": len(plan),
				"steps":      wrappedSteps,
			},
		}
	}
	if result.Meta == nil {
		result.Meta = map[string]any{}
	}
	lastStep := wrappedSteps[len(wrappedSteps)-1]
	result.Meta["background"] = background
	result.Meta["script_length"] = len(script)
	result.Meta["wrapped_tool_name"] = lastStep["tool_name"]
	result.Meta["wrapped_tool_input"] = lastStep["tool_input"]
	result.Meta["wrapped_tool_names"] = wrappedNames
	result.Meta["wrapped_tool_steps"] = wrappedSteps
	result.Meta["wrapped_tool_count"] = len(wrappedNames)
	result.Meta["wrapped_by"] = REPLToolName
	return result, nil
}

// RegisterREPLTools registers REPL tools to the registry
func RegisterREPLTools(r *tool.Registry) {
	r.Register(REPLTool{})
}

func callPrimitiveTool(ctx context.Context, name string, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	switch name {
	case file.FileReadToolName:
		return file.FileReadTool{}.Call(ctx, in, runtime)
	case file.FileWriteToolName:
		return file.FileWriteTool{}.Call(ctx, in, runtime)
	case file.FileEditToolName:
		return file.FileEditTool{}.Call(ctx, in, runtime)
	case "Grep":
		return search.GrepTool{}.Call(ctx, in, runtime)
	case "Glob":
		return search.GlobTool{}.Call(ctx, in, runtime)
	case "Bash":
		return bash.BashTool{}.Call(ctx, in, runtime)
	case notebook.NotebookEditToolName:
		return notebook.NotebookEditTool{}.Call(ctx, in, runtime)
	case agent.AgentToolName:
		return agent.CreateAgentTool(nil).Call(ctx, in, runtime)
	default:
		return tool.Result{}, fmt.Errorf("unsupported REPL primitive: %s", name)
	}
}

func cloneInput(in tool.Input) tool.Input {
	cloned := tool.Input{}
	for k, v := range in {
		cloned[k] = v
	}
	return cloned
}

func supportsBackgroundFlag(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), agent.AgentToolName) ||
		strings.EqualFold(strings.TrimSpace(name), "Bash")
}

func emitREPLProgress(runtime tool.Runtime, phase, toolName string, input tool.Input, result *tool.Result) {
	if runtime.EmitProgress == nil {
		return
	}

	payload := map[string]any{
		"type":      "repl_tool_call",
		"phase":     strings.ToLower(strings.TrimSpace(phase)),
		"toolName":  toolName,
		"toolInput": map[string]any(input),
	}
	if result != nil {
		if strings.TrimSpace(result.Error) != "" {
			payload["status"] = "error"
			payload["error"] = result.Error
		} else {
			payload["status"] = "ok"
		}
	}
	runtime.EmitProgress(payload)
}

func isEnvTruthy(val string) bool {
	normalized := strings.ToLower(strings.TrimSpace(val))
	return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
}

func isEnvDefinedFalsy(val string) bool {
	trimmed := strings.TrimSpace(val)
	if trimmed == "" {
		return false
	}
	normalized := strings.ToLower(trimmed)
	return normalized == "0" || normalized == "false" || normalized == "no" || normalized == "off"
}
