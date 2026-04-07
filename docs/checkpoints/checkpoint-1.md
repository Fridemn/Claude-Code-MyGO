# Checkpoint 1: Core Types & Interfaces

## Scope
Establish foundational types and interfaces that will be used throughout the migration.

## Files to Inspect

### Source Files
| File | Purpose |
|-------|---------|
| `src/types/ids.ts` | Brand types (SessionId, AgentId) |
| `src/types/message.ts` | Message types (Message, AssistantMessage, UserMessage, etc.) |
| `src/types/command.ts` | Command types (Command, LocalCommand, LocalJSXCommand, PromptCommand) |
| `src/types/permissions.ts` | Permission types |
| `src/types/plugin.ts` | Plugin types |
| `src/types/textInputTypes.ts` | Text input types |
| `src/constants/*.ts` | All constants |
| `src/bootstrap/state.ts` | Bootstrap state types |

### Target Structure
```
internal/domain/
├── types/
│   ├── ids.go          # SessionId, AgentId brand types
│   ├── message.go      # Message types
│   ├── command.go       # Command types
│   ├── permission.go    # Permission types
│   └── plugin.go       # Plugin types
└── pkg/
    └── constants/
        ├── api.go      # API limits, etc.
        ├── xml.go      # XML tags
        ├── betas.go    # Beta headers
        └── [other constants]
```

## Types to Implement

### 1.1 Brand Types (ids.go)
```go
// SessionId is a branded type for session identifiers
type SessionId string

// AgentId is a branded type for agent identifiers
type AgentId string

// Casting functions
func AsSessionId(id string) SessionId
func AsAgentId(id string) AgentId
func ToAgentId(s string) (AgentId, bool)
```

### 1.2 Message Types (message.go)
```go
// Message union types
type Message interface{ isMessage() }
type AssistantMessage struct { ... }
type UserMessage struct { ... }
type SystemMessage struct { ... }
type AttachmentMessage struct { ... }

// Content blocks
type ContentBlock interface{ isContentBlock() }
type TextBlock struct { ... }
type ToolUseBlock struct { ... }
type ToolResultBlock struct { ... }
```

### 1.3 Command Types (command.go)
```go
// Command types
type CommandType string
const (
    CommandTypeLocal   CommandType = "local"
    CommandTypeJSX    CommandType = "local-jsx"
    CommandTypePrompt  CommandType = "prompt"
)

// Command interface
type Command interface {
    GetName() string
    GetDescription() string
    GetType() CommandType
}

// LocalCommand
type LocalCommand struct {
    Name string
    Description string
    SupportsNonInteractive bool
    Handler func(args string, ctx CommandContext) (LocalCommandResult, error)
}

// LocalJSXCommand
type LocalJSXCommand struct {
    Name string
    Description string
    Immediate bool
    Handler func(onDone func(), ctx CommandContext, args string) (string, error)
}

// PromptCommand
type PromptCommand struct {
    Name string
    Description string
    ProgressMessage string
    ContentLength int
    GetPrompt func(args string, ctx CommandContext) ([]ContentBlock, error)
}
```

### 1.4 Permission Types (permission.go)
```go
// Permission mode
type PermissionMode string
const (
    PermissionModeAcceptEdits     PermissionMode = "acceptEdits"
    PermissionModeLimitTools      PermissionMode = "limitTools"
    PermissionModeBypass         PermissionMode = "bypassPermissions"
    PermissionModeAsk            PermissionMode = "ask"
)

// Permission decision
type PermissionDecision struct {
    Behavior string // "allow", "deny", "escalate"
    Reason   string
}
```

### 1.5 Constants (pkg/constants/*.go)
```go
// apiLimits.go
const (
    MaxTokens         = 8192
    MaxRetries        = 3
    RequestTimeoutMs  = 60_000
)

// xml.go
var XMLTags = struct {
    ToolUse    string
    ToolResult string
    Text       string
    Thinking   string
    Redacted   string
}{
    ToolUse:    "tool_use",
    ToolResult:"tool_result",
    Text:       "text",
    Thinking:   "thinking",
    Redacted:   "redacted",
}

// betas.go
var BetaHeaders = struct {
    PromptCache string
    AfkMode     string
    FastMode    string
}{
    PromptCache: "prompt-cache-1m-2025-08-07",
    AfkMode:     "afk-mode-2025-11-01",
    FastMode:    "fast-mode-2025-06-01",
}
```

## Validation Commands
```bash
# Compile check
go build ./internal/domain/...

# Type tests
go test ./internal/domain/types/... -v

# Vet
go vet ./internal/domain/...
```

## Parity Checklist
- [ ] SessionId/AgentId brand types
- [ ] Message type hierarchy
- [ ] Command type hierarchy
- [ ] Permission types
- [ ] All constants ported
- [ ] Type names match TS exactly
- [ ] Behavior matches TS (type coercion, validation)

## Known Deviations
1. Go doesn't have branded types natively; use type alias + validation functions
2. Go interfaces are explicit, not structural typing like TypeScript
3. Union types in Go require either interface + structs or custom enum + switch

## Next Checkpoint
- [Checkpoint 2: Command System](./checkpoint-2.md)