package file

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/tool"
)

// ListMcpResourcesTool implements listing MCP resources
type ListMcpResourcesTool struct{}

func (ListMcpResourcesTool) Name() string { return "ListMcpResourcesTool" }

func (ListMcpResourcesTool) Description() string {
	return "List available resources from configured MCP servers"
}

func (ListMcpResourcesTool) IsReadOnly(tool.Input) bool { return true }

func (ListMcpResourcesTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("Optional server name to filter resources by"),
	})
}

// MCPResource represents an MCP resource
type MCPResource struct {
	URI         string  `json:"uri"`
	Name        string  `json:"name"`
	MimeType    *string `json:"mimeType,omitempty"`
	Description *string `json:"description,omitempty"`
	Server      string  `json:"server"`
}

func (t ListMcpResourcesTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{Content: []MCPResource{}}, nil
	}

	server := tool.GetString(in, "server")

	// Get servers to process
	servers := runtime.MCP.Servers()
	if server != "" {
		// Filter to specific server
		found := false
		for _, s := range servers {
			if s == server {
				found = true
				break
			}
		}
		if !found {
			return tool.Result{}, fmt.Errorf("Server \"%s\" not found. Available servers: %s", server, strings.Join(servers, ", "))
		}
		servers = []string{server}
	}

	// Collect resources from all servers
	resources := make([]MCPResource, 0)
	for _, s := range servers {
		mcpResources := runtime.MCP.ListResources(s)
		for _, r := range mcpResources {
			resources = append(resources, MCPResource{
				URI:         r.URI,
				Name:        r.Name,
				MimeType:    stringPtr(r.MimeType),
				Description: stringPtr(r.Description),
				Server:      s,
			})
		}
	}

	if len(resources) == 0 {
		return tool.Result{
			Content: "No resources found. MCP servers may still provide tools even if they have no resources.",
		}, nil
	}

	return tool.Result{Content: resources}, nil
}

// ReadMcpResourceTool implements reading MCP resources
type ReadMcpResourceTool struct{}

func (ReadMcpResourceTool) Name() string { return "ReadMcpResourceTool" }

func (ReadMcpResourceTool) Description() string {
	return "Reads a specific resource from an MCP server, identified by server name and resource URI"
}

func (ReadMcpResourceTool) IsReadOnly(tool.Input) bool { return true }

func (ReadMcpResourceTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("The MCP server name"),
		"uri":    tool.SchemaString("The resource URI to read"),
	}, "server", "uri")
}

// MCPResourceContent represents the content of an MCP resource
type MCPResourceContent struct {
	URI         string  `json:"uri"`
	MimeType    *string `json:"mimeType,omitempty"`
	Text        *string `json:"text,omitempty"`
	BlobSavedTo *string `json:"blobSavedTo,omitempty"`
}

// MCPResourceReadResult represents the result of reading an MCP resource
type MCPResourceReadResult struct {
	Contents []MCPResourceContent `json:"contents"`
}

func (t ReadMcpResourceTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	server := tool.GetString(in, "server")
	uri := tool.GetString(in, "uri")

	if server == "" {
		return tool.Result{}, fmt.Errorf("server is required")
	}
	if uri == "" {
		return tool.Result{}, fmt.Errorf("uri is required")
	}

	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("MCP runtime is not configured")
	}

	// Check server exists
	servers := runtime.MCP.Servers()
	found := false
	for _, s := range servers {
		if s == server {
			found = true
			break
		}
	}
	if !found {
		return tool.Result{}, fmt.Errorf("Server \"%s\" not found. Available servers: %s", server, strings.Join(servers, ", "))
	}

	// Read the resource
	resource, err := runtime.MCP.ReadResource(server, uri)
	if err != nil {
		return tool.Result{}, err
	}

	// Parse content - MCP resources can be text or binary
	result := MCPResourceReadResult{
		Contents: []MCPResourceContent{
			{
				URI:      resource.URI,
				MimeType: stringPtr(resource.MimeType),
				Text:     stringPtr(resource.Content),
			},
		},
	}

	return tool.Result{Content: result}, nil
}

// stringPtr returns a pointer to the string if non-empty, otherwise nil
func stringPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
