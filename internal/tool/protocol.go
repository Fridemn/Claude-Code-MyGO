package tool

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"claude-go/internal/task"
	"claude-go/internal/types"
)

// toolCallPattern matches XML-style tool calls. We capture everything between
// the opening tag and closing tag, then parse it as JSON separately to handle
// nested braces correctly.
var toolCallPattern = regexp.MustCompile(`(?s)<tool_call\s+name="([^"]+)">\s*(.*?)\s*</tool_call>`)

type CallSpec struct {
	Name  string
	Input Input
	ID    string
	Raw   string
}

func ParseCalls(text string) ([]CallSpec, error) {
	matches := toolCallPattern.FindAllStringSubmatch(text, -1)
	if len(matches) == 0 {
		return nil, nil
	}

	out := make([]CallSpec, 0, len(matches))
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		toolName := match[1]
		jsonContent := strings.TrimSpace(match[2])

		// Handle empty JSON content
		if jsonContent == "" {
			out = append(out, CallSpec{
				Name:  toolName,
				Input: Input{},
				ID:    "",
				Raw:   match[0],
			})
			continue
		}

		var input Input
		if err := json.Unmarshal([]byte(jsonContent), &input); err != nil {
			return nil, fmt.Errorf("decode tool input for %s: %w", toolName, err)
		}
		out = append(out, CallSpec{
			Name:  toolName,
			Input: input,
			ID:    "",
			Raw:   match[0],
		})
	}
	return out, nil
}

func ParseNativeCalls(calls []types.ToolCall) ([]CallSpec, error) {
	if len(calls) == 0 {
		return nil, nil
	}

	out := make([]CallSpec, 0, len(calls))
	for _, call := range calls {
		var input Input
		if len(call.Arguments) > 0 && strings.TrimSpace(string(call.Arguments)) != "" {
			if err := json.Unmarshal(call.Arguments, &input); err != nil {
				return nil, fmt.Errorf("decode tool input for %s: %w", call.Name, err)
			}
		}
		if input == nil {
			input = Input{}
		}
		out = append(out, CallSpec{
			Name:  call.Name,
			Input: input,
			ID:    call.ID,
			Raw:   string(call.Arguments),
		})
	}
	return out, nil
}

func StripCalls(text string) string {
	return strings.TrimSpace(toolCallPattern.ReplaceAllString(text, ""))
}

func RenderResult(name string, result Result) string {
	lines := []string{
		fmt.Sprintf("tool=%s", name),
	}
	if result.Error != "" {
		lines = append(lines, "status=error", "error="+result.Error)
	} else {
		lines = append(lines, "status=ok")
	}
	if result.Content != nil {
		switch content := result.Content.(type) {
		case string:
			if strings.TrimSpace(content) != "" {
				lines = append(lines, "", content)
			}
		case *task.AgentTask:
			if content != nil {
				lines = append(lines, "", fmt.Sprintf("task_id=%s\nagent=%s\nstatus=%s", content.ID, content.AgentType, content.Status))
			}
		default:
			data, err := json.MarshalIndent(content, "", "  ")
			if err == nil {
				lines = append(lines, "", string(data))
			}
		}
	}
	return strings.Join(lines, "\n")
}

func DefinitionsToTypes(definitions []Definition) []types.ToolDefinition {
	if len(definitions) == 0 {
		return nil
	}

	out := make([]types.ToolDefinition, 0, len(definitions))
	for _, definition := range definitions {
		if definition == nil {
			continue
		}
		schemaBytes, _ := json.Marshal(map[string]any{
			"type":                 "object",
			"properties":           map[string]any{},
			"additionalProperties": true,
		})
		out = append(out, types.ToolDefinition{
			Name:        definition.Name(),
			Description: definition.Description(),
			InputSchema: schemaBytes,
		})
		if provider, ok := definition.(SchemaProvider); ok {
			if schema := provider.ParametersSchema(); len(schema) > 0 {
				schemaBytes, _ := json.Marshal(schema)
				out[len(out)-1].InputSchema = schemaBytes
			}
		}
	}
	return out
}

func SystemPromptFragment(definitions []Definition) string {
	if len(definitions) == 0 {
		return ""
	}

	lines := []string{
		"Available tools:",
	}
	for _, definition := range definitions {
		mode := "write"
		if definition.IsReadOnly(nil) {
			mode = "read"
		}
		lines = append(lines, fmt.Sprintf("- %s [%s]: %s", definition.Name(), mode, definition.Description()))
	}
	replMode := isReplModeEnabledForPrompt()
	lines = append(lines, "")
	lines = append(lines, "Tool preferences for code tasks:")
	if replMode {
		lines = append(lines, "- REPL mode is enabled: use REPL for Read/Write/Edit/Glob/Grep/Bash/NotebookEdit/Agent primitive operations")
	} else {
		lines = append(lines, "- File search: use Glob or list_files (NOT Bash ls/find)")
		lines = append(lines, "- Content search: use Grep (NOT Bash grep/rg)")
		lines = append(lines, "- Read files: use Read (NOT Bash cat/head/tail)")
	}
	lines = append(lines, "- Current repository root should be referenced as '.'; sibling repositories usually live at '../name'")
	lines = append(lines, "")
	lines = append(lines, "If the API runtime exposes native tool calling, use that instead of emitting tool markup in plain text.")
	lines = append(lines, "Only if native tool calling is unavailable, respond with one or more XML blocks in exactly this format:")
	lines = append(lines, `<tool_call name="tool_name">{"key":"value"}`+"")
	if replMode {
		lines = append(lines, "Useful tools for code tasks include REPL, list_files, TaskList, TaskGet, SendMessage, and dynamic MCP tools prefixed with mcp__.")
	} else {
		lines = append(lines, "Useful tools for code tasks include list_files, Read, Write, Edit, Grep, Glob, Bash, TaskList, TaskGet, Agent, SendMessage, and dynamic MCP tools prefixed with mcp__.")
	}
	lines = append(lines, "Use valid JSON inside each block. After tool results are returned, continue the task and provide the final answer without tool_call blocks.")
	return strings.Join(lines, "\n")
}

func isReplModeEnabledForPrompt() bool {
	if isEnvDefinedFalsy(strings.TrimSpace(os.Getenv("CLAUDE_CODE_REPL"))) {
		return false
	}
	if isEnvTruthy(strings.TrimSpace(os.Getenv("CLAUDE_REPL_MODE"))) {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(os.Getenv("USER_TYPE")), "ant") &&
		strings.EqualFold(strings.TrimSpace(os.Getenv("CLAUDE_CODE_ENTRYPOINT")), "cli")
}

func isEnvTruthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func isEnvDefinedFalsy(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return false
	}
	switch strings.ToLower(trimmed) {
	case "0", "false", "no", "off":
		return true
	default:
		return false
	}
}
