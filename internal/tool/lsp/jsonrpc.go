package lsp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
)

// JSON-RPC 2.0 message types
type jsonRPCRequest struct {
	ID     any                   `json:"id"`
	Method string                `json:"method"`
	Params json.RawMessage       `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	ID     any                   `json:"id"`
	Result json.RawMessage       `json:"result,omitempty"`
	Error  *jsonRPCError         `json:"error,omitempty"`
}

type jsonRPCError struct {
	Code    int64  `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type jsonRPCNotification struct {
	Method string      `json:"method"`
	Params any        `json:"params,omitempty"`
}

// JSONRPCClient is a JSON-RPC 2.0 client over stdio
type JSONRPCClient struct {
	stdin       io.Writer
	stdout      *bufio.Reader
	nextID      int64
	pendingMu   sync.RWMutex
	pending     map[int64]chan *jsonRPCResponse
	notifications chan *jsonRPCNotification
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewJSONRPCClient creates a new JSON-RPC client with stdin/stdout
func NewJSONRPCClient(stdin io.Writer, stdout *bufio.Reader) *JSONRPCClient {
	ctx, cancel := context.WithCancel(context.Background())
	return &JSONRPCClient{
		stdin:          stdin,
		stdout:         stdout,
		pending:        make(map[int64]chan *jsonRPCResponse),
		notifications:  make(chan *jsonRPCNotification, 100),
		ctx:            ctx,
		cancel:         cancel,
	}
}

// SendRequest sends a JSON-RPC request and waits for a response
func (c *JSONRPCClient) SendRequest(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := atomic.AddInt64(&c.nextID, 1)

	payload, err := json.Marshal(jsonRPCRequest{
		ID:     id,
		Method: method,
		Params: marshalParams(params),
	})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	// Send the request
	_, err = c.stdin.Write(payload)
	if err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}
	_, err = c.stdin.Write([]byte("\n"))
	if err != nil {
		return nil, fmt.Errorf("write newline: %w", err)
	}
	if f, ok := c.stdin.(interface{ Flush() error }); ok {
		if err := f.Flush(); err != nil {
			return nil, fmt.Errorf("flush: %w", err)
		}
	}

	// Wait for response
	respCh := make(chan *jsonRPCResponse, 1)
	c.pendingMu.Lock()
	c.pending[id] = respCh
	c.pendingMu.Unlock()

	defer func() {
		c.pendingMu.Lock()
		delete(c.pending, id)
		c.pendingMu.Unlock()
	}()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp := <-respCh:
		if resp == nil {
			return nil, fmt.Errorf("connection closed")
		}
		if resp.Error != nil {
			return nil, fmt.Errorf("rpc error: %s (code=%d)", resp.Error.Message, resp.Error.Code)
		}
		return resp.Result, nil
	}
}

// SendNotification sends a JSON-RPC notification (no response expected)
func (c *JSONRPCClient) SendNotification(method string, params any) error {
	payload, err := json.Marshal(jsonRPCNotification{
		Method: method,
		Params: params,
	})
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	_, err = c.stdin.Write(payload)
	if err != nil {
		return fmt.Errorf("write notification: %w", err)
	}
	_, err = c.stdin.Write([]byte("\n"))
	if err != nil {
		return fmt.Errorf("write newline: %w", err)
	}
	if f, ok := c.stdin.(interface{ Flush() error }); ok {
		if err := f.Flush(); err != nil {
			return fmt.Errorf("flush: %w", err)
		}
	}
	return nil
}

// Notifications returns a channel of incoming notifications
func (c *JSONRPCClient) Notifications() <-chan *jsonRPCNotification {
	return c.notifications
}

// ReadLoop processes incoming JSON-RPC messages
// Call this in a goroutine
func (c *JSONRPCClient) ReadLoop() {
	reader := c.stdout
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF || c.ctx.Err() != nil {
				break
			}
			continue
		}

		if len(line) == 0 {
			continue
		}

		// Try to parse as response
		var resp jsonRPCResponse
		if err := json.Unmarshal(line, &resp); err == nil && resp.ID != nil {
			c.pendingMu.RLock()
			id := toInt64(resp.ID)
			ch, ok := c.pending[id]
			c.pendingMu.RUnlock()
			if ok {
				select {
				case ch <- &resp:
				default:
					// Channel full, skip
				}
			}
			continue
		}

		// Try to parse as notification
		var notif jsonRPCNotification
		if err := json.Unmarshal(line, &notif); err == nil && notif.Method != "" {
			select {
			case c.notifications <- &notif:
			default:
				// Channel full, skip notification
			}
			continue
		}

		// Unknown message type, skip
	}
}

// Close closes the client
func (c *JSONRPCClient) Close() {
	c.cancel()
}

// toInt64 converts a JSON ID to int64
func toInt64(id any) int64 {
	switch v := id.(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case string:
		return 0
	}
	return 0
}

// marshalParams marshals params, returning nil for nil params
func marshalParams(p any) json.RawMessage {
	if p == nil {
		return nil
	}
	data, err := json.Marshal(p)
	if err != nil {
		return nil
	}
	if string(data) == "null" {
		return nil
	}
	return data
}
