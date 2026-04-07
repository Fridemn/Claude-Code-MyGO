# Checkpoint 4: QueryEngine

## Scope
Implement the core query engine that handles user input, API calls, tool execution, and response generation.

## Files to Inspect

### Source Files
| File | Purpose |
|-------|---------|
| `src/QueryEngine.ts` | Main query engine |
| `src/query.ts` | Query helpers |
| `src/Task.ts` | Task system |
| `src/context.ts` | Context building |
| `src/context/*.ts` | Context components |

### Target Structure
```
internal/app/query/
├── engine.go             # QueryEngine implementation
├── message.go            # Message processing
├── budget.go             # Budget management
├── context.go            # Context building
├── permission.go         # Permission tracking
└── types.go              # Query types
```

## Implementation Details

### 4.1 QueryEngine Core
```go
// engine.go
type QueryEngine struct {
    config          QueryEngineConfig
    messages        []Message
    abortController *AbortController
    permissionDenials []PermissionDenial
    totalUsage      Usage
    readFileState   FileStateCache
}

func (e *QueryEngine) SubmitMessage(
    ctx context.Context,
    prompt string,
    options *SubmitOptions,
) (<-chan SDKMessage, error)

func (e *QueryEngine) Interrupt()
func (e *QueryEngine) GetMessages() []Message
func (e *QueryEngine) GetSessionId() string
func (e *QueryEngine) SetModel(model string)
```

### 4.2 Message Processing
```go
// message.go
func ProcessUserInput(input string, ctx ProcessContext) (*ProcessResult, error)
func BuildSystemPrompt(config PromptConfig) (string, error)
func HandleToolUse(toolUse ToolUseBlock, ctx ToolContext) (*ToolResult, error)
func HandleAssistantMessage(msg AssistantMessage, ctx MessageContext) error
```

### 4.3 Budget Management
```go
// budget.go
type BudgetManager struct {
    maxTurns     int
    maxBudgetUsd float64
    taskBudget   int
    currentTurns int
    currentCost  float64
}

func (m *BudgetManager) CheckBudget() (*BudgetStatus, error)
func (m *BudgetManager) TrackUsage(usage Usage)
func (m *BudgetManager) ShouldStop() bool
```

### 4.4 Query Types
```go
// types.go
type QueryEngineConfig struct {
    CWD                string
    Tools              []Tool
    Commands           []Command
    MCPClients         []MCPServerConnection
    Agents             []AgentDefinition
    CanUseTool         CanUseToolFunc
    GetAppState        func() AppState
    SetAppState        func(AppState)
    InitialMessages    []Message
    ReadFileCache      FileStateCache
    CustomSystemPrompt string
    AppendSystemPrompt string
    UserSpecifiedModel string
    FallbackModel      string
    ThinkingConfig     *ThinkingConfig
    MaxTurns           int
    MaxBudgetUsd       float64
    TaskBudget         *TaskBudget
    JSONSchema         map[string]any
    Verbose            bool
}

type SubmitOptions struct {
    UUID   string
    IsMeta bool
}
```

## Query Flow

```
┌─────────────────────────────────────────────────────────────┐
│                     SubmitMessage Flow                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. Initialize                                              │
│     ├── Clear skill discovery tracking                      │
│     ├── Set working directory                               │
│     └── Create canUseTool wrapper (track denials)           │
│                                                             │
│  2. Get System Prompt                                       │
│     └── fetchSystemPromptParts({ tools, model, ... })       │
│                                                             │
│  3. Process User Input                                      │
│     └── processUserInput({ input: prompt, ... })            │
│                                                             │
│  4. API Call Loop                                           │
│     ┌───────────────────────────────────────┐               │
│     │  for message in query({ ... }):       │               │
│     │    handle message type:               │               │
│     │    - assistant → process tool uses    │               │
│     │    - tool_result → continue/stop      │               │
│     │    - stream_event → yield progress    │               │
│     └───────────────────────────────────────┘               │
│                                                             │
│  5. Result Processing                                       │
│     ├── Track token usage                                   │
│     ├── Check budget limits                                 │
│     └── Generate final result message                       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Validation Commands
```bash
# Build
go build ./internal/app/query/...

# Test
go test ./internal/app/query/... -v

# Integration test
go run ./cmd/claude
# In REPL:
# - Send message and check response
# - Test tool execution during query
# - Test budget limits
# - Test interruption
```

## Parity Checklist
- [ ] QueryEngine core functionality
- [ ] Message processing
- [ ] System prompt building
- [ ] Tool execution during queries
- [ ] Permission denial tracking
- [ ] Budget management (turns, USD)
- [ ] Task budget tracking
- [ ] API error handling
- [ ] Interruption handling
- [ ] Message history management
- [ ] AsyncGenerator-style streaming (Go channels)

## Known Deviations
1. Go uses channels instead of AsyncGenerator for streaming
2. Error handling patterns differ (Go idioms vs try-catch)
3. Context propagation is explicit in Go

## Risks
- Complex message flow requires careful state management
- Tool execution during queries adds complexity
- Budget management needs accurate cost calculation
- Streaming responses need proper channel handling

## Dependencies
- Checkpoint 1: Core Types
- Checkpoint 2: Commands (for /compact integration)
- Checkpoint 3: Tools (for tool execution)

## Next Checkpoint
- [Checkpoint 5: CLI & I/O](./checkpoint-5.md)