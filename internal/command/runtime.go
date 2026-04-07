package command

import (
	"claude-code-go/internal/agent"
	"claude-code-go/internal/bootstrap"
	"claude-code-go/internal/config"
	"claude-code-go/internal/engine"
	"claude-code-go/internal/task"
	"claude-code-go/internal/tool"
)

// Runtime provides context and dependencies for command execution
type Runtime struct {
	// Core components
	Engine *engine.Engine
	Agents *agent.Manager
	Tools  *tool.Registry
	State  *bootstrap.Store
	Config config.Config

	// Session management
	CompactSession func(maxMessages int) (before int, after int)

	// MCP functions
	MCPStatus       func() string
	MCPServers      func() []MCPServerInfo
	MCPTools        func(server string) []MCPToolInfo
	MCPResources    func(server string) []MCPResourceInfo
	MCPTemplates    func(server string) []MCPTemplateInfo
	MCPSearchTools  func(query string) []MCPToolMatchInfo
	MCPCallTool     func(server, tool string, args map[string]any) (string, error)
	MCPReadResource func(server, uri string) (MCPResourceInfo, error)
	ReloadMCP       func() string
	ResetMCP        func() string
	ConnectMCP      func(server string) bool
	DisconnectMCP   func(server string) bool
	RestartMCP      func(server string) bool
	PingMCP         func(server string) (string, bool)
	AuthenticateMCP func(server, token string) bool
	SetMCPEnabledAll     func(enabled bool)
	SetMCPServiceStatus  func(status string)
	SetMCPEnabled        func(name string, enabled bool) bool
	SetMCPStatus         func(name, status string) bool
	AddMCPServer         func(server MCPServerInfo)
	RemoveMCPServer      func(name string) bool

	// Plugin functions
	PluginStatus            func() string
	PluginList              func() []PluginInfo
	ReloadPlugins           func() string
	ResetPlugins            func() string
	SetPluginsEnabledAll    func(enabled bool)
	SetPluginsServiceStatus func(status string)
	SetPluginEnabled        func(name string, enabled bool) bool
	SetPluginStatus         func(name, status string) bool
	AddPlugin               func(plugin PluginInfo)
	RemovePlugin            func(name string) bool

	// Skills functions
	ReloadSkills func() string
	SkillStatus  func() string
	SkillList    func() []SkillInfo

	// Hooks functions
	HookStatus            func() string
	HookList              func() []HookInfo
	ReloadHooks           func() string
	ResetHooks            func() string
	SetHooksEnabledAll    func(enabled bool)
	SetHooksServiceStatus func(status string)
	SetHookEnabled        func(event string, enabled bool) bool
	SetHookStatus         func(event, status string) bool
	AddHook               func(hook HookInfo)
	RemoveHook            func(event string) bool

	// UI callbacks
	OnExit        func()
	OnClear       func()
	OnThemeChange func(theme string)
	OnModelChange func(model string)

	// Command registry for help/listing
	Commands func() []Command
}

// AgentTask is an alias for task.AgentTask
type AgentTask = task.AgentTask

// MCP info types
type MCPServerInfo struct {
	Name          string   `json:"name"`
	Transport     string   `json:"transport"`
	URL           string   `json:"url,omitempty"`
	Command       string   `json:"command,omitempty"`
	Args          []string `json:"args,omitempty"`
	Enabled       bool     `json:"enabled"`
	Status        string   `json:"status"`
	Description   string   `json:"description,omitempty"`
	ToolCount     int      `json:"tool_count,omitempty"`
	ResourceCount int      `json:"resource_count,omitempty"`
	Dev           bool     `json:"dev,omitempty"`
	Connected     bool     `json:"connected,omitempty"`
	Channel       string   `json:"channel,omitempty"`
	Auth          string   `json:"auth,omitempty"`
	LastConnected string   `json:"last_connected,omitempty"`
	LastCalledAt  string   `json:"last_called_at,omitempty"`
	LastResult    string   `json:"last_result,omitempty"`
}

type MCPToolInfo struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema,omitempty"`
	ReadOnly    bool           `json:"read_only,omitempty"`
	Response    string         `json:"response,omitempty"`
}

type MCPResourceInfo struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
	Content     string `json:"content,omitempty"`
}

type MCPTemplateInfo struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	URI         string `json:"uri,omitempty"`
}

type MCPToolMatchInfo struct {
	Name        string  `json:"name"`
	Server      string  `json:"server"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
	ReadOnly    bool    `json:"read_only,omitempty"`
}

// Plugin info types
type PluginInfo struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Enabled      bool     `json:"enabled"`
	Status       string   `json:"status"`
	Commands     []string `json:"commands,omitempty"`
	Source       string   `json:"source,omitempty"`
	SourceType   string   `json:"source_type,omitempty"`
	Marketplace  string   `json:"marketplace,omitempty"`
	Category     string   `json:"category,omitempty"`
	Path         string   `json:"path,omitempty"`
	CommandCount int      `json:"command_count,omitempty"`
	AgentCount   int      `json:"agent_count,omitempty"`
	SkillCount   int      `json:"skill_count,omitempty"`
	HookCount    int      `json:"hook_count,omitempty"`
	Dev          bool     `json:"dev,omitempty"`
}

// Skill info types
type SkillInfo struct {
	Name                   string   `json:"name"`
	Description            string   `json:"description"`
	ProgressMessage        string   `json:"progress_message,omitempty"`
	ContentLength          int      `json:"content_length,omitempty"`
	AllowedTools           []string `json:"allowed_tools,omitempty"`
	Source                 string   `json:"source,omitempty"`
	UserInvocable          bool     `json:"user_invocable,omitempty"`
	DisplayName            string   `json:"display_name,omitempty"`
	WhenToUse              string   `json:"when_to_use,omitempty"`
	Path                   string   `json:"path,omitempty"`
	Aliases                []string `json:"aliases,omitempty"`
	ArgumentHint           string   `json:"argument_hint,omitempty"`
	Version                string   `json:"version,omitempty"`
	Model                  string   `json:"model,omitempty"`
	Context                string   `json:"context,omitempty"`
	Agent                  string   `json:"agent,omitempty"`
	LoadedFrom             string   `json:"loaded_from,omitempty"`
	BaseDir                string   `json:"base_dir,omitempty"`
	DisableModelInvocation bool     `json:"disable_model_invocation,omitempty"`
}

// Hook info types
type HookInfo struct {
	Event       string `json:"event"`
	Command     string `json:"command"`
	Enabled     bool   `json:"enabled"`
	Status      string `json:"status"`
	Description string `json:"description,omitempty"`
	Source      string `json:"source,omitempty"`
	Matcher     string `json:"matcher,omitempty"`
	TimeoutMs   int    `json:"timeout_ms,omitempty"`
	Blocking    bool   `json:"blocking,omitempty"`
	Shell       string `json:"shell,omitempty"`
	RunCount    int    `json:"run_count,omitempty"`
	LastRunAt   string `json:"last_run_at,omitempty"`
	LastResult  string `json:"last_result,omitempty"`
	LastError   string `json:"last_error,omitempty"`
	LastOutput  string `json:"last_output,omitempty"`
}