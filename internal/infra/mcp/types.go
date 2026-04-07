package mcp

type Transport string

const (
	TransportStdio Transport = "stdio"
	TransportSSE   Transport = "sse"
	TransportSSEIDE Transport = "sse-ide"
	TransportHTTP  Transport = "http"
	TransportWS    Transport = "ws"
	TransportSDK   Transport = "sdk"
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

type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Response    string `json:"response,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty"`
}

type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
	Content     string `json:"content,omitempty"`
}

type Template struct {
	URI         string `json:"uri"`
	Description string `json:"description,omitempty"`
}

type ToolMatch struct {
	Server      string `json:"server"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	ReadOnly    bool   `json:"read_only,omitempty"`
}

type ServerConfig struct {
	Name          string     `json:"name"`
	Transport     Transport  `json:"transport"`
	Status        string     `json:"status"`
	ToolCount     int        `json:"tool_count"`
	ResourceCount int        `json:"resource_count"`
	Description   string     `json:"description,omitempty"`
	Enabled       bool       `json:"enabled"`
	URL           string     `json:"url,omitempty"`
	Auth          string     `json:"auth,omitempty"`
	Channel       string     `json:"channel,omitempty"`
	Dev           bool       `json:"dev,omitempty"`
	Command       string     `json:"command,omitempty"`
	Connected     bool       `json:"connected,omitempty"`
	LastConnected string     `json:"last_connected,omitempty"`
	LastCalledAt  string     `json:"last_called_at,omitempty"`
	LastResult    string     `json:"last_result,omitempty"`
	Tools         []Tool     `json:"tools,omitempty"`
	Resources     []Resource `json:"resources,omitempty"`
	Templates     []Template `json:"templates,omitempty"`
}

func (s ServerConfig) GetTransport() Transport { return s.Transport }

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
