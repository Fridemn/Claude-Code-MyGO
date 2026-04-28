package services

// LSP Server Manager - manages multiple LSP servers and routes requests.
// Ported from src/services/lsp/LSPServerManager.ts

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// LSPServerManager manages multiple LSP server instances.
// Ported from src/services/lsp/LSPServerManager.ts:LSPServerManager
type LSPServerManager struct {
	mu sync.Mutex

	servers       map[string]*LSPServerInstance
	extensionMap  map[string][]string // extension -> server names
	openedFiles   map[string]string   // file URI -> server name
	serverConfigs map[string]ScopedLspServerConfig
	diagnostics   *LSPDiagnosticRegistry
}

// CreateLSPServerManager creates a new LSP server manager.
// Ported from src/services/lsp/LSPServerManager.ts:createLSPServerManager
func CreateLSPServerManager() *LSPServerManager {
	return &LSPServerManager{
		servers:       make(map[string]*LSPServerInstance),
		extensionMap:  make(map[string][]string),
		openedFiles:   make(map[string]string),
		serverConfigs: make(map[string]ScopedLspServerConfig),
		diagnostics:   NewLSPDiagnosticRegistry(),
	}
}

// Diagnostics returns the diagnostic registry.
func (m *LSPServerManager) Diagnostics() *LSPDiagnosticRegistry {
	return m.diagnostics
}

// Initialize initializes the manager with server configurations.
// Ported from src/services/lsp/LSPServerManager.ts:initialize
func (m *LSPServerManager) Initialize(ctx context.Context, configs map[string]ScopedLspServerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.serverConfigs = configs

	for serverName, config := range configs {
		// Validate config
		if config.Command == "" {
			continue // Skip invalid config
		}
		if len(config.ExtensionToLanguage) == 0 {
			continue // Skip if no extensions
		}

		// Map file extensions to this server
		for ext := range config.ExtensionToLanguage {
			normalized := strings.ToLower(ext)
			if !strings.HasPrefix(normalized, ".") {
				normalized = "." + normalized
			}

			if m.extensionMap[normalized] == nil {
				m.extensionMap[normalized] = make([]string, 0)
			}
			m.extensionMap[normalized] = append(m.extensionMap[normalized], serverName)
		}

		// Create server instance
		instance := CreateLSPServerInstance(serverName, config)
		m.servers[serverName] = instance

		// Register workspace/configuration handler
		instance.OnRequest("workspace/configuration", func(params interface{}) (interface{}, error) {
			// Return empty/null config for each requested item
			// This satisfies the protocol without providing actual configuration
			if paramMap, ok := params.(map[string]interface{}); ok {
				if items, ok := paramMap["items"].([]interface{}); ok {
					result := make([]interface{}, len(items))
					for i := range result {
						result[i] = nil
					}
					return result, nil
				}
			}
			return []interface{}{nil}, nil
		})

		// Register publishDiagnostics handler
		// Ported from src/services/lsp/passiveFeedback.ts
		instance.OnNotification("textDocument/publishDiagnostics", func(params interface{}) {
			m.handlePublishDiagnostics(serverName, params)
		})
	}

	return nil
}

// Shutdown shuts down all running servers.
// Ported from src/services/lsp/LSPServerManager.ts:shutdown
func (m *LSPServerManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var errors []error

	for name, server := range m.servers {
		if server.State() == ServerStateRunning || server.State() == ServerStateError {
			if err := server.Stop(ctx); err != nil {
				errors = append(errors, fmt.Errorf("%s: %w", name, err))
			}
		}
	}

	m.servers = make(map[string]*LSPServerInstance)
	m.extensionMap = make(map[string][]string)
	m.openedFiles = make(map[string]string)

	if len(errors) > 0 {
		return fmt.Errorf("failed to stop %d LSP server(s): %v", len(errors), errors)
	}

	return nil
}

// GetServerForFile returns the server instance for a file path.
// Ported from src/services/lsp/LSPServerManager.ts:getServerForFile
func (m *LSPServerManager) GetServerForFile(filePath string) *LSPServerInstance {
	m.mu.Lock()
	defer m.mu.Unlock()

	ext := strings.ToLower(filepath.Ext(filePath))
	serverNames := m.extensionMap[ext]

	if len(serverNames) == 0 {
		return nil
	}

	// Use first server
	serverName := serverNames[0]
	return m.servers[serverName]
}

// EnsureServerStarted ensures the server for a file is started.
// Ported from src/services/lsp/LSPServerManager.ts:ensureServerStarted
func (m *LSPServerManager) EnsureServerStarted(ctx context.Context, filePath string) (*LSPServerInstance, error) {
	server := m.GetServerForFile(filePath)
	if server == nil {
		return nil, nil
	}

	if server.State() == ServerStateStopped || server.State() == ServerStateError {
		if err := server.Start(ctx); err != nil {
			return nil, fmt.Errorf("failed to start LSP server for file %s: %w", filePath, err)
		}
	}

	return server, nil
}

// SendRequest sends a request to the appropriate server.
// Ported from src/services/lsp/LSPServerManager.ts:sendRequest
func (m *LSPServerManager) SendRequest(ctx context.Context, filePath string, method string, params interface{}) (interface{}, error) {
	server, err := m.EnsureServerStarted(ctx, filePath)
	if err != nil {
		return nil, err
	}
	if server == nil {
		return nil, nil
	}

	result, err := server.SendRequest(ctx, method, params)
	if err != nil {
		return nil, fmt.Errorf("LSP request failed for file %s, method '%s': %w", filePath, method, err)
	}

	return result, nil
}

// GetAllServers returns all server instances.
// Ported from src/services/lsp/LSPServerManager.ts:getAllServers
func (m *LSPServerManager) GetAllServers() map[string]*LSPServerInstance {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.servers
}

// OpenFile synchronizes file open to LSP server.
// Ported from src/services/lsp/LSPServerManager.ts:openFile
func (m *LSPServerManager) OpenFile(ctx context.Context, filePath string, content string) error {
	server, err := m.EnsureServerStarted(ctx, filePath)
	if err != nil {
		return err
	}
	if server == nil {
		return nil
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fileUri := "file://" + absPath

	m.mu.Lock()
	if m.openedFiles[fileUri] == server.Name() {
		// Already opened
		m.mu.Unlock()
		return nil
	}
	m.mu.Unlock()

	// Get language ID from server's extension mapping
	ext := strings.ToLower(filepath.Ext(filePath))
	languageId := server.Config().ExtensionToLanguage[ext]
	if languageId == "" {
		languageId = "plaintext"
	}

	// Send didOpen notification
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        fileUri,
			"languageId": languageId,
			"version":    1,
			"text":       content,
		},
	}

	err = server.SendNotification(ctx, "textDocument/didOpen", params)
	if err != nil {
		return fmt.Errorf("failed to sync file open %s: %w", filePath, err)
	}

	// Track opened file
	m.mu.Lock()
	m.openedFiles[fileUri] = server.Name()
	m.mu.Unlock()

	return nil
}

// ChangeFile synchronizes file change to LSP server.
// Ported from src/services/lsp/LSPServerManager.ts:changeFile
func (m *LSPServerManager) ChangeFile(ctx context.Context, filePath string, content string) error {
	server := m.GetServerForFile(filePath)
	if server == nil || server.State() != ServerStateRunning {
		return m.OpenFile(ctx, filePath, content)
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fileUri := "file://" + absPath

	m.mu.Lock()
	if m.openedFiles[fileUri] != server.Name() {
		// Not opened on this server - open first
		m.mu.Unlock()
		return m.OpenFile(ctx, filePath, content)
	}
	m.mu.Unlock()

	// Send didChange notification
	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     fileUri,
			"version": 1,
		},
		"contentChanges": []interface{}{
			map[string]interface{}{
				"text": content,
			},
		},
	}

	err = server.SendNotification(ctx, "textDocument/didChange", params)
	if err != nil {
		return fmt.Errorf("failed to sync file change %s: %w", filePath, err)
	}

	return nil
}

// SaveFile synchronizes file save to LSP server.
// Ported from src/services/lsp/LSPServerManager.ts:saveFile
func (m *LSPServerManager) SaveFile(ctx context.Context, filePath string) error {
	server := m.GetServerForFile(filePath)
	if server == nil || server.State() != ServerStateRunning {
		return nil
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fileUri := "file://" + absPath

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileUri,
		},
	}

	err = server.SendNotification(ctx, "textDocument/didSave", params)
	if err != nil {
		return fmt.Errorf("failed to sync file save %s: %w", filePath, err)
	}

	return nil
}

// CloseFile synchronizes file close to LSP server.
// Ported from src/services/lsp/LSPServerManager.ts:closeFile
func (m *LSPServerManager) CloseFile(ctx context.Context, filePath string) error {
	server := m.GetServerForFile(filePath)
	if server == nil || server.State() != ServerStateRunning {
		return nil
	}

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}
	fileUri := "file://" + absPath

	params := map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": fileUri,
		},
	}

	err = server.SendNotification(ctx, "textDocument/didClose", params)
	if err != nil {
		return fmt.Errorf("failed to sync file close %s: %w", filePath, err)
	}

	// Remove from tracking
	m.mu.Lock()
	delete(m.openedFiles, fileUri)
	m.mu.Unlock()

	return nil
}

// IsFileOpen checks if a file is already open.
// Ported from src/services/lsp/LSPServerManager.ts:isFileOpen
func (m *LSPServerManager) IsFileOpen(filePath string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	absPath, _ := filepath.Abs(filePath)
	fileUri := "file://" + absPath
	return m.openedFiles[fileUri] != ""
}

// handlePublishDiagnostics handles publishDiagnostics notifications from LSP servers.
// Ported from src/services/lsp/passiveFeedback.ts
func (m *LSPServerManager) handlePublishDiagnostics(serverName string, params interface{}) {
	// Parse the notification params
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		return
	}

	var publishParams PublishDiagnosticsParams
	if err := json.Unmarshal(paramsJSON, &publishParams); err != nil {
		return
	}

	// Convert to diagnostic files format
	files := formatDiagnosticsForAttachment(publishParams)

	// Register in the diagnostic registry
	if len(files) > 0 {
		m.diagnostics.RegisterPendingLSPDiagnostic(serverName, files)
	}
}

// PublishDiagnosticsParams represents LSP publishDiagnostics notification params.
// Ported from src/services/lsp/passiveFeedback.ts
type PublishDiagnosticsParams struct {
	Uri         string        `json:"uri"`
	Diagnostics []LSPDiagnostic `json:"diagnostics"`
	Version     int           `json:"version,omitempty"`
}

// LSPDiagnostic represents a single LSP diagnostic.
type LSPDiagnostic struct {
	Range     LSPRange      `json:"range"`
	Severity  int           `json:"severity,omitempty"`
	Code      interface{}   `json:"code,omitempty"`
	Source    string        `json:"source,omitempty"`
	Message   string        `json:"message"`
	Tags      []int         `json:"tags,omitempty"`
	Related   []RelatedInfo `json:"relatedInformation,omitempty"`
}

// LSPRange represents an LSP range.
type LSPRange struct {
	Start LSPPosition `json:"start"`
	End   LSPPosition `json:"end"`
}

// LSPPosition represents an LSP position.
type LSPPosition struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// RelatedInfo represents related diagnostic information.
type RelatedInfo struct {
	Location LSPLocation `json:"location"`
	Message  string      `json:"message"`
}

// LSPLocation represents an LSP location.
type LSPLocation struct {
	Uri   string    `json:"uri"`
	Range LSPRange  `json:"range"`
}

// formatDiagnosticsForAttachment converts LSP diagnostics to DiagnosticFile format.
// Ported from src/services/lsp/passiveFeedback.ts:formatDiagnosticsForAttachment
func formatDiagnosticsForAttachment(params PublishDiagnosticsParams) []DiagnosticFile {
	if len(params.Diagnostics) == 0 {
		return nil
	}

	// Convert diagnostics
	diagnostics := make([]Diagnostic, 0, len(params.Diagnostics))
	for _, lspDiag := range params.Diagnostics {
		diag := Diagnostic{
			Message:  lspDiag.Message,
			Severity: mapLSPSeverity(lspDiag.Severity),
			Range:    lspDiag.Range,
			Source:   lspDiag.Source,
			Code:     lspDiag.Code,
		}
		diagnostics = append(diagnostics, diag)
	}

	// Apply volume limiting
	diagnostics = limitDiagnostics(diagnostics)

	if len(diagnostics) == 0 {
		return nil
	}

	return []DiagnosticFile{
		{
			Uri:         params.Uri,
			Diagnostics: diagnostics,
		},
	}
}

// mapLSPSeverity maps LSP severity numbers to string names.
// Ported from src/services/lsp/passiveFeedback.ts:mapLSPSeverity
func mapLSPSeverity(severity int) string {
	switch severity {
	case 1:
		return "Error"
	case 2:
		return "Warning"
	case 3:
		return "Info"
	case 4:
		return "Hint"
	default:
		return "Error"
	}
}

// limitDiagnostics applies volume limits to diagnostics.
func limitDiagnostics(diagnostics []Diagnostic) []Diagnostic {
	if len(diagnostics) <= MaxDiagnosticsPerFile {
		return diagnostics
	}

	// Sort by severity (errors first)
	sorted := make([]Diagnostic, len(diagnostics))
	copy(sorted, diagnostics)

	// Simple sort by severity (Error=1 < Warning=2 < Info=3 < Hint=4)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if severityToNumber(sorted[i].Severity) > severityToNumber(sorted[j].Severity) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted[:MaxDiagnosticsPerFile]
}

// CheckForLSPDiagnostics retrieves pending LSP diagnostics.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:checkForLSPDiagnostics
func (m *LSPServerManager) CheckForLSPDiagnostics() []DiagnosticFile {
	result := m.diagnostics.CheckForLSPDiagnostics()
	if len(result) == 0 {
		return nil
	}

	// Flatten all files from all servers
	var allFiles []DiagnosticFile
	for _, r := range result {
		allFiles = append(allFiles, r.Files...)
	}

	return allFiles
}

// ClearDeliveredDiagnosticsForFile clears diagnostics for a file after edit.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:clearDeliveredDiagnosticsForFile
func (m *LSPServerManager) ClearDeliveredDiagnosticsForFile(fileUri string) {
	m.diagnostics.ClearDeliveredDiagnosticsForFile(fileUri)
}

// ResetAllDiagnostics resets all diagnostic state on session clear.
// Ported from src/services/lsp/LSPDiagnosticRegistry.ts:resetAllLSPDiagnosticState
func (m *LSPServerManager) ResetAllDiagnostics() {
	m.diagnostics.ResetAllLSPDiagnosticState()
}

// FormatDiagnosticAttachment formats diagnostics as an attachment.
// Ported from src/utils/attachments.ts:getLSPDiagnosticAttachments
func FormatDiagnosticAttachment(files []DiagnosticFile) string {
	if len(files) == 0 {
		return ""
	}

	var lines []string

	// Summary line
	totalDiags := 0
	for _, f := range files {
		totalDiags += len(f.Diagnostics)
	}
	lines = append(lines, fmt.Sprintf("Found %d new diagnostic issues in %d files:", totalDiags, len(files)))

	// File details
	for _, file := range files {
		// Extract file path from URI
		filePath := strings.TrimPrefix(file.Uri, "file://")
		lines = append(lines, fmt.Sprintf("\n%s:", filePath))

		for _, diag := range file.Diagnostics {
			// Get line number if available
			lineNum := 0
			if rangeData, ok := diag.Range.(map[string]interface{}); ok {
				if start, ok := rangeData["start"].(map[string]interface{}); ok {
					if line, ok := start["line"].(float64); ok {
						lineNum = int(line) + 1 // LSP lines are 0-based
					}
				}
			}

			// Format diagnostic
			sevSymbol := getSeveritySymbol(diag.Severity)
			msg := diag.Message
			if len(msg) > 100 {
				msg = msg[:100] + "..."
			}

			if lineNum > 0 {
				lines = append(lines, fmt.Sprintf("  [%d] %s %s", lineNum, sevSymbol, msg))
			} else {
				lines = append(lines, fmt.Sprintf("  %s %s", sevSymbol, msg))
			}
		}
	}

	return strings.Join(lines, "\n")
}

// getSeveritySymbol returns a symbol for the severity level.
// Ported from src/services/diagnosticTracking.ts:getSeveritySymbol
func getSeveritySymbol(severity string) string {
	switch severity {
	case "Error":
		return "✘"
	case "Warning":
		return "⚠"
	case "Info":
		return "ℹ"
	case "Hint":
		return "💡"
	default:
		return "✘"
	}
}