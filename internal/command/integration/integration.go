package integration

import (
	"claude-code-go/internal/command"
)

// registerIntegrationCommands registers all integration commands (MCP, Plugins, Hooks)
func registerIntegrationCommands(r *command.Registry) {
	registerMCPCommands(r)
	registerPluginsCommands(r)
	registerHooksCommands(r)
}