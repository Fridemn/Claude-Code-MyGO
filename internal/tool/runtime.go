package tool

import (
	"context"

	"claude-go/internal/bootstrap"
	"claude-go/internal/infra/mcp"
	"claude-go/internal/task"
)

// TaskStore is the interface for background agent tasks
type TaskStore interface {
	List() []*task.AgentTask
	Get(id string) (*task.AgentTask, bool)
}

// ShellTaskStore is the interface for background shell tasks
// Ported from src/utils/ShellCommand.ts:ShellCommand
type ShellTaskStore interface {
	CreateTask(command, description string) (*task.ShellTaskState, error)
	UpdateTaskStatus(taskID string, status task.ShellTaskStatus, exitCode int, interrupted bool) error
	GetTask(taskID string) (*task.ShellTaskState, error)
	ListTasks() []*task.ShellTaskState
	WriteOutput(taskID string, data []byte) error
	ReadOutput(taskID string, maxBytes int64) (string, error)
	DeleteTask(taskID string) error
	KillTask(ctx context.Context, taskID string, reason string) error
	DrainNotices() []task.ShellTaskNotice
}

// TaskListStore is the interface for task list management
// Used by TaskCreate/TaskGet/TaskList/TaskUpdate/TaskStop tools
type TaskListStore interface {
	Create(subject, description, activeForm string, metadata map[string]interface{}) (*task.Task, error)
	Get(taskID string) (*task.Task, error)
	List() ([]*task.Task, error)
	Update(taskID string, updates map[string]interface{}) (*task.Task, error)
	Delete(taskID string) error
	BlockTask(fromTaskID, toTaskID string) error
}

type MCPDynamicToolInfo struct {
	Name        string
	Server      string
	Tool        string
	Description string
	ReadOnly    bool
}

type MCPRuntime interface {
	Servers() []string
	ListTools(server string) []mcp.Tool
	ListResources(server string) []mcp.Resource
	ListTemplates(server string) []mcp.Template
	SearchTools(query string) []mcp.ToolMatch
	CallTool(server, tool string, args map[string]any) (string, error)
	ReadResource(server, uri string) (mcp.Resource, error)
	Authenticate(server, token string) error
	Connect(server string) error
	Disconnect(server string) error
	Ping(server string) (string, error)
	DynamicTools() []MCPDynamicToolInfo
}

type Runtime struct {
	Tasks         TaskStore
	ShellTasks    ShellTaskStore // Background shell task management
	TaskList      TaskListStore
	Stop          func(taskID string) error
	SpawnAgent    func(context.Context, AgentSpawnRequest) (*task.AgentTask, error)
	ContinueAgent func(context.Context, string, string, bool) (*task.AgentTask, error)
	MCP           MCPRuntime
	Store         *bootstrap.Store // Bootstrap store for CWD management and state
	AgentId       string           // Agent ID for subagents, empty for main agent
	// EmitProgress allows tools to stream structured progress events into the
	// session transcript (for example REPL inner-step lifecycle messages).
	EmitProgress func(map[string]any)
	// AskPermission is called when a tool needs user permission
	// Returns true if approved, false if denied
	AskPermission func(ctx context.Context, toolName string, input Input, message string) (bool, error)
}

type AgentSpawnRequest struct {
	Type        string
	Prompt      string
	Description string
	Background  bool
}
