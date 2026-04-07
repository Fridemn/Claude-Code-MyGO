package output

import (
	"context"
	"encoding/json"
	"fmt"

	"claude-code-go/internal/tool"
)

// SyntheticOutputToolName is the name of the synthetic output tool
const SyntheticOutputToolName = "StructuredOutput"

// SyntheticOutputDescription describes the synthetic output tool
const SyntheticOutputDescription = `Return structured output in the requested format`

// SyntheticOutputPrompt contains the detailed prompt for the tool
const SyntheticOutputPrompt = `Use this tool to return your final response in the requested structured format. You MUST call this tool exactly once at the end of your response to provide the structured output.`

// SyntheticOutputTool implements the synthetic output tool
type SyntheticOutputTool struct{}

// Name returns the tool name
func (SyntheticOutputTool) Name() string { return SyntheticOutputToolName }

// Description returns the tool description
func (SyntheticOutputTool) Description() string { return SyntheticOutputDescription }

// IsReadOnly returns true as this tool only returns data
func (SyntheticOutputTool) IsReadOnly(tool.Input) bool { return true }

// ParametersSchema returns the JSON schema for the tool parameters
func (SyntheticOutputTool) ParametersSchema() map[string]any {
	// Allow any input object since the schema is provided dynamically
	return tool.SchemaObject(map[string]any{})
}

// Call executes the synthetic output tool
func (t SyntheticOutputTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	// The tool just validates and returns the input as the structured output
	return tool.Result{
		Content: "Structured output provided successfully",
		Meta: map[string]any{
			"structured_output": in,
		},
	}, nil
}

// CreateSyntheticOutputToolWithSchema creates a SyntheticOutputTool with a specific schema
func CreateSyntheticOutputToolWithSchema(jsonSchema map[string]any) (tool.Definition, error) {
	// Validate the schema is valid JSON
	if _, err := json.Marshal(jsonSchema); err != nil {
		return nil, fmt.Errorf("invalid JSON schema: %w", err)
	}

	return &SyntheticOutputToolWithSchema{
		baseSchema: jsonSchema,
	}, nil
}

// SyntheticOutputToolWithSchema implements the synthetic output tool with a custom schema
type SyntheticOutputToolWithSchema struct {
	baseSchema map[string]any
}

// Name returns the tool name
func (t *SyntheticOutputToolWithSchema) Name() string { return SyntheticOutputToolName }

// Description returns the tool description
func (t *SyntheticOutputToolWithSchema) Description() string { return SyntheticOutputDescription }

// IsReadOnly returns true as this tool only returns data
func (t *SyntheticOutputToolWithSchema) IsReadOnly(tool.Input) bool { return true }

// ParametersSchema returns the custom JSON schema
func (t *SyntheticOutputToolWithSchema) ParametersSchema() map[string]any {
	return t.baseSchema
}

// Call executes the synthetic output tool with schema validation
func (t *SyntheticOutputToolWithSchema) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	// Validate input against schema
	if err := validateAgainstSchema(in, t.baseSchema); err != nil {
		return tool.Result{}, fmt.Errorf("output does not match required schema: %w", err)
	}

	return tool.Result{
		Content: "Structured output provided successfully",
		Meta: map[string]any{
			"structured_output": in,
		},
	}, nil
}

// validateAgainstSchema validates input against a JSON schema
// This is a simplified validation - a full implementation would use a proper JSON schema validator
func validateAgainstSchema(input tool.Input, schema map[string]any) error {
	// Check required fields
	if required, ok := schema["required"].([]string); ok {
		for _, field := range required {
			if _, exists := input[field]; !exists {
				return fmt.Errorf("missing required field: %s", field)
			}
		}
	}

	// Check type constraints on properties
	if properties, ok := schema["properties"].(map[string]any); ok {
		for propName, propDef := range properties {
			if value, exists := input[propName]; exists {
				propSchema, ok := propDef.(map[string]any)
				if !ok {
					continue
				}
				if err := validatePropertyType(value, propSchema, propName); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// validatePropertyType validates a single property's type
func validatePropertyType(value any, propSchema map[string]any, propName string) error {
	expectedType, ok := propSchema["type"].(string)
	if !ok {
		return nil // No type constraint
	}

	switch expectedType {
	case "string":
		if _, ok := value.(string); !ok {
			return fmt.Errorf("%s: expected string, got %T", propName, value)
		}
	case "integer", "number":
		switch value.(type) {
		case int, int64, float64:
			// OK
		default:
			return fmt.Errorf("%s: expected number, got %T", propName, value)
		}
	case "boolean":
		if _, ok := value.(bool); !ok {
			return fmt.Errorf("%s: expected boolean, got %T", propName, value)
		}
	case "array":
		if _, ok := value.([]any); !ok {
			return fmt.Errorf("%s: expected array, got %T", propName, value)
		}
	case "object":
		if _, ok := value.(map[string]any); !ok {
			return fmt.Errorf("%s: expected object, got %T", propName, value)
		}
	}

	// Check enum constraints
	if enum, ok := propSchema["enum"].([]any); ok {
		for _, allowed := range enum {
			if value == allowed {
				return nil
			}
		}
		return fmt.Errorf("%s: value %v not in enum %v", propName, value, enum)
	}

	return nil
}

// RegisterOutputTools registers output tools to the registry
func RegisterOutputTools(r *tool.Registry) {
	r.Register(SyntheticOutputTool{})
}