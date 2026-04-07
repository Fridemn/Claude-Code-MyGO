package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"claude-code-go/internal/types"
)

// SelectMemoriesSystemPrompt is the system prompt for memory selection.
const SelectMemoriesSystemPrompt = `You are selecting memories that will be useful to Claude Code as it processes a user's query. You will be given the user's query and a list of available memory files with their filenames and descriptions.

Return a list of filenames for the memories that will clearly be useful to Claude Code as it processes the user's query (up to 5). Only include memories that you are certain will be helpful based on their name and description.
- If you are unsure if a memory will be useful in processing the user's query, then do not include it in your list. Be selective and discerning.
- If there are no memories in the list that would clearly be useful, feel free to return an empty list.
- If a list of recently-used tools is provided, do not select memories that are usage reference or API documentation for those tools (Claude Code is already exercising them). DO still select memories containing warnings, gotchas, or known issues about those tools — active use is exactly when those matter.`

// SelectedMemoriesResponse represents the JSON response from LLM selection.
type SelectedMemoriesResponse struct {
	SelectedMemories []string `json:"selected_memories"`
}

// MemorySelector selects relevant memories using LLM.
type MemorySelector struct {
	// SideQuery is a function that performs a side query with structured output.
	// This allows the caller to inject their own LLM client.
	SideQuery func(ctx context.Context, system, userPrompt string, maxTokens int) (string, error)
}

// MemorySelector creates a new memory selector.
func CreateMemorySelector(sideQuery func(ctx context.Context, system, userPrompt string, maxTokens int) (string, error)) *MemorySelector {
	return &MemorySelector{SideQuery: sideQuery}
}

// SelectRelevantMemories uses LLM to select the most relevant memories.
func (s *MemorySelector) SelectRelevantMemories(ctx context.Context, query string, memories []types.MemoryHeader, recentTools []string, maxSelection int) ([]string, error) {
	if len(memories) == 0 {
		return nil, nil
	}

	// Build manifest
	manifest := FormatMemoryManifest(memories)

	// Build user prompt
	var userPrompt strings.Builder
	userPrompt.WriteString("Query: ")
	userPrompt.WriteString(query)
	userPrompt.WriteString("\n\nAvailable memories:\n")
	userPrompt.WriteString(manifest)

	// Add recent tools if any
	if len(recentTools) > 0 {
		userPrompt.WriteString("\n\nRecently used tools: ")
		userPrompt.WriteString(strings.Join(recentTools, ", "))
	}

	// Call LLM
	if s.SideQuery == nil {
		// Fallback to keyword matching if no LLM available
		return s.keywordSelection(query, memories, maxSelection), nil
	}

	response, err := s.SideQuery(ctx, SelectMemoriesSystemPrompt, userPrompt.String(), 256)
	if err != nil {
		// Fallback to keyword matching on error
		return s.keywordSelection(query, memories, maxSelection), nil
	}

	// Parse JSON response
	var result SelectedMemoriesResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return s.keywordSelection(query, memories, maxSelection), nil
	}

	// Filter to valid filenames
	validFilenames := make(map[string]bool)
	for _, m := range memories {
		validFilenames[m.Filename] = true
	}

	var selected []string
	for _, filename := range result.SelectedMemories {
		if validFilenames[filename] {
			selected = append(selected, filename)
		}
	}

	// Limit to maxSelection
	if len(selected) > maxSelection {
		selected = selected[:maxSelection]
	}

	return selected, nil
}

// keywordSelection performs simple keyword-based memory selection.
func (s *MemorySelector) keywordSelection(query string, memories []types.MemoryHeader, maxSelection int) []string {
	var selected []string
	queryLower := strings.ToLower(query)

	// Tokenize query into keywords
	keywords := tokenizeKeywords(queryLower)

	for _, m := range memories {
		if len(selected) >= maxSelection {
			break
		}

		// Check filename
		filenameLower := strings.ToLower(m.Filename)
		for _, kw := range keywords {
			if strings.Contains(filenameLower, kw) {
				selected = append(selected, m.Filename)
				break
			}
		}

		// Check description
		if m.Description != "" {
			descLower := strings.ToLower(m.Description)
			for _, kw := range keywords {
				if strings.Contains(descLower, kw) {
					if !containsString(selected, m.Filename) {
						selected = append(selected, m.Filename)
					}
					break
				}
			}
		}
	}

	return selected
}

func tokenizeKeywords(query string) []string {
	// Simple tokenization - split by space and filter short words
	words := strings.Fields(query)
	var keywords []string
	for _, w := range words {
		if len(w) >= 3 {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// BuildSelectMemoriesPrompt builds the user prompt for memory selection.
func BuildSelectMemoriesPrompt(query string, manifest string, recentTools []string) string {
	var sb strings.Builder
	sb.WriteString("Query: ")
	sb.WriteString(query)
	sb.WriteString("\n\nAvailable memories:\n")
	sb.WriteString(manifest)

	if len(recentTools) > 0 {
		sb.WriteString("\n\nRecently used tools: ")
		sb.WriteString(strings.Join(recentTools, ", "))
	}

	return sb.String()
}

// ParseMemorySelectionResponse parses the LLM response for memory selection.
func ParseMemorySelectionResponse(response string) ([]string, error) {
	var result SelectedMemoriesResponse
	if err := json.Unmarshal([]byte(response), &result); err != nil {
		return nil, fmt.Errorf("failed to parse memory selection response: %w", err)
	}
	return result.SelectedMemories, nil
}
