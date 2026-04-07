package services

import (
	"context"
	"fmt"
	"strings"
)

// Tool use summary generator.
// Ported from src/services/toolUseSummary/toolUseSummaryGenerator.ts

const (
	// ToolUseSummarySystemPrompt is the system prompt for summary generation
	ToolUseSummarySystemPrompt = `Write a short summary label describing what these tool calls accomplished. It appears as a single-line row in a mobile app and truncates around 30 characters, so think git-commit-subject, not sentence.

Keep the verb in past tense and the most distinctive noun. Drop articles, connectors, and long location context first.

Examples:
- Searched in auth/
- Fixed NPE in UserService
- Created signup endpoint
- Read config.json
- Ran failing tests`
)

// ToolExecutionInfo contains information about a tool execution
type ToolExecutionInfo struct {
	Name   string
	Input  map[string]any
	Output string
	Error  string
}

// ToolUseSummaryService generates summaries of tool usage.
type ToolUseSummaryService struct {
	provider SummaryProvider
}

// EmptyToolUseSummaryService creates an empty tool use summary service.
func EmptyToolUseSummaryService() *ToolUseSummaryService {
	return &ToolUseSummaryService{}
}

// CreateToolUseSummaryService creates a new tool use summary service with a provider.
func CreateToolUseSummaryService(provider SummaryProvider) *ToolUseSummaryService {
	return &ToolUseSummaryService{provider: provider}
}

// SetProvider sets the LLM provider for summary generation.
func (s *ToolUseSummaryService) SetProvider(provider SummaryProvider) {
	s.provider = provider
}

// GenerateToolUseSummary generates a human-readable summary of completed tools.
// Ported from src/services/toolUseSummary/toolUseSummaryGenerator.ts:generateToolUseSummary
func (s *ToolUseSummaryService) GenerateToolUseSummary(ctx context.Context, tools []ToolExecutionInfo, lastAssistantText string) string {
	if len(tools) == 0 {
		return ""
	}

	// Build tool summaries
	var toolSummaries []string
	for _, tool := range tools {
		inputStr := truncateJSON(tool.Input, 300)
		var outputStr string
		if tool.Error != "" {
			outputStr = "Error: " + tool.Error
			if len(outputStr) > 300 {
				outputStr = outputStr[:297] + "..."
			}
		} else {
			outputStr = truncateJSONStr(tool.Output, 300)
		}
		toolSummaries = append(toolSummaries, fmt.Sprintf("Tool: %s\nInput: %s\nOutput: %s", tool.Name, inputStr, outputStr))
	}

	// Use LLM provider if available
	if s.provider != nil {
		var contextPrefix string
		if lastAssistantText != "" {
			if len(lastAssistantText) > 200 {
				lastAssistantText = lastAssistantText[:200]
			}
			contextPrefix = fmt.Sprintf("User's intent (from assistant's last message): %s\n\n", lastAssistantText)
		}
		prompt := fmt.Sprintf("%sTools completed:\n\n%s\n\nLabel:", contextPrefix, strings.Join(toolSummaries, "\n\n"))
		summary, err := s.provider.GenerateSummary(ctx, ToolUseSummarySystemPrompt+"\n\n"+prompt)
		if err == nil && summary != "" {
			return summary
		}
	}

	// Fallback: generate simple summary based on tool names
	return s.generateSimpleSummary(tools)
}

// generateSimpleSummary generates a simple summary without LLM.
func (s *ToolUseSummaryService) generateSimpleSummary(tools []ToolExecutionInfo) string {
	if len(tools) == 0 {
		return ""
	}

	// Count tools by type
	toolCounts := make(map[string]int)
	for _, t := range tools {
		toolCounts[t.Name]++
	}

	// Generate summary
	var parts []string
	for name, count := range toolCounts {
		if count > 1 {
			parts = append(parts, fmt.Sprintf("%s x%d", name, count))
		} else {
			parts = append(parts, name)
		}
	}

	summary := strings.Join(parts, ", ")
	if len(summary) > 50 {
		summary = summary[:47] + "..."
	}
	return summary + " completed"
}

// SummarizeSingleTool creates a summary for a single tool result.
func (s *ToolUseSummaryService) SummarizeSingleTool(name string, result string, isError bool) string {
	if isError {
		if len(result) > 50 {
			return fmt.Sprintf("%s failed: %s...", name, result[:50])
		}
		return fmt.Sprintf("%s failed: %s", name, result)
	}

	if strings.TrimSpace(result) == "" {
		return fmt.Sprintf("%s completed", name)
	}

	// Truncate result for summary
	if len(result) > 50 {
		return fmt.Sprintf("%s: %s...", name, result[:47])
	}
	return fmt.Sprintf("%s: %s", name, result)
}

// truncateJSON truncates a JSON map for display in prompts.
func truncateJSON(value map[string]any, maxLen int) string {
	if value == nil {
		return "{}"
	}

	var parts []string
	for k, v := range value {
		var valStr string
		switch vv := v.(type) {
		case string:
			if len(vv) > 50 {
				valStr = vv[:47] + "..."
			} else {
				valStr = vv
			}
		case map[string]any, []any:
			valStr = "[...]"
		default:
			valStr = fmt.Sprintf("%v", v)
		}
		parts = append(parts, fmt.Sprintf("%s=%s", k, valStr))

		if totalLen(parts) > maxLen {
			break
		}
	}

	result := "{" + strings.Join(parts, ", ") + "}"
	if len(result) > maxLen {
		return result[:maxLen-3] + "..."
	}
	return result
}

// truncateJSONStr truncates a JSON string for display.
func truncateJSONStr(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "(empty)"
	}
	if len(s) > maxLen {
		return s[:maxLen-3] + "..."
	}
	return s
}

func totalLen(parts []string) int {
	total := 0
	for _, p := range parts {
		total += len(p) + 2 // +2 for ", " separator
	}
	return total
}