package tests

import (
	"claude-code-go/internal/infra/mcp"
	mcptool "claude-code-go/internal/tool/mcp"

	"context"
	"testing"

	"claude-code-go/internal/tool"
)

func TestMCPToolsValidateInputsAndCallRuntime(t *testing.T) {
	t.Parallel()

	runtime := tool.Runtime{
		MCP: stubMCPRuntime{
			dynamic: []tool.MCPDynamicToolInfo{
				{
					Name:        "mcp__demo__workspace_echo",
					Server:      "demo",
					Tool:        "workspace.echo",
					Description: "echo",
					ReadOnly:    true,
				},
			},
		},
	}

	result, err := (mcptool.MCPServersTool{}).Call(context.Background(), tool.Input{}, runtime)
	if err != nil {
		t.Fatalf("mcp_list_servers: %v", err)
	}
	servers, ok := result.Content.([]string)
	if !ok || len(servers) == 0 || servers[0] != "demo" {
		t.Fatalf("unexpected mcp servers: %#v", result.Content)
	}

	if _, err := (mcptool.MCPToolsTool{}).Call(context.Background(), tool.Input{}, runtime); err == nil {
		t.Fatalf("expected missing server error")
	}

	result, err = (mcptool.MCPTemplatesTool{}).Call(context.Background(), tool.Input{"server": "demo"}, runtime)
	if err != nil {
		t.Fatalf("mcp_list_templates: %v", err)
	}
	// MCPTemplatesTool returns []mcp.Template
	templates, ok := result.Content.([]mcp.Template)
	if !ok || len(templates) != 1 {
		t.Fatalf("unexpected templates: %#v", result.Content)
	}

	result, err = (mcptool.MCPSearchTool{}).Call(context.Background(), tool.Input{"query": "echo"}, runtime)
	if err != nil {
		t.Fatalf("tool_search: %v", err)
	}
	// MCPSearchTool returns []mcp.ToolMatch
	matches, ok := result.Content.([]mcp.ToolMatch)
	if !ok || len(matches) != 1 || matches[0].Name != "workspace.echo" {
		t.Fatalf("unexpected search result: %#v", result.Content)
	}

	result, err = (mcptool.MCPCallTool{}).Call(context.Background(), tool.Input{
		"server": "demo",
		"tool":   "workspace.echo",
		"args":   map[string]any{"value": "hello"},
	}, runtime)
	if err != nil {
		t.Fatalf("mcp_call_tool: %v", err)
	}
	if result.Content != "dynamic:demo:workspace.echo" {
		t.Fatalf("unexpected call result: %#v", result.Content)
	}

	result, err = (mcptool.ReadMcpResourceTool{}).Call(context.Background(), tool.Input{
		"server": "demo",
		"uri":    "mcp://demo/readme",
	}, runtime)
	if err != nil {
		t.Fatalf("mcp_read_resource: %v", err)
	}
	// ReadMcpResourceTool returns a map with contents
	contentMap, ok := result.Content.(map[string]any)
	if !ok {
		t.Fatalf("unexpected resource content type: %T", result.Content)
	}
	if contents, ok := contentMap["contents"].([]map[string]any); !ok || len(contents) == 0 {
		t.Fatalf("unexpected resource contents: %#v", contentMap)
	}

	result, err = (mcptool.SimpleMCPAuthTool{}).Call(context.Background(), tool.Input{
		"server": "demo",
		"token":  "abc",
	}, runtime)
	if err != nil {
		t.Fatalf("mcp_auth: %v", err)
	}
	if metaServer, _ := result.Meta["server"].(string); metaServer != "demo" {
		t.Fatalf("unexpected auth meta: %#v", result.Meta)
	}

	result, err = (mcptool.MCPConnectTool{}).Call(context.Background(), tool.Input{"server": "demo"}, runtime)
	if err != nil {
		t.Fatalf("mcp_connect: %v", err)
	}
	if result.Content != "mcp connected" {
		t.Fatalf("unexpected connect result: %#v", result.Content)
	}

	result, err = (mcptool.MCPPingTool{}).Call(context.Background(), tool.Input{"server": "demo"}, runtime)
	if err != nil {
		t.Fatalf("mcp_ping: %v", err)
	}
	if result.Content != "connected" {
		t.Fatalf("unexpected ping result: %#v", result.Content)
	}
}

