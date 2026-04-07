package services

import (
	"fmt"
	"strings"
	"time"

	mcpinfra "claude-code-go/internal/infra/mcp"
)

type MCPServer = mcpinfra.ServerConfig
type MCPTool = mcpinfra.Tool
type MCPResource = mcpinfra.Resource
type MCPTemplate = mcpinfra.Template
type MCPToolMatch = mcpinfra.ToolMatch

type MCPService struct {
	enabled       bool
	statusSummary string
	servers       []MCPServer
	path          string
	lastLoadedAt  time.Time
	lastLoadError string
	manager       *mcpinfra.ConnectionManager
	permissions   mcpinfra.ChannelPermissions
}

func CreateMCPService(path string) *MCPService {
	service := &MCPService{
		enabled:       false,
		statusSummary: "configured",
		path:          path,
		manager:       mcpinfra.CreateConnectionManager(),
		permissions:   mcpinfra.DefaultChannelPermissions(),
	}
	service.manager.SetPermissions(service.permissions)
	service.load(path)
	return service
}

func defaultMCPServers() []MCPServer {
	return []MCPServer{
		{
			Name:          "local-workspace",
			Transport:     mcpinfra.TransportSDK,
			Status:        "configured",
			ToolCount:     2,
			ResourceCount: 2,
			Description:   "Default local MCP workspace server.",
			Enabled:       true,
			Channel:       "local",
			Tools: []MCPTool{
				{Name: "workspace.describe", Description: "Describe the local workspace placeholder.", Response: "local workspace placeholder", ReadOnly: true},
				{Name: "workspace.echo", Description: "Echo one MCP tool argument.", Response: "{value}", ReadOnly: true},
			},
			Resources: []MCPResource{
				{URI: "mcp://local-workspace/readme", Name: "readme", Description: "Local MCP placeholder readme.", MimeType: "text/plain", Content: "local workspace placeholder resource"},
				{URI: "mcp://local-workspace/config", Name: "config", Description: "Local MCP placeholder config resource.", MimeType: "application/json", Content: "{\"runtime\":\"local\",\"kind\":\"placeholder\"}"},
			},
			Templates: []MCPTemplate{
				{URI: "mcp://local-workspace/{name}", Description: "Placeholder resource template."},
			},
		},
	}
}

func (s *MCPService) syncManager() {
	if s.manager == nil {
		s.manager = mcpinfra.CreateConnectionManager()
	}
	s.manager.SetPermissions(s.permissions)
	s.manager.SetServers(append([]MCPServer(nil), s.servers...))
}

func (s *MCPService) syncFromManager() {
	if s.manager == nil {
		return
	}
	s.servers = append([]MCPServer(nil), s.manager.Servers()...)
}

func (s *MCPService) RegisterServer(server MCPServer) {
	s.AddServer(server)
}

func (s *MCPService) AddServer(server MCPServer) {
	if strings.TrimSpace(server.Name) == "" {
		return
	}
	if server.Transport == "" {
		server.Transport = mcpinfra.TransportSDK
	}
	for i := range s.servers {
		if s.servers[i].Name != server.Name {
			continue
		}
		s.servers[i] = server
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.syncManager()
		s.persist()
		return
	}
	s.servers = append(s.servers, server)
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.syncManager()
	s.persist()
}

func (s *MCPService) RemoveServer(name string) bool {
	for i := range s.servers {
		if s.servers[i].Name != name {
			continue
		}
		s.servers = append(s.servers[:i], s.servers[i+1:]...)
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.syncManager()
		s.persist()
		return true
	}
	return false
}

func (s *MCPService) Servers() []MCPServer {
	s.syncFromManager()
	return append([]MCPServer(nil), s.servers...)
}

func (s *MCPService) Connect(name string) bool {
	err := s.manager.ConnectServer(name)
	s.syncFromManager()
	s.lastLoadedAt = time.Now()
	if err != nil {
		s.lastLoadError = err.Error()
		s.persist()
		return false
	}
	s.lastLoadError = ""
	s.statusSummary = "active"
	s.persist()
	return true
}

func (s *MCPService) Disconnect(name string) bool {
	err := s.manager.DisconnectServer(name)
	s.syncFromManager()
	s.lastLoadedAt = time.Now()
	if err != nil {
		s.lastLoadError = err.Error()
		s.persist()
		return false
	}
	s.lastLoadError = ""
	s.persist()
	return true
}

func (s *MCPService) Restart(name string) bool {
	if !s.Disconnect(name) {
		return false
	}
	return s.Connect(name)
}

func (s *MCPService) Authenticate(name, token string) bool {
	err := s.manager.Authenticate(name, token)
	s.syncFromManager()
	s.lastLoadedAt = time.Now()
	if err != nil {
		s.lastLoadError = err.Error()
		s.persist()
		return false
	}
	s.lastLoadError = ""
	s.statusSummary = "active"
	s.persist()
	return true
}

func (s *MCPService) Ping(name string) (string, bool) {
	status, err := s.manager.PingServer(name)
	s.syncFromManager()
	s.lastLoadedAt = time.Now()
	if err != nil {
		s.lastLoadError = err.Error()
		s.persist()
		return string(status), false
	}
	s.lastLoadError = ""
	s.persist()
	return string(status), true
}

func (s *MCPService) ListTools(name string) []MCPTool {
	return append([]MCPTool(nil), s.manager.ListTools(name)...)
}

func (s *MCPService) ListResources(name string) []MCPResource {
	return append([]MCPResource(nil), s.manager.ListResources(name)...)
}

func (s *MCPService) ListTemplates(name string) []MCPTemplate {
	return append([]MCPTemplate(nil), s.manager.ListTemplates(name)...)
}

func (s *MCPService) SearchTools(query string) []MCPToolMatch {
	return append([]MCPToolMatch(nil), s.manager.SearchTools(query)...)
}

func (s *MCPService) CallTool(serverName, toolName string, args map[string]any) (string, error) {
	out, err := s.manager.CallTool(serverName, toolName, args)
	s.syncFromManager()
	s.lastLoadedAt = time.Now()
	if err != nil {
		s.lastLoadError = err.Error()
		s.persist()
		return "", err
	}
	s.lastLoadError = ""
	s.persist()
	return out, nil
}

func (s *MCPService) ReadResource(serverName, uri string) (MCPResource, error) {
	resource, err := s.manager.ReadResource(serverName, uri)
	s.syncFromManager()
	s.lastLoadedAt = time.Now()
	if err != nil {
		s.lastLoadError = err.Error()
		s.persist()
		return MCPResource{}, err
	}
	s.lastLoadError = ""
	s.persist()
	return resource, nil
}

func (s *MCPService) Reload() string {
	s.enabled = false
	s.statusSummary = "configured"
	s.servers = nil
	s.lastLoadedAt = time.Time{}
	s.lastLoadError = ""
	s.load(s.path)
	return fmt.Sprintf("mcp registry reloaded\nconnections=%d", len(s.servers))
}

func (s *MCPService) Reset() string {
	s.enabled = false
	s.statusSummary = "configured"
	s.servers = defaultMCPServers()
	s.syncManager()
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.persist()
	return fmt.Sprintf("mcp registry reset\nconnections=%d", len(s.servers))
}

func (s *MCPService) SetEnabled(enabled bool) {
	s.enabled = enabled
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.persist()
}

func (s *MCPService) SetStatus(status string) {
	s.statusSummary = status
	s.lastLoadedAt = time.Now()
	s.lastLoadError = ""
	s.persist()
}

func (s *MCPService) SetServerEnabled(name string, enabled bool) bool {
	for i := range s.servers {
		if s.servers[i].Name != name {
			continue
		}
		s.servers[i].Enabled = enabled
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.syncManager()
		s.persist()
		return true
	}
	return false
}

func (s *MCPService) SetServerStatus(name, status string) bool {
	for i := range s.servers {
		if s.servers[i].Name != name {
			continue
		}
		s.servers[i].Status = status
		s.lastLoadedAt = time.Now()
		s.lastLoadError = ""
		s.syncManager()
		s.persist()
		return true
	}
	return false
}

func (s *MCPService) load(path string) {
	payload, err := mcpinfra.LoadConfig(path)
	s.lastLoadedAt = time.Now()
	if err != nil {
		s.lastLoadError = err.Error()
	}
	if err == nil && (len(payload.Servers) > 0 || payload.Status != "" || payload.Enabled) {
		s.enabled = payload.Enabled
		if payload.Status != "" {
			s.statusSummary = payload.Status
		}
		s.servers = append([]MCPServer(nil), payload.Servers...)
	}
	if len(s.servers) == 0 {
		s.servers = defaultMCPServers()
	}
	s.syncManager()
}

func (s *MCPService) persist() {
	s.syncFromManager()
	payload := mcpinfra.FileConfig{
		Enabled: s.enabled,
		Status:  s.statusSummary,
		Servers: append([]MCPServer(nil), s.servers...),
	}
	if err := mcpinfra.SaveConfig(s.path, payload); err != nil {
		s.lastLoadError = err.Error()
		return
	}
	s.lastLoadError = ""
}

func (s *MCPService) Status() string {
	lines := []string{
		fmt.Sprintf("mcp=%s", s.statusSummary),
		fmt.Sprintf("enabled=%t", s.enabled),
		fmt.Sprintf("connections=%d", len(s.Servers())),
		fmt.Sprintf("config_path=%s", s.path),
		fmt.Sprintf("last_loaded=%s", formatLoadTime(s.lastLoadedAt)),
	}
	if strings.TrimSpace(s.lastLoadError) != "" {
		lines = append(lines, "last_error="+s.lastLoadError)
	}
	totalResources := 0
	totalTools := 0
	for _, server := range s.servers {
		totalResources += max(server.ResourceCount, len(server.Resources))
		totalTools += max(server.ToolCount, len(server.Tools))
	}
	lines = append(lines,
		fmt.Sprintf("resources=%d", totalResources),
		fmt.Sprintf("tools=%d", totalTools),
	)
	if len(s.servers) > 0 {
		lines = append(lines, "", "servers:")
		for _, server := range s.servers {
			line := fmt.Sprintf("- %s [%s] %s tools=%d resources=%d", server.Name, server.Transport, server.Status, max(server.ToolCount, len(server.Tools)), max(server.ResourceCount, len(server.Resources)))
			if server.Enabled {
				line += " enabled=true"
			}
			if server.Connected {
				line += " connected=true"
			}
			lines = append(lines, line)
			meta := []string{}
			if strings.TrimSpace(server.Channel) != "" {
				meta = append(meta, "channel="+server.Channel)
			}
			if strings.TrimSpace(server.Auth) != "" {
				meta = append(meta, "auth="+server.Auth)
			}
			if server.Dev {
				meta = append(meta, "dev=true")
			}
			if strings.TrimSpace(server.URL) != "" {
				meta = append(meta, "url="+server.URL)
			}
			if len(meta) > 0 {
				lines = append(lines, "  "+strings.Join(meta, "  "))
			}
			runtimeMeta := []string{}
			if strings.TrimSpace(server.LastConnected) != "" {
				runtimeMeta = append(runtimeMeta, "last_connected="+server.LastConnected)
			}
			if strings.TrimSpace(server.LastCalledAt) != "" {
				runtimeMeta = append(runtimeMeta, "last_called_at="+server.LastCalledAt)
			}
			if strings.TrimSpace(server.LastResult) != "" {
				runtimeMeta = append(runtimeMeta, "last_result="+server.LastResult)
			}
			if len(runtimeMeta) > 0 {
				lines = append(lines, "  "+strings.Join(runtimeMeta, "  "))
			}
			if strings.TrimSpace(server.Command) != "" {
				lines = append(lines, "  command="+server.Command)
			}
			if strings.TrimSpace(server.Description) != "" {
				lines = append(lines, "  "+server.Description)
			}
		}
	}
	return strings.Join(lines, "\n")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
