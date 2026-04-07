package services

// LSP Diagnostic Registry - stores and deduplicates LSP diagnostics.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts

import (
	"encoding/json"
	"sync"
	"time"
)

// Volume limiting constants
const (
	MaxDiagnosticsPerFile = 10
	MaxTotalDiagnostics   = 30
	MaxDeliveredFiles     = 500
)

// Diagnostic represents a single LSP diagnostic.
type Diagnostic struct {
	Message  string      `json:"message"`
	Severity string      `json:"severity,omitempty"` // "Error", "Warning", "Info", "Hint"
	Range    interface{} `json:"range,omitempty"`
	Source   string      `json:"source,omitempty"`
	Code     interface{} `json:"code,omitempty"`
}

// DiagnosticFile represents diagnostics for a file.
type DiagnosticFile struct {
	Uri         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// PendingLSPDiagnostic represents a pending diagnostic notification.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:PendingLSPDiagnostic
type PendingLSPDiagnostic struct {
	ServerName      string
	Files           []DiagnosticFile
	Timestamp       time.Time
	AttachmentSent  bool
}

// LSPDiagnosticRegistry stores and manages LSP diagnostics.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts
type LSPDiagnosticRegistry struct {
	mu sync.Mutex

	pendingDiagnostics   map[string]*PendingLSPDiagnostic
	deliveredDiagnostics *LRUCache // file URI -> set of diagnostic keys
}

// LRUCache is a simple LRU cache implementation.
type LRUCache struct {
	maxSize int
	data    map[string]interface{}
	order   []string // For LRU tracking
}

// NewLRUCache creates a new LRU cache.
func NewLRUCache(maxSize int) *LRUCache {
	return &LRUCache{
		maxSize: maxSize,
		data:    make(map[string]interface{}),
		order:   make([]string, 0),
	}
}

// Get retrieves a value from the cache.
func (c *LRUCache) Get(key string) interface{} {
	val, exists := c.data[key]
	if !exists {
		return nil
	}
	// Move to end (most recently used)
	c.touch(key)
	return val
}

// Set stores a value in the cache.
func (c *LRUCache) Set(key string, value interface{}) {
	if c.data[key] != nil {
		c.touch(key)
		c.data[key] = value
		return
	}

	// Evict oldest if at capacity
	if len(c.data) >= c.maxSize {
		oldest := c.order[0]
		c.order = c.order[1:]
		delete(c.data, oldest)
	}

	c.data[key] = value
	c.order = append(c.order, key)
}

// Has checks if a key exists.
func (c *LRUCache) Has(key string) bool {
	return c.data[key] != nil
}

// Delete removes a key.
func (c *LRUCache) Delete(key string) {
	delete(c.data, key)
	// Remove from order
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			break
		}
	}
}

// Clear removes all entries.
func (c *LRUCache) Clear() {
	c.data = make(map[string]interface{})
	c.order = make([]string, 0)
}

// touch moves a key to the end (most recently used).
func (c *LRUCache) touch(key string) {
	for i, k := range c.order {
		if k == key {
			c.order = append(c.order[:i], c.order[i+1:]...)
			c.order = append(c.order, key)
			break
		}
	}
}

// NewLSPDiagnosticRegistry creates a new diagnostic registry.
func NewLSPDiagnosticRegistry() *LSPDiagnosticRegistry {
	return &LSPDiagnosticRegistry{
		pendingDiagnostics:   make(map[string]*PendingLSPDiagnostic),
		deliveredDiagnostics: NewLRUCache(MaxDeliveredFiles),
	}
}

// RegisterPendingLSPDiagnostic registers diagnostics from a server.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:registerPendingLSPDiagnostic
func (r *LSPDiagnosticRegistry) RegisterPendingLSPDiagnostic(serverName string, files []DiagnosticFile) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate unique ID
	diagnosticId := generateDiagnosticId()

	r.pendingDiagnostics[diagnosticId] = &PendingLSPDiagnostic{
		ServerName:     serverName,
		Files:          files,
		Timestamp:      time.Now(),
		AttachmentSent: false,
	}
}

// generateDiagnosticId generates a unique diagnostic ID.
func generateDiagnosticId() string {
	return time.Now().Format("20060102150405") + "-" + randomString(8)
}

// randomString generates a random string of given length.
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[i%len(charset)]
	}
	return string(b)
}

// CheckForLSPDiagnostics retrieves pending diagnostics.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:checkForLSPDiagnostics
func (r *LSPDiagnosticRegistry) CheckForLSPDiagnostics() []struct {
	ServerName string
	Files      []DiagnosticFile
} {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Collect all pending files
	var allFiles []DiagnosticFile
	var serverNames []string
	var diagnosticsToMark []*PendingLSPDiagnostic

	for _, diagnostic := range r.pendingDiagnostics {
		if !diagnostic.AttachmentSent {
			allFiles = append(allFiles, diagnostic.Files...)
			serverNames = append(serverNames, diagnostic.ServerName)
			diagnosticsToMark = append(diagnosticsToMark, diagnostic)
		}
	}

	if len(allFiles) == 0 {
		return nil
	}

	// Deduplicate
	dedupedFiles := r.deduplicateDiagnosticFiles(allFiles)

	// Mark as sent and delete
	for _, diagnostic := range diagnosticsToMark {
		diagnostic.AttachmentSent = true
	}

	for id, diagnostic := range r.pendingDiagnostics {
		if diagnostic.AttachmentSent {
			delete(r.pendingDiagnostics, id)
		}
	}

	// Apply volume limiting
	dedupedFiles = r.applyVolumeLimiting(dedupedFiles)

	// Track delivered diagnostics
	for _, file := range dedupedFiles {
		delivered := r.deliveredDiagnostics.Get(file.Uri)
		if delivered == nil {
			delivered = make(map[string]bool)
			r.deliveredDiagnostics.Set(file.Uri, delivered)
		}

		deliveredSet := delivered.(map[string]bool)
		for _, diag := range file.Diagnostics {
			key := r.createDiagnosticKey(diag)
			deliveredSet[key] = true
		}
	}

	if len(dedupedFiles) == 0 {
		return nil
	}

	// Build result
	result := make([]struct {
		ServerName string
		Files      []DiagnosticFile
	}, 1)

	// Join server names
	serverNameStr := ""
	for i, name := range serverNames {
		if i > 0 {
			serverNameStr += ", "
		}
		serverNameStr += name
	}

	result[0].ServerName = serverNameStr
	result[0].Files = dedupedFiles

	return result
}

// deduplicateDiagnosticFiles deduplicates diagnostics by file and content.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:deduplicateDiagnosticFiles
func (r *LSPDiagnosticRegistry) deduplicateDiagnosticFiles(allFiles []DiagnosticFile) []DiagnosticFile {
	fileMap := make(map[string]map[string]bool)
	dedupedFiles := make([]DiagnosticFile, 0)

	for _, file := range allFiles {
		if fileMap[file.Uri] == nil {
			fileMap[file.Uri] = make(map[string]bool)
			dedupedFiles = append(dedupedFiles, DiagnosticFile{
				Uri:         file.Uri,
				Diagnostics: make([]Diagnostic, 0),
			})
		}

		seenDiagnostics := fileMap[file.Uri]
		var dedupedFile *DiagnosticFile
		for i := range dedupedFiles {
			if dedupedFiles[i].Uri == file.Uri {
				dedupedFile = &dedupedFiles[i]
				break
			}
		}

		// Get previously delivered diagnostics for cross-turn dedup
		previouslyDelivered := r.deliveredDiagnostics.Get(file.Uri)
		previouslyDeliveredSet := make(map[string]bool)
		if previouslyDelivered != nil {
			previouslyDeliveredSet = previouslyDelivered.(map[string]bool)
		}

		for _, diag := range file.Diagnostics {
			key := r.createDiagnosticKey(diag)

			// Skip if already seen or previously delivered
			if seenDiagnostics[key] || previouslyDeliveredSet[key] {
				continue
			}

			seenDiagnostics[key] = true
			dedupedFile.Diagnostics = append(dedupedFile.Diagnostics, diag)
		}
	}

	// Filter out files with no diagnostics
	result := make([]DiagnosticFile, 0)
	for _, file := range dedupedFiles {
		if len(file.Diagnostics) > 0 {
			result = append(result, file)
		}
	}

	return result
}

// applyVolumeLimiting applies volume limits to diagnostics.
func (r *LSPDiagnosticRegistry) applyVolumeLimiting(files []DiagnosticFile) []DiagnosticFile {
	totalDiagnostics := 0
	truncatedCount := 0

	for i := range files {
		// Sort by severity (Error=1 < Warning=2 < Info=3 < Hint=4)
		files[i].Diagnostics = r.sortDiagnosticsBySeverity(files[i].Diagnostics)

		// Cap per file
		if len(files[i].Diagnostics) > MaxDiagnosticsPerFile {
			truncatedCount += len(files[i].Diagnostics) - MaxDiagnosticsPerFile
			files[i].Diagnostics = files[i].Diagnostics[:MaxDiagnosticsPerFile]
		}

		// Cap total
		remainingCapacity := MaxTotalDiagnostics - totalDiagnostics
		if len(files[i].Diagnostics) > remainingCapacity {
			truncatedCount += len(files[i].Diagnostics) - remainingCapacity
			files[i].Diagnostics = files[i].Diagnostics[:remainingCapacity]
		}

		totalDiagnostics += len(files[i].Diagnostics)
	}

	// Filter out files with no diagnostics
	result := make([]DiagnosticFile, 0)
	for _, file := range files {
		if len(file.Diagnostics) > 0 {
			result = append(result, file)
		}
	}

	return result
}

// sortDiagnosticsBySeverity sorts diagnostics by severity.
func (r *LSPDiagnosticRegistry) sortDiagnosticsBySeverity(diagnostics []Diagnostic) []Diagnostic {
	// Sort by severity (Error=1 < Warning=2 < Info=3 < Hint=4)
	result := make([]Diagnostic, len(diagnostics))
	copy(result, diagnostics)

	// Simple bubble sort by severity number
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if severityToNumber(result[i].Severity) > severityToNumber(result[j].Severity) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// severityToNumber maps severity to numeric value.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:severityToNumber
func severityToNumber(severity string) int {
	switch severity {
	case "Error":
		return 1
	case "Warning":
		return 2
	case "Info":
		return 3
	case "Hint":
		return 4
	default:
		return 4
	}
}

// createDiagnosticKey creates a unique key for a diagnostic.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:createDiagnosticKey
func (r *LSPDiagnosticRegistry) createDiagnosticKey(diag Diagnostic) string {
	key := map[string]interface{}{
		"message":  diag.Message,
		"severity": diag.Severity,
		"range":    diag.Range,
		"source":   diag.Source,
		"code":     diag.Code,
	}

	keyJSON, _ := json.Marshal(key)
	return string(keyJSON)
}

// ClearAllLSPDiagnostics clears all pending diagnostics.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:clearAllLSPDiagnostics
func (r *LSPDiagnosticRegistry) ClearAllLSPDiagnostics() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pendingDiagnostics = make(map[string]*PendingLSPDiagnostic)
}

// ResetAllLSPDiagnosticState resets all diagnostic state.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:resetAllLSPDiagnosticState
func (r *LSPDiagnosticRegistry) ResetAllLSPDiagnosticState() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pendingDiagnostics = make(map[string]*PendingLSPDiagnostic)
	r.deliveredDiagnostics.Clear()
}

// ClearDeliveredDiagnosticsForFile clears delivered diagnostics for a file.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:clearDeliveredDiagnosticsForFile
func (r *LSPDiagnosticRegistry) ClearDeliveredDiagnosticsForFile(fileUri string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.deliveredDiagnostics.Delete(fileUri)
}

// GetPendingLSPDiagnosticCount returns count of pending diagnostics.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:getPendingLSPDiagnosticCount
func (r *LSPDiagnosticRegistry) GetPendingLSPDiagnosticCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.pendingDiagnostics)
}