package mcp

import "encoding/json"

type Transport string

const (
	TransportStdio   Transport = "stdio"
	TransportSSE     Transport = "sse"
	TransportSSEIDE  Transport = "sse-ide"
	TransportHTTP    Transport = "http"
	TransportWS      Transport = "ws"
	TransportSDK     Transport = "sdk"
)

type ConnectionStatus string

const (
	StatusConnected    ConnectionStatus = "connected"
	StatusFailed       ConnectionStatus = "failed"
	StatusNeedsAuth    ConnectionStatus = "needs_auth"
	StatusPending      ConnectionStatus = "pending"
	StatusDisabled     ConnectionStatus = "disabled"
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusConfigured   ConnectionStatus = "configured"
)

// ToolAnnotations provides hints about tool behavior
type ToolAnnotations struct {
	Title           string `json:"title,omitempty"`
	ReadOnlyHint    bool   `json:"readOnlyHint,omitempty"`
	DestructiveHint bool   `json:"destructiveHint,omitempty"`
	IdempotentHint  bool   `json:"idempotentHint,omitempty"`
	OpenWorldHint   bool   `json:"openWorldHint,omitempty"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name         string           `json:"name"`
	Title        string           `json:"title,omitempty"`
	Description  string           `json:"description,omitempty"`
	InputSchema  json.RawMessage  `json:"inputSchema,omitempty"`
	Annotations  *ToolAnnotations `json:"annotations,omitempty"`
	ReadOnly     bool             `json:"read_only,omitempty"`
	Response     string           `json:"response,omitempty"` // Only for backward-compatible mock mode
}

// Resource represents an MCP resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
	Content     string `json:"content,omitempty"`
}

// Template represents an MCP resource template
type Template struct {
	URI         string `json:"uri"`
	Description string `json:"description,omitempty"`
}

// ToolMatch represents a search result for tools
type ToolMatch struct {
	Server      string `json:"server"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty"`
}

// ServerConfig represents the configuration for an MCP server
// Supports both standard MCP format and custom format for backward compatibility
type ServerConfig struct {
	// Identity
	Name        string    `json:"name"`
	Transport   Transport `json:"transport"`
	Description string    `json:"description,omitempty"`
	Enabled     bool      `json:"enabled"`

	// Stdio transport (standard MCP format)
	Command string            `json:"command,omitempty"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`

	// SSE/HTTP transport (standard MCP format)
	URL     string            `json:"url,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`

	// Legacy/custom fields
	Type        string `json:"type,omitempty"` // Alternative to Transport
	Auth        string `json:"auth,omitempty"`
	Channel     string `json:"channel,omitempty"`
	Dev         bool   `json:"dev,omitempty"`
	Status      string `json:"status,omitempty"`
	ToolCount   int    `json:"tool_count,omitempty"`
	ResourceCount int  `json:"resource_count,omitempty"`
	Connected   bool   `json:"connected,omitempty"`
	LastConnected string `json:"last_connected,omitempty"`
	LastCalledAt  string `json:"last_called_at,omitempty"`
	LastResult    string `json:"last_result,omitempty"`

	// Tools/Resources/Templates (populated after connection)
	Tools     []Tool     `json:"tools,omitempty"`
	Resources []Resource `json:"resources,omitempty"`
	Templates []Template `json:"templates,omitempty"`
}

func (s ServerConfig) GetTransport() Transport {
	if s.Transport != "" {
		return s.Transport
	}
	// Support "type" field as alternative
	switch s.Type {
	case "stdio":
		return TransportStdio
	case "sse":
		return TransportSSE
	case "http":
		return TransportHTTP
	case "ws":
		return TransportWS
	}
	// Infer from other fields
	if s.Command != "" {
		return TransportStdio
	}
	if s.URL != "" {
		return TransportSSE
	}
	return TransportSDK
}

// Connection represents a live MCP server connection
type Connection struct {
	Name          string           `json:"name"`
	Config        ServerConfig     `json:"config"`
	Status        ConnectionStatus `json:"status"`
	Connected     bool             `json:"connected"`
	NeedsApproval bool             `json:"needs_approval,omitempty"`
	LastError     string           `json:"last_error,omitempty"`
}

func (c Connection) GetName() string             { return c.Name }
func (c Connection) GetStatus() ConnectionStatus { return c.Status }
func (c Connection) GetTools() []Tool            { return append([]Tool(nil), c.Config.Tools...) }
func (c Connection) GetResources() []Resource {
	return append([]Resource(nil), c.Config.Resources...)
}
func (c Connection) GetTemplates() []Template {
	return append([]Template(nil), c.Config.Templates...)
}
