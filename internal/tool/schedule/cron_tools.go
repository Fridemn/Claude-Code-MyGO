package schedule

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"claude-code-go/internal/tool"
)

// Cron tool names
const (
	CronCreateToolName = "CronCreate"
	CronDeleteToolName = "CronDelete"
	CronListToolName   = "CronList"
)

// CronCreateDescription describes the cron create tool
const CronCreateDescription = `Schedule a prompt to run at a future time — either recurring on a cron schedule, or once at a specific time. Uses standard 5-field cron in the user's local timezone: minute hour day-of-month month day-of-week. "0 9 * * *" means 9am local — no timezone conversion needed.`

// CronDeleteDescription describes the cron delete tool
const CronDeleteDescription = `Cancel a scheduled cron job by ID`

// CronListDescription describes the cron list tool
const CronListDescription = `List scheduled cron jobs`

// CronTask represents a scheduled cron task
type CronTask struct {
	ID        string    `json:"id"`
	Cron      string    `json:"cron"`
	Prompt    string    `json:"prompt"`
	Recurring bool      `json:"recurring"`
	Durable   bool      `json:"durable"`
	AgentID   string    `json:"agentId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// CronStore manages cron tasks
type CronStore struct {
	mu    sync.Mutex
	tasks map[string]*CronTask
}

// Global cron store
var globalCronStore = &CronStore{
	tasks: make(map[string]*CronTask),
}

// GetCronStore returns the global cron store
func GetCronStore() *CronStore {
	return globalCronStore
}

// Add adds a new cron task
func (s *CronStore) Add(task *CronTask) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tasks[task.ID] = task
}

// Remove removes a cron task by ID
func (s *CronStore) Remove(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tasks[id]; ok {
		delete(s.tasks, id)
		return true
	}
	return false
}

// Get retrieves a cron task by ID
func (s *CronStore) Get(id string) (*CronTask, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[id]
	return task, ok
}

// List returns all cron tasks
func (s *CronStore) List() []*CronTask {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks := make([]*CronTask, 0, len(s.tasks))
	for _, task := range s.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// --- CronCreateTool ---

// CronCreateTool implements the cron create tool
type CronCreateTool struct{}

// Name returns the tool name
func (CronCreateTool) Name() string { return CronCreateToolName }

// Description returns the tool description
func (CronCreateTool) Description() string { return CronCreateDescription }

// IsReadOnly returns false as this tool creates tasks
func (CronCreateTool) IsReadOnly(tool.Input) bool { return false }

// ParametersSchema returns the JSON schema for the tool parameters
func (CronCreateTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"cron":      tool.SchemaString("Standard 5-field cron expression in local time: \"M H DoM Mon DoW\" (e.g. \"*/5 * * * *\" = every 5 minutes, \"30 14 28 2 *\" = Feb 28 at 2:30pm local once)."),
		"prompt":    tool.SchemaString("The prompt to enqueue at each fire time."),
		"recurring": tool.SchemaBoolean("true (default) = fire on every cron match until deleted or auto-expired. false = fire once at the next match, then auto-delete."),
		"durable":   tool.SchemaBoolean("true = persist to .claude/scheduled_tasks.json and survive restarts. false (default) = in-memory only, dies when this Claude session ends."),
	}, "cron", "prompt")
}

// Call executes the cron create tool
func (t CronCreateTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	cronExpr := getString(in, "cron")
	prompt := getString(in, "prompt")
	recurring := getBool(in, "recurring")
	durable := getBool(in, "durable")

	// Default recurring to true if not specified
	if len(in) == 0 || in["recurring"] == nil {
		recurring = true
	}

	// Validate
	if cronExpr == "" {
		return tool.Result{}, fmt.Errorf("cron expression is required")
	}
	if prompt == "" {
		return tool.Result{}, fmt.Errorf("prompt is required")
	}

	// Validate cron expression (basic check - 5 fields)
	fields := splitCronFields(cronExpr)
	if len(fields) != 5 {
		return tool.Result{Error: fmt.Sprintf("Invalid cron expression '%s'. Expected 5 fields: M H DoM Mon DoW.", cronExpr)}, nil
	}

	// Generate task ID
	id := generateTaskID()

	// Create task
	task := &CronTask{
		ID:        id,
		Cron:      cronExpr,
		Prompt:    prompt,
		Recurring: recurring,
		Durable:   durable,
		CreatedAt: time.Now(),
	}

	// Add to store
	GetCronStore().Add(task)

	// Format human-readable schedule
	humanSchedule := cronToHuman(cronExpr)

	var where string
	if durable {
		where = "Persisted to .claude/scheduled_tasks.json"
	} else {
		where = "Session-only (not written to disk, dies when Claude exits)"
	}

	var resultMsg string
	if recurring {
		resultMsg = fmt.Sprintf("Scheduled recurring job %s (%s). %s. Auto-expires after 30 days. Use CronDelete to cancel sooner.", id, humanSchedule, where)
	} else {
		resultMsg = fmt.Sprintf("Scheduled one-shot task %s (%s). %s. It will fire once then auto-delete.", id, humanSchedule, where)
	}

	return tool.Result{
		Content: resultMsg,
		Meta: map[string]any{
			"id":            id,
			"cron":          cronExpr,
			"humanSchedule": humanSchedule,
			"recurring":     recurring,
			"durable":       durable,
			"prompt":        prompt,
		},
	}, nil
}

// --- CronDeleteTool ---

// CronDeleteTool implements the cron delete tool
type CronDeleteTool struct{}

// Name returns the tool name
func (CronDeleteTool) Name() string { return CronDeleteToolName }

// Description returns the tool description
func (CronDeleteTool) Description() string { return CronDeleteDescription }

// IsReadOnly returns false as this tool deletes tasks
func (CronDeleteTool) IsReadOnly(tool.Input) bool { return false }

// ParametersSchema returns the JSON schema for the tool parameters
func (CronDeleteTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{
		"id": tool.SchemaString("Job ID returned by CronCreate."),
	}, "id")
}

// Call executes the cron delete tool
func (t CronDeleteTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	id := getString(in, "id")

	if id == "" {
		return tool.Result{}, fmt.Errorf("id is required")
	}

	// Find task
	task, ok := GetCronStore().Get(id)
	if !ok {
		return tool.Result{Error: fmt.Sprintf("No scheduled job with id '%s'", id)}, nil
	}

	// Remove task
	GetCronStore().Remove(id)

	return tool.Result{
		Content: fmt.Sprintf("Cancelled job %s", id),
		Meta: map[string]any{
			"id":     id,
			"cron":   task.Cron,
			"prompt": task.Prompt,
		},
	}, nil
}

// --- CronListTool ---

// CronListTool implements the cron list tool
type CronListTool struct{}

// Name returns the tool name
func (CronListTool) Name() string { return CronListToolName }

// Description returns the tool description
func (CronListTool) Description() string { return CronListDescription }

// IsReadOnly returns true as this tool only lists tasks
func (CronListTool) IsReadOnly(tool.Input) bool { return true }

// ParametersSchema returns the JSON schema for the tool parameters
func (CronListTool) ParametersSchema() map[string]any {
	return tool.SchemaObject(map[string]any{})
}

// Call executes the cron list tool
func (t CronListTool) Call(ctx context.Context, in tool.Input, runtime tool.Runtime) (tool.Result, error) {
	tasks := GetCronStore().List()

	if len(tasks) == 0 {
		return tool.Result{Content: "No scheduled jobs."}, nil
	}

	var lines []string
	for _, task := range tasks {
		humanSchedule := cronToHuman(task.Cron)
		recurringStr := " (recurring)"
		if !task.Recurring {
			recurringStr = " (one-shot)"
		}
		durableStr := ""
		if !task.Durable {
			durableStr = " [session-only]"
		}
		promptTruncated := truncateString(task.Prompt, 80)
		lines = append(lines, fmt.Sprintf("%s — %s%s%s: %s", task.ID, humanSchedule, recurringStr, durableStr, promptTruncated))
	}

	return tool.Result{
		Content: strings.Join(lines, "\n"),
		Meta: map[string]any{
			"jobs":  tasks,
			"count": len(tasks),
		},
	}, nil
}

// Helper functions

// generateTaskID generates a random task ID
func generateTaskID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// splitCronFields splits a cron expression into its 5 fields
func splitCronFields(cron string) []string {
	// Simple split by space
	parts := strings.Fields(cron)
	return parts
}

// cronToHuman converts a cron expression to a human-readable string
func cronToHuman(cron string) string {
	fields := splitCronFields(cron)
	if len(fields) != 5 {
		return cron
	}

	minute, hour, dom, month, dow := fields[0], fields[1], fields[2], fields[3], fields[4]

	// Simple human-readable conversion
	if minute == "*" && hour == "*" && dom == "*" && month == "*" && dow == "*" {
		return "every minute"
	}
	if minute == "0" && hour == "*" && dom == "*" && month == "*" && dow == "*" {
		return "hourly"
	}
	if strings.HasPrefix(minute, "*/") {
		val := strings.TrimPrefix(minute, "*/")
		return fmt.Sprintf("every %s minutes", val)
	}
	if strings.HasPrefix(hour, "*/") {
		val := strings.TrimPrefix(hour, "*/")
		return fmt.Sprintf("every %s hours", val)
	}

	// Specific time
	if dom != "*" && month != "*" && minute != "*" && hour != "*" {
		return fmt.Sprintf("at %s:%s on %s/%s", hour, minute, month, dom)
	}
	if dow != "*" && minute != "*" && hour != "*" {
		return fmt.Sprintf("at %s:%s on %s", hour, minute, dowToDay(dow))
	}
	if minute != "*" && hour != "*" {
		return fmt.Sprintf("at %s:%s", hour, minute)
	}

	return cron
}

// dowToDay converts day of week number to name
func dowToDay(dow string) string {
	days := map[string]string{
		"0": "Sunday",
		"1": "Monday",
		"2": "Tuesday",
		"3": "Wednesday",
		"4": "Thursday",
		"5": "Friday",
		"6": "Saturday",
		"7": "Sunday",
	}
	if day, ok := days[dow]; ok {
		return day
	}
	return dow
}

// truncateString truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// RegisterCronTools registers cron tools to the registry
func RegisterCronTools(r *tool.Registry) {
	r.Register(CronCreateTool{})
	r.Register(CronDeleteTool{})
	r.Register(CronListTool{})
}

// Helper functions for extracting values from Input
func getString(in tool.Input, key string) string {
	if v, ok := in[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getBool(in tool.Input, key string) bool {
	if v, ok := in[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func getMap(in tool.Input, key string) map[string]any {
	if v, ok := in[key]; ok {
		if m, ok := v.(map[string]any); ok {
			return m
		}
	}
	return nil
}