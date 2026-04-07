# Migration Checkpoints Overview

## Checkpoint Summary

| ID | Name | Priority | Dependencies | Est. Complexity |
|----|------|----------|--------------|-----------------|
| 0 | Project Setup | Pre-req | None | Low |
| 1 | Core Types & Interfaces | P0 | None | Medium |
| 2 | Command System | P0 | 1 | High |
| 3 | Tool System | P0 | 1 | High |
| 4 | QueryEngine | P0 | 1, 2, 3 | High |
| 5 | CLI & I/O | P0 | 4 | Medium |
| 6 | State Management | P1 | 1 | Medium |
| 7 | Services - API & Auth | P1 | 1 | Medium |
| 8 | Services - MCP | P1 | 1, 7 | High |
| 9 | Bridge - Remote Control | P1 | 1, 7 | High |
| 10 | UI Components | P2 | 5, 6 | High |
| 11 | Advanced Features | P2 | 4-10 | Medium |
| 12 | Integration & Polish | P2 | All above | Medium |

## Checkpoint Dependencies Graph

```
[1: Core Types]
    ├─► [2: Commands] ─────────────────────┐
    ├─► [3: Tool System] ──► [4: QueryEngine] ─► [5: CLI & I/O] ─► [10: UI]
    │                                        │
    │                                        ▼
    └─► [6: State] ───────────────────► [11: Advanced]
    └─► [7: API & Auth] ─► [8: MCP] ──┬─► [9: Bridge] ──────────────────────┘
                                        │
                                        ▼
                                    [12: Integration]
```

## Priority Guidelines

### P0 (Must Have)
- Core types and interfaces
- Command system
- Tool execution
- Query engine
- CLI I/O

### P1 (Should Have)
- State management
- API and authentication
- MCP protocol support
- Remote control (Bridge)

### P2 (Nice to Have)
- UI components
- Advanced features
- Integration polish

---

## Migration Principles

1. **Checkpoint boundaries**: Each checkpoint should be independently testable
2. **Feature parity first**: Match TS behavior before optimizing for Go idioms
3. **Incremental validation**: Run tests after each checkpoint
4. **Documentation**: Update docs/README.md after each checkpoint
5. **No shortcuts**: Don't skip complexity to meet deadlines

---

## Additional Modules Reference

See [additional-modules.md](../docs/additional-modules.md) for modules not covered in core checkpoints:

| Module | Description | Checkpoint Coverage |
|--------|-------------|---------------------|
| Coordinator | Multi-worker orchestration | Checkpoint 11 |
| Remote | CCR session management | Checkpoint 9 |
| Screens | Top-level UI screens | Checkpoint 10 |
| Voice | Voice mode feature | Optional |
| Memory (memdir) | Persistent agent memory | Checkpoint 11 |
| Keybindings | Keyboard shortcuts | Checkpoint 10 |
| Ink | React-Ink UI framework | Checkpoint 10 |
| Server | Web server | Checkpoint 7/9 |
| Vim | Vim input mode | Checkpoint 10 |
| Migrations | Settings migrations | Checkpoint 11 |
| Native TS | TypeScript native modules | Checkpoint 10 |
| Upstream Proxy | CCR network proxy | Checkpoint 9 |
| Shims | Build-time shims | Checkpoint 0 |
| More Right | Internal hook | N/A |
| Output Styles | Output styling | Checkpoint 10 |
| Schemas | Configuration schemas | Checkpoint 1 |

---

## Validation Checklist

For each checkpoint, validate:
- [ ] Code compiles without errors
- [ ] Unit tests pass
- [ ] CLI commands work
- [ ] No regression in existing features
- [ ] Documentation updated
- [ ] Checkpoint report written