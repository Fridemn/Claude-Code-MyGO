package todo

import (
	"context"
	"fmt"

	"claude-go/internal/bootstrap"
	"claude-go/internal/tool"
)

// TodoWriteTool implements the TodoWrite tool for managing session task lists
type TodoWriteTool struct{}

// Tool name constant
const ToolName = "TodoWrite"

// Description
const Description = "Update the todo list for the current session. To be used proactively and often to track progress and pending tasks. Make sure that at least one task is in_progress at all times. Always provide both content (imperative) and activeForm (present continuous) for each task."

// TodoItemStatus represents the status of a todo item
type TodoItemStatus string

const (
	TodoStatusPending    TodoItemStatus = "pending"
	TodoStatusInProgress TodoItemStatus = "in_progress"
	TodoStatusCompleted  TodoItemStatus = "completed"
)

// IsValid checks if the status is valid
func (s TodoItemStatus) IsValid() bool {
	switch s {
	case TodoStatusPending, TodoStatusInProgress, TodoStatusCompleted:
		return true
	default:
		return false
	}
}

// TodoItem represents a single todo item
type TodoItem struct {
	Content    string        `json:"content"`
	Status     TodoItemStatus `json:"status"`
	ActiveForm string        `json:"activeForm"`
}

// IsValid checks if the todo item is valid
func (t TodoItem) IsValid() bool {
	if t.Content == "" {
		return false
	}
	if t.ActiveForm == "" {
		return false
	}
	if !t.Status.IsValid() {
		return false
	}
	return true
}

// TodoList is a list of todo items
type TodoList []TodoItem

// IsValid checks if all items in the list are valid
func (l TodoList) IsValid() bool {
	for _, item := range l {
		if !item.IsValid() {
			return false
		}
	}
	return true
}

// AllCompleted checks if all items are completed
func (l TodoList) AllCompleted() bool {
	for _, item := range l {
		if item.Status != TodoStatusCompleted {
			return false
		}
	}
	return true
}

// HasInProgress checks if any item is in progress
func (l TodoList) HasInProgress() bool {
	for _, item := range l {
		if item.Status == TodoStatusInProgress {
			return true
		}
	}
	return false
}

// Output represents the tool output
type Output struct {
	OldTodos    TodoList `json:"oldTodos"`
	NewTodos    TodoList `json:"newTodos"`
	NudgeNeeded bool     `json:"verificationNudgeNeeded,omitempty"`
}

// Name returns the tool name
func (TodoWriteTool) Name() string {
	return ToolName
}

// Description returns the tool description
func (TodoWriteTool) Description() string {
	return Description
}

// IsReadOnly returns false - TodoWrite always modifies state
func (TodoWriteTool) IsReadOnly(tool.Input) bool {
	return false
}

// ParametersSchema returns the JSON schema for the input
func (TodoWriteTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"todos": tool.SchemaArray("The updated todo list", tool.SchemaObject(map[string]any{
			"content":    tool.SchemaString("The task content in imperative form (e.g., \"Run tests\")"),
			"status":     tool.SchemaEnumString("The task status", "pending", "in_progress", "completed"),
			"activeForm": tool.SchemaString("The task in present continuous form (e.g., \"Running tests\")"),
		}, "content", "status", "activeForm")),
	}, "todos")
}

// Call executes the tool
func (t TodoWriteTool) Call(ctx context.Context, in tool.Input, rt tool.Runtime) (tool.Result, error) {
	// Parse todos from input
	todosRaw, ok := in["todos"].([]any)
	if !ok {
		return tool.Result{Error: "todos parameter is required and must be an array"}, nil
	}

	todos := make(TodoList, 0, len(todosRaw))
	for _, itemRaw := range todosRaw {
		itemMap, ok := itemRaw.(map[string]any)
		if !ok {
			continue
		}

		content, _ := itemMap["content"].(string)
		statusStr, _ := itemMap["status"].(string)
		activeForm, _ := itemMap["activeForm"].(string)

		status := TodoItemStatus(statusStr)
		if !status.IsValid() {
			return tool.Result{
				Error: fmt.Sprintf("invalid status: %q", statusStr),
			}, nil
		}

		item := TodoItem{
			Content:    content,
			Status:     status,
			ActiveForm: activeForm,
		}

		if !item.IsValid() {
			return tool.Result{
				Error: "todo item is invalid: content and activeForm must be non-empty",
			}, nil
		}

		todos = append(todos, item)
	}

	// Determine the todo key: agent ID for subagents, session ID for main agent
	todoKey := rt.AgentId
	if todoKey == "" && rt.Store != nil {
		todoKey = rt.Store.GetSessionID()
	}

	// Get old todos from store
	var oldTodos TodoList
	if rt.Store != nil {
		oldItems := rt.Store.GetTodos(todoKey)
		for _, item := range oldItems {
			oldTodos = append(oldTodos, TodoItem{
				Content:    item.Content,
				Status:     TodoItemStatus(item.Status),
				ActiveForm: item.ActiveForm,
			})
		}
	}

	// Determine if all tasks are done
	allDone := todos.AllCompleted()
	var newTodos TodoList
	if allDone {
		// Clear todos when all completed
		newTodos = TodoList{}
	} else {
		newTodos = todos
	}

	// Convert to bootstrap.TodoItem for storage
	storeTodos := make([]bootstrap.TodoItem, len(newTodos))
	for i, item := range newTodos {
		storeTodos[i] = bootstrap.TodoItem{
			Content:    item.Content,
			Status:     bootstrap.TodoItemStatus(item.Status),
			ActiveForm: item.ActiveForm,
		}
	}

	// Persist to store
	if rt.Store != nil {
		rt.Store.SetTodos(todoKey, storeTodos)
	}

	// Structural nudge for verification agent (simplified)
	// In TS, this checks for verification agent features
	// We skip this for the basic implementation
	var nudgeNeeded bool

	return tool.Result{
		Content: Output{
			OldTodos:    oldTodos,
			NewTodos:    newTodos,
			NudgeNeeded: nudgeNeeded,
		},
	}, nil
}

// RegisterTodoTools registers todo tools with the registry
func RegisterTodoTools(r *tool.Registry) {
	r.Register(TodoWriteTool{})
}