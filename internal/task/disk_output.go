package task

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"claude-go/internal/utils"
)

// ShellTaskType represents the type of shell task
type ShellTaskType string

const (
	ShellTaskTypeBash   ShellTaskType = "local_bash"
	ShellTaskTypeMonitor ShellTaskType = "monitor"
)

// ShellTaskStatus represents the status of a shell task
type ShellTaskStatus string

const (
	ShellTaskStatusRunning     ShellTaskStatus = "running"
	ShellTaskStatusCompleted   ShellTaskStatus = "completed"
	ShellTaskStatusFailed      ShellTaskStatus = "failed"
	ShellTaskStatusInterrupted ShellTaskStatus = "interrupted"
	ShellTaskStatusKilled      ShellTaskStatus = "killed"
)

// ShellTaskState represents the state of a background shell task
// Ported from src/tasks/LocalShellTask/guards.ts:LocalShellTaskState
type ShellTaskState struct {
	ID          string         `json:"id"`
	Type        ShellTaskType  `json:"type"`
	Status      ShellTaskStatus `json:"status"`
	Command     string         `json:"command"`
	Description string         `json:"description"`
	StartTime   time.Time      `json:"start_time"`
	EndTime     time.Time      `json:"end_time,omitempty"`
	ExitCode    int            `json:"exit_code,omitempty"`
	Interrupted bool           `json:"interrupted"`
	OutputPath  string         `json:"output_path"`
	OutputSize  int64          `json:"output_size"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// ShellTaskResult represents the result of a shell task
type ShellTaskResult struct {
	Code        int    `json:"code"`
	Stdout      string `json:"stdout"`
	Stderr      string `json:"stderr"`
	Interrupted bool   `json:"interrupted"`
}

// DiskTaskOutput manages task output stored on disk
// Ported from src/utils/task/diskOutput.ts
type DiskTaskOutput struct {
	mu        sync.Mutex
	outputDir string
	sessionID string
}

// CreateDiskTaskOutput creates a new disk task output manager
// Ported from src/utils/task/diskOutput.ts:getTaskOutputDir
func CreateDiskTaskOutput(sessionDir, sessionID string) *DiskTaskOutput {
	outputDir := filepath.Join(sessionDir, "tasks", sanitizePathComponent(sessionID))
	return &DiskTaskOutput{
		outputDir: outputDir,
		sessionID: sessionID,
	}
}

// GetTaskOutputPath returns the path for a task's output file
// Ported from src/utils/task/diskOutput.ts:getTaskOutputPath
func (d *DiskTaskOutput) GetTaskOutputPath(taskID string) string {
	return filepath.Join(d.outputDir, sanitizePathComponent(taskID)+".output")
}

// GetTaskMetaPath returns the path for a task's metadata file
func (d *DiskTaskOutput) GetTaskMetaPath(taskID string) string {
	return filepath.Join(d.outputDir, sanitizePathComponent(taskID)+".json")
}

// EnsureDir ensures the output directory exists
func (d *DiskTaskOutput) EnsureDir() error {
	return os.MkdirAll(d.outputDir, 0755)
}

// WriteOutput appends output to a task's output file
func (d *DiskTaskOutput) WriteOutput(taskID string, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.EnsureDir(); err != nil {
		return err
	}

	path := d.GetTaskOutputPath(taskID)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(data)
	return err
}

// ReadOutput reads the output from a task's output file
// Ported from src/utils/task/diskOutput.ts:getTaskOutput
func (d *DiskTaskOutput) ReadOutput(taskID string, maxBytes int64) (string, error) {
	path := d.GetTaskOutputPath(taskID)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}

	// Cap at maxBytes (tail from end)
	if maxBytes > 0 && int64(len(data)) > maxBytes {
		start := int64(len(data)) - maxBytes
		data = data[start:]
	}

	return string(data), nil
}

// ReadOutputDelta reads output from a given offset
// Ported from src/utils/task/diskOutput.ts:getTaskOutputDelta
func (d *DiskTaskOutput) ReadOutputDelta(taskID string, fromOffset int64, maxBytes int64) (string, int64, error) {
	path := d.GetTaskOutputPath(taskID)

	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fromOffset, nil
		}
		return "", fromOffset, err
	}
	defer f.Close()

	// Get file size
	stat, err := f.Stat()
	if err != nil {
		return "", fromOffset, err
	}
	fileSize := stat.Size()

	// Read from offset
	if fromOffset >= fileSize {
		return "", fileSize, nil
	}

	remaining := fileSize - fromOffset
	toRead := remaining
	if maxBytes > 0 && remaining > maxBytes {
		toRead = maxBytes
	}

	f.Seek(fromOffset, 0)
	buf := make([]byte, toRead)
	n, err := f.Read(buf)
	if err != nil {
		return "", fromOffset, err
	}

	return string(buf[:n]), fromOffset + int64(n), nil
}

// GetOutputSize returns the size of a task's output file
// Ported from src/utils/task/diskOutput.ts:getTaskOutputSize
func (d *DiskTaskOutput) GetOutputSize(taskID string) (int64, error) {
	path := d.GetTaskOutputPath(taskID)

	stat, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}

	return stat.Size(), nil
}

// SaveTaskMeta saves the task metadata
func (d *DiskTaskOutput) SaveTaskMeta(state *ShellTaskState) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if err := d.EnsureDir(); err != nil {
		return err
	}

	state.OutputPath = d.GetTaskOutputPath(state.ID)
	state.UpdatedAt = time.Now()

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}

	path := d.GetTaskMetaPath(state.ID)
	return os.WriteFile(path, data, 0644)
}

// LoadTaskMeta loads the task metadata
func (d *DiskTaskOutput) LoadTaskMeta(taskID string) (*ShellTaskState, error) {
	path := d.GetTaskMetaPath(taskID)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("task not found: %s", taskID)
		}
		return nil, err
	}

	var state ShellTaskState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// DeleteTask deletes a task's output and metadata
// Ported from src/utils/task/diskOutput.ts:evictTaskOutput
func (d *DiskTaskOutput) DeleteTask(taskID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	outputPath := d.GetTaskOutputPath(taskID)
	metaPath := d.GetTaskMetaPath(taskID)

	// Remove output file
	if err := os.Remove(outputPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Remove metadata file
	if err := os.Remove(metaPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

// ListTasks lists all task IDs with output files
func (d *DiskTaskOutput) ListTasks() ([]string, error) {
	entries, err := os.ReadDir(d.outputDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var taskIDs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Extract task ID from .json metadata files
		if filepath.Ext(name) == ".json" {
			taskID := name[:len(name)-5]
			taskIDs = append(taskIDs, taskID)
		}
	}

	return taskIDs, nil
}

// GenerateShellTaskID generates a unique task ID for shell tasks
// Ported from src/Task.ts:generateTaskId
func GenerateShellTaskID() string {
	id, _ := utils.GenerateID("b", 8) // 'b' prefix for local_bash
	return id
}

// MaxTaskOutputBytes is the maximum size for task output
// Ported from src/utils/task/diskOutput.ts:MAX_TASK_OUTPUT_BYTES
const MaxTaskOutputBytes = 5 * 1024 * 1024 * 1024 // 5 GB