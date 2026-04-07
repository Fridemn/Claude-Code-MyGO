package task

import (
	"context"
	"fmt"
	"strings"

	"claude-code-go/internal/task"
	"claude-code-go/internal/tool"
)

// ============== TaskCreate Tool ==============

type TaskCreateTool struct{}

func (TaskCreateTool) Name() string        { return "TaskCreate" }
func (TaskCreateTool) Description() string { return "Create a new task in the task list" }
func (TaskCreateTool) IsReadOnly(tool.Input) bool { return false }

func (t TaskCreateTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"subject":     tool.SchemaString("A brief title for the task"),
		"description": tool.SchemaString("What needs to be done"),
		"activeForm":  tool.SchemaString("Present continuous form shown in spinner when in_progress (e.g., \"Running tests\")"),
		"metadata":    tool.SchemaAnyObject("Arbitrary metadata to attach to the task"),
	}, "subject", "description")
}

func (t TaskCreateTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.TaskList == nil {
		return tool.Result{}, fmt.Errorf("task list not available")
	}

	subject, _ := in["subject"].(string)
	description, _ := in["description"].(string)
	activeForm, _ := in["activeForm"].(string)
	metadata, _ := in["metadata"].(map[string]interface{})

	if strings.TrimSpace(subject) == "" {
		return tool.Result{}, fmt.Errorf("subject is required")
	}

	taskItem, err := runtime.TaskList.Create(subject, description, activeForm, metadata)
	if err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		Content: map[string]any{
			"task": map[string]any{
				"id":      taskItem.ID,
				"subject": taskItem.Subject,
			},
		},
	}, nil
}

// ============== TaskGet Tool ==============

type TaskGetTool struct{}

func (TaskGetTool) Name() string        { return "TaskGet" }
func (TaskGetTool) Description() string { return "Get a task by ID from the task list" }
func (TaskGetTool) IsReadOnly(tool.Input) bool { return true }

func (t TaskGetTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"taskId": tool.SchemaString("The ID of the task to retrieve"),
	}, "taskId")
}

func (t TaskGetTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.TaskList == nil {
		return tool.Result{}, fmt.Errorf("task list not available")
	}

	taskID, _ := in["taskId"].(string)
	if strings.TrimSpace(taskID) == "" {
		return tool.Result{}, fmt.Errorf("taskId is required")
	}

	taskItem, err := runtime.TaskList.Get(taskID)
	if err != nil {
		return tool.Result{Content: map[string]any{"task": nil}}, nil
	}

	return tool.Result{
		Content: map[string]any{
			"task": map[string]any{
				"id":          taskItem.ID,
				"subject":     taskItem.Subject,
				"description": taskItem.Description,
				"status":      taskItem.Status,
				"blocks":      taskItem.Blocks,
				"blockedBy":   taskItem.BlockedBy,
			},
		},
	}, nil
}

// ============== TaskListTool Tool ==============

type TaskListTool struct{}

func (TaskListTool) Name() string        { return "TaskList" }
func (TaskListTool) Description() string { return "List all tasks in the task list" }
func (TaskListTool) IsReadOnly(tool.Input) bool { return true }

func (t TaskListTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{})
}

func (t TaskListTool) Call(_ context.Context, _ tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.TaskList == nil {
		return tool.Result{}, fmt.Errorf("task list not available")
	}

	tasks, err := runtime.TaskList.List()
	if err != nil {
		return tool.Result{}, err
	}

	// Build resolved task IDs set
	resolvedTaskIDs := make(map[string]bool)
	for _, t := range tasks {
		if t.Status == task.TaskListStatusCompleted {
			resolvedTaskIDs[t.ID] = true
		}
	}

	// Filter out internal tasks and format output
	var taskList []map[string]any
	for _, t := range tasks {
		// Skip internal tasks
		if t.Metadata != nil {
			if _, internal := t.Metadata["_internal"]; internal {
				continue
			}
		}

		// Filter blockedBy to only unresolved tasks
		var blockedBy []string
		for _, id := range t.BlockedBy {
			if !resolvedTaskIDs[id] {
				blockedBy = append(blockedBy, id)
			}
		}

		taskList = append(taskList, map[string]any{
			"id":        t.ID,
			"subject":   t.Subject,
			"status":    t.Status,
			"owner":     t.Owner,
			"blockedBy": blockedBy,
		})
	}

	return tool.Result{Content: map[string]any{"tasks": taskList}}, nil
}

// ============== TaskUpdate Tool ==============

type TaskUpdateTool struct{}

func (TaskUpdateTool) Name() string        { return "TaskUpdate" }
func (TaskUpdateTool) Description() string { return "Update a task in the task list" }
func (TaskUpdateTool) IsReadOnly(tool.Input) bool { return false }

func (t TaskUpdateTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"taskId":       tool.SchemaString("The ID of the task to update"),
		"subject":      tool.SchemaString("New subject for the task"),
		"description":  tool.SchemaString("New description for the task"),
		"activeForm":   tool.SchemaString("Present continuous form shown in spinner when in_progress (e.g., \"Running tests\")"),
		"status":       tool.SchemaEnumString("New status for the task (pending, in_progress, completed, or deleted)", "pending", "in_progress", "completed", "deleted"),
		"addBlocks":    tool.SchemaArray("Task IDs that this task blocks", tool.SchemaString("Task ID")),
		"addBlockedBy": tool.SchemaArray("Task IDs that block this task", tool.SchemaString("Task ID")),
		"owner":        tool.SchemaString("New owner for the task"),
		"metadata":     tool.SchemaAnyObject("Metadata keys to merge into the task. Set a key to null to delete it."),
	}, "taskId")
}

func (t TaskUpdateTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.TaskList == nil {
		return tool.Result{}, fmt.Errorf("task list not available")
	}

	taskID, _ := in["taskId"].(string)
	if strings.TrimSpace(taskID) == "" {
		return tool.Result{}, fmt.Errorf("taskId is required")
	}

	// Check if task exists
	existingTask, err := runtime.TaskList.Get(taskID)
	if err != nil {
		return tool.Result{
			Content: map[string]any{
				"success":       false,
				"taskId":        taskID,
				"updatedFields": []string{},
				"error":         "Task not found",
			},
		}, nil
	}

	updates := make(map[string]interface{})
	var updatedFields []string
	var statusChange map[string]string

	// Handle status updates
	if status, ok := in["status"].(string); ok && status != "" {
		if status == "deleted" {
			// Delete the task
			if err := runtime.TaskList.Delete(taskID); err != nil {
				return tool.Result{
					Content: map[string]any{
						"success":       false,
						"taskId":        taskID,
						"updatedFields": []string{},
						"error":         "Failed to delete task",
					},
				}, nil
			}
			return tool.Result{
				Content: map[string]any{
					"success":       true,
					"taskId":        taskID,
					"updatedFields": []string{"deleted"},
					"statusChange":  map[string]string{"from": string(existingTask.Status), "to": "deleted"},
				},
			}, nil
		}

		if task.TaskListStatus(status) != existingTask.Status {
			updates["status"] = status
			updatedFields = append(updatedFields, "status")
			statusChange = map[string]string{"from": string(existingTask.Status), "to": status}
		}
	}

	// Handle other field updates
	if subject, ok := in["subject"].(string); ok && subject != "" && subject != existingTask.Subject {
		updates["subject"] = subject
		updatedFields = append(updatedFields, "subject")
	}
	if description, ok := in["description"].(string); ok && description != "" && description != existingTask.Description {
		updates["description"] = description
		updatedFields = append(updatedFields, "description")
	}
	if activeForm, ok := in["activeForm"].(string); ok && activeForm != existingTask.ActiveForm {
		updates["activeForm"] = activeForm
		updatedFields = append(updatedFields, "activeForm")
	}
	if owner, ok := in["owner"].(string); ok && owner != existingTask.Owner {
		updates["owner"] = owner
		updatedFields = append(updatedFields, "owner")
	}
	if metadata, ok := in["metadata"].(map[string]interface{}); ok {
		updates["metadata"] = metadata
		updatedFields = append(updatedFields, "metadata")
	}

	// Apply updates
	if len(updates) > 0 {
		if _, err := runtime.TaskList.Update(taskID, updates); err != nil {
			return tool.Result{}, err
		}
	}

	// Handle addBlocks
	if addBlocks, ok := in["addBlocks"].([]interface{}); ok {
		for _, id := range addBlocks {
			blockID, _ := id.(string)
			if blockID == "" {
				continue
			}
			alreadyBlocked := false
			for _, existing := range existingTask.Blocks {
				if existing == blockID {
					alreadyBlocked = true
					break
				}
			}
			if !alreadyBlocked {
				if err := runtime.TaskList.BlockTask(taskID, blockID); err == nil {
					updatedFields = append(updatedFields, "blocks")
				}
			}
		}
	}

	// Handle addBlockedBy
	if addBlockedBy, ok := in["addBlockedBy"].([]interface{}); ok {
		for _, id := range addBlockedBy {
			blockerID, _ := id.(string)
			if blockerID == "" {
				continue
			}
			alreadyBlocked := false
			for _, existing := range existingTask.BlockedBy {
				if existing == blockerID {
					alreadyBlocked = true
					break
				}
			}
			if !alreadyBlocked {
				if err := runtime.TaskList.BlockTask(blockerID, taskID); err == nil {
					updatedFields = append(updatedFields, "blockedBy")
				}
			}
		}
	}

	result := map[string]any{
		"success":       true,
		"taskId":        taskID,
		"updatedFields": updatedFields,
	}
	if statusChange != nil {
		result["statusChange"] = statusChange
	}

	return tool.Result{Content: result}, nil
}

// ============== TaskStop Tool ==============

type TaskStopTool struct{}

func (TaskStopTool) Name() string        { return "TaskStop" }
func (TaskStopTool) Description() string { return "Stop a running background task by ID" }
func (TaskStopTool) IsReadOnly(tool.Input) bool { return false }

func (t TaskStopTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"task_id":  tool.SchemaString("The ID of the background task to stop"),
		"shell_id": tool.SchemaString("Deprecated: use task_id instead"),
	}, "task_id")
}

func (t TaskStopTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.Stop == nil {
		return tool.Result{}, fmt.Errorf("task stop function not available")
	}

	// Support both task_id and shell_id (deprecated KillShell compat)
	taskID, _ := in["task_id"].(string)
	if taskID == "" {
		taskID, _ = in["shell_id"].(string)
	}

	if strings.TrimSpace(taskID) == "" {
		return tool.Result{}, fmt.Errorf("task_id is required")
	}

	// Verify task exists and is running
	if runtime.Tasks != nil {
		agentTask, ok := runtime.Tasks.Get(taskID)
		if !ok {
			return tool.Result{}, fmt.Errorf("no task found with ID: %s", taskID)
		}
		if agentTask.Status != task.StatusRunning {
			return tool.Result{}, fmt.Errorf("task %s is not running (status: %s)", taskID, agentTask.Status)
		}
	}

	if err := runtime.Stop(taskID); err != nil {
		return tool.Result{}, err
	}

	return tool.Result{
		Content: map[string]any{
			"message":   fmt.Sprintf("Successfully stopped task: %s", taskID),
			"task_id":   taskID,
			"task_type": "background",
		},
	}, nil
}

// ============== TaskOutput Tool ==============

type TaskOutputTool struct{}

func (TaskOutputTool) Name() string        { return "TaskOutput" }
func (TaskOutputTool) Description() string { return "Get output from a running or completed task" }
func (TaskOutputTool) IsReadOnly(tool.Input) bool { return true }

func (t TaskOutputTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"task_id": tool.SchemaString("The task ID to get output from"),
		"block":   tool.SchemaBoolean("Whether to wait for completion"),
		"timeout": tool.SchemaInteger("Max wait time in ms"),
	}, "task_id")
}

func (t TaskOutputTool) Call(_ context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	if runtime.Tasks == nil {
		return tool.Result{}, fmt.Errorf("task runtime not available")
	}

	taskID, _ := in["task_id"].(string)
	if strings.TrimSpace(taskID) == "" {
		return tool.Result{}, fmt.Errorf("task_id is required")
	}

	agentTask, ok := runtime.Tasks.Get(taskID)
	if !ok {
		return tool.Result{}, fmt.Errorf("no task found with ID: %s", taskID)
	}

	output := ""
	if agentTask.Summary != "" {
		output = agentTask.Summary
	} else if agentTask.Output != "" {
		output = agentTask.Output
	} else if agentTask.Error != "" {
		output = agentTask.Error
	}

	return tool.Result{
		Content: map[string]any{
			"retrieval_status": "success",
			"task": map[string]any{
				"task_id":     agentTask.ID,
				"task_type":   string(agentTask.Type),
				"status":      string(agentTask.Status),
				"description": agentTask.Description,
				"output":      output,
				"error":       agentTask.Error,
				"prompt":      agentTask.Prompt,
				"result":      agentTask.Summary,
			},
		},
	}, nil
}

// RegisterTaskTools registers all task tools with the registry
func RegisterTaskTools(r *tool.Registry) {
	r.Register(TaskCreateTool{})
	r.Register(TaskGetTool{})
	r.Register(TaskListTool{})
	r.Register(TaskUpdateTool{})
	r.Register(TaskStopTool{})
	r.Register(TaskOutputTool{})
}