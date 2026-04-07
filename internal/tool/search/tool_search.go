package search

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/tool"
)

// ToolSearchToolName is the name of the tool search tool
const ToolSearchToolName = "ToolSearch"

// ToolSearchDescription describes the tool search functionality
const ToolSearchDescription = `Fetches full schema definitions for deferred tools so they can be called.

Deferred tools appear by name in <system-reminder> messages. Until fetched, only the name is known — there is no parameter schema, so the tool cannot be invoked. This tool takes a query, matches it against the deferred tool list, and returns the matched tools' complete JSONSchema definitions inside a <functions> block. Once a tool's schema appears in that result, it is callable exactly like any tool defined at the top of the prompt.

Query forms:
- "select:Read,Edit,Grep" — fetch these exact tools by name
- "notebook jupyter" — keyword search, up to max_results best matches
- "+slack send" — require "slack" in the name, rank by remaining terms`

// ToolSearchTool implements the tool search tool
type ToolSearchTool struct{}

// Name returns the tool name
func (ToolSearchTool) Name() string { return ToolSearchToolName }

// Description returns the tool description
func (ToolSearchTool) Description() string { return ToolSearchDescription }

// IsReadOnly returns true as this tool only searches
func (ToolSearchTool) IsReadOnly(tool.Input) bool { return true }

// ParametersSchema returns the JSON schema for the tool parameters
func (ToolSearchTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"query":       tool.SchemaString("Query to find deferred tools. Use \"select:<tool_name>\" for direct selection, or keywords to search."),
		"max_results": tool.SchemaInteger("Maximum number of results to return (default: 5)"),
	}, "query")
}

// Call executes the tool search tool
func (t ToolSearchTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	query := getString(in, "query")
	maxResults := getInt(in, "max_results", 5)

	if query == "" {
		return tool.Result{}, fmt.Errorf("query is required")
	}

	// Get all registered tools
	allTools := tool.ListAll()

	// Filter for deferred tools (tools that should be deferred)
	// In this implementation, we consider tools with shouldDefer-like behavior
	// as deferred. For now, we'll use a simple heuristic.
	var deferredTools []tool.Definition
	for _, t := range allTools {
		// Skip ToolSearch itself
		if t.Name() == ToolSearchToolName {
			continue
		}
		// In a real implementation, we would check a ShouldDefer flag
		// For now, include all tools as potential matches
		deferredTools = append(deferredTools, t)
	}

	// Check for select: prefix
	if strings.HasPrefix(query, "select:") {
		toolNames := strings.TrimPrefix(query, "select:")
		return t.handleSelectQuery(toolNames, deferredTools, maxResults)
	}

	// Keyword search
	return t.handleKeywordSearch(query, deferredTools, maxResults)
}

// handleSelectQuery handles a select: query
func (ToolSearchTool) handleSelectQuery(toolNames string, deferredTools []tool.Definition, maxResults int) (tool.Result, error) {
	names := strings.Split(toolNames, ",")
	var matches []string
	var missing []string

	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}

		found := false
		for _, t := range deferredTools {
			if t.Name() == name {
				matches = append(matches, name)
				found = true
				break
			}
		}
		if !found {
			missing = append(missing, name)
		}
	}

	if len(matches) == 0 {
		return tool.Result{
			Content: "No matching deferred tools found",
			Meta: map[string]any{
				"matches":              []string{},
				"query":                "select:" + toolNames,
				"total_deferred_tools": len(deferredTools),
			},
		}, nil
	}

	return tool.Result{
		Content: formatToolMatches(matches),
		Meta: map[string]any{
			"matches":              matches,
			"query":                "select:" + toolNames,
			"total_deferred_tools": len(deferredTools),
			"missing":              missing,
		},
	}, nil
}

// handleKeywordSearch handles a keyword search query
func (ToolSearchTool) handleKeywordSearch(query string, deferredTools []tool.Definition, maxResults int) (tool.Result, error) {
	queryLower := strings.ToLower(query)
	queryTerms := strings.Fields(queryLower)

	// Score each tool
	type scoredTool struct {
		name  string
		score int
	}

	var scoredTools []scoredTool

	for _, t := range deferredTools {
		name := strings.ToLower(t.Name())
		desc := strings.ToLower(t.Description())

		score := 0

		// Check for exact name match
		if name == queryLower {
			score = 100
		} else {
			// Check for partial name match
			if strings.Contains(name, queryLower) {
				score += 50
			}

			// Check for term matches in name
			for _, term := range queryTerms {
				if strings.Contains(name, term) {
					score += 20
				}
				// Check for term matches in description
				if strings.Contains(desc, term) {
					score += 10
				}
			}
		}

		if score > 0 {
			scoredTools = append(scoredTools, scoredTool{name: t.Name(), score: score})
		}
	}

	// Sort by score (simple bubble sort for small lists)
	for i := 0; i < len(scoredTools); i++ {
		for j := i + 1; j < len(scoredTools); j++ {
			if scoredTools[j].score > scoredTools[i].score {
				scoredTools[i], scoredTools[j] = scoredTools[j], scoredTools[i]
			}
		}
	}

	// Limit results
	if len(scoredTools) > maxResults {
		scoredTools = scoredTools[:maxResults]
	}

	// Extract matches
	var matches []string
	for _, st := range scoredTools {
		matches = append(matches, st.name)
	}

	if len(matches) == 0 {
		return tool.Result{
			Content: "No matching deferred tools found",
			Meta: map[string]any{
				"matches":              []string{},
				"query":                query,
				"total_deferred_tools": len(deferredTools),
			},
		}, nil
	}

	return tool.Result{
		Content: formatToolMatches(matches),
		Meta: map[string]any{
			"matches":              matches,
			"query":                query,
			"total_deferred_tools": len(deferredTools),
		},
	}, nil
}

// formatToolMatches formats the matched tools
func formatToolMatches(matches []string) string {
	var sb strings.Builder
	sb.WriteString("<functions>\n")
	for _, name := range matches {
		t, ok := tool.Get(name)
		if ok {
			sb.WriteString("<function>")
			sb.WriteString("{\"name\":\"")
			sb.WriteString(name)
			sb.WriteString("\",\"description\":\"")
			sb.WriteString(strings.ReplaceAll(t.Description(), "\"", "\\\""))
			sb.WriteString("\"}")
			sb.WriteString("</function>\n")
		}
	}
	sb.WriteString("</functions>")
	return sb.String()
}

// RegisterToolSearchTools registers tool search tools to the registry
func RegisterToolSearchTools(r *tool.Registry) {
	r.Register(ToolSearchTool{})
}

// Helper functions for extracting values from Input
func getString(in tool.Input, key string) string {
	if v, ok := in[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getInt(in tool.Input, key string, def int) int {
	if v, ok := in[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return def
}