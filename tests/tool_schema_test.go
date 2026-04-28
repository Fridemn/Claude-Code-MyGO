package tests

import (
	"context"
	"encoding/json"
	"testing"

	bashtool "claude-go/internal/tool/bash"
	"claude-go/internal/tool/file"
	"claude-go/internal/tool"
)

func TestDefinitionsToTypes_UsesToolSchemasWhenAvailable(t *testing.T) {
	t.Parallel()

	defs := tool.DefinitionsToTypes([]tool.Definition{
		file.FileReadTool{},
		bashtool.BashTool{},
		dynamicSchemaFallbackTool{},
	})

	if len(defs) != 3 {
		t.Fatalf("expected 3 tool definitions, got %d", len(defs))
	}

	var readFileSchema map[string]interface{}
	if err := json.Unmarshal(defs[0].InputSchema, &readFileSchema); err != nil {
		t.Fatalf("failed to unmarshal read_file schema: %v", err)
	}
	required, ok := readFileSchema["required"].([]interface{})
	if !ok || len(required) == 0 || required[0] != "file_path" {
		t.Fatalf("expected read_file schema to require path, got %#v", readFileSchema["required"])
	}
	properties, ok := readFileSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected read_file schema properties, got %#v", readFileSchema["properties"])
	}
	if _, ok := properties["offset"]; !ok {
		t.Fatalf("expected read_file schema to expose offset")
	}

	var bashSchema map[string]interface{}
	if err := json.Unmarshal(defs[1].InputSchema, &bashSchema); err != nil {
		t.Fatalf("failed to unmarshal bash schema: %v", err)
	}
	bashProps, ok := bashSchema["properties"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected bash schema properties, got %#v", bashSchema["properties"])
	}
	if _, ok := bashProps["run_in_background"]; !ok {
		t.Fatalf("expected Bash schema to expose run_in_background")
	}

	var fallbackSchema map[string]interface{}
	if err := json.Unmarshal(defs[2].InputSchema, &fallbackSchema); err != nil {
		t.Fatalf("failed to unmarshal fallback schema: %v", err)
	}
	if fallbackSchema["additionalProperties"] != true {
		t.Fatalf("expected fallback schema to allow additional properties, got %#v", fallbackSchema)
	}
}

type dynamicSchemaFallbackTool struct{}

func (dynamicSchemaFallbackTool) Name() string               { return "dynamic_fallback" }
func (dynamicSchemaFallbackTool) Description() string        { return "tool without explicit schema" }
func (dynamicSchemaFallbackTool) IsReadOnly(tool.Input) bool { return true }
func (dynamicSchemaFallbackTool) Call(_ context.Context, _ tool.Input, _ tool.Runtime) (tool.Result, error) {
	return tool.Result{}, nil
}
