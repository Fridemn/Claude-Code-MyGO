package task

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ShellTaskManager manages background shell tasks
// Ported from src/tasks/LocalShellTask/LocalShellTask.tsx
type ShellTaskManager struct {
	mu          sync.RWMutex
	tasks       map[string]*ShellTaskState
	diskOutput  *DiskTaskOutput
	notices     []ShellTaskNotice
}

// ShellTaskNotice represents a notification about a shell task
type ShellTaskNotice struct {
	TaskID      string
	Status      ShellTaskStatus
	Description string
	ExitCode    int
	OutputPath  string
	Summary     string
}

// CreateShellTaskManager creates a new shell task manager
func CreateShellTaskManager(sessionDir, sessionID string) *ShellTaskManager {
	return &ShellTaskManager{
		tasks:      make(map[string]*ShellTaskState),
		diskOutput: CreateDiskTaskOutput(sessionDir, sessionID),
		notices:    make([]ShellTaskNotice, 0),
	}
}

// CreateTask creates a new background shell task
// Ported from src/tasks/LocalShellTask/LocalShellTask.tsx:spawnShellTask
func (m *ShellTaskManager) CreateTask(command, description string) (*ShellTaskState, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskID := GenerateShellTaskID()

	state := &ShellTaskState{
		ID:          taskID,
		Type:        ShellTaskTypeBash,
		Status:      ShellTaskStatusRunning,
		Command:     command,
		Description: description,
		StartTime:   time.Now(),
		UpdatedAt:   time.Now(),
		OutputPath:  m.diskOutput.GetTaskOutputPath(taskID),
	}

	m.tasks[taskID] = state

	// Save metadata
	if err := m.diskOutput.SaveTaskMeta(state); err != nil {
		return nil, fmt.Errorf("failed to save task metadata: %w", err)
	}

	return state, nil
}

// UpdateTaskStatus updates the status of a task
func (m *ShellTaskManager) UpdateTaskStatus(taskID string, status ShellTaskStatus, exitCode int, interrupted bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	state.Status = status
	state.ExitCode = exitCode
	state.Interrupted = interrupted
	state.EndTime = time.Now()
	state.UpdatedAt = time.Now()

	// Get output size
	size, _ := m.diskOutput.GetOutputSize(taskID)
	state.OutputSize = size

	// Save metadata
	m.diskOutput.SaveTaskMeta(state)

	// Enqueue notification
	summary := formatTaskSummary(state)
	m.notices = append(m.notices, ShellTaskNotice{
		TaskID:      taskID,
		Status:      status,
		Description: state.Description,
		ExitCode:    exitCode,
		OutputPath:  state.OutputPath,
		Summary:     summary,
	})

	return nil
}

// GetTask retrieves a task by ID
func (m *ShellTaskManager) GetTask(taskID string) (*ShellTaskState, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.tasks[taskID]
	if !ok {
		// Try to load from disk
		state, err := m.diskOutput.LoadTaskMeta(taskID)
		if err != nil {
			return nil, err
		}
		return state, nil
	}

	return state, nil
}

// ListTasks returns all running/backgrounded tasks
func (m *ShellTaskManager) ListTasks() []*ShellTaskState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ShellTaskState, 0, len(m.tasks))
	for _, state := range m.tasks {
		result = append(result, state)
	}

	// Sort by start time
	// Simple sort since we have a small list
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].StartTime.After(result[j].StartTime) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// DrainNotices drains pending notifications
func (m *ShellTaskManager) DrainNotices() []ShellTaskNotice {
	m.mu.Lock()
	defer m.mu.Unlock()

	notices := append([]ShellTaskNotice(nil), m.notices...)
	m.notices = make([]ShellTaskNotice, 0)
	return notices
}

// WriteOutput writes output to a task's output file
func (m *ShellTaskManager) WriteOutput(taskID string, data []byte) error {
	return m.diskOutput.WriteOutput(taskID, data)
}

// ReadOutput reads output from a task's output file
func (m *ShellTaskManager) ReadOutput(taskID string, maxBytes int64) (string, error) {
	return m.diskOutput.ReadOutput(taskID, maxBytes)
}

// DeleteTask removes a task
func (m *ShellTaskManager) DeleteTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.tasks, taskID)
	return m.diskOutput.DeleteTask(taskID)
}

// KillTask kills a running task
func (m *ShellTaskManager) KillTask(ctx context.Context, taskID string, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if state.Status != ShellTaskStatusRunning {
		return nil // Already finished
	}

	state.Status = ShellTaskStatusKilled
	state.EndTime = time.Now()
	state.UpdatedAt = time.Now()

	m.diskOutput.SaveTaskMeta(state)

	m.notices = append(m.notices, ShellTaskNotice{
		TaskID:      taskID,
		Status:      ShellTaskStatusKilled,
		Description: state.Description,
		Summary:     reason,
	})

	return nil
}

// formatTaskSummary creates a summary for a task notification
// Ported from src/tasks/LocalShellTask/LocalShellTask.tsx:enqueueShellNotification
func formatTaskSummary(state *ShellTaskState) string {
	switch state.Status {
	case ShellTaskStatusCompleted:
		if state.ExitCode == 0 {
			return "completed successfully"
		}
		return fmt.Sprintf("completed with exit code %d", state.ExitCode)
	case ShellTaskStatusFailed:
		return fmt.Sprintf("failed with exit code %d", state.ExitCode)
	case ShellTaskStatusInterrupted:
		return "interrupted"
	case ShellTaskStatusKilled:
		return "killed"
	default:
		return string(state.Status)
	}
}

// FormatTaskNotification formats a task notification as XML
// Ported from src/tasks/LocalShellTask/LocalShellTask.tsx:enqueueShellNotification
func FormatTaskNotification(notice ShellTaskNotice) string {
	return fmt.Sprintf(`<task_notification>
<task_id>%s</task_id>
<output_file>%s</output_file>
<status>%s</status>
<summary>%s</summary>
</task_notification>`,
		notice.TaskID,
		notice.OutputPath,
		notice.Status,
		notice.Summary)
}