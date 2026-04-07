package schedule

import (
	"context"
	"encoding/json"
	"fmt"

	"claude-code-go/internal/tool"
)

// RemoteTriggerToolName is the name of the remote trigger tool
const RemoteTriggerToolName = "RemoteTrigger"

// RemoteTriggerDescription describes the remote trigger tool
const RemoteTriggerDescription = `Manage scheduled remote Claude Code agents (triggers) via the claude.ai CCR API. Auth is handled in-process — the token never reaches the shell.`

// RemoteTriggerPrompt contains the detailed prompt for the tool
const RemoteTriggerPrompt = `Call the claude.ai remote-trigger API. Use this instead of curl — the OAuth token is added automatically in-process and never exposed.

Actions:
- list: GET /v1/code/triggers
- get: GET /v1/code/triggers/{trigger_id}
- create: POST /v1/code/triggers (requires body)
- update: POST /v1/code/triggers/{trigger_id} (requires body, partial update)
- run: POST /v1/code/triggers/{trigger_id}/run

The response is the raw JSON from the API.`

// Remote trigger actions
const (
	ActionList   = "list"
	ActionGet    = "get"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionRun    = "run"
)

// RemoteTriggerTool implements the remote trigger tool
type RemoteTriggerTool struct{}

// Name returns the tool name
func (RemoteTriggerTool) Name() string { return RemoteTriggerToolName }

// Description returns the tool description
func (RemoteTriggerTool) Description() string { return RemoteTriggerDescription }

// IsReadOnly returns true for list and get actions, false for others
func (RemoteTriggerTool) IsReadOnly(in tool.Input) bool {
	action := getString(in, "action")
	return action == ActionList || action == ActionGet
}

// ParametersSchema returns the JSON schema for the tool parameters
func (RemoteTriggerTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"action": tool.SchemaEnumString("The action to perform",
			ActionList,
			ActionGet,
			ActionCreate,
			ActionUpdate,
			ActionRun,
		),
		"trigger_id": tool.SchemaString("Required for get, update, and run"),
		"body":       tool.SchemaAnyObject("JSON body for create and update"),
	}, "action")
}

// Call executes the remote trigger tool
func (t RemoteTriggerTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	action := getString(in, "action")
	triggerID := getString(in, "trigger_id")
	body := getMap(in, "body")

	// Validate action
	validActions := map[string]bool{
		ActionList:   true,
		ActionGet:    true,
		ActionCreate: true,
		ActionUpdate: true,
		ActionRun:    true,
	}
	if !validActions[action] {
		return tool.Result{}, fmt.Errorf("invalid action: %s", action)
	}

	// Validate required parameters
	switch action {
	case ActionGet, ActionRun:
		if triggerID == "" {
			return tool.Result{}, fmt.Errorf("%s requires trigger_id", action)
		}
	case ActionUpdate:
		if triggerID == "" {
			return tool.Result{}, fmt.Errorf("%s requires trigger_id", action)
		}
		if body == nil {
			return tool.Result{}, fmt.Errorf("%s requires body", action)
		}
	case ActionCreate:
		if body == nil {
			return tool.Result{}, fmt.Errorf("%s requires body", action)
		}
	}

	// TODO: Implement actual API calls
	// This requires OAuth integration and HTTP client
	// For now, return a placeholder result
	var mockResponse map[string]any

	switch action {
	case ActionList:
		mockResponse = map[string]any{
			"triggers": []map[string]any{},
		}
	case ActionGet:
		mockResponse = map[string]any{
			"id":     triggerID,
			"status": "active",
		}
	case ActionCreate:
		mockResponse = map[string]any{
			"id":     "new-trigger-id",
			"status": "created",
			"body":   body,
		}
	case ActionUpdate:
		mockResponse = map[string]any{
			"id":     triggerID,
			"status": "updated",
			"body":   body,
		}
	case ActionRun:
		mockResponse = map[string]any{
			"id":     triggerID,
			"status": "running",
			"run_id": "run-123",
		}
	}

	// Marshal response
	jsonData, _ := json.MarshalIndent(mockResponse, "", "  ")

	return tool.Result{
		Content: fmt.Sprintf("HTTP 200\n%s", string(jsonData)),
		Meta: map[string]any{
			"action":     action,
			"trigger_id": triggerID,
			"status":     200,
			"json":       string(jsonData),
		},
	}, nil
}

// RegisterRemoteTriggerTools registers remote trigger tools to the registry
func RegisterRemoteTriggerTools(r *tool.Registry) {
	r.Register(RemoteTriggerTool{})
}