package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
)

// TaskListStatus represents the status of a task list item
type TaskListStatus string

const (
	TaskListStatusPending    TaskListStatus = "pending"
	TaskListStatusInProgress TaskListStatus = "in_progress"
	TaskListStatusCompleted  TaskListStatus = "completed"
)

// Task represents a task in the task list
type Task struct {
	ID          string                 `json:"id"`
	Subject     string                 `json:"subject"`
	Description string                 `json:"description"`
	ActiveForm  string                 `json:"activeForm,omitempty"`
	Owner       string                 `json:"owner,omitempty"`
	Status      TaskListStatus         `json:"status"`
	Blocks      []string               `json:"blocks"`
	BlockedBy   []string               `json:"blockedBy"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// TaskListManager manages a list of tasks stored as JSON files
type TaskListManager struct {
	mu         sync.RWMutex
	tasksDir   string
	taskListID string
}

// CreateTaskListManager creates a new task list manager
func CreateTaskListManager(configDir, taskListID string) *TaskListManager {
	tasksDir := filepath.Join(configDir, "tasks", sanitizePathComponent(taskListID))
	return &TaskListManager{
		tasksDir:   tasksDir,
		taskListID: taskListID,
	}
}

func sanitizePathComponent(input string) string {
	result := make([]rune, 0, len(input))
	for _, r := range input {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			result = append(result, r)
		} else {
			result = append(result, '-')
		}
	}
	return string(result)
}

func (m *TaskListManager) ensureTasksDir() error {
	return os.MkdirAll(m.tasksDir, 0755)
}

func (m *TaskListManager) getTaskPath(taskID string) string {
	return filepath.Join(m.tasksDir, sanitizePathComponent(taskID)+".json")
}

// Create creates a new task with a unique ID
func (m *TaskListManager) Create(subject, description, activeForm string, metadata map[string]interface{}) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.ensureTasksDir(); err != nil {
		return nil, fmt.Errorf("failed to create tasks directory: %w", err)
	}

	highestID := m.findHighestTaskID()
	newID := fmt.Sprintf("%d", highestID+1)

	task := &Task{
		ID:          newID,
		Subject:     subject,
		Description: description,
		ActiveForm:  activeForm,
		Status:      TaskListStatusPending,
		Blocks:      []string{},
		BlockedBy:   []string{},
		Metadata:    metadata,
	}

	if err := m.saveTask(task); err != nil {
		return nil, err
	}

	return task, nil
}

// Get retrieves a task by ID
func (m *TaskListManager) Get(taskID string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.loadTask(taskID)
}

// List returns all tasks
func (m *TaskListManager) List() ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(m.tasksDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*Task{}, nil
		}
		return nil, err
	}

	var tasks []*Task
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		taskID := entry.Name()[:len(entry.Name())-5] // remove .json
		task, err := m.loadTask(taskID)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	// Sort by ID (numeric order)
	sort.Slice(tasks, func(i, j int) bool {
		var idI, idJ int
		fmt.Sscanf(tasks[i].ID, "%d", &idI)
		fmt.Sscanf(tasks[j].ID, "%d", &idJ)
		return idI < idJ
	})

	return tasks, nil
}

// Update updates a task
func (m *TaskListManager) Update(taskID string, updates map[string]interface{}) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, err := m.loadTask(taskID)
	if err != nil {
		return nil, err
	}

	// Apply updates
	if subject, ok := updates["subject"].(string); ok {
		task.Subject = subject
	}
	if description, ok := updates["description"].(string); ok {
		task.Description = description
	}
	if activeForm, ok := updates["activeForm"].(string); ok {
		task.ActiveForm = activeForm
	}
	if owner, ok := updates["owner"].(string); ok {
		task.Owner = owner
	}
	if status, ok := updates["status"].(TaskListStatus); ok {
		task.Status = status
	}
	if statusStr, ok := updates["status"].(string); ok {
		task.Status = TaskListStatus(statusStr)
	}
	if metadata, ok := updates["metadata"].(map[string]interface{}); ok {
		if task.Metadata == nil {
			task.Metadata = make(map[string]interface{})
		}
		for k, v := range metadata {
			if v == nil {
				delete(task.Metadata, k)
			} else {
				task.Metadata[k] = v
			}
		}
	}
	if blocks, ok := updates["blocks"].([]string); ok {
		task.Blocks = blocks
	}
	if blockedBy, ok := updates["blockedBy"].([]string); ok {
		task.BlockedBy = blockedBy
	}

	if err := m.saveTask(task); err != nil {
		return nil, err
	}

	return task, nil
}

// Delete deletes a task
func (m *TaskListManager) Delete(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.getTaskPath(taskID)
	return os.Remove(path)
}

// BlockTask establishes a dependency between tasks (fromTask blocks toTask)
func (m *TaskListManager) BlockTask(fromTaskID, toTaskID string) error {
	fromTask, err := m.Get(fromTaskID)
	if err != nil {
		return err
	}
	toTask, err := m.Get(toTaskID)
	if err != nil {
		return err
	}

	// Update fromTask: add toTask to blocks list
	shouldUpdateFrom := true
	for _, id := range fromTask.Blocks {
		if id == toTaskID {
			shouldUpdateFrom = false
			break
		}
	}
	if shouldUpdateFrom {
		fromTask.Blocks = append(fromTask.Blocks, toTaskID)
		if err := m.saveTaskLocked(fromTask); err != nil {
			return err
		}
	}

	// Update toTask: add fromTask to blockedBy list
	shouldUpdateTo := true
	for _, id := range toTask.BlockedBy {
		if id == fromTaskID {
			shouldUpdateTo = false
			break
		}
	}
	if shouldUpdateTo {
		toTask.BlockedBy = append(toTask.BlockedBy, fromTaskID)
		if err := m.saveTaskLocked(toTask); err != nil {
			return err
		}
	}

	return nil
}

func (m *TaskListManager) findHighestTaskID() int {
	entries, err := os.ReadDir(m.tasksDir)
	if err != nil {
		return 0
	}

	highest := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		taskID := entry.Name()[:len(entry.Name())-5]
		var id int
		if n, _ := fmt.Sscanf(taskID, "%d", &id); n == 1 && id > highest {
			highest = id
		}
	}
	return highest
}

func (m *TaskListManager) loadTask(taskID string) (*Task, error) {
	path := m.getTaskPath(taskID)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}
		return nil, err
	}

	var task Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("failed to parse task: %w", err)
	}

	return &task, nil
}

func (m *TaskListManager) saveTask(task *Task) error {
	return m.saveTaskLocked(task)
}

func (m *TaskListManager) saveTaskLocked(task *Task) error {
	if err := m.ensureTasksDir(); err != nil {
		return err
	}

	data, err := json.MarshalIndent(task, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	path := m.getTaskPath(task.ID)
	return os.WriteFile(path, data, 0644)
}