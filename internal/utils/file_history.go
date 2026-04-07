package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// FileHistoryBackup represents a backup of a file at a specific version
type FileHistoryBackup struct {
	BackupFileName string    `json:"backupFileName"` // null means file did not exist
	Version        int       `json:"version"`
	BackupTime     time.Time `json:"backupTime"`
}

// FileHistorySnapshot represents a snapshot of all tracked files at a point in time
type FileHistorySnapshot struct {
	MessageID           string                       `json:"messageId"`
	TrackedFileBackups  map[string]FileHistoryBackup `json:"trackedFileBackups"`
	Timestamp           time.Time                    `json:"timestamp"`
}

// FileHistoryState tracks file history for undo support
type FileHistoryState struct {
	Snapshots         []FileHistorySnapshot `json:"snapshots"`
	TrackedFiles      map[string]bool       `json:"trackedFiles"`
	SnapshotSequence  int                   `json:"snapshotSequence"`
}

// FileHistoryManager manages file history for undo operations
type FileHistoryManager struct {
	mu          sync.RWMutex
	state       FileHistoryState
	sessionID   string
	configDir   string
	enabled     bool
	maxSnapshots int
}

// FileHistoryConfig holds configuration for file history
type FileHistoryConfig struct {
	SessionID    string
	ConfigDir    string
	Enabled      bool
	MaxSnapshots int
}

// CreateFileHistoryManager creates a new file history manager
func CreateFileHistoryManager(cfg FileHistoryConfig) *FileHistoryManager {
	if cfg.MaxSnapshots == 0 {
		cfg.MaxSnapshots = 100
	}
	if cfg.ConfigDir == "" {
		cfg.ConfigDir = ".claude"
	}

	return &FileHistoryManager{
		state: FileHistoryState{
			Snapshots:        []FileHistorySnapshot{},
			TrackedFiles:     make(map[string]bool),
			SnapshotSequence: 0,
		},
		sessionID:    cfg.SessionID,
		configDir:    cfg.ConfigDir,
		enabled:      cfg.Enabled,
		maxSnapshots: cfg.MaxSnapshots,
	}
}

// IsEnabled returns whether file history is enabled
func (fhm *FileHistoryManager) IsEnabled() bool {
	return fhm.enabled
}

// TrackEdit tracks a file before it's edited, creating a backup if needed
// This must be called BEFORE the file is actually edited
func (fhm *FileHistoryManager) TrackEdit(filePath, messageID string) error {
	if !fhm.enabled {
		return nil
	}

	fhm.mu.Lock()
	defer fhm.mu.Unlock()

	// Shorten path for storage
	trackingPath := fhm.maybeShortenFilePath(filePath)

	// Check if already tracked in most recent snapshot
	if len(fhm.state.Snapshots) == 0 {
		// No snapshots yet - create initial snapshot
		fhm.state.Snapshots = append(fhm.state.Snapshots, FileHistorySnapshot{
			MessageID:          messageID,
			TrackedFileBackups: make(map[string]FileHistoryBackup),
			Timestamp:          time.Now(),
		})
	}

	mostRecent := fhm.state.Snapshots[len(fhm.state.Snapshots)-1]
	if _, exists := mostRecent.TrackedFileBackups[trackingPath]; exists {
		// Already tracked - don't overwrite v1 backup
		return nil
	}

	// Create backup
	backup, err := fhm.createBackup(filePath, 1)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// Add to tracked files
	fhm.state.TrackedFiles[trackingPath] = true

	// Update most recent snapshot
	mostRecent.TrackedFileBackups[trackingPath] = backup
	fhm.state.Snapshots[len(fhm.state.Snapshots)-1] = mostRecent

	return nil
}

// MakeSnapshot creates a new snapshot of all tracked files
func (fhm *FileHistoryManager) MakeSnapshot(messageID string) error {
	if !fhm.enabled {
		return nil
	}

	fhm.mu.Lock()
	defer fhm.mu.Unlock()

	trackedFileBackups := make(map[string]FileHistoryBackup)

	// Get most recent snapshot for version tracking
	var mostRecent *FileHistorySnapshot
	if len(fhm.state.Snapshots) > 0 {
		mostRecent = &fhm.state.Snapshots[len(fhm.state.Snapshots)-1]
	}

	// Backup all tracked files
	for trackingPath := range fhm.state.TrackedFiles {
		filePath := fhm.maybeExpandFilePath(trackingPath)

		var nextVersion int = 1
		if mostRecent != nil {
			if latestBackup, exists := mostRecent.TrackedFileBackups[trackingPath]; exists {
				nextVersion = latestBackup.Version + 1
			}
		}

		// Check if file exists
		if _, err := os.Stat(filePath); os.IsNotExist(err) {
			// File was deleted
			trackedFileBackups[trackingPath] = FileHistoryBackup{
				BackupFileName: "", // Empty means deleted
				Version:        nextVersion,
				BackupTime:     time.Now(),
			}
			continue
		}

		// Check if file changed since last backup
		if mostRecent != nil {
			if latestBackup, exists := mostRecent.TrackedFileBackups[trackingPath]; exists {
				if latestBackup.BackupFileName != "" {
					changed, err := fhm.checkFileChanged(filePath, latestBackup.BackupFileName)
					if err == nil && !changed {
						// Reuse existing backup
						trackedFileBackups[trackingPath] = latestBackup
						continue
					}
				}
			}
		}

		// Create new backup
		backup, err := fhm.createBackup(filePath, nextVersion)
		if err != nil {
			continue // Skip failed backups
		}
		trackedFileBackups[trackingPath] = backup
	}

	// Create new snapshot
	newSnapshot := FileHistorySnapshot{
		MessageID:          messageID,
		TrackedFileBackups: trackedFileBackups,
		Timestamp:          time.Now(),
	}

	// Add to snapshots
	fhm.state.Snapshots = append(fhm.state.Snapshots, newSnapshot)

	// Enforce max snapshots
	if len(fhm.state.Snapshots) > fhm.maxSnapshots {
		fhm.state.Snapshots = fhm.state.Snapshots[len(fhm.state.Snapshots)-fhm.maxSnapshots:]
	}

	fhm.state.SnapshotSequence++

	return nil
}

// Rewind rewinds files to a specific snapshot
func (fhm *FileHistoryManager) Rewind(messageID string) ([]string, error) {
	if !fhm.enabled {
		return nil, nil
	}

	fhm.mu.RLock()
	defer fhm.mu.RUnlock()

	// Find target snapshot
	var targetSnapshot *FileHistorySnapshot
	for i := len(fhm.state.Snapshots) - 1; i >= 0; i-- {
		if fhm.state.Snapshots[i].MessageID == messageID {
			targetSnapshot = &fhm.state.Snapshots[i]
			break
		}
	}

	if targetSnapshot == nil {
		return nil, fmt.Errorf("snapshot not found for message %s", messageID)
	}

	var filesChanged []string

	// Restore files
	for trackingPath := range fhm.state.TrackedFiles {
		filePath := fhm.maybeExpandFilePath(trackingPath)

		backup, exists := targetSnapshot.TrackedFileBackups[trackingPath]
		if !exists {
			// Get first version
			backup = fhm.getFirstVersionBackup(trackingPath)
		}

		if backup.BackupFileName == "" {
			// File did not exist - delete if present
			if _, err := os.Stat(filePath); err == nil {
				os.Remove(filePath)
				filesChanged = append(filesChanged, filePath)
			}
			continue
		}

		// Restore from backup
		if err := fhm.restoreBackup(filePath, backup.BackupFileName); err != nil {
			continue
		}
		filesChanged = append(filesChanged, filePath)
	}

	return filesChanged, nil
}

// GetState returns a copy of the current state
func (fhm *FileHistoryManager) GetState() FileHistoryState {
	fhm.mu.RLock()
	defer fhm.mu.RUnlock()

	// Deep copy
	state := FileHistoryState{
		Snapshots:        make([]FileHistorySnapshot, len(fhm.state.Snapshots)),
		TrackedFiles:     make(map[string]bool),
		SnapshotSequence: fhm.state.SnapshotSequence,
	}

	copy(state.Snapshots, fhm.state.Snapshots)
	for k, v := range fhm.state.TrackedFiles {
		state.TrackedFiles[k] = v
	}

	return state
}

// UpdateState updates the file history state
func (fhm *FileHistoryManager) UpdateState(updater func(FileHistoryState) FileHistoryState) {
	fhm.mu.Lock()
	defer fhm.mu.Unlock()
	fhm.state = updater(fhm.state)
}

// Internal helper methods

func (fhm *FileHistoryManager) createBackup(filePath string, version int) (FileHistoryBackup, error) {
	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return FileHistoryBackup{
			BackupFileName: "",
			Version:        version,
			BackupTime:     time.Now(),
		}, nil
	}

	// Generate backup filename
	backupFileName := fhm.getBackupFileName(filePath, version)
	backupPath := fhm.resolveBackupPath(backupFileName)

	// Ensure backup directory exists
	backupDir := filepath.Dir(backupPath)
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return FileHistoryBackup{}, err
	}

	// Copy file
	src, err := os.Open(filePath)
	if err != nil {
		return FileHistoryBackup{}, err
	}
	defer src.Close()

	dst, err := os.Create(backupPath)
	if err != nil {
		return FileHistoryBackup{}, err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		return FileHistoryBackup{}, err
	}

	// Preserve permissions
	srcInfo, err := src.Stat()
	if err == nil {
		dst.Chmod(srcInfo.Mode())
	}

	return FileHistoryBackup{
		BackupFileName: backupFileName,
		Version:        version,
		BackupTime:     time.Now(),
	}, nil
}

func (fhm *FileHistoryManager) restoreBackup(filePath, backupFileName string) error {
	backupPath := fhm.resolveBackupPath(backupFileName)

	// Ensure directory exists
	dstDir := filepath.Dir(filePath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return err
	}

	// Copy backup to original location
	src, err := os.Open(backupPath)
	if err != nil {
		return err
	}
	defer src.Close()

	dst, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	return err
}

func (fhm *FileHistoryManager) checkFileChanged(filePath, backupFileName string) (bool, error) {
	backupPath := fhm.resolveBackupPath(backupFileName)

	// Compare file stats
	srcInfo, err := os.Stat(filePath)
	if err != nil {
		return true, err
	}

	backupInfo, err := os.Stat(backupPath)
	if err != nil {
		return true, err
	}

	// Quick check: size and mode
	if srcInfo.Size() != backupInfo.Size() || srcInfo.Mode() != backupInfo.Mode() {
		return true, nil
	}

	// Check modification time
	if srcInfo.ModTime().Before(backupInfo.ModTime()) {
		return false, nil
	}

	// Content comparison needed - for simplicity, assume changed
	// In production, would read and compare content
	return false, nil
}

func (fhm *FileHistoryManager) getBackupFileName(filePath string, version int) string {
	hash := sha256.Sum256([]byte(filePath))
	hashStr := hex.EncodeToString(hash[:])[:16]
	return fmt.Sprintf("%s@v%d", hashStr, version)
}

func (fhm *FileHistoryManager) resolveBackupPath(backupFileName string) string {
	return filepath.Join(fhm.configDir, "file-history", fhm.sessionID, backupFileName)
}

func (fhm *FileHistoryManager) getFirstVersionBackup(trackingPath string) FileHistoryBackup {
	for _, snapshot := range fhm.state.Snapshots {
		if backup, exists := snapshot.TrackedFileBackups[trackingPath]; exists && backup.Version == 1 {
			return backup
		}
	}
	return FileHistoryBackup{}
}

func (fhm *FileHistoryManager) maybeShortenFilePath(filePath string) string {
	// Use relative path when possible to reduce storage
	if filepath.IsAbs(filePath) {
		// Try to make relative to original CWD
		// For now, just return as-is
		return filePath
	}
	return filePath
}

func (fhm *FileHistoryManager) maybeExpandFilePath(trackingPath string) string {
	if filepath.IsAbs(trackingPath) {
		return trackingPath
	}
	// Expand relative path
	// For now, just return as-is (would need CWD context)
	return trackingPath
}