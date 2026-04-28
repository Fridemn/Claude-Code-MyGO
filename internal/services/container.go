package services

import (
	"context"
	"fmt"
	"os"
	"strings"

	"claude-go/internal/agent"
	"claude-go/internal/api"
	"claude-go/internal/bootstrap"
	"claude-go/internal/bridge"
	"claude-go/internal/command"
	cmdaddir "claude-go/internal/command/addir"
	cmdagent "claude-go/internal/command/agent"
	cmdbtw "claude-go/internal/command/btw"
	cmdconfig "claude-go/internal/command/config"
	cmdcontext "claude-go/internal/command/context"
	cmddev "claude-go/internal/command/dev"
	cmdfast "claude-go/internal/command/fast"
	cmdfiles "claude-go/internal/command/files"
	cmdhelp "claude-go/internal/command/help"
	cmdhooks "claude-go/internal/command/hooks"
	cmdide "claude-go/internal/command/ide"
	cmdintegration "claude-go/internal/command/integration"
	cmdmcp "claude-go/internal/command/mcp"
	cmdmemory "claude-go/internal/command/memory"
	cmdmeta "claude-go/internal/command/meta"
	cmdmodel "claude-go/internal/command/model"
	cmdprompt "claude-go/internal/command/prompt"
	cmdsandbox "claude-go/internal/command/sandbox"
	cmdsession "claude-go/internal/command/session"
	cmdskills "claude-go/internal/command/skills"
	cmdstats "claude-go/internal/command/stats"
	"claude-go/internal/config"
	"claude-go/internal/engine"
	"claude-go/internal/infra/mcp"
	"claude-go/internal/prompt"
	"claude-go/internal/session"
	"claude-go/internal/task"
	"claude-go/internal/tool"
	agenttool "claude-go/internal/tool/agent"
	"claude-go/internal/tool/bash"
	configtool "claude-go/internal/tool/config"
	"claude-go/internal/tool/file"
	"claude-go/internal/tool/interaction"
	"claude-go/internal/tool/lsp"
	mcptool "claude-go/internal/tool/mcp"
	"claude-go/internal/tool/notebook"
	"claude-go/internal/tool/output"
	"claude-go/internal/tool/plan"
	"claude-go/internal/tool/repl"
	"claude-go/internal/tool/schedule"
	"claude-go/internal/tool/search"
	"claude-go/internal/tool/skill"
	"claude-go/internal/tool/sleep"
	tasktool "claude-go/internal/tool/task"
	"claude-go/internal/tool/team"
	"claude-go/internal/tool/todo"
	"claude-go/internal/tool/web"
	"claude-go/internal/tool/worktree"
)

type Container struct {
	cfg              config.Config
	state            *bootstrap.Store
	provider         *api.OpenAICompatibleClient
	sessions         *session.Manager
	transcripts      *session.EnhancedManager
	tools            *tool.Registry
	engine           *engine.Engine
	agents           *agent.Manager
	shellTasks       *task.ShellTaskManager
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

// engineCompactAdapter adapts services.CompactService to engine.CompactService interface
type engineCompactAdapter struct {
	svc *CompactService
}

func (a *engineCompactAdapter) Compact(ctx context.Context, messages []engine.CompactMessage, customInstructions string, isAutoCompact bool) (*engine.CompactResult, error) {
	// Convert engine.CompactMessage to services.CompactMessage
	svcMsgs := make([]CompactMessage, len(messages))
	for i, m := range messages {
		svcMsgs[i] = CompactMessage{
			UUID:    m.UUID,
			Type:    m.Type,
			Role:    m.Role,
			Content: m.Content,
		}
	}

	result, err := a.svc.Compact(ctx, svcMsgs, customInstructions, isAutoCompact)
	if err != nil {
		return nil, err
	}

	// Convert services.CompactionResult to engine.CompactResult
	engResult := &engine.CompactResult{
		PreCompactTokenCount:  result.PreCompactTokenCount,
		PostCompactTokenCount: result.PostCompactTokenCount,
	}

	// Convert summary messages
	for _, sm := range result.SummaryMessages {
		engResult.SummaryMessages = append(engResult.SummaryMessages, engine.CompactMessage{
			UUID:    sm.UUID,
			Type:    sm.Type,
			Role:    sm.Role,
			Content: sm.Content,
		})
	}

	return engResult, nil
}

func Create(ctx context.Context, cfg config.Config, state *bootstrap.Store, sessionID string) (*Container, error) {
	sessionManager, err := session.CreateManager(cfg.SessionDir)
	if err != nil {
		return nil, err
	}
	transcriptManager, err := session.CreateEnhancedManager(cfg.SessionDir)
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

	// Create ShellTaskManager for background shell tasks
	shellTaskManager := task.CreateShellTaskManager(cfg.SessionDir, state.Snapshot().SessionID)

	commands := command.EmptyRegistry()
	command.RegisterBuiltins(commands)
	cmdagent.Register(commands)
	cmdaddir.Register(commands)
	cmdbtw.Register(commands)
	cmdconfig.Register(commands)
	cmdcontext.Register(commands)
	cmddev.Register(commands)
	cmdfast.Register(commands)
	cmdfiles.Register(commands)
	cmdhelp.Register(commands)
	cmdhooks.Register(commands)
	cmdide.Register(commands)
	cmdintegration.Register(commands)
	cmdmcp.Register(commands)
	cmdmemory.Register(commands)
	cmdmeta.Register(commands)
	cmdmodel.Register(commands)
	cmdprompt.Register(commands)
	cmdsession.Register(commands)
	cmdskills.Register(commands)
	cmdsandbox.Register(commands)
	cmdstats.Register(commands)

	container := &Container{
		cfg:              cfg,
		state:            state,
		provider:         provider,
		sessions:         sessionManager,
		transcripts:      transcriptManager,
		tools:            tools,
		agents:           agentManager,
		shellTasks:       shellTaskManager,
		commands:         commands,
		compact:          CreateCompactService(provider, cfg.Model),
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
		Config:    cfg,
		Provider:  provider,
		Tools:     tools,
		Hooks:     hooks,
		SessionID: sessionID,
		ToolRuntime: tool.Runtime{
			Store:      state,
			Tasks:      agentManager.Tasks(),
			ShellTasks: shellTaskManager,
			TaskList:   taskListManager,
			Stop:       agentManager.Stop,
			MCP:        mcpRuntime,
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
		Sessions:       sessionManager,
		Transcripts:    transcriptManager,
		CompactService: &engineCompactAdapter{svc: container.compact},
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
	replModeEnabled := repl.IsModeEnabled()

	if replModeEnabled {
		// TS parity: when REPL mode is enabled, hide REPL primitive tools from
		// direct model use and keep them reachable through the REPL wrapper.
		r.Register(file.ListFilesTool{})
		r.Register(file.ListMcpResourcesTool{})
		r.Register(file.ReadMcpResourceTool{})
	} else {
		file.RegisterFileTools(r)
		search.RegisterSearchTools(r)
	}

	search.RegisterToolSearchTools(r)

	if !replModeEnabled {
		bash.RegisterShellTools(r)
	}
	bash.RegisterPowerShellTool(r)
	if replModeEnabled {
		// TS parity: Agent is REPL-only in REPL mode; keep communication tools.
		r.Register(agenttool.CreateSendMessageTool())
		r.Register(agenttool.CreateBriefTool())
	} else {
		agenttool.RegisterAgentTools(r)
	}
	tasktool.RegisterTaskTools(r)
	todo.RegisterTodoTools(r)
	interaction.RegisterInteractionTools(r)
	plan.RegisterPlanTools(r)
	worktree.RegisterWorktreeTools(r)
	lsp.RegisterLSPTools(r)
	if !replModeEnabled {
		notebook.RegisterNotebookTools(r)
	}
	configtool.RegisterConfigTools(r)
	skill.RegisterSkillTools(r)
	team.RegisterTeamTools(r)
	sleep.RegisterSleepTools(r)
	output.RegisterOutputTools(r)
	if replModeEnabled {
		repl.RegisterREPLTools(r)
	}
	schedule.RegisterCronTools(r)
	schedule.RegisterRemoteTriggerTools(r)
	web.RegisterWebTools(r)
	tool.RegisterWebFetchTool(r)
	mcptool.RegisterMCPTools(r)
}

func (c *Container) Config() config.Config                 { return c.cfg }
func (c *Container) State() *bootstrap.Store               { return c.state }
func (c *Container) Provider() *api.OpenAICompatibleClient { return c.provider }
func (c *Container) Sessions() *session.Manager            { return c.sessions }
func (c *Container) Transcripts() *session.EnhancedManager { return c.transcripts }
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
func (c *Container) MCP() *MCPService                   { return c.mcp }
func (c *Container) Plugins() *PluginsService           { return c.plugins }
func (c *Container) Skills() *SkillsService             { return c.skills }
func (c *Container) Hooks() *HooksService               { return c.hooks }
func (c *Container) ShellTasks() *task.ShellTaskManager { return c.shellTasks }
func (c *Container) SyncExtensions()                    { c.syncExtensions() }

func (c *Container) ApplyConfig(cfg config.Config) {
	c.cfg = cfg
	if c.provider != nil {
		c.provider.Configure(cfg)
	}
	if c.engine != nil {
		c.engine.SetConfig(cfg)
	}
	if c.compact != nil {
		c.compact.SetProvider(c.provider)
		c.compact.SetSummaryModel(firstNonEmpty(cfg.SummaryModel, cfg.Model))
	}
	if c.state != nil {
		c.state.SetCurrentModel(cfg.Model)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// SetAskPermissionHandler sets the permission handler for tool execution
// This is called by the runner after initialization to enable interactive permission requests
func (c *Container) SetAskPermissionHandler(handler func(ctx context.Context, toolName string, input tool.Input, message string) (bool, error)) {
	if c.engine != nil {
		runtime := c.engine.Runtime()
		runtime.AskPermission = handler
	}
}

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
	cmdaddir.Register(r)
	cmdbtw.Register(r)
	cmdconfig.Register(r)
	cmdcontext.Register(r)
	cmddev.Register(r)
	cmdfast.Register(r)
	cmdfiles.Register(r)
	cmdhelp.Register(r)
	cmdhooks.Register(r)
	cmdide.Register(r)
	cmdintegration.Register(r)
	cmdmcp.Register(r)
	cmdmemory.Register(r)
	cmdmeta.Register(r)
	cmdmodel.Register(r)
	cmdprompt.Register(r)
	cmdsession.Register(r)
	cmdskills.Register(r)
	cmdsandbox.Register(r)
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
