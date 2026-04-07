package services

// LSP Server Instance - manages lifecycle of a single LSP server.
// Ported from src/services/lsp/LSPServerInstance.ts

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// LSP error code for "content modified" - transient error
const LSPErrorContentModified = -32801

// Max retries for transient LSP errors
const MaxRetriesForTransientErrors = 3

// Base delay for exponential backoff on transient errors (500ms, 1000ms, 2000ms)
const RetryBaseDelayMs = 500

// LspServerState represents the state of an LSP server.
// Ported from src/services/lsp/types.ts:LspServerState
type LspServerState string

const (
	ServerStateStopped  LspServerState = "stopped"
	ServerStateStarting LspServerState = "starting"
	ServerStateRunning  LspServerState = "running"
	ServerStateStopping LspServerState = "stopping"
	ServerStateError    LspServerState = "error"
)

// ScopedLspServerConfig represents configuration for an LSP server.
// Ported from src/services/lsp/types.ts:ScopedLspServerConfig
type ScopedLspServerConfig struct {
	Command              string            `json:"command"`
	Args                 []string          `json:"args,omitempty"`
	Env                  map[string]string `json:"env,omitempty"`
	WorkspaceFolder      string            `json:"workspaceFolder,omitempty"`
	ExtensionToLanguage  map[string]string `json:"extensionToLanguage"`
	InitializationOptions interface{}       `json:"initializationOptions,omitempty"`
	StartupTimeout       int               `json:"startupTimeout,omitempty"` // milliseconds
	MaxRestarts          int               `json:"maxRestarts,omitempty"`    // default 3
}

// LSPServerInstance manages a single LSP server lifecycle.
// Ported from src/services/lsp/LSPServerInstance.ts:LSPServerInstance
type LSPServerInstance struct {
	mu sync.Mutex

	name         string
	config       ScopedLspServerConfig
	state        LspServerState
	startTime    *time.Time
	lastError    error
	restartCount int
	crashRecoveryCount int
	client       LSPClient
}

// CreateLSPServerInstance creates a new LSP server instance.
// Ported from src/services/lsp/LSPServerInstance.ts:createLSPServerInstance
func CreateLSPServerInstance(name string, config ScopedLspServerConfig) *LSPServerInstance {
	instance := &LSPServerInstance{
		name:   name,
		config: config,
		state:  ServerStateStopped,
	}

	// Create client with crash handler
	instance.client = CreateLSPClient(name, func(err error) {
		instance.mu.Lock()
		instance.state = ServerStateError
		instance.lastError = err
		instance.crashRecoveryCount++
		instance.mu.Unlock()
	})

	return instance
}

// Name returns the server name.
func (s *LSPServerInstance) Name() string {
	return s.name
}

// Config returns the server configuration.
func (s *LSPServerInstance) Config() ScopedLspServerConfig {
	return s.config
}

// State returns the current server state.
func (s *LSPServerInstance) State() LspServerState {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state
}

// StartTime returns when the server was last started.
func (s *LSPServerInstance) StartTime() *time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.startTime
}

// LastError returns the last error encountered.
func (s *LSPServerInstance) LastError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastError
}

// RestartCount returns the number of restarts.
func (s *LSPServerInstance) RestartCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.restartCount
}

// Start starts the LSP server.
// Ported from src/services/lsp/LSPServerInstance.ts:start
func (s *LSPServerInstance) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == ServerStateRunning || s.state == ServerStateStarting {
		return nil
	}

	// Cap crash-recovery attempts
	maxRestarts := s.config.MaxRestarts
	if maxRestarts == 0 {
		maxRestarts = 3
	}

	if s.state == ServerStateError && s.crashRecoveryCount > maxRestarts {
		err := fmt.Errorf("LSP server '%s' exceeded max crash recovery attempts (%d)", s.name, maxRestarts)
		s.lastError = err
		return err
	}

	s.state = ServerStateStarting

	// Start the client
	options := &LSPStartOptions{
		Env: s.config.Env,
		Cwd: s.config.WorkspaceFolder,
	}

	if err := s.client.Start(ctx, s.config.Command, s.config.Args, options); err != nil {
		s.state = ServerStateError
		s.lastError = err
		return fmt.Errorf("LSP server '%s' failed to start: %w", s.name, err)
	}

	// Build initialize params
	workspaceFolder := s.config.WorkspaceFolder
	if workspaceFolder == "" {
		workspaceFolder = "." // Use current directory
	}

	workspaceUri := "file://" + workspaceFolder

	initParams := InitializeParams{
		ProcessId:             -1, // TODO: get actual process ID
		InitializationOptions: s.config.InitializationOptions,
		WorkspaceFolders: []WorkspaceFolder{
			{
				Uri:  workspaceUri,
				Name: workspaceFolder,
			},
		},
		RootPath: workspaceFolder, // Deprecated but needed by some servers
		RootUri:  workspaceUri,    // Deprecated but needed by some servers
		Capabilities: ClientCapabilities{
			Workspace: WorkspaceCapabilities{
				Configuration:    false,
				WorkspaceFolders: false,
			},
			TextDocument: TextDocumentCapabilities{
				Synchronization: SynchronizationCapabilities{
					DynamicRegistration: false,
					WillSave:            false,
					WillSaveWaitUntil:   false,
					DidSave:             true,
				},
				PublishDiagnostics: PublishDiagnosticsCapabilities{
					RelatedInformation: true,
					TagSupport: TagSupport{
						ValueSet: []int{1, 2}, // Unnecessary, Deprecated
					},
					VersionSupport:  false,
					CodeDescription: true,
					DataSupport:     false,
				},
				Hover: HoverCapabilities{
					DynamicRegistration: false,
					ContentFormat:       []string{"markdown", "plaintext"},
				},
				Definition: DefinitionCapabilities{
					DynamicRegistration: false,
					LinkSupport:         true,
				},
				References: ReferencesCapabilities{
					DynamicRegistration: false,
				},
				DocumentSymbol: DocumentSymbolCapabilities{
					DynamicRegistration:        false,
					HierarchicalDocumentSymbol: true,
				},
			},
			General: GeneralCapabilities{
				PositionEncodings: []string{"utf-16"},
			},
		},
	}

	// Initialize with timeout
	initCtx := ctx
	if s.config.StartupTimeout > 0 {
		timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(s.config.StartupTimeout)*time.Millisecond)
		defer cancel()
		initCtx = timeoutCtx
	}

	_, err := s.client.Initialize(initCtx, initParams)
	if err != nil {
		// Clean up on timeout/error
		s.client.Stop(ctx)
		s.state = ServerStateError
		s.lastError = err
		return fmt.Errorf("LSP server '%s' initialization failed: %w", s.name, err)
	}

	now := time.Now()
	s.startTime = &now
	s.state = ServerStateRunning
	s.crashRecoveryCount = 0

	return nil
}

// Stop stops the LSP server gracefully.
// Ported from src/services/lsp/LSPServerInstance.ts:stop
func (s *LSPServerInstance) Stop(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == ServerStateStopped || s.state == ServerStateStopping {
		return nil
	}

	s.state = ServerStateStopping

	err := s.client.Stop(ctx)
	if err != nil {
		s.state = ServerStateError
		s.lastError = err
		return err
	}

	s.state = ServerStateStopped
	s.startTime = nil

	return nil
}

// Restart manually restarts the server.
// Ported from src/services/lsp/LSPServerInstance.ts:restart
func (s *LSPServerInstance) Restart(ctx context.Context) error {
	// Stop first
	if err := s.Stop(ctx); err != nil {
		return fmt.Errorf("failed to stop LSP server '%s' during restart: %w", s.name, err)
	}

	s.mu.Lock()
	s.restartCount++
	s.mu.Unlock()

	// Check max restarts
	maxRestarts := s.config.MaxRestarts
	if maxRestarts == 0 {
		maxRestarts = 3
	}

	if s.restartCount > maxRestarts {
		return fmt.Errorf("max restart attempts (%d) exceeded for server '%s'", maxRestarts, s.name)
	}

	// Start again
	if err := s.Start(ctx); err != nil {
		return fmt.Errorf("failed to start LSP server '%s' during restart (attempt %d/%d): %w", s.name, s.restartCount, maxRestarts, err)
	}

	return nil
}

// IsHealthy checks if the server is healthy and ready.
// Ported from src/services/lsp/LSPServerInstance.ts:isHealthy
func (s *LSPServerInstance) IsHealthy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.state == ServerStateRunning && s.client.IsInitialized()
}

// SendRequest sends a request with retry logic for transient errors.
// Ported from src/services/lsp/LSPServerInstance.ts:sendRequest
func (s *LSPServerInstance) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	if !s.IsHealthy() {
		errMsg := fmt.Sprintf("Cannot send request to LSP server '%s': server is %s", s.name, s.State())
		if s.lastError != nil {
			errMsg += fmt.Sprintf(", last error: %v", s.lastError)
		}
		return nil, fmt.Errorf(errMsg)
	}

	var lastAttemptError error

	for attempt := 0; attempt <= MaxRetriesForTransientErrors; attempt++ {
		result, err := s.client.SendRequest(ctx, method, params)
		if err == nil {
			return result, nil
		}

		lastAttemptError = err

		// Check if this is a transient "content modified" error
		// Use duck typing to check for error code
		errorCode := extractErrorCode(err)
		if errorCode == LSPErrorContentModified && attempt < MaxRetriesForTransientErrors {
			delay := RetryBaseDelayMs * (1 << attempt) // 500, 1000, 2000
			time.Sleep(time.Duration(delay) * time.Millisecond)
			continue
		}

		// Non-retryable error or max retries exceeded
		break
	}

	return nil, fmt.Errorf("LSP request '%s' failed for server '%s': %w", method, s.name, lastAttemptError)
}

// SendNotification sends a notification to the server.
// Ported from src/services/lsp/LSPServerInstance.ts:sendNotification
func (s *LSPServerInstance) SendNotification(ctx context.Context, method string, params interface{}) error {
	if !s.IsHealthy() {
		return fmt.Errorf("cannot send notification to LSP server '%s': server is %s", s.name, s.State())
	}

	err := s.client.SendNotification(ctx, method, params)
	if err != nil {
		return fmt.Errorf("LSP notification '%s' failed for server '%s': %w", method, s.name, err)
	}

	return nil
}

// OnNotification registers a handler for notifications from the server.
func (s *LSPServerInstance) OnNotification(method string, handler func(interface{})) {
	s.client.OnNotification(method, handler)
}

// OnRequest registers a handler for requests from the server.
func (s *LSPServerInstance) OnRequest(method string, handler func(interface{}) (interface{}, error)) {
	s.client.OnRequest(method, handler)
}

// extractErrorCode extracts LSP error code from an error.
func extractErrorCode(err error) int {
	// Try to parse error message for code
	errMsg := err.Error()

	// Look for pattern "(code X)"
	if idx := strings.Index(errMsg, "(code "); idx >= 0 {
		rest := errMsg[idx+6:]
		code := 0
		for _, ch := range rest {
			if ch >= '0' && ch <= '9' {
				code = code*10 + int(ch-'0')
			} else if ch == ')' {
				break
			} else {
				break
			}
		}
		return code
	}

	return 0
}