package tests

import (
	"context"
	"errors"
	"testing"

	"claude-go/internal/task"
	"claude-go/internal/tool"
	"claude-go/internal/tool/agent"
	tasktool "claude-go/internal/tool/task"
)

// mockTaskListStore implements tool.TaskListStore for testing
type mockTaskListStore struct {
	tasks []*task.Task
}

func (m *mockTaskListStore) Create(subject, description, activeForm string, metadata map[string]interface{}) (*task.Task, error) {
	t := &task.Task{
		ID:          "1",
		Subject:     subject,
		Description: description,
		ActiveForm:  activeForm,
		Status:      task.TaskListStatusPending,
		Metadata:    metadata,
	}
	m.tasks = append(m.tasks, t)
	return t, nil
}

func (m *mockTaskListStore) Get(taskID string) (*task.Task, error) {
	for _, t := range m.tasks {
		if t.ID == taskID {
			return t, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockTaskListStore) List() ([]*task.Task, error) {
	return m.tasks, nil
}

func (m *mockTaskListStore) Update(taskID string, updates map[string]interface{}) (*task.Task, error) {
	for _, t := range m.tasks {
		if t.ID == taskID {
			if subject, ok := updates["subject"].(string); ok {
				t.Subject = subject
			}
			if status, ok := updates["status"].(string); ok {
				t.Status = task.TaskListStatus(status)
			}
			return t, nil
		}
	}
	return nil, errors.New("not found")
}

func (m *mockTaskListStore) Delete(taskID string) error {
	for i, t := range m.tasks {
		if t.ID == taskID {
			m.tasks = append(m.tasks[:i], m.tasks[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *mockTaskListStore) BlockTask(fromTaskID, toTaskID string) error {
	return nil
}

func TestTaskTools(t *testing.T) {
	t.Parallel()

	taskListStore := &mockTaskListStore{}
	_, err := taskListStore.Create("demo task", "test description", "Testing", nil)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}

	runtime := tool.Runtime{
		TaskList: taskListStore,
	}

	// Test TaskListTool
	result, err := (tasktool.TaskListTool{}).Call(context.Background(), tool.Input{}, runtime)
	if err != nil {
		t.Fatalf("task_list: %v", err)
	}
	listResult, ok := result.Content.(map[string]any)
	if !ok {
		t.Fatalf("unexpected task_list result type: %T", result.Content)
	}
	tasks, ok := listResult["tasks"].([]map[string]any)
	if !ok || len(tasks) != 1 {
		t.Fatalf("unexpected tasks in list: %#v", listResult)
	}

	// Test TaskGetTool
	result, err = (tasktool.TaskGetTool{}).Call(context.Background(), tool.Input{"taskId": "1"}, runtime)
	if err != nil {
		t.Fatalf("task_get: %v", err)
	}
	getResult, ok := result.Content.(map[string]any)
	if !ok {
		t.Fatalf("unexpected task_get result type: %T", result.Content)
	}
	taskData, ok := getResult["task"].(map[string]any)
	if !ok || taskData["id"] != "1" {
		t.Fatalf("unexpected task_get result: %#v", getResult)
	}

	// Test TaskUpdateTool
	result, err = (tasktool.TaskUpdateTool{}).Call(context.Background(), tool.Input{
		"taskId": "1",
		"status": "completed",
	}, runtime)
	if err != nil {
		t.Fatalf("task_update: %v", err)
	}
	updateResult, ok := result.Content.(map[string]any)
	if !ok {
		t.Fatalf("unexpected task_update result type: %T", result.Content)
	}
	if updateResult["success"] != true {
		t.Fatalf("expected success: %#v", updateResult)
	}
}

func TestAgentTools(t *testing.T) {
	t.Parallel()

	spawned := false
	continued := false
	runtime := tool.Runtime{
		SpawnAgent: func(_ context.Context, req tool.AgentSpawnRequest) (*task.AgentTask, error) {
			spawned = true
			if req.Prompt == "" {
				return nil, errors.New("missing prompt")
			}
			return &task.AgentTask{ID: "a1", Prompt: req.Prompt, AgentType: req.Type}, nil
		},
		ContinueAgent: func(_ context.Context, taskID, prompt string, background bool) (*task.AgentTask, error) {
			continued = true
			return &task.AgentTask{ID: taskID, Prompt: prompt}, nil
		},
	}

	// Create an AgentTool with a mock agent definition
	mockAgent := agent.BuiltInAgentDefinition{}
	mockAgent.AgentType = "general-purpose"
	agentTool := agent.CreateAgentTool([]agent.AgentDefinition{mockAgent})

	if _, err := agentTool.Call(context.Background(), tool.Input{}, runtime); err == nil {
		t.Fatalf("expected agent_run missing prompt error")
	}

	result, err := agentTool.Call(context.Background(), tool.Input{
		"type":        "general-purpose",
		"prompt":      "hello",
		"description": "demo",
		"background":  true,
	}, runtime)
	if err != nil {
		t.Fatalf("agent_run: %v", err)
	}
	if !spawned {
		t.Fatalf("expected spawn agent callback")
	}
	// AgentTool returns a map, not *task.AgentTask
	resultMap, ok := result.Content.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type: %T", result.Content)
	}
	if resultMap["agent_id"] != "a1" {
		t.Fatalf("unexpected agent_id: %v", resultMap["agent_id"])
	}

	if _, err := (agent.SendMessageTool{}).Call(context.Background(), tool.Input{"to": "a1"}, runtime); err == nil {
		t.Fatalf("expected send_message missing prompt error")
	}

	result, err = (agent.SendMessageTool{}).Call(context.Background(), tool.Input{
		"to":      "a1",
		"message": "follow up",
	}, runtime)
	if err != nil {
		t.Fatalf("send_message: %v", err)
	}
	if !continued {
		t.Fatalf("expected continue agent callback")
	}
	sendResult, ok := result.Content.(agent.MessageOutput)
	if !ok {
		t.Fatalf("unexpected send_message result type: %T", result.Content)
	}
	if !sendResult.Success {
		t.Fatalf("expected success: %v", sendResult)
	}
}

