package services

import (
	"context"
	"fmt"
	"os"
	"strings"

	"claude-code-go/internal/agent"
	"claude-code-go/internal/api"
	"claude-code-go/internal/bootstrap"
	"claude-code-go/internal/bridge"
	"claude-code-go/internal/command"
	cmdagent "claude-code-go/internal/command/agent"
	cmddev "claude-code-go/internal/command/dev"
	cmdfiles "claude-code-go/internal/command/files"
	cmdintegration "claude-code-go/internal/command/integration"
	cmdmemory "claude-code-go/internal/command/memory"
	cmdmeta "claude-code-go/internal/command/meta"
	cmdprompt "claude-code-go/internal/command/prompt"
	cmdsession "claude-code-go/internal/command/session"
	cmdskills "claude-code-go/internal/command/skills"
	cmdstats "claude-code-go/internal/command/stats"
	"claude-code-go/internal/config"
	"claude-code-go/internal/engine"
	"claude-code-go/internal/infra/mcp"
	"claude-code-go/internal/prompt"
	"claude-code-go/internal/session"
	"claude-code-go/internal/task"
	"claude-code-go/internal/tool"
	agenttool "claude-code-go/internal/tool/agent"
	"claude-code-go/internal/tool/bash"
	configtool "claude-code-go/internal/tool/config"
	"claude-code-go/internal/tool/file"
	"claude-code-go/internal/tool/interaction"
	"claude-code-go/internal/tool/lsp"
	mcptool "claude-code-go/internal/tool/mcp"
	"claude-code-go/internal/tool/notebook"
	"claude-code-go/internal/tool/output"
	"claude-code-go/internal/tool/plan"
	"claude-code-go/internal/tool/repl"
	"claude-code-go/internal/tool/schedule"
	"claude-code-go/internal/tool/search"
	"claude-code-go/internal/tool/skill"
	"claude-code-go/internal/tool/sleep"
	tasktool "claude-code-go/internal/tool/task"
	"claude-code-go/internal/tool/team"
	"claude-code-go/internal/tool/todo"
	"claude-code-go/internal/tool/web"
	"claude-code-go/internal/tool/worktree"
)

type Container struct {
	cfg              config.Config
	state            *bootstrap.Store
	provider         *api.OpenAICompatibleClient
	sessions         *session.Manager
	tools            *tool.Registry
	engine           *engine.Engine
	agents           *agent.Manager
	commands         *command.Registry
	compact          *CompactService
	bridge           *bridge.Client
	agentSummary     *AgentSummaryService
	promptSuggestion *PromptSuggestionService
	sessionMemory    *SessionMemoryService
	toolUseSummary   *ToolUseSummaryService
	mcp              *MCPService
	plugins          *PluginsService
	skills           *SkillsService
	hooks            *HooksService
}

func Create(ctx context.Context, cfg config.Config, state *bootstrap.Store) (*Container, error) {
	sessionManager, err := session.CreateManager(cfg.SessionDir)
	if err != nil {
		return nil, err
	}

	tools := tool.EmptyRegistry()
	registerBuiltinTools(tools)
	hooks := CreateHooksService(cfg.HooksConfigPath)
	mcp := CreateMCPService(cfg.MCPConfigPath)
	plugins := CreatePluginsService(cfg.PluginsConfigPath)
	cwd, _ := os.Getwd()
	skills := CreateSkillsService(cwd)

	if cfg.SystemPrompt == "" {
		cfg.SystemPrompt = prompt.System(cfg)
	}

	provider := api.CreateOpenAICompatibleClient(cfg)
	mcpRuntime := createMCPToolRuntime(mcp)
	agentManager := agent.CreateManager(cfg, provider, tools, sessionManager, hooks, mcpRuntime)

	// Create TaskListManager for tracking in-progress tasks
	taskListManager := task.CreateTaskListManager(cfg.SessionDir, state.Snapshot().SessionID)

	commands := command.EmptyRegistry()
	command.RegisterBuiltins(commands)
	cmdagent.Register(commands)
	cmddev.Register(commands)
	cmdfiles.Register(commands)
	cmdintegration.Register(commands)
	cmdmemory.Register(commands)
	cmdmeta.Register(commands)
	cmdprompt.Register(commands)
	cmdsession.Register(commands)
	cmdskills.Register(commands)
	cmdstats.Register(commands)

	container := &Container{
		cfg:              cfg,
		state:            state,
		provider:         provider,
		sessions:         sessionManager,
		tools:            tools,
		agents:           agentManager,
		commands:         commands,
		compact:          EmptyCompactService(),
		bridge:           bridge.CreateClient(),
		agentSummary:     EmptyAgentSummaryService(),
		promptSuggestion: EmptyPromptSuggestionService(),
		sessionMemory:    CreateSessionMemoryService(),
		toolUseSummary:   EmptyToolUseSummaryService(),
		mcp:              mcp,
		plugins:          plugins,
		skills:           skills,
		hooks:            hooks,
	}
	container.syncExtensions()

	eng, err := engine.Create(ctx, engine.Options{
		Config:   cfg,
		Provider: provider,
		Tools:    tools,
		Hooks:    hooks,
		ToolRuntime: tool.Runtime{
			Store:    state,
			Tasks:    agentManager.Tasks(),
			TaskList: taskListManager,
			Stop:     agentManager.Stop,
			MCP:      mcpRuntime,
			SpawnAgent: func(ctx context.Context, req tool.AgentSpawnRequest) (*task.AgentTask, error) {
				result, err := agentManager.Spawn(ctx, agent.SpawnInput{
					Description:  req.Description,
					Prompt:       req.Prompt,
					SubagentType: req.Type,
					Background:   req.Background,
				})
				if err != nil {
					return nil, err
				}
				return result.Task, nil
			},
			ContinueAgent: func(ctx context.Context, taskID, prompt string, background bool) (*task.AgentTask, error) {
				result, err := agentManager.Continue(ctx, taskID, prompt, background)
				if err != nil {
					return nil, err
				}
				return result.Task, nil
			},
		},
		Sessions: sessionManager,
	})
	if err != nil {
		return nil, err
	}
	container.engine = eng

	if container.state != nil {
		container.state.SetCurrentModel(cfg.Model)
		container.state.SetSessionID(eng.SessionID())
	}
	return container, nil
}

func registerBuiltinTools(r *tool.Registry) {
	file.RegisterFileTools(r)
	search.RegisterSearchTools(r)
	search.RegisterToolSearchTools(r)
	bash.RegisterShellTools(r)
	bash.RegisterPowerShellTool(r)
	agenttool.RegisterAgentTools(r)
	tasktool.RegisterTaskTools(r)
	todo.RegisterTodoTools(r)
	interaction.RegisterInteractionTools(r)
	plan.RegisterPlanTools(r)
	worktree.RegisterWorktreeTools(r)
	lsp.RegisterLSPTools(r)
	notebook.RegisterNotebookTools(r)
	configtool.RegisterConfigTools(r)
	skill.RegisterSkillTools(r)
	team.RegisterTeamTools(r)
	sleep.RegisterSleepTools(r)
	output.RegisterOutputTools(r)
	repl.RegisterREPLTools(r)
	schedule.RegisterCronTools(r)
	schedule.RegisterRemoteTriggerTools(r)
	web.RegisterWebTools(r)
	mcptool.RegisterMCPTools(r)
}

func (c *Container) Config() config.Config                 { return c.cfg }
func (c *Container) State() *bootstrap.Store               { return c.state }
func (c *Container) Provider() *api.OpenAICompatibleClient { return c.provider }
func (c *Container) Sessions() *session.Manager            { return c.sessions }
func (c *Container) Tools() *tool.Registry                 { return c.tools }
func (c *Container) Engine() *engine.Engine                { return c.engine }
func (c *Container) Agents() *agent.Manager                { return c.agents }
func (c *Container) Commands() *command.Registry           { return c.commands }
func (c *Container) Compact() *CompactService              { return c.compact }
func (c *Container) Bridge() *bridge.Client                { return c.bridge }
func (c *Container) AgentSummary() *AgentSummaryService    { return c.agentSummary }
func (c *Container) PromptSuggestion() *PromptSuggestionService {
	return c.promptSuggestion
}
func (c *Container) SessionMemory() *SessionMemoryService { return c.sessionMemory }
func (c *Container) ToolUseSummary() *ToolUseSummaryService {
	return c.toolUseSummary
}
func (c *Container) MCP() *MCPService         { return c.mcp }
func (c *Container) Plugins() *PluginsService { return c.plugins }
func (c *Container) Skills() *SkillsService   { return c.skills }
func (c *Container) Hooks() *HooksService     { return c.hooks }
func (c *Container) SyncExtensions()          { c.syncExtensions() }

func (c *Container) ReloadPluginsRuntime() string {
	out := c.plugins.Reload()
	c.syncExtensions()
	return out
}

func (c *Container) ResetPluginsRuntime() string {
	out := c.plugins.Reset()
	c.syncExtensions()
	return out
}

func (c *Container) ReloadSkillsRuntime() string {
	c.skills.Reload(c.plugins.Skills())
	c.rebuildCommands()
	return fmt.Sprintf("skills reloaded\nregistered=%d", len(c.skills.List()))
}

func (c *Container) syncExtensions() {
	if c.skills != nil {
		c.skills.Reload(c.plugins.Skills())
	}
	if c.hooks != nil {
		c.hooks.LoadPluginHooks(c.plugins.PluginHooks())
	}
	if c.agents != nil {
		c.agents.ReloadDefinitions(c.plugins.Agents())
	}
	c.rebuildCommands()
}

func (c *Container) rebuildCommands() {
	r := command.EmptyRegistry()
	// Register all builtin commands from subdirectories
	cmdagent.Register(r)
	cmddev.Register(r)
	cmdfiles.Register(r)
	cmdintegration.Register(r)
	cmdmemory.Register(r)
	cmdmeta.Register(r)
	cmdprompt.Register(r)
	cmdsession.Register(r)
	cmdskills.Register(r)
	cmdstats.Register(r)
	// Register dynamic commands from skills and plugins
	if c.skills != nil {
		for _, cmd := range c.skills.Commands() {
			r.Register(cmd)
		}
	}
	if c.plugins != nil {
		for _, cmd := range c.plugins.Commands() {
			r.Register(cmd)
		}
	}
	c.commands = r
}

type mcpToolRuntime struct {
	service *MCPService
}

func createMCPToolRuntime(service *MCPService) tool.MCPRuntime {
	return &mcpToolRuntime{service: service}
}

func (m *mcpToolRuntime) Servers() []string {
	if m == nil || m.service == nil {
		return nil
	}
	servers := m.service.Servers()
	out := make([]string, 0, len(servers))
	for _, server := range servers {
		out = append(out, server.Name)
	}
	return out
}

func (m *mcpToolRuntime) ListTools(server string) []mcp.Tool {
	if m == nil || m.service == nil {
		return nil
	}
	return m.service.ListTools(server)
}

func (m *mcpToolRuntime) ListResources(server string) []mcp.Resource {
	if m == nil || m.service == nil {
		return nil
	}
	return m.service.ListResources(server)
}

func (m *mcpToolRuntime) ListTemplates(server string) []mcp.Template {
	if m == nil || m.service == nil {
		return nil
	}
	return m.service.ListTemplates(server)
}

func (m *mcpToolRuntime) SearchTools(query string) []mcp.ToolMatch {
	if m == nil || m.service == nil {
		return nil
	}
	return m.service.SearchTools(query)
}

func (m *mcpToolRuntime) DynamicTools() []tool.MCPDynamicToolInfo {
	if m == nil || m.service == nil {
		return nil
	}
	servers := m.service.Servers()
	out := make([]tool.MCPDynamicToolInfo, 0)
	for _, server := range servers {
		if !server.Enabled {
			continue
		}
		for _, item := range server.Tools {
			out = append(out, tool.MCPDynamicToolInfo{
				Name:        "mcp__" + sanitizeMCPRuntimeName(server.Name) + "__" + sanitizeMCPRuntimeName(item.Name),
				Server:      server.Name,
				Tool:        item.Name,
				Description: item.Description,
				ReadOnly:    item.ReadOnly,
			})
		}
	}
	return out
}

func (m *mcpToolRuntime) CallTool(server, toolName string, args map[string]any) (string, error) {
	if m == nil || m.service == nil {
		return "", nil
	}
	return m.service.CallTool(server, toolName, args)
}

func (m *mcpToolRuntime) ReadResource(server, uri string) (mcp.Resource, error) {
	if m == nil || m.service == nil {
		return mcp.Resource{}, nil
	}
	return m.service.ReadResource(server, uri)
}

func (m *mcpToolRuntime) Authenticate(server, token string) error {
	if m == nil || m.service == nil {
		return fmt.Errorf("mcp runtime is not configured")
	}
	if !m.service.Authenticate(server, token) {
		return fmt.Errorf("mcp authentication failed: %s", server)
	}
	return nil
}

func (m *mcpToolRuntime) Connect(server string) error {
	if m == nil || m.service == nil {
		return fmt.Errorf("mcp runtime is not configured")
	}
	if !m.service.Connect(server) {
		return fmt.Errorf("mcp connect failed: %s", server)
	}
	return nil
}

func (m *mcpToolRuntime) Disconnect(server string) error {
	if m == nil || m.service == nil {
		return fmt.Errorf("mcp runtime is not configured")
	}
	if !m.service.Disconnect(server) {
		return fmt.Errorf("mcp disconnect failed: %s", server)
	}
	return nil
}

func (m *mcpToolRuntime) Ping(server string) (string, error) {
	if m == nil || m.service == nil {
		return "", fmt.Errorf("mcp runtime is not configured")
	}
	status, ok := m.service.Ping(server)
	if !ok {
		return status, fmt.Errorf("mcp ping failed: %s", server)
	}
	return status, nil
}

func sanitizeMCPRuntimeName(value string) string {
	out := make([]rune, 0, len(value))
	for _, ch := range value {
		switch {
		case ch >= 'a' && ch <= 'z':
			out = append(out, ch)
		case ch >= 'A' && ch <= 'Z':
			out = append(out, ch)
		case ch >= '0' && ch <= '9':
			out = append(out, ch)
		default:
			out = append(out, '_')
		}
	}
	return strings.Trim(string(out), "_")
}
