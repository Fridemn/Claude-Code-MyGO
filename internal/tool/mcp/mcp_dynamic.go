package mcp

import (
	"context"
	"regexp"
	"sort"
	"strings"

	"claude-go/internal/tool"
)

var nonWordPattern = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

type dynamicMCPTool struct {
	exportName  string
	server      string
	toolName    string
	description string
	readOnly    bool
}

func (d dynamicMCPTool) Name() string        { return d.exportName }
func (d dynamicMCPTool) Description() string { return d.description }
func (d dynamicMCPTool) IsReadOnly(tool.Input) bool {
	return d.readOnly
}

func (d dynamicMCPTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	out, err := runtime.MCP.CallTool(d.server, d.toolName, map[string]any(in))
	if err != nil {
		return tool.Result{}, err
	}
	return tool.Result{
		Content: out,
		Meta: map[string]any{
			"server": d.server,
			"tool":   d.toolName,
			"mcp":    true,
		},
	}, nil
}

func sanitizeDynamicToolNameLocal(value string) string {
	sanitized := nonWordPattern.ReplaceAllString(strings.TrimSpace(value), "_")
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		return "tool"
	}
	return sanitized
}

// MergedDefinitions merges base tool definitions with dynamic MCP tools.
func MergedDefinitions(base *tool.Registry, runtime tool.Runtime) []tool.Definition {
	merged := make([]tool.Definition, 0)
	seen := map[string]bool{}
	if base != nil {
		for _, def := range base.List() {
			if def == nil {
				continue
			}
			seen[def.Name()] = true
			merged = append(merged, def)
		}
	}
	if runtime.MCP == nil {
		sortDefinitions(merged)
		return merged
	}
	for _, item := range runtime.MCP.DynamicTools() {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = "mcp__" + sanitizeDynamicToolNameLocal(item.Server) + "__" + sanitizeDynamicToolNameLocal(item.Tool)
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		merged = append(merged, dynamicMCPTool{
			exportName:  name,
			server:      item.Server,
			toolName:    item.Tool,
			description: item.Description,
			readOnly:    item.ReadOnly,
		})
	}
	sortDefinitions(merged)
	return merged
}

// LookupDefinition looks up a tool definition by name, checking both base and dynamic MCP tools.
func LookupDefinition(base *tool.Registry, runtime tool.Runtime, name string) (tool.Definition, bool) {
	if base != nil {
		if def, ok := base.Get(name); ok {
			return def, true
		}
	}
	if runtime.MCP == nil {
		return nil, false
	}
	for _, item := range runtime.MCP.DynamicTools() {
		exportName := strings.TrimSpace(item.Name)
		if exportName == "" {
			exportName = "mcp__" + sanitizeDynamicToolNameLocal(item.Server) + "__" + sanitizeDynamicToolNameLocal(item.Tool)
		}
		if exportName != name {
			continue
		}
		return dynamicMCPTool{
			exportName:  exportName,
			server:      item.Server,
			toolName:    item.Tool,
			description: item.Description,
			readOnly:    item.ReadOnly,
		}, true
	}
	return nil, false
}

func sortDefinitions(defs []tool.Definition) {
	sort.Slice(defs, func(i, j int) bool {
		return defs[i].Name() < defs[j].Name()
	})
}
