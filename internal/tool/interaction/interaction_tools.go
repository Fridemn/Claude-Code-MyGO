package interaction

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"claude-code-go/internal/tool"
)

// Constants matching TS version
const (
	AskUserQuestionToolName      = "AskUserQuestion"
	AskUserQuestionToolChipWidth = 12
)

// AskUserQuestionToolDescription matches the TS DESCRIPTION
const AskUserQuestionToolDescription = `Asks the user multiple choice questions to gather information, clarify ambiguity, understand preferences, make decisions or offer them choices.

Use this tool when you need to ask the user questions during execution. This allows you to:
1. Gather user preferences or requirements
2. Clarify ambiguous instructions
3. Get decisions on implementation choices as you work
4. Offer choices to the user about what direction to take.

Usage notes:
- Users will always be able to select "Other" to provide custom text input
- Use multiSelect: true to allow multiple answers to be selected for a question
- If you recommend a specific option, make that the first option in the list and add "(Recommended)" at the end of the label

Plan mode note: In plan mode, use this tool to clarify requirements or choose between approaches BEFORE finalizing your plan. Do NOT use this tool to ask "Is my plan ready?" or "Should I proceed?" - use ExitPlanMode for plan approval. IMPORTANT: Do not reference "the plan" in your questions (e.g., "Do you have feedback about the plan?", "Does the plan look good?") because the user cannot see the plan in the UI until you call ExitPlanMode. If you need plan approval, use ExitPlanMode instead.

Preview feature:
Use the optional preview field on options when presenting concrete artifacts that users need to visually compare:
- ASCII mockups of UI layouts or components
- Code snippets showing different implementations
- Diagram variations
- Configuration examples

Preview content is rendered as markdown in a monospace box. Multi-line text with newlines is supported. When any option has a preview, the UI switches to a side-by-side layout with a vertical option list on the left and preview on the right. Do not use previews for simple preference questions where labels and descriptions suffice. Note: previews are only supported for single-select questions (not multiSelect).`

// --- AskUserQuestionTool ---

type AskUserQuestionTool struct{}

func (AskUserQuestionTool) Name() string { return AskUserQuestionToolName }
func (AskUserQuestionTool) Description() string {
	return AskUserQuestionToolDescription
}
func (AskUserQuestionTool) IsReadOnly(tool.Input) bool { return true }

// QuestionAnnotation represents per-question annotations from the user
type QuestionAnnotation struct {
	Preview string `json:"preview,omitempty"` // Preview content of the selected option
	Notes   string `json:"notes,omitempty"`   // Free-text notes the user added
}

// QuestionMetadata represents metadata for tracking and analytics
type QuestionMetadata struct {
	Source string `json:"source,omitempty"` // Optional identifier for the source (e.g., "remember")
}

// QuestionOption represents an option for a question
type QuestionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Preview     string `json:"preview,omitempty"` // Optional preview content (markdown or HTML)
}

// Question represents a question to ask the user
type Question struct {
	Question    string           `json:"question"`
	Header      string           `json:"header"`
	Options     []QuestionOption `json:"options"`
	MultiSelect bool             `json:"multiSelect,omitempty"`
}

// AskUserQuestionInput represents the input for the tool
type AskUserQuestionInput struct {
	Questions   []Question                    `json:"questions"`
	Answers     map[string]string             `json:"answers,omitempty"`
	Annotations map[string]QuestionAnnotation `json:"annotations,omitempty"`
	Metadata    *QuestionMetadata             `json:"metadata,omitempty"`
}

// AskUserQuestionOutput represents the output of the tool
type AskUserQuestionOutput struct {
	Questions   []Question                    `json:"questions"`
	Answers     map[string]string             `json:"answers"`
	Annotations map[string]QuestionAnnotation `json:"annotations,omitempty"`
}

func (AskUserQuestionTool) ParametersSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{
			"questions": map[string]any{
				"type":        "array",
				"description": "Questions to ask the user (1-4 questions)",
				"items":       questionSchema(),
			},
			"answers": map[string]any{
				"type":                 "object",
				"description":          "User answers collected by the permission component",
				"additionalProperties": true,
			},
			"annotations": map[string]any{
				"type":        "object",
				"description": "Optional per-question annotations from the user (e.g., notes on preview selections). Keyed by question text.",
				"additionalProperties": map[string]any{
					"type":       "object",
					"properties": map[string]any{
						"preview": map[string]any{
							"type":        "string",
							"description": "The preview content of the selected option, if the question used previews.",
						},
						"notes": map[string]any{
							"type":        "string",
							"description": "Free-text notes the user added to their selection.",
						},
					},
				},
			},
			"metadata": map[string]any{
				"type":       "object",
				"description": "Optional metadata for tracking and analytics purposes. Not displayed to user.",
				"properties": map[string]any{
					"source": map[string]any{
						"type":        "string",
						"description": "Optional identifier for the source of this question (e.g., 'remember' for /remember command). Used for analytics tracking.",
					},
				},
			},
		},
		"required": []string{"questions"},
	}
}

func questionSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{
			"question": map[string]any{
				"type":        "string",
				"description": "The complete question to ask the user. Should be clear, specific, and end with a question mark.",
			},
			"header": map[string]any{
				"type":        "string",
				"description": "Very short label displayed as a chip/tag (max 12 chars). Examples: 'Auth method', 'Library', 'Approach'.",
			},
			"options": map[string]any{
				"type":        "array",
				"description": "The available choices for this question. Must have 2-4 options. Each option should be a distinct, mutually exclusive choice (unless multiSelect is enabled). There should be no 'Other' option, that will be provided automatically.",
				"items":       optionSchema(),
			},
			"multiSelect": map[string]any{
				"type":        "boolean",
				"description": "Set to true to allow the user to select multiple options instead of just one. Use when choices are not mutually exclusive.",
			},
		},
		"required": []string{"question", "header", "options"},
	}
}

func optionSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{
			"label": map[string]any{
				"type":        "string",
				"description": "The display text for this option that the user will see and select. Should be concise (1-5 words) and clearly describe the choice.",
			},
			"description": map[string]any{
				"type":        "string",
				"description": "Explanation of what this option means or what will happen if chosen. Useful for providing context about trade-offs or implications.",
			},
			"preview": map[string]any{
				"type":        "string",
				"description": "Optional preview content rendered when this option is focused. Use for mockups, code snippets, or visual comparisons that help users compare options.",
			},
		},
		"required": []string{"label"},
	}
}

// UserInputHandler is an interface for getting user input
type UserInputHandler interface {
	AskQuestion(q Question) (string, error)
	AskMultiSelect(q Question) ([]string, error)
}

// Global user input handler
var globalUserInputHandler UserInputHandler

// SetUserInputHandler sets the global user input handler
func SetUserInputHandler(h UserInputHandler) {
	globalUserInputHandler = h
}

func (AskUserQuestionTool) Call(ctx context.Context, in tool.Input, _ tool.Runtime) (tool.Result, error) {
	// Parse questions
	questionsRaw, ok := in["questions"].([]any)
	if !ok || len(questionsRaw) == 0 {
		return tool.Result{}, fmt.Errorf("questions array is required")
	}

	questions := make([]Question, 0, len(questionsRaw))
	for _, qr := range questionsRaw {
		qMap, ok := qr.(map[string]any)
		if !ok {
			continue
		}
		q := Question{
			Question:    getStringFromMap(qMap, "question"),
			Header:      getStringFromMap(qMap, "header"),
			MultiSelect: getBoolFromMap(qMap, "multiSelect"),
		}

		// Parse options
		optsRaw, ok := qMap["options"].([]any)
		if ok {
			for _, or := range optsRaw {
				oMap, ok := or.(map[string]any)
				if !ok {
					continue
				}
				q.Options = append(q.Options, QuestionOption{
					Label:       getStringFromMap(oMap, "label"),
					Description: getStringFromMap(oMap, "description"),
					Preview:     getStringFromMap(oMap, "preview"),
				})
			}
		}

		questions = append(questions, q)
	}

	// Validate questions count (1-4)
	if len(questions) < 1 {
		return tool.Result{}, fmt.Errorf("at least 1 question is required")
	}
	if len(questions) > 4 {
		return tool.Result{}, fmt.Errorf("maximum 4 questions allowed")
	}

	// Validate options count per question (2-4)
	for _, q := range questions {
		if len(q.Options) < 2 {
			return tool.Result{}, fmt.Errorf("question '%s' must have at least 2 options", q.Question)
		}
		if len(q.Options) > 4 {
			return tool.Result{}, fmt.Errorf("question '%s' must have at most 4 options", q.Question)
		}
	}

	// Validate uniqueness: question texts must be unique, option labels must be unique within each question
	if err := validateUniqueness(questions); err != nil {
		return tool.Result{}, err
	}

	// Parse answers (may be pre-filled from permission component)
	answersRaw, ok := in["answers"].(map[string]any)
	answers := make(map[string]string)
	if ok {
		for k, v := range answersRaw {
			if s, ok := v.(string); ok {
				answers[k] = s
			}
		}
	}

	// Parse annotations
	annotationsRaw, ok := in["annotations"].(map[string]any)
	annotations := make(map[string]QuestionAnnotation)
	if ok {
		for qText, annRaw := range annotationsRaw {
			if annMap, ok := annRaw.(map[string]any); ok {
				annotations[qText] = QuestionAnnotation{
					Preview: getStringFromMap(annMap, "preview"),
					Notes:   getStringFromMap(annMap, "notes"),
				}
			}
		}
	}

	// If no answers provided, need user interaction
	if len(answers) == 0 && globalUserInputHandler != nil {
		for _, q := range questions {
			var answer string
			var err error

			if q.MultiSelect {
				// Multi-select
				selected, err := globalUserInputHandler.AskMultiSelect(q)
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to get user input: %w", err)
				}
				answer = strings.Join(selected, ", ")
			} else {
				// Single select
				answer, err = globalUserInputHandler.AskQuestion(q)
				if err != nil {
					return tool.Result{}, fmt.Errorf("failed to get user input: %w", err)
				}
			}

			answers[q.Question] = answer
		}
	}

	output := AskUserQuestionOutput{
		Questions:   questions,
		Answers:     answers,
		Annotations: annotations,
	}

	// Format result matching TS mapToolResultToToolResultBlockParam
	var content strings.Builder
	content.WriteString("User has answered your questions: ")

	answerParts := make([]string, 0, len(answers))
	for qText, answer := range answers {
		parts := []string{fmt.Sprintf(`"%s"="%s"`, qText, answer)}
		if ann, ok := annotations[qText]; ok {
			if ann.Preview != "" {
				parts = append(parts, fmt.Sprintf("selected preview:\n%s", ann.Preview))
			}
			if ann.Notes != "" {
				parts = append(parts, fmt.Sprintf("user notes: %s", ann.Notes))
			}
		}
		answerParts = append(answerParts, strings.Join(parts, " "))
	}
	content.WriteString(strings.Join(answerParts, ", "))
	content.WriteString(". You can now continue with the user's answers in mind.")

	meta := map[string]any{
		"questions": output.Questions,
		"answers":   output.Answers,
	}
	if len(annotations) > 0 {
		meta["annotations"] = annotations
	}

	return tool.Result{
		Content: content.String(),
		Meta:    meta,
	}, nil
}

// validateUniqueness checks that question texts are unique and option labels are unique within each question
func validateUniqueness(questions []Question) error {
	// Check question texts uniqueness
	seenQuestions := make(map[string]bool)
	for _, q := range questions {
		if seenQuestions[q.Question] {
			return fmt.Errorf("question texts must be unique, duplicate found: '%s'", q.Question)
		}
		seenQuestions[q.Question] = true
	}

	// Check option labels uniqueness within each question
	for _, q := range questions {
		seenLabels := make(map[string]bool)
		for _, opt := range q.Options {
			if seenLabels[opt.Label] {
				return fmt.Errorf("option labels must be unique within each question, duplicate found in '%s': '%s'", q.Question, opt.Label)
			}
			seenLabels[opt.Label] = true
		}
	}

	return nil
}

// validateHtmlPreview performs lightweight HTML fragment validation
// Matches TS validateHtmlPreview function
func validateHtmlPreview(preview string) string {
	if preview == "" {
		return ""
	}

	// Check for full document tags
	if matched, _ := regexp.MatchString(`<\s*(html|body|!doctype)\b`, preview); matched {
		return "preview must be an HTML fragment, not a full document (no <html>, <body>, or <!DOCTYPE>)"
	}

	// Check for script/style tags
	if matched, _ := regexp.MatchString(`<\s*(script|style)\b`, preview); matched {
		return "preview must not contain <script> or <style> tags. Use inline styles via the style attribute if needed."
	}

	// Check that it contains HTML tags
	if matched, _ := regexp.MatchString(`<[a-z][^>]*>`, preview); !matched {
		return "preview must contain HTML (previewFormat is set to 'html'). Wrap content in a tag like <div> or <pre>."
	}

	return ""
}

// Helper functions
func getStringFromMap(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getBoolFromMap(m map[string]any, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}

// --- Terminal User Input Handler ---

// TerminalInputHandler implements UserInputHandler for terminal
type TerminalInputHandler struct {
	reader func() (string, error)
	writer func(string)
}

// CreateTerminalInputHandler creates a terminal input handler
func CreateTerminalInputHandler(reader func() (string, error), writer func(string)) *TerminalInputHandler {
	return &TerminalInputHandler{
		reader: reader,
		writer: writer,
	}
}

func (t *TerminalInputHandler) AskQuestion(q Question) (string, error) {
	// Display question
	t.writer(fmt.Sprintf("\n%s\n", q.Question))
	t.writer(fmt.Sprintf("[%s]\n", q.Header))

	// Display options
	for i, opt := range q.Options {
		desc := ""
		if opt.Description != "" {
			desc = fmt.Sprintf(" - %s", opt.Description)
		}
		t.writer(fmt.Sprintf("  %d. %s%s\n", i+1, opt.Label, desc))
	}

	// Get user input
	for {
		t.writer("Select option (1-" + fmt.Sprintf("%d", len(q.Options)) + "): ")

		input, err := t.reader()
		if err != nil {
			return "", err
		}

		input = strings.TrimSpace(input)
		var choice int
		if _, err := fmt.Sscanf(input, "%d", &choice); err != nil {
			t.writer("Invalid input. Please enter a number.\n")
			continue
		}

		if choice < 1 || choice > len(q.Options) {
			t.writer(fmt.Sprintf("Please enter a number between 1 and %d.\n", len(q.Options)))
			continue
		}

		return q.Options[choice-1].Label, nil
	}
}

func (t *TerminalInputHandler) AskMultiSelect(q Question) ([]string, error) {
	// Display question
	t.writer(fmt.Sprintf("\n%s (multi-select)\n", q.Question))
	t.writer(fmt.Sprintf("[%s]\n", q.Header))

	// Display options
	for i, opt := range q.Options {
		desc := ""
		if opt.Description != "" {
			desc = fmt.Sprintf(" - %s", opt.Description)
		}
		t.writer(fmt.Sprintf("  %d. %s%s\n", i+1, opt.Label, desc))
	}

	// Get user input
	t.writer("Select options (comma-separated, e.g., '1,2,3'): ")

	input, err := t.reader()
	if err != nil {
		return nil, err
	}

	input = strings.TrimSpace(input)
	parts := strings.Split(input, ",")

	var selected []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var choice int
		if _, err := fmt.Sscanf(p, "%d", &choice); err != nil {
			continue
		}
		if choice >= 1 && choice <= len(q.Options) {
			selected = append(selected, q.Options[choice-1].Label)
		}
	}

	if len(selected) == 0 {
		return nil, fmt.Errorf("no valid options selected")
	}

	return selected, nil
}

// RegisterInteractionTools registers user interaction tools
func RegisterInteractionTools(r *tool.Registry) {
	r.Register(AskUserQuestionTool{})
}
