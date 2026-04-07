package services

import "strings"

type PromptSuggestionService struct{}

func EmptyPromptSuggestionService() *PromptSuggestionService {
	return &PromptSuggestionService{}
}

func (s *PromptSuggestionService) Suggest(input string) []string {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}
	return []string{
		"Clarify the target files or modules involved.",
		"State the expected output or verification step.",
		"Keep prompts task-oriented and specific.",
	}
}
