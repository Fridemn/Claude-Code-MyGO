package mcp

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"claude-code-go/internal/infra/mcp/transport"
)

type ConnectionManager struct {
	connections map[string]*Connection
	permissions ChannelPermissions
}

func CreateConnectionManager() *ConnectionManager {
	return &ConnectionManager{
		connections: map[string]*Connection{},
		permissions: DefaultChannelPermissions(),
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
	callbacks := CreateChannelPermissionCallbacks(strings.TrimSpace(conn.Config.Channel), &m.permissions)
	if !callbacks.Claim() {
		conn.Status = StatusPending
		conn.Connected = false
		conn.NeedsApproval = true
		conn.LastError = fmt.Sprintf("channel access denied: %s", conn.Config.Channel)
		return fmt.Errorf("mcp server requires channel approval: %s", name)
	}
	if strings.EqualFold(strings.TrimSpace(conn.Config.Auth), "required") {
		conn.Status = StatusNeedsAuth
		conn.Connected = false
		conn.NeedsApproval = false
		callbacks.Release()
		return fmt.Errorf("mcp server needs auth: %s", name)
	}
	if err := transport.Connect(transport.Config{
		Transport: string(conn.Config.Transport),
		Command:   conn.Config.Command,
		URL:       conn.Config.URL,
	}); err != nil {
		conn.Status = StatusFailed
		conn.Connected = false
		conn.NeedsApproval = false
		conn.LastError = err.Error()
		callbacks.Release()
		return err
	}
	conn.Status = StatusConnected
	conn.Connected = true
	conn.NeedsApproval = false
	conn.Config.LastConnected = time.Now().Format(time.RFC3339)
	conn.Config.ToolCount = len(conn.Config.Tools)
	conn.Config.ResourceCount = len(conn.Config.Resources)
	conn.Config.LastResult = "connected"
	conn.LastError = ""
	return nil
}

func (m *ConnectionManager) DisconnectServer(name string) error {
	conn, ok := m.connections[name]
	if !ok {
		return fmt.Errorf("mcp server not found: %s", name)
	}
	conn.Status = StatusDisconnected
	conn.Connected = false
	conn.NeedsApproval = false
	conn.Config.LastResult = "disconnected"
	CreateChannelPermissionCallbacks(strings.TrimSpace(conn.Config.Channel), &m.permissions).Release()
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
	for _, item := range conn.Config.Tools {
		if item.Name != toolName {
			continue
		}
		conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
		conn.Config.LastResult = "ok"
		return interpolateResponse(item.Response, args), nil
	}
	conn.Config.LastResult = "tool_not_found"
	return "", fmt.Errorf("mcp tool not found: %s", toolName)
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
	for _, item := range conn.Config.Resources {
		if item.URI != uri {
			continue
		}
		conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
		conn.Config.LastResult = "ok"
		return item, nil
	}
	for _, tmpl := range conn.Config.Templates {
		pattern := strings.ReplaceAll(tmpl.URI, "{name}", "*")
		if ok, _ := path.Match(pattern, uri); ok {
			result := HandleElicitation(context.Background(), ElicitRequest{
				Server: serverName,
				Params: map[string]string{
					"uri":      uri,
					"template": tmpl.URI,
				},
			})
			conn.Config.LastCalledAt = time.Now().Format(time.RFC3339)
			conn.Config.LastResult = result.Status
			return Resource{
				URI:         uri,
				Name:        path.Base(uri),
				Description: tmpl.Description,
				MimeType:    "text/plain",
				Content:     fmt.Sprintf("server=%s\ntemplate=%s\nuri=%s\nstatus=%s", serverName, tmpl.URI, uri, result.Status),
			}, nil
		}
	}
	conn.Config.LastResult = "resource_not_found"
	return Resource{}, fmt.Errorf("mcp resource not found: %s", uri)
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
