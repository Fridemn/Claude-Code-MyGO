package mcp

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"claude-code-go/internal/tool"
)

// MCPTool is a stub tool for dynamic MCP tool invocation.
// It serves as a placeholder - actual MCP tools are created dynamically via DynamicTools().
// Mirrors TS MCPTool which is overridden at runtime.
type MCPTool struct{}

func (MCPTool) Name() string        { return "mcp" }
func (MCPTool) Description() string { return "Call a tool on an MCP server. Parameters are overridden at runtime." }
func (MCPTool) IsReadOnly(tool.Input) bool {
	// Overridden at runtime based on actual MCP tool
	return false
}
func (MCPTool) ParametersSchema() map[string]any {
	// Overridden at runtime based on actual MCP tool schema
	return tool.SchemaObject(map[string]any{})
}
func (MCPTool) Call(_ context.Context, _ tool.Input, _ tool.Runtime) (tool.Result, error) {
	// This is a stub - actual calls are handled by dynamicMCPTool
	return tool.Result{Content: ""}, nil
}

// MCPServersTool lists all configured MCP servers.
type MCPServersTool struct{}

func (MCPServersTool) Name() string          { return "mcp_list_servers" }
func (MCPServersTool) Description() string   { return "list configured MCP servers" }
func (MCPServersTool) IsReadOnly(tool.Input) bool { return true }
func (MCPServersTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{})
}
func (MCPServersTool) Call(_ context.Context, _ tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	return tool.Result{Content: runtime.MCP.Servers()}, nil
}

// MCPToolsTool lists tools for one MCP server.
type MCPToolsTool struct{}

func (MCPToolsTool) Name() string          { return "mcp_list_tools" }
func (MCPToolsTool) Description() string   { return "list tools for one MCP server" }
func (MCPToolsTool) IsReadOnly(tool.Input) bool { return true }
func (MCPToolsTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("Configured MCP server name."),
	}, "server")
}
func (MCPToolsTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	server, _ := in["server"].(string)
	if strings.TrimSpace(server) == "" {
		return tool.Result{}, fmt.Errorf("server is required")
	}
	return tool.Result{Content: runtime.MCP.ListTools(server)}, nil
}

// ListMcpResourcesTool lists available resources from configured MCP servers.
// Mirrors TS ListMcpResourcesTool - can list from all servers or filter by server name.
type ListMcpResourcesTool struct{}

func (ListMcpResourcesTool) Name() string { return "ListMcpResourcesTool" }
func (ListMcpResourcesTool) Description() string {
	return `Lists available resources from configured MCP servers.
Each resource object includes a 'server' field indicating which server it's from.

Usage examples:
- List all resources from all servers: listMcpResources
- List resources from a specific server: listMcpResources({ server: "myserver" })`
}
func (ListMcpResourcesTool) IsReadOnly(tool.Input) bool { return true }
func (ListMcpResourcesTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("Optional server name to filter resources by."),
	})
}
func (ListMcpResourcesTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	targetServer, _ := in["server"].(string)

	// Get list of servers
	servers := runtime.MCP.Servers()
	if len(servers) == 0 {
		return tool.Result{Content: []any{}}, nil
	}

	// Filter by server if provided
	var serversToProcess []string
	if strings.TrimSpace(targetServer) != "" {
		// Check if server exists
		found := false
		for _, s := range servers {
			if s == targetServer {
				found = true
				break
			}
		}
		if !found {
			return tool.Result{}, fmt.Errorf("server %q not found. Available servers: %s", targetServer, strings.Join(servers, ", "))
		}
		serversToProcess = []string{targetServer}
	} else {
		serversToProcess = servers
	}

	// Collect resources from all servers
	var results []map[string]any
	for _, server := range serversToProcess {
		resources := runtime.MCP.ListResources(server)
		for _, r := range resources {
			results = append(results, map[string]any{
				"uri":         r.URI,
				"name":        r.Name,
				"mimeType":    r.MimeType,
				"description": r.Description,
				"server":      server,
			})
		}
	}

	return tool.Result{Content: results}, nil
}

// ReadMcpResourceTool reads a specific resource from an MCP server.
// Mirrors TS ReadMcpResourceTool - handles binary content by persisting to disk.
type ReadMcpResourceTool struct{}

func (ReadMcpResourceTool) Name() string { return "ReadMcpResourceTool" }
func (ReadMcpResourceTool) Description() string {
	return `Reads a specific resource from an MCP server.
- server: The name of the MCP server to read from
- uri: The URI of the resource to read

Usage examples:
- Read a resource from a server: readMcpResource({ server: "myserver", uri: "my-resource-uri" })`
}
func (ReadMcpResourceTool) IsReadOnly(tool.Input) bool { return true }
func (ReadMcpResourceTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("The MCP server name."),
		"uri":    tool.SchemaString("The resource URI to read."),
	}, "server", "uri")
}
func (ReadMcpResourceTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	serverName, _ := in["server"].(string)
	uri, _ := in["uri"].(string)

	if strings.TrimSpace(serverName) == "" {
		return tool.Result{}, fmt.Errorf("server is required")
	}
	if strings.TrimSpace(uri) == "" {
		return tool.Result{}, fmt.Errorf("uri is required")
	}

	// Check server exists
	servers := runtime.MCP.Servers()
	found := false
	for _, s := range servers {
		if s == serverName {
			found = true
			break
		}
	}
	if !found {
		return tool.Result{}, fmt.Errorf("server %q not found. Available servers: %s", serverName, strings.Join(servers, ", "))
	}

	resource, err := runtime.MCP.ReadResource(serverName, uri)
	if err != nil {
		return tool.Result{}, err
	}

	// Build result content
	contents := []map[string]any{
		{
			"uri":      resource.URI,
			"mimeType": resource.MimeType,
		},
	}

	// Handle content - check if it looks like base64-encoded binary data
	if resource.Content != "" {
		if decoded, err := base64.StdEncoding.DecodeString(resource.Content); err == nil && isBinaryContent(resource.MimeType) {
			// Persist binary content to disk
			filepath, err := persistBinaryContent(decoded, resource.MimeType)
			if err != nil {
				contents[0]["text"] = fmt.Sprintf("Binary content could not be saved to disk: %s", err.Error())
			} else {
				contents[0]["blobSavedTo"] = filepath
				contents[0]["text"] = fmt.Sprintf("Binary content saved to: %s (%d bytes, %s)", filepath, len(decoded), resource.MimeType)
			}
		} else {
			contents[0]["text"] = resource.Content
		}
	}

	return tool.Result{Content: map[string]any{"contents": contents}}, nil
}

// isBinaryContent checks if the MIME type indicates binary content
func isBinaryContent(mimeType string) bool {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	// Text-based MIME types
	textTypes := []string{
		"text/", "application/json", "application/xml", "application/javascript",
		"application/x-www-form-urlencoded", "application/xhtml+xml",
	}
	for _, t := range textTypes {
		if strings.HasPrefix(mimeType, t) {
			return false
		}
	}
	return mimeType != ""
}

// persistBinaryContent saves binary content to a temp file and returns the path
func persistBinaryContent(data []byte, mimeType string) (string, error) {
	// Create temp directory if needed
	tmpDir := filepath.Join(os.TempDir(), "claude-code-mcp")
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Generate filename with appropriate extension
	ext := extensionFromMimeType(mimeType)
	filename := fmt.Sprintf("mcp-resource-%d-%s%s", time.Now().UnixMilli(), randomString(6), ext)
	filepath := filepath.Join(tmpDir, filename)

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return filepath, nil
}

// extensionFromMimeType returns a file extension for a MIME type
func extensionFromMimeType(mimeType string) string {
	mimeType = strings.ToLower(strings.TrimSpace(mimeType))
	// Common MIME type to extension mappings
	extMap := map[string]string{
		"image/png":                ".png",
		"image/jpeg":               ".jpg",
		"image/gif":                ".gif",
		"image/webp":               ".webp",
		"image/svg+xml":            ".svg",
		"application/pdf":          ".pdf",
		"application/zip":          ".zip",
		"application/octet-stream": ".bin",
	}
	if ext, ok := extMap[mimeType]; ok {
		return ext
	}
	// Try to extract from MIME type
	if idx := strings.Index(mimeType, "/"); idx >= 0 {
		return "." + mimeType[idx+1:]
	}
	return ".bin"
}

// randomString generates a random alphanumeric string
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[(time.Now().Nanosecond()+i)%len(letters)]
	}
	return string(b)
}

// MCPTemplatesTool lists templates for one MCP server.
type MCPTemplatesTool struct{}

func (MCPTemplatesTool) Name() string          { return "mcp_list_templates" }
func (MCPTemplatesTool) Description() string   { return "list templates for one MCP server" }
func (MCPTemplatesTool) IsReadOnly(tool.Input) bool { return true }
func (MCPTemplatesTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("Configured MCP server name."),
	}, "server")
}
func (MCPTemplatesTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	server, _ := in["server"].(string)
	if strings.TrimSpace(server) == "" {
		return tool.Result{}, fmt.Errorf("server is required")
	}
	return tool.Result{Content: runtime.MCP.ListTemplates(server)}, nil
}

// MCPSearchTool searches deferred or dynamic MCP tools.
type MCPSearchTool struct{}

func (MCPSearchTool) Name() string         { return "tool_search" }
func (MCPSearchTool) Description() string  { return "search deferred or dynamic MCP tools" }
func (MCPSearchTool) IsReadOnly(tool.Input) bool { return true }
func (MCPSearchTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"query": tool.SchemaString("Search string used to filter available MCP tools."),
	})
}
func (MCPSearchTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	query, _ := in["query"].(string)
	return tool.Result{Content: runtime.MCP.SearchTools(query)}, nil
}

// MCPCallTool calls a tool on one MCP server.
type MCPCallTool struct{}

func (MCPCallTool) Name() string         { return "mcp_call_tool" }
func (MCPCallTool) Description() string  { return "call a tool on one MCP server" }
func (MCPCallTool) IsReadOnly(tool.Input) bool { return false }
func (MCPCallTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("Configured MCP server name."),
		"tool":   tool.SchemaString("Tool name exposed by the MCP server."),
		"args":   tool.SchemaAnyObject("Arguments to pass through to the MCP tool."),
	}, "server", "tool")
}
func (MCPCallTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	server, _ := in["server"].(string)
	toolName, _ := in["tool"].(string)
	if strings.TrimSpace(server) == "" || strings.TrimSpace(toolName) == "" {
		return tool.Result{}, fmt.Errorf("server and tool are required")
	}
	args := map[string]any{}
	if raw, ok := in["args"].(map[string]any); ok {
		args = raw
	}
	out, err := runtime.MCP.CallTool(server, toolName, args)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Content: out}, nil
}

// McpAuthToolOutput represents the output of the McpAuthTool.
// Mirrors TS McpAuthOutput structure.
type McpAuthToolOutput struct {
	Status  string `json:"status"`            // "auth_url" | "unsupported" | "error" | "success"
	Message string `json:"message"`           // Human-readable message
	AuthURL string `json:"authUrl,omitempty"` // OAuth URL if status is "auth_url"
}

// McpAuthTool authenticates with an MCP server.
// Mirrors TS McpAuthTool - provides OAuth flow for SSE/HTTP servers.
// The tool is dynamically created for servers that need authentication.
type McpAuthTool struct {
	serverName string
	serverURL  string
	transport  string
}

// CreateMcpAuthTool creates a new MCP auth tool for a specific server.
// This mirrors the TS createMcpAuthTool function.
func CreateMcpAuthTool(serverName string, serverURL string, transport string) *McpAuthTool {
	return &McpAuthTool{
		serverName: serverName,
		serverURL:  serverURL,
		transport:  transport,
	}
}

func (t *McpAuthTool) Name() string {
	return "mcp__" + sanitizeDynamicToolName(t.serverName) + "__authenticate"
}

func (t *McpAuthTool) Description() string {
	location := t.transport
	if t.serverURL != "" {
		location = t.transport + " at " + t.serverURL
	}
	return fmt.Sprintf(
		"The `%s` MCP server (%s) is installed but requires authentication. "+
			"Call this tool to start the OAuth flow — you'll receive an authorization URL to share with the user. "+
			"Once the user completes authorization in their browser, the server's real tools will become available automatically.",
		t.serverName, location)
}

func (t *McpAuthTool) IsReadOnly(tool.Input) bool { return false }

func (t *McpAuthTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{})
}

func (t *McpAuthTool) Call(_ context.Context, _ tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}

	// Check for unsupported transport types
	if t.transport == "claudeai-proxy" {
		return tool.Result{Content: McpAuthToolOutput{
			Status:  "unsupported",
			Message: fmt.Sprintf("This is a claude.ai MCP connector. Ask the user to run /mcp and select %q to authenticate.", t.serverName),
		}}, nil
	}

	// OAuth is only supported for sse and http transports
	if t.transport != "sse" && t.transport != "http" {
		return tool.Result{Content: McpAuthToolOutput{
			Status:  "unsupported",
			Message: fmt.Sprintf("Server %q uses %s transport which does not support OAuth from this tool. Ask the user to run /mcp and authenticate manually.", t.serverName, t.transport),
		}}, nil
	}

	// For now, return a message asking user to authenticate via /mcp
	// Full OAuth implementation would require additional infrastructure
	return tool.Result{Content: McpAuthToolOutput{
		Status:  "auth_url",
		Message: fmt.Sprintf("Ask the user to run /mcp and select %q to authenticate. OAuth flow from this tool is not yet implemented.", t.serverName),
	}}, nil
}

// sanitizeDynamicToolName sanitizes a string for use in a dynamic tool name.
func sanitizeDynamicToolName(value string) string {
	nonWordPattern := strings.NewReplacer(
		" ", "_",
		"-", "_",
		".", "_",
		"/", "_",
		":", "_",
	)
	sanitized := nonWordPattern.Replace(strings.TrimSpace(value))
	sanitized = strings.Trim(sanitized, "_")
	if sanitized == "" {
		return "tool"
	}
	return sanitized
}

// MCPAuthTool is a simple auth tool that accepts a token.
// This is the older-style auth tool that takes an explicit token.
type SimpleMCPAuthTool struct{}

func (SimpleMCPAuthTool) Name() string         { return "mcp_auth" }
func (SimpleMCPAuthTool) Description() string  { return "authenticate with an MCP server using a token" }
func (SimpleMCPAuthTool) IsReadOnly(tool.Input) bool { return false }
func (SimpleMCPAuthTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("Configured MCP server name."),
		"token":  tool.SchemaString("Authentication token or credential."),
	}, "server")
}
func (SimpleMCPAuthTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	server, _ := in["server"].(string)
	token, _ := in["token"].(string)
	if strings.TrimSpace(server) == "" {
		return tool.Result{}, fmt.Errorf("server is required")
	}
	if err := runtime.MCP.Authenticate(server, token); err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Content: "mcp authenticated", Meta: map[string]any{"server": server}}, nil
}

// MCPConnectTool connects to a configured MCP server.
type MCPConnectTool struct{}

func (MCPConnectTool) Name() string         { return "mcp_connect" }
func (MCPConnectTool) Description() string  { return "connect to a configured MCP server" }
func (MCPConnectTool) IsReadOnly(tool.Input) bool { return false }
func (MCPConnectTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("Configured MCP server name."),
	}, "server")
}
func (MCPConnectTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	server, _ := in["server"].(string)
	if strings.TrimSpace(server) == "" {
		return tool.Result{}, fmt.Errorf("server is required")
	}
	if err := runtime.MCP.Connect(server); err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Content: "mcp connected", Meta: map[string]any{"server": server}}, nil
}

// MCPDisconnectTool disconnects from a configured MCP server.
type MCPDisconnectTool struct{}

func (MCPDisconnectTool) Name() string         { return "mcp_disconnect" }
func (MCPDisconnectTool) Description() string  { return "disconnect from a configured MCP server" }
func (MCPDisconnectTool) IsReadOnly(tool.Input) bool { return false }
func (MCPDisconnectTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("Configured MCP server name."),
	}, "server")
}
func (MCPDisconnectTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	server, _ := in["server"].(string)
	if strings.TrimSpace(server) == "" {
		return tool.Result{}, fmt.Errorf("server is required")
	}
	if err := runtime.MCP.Disconnect(server); err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Content: "mcp disconnected", Meta: map[string]any{"server": server}}, nil
}

// MCPPingTool pings one configured MCP server.
type MCPPingTool struct{}

func (MCPPingTool) Name() string         { return "mcp_ping" }
func (MCPPingTool) Description() string  { return "ping one configured MCP server" }
func (MCPPingTool) IsReadOnly(tool.Input) bool { return true }
func (MCPPingTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"server": tool.SchemaString("Configured MCP server name."),
	}, "server")
}
func (MCPPingTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.MCP == nil {
		return tool.Result{}, fmt.Errorf("mcp runtime is not configured")
	}
	server, _ := in["server"].(string)
	if strings.TrimSpace(server) == "" {
		return tool.Result{}, fmt.Errorf("server is required")
	}
	status, err := runtime.MCP.Ping(server)
	if err != nil {
		return tool.Result{}, err
	}
	return tool.Result{Content: status, Meta: map[string]any{"server": server}}, nil
}

// RegisterMCPTools registers all MCP tools with the registry.
func RegisterMCPTools(r *tool.Registry) {
	// Core MCP management tools
	r.Register(MCPTool{})             // Stub for dynamic MCP tools
	r.Register(MCPServersTool{})      // List all servers
	r.Register(MCPToolsTool{})        // List tools for a server
	r.Register(ListMcpResourcesTool{}) // List resources (mirrors TS)
	r.Register(ReadMcpResourceTool{})  // Read resource (mirrors TS)
	r.Register(MCPTemplatesTool{})     // List templates
	r.Register(MCPSearchTool{})        // Search tools
	r.Register(MCPCallTool{})          // Call a tool
	r.Register(SimpleMCPAuthTool{})    // Auth with token
	r.Register(MCPConnectTool{})       // Connect to server
	r.Register(MCPDisconnectTool{})    // Disconnect from server
	r.Register(MCPPingTool{})          // Ping server
}
