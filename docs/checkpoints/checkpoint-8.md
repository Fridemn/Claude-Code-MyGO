# Checkpoint 8: Services - MCP

## Scope
Implement MCP (Model Context Protocol) services.

## Files to Inspect

### Source Files
| File | Purpose |
|-------|---------|
| `src/services/mcp/types.ts` | MCP types |
| `src/services/mcp/MCPConnectionManager.tsx` | Connection manager |
| `src/services/mcp/config.ts` | Config loading |
| `src/services/mcp/elicitationHandler.ts` | Elicitation |
| `src/services/mcp/channelPermissions.ts` | Channel permissions |

### Target Structure
```
internal/infra/mcp/
├── types.go              # MCP types
├── manager.go            # Connection manager
├── config.go             # Config loading
├── transport/
│   ├── stdio.go          # Stdio transport
│   ├── sse.go            # SSE transport
│   ├── http.go           # HTTP transport
│   └── websocket.go      # WebSocket transport
├── elicitation.go        # Elicitation handler
└── channel.go            # Channel permissions
```

## Implementation Details

### 8.1 MCP Types
```go
// types.go
type Transport string
const (
    TransportStdio Transport = "stdio"
    TransportSSE   Transport = "sse"
    TransportHTTP  Transport = "http"
    TransportWS    Transport = "ws"
    TransportSDK   Transport = "sdk"
)

type MCPServerConfig interface {
    GetTransport() Transport
}

type MCPServerConnection interface {
    GetName() string
    GetStatus() ConnectionStatus
    GetTools() []Tool
}
```

### 8.2 Connection Manager
```go
// manager.go
type MCPConnectionManager struct {
    connections map[string]MCPServerConnection
}

func (m *MCPConnectionManager) ConnectServer(name string, config MCPServerConfig) error
func (m *MCPConnectionManager) DisconnectServer(name string) error
func (m *MCPConnectionManager) ListTools() []Tool
func (m *MCPConnectionManager) CallTool(name string, args any) (any, error)
```

## Parity Checklist
- [ ] MCP types
- [ ] Connection manager
- [ ] Config loading
- [ ] Stdio transport
- [ ] SSE transport
- [ ] HTTP transport
- [ ] WebSocket transport
- [ ] Elicitation handler

## Next Checkpoint
- [Checkpoint 9: Bridge - Remote Control](./checkpoint-9.md)