# Additional Modules Documentation

This document covers modules not included in the core modules documentation.

## Table of Contents

1. [Coordinator Module](#coordinator-module)
2. [Remote Module](#remote-module)
3. [Screens Module](#screens-module)
4. [Voice Module](#voice-module)
5. [Memory Directory (memdir) Module](#memory-directory-memdir-module)
6. [Keybindings Module](#keybindings-module)
7. [Ink Framework Module](#ink-framework-module)
8. [Server Module](#server-module)
9. [Vim Module](#vim-module)
10. [Migrations Module](#migrations-module)
11. [Native TypeScript Module](#native-typescript-module)
12. [Upstream Proxy Module](#upstream-proxy-module)
13. [Shims Module](#shims-module)
14. [More Right Module](#more-right-module)
15. [Output Styles Module](#output-styles-module)
16. [Schemas Module](#schemas-module)

---

## Coordinator Module

**Location:** `src/coordinator/`

### Purpose
Implements coordinator mode for orchestrating multiple worker agents. The coordinator delegates tasks to workers and synthesizes results.

### Files

| File | Purpose |
|------|---------|
| `coordinatorMode.ts` | Main coordinator mode logic |

### Key Functions

```typescript
// Check if coordinator mode is enabled
function isCoordinatorMode(): boolean

// Match session mode on resume
function matchSessionMode(sessionMode: 'coordinator' | 'normal' | undefined): string | undefined

// Get user context for coordinator mode
function getCoordinatorUserContext(
  mcpClients: ReadonlyArray<{ name: string }>,
  scratchpadDir?: string,
): { [k: string]: string }

// Get system prompt for coordinator
function getCoordinatorSystemPrompt(): string
```

### Coordinator Mode Features

1. **Worker Orchestration**: Spawns and manages worker agents via AgentTool
2. **Task Delegation**: Sends tasks to workers via SendMessage tool
3. **Result Synthesis**: Collects and synthesizes worker results
4. **Parallel Execution**: Runs multiple workers concurrently
5. **Task Stopping**: Can stop workers via TaskStop tool

### Worker Tools
Workers have access to:
- Bash, Read, Edit tools (simple mode)
- Standard tools + MCP tools (normal mode)
- Project skills via Skill tool

### Target Structure (Go)
```
internal/app/coordinator/
├── coordinator.go       # Main coordinator logic
├── worker.go           # Worker management
└── prompts.go          # System prompt generation
```

---

## Remote Module

**Location:** `src/remote/`

### Purpose
Implements remote session management for CCR (Claude Code Remote) sessions. Manages WebSocket connections and message translation.

### Files

| File | Purpose |
|------|---------|
| `RemoteSessionManager.ts` | Manages remote CCR sessions |
| `SessionsWebSocket.ts` | WebSocket client for session subscription |
| `sdkMessageAdapter.ts` | Converts SDK messages to REPL format |
| `remotePermissionBridge.ts` | Permission handling for remote tools |

### Key Types

```typescript
// Remote session configuration
type RemoteSessionConfig = {
  sessionId: string
  getAccessToken: () => string
  orgUuid: string
  hasInitialPrompt?: boolean
  viewerOnly?: boolean  // True for "claude assistant" mode
}

// Permission response
type RemotePermissionResponse =
  | { behavior: 'allow'; updatedInput: Record<string, unknown> }
  | { behavior: 'deny'; message: string }
```

### RemoteSessionManager

```typescript
class RemoteSessionManager {
  constructor(config: RemoteSessionConfig, callbacks: RemoteSessionCallbacks)
  
  // Connect to remote session via WebSocket
  connect(): void
  
  // Send user message via HTTP POST
  sendMessage(content: RemoteMessageContent, opts?: { uuid?: string }): Promise<boolean>
  
  // Respond to permission request
  respondToPermissionRequest(requestId: string, result: RemotePermissionResponse): void
  
  // Send interrupt signal
  cancelSession(): void
  
  // Disconnect from session
  disconnect(): void
}
```

### SessionsWebSocket

```typescript
class SessionsWebSocket {
  // WebSocket connection to /v1/sessions/ws/{id}/subscribe
  // Handles reconnection, ping/pong, and message parsing
  
  connect(): Promise<void>
  sendControlResponse(response: SDKControlResponse): void
  sendControlRequest(request: SDKControlRequestInner): void
  close(): void
  reconnect(): void
}
```

### Message Flow
1. WebSocket subscribes to session events
2. SDK messages arrive via WebSocket
3. `sdkMessageAdapter` converts to REPL Message types
4. Permission requests handled via control messages
5. User messages sent via HTTP POST

### Target Structure (Go)
```
internal/infra/remote/
├── session_manager.go    # RemoteSessionManager
├── websocket.go         # WebSocket client
├── message_adapter.go   # SDK message conversion
└── permission_bridge.go # Permission handling
```

---

## Screens Module

**Location:** `src/screens/`

### Purpose
Top-level screen components for different application modes.

### Files

| File | Purpose |
|------|---------|
| `REPL.tsx` | Main REPL screen (875KB - core application) |
| `Doctor.tsx` | Diagnostics screen |
| `ResumeConversation.tsx` | Session resume screen |

### REPL Screen
The main application screen containing:
- Message display and rendering
- Prompt input handling
- Tool result visualization
- Status line updates
- Keybinding handling

### Doctor Screen
Displays diagnostics:
- Installation status
- Version information
- Update channel
- Environment validation
- Sandbox status
- MCP parsing warnings
- Keybinding warnings
- Version locks

### ResumeConversation Screen
Handles session resumption:
- Session log loading
- Session selection UI
- Cross-project resume detection
- Session state restoration
- Agent definition restoration

### Target Structure (Go)
```
internal/ui/screens/
├── repl.go           # Main REPL screen
├── doctor.go         # Diagnostics screen
└── resume.go         # Session resume screen
```

---

## Voice Module

**Location:** `src/voice/`

### Purpose
Voice mode feature flags and authentication checks.

### Files

| File | Purpose |
|------|---------|
| `voiceModeEnabled.ts` | Voice mode feature detection |

### Key Functions

```typescript
// Check GrowthBook kill-switch
function isVoiceGrowthBookEnabled(): boolean

// Check auth status (requires Anthropic OAuth)
function hasVoiceAuth(): boolean

// Full runtime check (auth + GrowthBook)
function isVoiceModeEnabled(): boolean
```

### Notes
- Voice mode requires Anthropic OAuth
- Uses `voice_stream` endpoint on claude.ai
- Not available with API keys, Bedrock, Vertex, or Foundry

### Target Structure (Go)
```
internal/features/voice/
└── enabled.go    # Voice mode checks
```

---

## Memory Directory (memdir) Module

**Location:** `src/memdir/`

### Purpose
Implements the typed-memory system for persistent agent memory.

### Files

| File | Purpose |
|------|---------|
| `memdir.ts` | Main memory prompt builder |
| `memoryTypes.ts` | Memory type definitions and guidance |
| `paths.ts` | Memory directory path resolution |
| `memoryScan.ts` | Memory file scanning |
| `findRelevantMemories.ts` | Memory search functionality |
| `teamMemPaths.ts` | Team memory paths |
| `teamMemPrompts.ts` | Team memory prompts |
| `memoryAge.ts` | Memory age tracking |

### Memory Types
Four closed types:
1. **user**: User role, goals, preferences
2. **feedback**: Behavior guidance (what to avoid/repeat)
3. **project**: Project context not derivable from code
4. **reference**: Pointers to external systems

### Key Functions

```typescript
// Load memory prompt for system prompt
function loadMemoryPrompt(): Promise<string | null>

// Build memory prompt with content
function buildMemoryPrompt(params: {
  displayName: string
  memoryDir: string
  extraGuidelines?: string[]
}): string

// Build memory lines (without MEMORY.md content)
function buildMemoryLines(
  displayName: string,
  memoryDir: string,
  extraGuidelines?: string[],
  skipIndex?: boolean,
): string[]

// Ensure memory directory exists
function ensureMemoryDirExists(memoryDir: string): Promise<void>

// Truncate MEMORY.md to limits
function truncateEntrypointContent(raw: string): EntrypointTruncation
```

### Constants
- `ENTRYPOINT_NAME`: "MEMORY.md"
- `MAX_ENTRYPOINT_LINES`: 200
- `MAX_ENTRYPOINT_BYTES`: 25,000

### Target Structure (Go)
```
internal/app/memory/
├── memdir.go         # Memory prompt building
├── types.go          # Memory type definitions
├── paths.go          # Path resolution
├── scan.go           # Memory scanning
└── team/
    ├── paths.go      # Team memory paths
    └── prompts.go    # Team memory prompts
```

---

## Keybindings Module

**Location:** `src/keybindings/`

### Purpose
Keyboard shortcut system with context-aware binding resolution.

### Files

| File | Purpose |
|------|---------|
| `useKeybinding.ts` | React hooks for keybindings |
| `KeybindingContext.tsx` | React context for keybinding state |
| `KeybindingProviderSetup.tsx` | Provider component |
| `resolver.ts` | Binding resolution logic |
| `parser.ts` | Keystroke parsing |
| `matcher.ts` | Key event matching |
| `defaultBindings.ts` | Default keybindings |
| `loadUserBindings.ts` | User configuration loading |
| `schema.ts` | Configuration schema |
| `validate.ts` | Validation logic |
| `shortcutFormat.ts` | Shortcut display formatting |
| `useShortcutDisplay.ts` | Hook for shortcut display |
| `reservedShortcuts.ts` | Reserved shortcuts |
| `template.ts` | Binding templates |

### Key Hooks

```typescript
// Handle single keybinding
function useKeybinding(
  action: string,
  handler: () => void | false | Promise<void>,
  options?: { context?: KeybindingContextName; isActive?: boolean },
): void

// Handle multiple keybindings
function useKeybindings(
  handlers: Record<string, () => void | false | Promise<void>>,
  options?: { context?: KeybindingContextName; isActive?: boolean },
): void
```

### Chord Support
Supports chord sequences like "ctrl+k ctrl+s":
1. First key starts chord
2. Second key completes or cancels
3. `ChordInterceptor` shows pending chord indicator

### Context Priority
1. Registered active contexts
2. Specified context
3. 'Global' fallback

### Target Structure (Go)
```
internal/ui/keybindings/
├── hooks.go          # Keybinding hooks (if using bubbletea)
├── resolver.go       # Binding resolution
├── parser.go         # Keystroke parsing
├── matcher.go        # Event matching
├── defaults.go       # Default bindings
├── config.go         # User config loading
└── context.go        # Context management
```

---

## Ink Framework Module

**Location:** `src/ink/`

### Purpose
React-Ink terminal UI framework with custom rendering pipeline.

### Files

| File | Purpose |
|------|---------|
| `ink.tsx` | Main Ink entry point |
| `renderer.ts` | Custom renderer |
| `render-to-screen.ts` | Screen rendering |
| `render-node-to-output.ts` | Node rendering |
| `reconciler.ts` | React reconciler |
| `termio.ts` | Terminal I/O |
| `screen.ts` | Screen buffer management |
| `measure-text.ts` | Text measurement |
| `measure-element.ts` | Element measurement |
| `get-max-width.ts` | Width calculation |
| `stringWidth.ts` | String width calculation |
| `supports-hyperlinks.ts` | Hyperlink support detection |
| `clearTerminal.ts` | Terminal clearing |
| `Ansi.tsx` | ANSI component |
| `render-border.ts` | Border rendering |

### Components

| Component | Purpose |
|-----------|---------|
| `App.tsx` | Root app component |
| `Box.tsx` | Layout container |
| `Text.tsx` | Text rendering |
| `NoSelect.tsx` | Non-selectable content |
| `TerminalSizeContext.tsx` | Terminal size context |
| `ErrorOverview.tsx` | Error display |

### Renderer Pipeline
1. Yoga layout calculation
2. Node-to-output rendering
3. Screen buffer diffing
4. ANSI output generation
5. Terminal cursor management

### Target Structure (Go)
```
internal/ui/ink/
├── renderer.go       # Custom renderer
├── screen.go         # Screen buffer
├── termio.go         # Terminal I/O
├── layout/          # Yoga layout (or use Go equivalent)
│   ├── yoga.go
│   └── measure.go
└── components/
    ├── box.go
    ├── text.go
    └── app.go
```

**Note:** In Go, consider using `bubbletea` instead of porting Ink directly.

---

## Server Module

**Location:** `src/server/`

### Purpose
Web server and direct connect session management.

### Files

| File | Purpose |
|------|---------|
| `types.ts` | Server types |
| `directConnectManager.ts` | Direct connect management |
| `createDirectConnectSession.ts` | Session creation |

### Web Server Files

| File | Purpose |
|------|---------|
| `web/auth.ts` | Authentication |
| `web/session-store.ts` | Session storage |
| `web/terminal.ts` | Terminal handling |
| `web/admin.ts` | Admin endpoints |
| `web/pty-server.ts` | PTY management |
| `web/session-manager.ts` | Session management |
| `web/user-store.ts` | User storage |
| `web/scrollback-buffer.ts` | Scrollback buffer |

### Key Types

```typescript
type ServerConfig = {
  port: number
  host: string
  authToken: string
  unix?: string
  idleTimeoutMs?: number
  maxSessions?: number
  workspace?: string
}

type SessionState = 'starting' | 'running' | 'detached' | 'stopping' | 'stopped'

type SessionInfo = {
  id: string
  status: SessionState
  createdAt: number
  workDir: string
  process: ChildProcess | null
  sessionKey?: string
}
```

### Authentication Methods
- OAuth authentication
- API key authentication
- Token authentication

### Target Structure (Go)
```
internal/infra/server/
├── server.go         # Main server
├── config.go         # Server config
├── session.go        # Session management
├── auth/
│   ├── oauth.go
│   ├── apikey.go
│   └── token.go
└── web/
    ├── terminal.go
    ├── pty.go
    └── admin.go
```

---

## Vim Module

**Location:** `src/vim/`

### Purpose
Vim mode implementation for prompt input.

### Files

| File | Purpose |
|------|---------|
| `types.ts` | Vim state machine types |
| `transitions.ts` | State transitions |
| `motions.ts` | Motion commands |
| `operators.ts` | Operator commands |
| `textObjects.ts` | Text object handling |

### State Machine

```
VimState
├── INSERT (tracks insertedText)
└── NORMAL (CommandState machine)
    ├── idle
    ├── count
    ├── operator
    ├── operatorCount
    ├── operatorFind
    ├── operatorTextObj
    ├── find
    ├── g
    ├── operatorG
    ├── replace
    └── indent
```

### Key Types

```typescript
type Operator = 'delete' | 'change' | 'yank'
type FindType = 'f' | 'F' | 't' | 'T'
type TextObjScope = 'inner' | 'around'

type VimState =
  | { mode: 'INSERT'; insertedText: string }
  | { mode: 'NORMAL'; command: CommandState }

type PersistentState = {
  lastChange: RecordedChange | null
  lastFind: { type: FindType; char: string } | null
  register: string
  registerIsLinewise: boolean
}
```

### Supported Operations
- Basic motions: h, l, j, k, w, b, e, W, B, E
- Line positions: 0, ^, $
- Operators: d (delete), c (change), y (yank)
- Text objects: w, W, quotes, parens, brackets, braces
- Find: f, F, t, T
- Replace: r
- Indent: >, <
- Count prefix

### Target Structure (Go)
```
internal/ui/vim/
├── types.go          # Vim types
├── state.go          # State machine
├── transitions.go    # State transitions
├── motions.go        # Motion handling
├── operators.go      # Operator handling
└── text_objects.go   # Text object handling
```

---

## Migrations Module

**Location:** `src/migrations/`

### Purpose
Settings migrations for backward compatibility.

### Files

| File | Purpose |
|------|---------|
| `migrateSonnet45ToSonnet46.ts` | Model migration |
| `migrateSonnet1mToSonnet45.ts` | Model migration |
| `migrateFennecToOpus.ts` | Model migration |
| `migrateLegacyOpusToCurrent.ts` | Model migration |
| `migrateOpusToOpus1m.ts` | Model migration |
| `resetProToOpusDefault.ts` | Default model reset |
| `resetAutoModeOptInForDefaultOffer.ts` | Auto mode opt-in |
| `migrateBypassPermissionsAcceptedToSettings.ts` | Permission settings |
| `migrateEnableAllProjectMcpServersToSettings.ts` | MCP settings |
| `migrateAutoUpdatesToSettings.ts` | Update settings |
| `migrateReplBridgeEnabledToRemoteControlAtStartup.ts` | Bridge settings |

### Migration Pattern

```typescript
function migrateSonnet45ToSonnet46(): void {
  // Check provider
  if (getAPIProvider() !== 'firstParty') return
  
  // Check subscription tier
  if (!isProSubscriber() && !isMaxSubscriber()) return
  
  // Get current settings
  const model = getSettingsForSource('userSettings')?.model
  
  // Check if migration applies
  if (model !== 'claude-sonnet-4-5-20250929') return
  
  // Apply migration
  updateSettingsForSource('userSettings', { model: 'sonnet' })
  
  // Log event
  logEvent('tengu_sonnet45_to_46_migration', { from_model: model })
}
```

### Target Structure (Go)
```
internal/app/migrations/
├── registry.go       # Migration registry
├── runner.go         # Migration runner
└── migrations/
    ├── model_migration.go
    ├── settings_migration.go
    └── ...
```

---

## Native TypeScript Module

**Location:** `src/native-ts/`

### Purpose
Pure TypeScript implementations of native modules.

### Subdirectories

| Directory | Purpose |
|-----------|---------|
| `color-diff/` | Syntax highlighting and diff rendering |
| `file-index/` | File indexing |
| `yoga-layout/` | Yoga layout bindings |

### Color Diff Module
Pure TypeScript port of vendor/color-diff-src:
- Syntax highlighting via highlight.js
- Word diff via `diff` package
- Theme support (Monokai, GitHub, ANSI)
- ANSI color output

### Key Classes

```typescript
class ColorDiff {
  constructor(hunk: Hunk, firstLine: string | null, filePath: string)
  render(themeName: string, width: number, dim: boolean): string[] | null
}

class ColorFile {
  constructor(code: string, filePath: string)
  render(themeName: string, width: number, dim: boolean): string[] | null
}
```

### Target Structure (Go)
```
internal/native/
├── color_diff.go     # Port or use existing Go libraries
├── file_index.go     # File indexing
└── layout.go         # Yoga layout or equivalent
```

**Note:** Consider using Go-native libraries like:
- `chroma` for syntax highlighting
- `go-diff` for diffing
- `flex` or CSS grid libraries for layout

---

## Upstream Proxy Module

**Location:** `src/upstreamproxy/`

### Purpose
CCR upstream proxy for container-side network interception.

### Files

| File | Purpose |
|------|---------|
| `upstreamproxy.ts` | Main proxy initialization |
| `relay.ts` | CONNECT→WebSocket relay |

### Key Functions

```typescript
// Initialize upstream proxy
async function initUpstreamProxy(opts?: {
  tokenPath?: string
  systemCaPath?: string
  caBundlePath?: string
  ccrBaseUrl?: string
}): Promise<UpstreamProxyState>

// Get env vars for subprocesses
function getUpstreamProxyEnv(): Record<string, string>
```

### Process
1. Read session token from `/run/ccr/session_token`
2. Set `prctl(PR_SET_DUMPABLE, 0)` to block ptrace
3. Download CA cert and concatenate with system bundle
4. Start CONNECT→WebSocket relay
5. Unlink token file
6. Export HTTPS_PROXY / SSL_CERT_FILE env vars

### Target Structure (Go)
```
internal/infra/proxy/
├── upstream.go       # Proxy initialization
└── relay.go          # WebSocket relay
```

---

## Shims Module

**Location:** `src/shims/`

### Purpose
Build-time shims for external builds.

### Files

| File | Purpose |
|------|---------|
| `macro.ts` | Version and package info shim |
| `bun-bundle.ts` | Bun bundle configuration |
| `preload.ts` | Preload script |

### Macro Shim

```typescript
const MACRO_OBJ = {
  VERSION: version,      // From package.json
  PACKAGE_URL: '@anthropic-ai/claude-code',
  ISSUES_EXPLAINER: 'report issues at https://github.com/anthropics/claude-code/issues',
}

globalThis.MACRO = MACRO_OBJ
```

### Target Structure (Go)
```
// In Go, these would be compile-time constants or build-time variables
internal/build/
└── info.go          // Build info
```

---

## More Right Module

**Location:** `src/moreright/`

### Purpose
Hook for "more right" functionality (internal only, stubbed for external builds).

### Files

| File | Purpose |
|------|---------|
| `useMoreRight.tsx` | More right hook (stub for external) |

### Hook Interface

```typescript
function useMoreRight(args: {
  enabled: boolean
  setMessages: (action: M[] | ((prev: M[]) => M[])) => void
  inputValue: string
  setInputValue: (s: string) => void
  setToolJSX: (args: M) => void
}): {
  onBeforeQuery: (input: string, all: M[], n: number) => Promise<boolean>
  onTurnComplete: (all: M[], aborted: boolean) => Promise<void>
  render: () => null
}
```

### Target Structure (Go)
```
// Internal only - may not need Go equivalent
```

---

## Output Styles Module

**Location:** `src/outputStyles/`

### Purpose
Output style directory loading.

### Files

| File | Purpose |
|------|---------|
| `loadOutputStylesDir.ts` | Load output style configurations |

### Target Structure (Go)
```
internal/ui/styles/
└── loader.go        # Style loading
```

---

## Schemas Module

**Location:** `src/schemas/`

### Purpose
Configuration schema definitions.

### Files

| File | Purpose |
|------|---------|
| `hooks.ts` | Hook schema definitions |

### Target Structure (Go)
```
internal/schemas/
└── hooks.go         # Hook schemas (or use Go struct tags)
```

---

## Entrypoints Module

**Location:** `src/entrypoints/`

### Purpose
Application entry points and SDK type definitions.

### Files

| File | Purpose |
|------|---------|
| `cli.tsx` | CLI entry point |
| `init.ts` | Application initialization |
| `agentSdkTypes.ts` | SDK type exports |
| `mcp.ts` | MCP entry point |
| `sandboxTypes.ts` | Sandbox type definitions |

### SDK Subdirectory (`entrypoints/sdk/`)

| File | Purpose |
|------|---------|
| `coreTypes.ts` | Core serializable types |
| `runtimeTypes.ts` | Runtime types (callbacks, interfaces) |
| `controlTypes.ts` | Control protocol types |
| `toolTypes.ts` | Tool-related types |
| `coreSchemas.ts` | Core JSON schemas |
| `controlSchemas.ts` | Control JSON schemas |
| `settingsTypes.generated.ts` | Generated settings types |

### Target Structure (Go)
```
internal/entrypoints/
├── cli.go            # CLI entry
├── init.go           # Initialization
└── sdk/
    ├── types.go      # SDK types
    ├── control.go    # Control types
    └── tool.go       # Tool types
```

---

## Context Module

**Location:** `src/context/`

### Purpose
React context providers for application state.

### Files

| File | Purpose |
|------|---------|
| `notifications.tsx` | Notification context |
| `QueuedMessageContext.tsx` | Queued message context |
| `promptOverlayContext.tsx` | Prompt overlay state |
| `fpsMetrics.tsx` | FPS metrics |
| `voice.tsx` | Voice context |
| `overlayContext.tsx` | Overlay state |
| `modalContext.tsx` | Modal management |
| `mailbox.tsx` | Message mailbox |
| `stats.tsx` | Statistics context |

### Target Structure (Go)
```
internal/ui/context/
├── notifications.go
├── mailbox.go
├── modal.go
└── overlay.go
```

---

## Skills Module

**Location:** `src/skills/`

### Purpose
Skill system for user-defined capabilities.

### Files

| File | Purpose |
|------|---------|
| `bundledSkills.ts` | Bundled skill loader |
| `loadSkillsDir.ts` | User skill directory loader |
| `mcpSkillBuilders.ts` | MCP skill builders |

### Bundled Skills (`skills/bundled/`)

| Skill | Purpose |
|-------|---------|
| `verify.ts` | Code verification |
| `debug.ts` | Debugging assistance |
| `commit.ts` | Git commit |
| `remember.ts` | Memory storage |
| `loop.ts` | Loop execution |
| `batch.ts` | Batch operations |
| `keybindings.ts` | Keybinding config |
| `claudeApi.ts` | API interaction |
| `skillify.ts` | Skill creation |
| `stuck.ts` | Debug stuck states |
| `simplify.ts` | Code simplification |

### Target Structure (Go)
```
internal/app/skills/
├── loader.go         # Skill loader
├── registry.go       # Skill registry
└── bundled/
    ├── verify.go
    ├── debug.go
    └── ...
```

---

## Query Module

**Location:** `src/query/`

### Purpose
Query engine configuration and lifecycle.

### Files

| File | Purpose |
|------|---------|
| `config.ts` | Query configuration |
| `deps.ts` | Query dependencies |
| `transitions.ts` | State transitions |
| `stopHooks.ts` | Stop hooks |
| `tokenBudget.ts` | Token budget management |

### Target Structure (Go)
```
internal/app/query/
├── config.go         # Query config
├── transitions.go    # State transitions
├── stop_hooks.go     # Stop hooks
└── token_budget.go   # Token management
```

---

## Buddy Module

**Location:** `src/buddy/`

### Purpose
Companion sprite system for UI personality.

### Files

| File | Purpose |
|------|---------|
| `types.ts` | Buddy types |
| `companion.ts` | Companion logic |
| `CompanionSprite.tsx` | Sprite component |
| `useBuddyNotification.tsx` | Notification hook |
| `sprites.ts` | Sprite definitions |
| `prompt.ts` | Buddy prompts |

### Target Structure (Go)
```
internal/ui/buddy/
├── companion.go
├── sprites.go
└── prompt.go
```

---

## Plugins Module

**Location:** `src/plugins/`

### Purpose
Plugin system for extensibility.

### Files

| File | Purpose |
|------|---------|
| `builtinPlugins.ts` | Built-in plugin definitions |
| `bundled/index.ts` | Bundled plugin loader |

### Target Structure (Go)
```
internal/app/plugins/
├── builtin.go        # Built-in plugins
└── loader.go         # Plugin loader
```

---

## Assistant Module

**Location:** `src/assistant/`

### Purpose
Assistant-specific functionality.

### Files

| File | Purpose |
|------|---------|
| `sessionHistory.ts` | Session history management |

### Target Structure (Go)
```
internal/app/assistant/
└── history.go        # Session history
```

---

## Key Standalone Files

### main.tsx (4709 lines)
Main application entry point with:
- Application bootstrap
- Component tree setup
- Global configuration
- Error boundaries

### query.ts (1730 lines)
Query management:
- Query creation
- Lifecycle management
- API interaction

### interactiveHelpers.tsx (385 lines)
Interactive UI helpers:
- Dialog launching
- Prompt handling
- User interaction

### setup.ts
Application setup:
- Environment configuration
- Feature initialization
- Global state setup

### costHook.ts / cost-tracker.ts
Cost tracking:
- API cost calculation
- Usage statistics

### history.ts
History management:
- Command history
- Navigation state

---

## Summary

### Priority for Go Migration

| Priority | Module | Reason |
|----------|--------|--------|
| High | Coordinator | Core agent orchestration |
| High | Remote | CCR support |
| High | Screens | Main UI |
| High | Keybindings | User interaction |
| High | Ink | UI rendering |
| High | Vim | Editor functionality |
| Medium | Memory | Agent persistence |
| Medium | Server | Web interface |
| Medium | Migrations | Settings compatibility |
| Low | Voice | Optional feature |
| Low | Upstream Proxy | CCR-specific |
| Low | Shims | Build-time only |
| Low | Native TS | May use Go alternatives |
| Low | More Right | Internal only |
| Low | Output Styles | Optional |
| Low | Schemas | Configuration |

### Go Library Recommendations

| TypeScript Module | Go Alternative |
|-------------------|----------------|
| Ink | bubbletea, tview |
| highlight.js | chroma |
| diff | go-diff |
| Yoga | flex, go-css |
| WebSocket | gorilla/websocket |
| React Context | Dependency injection |

---

## Next Steps

1. Add these modules to checkpoint documentation
2. Update checkpoint dependencies
3. Create Go structure plans for each module
4. Identify shared types and interfaces