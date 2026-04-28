package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	mcpsdk "github.com/mark3labs/mcp-go/mcp"

	"claude-go/internal/infra/mcp/transport"
)

type ConnectionManager struct {
	connections map[string]*Connection
	permissions ChannelPermissions
	clients     map[string]*transport.MCPConnection
}

func CreateConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: map[string]*Connection{},
		permissions: DefaultChannelPermissions(),
		clients:     map[string]*transport.MCPConnection{},
	}
}

func (m *ConnectionManager) SetPermissions(permissions ChannelPermissions) {
	m.permissions = permissions
	if m.permissions.Claimed == nil {
		m.permissions.Claimed = map[string]bool{}
	}
}

func (m *ConnectionManager) SetServers(servers []ServerConfig) {
	m.connections = map[string]*Connection{}
	for _, server := range servers {
		status := StatusConfigured
		if !server.Enabled {
			status = StatusDisabled
		} else if strings.TrimSpace(server.Status) != "" {
			status = ConnectionStatus(server.Status)
		}
		m.connections[server.Name] = &Connection{
			Name:      server.Name,
			Config:    server,
			Status:    status,
			Connected: server.Connected,
		}
	}
}

func (m *ConnectionManager) Servers() []ServerConfig {
	out := make([]ServerConfig, 0, len(m.connections))
	for _, conn := range m.connections {
		conn.Config.Status = string(conn.Status)
		conn.Config.Connected = conn.Connected
		out = append(out, conn.Config)
	}
	return out
}

func (m *ConnectionManager) ConnectServer(name string) error {
	conn, ok := m.connections[name]
	if !ok {
		return fmt.Errorf("mcp server not found: %s", name)
	}
	if !conn.Config.Enabled {
		conn.Status = StatusDisabled
		return fmt.Errorf("mcp server disabled: %s", name)
	}

	// Check channel permissions
	channel := strings.TrimSpace(conn.Config.Channel)
	if channel != "" && channel != "sdk" && channel != "local" {
		if m.permissions.Allowed != nil {
			if !m.permissions.Allowed[channel] {
				conn.Status = StatusFailed
				conn.Connected = false
				conn.LastError = "channel_blocked"
				return fmt.Errorf("channel %s is not allowed", channel)
			}
		}
	}

	// For "local" channel, check if it's blocked
	if channel == "local" && m.permissions.Allowed != nil {
		if blocked, exists := m.permissions.Allowed["local"]; exists && !blocked {
			conn.Status = StatusFailed
			conn.Connected = false
			conn.LastError = "channel_blocked"
			return fmt.Errorf("channel local is blocked")
		}
	}

	// Check auth requirement
	if conn.Config.Auth == "required" && !conn.Connected {
		conn.Status = StatusNeedsAuth
		conn.NeedsApproval = true
		return fmt.Errorf("mcp server requires authentication: %s", name)
	}

	// For SDK transport (mock/in-process), just mark as connected
	if conn.Config.Transport == TransportSDK || conn.Config.Transport == "" {
		conn.Status = StatusConnected
		conn.Connected = true
		conn.NeedsApproval = false
		conn.Config.LastConnected = time.Now().Format(time.RFC3339)
		conn.Config.LastResult = "connected"
		conn.LastError = ""
		return nil
	}

	// Create transport config for real transports
	tcfg := transport.Config{
		Transport: string(conn.Config.Transport),
		Command:   conn.Config.Command,
		Args:      conn.Config.Args,
		Env:       conn.Config.Env,
		URL:       conn.Config.URL,
		Headers:   conn.Config.Headers,
	}

	// Establish connection
	mcpConn, err := transport.Connect(tcfg)
	if err != nil {
		conn.Status = StatusFailed
		conn.Connected = false
		conn.LastError = err.Error()
		return err
	}

	// Store the client
	m.clients[name] = mcpConn

	// Discover tools and resources
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// List tools
	if mcpConn != nil && mcpConn.Client != nil {
		tools, err := mcpConn.Client.ListTools(ctx, mcpsdk.ListToolsRequest{})
		if err == nil && tools != nil {
			conn.Config.Tools = convertTools(tools.Tools)
			conn.Config.ToolCount = len(conn.Config.Tools)
		}

		// List resources
		resources, err := mcpConn.Client.ListResources(ctx, mcpsdk.ListResourcesRequest{})
		if err == nil && resources != nil {
			conn.Config.Resources = convertResources(resources.Resources)
			conn.Config.ResourceCount = len(conn.Config.Resources)
		}
	}

	conn.Status = StatusConnected
	conn.Connected = true
	conn.NeedsApproval = false
	conn.Config.LastConnected = time.Now().Format(time.RFC3339)
	conn.Config.LastResult = "connected"
	conn.LastError = ""
	return nil
}

func (m *ConnectionManager) DisconnectServer(name string) error {
	conn, ok := m.connections[name]
	if !ok {
		return fmt.Errorf("mcp server not found: %s", name)
	}

	// Close the client
	if client, ok := m.clients[name]; ok && client != nil && client.Client != nil {
		client.Client.Close()
	}
	delete(m.clients, name)

	conn.Status = StatusDisconnected
	conn.Connected = false
	conn.NeedsApproval = false
	conn.Config.LastResult = "disconnected"
	return nil
}

func (m *ConnectionManager) Authenticate(name, token string) error {
	conn, ok := m.connections[name]
	if !ok {
		return fmt.Errorf("mcp server not found: %s", name)
	}
	if strings.TrimSpace(token) == "" {
		conn.Status = StatusNeedsAuth
		conn.Config.LastResult = "auth_missing"
		return fmt.Errorf("missing auth token")
	}
	conn.Config.Auth = "authenticated"
	if err := m.ConnectServer(name); err != nil {
		return err
	}
	conn.Config.LastResult = "auth_ok"
	return nil
}

func (m *ConnectionManager) ListTools(server string) []Tool {
	conn, ok := m.connections[server]
	if !ok {
		return nil
	}
	return append([]Tool(nil), conn.Config.Tools...)
}

func (m *ConnectionManager) ListResources(server string) []Resource {
	conn, ok := m.connections[server]
	if !ok {
		return nil
	}
	return append([]Resource(nil), conn.Config.Resources...)
}

func (m *ConnectionManager) ListTemplates(server string) []Template {
	conn, ok := m.connections[server]
	if !ok {
		return nil
	}
	return append([]Template(nil), conn.Config.Templates...)
}

func (m *ConnectionManager) SearchTools(query string) []ToolMatch {
	query = strings.ToLower(strings.TrimSpace(query))
	var matches []ToolMatch
	for _, conn := range m.connections {
		if !conn.Config.Enabled {
			continue
		}
		for _, item := range conn.Config.Tools {
			haystack := strings.ToLower(conn.Name + " " + item.Name + " " + item.Description)
			if query != "" && !strings.Contains(haystack, query) {
				continue
			}
			matches = append(matches, ToolMatch{
				Server:      conn.Name,
				Name:        item.Name,
				Description: item.Description,
				ReadOnly:    item.ReadOnly,
			})
		}
	}
	return matches
}

func (m *ConnectionManager) CallTool(serverName, toolName string, args map[string]any) (string, error) {
	conn, ok := m.connections[serverName]
	if !ok {
		return "", fmt.Errorf("mcp server not found: %s", serverName)
	}
	if !conn.Connected {
		if err := m.ConnectServer(serverName); err != nil {
			return "", err
		}
	}

	// For SDK transport (mock), use interpolateResponse
	if conn.Config.Transport == TransportSDK || conn.Config.Transport == "" {
		// Find the tool in config and interpolate response
		for _, tool := range conn.Config.Tools {
			if tool.Name == toolName {
				conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
				conn.Config.LastResult = "ok"
				return interpolateResponse(tool.Response, args), nil
			}
		}
		return "", fmt.Errorf("tool not found: %s", toolName)
	}

	// Get the live client for real transports
	mcpConn, ok := m.clients[serverName]
	if mcpConn == nil || mcpConn.Client == nil {
		conn.Config.LastResult = "not_connected"
		return "", fmt.Errorf("mcp server not connected: %s", serverName)
	}

	// Make the real MCP call
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := mcpConn.Client.CallTool(ctx, mcpsdk.CallToolRequest{
		Params: mcpsdk.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	})
	if err != nil {
		conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
		conn.Config.LastResult = "error"
		return "", fmt.Errorf("mcp tool call failed: %w", err)
	}

	conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
	conn.Config.LastResult = "ok"

	// Convert result to string
	return formatCallToolResult(result), nil
}

func (m *ConnectionManager) ReadResource(serverName, uri string) (Resource, error) {
	conn, ok := m.connections[serverName]
	if !ok {
		return Resource{}, fmt.Errorf("mcp server not found: %s", serverName)
	}
	if !conn.Connected {
		if err := m.ConnectServer(serverName); err != nil {
			return Resource{}, err
		}
	}

	// For SDK transport (mock), find resource from config
	if conn.Config.Transport == TransportSDK || conn.Config.Transport == "" {
		// Check direct resources
		for _, r := range conn.Config.Resources {
			if r.URI == uri {
				conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
				conn.Config.LastResult = "ok"
				return r, nil
			}
		}
		// Check templates
		for _, tmpl := range conn.Config.Templates {
			// Simple template matching: {name} wildcard
			pattern := strings.ReplaceAll(tmpl.URI, "{name}", "*")
			if ok, _ := path.Match(pattern, uri); ok {
				conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
				conn.Config.LastResult = "ok"
				return Resource{
					URI:         uri,
					Name:        uri,
					Description: tmpl.Description,
					Content:     "templated resource content",
				}, nil
			}
		}
		conn.Config.LastResult = "resource_not_found"
		return Resource{}, fmt.Errorf("resource not found: %s", uri)
	}

	// Get the live client for real transports
	mcpConn, ok := m.clients[serverName]
	if mcpConn == nil || mcpConn.Client == nil {
		conn.Config.LastResult = "not_connected"
		return Resource{}, fmt.Errorf("mcp server not connected: %s", serverName)
	}

	// Make the real MCP call
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := mcpConn.Client.ReadResource(ctx, mcpsdk.ReadResourceRequest{
		Params: mcpsdk.ReadResourceParams{
			URI: uri,
		},
	})
	if err != nil {
		conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
		conn.Config.LastResult = "resource_not_found"
		return Resource{}, fmt.Errorf("mcp resource read failed: %w", err)
	}

	conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
	conn.Config.LastResult = "ok"

	// Convert result
	if result != nil && len(result.Contents) > 0 {
		content := result.Contents[0]
		mimeType := resourceMimeType(content)
		return Resource{
			URI:         uri,
			Name:        uri,
			MimeType:    mimeType,
			Description: "",
			Content:     formatResourceContent(content),
		}, nil
	}

	return Resource{URI: uri}, nil
}

func (m *ConnectionManager) PingServer(name string) (ConnectionStatus, error) {
	conn, ok := m.connections[name]
	if !ok {
		return StatusFailed, fmt.Errorf("mcp server not found: %s", name)
	}
	if !conn.Connected {
		if err := m.ConnectServer(name); err != nil {
			return conn.Status, err
		}
	}

	// For SDK transport, just return connected status
	if conn.Config.Transport == TransportSDK || conn.Config.Transport == "" {
		conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
		conn.Config.LastResult = "pong"
		return conn.Status, nil
	}

	// Ping the live client
	mcpConn, ok := m.clients[name]
	if mcpConn == nil || mcpConn.Client == nil {
		return conn.Status, fmt.Errorf("mcp server not connected: %s", name)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := mcpConn.Client.Ping(ctx); err != nil {
		conn.Config.LastResult = "ping_failed"
		return StatusFailed, err
	}

	conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
	conn.Config.LastResult = "pong"
	return conn.Status, nil
}

func (m *ConnectionManager) FindMatchingResources(serverName, target string) []Resource {
	conn, ok := m.connections[serverName]
	if !ok {
		return nil
	}
	var out []Resource
	for _, item := range conn.Config.Resources {
		if item.URI == target {
			out = append(out, item)
			continue
		}
		for _, tmpl := range conn.Config.Templates {
			if ok, _ := path.Match(strings.ReplaceAll(tmpl.URI, "{name}", "*"), target); ok {
				out = append(out, item)
				break
			}
		}
	}
	return out
}

// Helper functions

func convertTools(mcpTools []mcpsdk.Tool) []Tool {
	tools := make([]Tool, len(mcpTools))
	for i, t := range mcpTools {
		inputSchema, _ := json.Marshal(t.InputSchema)
		annotations := &ToolAnnotations{
			Title:           t.Annotations.Title,
			ReadOnlyHint:    boolPtr(t.Annotations.ReadOnlyHint),
			DestructiveHint: boolPtr(t.Annotations.DestructiveHint),
			IdempotentHint:  boolPtr(t.Annotations.IdempotentHint),
			OpenWorldHint:   boolPtr(t.Annotations.OpenWorldHint),
		}
		tools[i] = Tool{
			Name:        t.Name,
			Title:       t.Annotations.Title,
			Description: t.Description,
			InputSchema: inputSchema,
			Annotations: annotations,
			ReadOnly:    boolPtr(t.Annotations.ReadOnlyHint),
		}
	}
	return tools
}

func boolPtr(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func convertResources(mcpResources []mcpsdk.Resource) []Resource {
	resources := make([]Resource, len(mcpResources))
	for i, r := range mcpResources {
		resources[i] = Resource{
			URI:         r.URI,
			Name:        r.Name,
			Description: r.Description,
			MimeType:    r.MIMEType,
		}
	}
	return resources
}

func formatCallToolResult(result *mcpsdk.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return "ok"
	}
	var parts []string
	for _, content := range result.Content {
		switch c := content.(type) {
		case mcpsdk.TextContent:
			parts = append(parts, c.Text)
		case mcpsdk.ImageContent:
			parts = append(parts, "[image]")
		default:
			parts = append(parts, fmt.Sprintf("%v", content))
		}
	}
	return strings.Join(parts, "\n")
}

func formatResourceContent(content mcpsdk.ResourceContents) string {
	switch c := content.(type) {
	case mcpsdk.TextResourceContents:
		return c.Text
	case mcpsdk.BlobResourceContents:
		return "[binary data]"
	default:
		return fmt.Sprintf("%v", content)
	}
}

func resourceMimeType(content mcpsdk.ResourceContents) string {
	switch c := content.(type) {
	case mcpsdk.TextResourceContents:
		return c.MIMEType
	case mcpsdk.BlobResourceContents:
		return c.MIMEType
	default:
		return ""
	}
}

func interpolateResponse(template string, args map[string]any) string {
	response := strings.TrimSpace(template)
	if response == "" {
		response = "ok"
	}
	for key, value := range args {
		response = strings.ReplaceAll(response, "{"+key+"}", fmt.Sprint(value))
	}
	return response
}