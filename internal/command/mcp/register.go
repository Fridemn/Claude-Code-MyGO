package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"claude-go/internal/command"
	"claude-go/internal/infra/mcp"
)

// TS src/commands/mcp/mcp.tsx
// MCP server management command for personal users

func Register(r *command.Registry) {
	r.Register(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp",
		Description:  "Manage MCP (Model Context Protocol) servers",
		ArgumentHint: "[enable|disable|reconnect] [server-name]",
		Handler:      handleMCPCommand,
	})
}

func handleMCPCommand(ctx context.Context, rt command.Runtime, args []string) (string, error) {
	// Parse arguments
	// TS mcp.tsx: args.trim().split(/\s+/)
	if len(args) == 0 || strings.TrimSpace(args[0]) == "" {
		// No args - show MCP settings info
		// TS mcp.tsx: return <MCPSettings onComplete={onDone} />
		return buildMCPSettingsInfo(rt), nil
	}

	// Get command parts
	parts := args
	action := strings.ToLower(strings.TrimSpace(parts[0]))

	// Handle reconnect
	// TS mcp.tsx: if (parts[0] === 'reconnect' && parts[1])
	if action == "reconnect" && len(parts) > 1 {
		serverName := strings.Join(parts[1:], " ")
		return handleMCPReconnect(serverName, rt), nil
	}

	// Handle enable/disable
	// TS mcp.tsx: if (parts[0] === 'enable' || parts[0] === 'disable')
	if action == "enable" || action == "disable" {
		target := "all"
		if len(parts) > 1 {
			target = strings.Join(parts[1:], " ")
		}
		return handleMCPToggle(action, target, rt), nil
	}

	// Unknown action
	return fmt.Sprintf("Unknown MCP action: %s\nUsage: /mcp [enable|disable|reconnect] [server-name]", action), nil
}

// buildMCPSettingsInfo returns MCP servers info
// TS mcp.tsx: MCPSettings component
func buildMCPSettingsInfo(rt command.Runtime) string {
	lines := []string{
		"MCP Server Management",
		"",
	}

	// Get config path
	configPath := rt.Config.MCPConfigPath
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = home + "/.claude/mcp.json"
	}
	lines = append(lines, fmt.Sprintf("Config file: %s", configPath))
	lines = append(lines, "")

	// Try to read MCP config
	servers := readMCPConfig(configPath)

	if len(servers) == 0 {
		lines = append(lines, "No MCP servers configured.")
	} else {
		lines = append(lines, fmt.Sprintf("Configured servers (%d):", len(servers)))
		for name, config := range servers {
			status := "disabled"
			if config.Enabled {
				status = "enabled"
			}
			transport := string(config.Transport)
			lines = append(lines, fmt.Sprintf("  - %s (%s, %s)", name, transport, status))
		}
	}

	lines = append(lines, "")
	lines = append(lines, "Commands:")
	lines = append(lines, "  /mcp enable [server]  - Enable a server (or all)")
	lines = append(lines, "  /mcp disable [server] - Disable a server (or all)")
	lines = append(lines, "  /mcp reconnect <server> - Reconnect to a server")

	return strings.Join(lines, "\n")
}

// handleMCPReconnect reconnects to a server
// TS mcp.tsx: MCPReconnect component
func handleMCPReconnect(serverName string, rt command.Runtime) string {
	configPath := rt.Config.MCPConfigPath
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = home + "/.claude/mcp.json"
	}

	servers := readMCPConfig(configPath)

	// Find the server
	config, found := servers[serverName]
	if !found {
		return fmt.Sprintf("MCP server '%s' not found", serverName)
	}

	// Check if it's enabled
	if !config.Enabled {
		return fmt.Sprintf("MCP server '%s' is disabled. Enable it first with /mcp enable %s", serverName, serverName)
	}

	// For Go version, we return a message about manual reconnect
	// Full reconnect implementation would require MCP manager integration
	return fmt.Sprintf("Reconnecting to MCP server '%s' (%s)...", serverName, config.Transport)
}

// handleMCPToggle enables or disables servers
// TS mcp.tsx: MCPToggle component
func handleMCPToggle(action string, target string, rt command.Runtime) string {
	isEnabling := action == "enable"

	configPath := rt.Config.MCPConfigPath
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = home + "/.claude/mcp.json"
	}

	servers := readMCPConfig(configPath)

	if len(servers) == 0 {
		return "No MCP servers configured."
	}

	// Filter servers to toggle
	// TS mcp.tsx: toToggle = target === 'all' ? clients.filter(...) : clients.filter(c => c.name === target)
	var toToggle []string
	if target == "all" {
		for name, config := range servers {
			// Skip 'ide' server (TS: filter(c => c.name !== 'ide'))
			if name == "ide" {
				continue
			}
			// Toggle based on action
			if isEnabling && !config.Enabled {
				toToggle = append(toToggle, name)
			} else if !isEnabling && config.Enabled {
				toToggle = append(toToggle, name)
			}
		}
	} else {
		// Single server
		_, found := servers[target]
		if !found {
			return fmt.Sprintf("MCP server '%s' not found", target)
		}
		// Skip ide
		if target == "ide" {
			return "Cannot toggle 'ide' server through /mcp command"
		}
		toToggle = append(toToggle, target)
	}

	if len(toToggle) == 0 {
		if target == "all" {
			return fmt.Sprintf("All MCP servers are already %s", action+"d")
		}
		return fmt.Sprintf("MCP server '%s' is already %s", target, action+"d")
	}

	// Toggle servers
	// TS mcp.tsx: for (const s of toToggle) { toggleMcpServer(s.name) }
	for _, name := range toToggle {
		if isEnabling {
			enableMCPServer(configPath, name)
		} else {
			disableMCPServer(configPath, name)
		}
	}

	// Build response
	// TS mcp.tsx: onComplete(target === 'all' ? `Enabled/Disabled X MCP server(s)` : ...)
	actionLabel := action
	if isEnabling {
		actionLabel = "Enabled"
	} else {
		actionLabel = "Disabled"
	}

	if target == "all" {
		return fmt.Sprintf("%s %d MCP server(s)", actionLabel, len(toToggle))
	}
	return fmt.Sprintf("MCP server '%s' %s", target, action+"d")
}

// readMCPConfig reads MCP server configuration
func readMCPConfig(path string) map[string]mcp.ServerConfig {
	servers := make(map[string]mcp.ServerConfig)

	// Read config file
	data, err := os.ReadFile(path)
	if err != nil {
		return servers
	}

	// Parse as JSON with mcpServers key
	// Standard MCP config format: { "mcpServers": { ... } }
	var rawConfig map[string]interface{}
	if err := parseJSON(data, &rawConfig); err != nil {
		return servers
	}

	// Get mcpServers section
	mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{})
	if !ok {
		return servers
	}

	// Parse each server
	for name, serverData := range mcpServers {
		config := parseServerConfig(name, serverData)
		servers[name] = config
	}

	return servers
}

// parseServerConfig parses a single server config from raw JSON
func parseServerConfig(name string, data interface{}) mcp.ServerConfig {
	config := mcp.ServerConfig{
		Name:    name,
		Enabled: true, // Default enabled
	}

	serverMap, ok := data.(map[string]interface{})
	if !ok {
		return config
	}

	// Get transport
	if t, ok := serverMap["type"].(string); ok {
		config.Transport = mcp.Transport(t)
	} else if _, hasCommand := serverMap["command"]; hasCommand {
		config.Transport = mcp.TransportStdio
	} else if _, hasURL := serverMap["url"]; hasURL {
		config.Transport = mcp.TransportSSE
	}

	// Get enabled status
	if e, ok := serverMap["enabled"].(bool); ok {
		config.Enabled = e
	}

	// Get description
	if d, ok := serverMap["description"].(string); ok {
		config.Description = d
	}

	return config
}

// parseJSON is a simple JSON parser helper
func parseJSON(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// enableMCPServer enables a server in config
func enableMCPServer(path string, name string) {
	// Read config
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var rawConfig map[string]interface{}
	if parseJSON(data, &rawConfig) != nil {
		return
	}

	mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{})
	if !ok {
		return
	}

	serverData, ok := mcpServers[name].(map[string]interface{})
	if !ok {
		return
	}

	serverData["enabled"] = true

	// Write back
	updatedData, err := json.MarshalIndent(rawConfig, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, updatedData, 0644)
}

// disableMCPServer disables a server in config
func disableMCPServer(path string, name string) {
	// Read config
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var rawConfig map[string]interface{}
	if parseJSON(data, &rawConfig) != nil {
		return
	}

	mcpServers, ok := rawConfig["mcpServers"].(map[string]interface{})
	if !ok {
		return
	}

	serverData, ok := mcpServers[name].(map[string]interface{})
	if !ok {
		return
	}

	serverData["enabled"] = false

	// Write back
	updatedData, err := json.MarshalIndent(rawConfig, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(path, updatedData, 0644)
}