package services

// LSP Client using JSON-RPC over stdio.
// Ported from src/services/lsp/LSPClient.ts

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

// ServerCapabilities represents LSP server capabilities.
type ServerCapabilities struct {
	TextDocumentSync           interface{} `json:"textDocumentSync,omitempty"`
	HoverProvider              interface{} `json:"hoverProvider,omitempty"`
	DefinitionProvider         interface{} `json:"definitionProvider,omitempty"`
	ReferencesProvider         interface{} `json:"referencesProvider,omitempty"`
	DocumentSymbolProvider     interface{} `json:"documentSymbolProvider,omitempty"`
	PublishDiagnosticsProvider bool        `json:"publishDiagnosticsProvider,omitempty"`
}

// InitializeResult represents the result of an LSP initialize request.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

// InitializeParams represents LSP initialize request parameters.
type InitializeParams struct {
	ProcessId             int                    `json:"processId"`
	InitializationOptions interface{}            `json:"initializationOptions,omitempty"`
	WorkspaceFolders      []WorkspaceFolder      `json:"workspaceFolders,omitempty"`
	RootPath              string                 `json:"rootPath,omitempty"`  // Deprecated
	RootUri               string                 `json:"rootUri,omitempty"`   // Deprecated
	Capabilities          ClientCapabilities     `json:"capabilities"`
}

// WorkspaceFolder represents a workspace folder.
type WorkspaceFolder struct {
	Uri  string `json:"uri"`
	Name string `json:"name"`
}

// ClientCapabilities represents client capabilities.
type ClientCapabilities struct {
	Workspace    WorkspaceCapabilities    `json:"workspace"`
	TextDocument TextDocumentCapabilities `json:"textDocument"`
	General      GeneralCapabilities      `json:"general,omitempty"`
}

// WorkspaceCapabilities represents workspace capabilities.
type WorkspaceCapabilities struct {
	Configuration    bool `json:"configuration"`
	WorkspaceFolders bool `json:"workspaceFolders"`
}

// TextDocumentCapabilities represents text document capabilities.
type TextDocumentCapabilities struct {
	Synchronization         SynchronizationCapabilities         `json:"synchronization"`
	PublishDiagnostics      PublishDiagnosticsCapabilities      `json:"publishDiagnostics"`
	Hover                   HoverCapabilities                   `json:"hover"`
	Definition              DefinitionCapabilities              `json:"definition"`
	References              ReferencesCapabilities              `json:"references"`
	DocumentSymbol          DocumentSymbolCapabilities          `json:"documentSymbol"`
	CallHierarchy           CallHierarchyCapabilities           `json:"callHierarchy,omitempty"`
}

// SynchronizationCapabilities represents synchronization capabilities.
type SynchronizationCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration"`
	WillSave            bool `json:"willSave"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil"`
	DidSave             bool `json:"didSave"`
}

// PublishDiagnosticsCapabilities represents publish diagnostics capabilities.
type PublishDiagnosticsCapabilities struct {
	RelatedInformation bool                `json:"relatedInformation"`
	TagSupport         TagSupport          `json:"tagSupport"`
	VersionSupport     bool                `json:"versionSupport"`
	CodeDescription    bool                `json:"codeDescriptionSupport"`
	DataSupport        bool                `json:"dataSupport"`
}

// TagSupport represents tag support.
type TagSupport struct {
	ValueSet []int `json:"valueSet"`
}

// HoverCapabilities represents hover capabilities.
type HoverCapabilities struct {
	DynamicRegistration bool     `json:"dynamicRegistration"`
	ContentFormat       []string `json:"contentFormat"`
}

// DefinitionCapabilities represents definition capabilities.
type DefinitionCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration"`
	LinkSupport         bool `json:"linkSupport"`
}

// ReferencesCapabilities represents references capabilities.
type ReferencesCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration"`
}

// DocumentSymbolCapabilities represents document symbol capabilities.
type DocumentSymbolCapabilities struct {
	DynamicRegistration         bool `json:"dynamicRegistration"`
	HierarchicalDocumentSymbol  bool `json:"hierarchicalDocumentSymbolSupport"`
}

// CallHierarchyCapabilities represents call hierarchy capabilities.
type CallHierarchyCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration"`
}

// GeneralCapabilities represents general capabilities.
type GeneralCapabilities struct {
	PositionEncodings []string `json:"positionEncodings"`
}

// JSONRPCRequest represents a JSON-RPC request.
type JSONRPCRequest struct {
	Jsonrpc string      `json:"jsonrpc"`
	Id      interface{} `json:"id,omitempty"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// JSONRPCResponse represents a JSON-RPC response.
type JSONRPCResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	Id      interface{}     `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error.
type JSONRPCError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSONRPCNotification represents a JSON-RPC notification.
type JSONRPCNotification struct {
	Jsonrpc string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// LSPClient interface for LSP communication.
// Ported from src/services/lsp/LSPClient.ts:LSPClient
type LSPClient interface {
	Capabilities() ServerCapabilities
	IsInitialized() bool
	Start(ctx context.Context, command string, args []string, options *LSPStartOptions) error
	Initialize(ctx context.Context, params InitializeParams) (InitializeResult, error)
	SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error)
	SendNotification(ctx context.Context, method string, params interface{}) error
	OnNotification(method string, handler func(params interface{}))
	OnRequest(method string, handler func(params interface{}) (interface{}, error))
	Stop(ctx context.Context) error
}

// LSPStartOptions contains options for starting an LSP server.
type LSPStartOptions struct {
	Env map[string]string
	Cwd string
}

// JSONRPCClient implements LSP client using JSON-RPC over stdio.
type JSONRPCClient struct {
	serverName string

	mu             sync.Mutex
	process        *exec.Cmd
	stdin          io.WriteCloser
	stdout         io.Reader
	stderr         io.Reader
	reader         *bufio.Reader
	capabilities   ServerCapabilities
	isInitialized  bool
	startFailed    bool
	startError     error
	isStopping     bool

	// Pending handlers registered before connection ready
	pendingNotificationHandlers []notificationHandler
	pendingRequestHandlers      []requestHandler

	// Registered handlers
	notificationHandlers map[string]func(interface{})
	requestHandlers      map[string]func(interface{}) (interface{}, error)

	// Request/response tracking
	nextRequestId  int
	pendingRequests map[interface{}]chan *JSONRPCResponse

	// Crash callback
	onCrash func(error)
}

type notificationHandler struct {
	method  string
	handler func(interface{})
}

type requestHandler struct {
	method  string
	handler func(interface{}) (interface{}, error)
}

// CreateLSPClient creates a new LSP client.
// Ported from src/services/lsp/LSPClient.ts:createLSPClient
func CreateLSPClient(serverName string, onCrash func(error)) LSPClient {
	return &JSONRPCClient{
		serverName:             serverName,
		onCrash:                onCrash,
		notificationHandlers:   make(map[string]func(interface{})),
		requestHandlers:        make(map[string]func(interface{}) (interface{}, error)),
		pendingRequests:        make(map[interface{}]chan *JSONRPCResponse),
		pendingNotificationHandlers: make([]notificationHandler, 0),
		pendingRequestHandlers:      make([]requestHandler, 0),
	}
}

// Capabilities returns server capabilities.
func (c *JSONRPCClient) Capabilities() ServerCapabilities {
	return c.capabilities
}

// IsInitialized returns whether client is initialized.
func (c *JSONRPCClient) IsInitialized() bool {
	return c.isInitialized
}

// Start starts the LSP server process.
// Ported from src/services/lsp/LSPClient.ts:start
func (c *JSONRPCClient) Start(ctx context.Context, command string, args []string, options *LSPStartOptions) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Reset state
	c.startFailed = false
	c.startError = nil

	// Build environment
	env := os.Environ()
	if options != nil && options.Env != nil {
		for k, v := range options.Env {
			env = append(env, k+"="+v)
		}
	}

	// Spawn process
	c.process = exec.CommandContext(ctx, command, args...)
	c.process.Env = env
	if options != nil && options.Cwd != "" {
		c.process.Dir = options.Cwd
	}

	// Set up stdio pipes
	stdinPipe, err := c.process.StdinPipe()
	if err != nil {
		c.startFailed = true
		c.startError = err
		return fmt.Errorf("LSP server %s: failed to create stdin pipe: %w", c.serverName, err)
	}
	c.stdin = stdinPipe

	stdoutPipe, err := c.process.StdoutPipe()
	if err != nil {
		c.startFailed = true
		c.startError = err
		return fmt.Errorf("LSP server %s: failed to create stdout pipe: %w", c.serverName, err)
	}
	c.stdout = stdoutPipe
	c.reader = bufio.NewReader(stdoutPipe)

	stderrPipe, err := c.process.StderrPipe()
	if err != nil {
		c.startFailed = true
		c.startError = err
		return fmt.Errorf("LSP server %s: failed to create stderr pipe: %w", c.serverName, err)
	}
	c.stderr = stderrPipe

	// Start process
	if err := c.process.Start(); err != nil {
		c.startFailed = true
		c.startError = err
		return fmt.Errorf("LSP server %s failed to start: %w", c.serverName, err)
	}

	// Handle stderr logging
	go c.handleStderr()

	// Handle process exit
	go c.handleProcessExit()

	// Start message reader
	go c.readMessages()

	// Apply queued handlers
	for _, h := range c.pendingNotificationHandlers {
		c.notificationHandlers[h.method] = h.handler
	}
	c.pendingNotificationHandlers = nil

	for _, h := range c.pendingRequestHandlers {
		c.requestHandlers[h.method] = h.handler
	}
	c.pendingRequestHandlers = nil

	return nil
}

// handleStderr reads and logs stderr output.
func (c *JSONRPCClient) handleStderr() {
	scanner := bufio.NewScanner(c.stderr)
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			// Log stderr for debugging
			fmt.Fprintf(os.Stderr, "[LSP SERVER %s] %s\n", c.serverName, line)
		}
	}
}

// handleProcessExit handles process exit events.
func (c *JSONRPCClient) handleProcessExit() {
	err := c.process.Wait()
	if err != nil && !c.isStopping {
		c.mu.Lock()
		c.startFailed = true
		c.startError = err
		c.isInitialized = false
		c.mu.Unlock()

		crashErr := fmt.Errorf("LSP server %s crashed: %w", c.serverName, err)
		if c.onCrash != nil {
			c.onCrash(crashErr)
		}
	}
}

// readMessages reads JSON-RPC messages from stdout.
func (c *JSONRPCClient) readMessages() {
	for {
		// Read message header
		line, err := c.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			continue
		}

		// Parse content-length header
		if strings.HasPrefix(line, "Content-Length:") {
			// Extract length
			lengthStr := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))
			length := 0
			for _, ch := range lengthStr {
				if ch >= '0' && ch <= '9' {
					length = length*10 + int(ch-'0')
				}
			}

			// Skip empty line after header
			emptyLine, _ := c.reader.ReadString('\n')
			if emptyLine != "\n" && emptyLine != "\r\n" {
				continue
			}

			// Read message body
			body := make([]byte, length)
			if _, err := io.ReadFull(c.reader, body); err != nil {
				continue
			}

			// Parse message
			c.handleMessage(body)
		}
	}
}

// handleMessage handles a JSON-RPC message.
func (c *JSONRPCClient) handleMessage(body []byte) {
	// Determine message type
	var base struct {
		Jsonrpc string          `json:"jsonrpc"`
		Id      interface{}     `json:"id,omitempty"`
		Method  string          `json:"method,omitempty"`
		Params  json.RawMessage `json:"params,omitempty"`
		Result  json.RawMessage `json:"result,omitempty"`
		Error   *JSONRPCError   `json:"error,omitempty"`
	}

	if err := json.Unmarshal(body, &base); err != nil {
		return
	}

	// Check if it's a response
	if base.Id != nil && base.Method == "" {
		// It's a response
		response := &JSONRPCResponse{
			Jsonrpc: base.Jsonrpc,
			Id:      base.Id,
			Result:  base.Result,
			Error:   base.Error,
		}

		// Find pending request
		c.mu.Lock()
		ch, exists := c.pendingRequests[base.Id]
		if exists {
			delete(c.pendingRequests, base.Id)
		}
		c.mu.Unlock()

		if ch != nil {
			ch <- response
		}
	} else if base.Method != "" {
		// It's a notification or request
		var params interface{}
		if base.Params != nil {
			json.Unmarshal(base.Params, &params)
		}

		if base.Id != nil {
			// It's a request from server
			handler, exists := c.requestHandlers[base.Method]
			if exists {
				result, err := handler(params)
				c.sendResponse(base.Id, result, err)
			} else {
				// No handler - send error response
				c.sendResponse(base.Id, nil, fmt.Errorf("method not found: %s", base.Method))
			}
		} else {
			// It's a notification
			handler, exists := c.notificationHandlers[base.Method]
			if exists {
				handler(params)
			}
		}
	}
}

// sendResponse sends a response to a request from the server.
func (c *JSONRPCClient) sendResponse(id interface{}, result interface{}, err error) {
	response := JSONRPCResponse{
		Jsonrpc: "2.0",
		Id:      id,
	}

	if err != nil {
		response.Error = &JSONRPCError{
			Code:    -32603, // Internal error
			Message: err.Error(),
		}
	} else {
		resultJSON, _ := json.Marshal(result)
		response.Result = json.RawMessage(resultJSON)
	}

	c.sendMessage(response)
}

// sendMessage sends a JSON-RPC message.
func (c *JSONRPCClient) sendMessage(msg interface{}) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// Write header and body
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	if _, err := c.stdin.Write([]byte(header)); err != nil {
		return err
	}
	if _, err := c.stdin.Write(body); err != nil {
		return err
	}

	return nil
}

// Initialize initializes the LSP server.
// Ported from src/services/lsp/LSPClient.ts:initialize
func (c *JSONRPCClient) Initialize(ctx context.Context, params InitializeParams) (InitializeResult, error) {
	c.mu.Lock()
	if c.stdin == nil {
		c.mu.Unlock()
		return InitializeResult{}, fmt.Errorf("LSP client not started")
	}
	if c.startFailed {
		err := c.startError
		if err == nil {
			err = fmt.Errorf("LSP server %s failed to start", c.serverName)
		}
		c.mu.Unlock()
		return InitializeResult{}, err
	}
	c.mu.Unlock()

	// Send initialize request
	result, err := c.SendRequest(ctx, "initialize", params)
	if err != nil {
		return InitializeResult{}, fmt.Errorf("LSP server %s initialize failed: %w", c.serverName, err)
	}

	// Parse result
	var initResult InitializeResult
	resultJSON, _ := json.Marshal(result)
	if err := json.Unmarshal(resultJSON, &initResult); err != nil {
		return InitializeResult{}, fmt.Errorf("failed to parse initialize result: %w", err)
	}

	c.mu.Lock()
	c.capabilities = initResult.Capabilities
	c.mu.Unlock()

	// Send initialized notification
	if err := c.SendNotification(ctx, "initialized", nil); err != nil {
		// Log but continue - this is often not critical
		fmt.Fprintf(os.Stderr, "[LSP] initialized notification failed: %v\n", err)
	}

	c.mu.Lock()
	c.isInitialized = true
	c.mu.Unlock()

	return initResult, nil
}

// SendRequest sends a request to the LSP server.
// Ported from src/services/lsp/LSPClient.ts:sendRequest
func (c *JSONRPCClient) SendRequest(ctx context.Context, method string, params interface{}) (interface{}, error) {
	c.mu.Lock()
	if c.stdin == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("LSP client not started")
	}
	if c.startFailed {
		err := c.startError
		c.mu.Unlock()
		return nil, err
	}
	if !c.isInitialized {
		c.mu.Unlock()
		return nil, fmt.Errorf("LSP server not initialized")
	}

	// Generate request ID
	c.nextRequestId++
	id := c.nextRequestId

	// Create response channel
	ch := make(chan *JSONRPCResponse, 1)
	c.pendingRequests[id] = ch
	c.mu.Unlock()

	// Send request
	request := JSONRPCRequest{
		Jsonrpc: "2.0",
		Id:      id,
		Method:  method,
		Params:  params,
	}

	if err := c.sendMessage(request); err != nil {
		c.mu.Lock()
		delete(c.pendingRequests, id)
		c.mu.Unlock()
		return nil, fmt.Errorf("LSP server %s request %s failed: %w", c.serverName, method, err)
	}

	// Wait for response
	select {
	case response := <-ch:
		if response.Error != nil {
			return nil, fmt.Errorf("LSP request error: %s (code %d)", response.Error.Message, response.Error.Code)
		}

		var result interface{}
		if response.Result != nil {
			json.Unmarshal(response.Result, &result)
		}
		return result, nil

	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pendingRequests, id)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

// SendNotification sends a notification to the LSP server.
// Ported from src/services/lsp/LSPClient.ts:sendNotification
func (c *JSONRPCClient) SendNotification(ctx context.Context, method string, params interface{}) error {
	c.mu.Lock()
	if c.stdin == nil {
		c.mu.Unlock()
		return fmt.Errorf("LSP client not started")
	}
	if c.startFailed {
		c.mu.Unlock()
		return c.startError
	}
	c.mu.Unlock()

	notification := JSONRPCNotification{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
	}

	if err := c.sendMessage(notification); err != nil {
		// Don't re-throw - notifications are fire-and-forget
		fmt.Fprintf(os.Stderr, "[LSP] notification %s failed: %v\n", method, err)
		return err
	}

	return nil
}

// OnNotification registers a handler for notifications from the server.
// Ported from src/services/lsp/LSPClient.ts:onNotification
func (c *JSONRPCClient) OnNotification(method string, handler func(interface{})) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin == nil {
		// Queue handler for later
		c.pendingNotificationHandlers = append(c.pendingNotificationHandlers, notificationHandler{
			method:  method,
			handler: handler,
		})
		return
	}

	c.notificationHandlers[method] = handler
}

// OnRequest registers a handler for requests from the server.
// Ported from src/services/lsp/LSPClient.ts:onRequest
func (c *JSONRPCClient) OnRequest(method string, handler func(interface{}) (interface{}, error)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stdin == nil {
		// Queue handler for later
		c.pendingRequestHandlers = append(c.pendingRequestHandlers, requestHandler{
			method:  method,
			handler: handler,
		})
		return
	}

	c.requestHandlers[method] = handler
}

// Stop stops the LSP server gracefully.
// Ported from src/services/lsp/LSPClient.ts:stop
func (c *JSONRPCClient) Stop(ctx context.Context) error {
	c.mu.Lock()
	c.isStopping = true
	c.mu.Unlock()

	var shutdownError error

	// Try to send shutdown request and exit notification
	if c.stdin != nil {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		// Send shutdown request
		_, err := c.SendRequest(ctx, "shutdown", nil)
		if err != nil {
			shutdownError = err
		}

		// Send exit notification
		c.SendNotification(ctx, "exit", nil)
	}

	// Close stdin
	if c.stdin != nil {
		c.stdin.Close()
	}

	// Kill process
	if c.process != nil && c.process.Process != nil {
		c.process.Process.Kill()
	}

	c.mu.Lock()
	c.isInitialized = false
	c.capabilities = ServerCapabilities{}
	c.isStopping = false
	c.stdin = nil
	c.stdout = nil
	c.stderr = nil
	c.reader = nil
	c.mu.Unlock()

	return shutdownError
}