package tool

import (
	"context"

	"claude-code-go/internal/bootstrap"
	"claude-code-go/internal/infra/mcp"
	"claude-code-go/internal/task"
)

// TaskStore is the interface for background agent tasks
type TaskStore interface {
	List() []*task.AgentTask
	Get(id string) (*task.AgentTask, bool)
}

// TaskListStore is the interface for the Todo V2 task list system
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
	TaskList      TaskListStore
	Stop          func(taskID string) error
	SpawnAgent    func(context.Context, AgentSpawnRequest) (*task.AgentTask, error)
	ContinueAgent func(context.Context, string, string, bool) (*task.AgentTask, error)
	MCP           MCPRuntime
	Store         *bootstrap.Store // Bootstrap store for CWD management and state
	AgentId       string           // Agent ID for subagents, empty for main agent
}

type AgentSpawnRequest struct {
	Type        string
	Prompt      string
	Description string
	Background  bool
}
