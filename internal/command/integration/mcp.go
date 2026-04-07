package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"claude-code-go/internal/command"
)

func registerMCPCommands(r *command.Registry) {
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "permissions",
		Description: "show current permission mode and command policy status",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			lines := []string{
				"mode=workspace-write",
				"network=restricted",
				"destructive_actions=manual",
				"exec_tool=enabled",
				"agent_background=enabled",
			}
			if runtime.State != nil {
				state := runtime.State.Snapshot()
				lines = append(lines,
					fmt.Sprintf("cwd=%s", command.EmptyDash(state.CWD)),
					fmt.Sprintf("interactive=%t", state.IsInteractive),
				)
			}
			return strings.Join(lines, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "theme",
		Description: "show current terminal theme configuration",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			lines := []string{
				"name=claude-code-dark",
				"ui=full-screen-tui",
				"brand=orange",
				"message_cards=enabled",
				"markdown=enabled",
			}
			if runtime.Config.AppName != "" {
				lines = append(lines, "app="+runtime.Config.AppName)
			}
			return strings.Join(lines, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-mode",
		Description:  "set MCP service enabled state",
		ArgumentHint: "<on|off>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-mode", "<on|off>"))
			}
			if runtime.SetMCPEnabledAll == nil {
				return "", fmt.Errorf("mcp service mutation is not configured")
			}
			enabled, err := command.ParseToggle(args[0])
			if err != nil {
				return "", err
			}
			runtime.SetMCPEnabledAll(enabled)
			return fmt.Sprintf("mcp service updated\nenabled=%t", enabled), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-service-status",
		Description:  "set MCP service status",
		ArgumentHint: "<status>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-service-status", "<status>"))
			}
			if runtime.SetMCPServiceStatus == nil {
				return "", fmt.Errorf("mcp service mutation is not configured")
			}
			status := strings.Join(args, " ")
			runtime.SetMCPServiceStatus(status)
			return fmt.Sprintf("mcp service updated\nstatus=%s", status), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "mcp-service-show",
		Description: "show MCP service metadata",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.MCPStatus == nil {
				return "", fmt.Errorf("mcp service is not configured")
			}
			return runtime.MCPStatus(), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "mcp-summary",
		Description: "show MCP registry summary",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			servers := []command.MCPServerInfo(nil)
			if runtime.MCPServers != nil {
				servers = runtime.MCPServers()
			}
			return strings.Join([]string{
				"summary:",
				"registry=mcp",
				fmt.Sprintf("entries=%d", len(servers)),
				fmt.Sprintf("enabled_entries=%d", countEnabledMCPServers(servers)),
				fmt.Sprintf("disabled_entries=%d", len(servers)-countEnabledMCPServers(servers)),
				fmt.Sprintf("dev_entries=%d", countDevMCPServers(servers)),
			}, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "mcp-validate",
		Description: "validate MCP registry entries",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			servers := []command.MCPServerInfo(nil)
			if runtime.MCPServers != nil {
				servers = runtime.MCPServers()
			}
			return validateMCPServers(servers), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "mcp-json",
		Description: "show normalized MCP registry JSON",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			servers := []command.MCPServerInfo(nil)
			if runtime.MCPServers != nil {
				servers = runtime.MCPServers()
			}
			return marshalPanelJSON(map[string]any{
				"registry": "mcp",
				"servers":  servers,
			})
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "mcp-fields",
		Description: "show editable MCP server fields",
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return strings.Join([]string{
				"fields:",
				"- transport",
				"- status",
				"- description",
				"- url",
				"- auth",
				"- channel",
				"- command",
				"- enabled",
				"- tool_count",
				"- resource_count",
				"- dev",
			}, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "mcp-template",
		Description: "show a minimal MCP server template",
		Handler: func(_ context.Context, _ command.Runtime, _ []string) (string, error) {
			return strings.Join([]string{
				"template:",
				"name=my-mcp",
				"transport=sdk",
				"status=configured",
				"enabled=true",
				"channel=local",
			}, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocalJSX,
		Name:        "mcp",
		Description: "show MCP subsystem status",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			return renderMCPOverview(runtime), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "reload-mcp",
		Description: "reload MCP registry from config",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.ReloadMCP != nil {
				return runtime.ReloadMCP(), nil
			}
			return "mcp registry reloaded (placeholder)", nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:        command.KindLocal,
		Name:        "reset-mcp",
		Description: "reset MCP registry to defaults",
		Handler: func(_ context.Context, runtime command.Runtime, _ []string) (string, error) {
			if runtime.ResetMCP == nil {
				return "", fmt.Errorf("mcp reset is not configured")
			}
			return runtime.ResetMCP(), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-connect",
		Description:  "connect an MCP server",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-connect", "<name>"))
			}
			if runtime.ConnectMCP == nil || !runtime.ConnectMCP(args[0]) {
				return "", fmt.Errorf("mcp server not found: %s", args[0])
			}
			return fmt.Sprintf("mcp server connected\nname=%s", args[0]), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-disconnect",
		Description:  "disconnect an MCP server",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-disconnect", "<name>"))
			}
			if runtime.DisconnectMCP == nil || !runtime.DisconnectMCP(args[0]) {
				return "", fmt.Errorf("mcp server not found: %s", args[0])
			}
			return fmt.Sprintf("mcp server disconnected\nname=%s", args[0]), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-restart",
		Description:  "restart an MCP server connection",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-restart", "<name>"))
			}
			if runtime.RestartMCP == nil || !runtime.RestartMCP(args[0]) {
				return "", fmt.Errorf("mcp restart failed: %s", args[0])
			}
			return fmt.Sprintf("mcp server restarted\nname=%s", args[0]), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-auth",
		Description:  "authenticate an MCP server",
		ArgumentHint: "<name> <token>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-auth", "<name> <token>"))
			}
			if runtime.AuthenticateMCP == nil || !runtime.AuthenticateMCP(args[0], args[1]) {
				return "", fmt.Errorf("mcp auth failed: %s", args[0])
			}
			return fmt.Sprintf("mcp server authenticated\nname=%s", args[0]), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-ping",
		Description:  "ping an MCP server connection",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-ping", "<name>"))
			}
			if runtime.PingMCP == nil {
				return "", fmt.Errorf("mcp ping is not configured")
			}
			status, ok := runtime.PingMCP(args[0])
			if ok {
				return fmt.Sprintf("mcp ping ok\nname=%s\nstatus=%s", args[0], status), nil
			}
			return fmt.Sprintf("mcp ping requires attention\nname=%s\nstatus=%s", args[0], status), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-search",
		Description:  "search tools across MCP servers",
		ArgumentHint: "<query>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-search", "<query>"))
			}
			if runtime.MCPSearchTools == nil {
				return "", fmt.Errorf("mcp tool search is not configured")
			}
			matches := runtime.MCPSearchTools(strings.Join(args, " "))
			lines := []string{"matches:"}
			for _, item := range matches {
				line := "- " + item.Server + ":" + item.Name
				if item.ReadOnly {
					line += " read_only=true"
				}
				lines = append(lines, line)
				if strings.TrimSpace(item.Description) != "" {
					lines = append(lines, "  "+item.Description)
				}
			}
			if len(matches) == 0 {
				lines = append(lines, "result=empty")
			}
			return strings.Join(lines, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-tools",
		Description:  "list tools for one MCP server",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-tools", "<name>"))
			}
			if runtime.MCPTools == nil {
				return "", fmt.Errorf("mcp tool listing is not configured")
			}
			items := runtime.MCPTools(args[0])
			lines := []string{"tools:", "server=" + args[0]}
			for _, item := range items {
				line := "- " + item.Name
				if item.ReadOnly {
					line += " read_only=true"
				}
				lines = append(lines, line)
				if strings.TrimSpace(item.Description) != "" {
					lines = append(lines, "  "+item.Description)
				}
			}
			return strings.Join(lines, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-resources",
		Description:  "list resources for one MCP server",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-resources", "<name>"))
			}
			if runtime.MCPResources == nil {
				return "", fmt.Errorf("mcp resource listing is not configured")
			}
			items := runtime.MCPResources(args[0])
			lines := []string{"resources:", "server=" + args[0]}
			for _, item := range items {
				line := "- " + item.URI
				if strings.TrimSpace(item.Name) != "" {
					line += " name=" + item.Name
				}
				lines = append(lines, line)
				if strings.TrimSpace(item.Description) != "" {
					lines = append(lines, "  "+item.Description)
				}
			}
			return strings.Join(lines, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-templates",
		Description:  "list templates for one MCP server",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-templates", "<name>"))
			}
			if runtime.MCPTemplates == nil {
				return "", fmt.Errorf("mcp template listing is not configured")
			}
			items := runtime.MCPTemplates(args[0])
			lines := []string{"templates:", "server=" + args[0]}
			for _, item := range items {
				lines = append(lines, "- "+item.URI)
				if strings.TrimSpace(item.Description) != "" {
					lines = append(lines, "  "+item.Description)
				}
			}
			return strings.Join(lines, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-read",
		Description:  "read one MCP resource",
		ArgumentHint: "<server> <uri>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-read", "<server> <uri>"))
			}
			if runtime.MCPReadResource == nil {
				return "", fmt.Errorf("mcp resource read is not configured")
			}
			resource, err := runtime.MCPReadResource(args[0], args[1])
			if err != nil {
				return "", err
			}
			lines := []string{
				"resource:",
				"server=" + args[0],
				"uri=" + resource.URI,
			}
			if strings.TrimSpace(resource.MimeType) != "" {
				lines = append(lines, "mime_type="+resource.MimeType)
			}
			if strings.TrimSpace(resource.Content) != "" {
				lines = append(lines, "", resource.Content)
			}
			return strings.Join(lines, "\n"), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-call",
		Description:  "call an MCP tool",
		ArgumentHint: "<server> <tool> [key=value ...]",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-call", "<server> <tool> [key=value ...]"))
			}
			if runtime.MCPCallTool == nil {
				return "", fmt.Errorf("mcp tool call is not configured")
			}
			params := map[string]any{}
			for _, raw := range args[2:] {
				key, value, ok := strings.Cut(raw, "=")
				if !ok || strings.TrimSpace(key) == "" {
					continue
				}
				params[strings.TrimSpace(key)] = strings.TrimSpace(value)
			}
			out, err := runtime.MCPCallTool(args[0], args[1], params)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("tool_result:\nserver=%s\ntool=%s\n\n%s", args[0], args[1], out), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-enable",
		Description:  "set MCP server enabled state",
		ArgumentHint: "<name> <on|off>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-enable", "<name> <on|off>"))
			}
			if runtime.SetMCPEnabled == nil {
				return "", fmt.Errorf("mcp mutation is not configured")
			}
			enabled, err := command.ParseToggle(args[1])
			if err != nil {
				return "", err
			}
			if !runtime.SetMCPEnabled(args[0], enabled) {
				return "", fmt.Errorf("mcp server not found: %s", args[0])
			}
			return fmt.Sprintf("mcp server updated\nname=%s\nenabled=%t", args[0], enabled), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-status",
		Description:  "set MCP server status",
		ArgumentHint: "<name> <status>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-status", "<name> <status>"))
			}
			if runtime.SetMCPStatus == nil {
				return "", fmt.Errorf("mcp mutation is not configured")
			}
			status := strings.Join(args[1:], " ")
			if !runtime.SetMCPStatus(args[0], status) {
				return "", fmt.Errorf("mcp server not found: %s", args[0])
			}
			return fmt.Sprintf("mcp server updated\nname=%s\nstatus=%s", args[0], status), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-add",
		Description:  "add or replace an MCP server entry",
		ArgumentHint: "<name> <transport> [status]",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 2 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-add", "<name> <transport> [status]"))
			}
			if runtime.AddMCPServer == nil {
				return "", fmt.Errorf("mcp registry mutation is not configured")
			}
			status := "configured"
			if len(args) > 2 {
				status = strings.Join(args[2:], " ")
			}
			runtime.AddMCPServer(command.MCPServerInfo{
				Name:      args[0],
				Transport: args[1],
				Status:    status,
				Enabled:   true,
				Channel:   "local",
			})
			return fmt.Sprintf("mcp server saved\nname=%s\ntransport=%s\nstatus=%s", args[0], args[1], status), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-show",
		Description:  "show a single MCP server entry",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-show", "<name>"))
			}
			if runtime.MCPServers == nil {
				return "", fmt.Errorf("mcp registry is not configured")
			}
			server, ok := findMCPServer(runtime.MCPServers(), args[0])
			if !ok {
				return "", fmt.Errorf("mcp server not found: %s", args[0])
			}
			return renderMCPServer(server), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-set",
		Description:  "set a field on an MCP server entry",
		ArgumentHint: "<name> <field> <value>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) < 3 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-set", "<name> <field> <value>"))
			}
			if runtime.MCPServers == nil || runtime.AddMCPServer == nil {
				return "", fmt.Errorf("mcp registry mutation is not configured")
			}
			server, ok := findMCPServer(runtime.MCPServers(), args[0])
			if !ok {
				return "", fmt.Errorf("mcp server not found: %s", args[0])
			}
			updated, err := applyMCPField(server, args[1], strings.Join(args[2:], " "))
			if err != nil {
				return "", err
			}
			runtime.AddMCPServer(updated)
			return renderMCPServer(updated), nil
		},
	})
	r.RegisterLegacy(command.LegacyCommand{
		Type:         command.KindLocal,
		Name:         "mcp-remove",
		Description:  "remove an MCP server entry",
		ArgumentHint: "<name>",
		Handler: func(_ context.Context, runtime command.Runtime, args []string) (string, error) {
			if len(args) != 1 {
				return "", fmt.Errorf("%s", command.FormatCommandUsage("mcp-remove", "<name>"))
			}
			if runtime.RemoveMCPServer == nil {
				return "", fmt.Errorf("mcp registry mutation is not configured")
			}
			if !runtime.RemoveMCPServer(args[0]) {
				return "", fmt.Errorf("mcp server not found: %s", args[0])
			}
			return fmt.Sprintf("mcp server removed\nname=%s", args[0]), nil
		},
	})
}

// MCP helper functions

func renderMCPOverview(runtime command.Runtime) string {
	lines := []string{
		"overview:",
		"registry=mcp",
	}
	servers := []command.MCPServerInfo(nil)
	if runtime.MCPServers != nil {
		servers = runtime.MCPServers()
		lines = append(lines,
			fmt.Sprintf("entries=%d", len(servers)),
			fmt.Sprintf("enabled_entries=%d", countEnabledMCPServers(servers)),
			fmt.Sprintf("disabled_entries=%d", len(servers)-countEnabledMCPServers(servers)),
			fmt.Sprintf("dev_entries=%d", countDevMCPServers(servers)),
		)
	}
	lines = append(lines, "mutable_config=true")
	if runtime.MCPStatus != nil {
		lines = append(lines, "", runtime.MCPStatus())
	} else {
		lines = append(lines, "", "mcp=not_yet_ported")
	}
	if runtime.Tools != nil {
		lines = append(lines, fmt.Sprintf("local_tools=%d", len(runtime.Tools.List())))
	}
	if len(servers) > 0 {
		lines = append(lines, "", "entries:")
		for _, server := range servers {
			line := fmt.Sprintf("- %s [%s] %s tools=%d resources=%d", server.Name, server.Transport, server.Status, server.ToolCount, server.ResourceCount)
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
	lines = append(lines,
		"",
		"actions:",
		"- /mcp-mode <on|off>",
		"- /mcp-summary",
		"- /mcp-validate",
		"- /mcp-json",
		"- /mcp-service-show",
		"- /mcp-service-status <status>",
		"- /mcp-fields",
		"- /mcp-template",
		"- /mcp-add <name> <transport> [status]",
		"- /mcp-connect <name>",
		"- /mcp-disconnect <name>",
		"- /mcp-restart <name>",
		"- /mcp-auth <name> <token>",
		"- /mcp-ping <name>",
		"- /mcp-search <query>",
		"- /mcp-show <name>",
		"- /mcp-tools <name>",
		"- /mcp-resources <name>",
		"- /mcp-templates <name>",
		"- /mcp-read <server> <uri>",
		"- /mcp-call <server> <tool> [key=value ...]",
		"- /mcp-set <name> <field> <value>",
		"- /mcp-enable <name> <on|off>",
		"- /mcp-status <name> <status>",
		"- /mcp-remove <name>",
		"- /reset-mcp",
		"- /reload-mcp",
	)
	return strings.Join(lines, "\n")
}

func findMCPServer(servers []command.MCPServerInfo, name string) (command.MCPServerInfo, bool) {
	for _, server := range servers {
		if server.Name == name {
			return server, true
		}
	}
	return command.MCPServerInfo{}, false
}

func renderMCPServer(server command.MCPServerInfo) string {
	lines := []string{
		"name=" + server.Name,
		"transport=" + server.Transport,
		"status=" + server.Status,
		fmt.Sprintf("enabled=%t", server.Enabled),
		fmt.Sprintf("connected=%t", server.Connected),
		fmt.Sprintf("tool_count=%d", server.ToolCount),
		fmt.Sprintf("resource_count=%d", server.ResourceCount),
	}
	if strings.TrimSpace(server.Description) != "" {
		lines = append(lines, "description="+server.Description)
	}
	if strings.TrimSpace(server.URL) != "" {
		lines = append(lines, "url="+server.URL)
	}
	if strings.TrimSpace(server.Auth) != "" {
		lines = append(lines, "auth="+server.Auth)
	}
	if strings.TrimSpace(server.Channel) != "" {
		lines = append(lines, "channel="+server.Channel)
	}
	if strings.TrimSpace(server.Command) != "" {
		lines = append(lines, "command="+server.Command)
	}
	if strings.TrimSpace(server.LastConnected) != "" {
		lines = append(lines, "last_connected="+server.LastConnected)
	}
	if strings.TrimSpace(server.LastCalledAt) != "" {
		lines = append(lines, "last_called_at="+server.LastCalledAt)
	}
	if strings.TrimSpace(server.LastResult) != "" {
		lines = append(lines, "last_result="+server.LastResult)
	}
	if server.Dev {
		lines = append(lines, "dev=true")
	}
	return strings.Join(lines, "\n")
}

func applyMCPField(server command.MCPServerInfo, field, value string) (command.MCPServerInfo, error) {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "transport":
		server.Transport = value
	case "status":
		server.Status = value
	case "description":
		server.Description = value
	case "url":
		server.URL = value
	case "auth":
		server.Auth = value
	case "channel":
		server.Channel = value
	case "command":
		server.Command = value
	case "enabled":
		enabled, err := command.ParseToggle(value)
		if err != nil {
			return server, err
		}
		server.Enabled = enabled
	case "tool_count":
		count, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return server, fmt.Errorf("invalid tool_count: %s", value)
		}
		server.ToolCount = count
	case "resource_count":
		count, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil {
			return server, fmt.Errorf("invalid resource_count: %s", value)
		}
		server.ResourceCount = count
	case "dev":
		enabled, err := command.ParseToggle(value)
		if err != nil {
			return server, err
		}
		server.Dev = enabled
	default:
		return server, fmt.Errorf("unsupported mcp field: %s", field)
	}
	return server, nil
}

func countEnabledMCPServers(servers []command.MCPServerInfo) int {
	count := 0
	for _, server := range servers {
		if server.Enabled {
			count++
		}
	}
	return count
}

func countDevMCPServers(servers []command.MCPServerInfo) int {
	count := 0
	for _, server := range servers {
		if server.Dev {
			count++
		}
	}
	return count
}

func validateMCPServers(servers []command.MCPServerInfo) string {
	lines := []string{
		"validation:",
		"registry=mcp",
	}
	seen := map[string]int{}
	issues := 0
	for _, server := range servers {
		seen[server.Name]++
	}
	for _, server := range servers {
		prefix := "- " + server.Name + ": "
		switch {
		case strings.TrimSpace(server.Name) == "":
			lines = append(lines, prefix+"missing name")
			issues++
		case seen[server.Name] > 1:
			lines = append(lines, prefix+"duplicate name")
			issues++
		case strings.TrimSpace(server.Transport) == "":
			lines = append(lines, prefix+"missing transport")
			issues++
		case strings.TrimSpace(server.Status) == "":
			lines = append(lines, prefix+"missing status")
			issues++
		case server.ToolCount < 0 || server.ResourceCount < 0:
			lines = append(lines, prefix+"negative counters")
			issues++
		}
	}
	if issues == 0 {
		lines = append(lines, "result=ok", "issues=0")
		return strings.Join(lines, "\n")
	}
	lines = append(lines, fmt.Sprintf("result=issues"), fmt.Sprintf("issues=%d", issues))
	return strings.Join(lines, "\n")
}

func marshalPanelJSON(v any) (string, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "", err
	}
	return "```json\n" + string(data) + "\n```", nil
}