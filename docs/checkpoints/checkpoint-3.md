# Checkpoint 3: Tool System

## Scope
Implement the complete tool system with 40+ tools.

## Files to Inspect

### Source Files
| File | Purpose |
|-------|---------|
| `src/Tool.ts` | Tool interface definition |
| `src/tools.ts` | Tool registration and loading |
| `src/tools/*/ToolName.ts[x]` | Tool implementations |
| `src/tools/*/prompt.ts` | Tool prompts |
| `src/tools/*/UI.tsx` | Tool UI components |

### Target Structure
```
internal/app/tools/
├── registry.go           # Tool registry
├── filter.go             # Tool filtering
├── tool.go               # Tool interface
├── result.go             # Tool result types
├── context.go            # Tool execution context
└── tools/
    ├── bash/
    │   └── bash.go
    ├── file_read/
    │   └── file_read.go
    ├── file_write/
    │   └── file_write.go
    ├── file_edit/
    │   └── file_edit.go
    ├── glob/
    │   └── glob.go
    ├── grep/
    │   └── grep.go
    ├── agent/
    │   └── agent.go
    ├── web_fetch/
    │   └── web_fetch.go
    ├── web_search/
    │   └── web_search.go
    ├── ask_user/
    │   └── ask_user.go
    ├── todo_write/
    │   └── todo_write.go
    ├── notebook_edit/
    │   └── notebook_edit.go
    ├── lsp/
    │   └── lsp.go
    ├── mcp/
    │   └── mcp.go
    ├── skill/
    │   └── skill.go
    ├── task_create/
    │   └── task_create.go
    ├── task_get/
    │   └── task_get.go
    ├── task_list/
    │   └── task_list.go
    ├── task_stop/
    │   └── task_stop.go
    ├── task_update/
    │   └── task_update.go
    ├── task_output/
    │   └── task_output.go
    ├── enter_plan_mode/
    │   └── enter_plan_mode.go
    ├── exit_plan_mode/
    │   └── exit_plan_mode.go
    ├── enter_worktree/
    │   └── enter_worktree.go
    ├── exit_worktree/
    │   └── exit_worktree.go
    └── [other tools]
```

## Tool Categories

### 3.1 File Operations (4 tools)
| Tool | Read-only | Destructive | Description |
|------|-----------|-------------|-------------|
| FileReadTool | ✓ | - | Read file content |
| FileWriteTool | - | ✓ | Create/overwrite file |
| FileEditTool | - | ⚠ | Edit file |
| GlobTool | ✓ | - | File pattern matching |

### 3.2 Shell Execution (2 tools)
| Tool | Platform | Description |
|------|----------|-------------|
| BashTool | Unix/Linux/Mac | Shell command execution |
| PowerShellTool | Windows | PowerShell execution |

### 3.3 Search (2 tools)
| Tool | Description |
|------|-------------|
| GrepTool | Regex search in files |
| GlobTool | Filename pattern matching |

### 3.4 Agent & Task (8 tools)
| Tool | Description |
|------|-------------|
| AgentTool | Start subagent |
| TaskCreateTool | Create task |
| TaskGetTool | Get task details |
| TaskListTool | List tasks |
| TaskUpdateTool | Update task status |
| TaskStopTool | Stop task |
| TaskOutputTool | Get task output |

### 3.5 Mode Switching (4 tools)
| Tool | Description |
|------|-------------|
| EnterPlanModeTool | Enter plan mode |
| ExitPlanModeTool | Exit plan mode |
| EnterWorktreeTool | Enter git worktree |
| ExitWorktreeTool | Exit worktree |

### 3.6 MCP (3 tools)
| Tool | Description |
|------|-------------|
| MCPTool | Execute MCP server tool |
| ListMcpResourcesTool | List MCP resources |
| ReadMcpResourceTool | Read MCP resource |

### 3.7 Communication (3 tools)
| Tool | Description |
|------|-------------|
| AskUserQuestionTool | Request user input |
| SendMessageTool | Send message to agent |
| SkillTool | Execute skill |

### 3.8 Network (2 tools)
| Tool | Description |
|------|-------------|
| WebFetchTool | Fetch web content |
| WebSearchTool | Web search |

### 3.9 Other (5+ tools)
| Tool | Description |
|------|-------------|
| NotebookEditTool | Edit Jupyter Notebook |
| LSPTool | LSP language services |
| TodoWriteTool | Todo list management |
| ConfigTool | Configuration management |
| ToolSearchTool | Search available tools |

## Implementation Details

### 3.1 Tool Interface
```go
// tool.go
type Tool interface {
    Name() string
    Aliases() []string
    InputSchema() any // JSON Schema
    OutputSchema() any // JSON Schema
    
    // Execution
    Call(ctx context.Context, input any, tctx ToolContext) (*ToolResult, error)
    
    // Description
    Description(input any) string
    Prompt() string
    
    // Status
    IsEnabled() bool
    IsConcurrencySafe(input any) bool
    IsReadOnly(input any) bool
    IsDestructive(input any) bool
    
    // Permission
    ValidateInput(input any, ctx ToolContext) (*ValidationResult, error)
    CheckPermissions(input any, ctx ToolContext) (*PermissionResult, error)
}

type ToolResult struct {
    Data        any
    NewMessages []Message
    ContextModifier func(ToolContext) ToolContext
    MCPMeta     *MCPMetadata
}

type ToolContext struct {
    CWD             string
    Options         ToolOptions
    AbortController *AbortController
    GetAppState     func() AppState
    SetAppState     func(AppState)
    ReadFileState   FileStateCache
    // ... other context fields
}
```

### 3.2 Tool Registry
```go
// registry.go
type ToolRegistry struct {
    tools map[string]Tool
}

func (r *ToolRegistry) Register(tool Tool) error
func (r *ToolRegistry) Get(name string) (Tool, bool)
func (r *ToolRegistry) List() []Tool
func (r *ToolRegistry) Filter(predicate func(Tool) bool) []Tool
```

### 3.3 Tool Builder
```go
// builder.go
type ToolBuilder struct {
    tool *BaseTool
}

func NewToolBuilder(name string) *ToolBuilder
func (b *ToolBuilder) WithAliases(aliases ...string) *ToolBuilder
func (b *ToolBuilder) WithInputSchema(schema any) *ToolBuilder
func (b *ToolBuilder) WithCall(fn func(context.Context, any, ToolContext) (*ToolResult, error)) *ToolBuilder
func (b *ToolBuilder) WithDescription(fn func(any) string) *ToolBuilder
func (b *ToolBuilder) WithPrompt(prompt string) *ToolBuilder
func (b *ToolBuilder) WithReadOnly(readOnly bool) *ToolBuilder
func (b *ToolBuilder) WithDestructive(destructive bool) *ToolBuilder
func (b *ToolBuilder) WithPermissions(fn func(any, ToolContext) (*PermissionResult, error)) *ToolBuilder
func (b *ToolBuilder) Build() Tool
```

## Validation Commands
```bash
# Build
go build ./internal/app/tools/...

# Test
go test ./internal/app/tools/... -v

# Integration test
go run ./cmd/claude
# In REPL:
# - Test /files command
# - Test file read/write
# - Test bash execution
# - Test grep/glob
```

## Parity Checklist
- [ ] All 40+ tools ported
- [ ] Tool interface matches TS
- [ ] Input validation works
- [ ] Permission checking works
- [ ] BashTool security checks
- [ ] FileReadTool device blocking
- [ ] FileEditTool string matching
- [ ] GlobTool pattern matching
- [ ] GrepTool regex search
- [ ] AgentTool subagent spawning
- [ ] WebFetchTool content fetching
- [ ] WebSearchTool search
- [ ] AskUserQuestionTool UI
- [ ] Tool progress reporting
- [ ] Result truncation and persistence

## Known Deviations
1. Go doesn't have JSX - UI rendering needs different approach
2. Progress reporting may need channel-based approach
3. Bash sandboxing may differ on Windows

## Risks
- AgentTool requires subagent process management
- LSPTool requires LSP client implementation
- MCPTool depends on MCP service layer
- Progress reporting mechanism differs

## Next Checkpoint
- [Checkpoint 4: QueryEngine](./checkpoint-4.md)