# Checkpoint 9: Bridge - Remote Control

## Scope
Implement Remote Control (Bridge) functionality.

## Files to Inspect

### Source Files
| File | Purpose |
|-------|---------|
| `src/bridge/types.ts` | Bridge types |
| `src/bridge/bridgeConfig.ts` | Auth config |
| `src/bridge/bridgeEnabled.ts` | Feature flags |
| `src/bridge/bridgeApi.ts` | Environments API |
| `src/bridge/replBridge.ts` | REPL Bridge core |
| `src/bridge/remoteBridgeCore.ts` | Env-less Bridge |
| `src/bridge/sessionRunner.ts` | Session runner |
| `src/bridge/createSession.ts` | Session creation |
| `src/bridge/workSecret.ts` | Work secret |
| `src/bridge/replBridgeTransport.ts` | Transport |

### Target Structure
```
internal/infra/bridge/
├── types.go              # Bridge types
├── config.go            # Configuration
├── enabled.go           # Feature flags
├── api.go              # Environments API client
├── repl_bridge.go       # REPL Bridge core
├── remote_bridge.go     # Env-less Bridge core
├── session_runner.go   # Session runner
├── session.go          # Session management
├── work_secret.go      # Work secret
└── transport/
    └── repl_transport.go # Transport implementation
```

## Implementation Details

### 9.1 Bridge Types
```go
// types.go
type SpawnMode string
const (
    SpawnModeSingleSession SpawnMode = "single-session"
    SpawnModeWorktree     SpawnMode = "worktree"
    SpawnModeSameDir      SpawnMode = "same-dir"
)

type SessionDoneStatus string
const (
    SessionCompleted   SessionDoneStatus = "completed"
    SessionFailed      SessionDoneStatus = "failed"
    SessionInterrupted  SessionDoneStatus = "interrupted"
)

type BridgeConfig struct {
    Dir           string
    MachineName   string
    Branch        string
    GitRepoURL    string
    MaxSessions   int
    SpawnMode     SpawnMode
    Verbose       bool
    Sandbox       bool
    BridgeID      string
    WorkerType    string
    EnvironmentID string
    APIBaseURL    string
    SessionIngressURL string
}
```

### 9.2 Bridge API
```go
// api.go
type BridgeAPI interface {
    RegisterEnvironment(config BridgeConfig) (*EnvironmentCredentials, error)
    PollForWork(envID, envSecret string, signal <-chan struct{}) (*WorkResponse, error)
    AcknowledgeWork(envID, workID, token string) error
    StopWork(envID, workID string, force bool) error
    DeregisterEnvironment(envID string) error
}
```

### 9.3 Session Management
```go
// session.go
type SessionHandle struct {
    SessionID     string
    Done          <-chan SessionDoneStatus
    AccessToken   string
    Activities    []SessionActivity
    CurrentActivity *SessionActivity
}

func (s *SessionHandle) Kill()
func (s *SessionHandle) ForceKill()
func (s *SessionHandle) WriteStdin(data string)
func (s *SessionHandle) UpdateAccessToken(token string)
```

## Parity Checklist
- [ ] Bridge types
- [ ] Feature flags (isBridgeEnabled, etc.)
- [ ] Environments API client
- [ ] REPL Bridge core
- [ ] Env-less Bridge core
- [ ] Session runner
- [ ] Work secret handling
- [ ] Permission callbacks
- [ ] Session recovery

## Next Checkpoint
- [Checkpoint 10: UI Components](./checkpoint-10.md)