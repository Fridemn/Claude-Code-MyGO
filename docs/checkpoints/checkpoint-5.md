# Checkpoint 5: CLI & I/O

## Scope
Implement CLI input/output handling, structured IO, and transports.

## Files to Inspect

### Source Files
| File | Purpose |
|-------|---------|
| `src/cli/print.ts` | Print and output management |
| `src/cli/structuredIO.ts` | StructuredIO class |
| `src/cli/remoteIO.ts` | Remote IO |
| `src/cli/transports/*.ts` | Transport implementations |
| `src/cli/exit.ts` | Exit helpers |
| `src/cli/update.ts` | Version update |

### Target Structure
```
internal/ui/cli/
├── structured_io.go      # StructuredIO
├── remote_io.go          # RemoteIO
├── print.go              # Print helpers
├── exit.go               # Exit helpers
├── update.go             # Version update
└── transport/
    ├── transport.go      # Transport interface
    ├── websocket.go      # WebSocket transport
    ├── sse.go            # SSE transport
    ├── hybrid.go         # Hybrid transport
    └── batch_uploader.go # Batch event uploader
```

## Implementation Details

### 5.1 StructuredIO
```go
// structured_io.go
type StructuredIO struct {
    input           <-chan StdinMessage
    pendingRequests map[string]*PendingRequest
    inputClosed     bool
}

func (io *StructuredIO) Write(msg SDKMessage) error
func (io *StructuredIO) WriteBatch(msgs []SDKMessage) error
func (io *StructuredIO) SendControlRequest(req SDKControlRequest, schema any) (any, error)
func (io *StructuredIO) SendControlCancelRequest(requestID string) error
func (io *StructuredIO) SendResult() error
func (io *StructuredIO) GetPendingRequests() map[string]*PendingRequest
```

### 5.2 Transport Interface
```go
// transport/transport.go
type TransportState string
const (
    TransportStateConnecting TransportState = "connecting"
    TransportStateOpen       TransportState = "open"
    TransportStateClosing    TransportState = "closing"
    TransportStateClosed     TransportState = "closed"
)

type Transport interface {
    State() TransportState
    Messages() <-chan StdoutMessage
    Write(msg StdoutMessage) error
    WriteBatch(msgs []StdoutMessage) error
    Close() error
}
```

### 5.3 WebSocket Transport
```go
// transport/websocket.go
type WebSocketTransport struct {
    url       string
    headers   map[string]string
    state     TransportState
    messages  chan StdoutMessage
}

func NewWebSocketTransport(url string, headers map[string]string) *WebSocketTransport
```

### 5.4 SSE Transport
```go
// transport/sse.go
type SSETransport struct {
    url       string
    headers   map[string]string
    state     TransportState
    messages  chan StdoutMessage
}

func NewSSETransport(url string, headers map[string]string) *SSETransport
func ParseSSEFrames(buffer string) ([]SSEFrame, string)
```

### 5.5 Hybrid Transport
```go
// transport/hybrid.go
type HybridTransport struct {
    ws          *WebSocketTransport
    postUrl     string
    uploader    *SerialBatchEventUploader
}

func NewHybridTransport(url string, headers map[string]string) *HybridTransport
```

## Validation Commands
```bash
go build ./internal/ui/cli/...
go test ./internal/ui/cli/... -v
```

## Parity Checklist
- [ ] StructuredIO core
- [ ] Transport interface
- [ ] WebSocket transport
- [ ] SSE transport
- [ ] Hybrid transport
- [ ] Serial batch uploader
- [ ] SSE frame parsing
- [ ] Permission request handling
- [ ] Control message handling

## Next Checkpoint
- [Checkpoint 6: State Management](./checkpoint-6.md)