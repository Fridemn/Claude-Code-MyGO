package mcp

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// FileConfig represents the custom Go format (backward compatible)
type FileConfig struct {
	Enabled bool           `json:"enabled"`
	Status  string         `json:"status"`
	Servers []ServerConfig `json:"servers"`
}

// StandardMCPConfig represents the standard .mcp.json format
// {
//   "mcpServers": {
//     "server-name": { "command": "...", "args": [...], "env": {...} },
//     "remote-server": { "type": "sse", "url": "...", "headers": {...} }
//   }
// }
type StandardMCPConfig struct {
	MCPServers map[string]json.RawMessage `json:"mcpServers"`
}

// StdioServerConfig represents stdio server in standard MCP format
type StdioServerConfig struct {
	Type        string            `json:"type,omitempty"` // "stdio" - optional for backward compat
	Command     string            `json:"command"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	Description string            `json:"description,omitempty"`
}

// SSEServerConfig represents SSE server in standard MCP format
type SSEServerConfig struct {
	Type     string            `json:"type"` // "sse"
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers,omitempty"`
	Description string        `json:"description,omitempty"`
}

// HTTP server config
type HTTPServerConfig struct {
	Type     string            `json:"type"` // "http"
	URL      string            `json:"url"`
	Headers  map[string]string `json:"headers,omitempty"`
}

// SDK server config
type SDKServerConfig struct {
	Type string `json:"type"` // "sdk"
	Name string `json:"name"`
}

// LoadConfig loads MCP configuration from a file.
// Supports both standard "mcpServers" format and custom format.
func LoadConfig(path string) (FileConfig, error) {
	var cfg FileConfig
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if len(data) == 0 {
		return cfg, nil
	}

	// Try parsing as standard MCP format first
	var standardCfg StandardMCPConfig
	if err := json.Unmarshal(data, &standardCfg); err == nil && len(standardCfg.MCPServers) > 0 {
		// Convert standard format to internal format
		cfg.Enabled = true
		cfg.Status = "configured"
		for name, rawConfig := range standardCfg.MCPServers {
			server, err := parseServerConfig(name, rawConfig)
			if err != nil {
				continue // Skip invalid servers
			}
			cfg.Servers = append(cfg.Servers, server)
		}
		return cfg, nil
	}

	// Fall back to custom format
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// parseServerConfig parses a single server config from standard format
func parseServerConfig(name string, rawConfig json.RawMessage) (ServerConfig, error) {
	// First, try to detect the type
	var typeOnly struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(rawConfig, &typeOnly); err == nil {
		switch typeOnly.Type {
		case "sse":
			var sse SSEServerConfig
			if err := json.Unmarshal(rawConfig, &sse); err != nil {
				return ServerConfig{}, err
			}
			return ServerConfig{
				Name:        name,
				Transport:   TransportSSE,
				URL:         sse.URL,
				Headers:     sse.Headers,
				Description: sse.Description,
				Enabled:     true,
				Status:      "configured",
			}, nil
		case "http":
			var http HTTPServerConfig
			if err := json.Unmarshal(rawConfig, &http); err != nil {
				return ServerConfig{}, err
			}
			return ServerConfig{
				Name:      name,
				Transport: TransportHTTP,
				URL:       http.URL,
				Headers:   http.Headers,
				Enabled:   true,
				Status:    "configured",
			}, nil
		case "sdk":
			var sdk SDKServerConfig
			if err := json.Unmarshal(rawConfig, &sdk); err != nil {
				return ServerConfig{}, err
			}
			return ServerConfig{
				Name:        name,
				Transport:   TransportSDK,
				Description: sdk.Name,
				Enabled:     true,
				Status:      "configured",
			}, nil
		}
	}

	// Default to stdio
	var stdio StdioServerConfig
	if err := json.Unmarshal(rawConfig, &stdio); err != nil {
		return ServerConfig{}, err
	}
	if stdio.Command == "" {
		return ServerConfig{}, errors.New("empty command")
	}

	return ServerConfig{
		Name:        name,
		Transport:   TransportStdio,
		Command:    stdio.Command,
		Args:        stdio.Args,
		Env:         stdio.Env,
		Description: stdio.Description,
		Enabled:     true,
		Status:      "configured",
	}, nil
}

// SaveConfig saves MCP configuration to a file.
// Saves in standard "mcpServers" format for interoperability.
func SaveConfig(path string, cfg FileConfig) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	// Convert to standard format
	standardCfg := StandardMCPConfig{
		MCPServers: make(map[string]json.RawMessage),
	}

	for _, server := range cfg.Servers {
		var rawConfig json.RawMessage
		var err error

		switch server.Transport {
		case TransportSSE, TransportSSEIDE:
			sseCfg := SSEServerConfig{
				Type:        "sse",
				URL:         server.URL,
				Headers:     server.Headers,
				Description: server.Description,
			}
			rawConfig, err = json.Marshal(sseCfg)
		case TransportHTTP:
			httpCfg := HTTPServerConfig{
				Type:        "http",
				URL:         server.URL,
				Headers:     server.Headers,
			}
			rawConfig, err = json.Marshal(httpCfg)
		case TransportSDK:
			sdkCfg := SDKServerConfig{
				Type: "sdk",
				Name: server.Description,
			}
			rawConfig, err = json.Marshal(sdkCfg)
		default:
			// Default to stdio
			stdioCfg := StdioServerConfig{
				Type:        "stdio",
				Command:     server.Command,
				Args:        server.Args,
				Env:         server.Env,
				Description: server.Description,
			}
			rawConfig, err = json.Marshal(stdioCfg)
		}

		if err != nil {
			return err
		}
		standardCfg.MCPServers[server.Name] = rawConfig
	}

	data, err := json.MarshalIndent(standardCfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
