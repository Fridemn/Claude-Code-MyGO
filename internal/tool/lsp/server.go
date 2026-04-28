package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Server represents an LSP server instance
type Server struct {
	name      string
	config    ServerConfig
	process   *os.Process
	client    *JSONRPCClient
	state     ServerState
	mu        sync.RWMutex
	crashCount int
	maxRestarts int
	startTime  time.Time
	lastError  string
}

// NewServer creates a new LSP server
func NewServer(name string, cfg ServerConfig) *Server {
	maxRestarts := 3
	if cfg.MaxRestarts > 0 {
		maxRestarts = cfg.MaxRestarts
	}
	return &Server{
		name:        name,
		config:      cfg,
		state:       ServerStateStopped,
		maxRestarts: maxRestarts,
	}
}

// Name returns the server name
func (s *Server) Name() string { return s.name }

// State returns the current server state
func (s *Server) State() ServerState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Capabilities returns the server capabilities
func (s *Server) Capabilities() *ServerCapabilities {
	// This would be set during initialization
	return nil
}

// CrashCount returns the number of crashes
func (s *Server) CrashCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.crashCount
}

// LastError returns the last error message
func (s *Server) LastError() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.lastError
}

// Start starts the LSP server
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.state == ServerStateRunning || s.state == ServerStateStarting {
		s.mu.Unlock()
		return nil
	}
	s.state = ServerStateStarting
	s.mu.Unlock()

	cmd := exec.Command(s.config.Command, s.config.Args...)

	// Set environment
	if len(s.config.Env) > 0 {
		env := os.Environ()
		for k, v := range s.config.Env {
			env = append(env, k+"="+v)
		}
		cmd.Env = env
	}

	// Set workspace folder
	if s.config.WorkspaceFolder != "" {
		cmd.Dir = s.config.WorkspaceFolder
	}

	// Connect stdin/stdout
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		stdin.Close()
		stdout.Close()
		return fmt.Errorf("stderr pipe: %w", err)
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		s.mu.Lock()
		s.state = ServerStateError
		s.lastError = err.Error()
		s.mu.Unlock()
		return fmt.Errorf("start process: %w", err)
	}

	s.process = cmd.Process
	s.client = NewJSONRPCClient(stdin, bufio.NewReader(stdout))

	// Start reading in background
	go func() {
		// Drain stderr
		go io.ReadAll(stderr)
		s.client.ReadLoop()
	}()

	// Initialize with timeout
	timeout := 30 * time.Second
	if s.config.StartupTimeout > 0 {
		timeout = time.Duration(s.config.StartupTimeout) * time.Second
	}
	initCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := s.initialize(initCtx); err != nil {
		s.mu.Lock()
		s.state = ServerStateError
		s.lastError = err.Error()
		s.mu.Unlock()
		s.Stop()
		return fmt.Errorf("initialize: %w", err)
	}

	s.mu.Lock()
	s.state = ServerStateRunning
	s.startTime = time.Now()
	s.mu.Unlock()

	return nil
}

// initialize sends the LSP initialize request
func (s *Server) initialize(ctx context.Context) error {
	params := map[string]any{
		"processId": os.Getpid(),
		"clientInfo": map[string]string{
			"name":    "claude-go",
			"version": "1.0.0",
		},
		"locale": "en",
		"capabilities": map[string]any{
			"workspace": map[string]any{
				"workspaceFolders": true,
			},
			"textDocument": map[string]any{
				"synchronization": map[string]any{
					"willSave":              true,
					"willSaveWaitUntil":     false,
					"didSave":               true,
					"didChange":             map[string]any{"dynamicRegistration": false, "textDocumentSync": 1},
				},
				"hover": map[string]any{
					"dynamicRegistration": false,
				},
				"definition": map[string]any{
					"dynamicRegistration": false,
				},
				"references": map[string]any{
					"dynamicRegistration": false,
				},
				"documentSymbol": map[string]any{
					"dynamicRegistration": false,
					"symbolKind":         map[string]any{"valueSet": []uint32{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26}},
				},
				"implementation": map[string]any{
					"dynamicRegistration": false,
				},
				"callHierarchy": map[string]any{
					"dynamicRegistration": false,
				},
			},
		},
	}

	// Add workspace folder if configured
	if s.config.WorkspaceFolder != "" {
		params["workspaceFolders"] = []map[string]string{
			{"uri": "file://" + s.config.WorkspaceFolder, "name": "workspace"},
		}
	}

	// Add initialization options if configured
	if s.config.InitializationOptions != nil {
		params["initializationOptions"] = s.config.InitializationOptions
	}

	result, err := s.client.SendRequest(ctx, "initialize", params)
	if err != nil {
		return err
	}

	var initResult InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return fmt.Errorf("parse initialize result: %w", err)
	}

	// Send initialized notification
	if err := s.client.SendNotification("initialized", InitializedParams{}); err != nil {
		return fmt.Errorf("send initialized: %w", err)
	}

	return nil
}

// Stop stops the LSP server gracefully
func (s *Server) Stop() error {
	s.mu.Lock()
	if s.state == ServerStateStopped {
		s.mu.Unlock()
		return nil
	}
	s.state = ServerStateStopping
	s.mu.Unlock()

	if s.client != nil {
		// Send shutdown request
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// Ignore errors on shutdown
		s.client.SendRequest(ctx, "shutdown", nil)
		s.client.SendNotification("exit", nil)
		s.client.Close()
	}

	if s.process != nil {
		s.process.Kill()
		s.process.Wait()
	}

	s.mu.Lock()
	s.state = ServerStateStopped
	s.mu.Unlock()

	return nil
}

// IsRunning returns true if the server is running
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state == ServerStateRunning
}

// SendRequest sends a JSON-RPC request to the server
func (s *Server) SendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	s.mu.RLock()
	if s.state != ServerStateRunning {
		s.mu.RUnlock()
		return nil, fmt.Errorf("server not running (state=%s)", s.state)
	}
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("client not initialized")
	}

	return client.SendRequest(ctx, method, params)
}

// SendNotification sends a JSON-RPC notification to the server
func (s *Server) SendNotification(method string, params any) error {
	s.mu.RLock()
	if s.state != ServerStateRunning {
		s.mu.RUnlock()
		return nil
	}
	client := s.client
	s.mu.RUnlock()

	if client == nil {
		return nil
	}

	return client.SendNotification(method, params)
}

// OpenFile sends textDocument/didOpen notification
func (s *Server) OpenFile(uri URI, languageID string, content string) error {
	return s.SendNotification("textDocument/didOpen", map[string]any{
		"textDocument": map[string]any{
			"uri":      uri,
			"languageId": languageID,
			"version":  1,
			"text":     content,
		},
	})
}

// ChangeFile sends textDocument/didChange notification
func (s *Server) ChangeFile(uri URI, content string, version int) error {
	return s.SendNotification("textDocument/didChange", map[string]any{
		"textDocument": map[string]any{
			"uri":     uri,
			"version": version,
		},
		"contentChanges": []map[string]any{
			{"text": content},
		},
	})
}

// SaveFile sends textDocument/didSave notification
func (s *Server) SaveFile(uri URI, text *string) error {
	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
	}
	if text != nil {
		params["text"] = *text
	}
	return s.SendNotification("textDocument/didSave", params)
}

// CloseFile sends textDocument/didClose notification
func (s *Server) CloseFile(uri URI) error {
	return s.SendNotification("textDocument/didClose", map[string]any{
		"textDocument": map[string]any{"uri": uri},
	})
}

// Notifications returns the notification channel
func (s *Server) Notifications() <-chan *jsonRPCNotification {
	s.mu.RLock()
	client := s.client
	s.mu.RUnlock()
	if client != nil {
		return client.Notifications()
	}
	ch := make(chan *jsonRPCNotification)
	close(ch)
	return ch
}

// CanHandle returns true if this server can handle the given file extension or language
func (s *Server) CanHandle(filename string) bool {
	filename = strings.ToLower(filename)
	for _, ext := range s.config.Extensions {
		if strings.HasSuffix(filename, strings.ToLower(ext)) {
			return true
		}
	}
	for _, lang := range s.config.Languages {
		if langMatches(filename, lang) {
			return true
		}
	}
	return false
}

// languageToExtensions maps language IDs to common extensions
var languageToExtensions = map[string][]string{
	"go":         {".go"},
	"typescript": {".ts", ".tsx"},
	"javascript": {".js", ".jsx", ".mjs"},
	"python":     {".py"},
	"rust":      {".rs"},
	"java":      {".java"},
	"cpp":       {".cpp", ".cc", ".cxx", ".h", ".hpp"},
	"c":         {".c", ".h"},
	"csharp":    {".cs"},
	"ruby":      {".rb"},
	"php":       {".php"},
	"swift":     {".swift"},
	"kotlin":    {".kt", ".kts"},
	"typescriptreact": {".tsx"},
	"javascriptreact": {".jsx"},
	"vue":       {".vue"},
	"svelte":    {".svelte"},
	"html":      {".html", ".htm"},
	"css":       {".css", ".scss", ".sass", ".less"},
	"json":      {".json"},
	"yaml":      {".yaml", ".yml"},
	"toml":      {".toml"},
	"markdown":  {".md"},
	"sql":       {".sql"},
	"shell":     {".sh", ".bash"},
}

func langMatches(filename, lang string) bool {
	extensions, ok := languageToExtensions[strings.ToLower(lang)]
	if !ok {
		return false
	}
	for _, ext := range extensions {
		if strings.HasSuffix(strings.ToLower(filename), ext) {
			return true
		}
	}
	return false
}
