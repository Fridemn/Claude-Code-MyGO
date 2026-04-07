# Checkpoint 2: Command System

## Scope
Implement the complete command system with 80+ slash commands.

## Files to Inspect

### Source Files
| File | Purpose |
|-------|---------|
| `src/commands.ts` | Command registration and loading |
| `src/commands/*/index.ts` | Command metadata (all directories) |
| `src/commands/*/*.ts` | Command implementations |
| `src/commands/*/*.tsx` | JSX command implementations |
| `src/types/command.ts` | Command type definitions |

### Target Structure
```
internal/app/commands/
├── registry.go           # Command registry
├── loader.go             # Command loading
├── filter.go             # Command filtering
├── commands/
│   ├── add_dir.go
│   ├── agents.go
│   ├── branch.go
│   ├── clear.go
│   ├── compact.go
│   ├── commit.go
│   ├── config.go
│   ├── cost.go
│   ├── diff.go
│   ├── doctor.go
│   ├── exit.go
│   ├── files.go
│   ├── help.go
│   ├── mcp.go
│   ├── memory.go
│   ├── model.go
│   ├── permissions.go
│   ├── plan.go
│   ├── session.go
│   ├── skills.go
│   ├── stats.go
│   ├── tasks.go
│   ├── theme.go
│   ├── usage.go
│   └── [other 60+ commands]
└── internal/
    └── [internal commands]
```

## Command Categories

### 2.1 Session Management (7 commands)
| Command | Type | Description |
|---------|------|-------------|
| `/clear` | local | Clear conversation history |
| `/compact` | local | Compress context |
| `/session` | local-jsx | Remote session management |
| `/resume` | local-jsx | Resume session |
| `/rewind` | local-jsx | Rewind session |
| `/summary` | local-jsx | Generate summary |
| `/export` | local-jsx | Export session |

### 2.2 Configuration (8 commands)
| Command | Type | Description |
|---------|------|-------------|
| `/config` | local-jsx | Configuration management |
| `/model` | local-jsx | Model selection |
| `/theme` | local-jsx | Theme switching |
| `/color` | local-jsx | Agent color |
| `/fast` | local-jsx | Fast mode |
| `/output-style` | local-jsx | Output style |
| `/privacy-settings` | local-jsx | Privacy settings |
| `/permissions` | local-jsx | Permission management |

### 2.3 File and Code (6 commands)
| Command | Type | Description |
|---------|------|-------------|
| `/init` | prompt | Initialize project |
| `/diff` | local-jsx | Git diff |
| `/branch` | local-jsx | Git branch |
| `/commit` | local-jsx | Git commit |
| `/review` | prompt | Code review |
| `/files` | local-jsx | File list |

### 2.4 Tools and Extensions (5 commands)
| Command | Type | Description |
|---------|------|-------------|
| `/mcp` | local-jsx | MCP server management |
| `/skills` | local-jsx | Skills management |
| `/plugins` | local-jsx | Plugin management |
| `/hooks` | local-jsx | Hook management |
| `/reload-plugins` | local | Reload plugins |

### 2.5 Development Tools (4 commands)
| Command | Type | Description |
|---------|------|-------------|
| `/doctor` | local | Health check |
| `/ide` | local-jsx | IDE integration |
| `/terminalSetup` | local-jsx | Terminal setup |
| `/keybindings` | local-jsx | Keybindings |

### 2.6 Tasks and Agents (4 commands)
| Command | Type | Description |
|---------|------|-------------|
| `/tasks` | local-jsx | Task management |
| `/agents` | local-jsx | Agent configuration |
| `/plan` | local-jsx | Plan mode |
| `/passes` | local-jsx | Verification execution |

### 2.7 Statistics (4 commands)
| Command | Type | Description |
|---------|------|-------------|
| `/cost` | local | Cost statistics |
| `/usage` | local-jsx | Usage statistics |
| `/stats` | local-jsx | Detailed stats |
| `/effort` | local-jsx | Effort estimation |

### 2.8 Remote and Collaboration (5 commands)
| Command | Type | Description |
|---------|------|-------------|
| `/remote-setup` | local-jsx | Remote setup |
| `/teleport` | local-jsx | Remote teleport |
| `/feedback` | local-jsx | Feedback |
| `/share` | local-jsx | Share session |
| `/mobile` | local-jsx | Mobile |

## Implementation Details

### 2.1 Command Registry
```go
// registry.go
type CommandRegistry struct {
    commands map[string]Command
    aliases  map[string]string
}

func (r *CommandRegistry) Register(cmd Command) error
func (r *CommandRegistry) Get(name string) (Command, bool)
func (r *CommandRegistry) GetByAlias(alias string) (Command, bool)
func (r *CommandRegistry) List() []Command
func (r *CommandRegistry) Filter(predicate func(Command) bool) []Command
```

### 2.2 Command Loader
```go
// loader.go
type CommandLoader struct {
    registry *CommandRegistry
}

func (l *CommandLoader) LoadBuiltIn() error
func (l *CommandLoader) LoadFromDirectory(dir string) error
func (l *CommandLoader) LoadFromPlugin(plugin Plugin) error
```

### 2.3 Command Filters
```go
// filter.go
func FilterByAvailability(cmds []Command, availability CommandAvailability) []Command
func FilterByEnabled(cmds []Command) []Command
func FilterByHidden(cmds []Command, includeHidden bool) []Command
func FilterRemoteSafe(cmds []Command) []Command
func FilterBridgeSafe(cmds []Command) []Command
```

## Validation Commands
```bash
# Build
go build ./internal/app/commands/...

# Test
go test ./internal/app/commands/... -v

# Run CLI and test commands
go run ./cmd/claude --help
go run ./cmd/claude /help
go run ./cmd/claude /doctor
go run ./cmd/claude /cost
```

## Parity Checklist
- [ ] All 80+ commands ported
- [ ] Command types match (local, local-jsx, prompt)
- [ ] Aliases work correctly
- [ ] Availability filtering works
- [ ] Remote-safe filtering works
- [ ] Bridge-safe filtering works
- [ ] Command descriptions match
- [ ] Argument hints match
- [ ] Immediate flag behavior correct
- [ ] Hidden flag behavior correct

## Known Deviations
1. JSX commands will need different UI approach in Go (likely terminal UI library)
2. Prompt commands need API integration
3. Dynamic descriptions require closure handling

## Risks
- JSX commands require terminal UI framework (e.g., bubbletea, tcell)
- Large number of commands may slow initial loading
- Some commands have complex dependencies

## Next Checkpoint
- [Checkpoint 3: Tool System](./checkpoint-3.md)