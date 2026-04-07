# Checkpoint 11: Advanced Features

## Scope
Implement advanced features that depend on previous checkpoints.

## Features

### 11.1 Context Management
- Context building
- System prompt generation
- Token estimation

### 11.2 Compact Services
- Auto context compaction
- Micro compact (tool result folding)
- Compaction scheduling

### 11.3 LSP Services
- LSP server management
- LSP client implementation
- Diagnostic registry

### 11.4 Session Memory
- Session memory storage
- Memory extraction
- Team memory sync

### 11.5 Skills System
- Skill loading
- Skill execution
- Skill registry

### 11.6 Plugins System
- Plugin loading
- Plugin management
- Plugin API

## Files to Inspect

| Category | Source |
|----------|--------|
| Context | `src/context/*.ts` |
| Compact | `src/services/compact/*.ts` |
| LSP | `src/services/lsp/*.ts` |
| Memory | `src/services/SessionMemory/*.ts` |
| Skills | `src/skills/*.ts` |
| Plugins | `src/plugins/*.ts` |

## Target Structure
```
internal/app/
├── context/            # Context building
│   ├── system_prompt.go
│   └── token_estimation.go
├── compact/           # Compaction services
│   ├── compact.go
│   ├── auto_compact.go
│   └── micro_compact.go
├── lsp/               # LSP services
│   ├── server.go
│   ├── client.go
│   └── diagnostics.go
├── memory/            # Memory services
│   ├── session_memory.go
│   └── team_memory.go
├── skills/           # Skills
│   ├── registry.go
│   └── executor.go
└── plugins/          # Plugins
    ├── loader.go
    └── manager.go
```

## Parity Checklist
- [ ] System prompt generation
- [ ] Token estimation
- [ ] Auto compaction
- [ ] Micro compaction
- [ ] LSP server management
- [ ] LSP client
- [ ] Session memory
- [ ] Skills system
- [ ] Plugins system

## Next Checkpoint
- [Checkpoint 12: Integration & Polish](./checkpoint-12.md)