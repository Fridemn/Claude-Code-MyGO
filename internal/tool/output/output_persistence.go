package output


import (
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"claude-code-go/internal/bootstrap"
)

// OutputPersistence manages large output persistence to disk
type OutputPersistence struct {
	mu          sync.Mutex
	store       *bootstrap.Store
	toolResultsDir string
	maxPersistedSize int64
}

// OutputPersistenceConfig holds configuration for output persistence
type OutputPersistenceConfig struct {
	ToolResultsDir string
	MaxPersistedSize int64 // Maximum size for persisted output (default 64MB)
}

// DefaultOutputPersistenceConfig returns default configuration
func DefaultOutputPersistenceConfig() OutputPersistenceConfig {
	return OutputPersistenceConfig{
		ToolResultsDir: ".claude/tool-results",
		MaxPersistedSize: 64 * 1024 * 1024, // 64 MB
	}
}

// CreateOutputPersistence creates a new output persistence manager
func CreateOutputPersistence(store *bootstrap.Store, cfg OutputPersistenceConfig) *OutputPersistence {
	if cfg.ToolResultsDir == "" {
		cfg.ToolResultsDir = DefaultOutputPersistenceConfig().ToolResultsDir
	}
	if cfg.MaxPersistedSize == 0 {
		cfg.MaxPersistedSize = DefaultOutputPersistenceConfig().MaxPersistedSize
	}
	return &OutputPersistence{
		store: store,
		toolResultsDir: cfg.ToolResultsDir,
		maxPersistedSize: cfg.MaxPersistedSize,
	}
}

// EnsureToolResultsDir creates the tool-results directory if it doesn't exist
func (op *OutputPersistence) EnsureToolResultsDir() error {
	op.mu.Lock()
	defer op.mu.Unlock()

	dir := op.toolResultsDir
	if op.store != nil {
		cwd := op.store.GetCWD()
		dir = filepath.Join(cwd, op.toolResultsDir)
	}

	return os.MkdirAll(dir, 0755)
}

// GetToolResultPath returns the path for a tool result file
func (op *OutputPersistence) GetToolResultPath(taskID string) string {
	op.mu.Lock()
	defer op.mu.Unlock()

	baseDir := op.toolResultsDir
	if op.store != nil {
		cwd := op.store.GetCWD()
		baseDir = filepath.Join(cwd, op.toolResultsDir)
	}

	return filepath.Join(baseDir, taskID+".txt")
}

// PersistOutput persists large output to disk
// Returns the path to the persisted file and its size
func (op *OutputPersistence) PersistOutput(output string, taskID string) (path string, size int64, err error) {
	// Ensure directory exists
	if err = op.EnsureToolResultsDir(); err != nil {
		return "", 0, err
	}

	// Get destination path
	dest := op.GetToolResultPath(taskID)

	// Write output to file
	data := []byte(output)
	size = int64(len(data))

	// Truncate if too large
	if size > op.maxPersistedSize {
		data = data[:op.maxPersistedSize]
		size = op.maxPersistedSize
	}

	// Write file
	if err = os.WriteFile(dest, data, 0644); err != nil {
		return "", 0, err
	}

	return dest, size, nil
}

// PersistOutputFromReader persists output from a reader
func (op *OutputPersistence) PersistOutputFromReader(r io.Reader, taskID string, maxSize int64) (path string, size int64, err error) {
	// Ensure directory exists
	if err = op.EnsureToolResultsDir(); err != nil {
		return "", 0, err
	}

	// Get destination path
	dest := op.GetToolResultPath(taskID)

	// Create file
	f, err := os.Create(dest)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	// Copy with size limit
	if maxSize == 0 {
		maxSize = op.maxPersistedSize
	}

	limitedReader := io.LimitReader(r, maxSize)
	size, err = io.Copy(f, limitedReader)
	if err != nil {
		return "", 0, err
	}

	return dest, size, nil
}

// ReadPersistedOutput reads persisted output from disk
func (op *OutputPersistence) ReadPersistedOutput(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// CleanupToolResults removes old tool result files
func (op *OutputPersistence) CleanupToolResults(maxAge int64) error {
	op.mu.Lock()
	defer op.mu.Unlock()

	baseDir := op.toolResultsDir
	if op.store != nil {
		cwd := op.store.GetCWD()
		baseDir = filepath.Join(cwd, op.toolResultsDir)
	}

	// Read directory
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	now := time.Now().Unix()
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Check age
		if now-info.ModTime().Unix() > maxAge {
			filePath := filepath.Join(baseDir, entry.Name())
			os.Remove(filePath)
		}
	}

	return nil
}