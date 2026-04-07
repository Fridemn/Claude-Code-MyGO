# Checkpoint 6: State Management

## Scope
Implement state management including bootstrap state and AppState.

## Files to Inspect

### Source Files
| File | Purpose |
|-------|---------|
| `src/bootstrap/state.ts` | Global bootstrap state |
| `src/state/AppStateStore.ts` | AppState store |
| `src/state/AppState.tsx` | AppState provider |
| `src/state/onChangeAppState.ts` | State change handling |
| `src/state/selectors.ts` | State selectors |
| `src/state/store.ts` | Store utilities |

### Target Structure
```
internal/app/state/
├── bootstrap.go          # Bootstrap state (session-level)
├── app_state.go          # AppState definition
├── store.go              # State store
├── selectors.go          # State selectors
└── changes.go            # State change handlers
```

## Implementation Details

### 6.1 Bootstrap State
```go
// bootstrap.go
type BootstrapState struct {
    // Path state
    OriginalCWD  string
    ProjectRoot  string
    CWD          string

    // Cost & Usage
    TotalCostUSD             float64
    TotalAPIDuration         time.Duration
    TotalAPIDurationWithoutRetries time.Duration
    TotalToolDuration        time.Duration
    ModelUsage               map[string]ModelUsage

    // Session
    SessionID       SessionId
    ParentSessionID SessionId
    IsInteractive   bool
    StartTime       time.Time

    // Telemetry
    Meter          Meter
    SessionCounter Counter
    // ... more counters

    // Session flags
    SessionBypassPermissionsMode bool
    SessionTrustAccepted         bool
    SessionPersistenceDisabled   bool
    // ... more flags

    // Latch states
    AfkModeHeaderLatched       *bool
    FastModeHeaderLatched      *bool
    CacheEditingHeaderLatched  *bool
}

// Global singleton
var globalState = NewBootstrapState()

func GetSessionID() SessionId
func GetOriginalCWD() string
func GetProjectRoot() string
func GetCWDState() string
func SetCWDState(cwd string)
func GetTotalCostUSD() float64
func AddToTotalCost(cost float64, usage ModelUsage, model string)
// ... more getters/setters
```

### 6.2 AppState
```go
// app_state.go
type AppState struct {
    // Settings
    Settings       SettingsJson
    Verbose        bool
    MainLoopModel  ModelSetting

    // UI
    StatusLineText    string
    ExpandedView      string
    IsBriefOnly       bool

    // Agents
    AgentDefinitions  []AgentDefinition
    AgentNameRegistry map[string]AgentId
    ForegroundedTaskID string

    // Permissions
    ToolPermissionContext ToolPermissionContext

    // Tasks
    Tasks map[string]TaskState

    // MCP
    MCP MCPState

    // Plugins
    Plugins PluginsState

    // Files
    FileHistory  FileHistoryState
    Attribution  AttributionState

    // Todos
    Todos map[string]TodoList

    // Notifications
    Notifications NotificationsState

    // Bridge
    ReplBridgeEnabled    bool
    ReplBridgeConnected  bool
    ReplBridgeSessionURL string

    // ... more fields
}
```

### 6.3 State Store
```go
// store.go
type Store struct {
    mu     sync.RWMutex
    state  AppState
    subs   []chan struct{}
}

func NewStore(initial AppState) *Store
func (s *Store) Get() AppState
func (s *Store) Set(update func(AppState) AppState)
func (s *Store) Subscribe() <-chan struct{}
func (s *Store) Unsubscribe(ch <-chan struct{})
```

### 6.4 Selectors
```go
// selectors.go
func SelectVerbose(s AppState) bool
func SelectModel(s AppState) ModelSetting
func SelectTasks(s AppState) map[string]TaskState
func SelectNotifications(s AppState) NotificationsState
```

## Validation Commands
```bash
go build ./internal/app/state/...
go test ./internal/app/state/... -v
```

## Parity Checklist
- [ ] Bootstrap state fields
- [ ] AppState fields (100+ fields)
- [ ] Getters/setters
- [ ] Thread-safe store
- [ ] Subscription mechanism
- [ ] State change handling

## Next Checkpoint
- [Checkpoint 7: Services - API & Auth](./checkpoint-7.md)