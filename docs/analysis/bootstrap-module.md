# Bootstrap 模块详细文档

## 目录

1. [概述](#1-概述)
2. [State 类型定义](#2-state-类型定义)
3. [状态分类](#3-状态分类)
4. [Getter/Setter API](#4-gettersetter-api)
5. [核心功能](#5-核心功能)
6. [设计原则](#6-设计原则)

---

## 1. 概述

Bootstrap 模块是 Claude Code 的全局状态管理中心，提供跨模块共享的状态存储和访问接口。该模块是整个应用的基础设施层，位于依赖图的叶子节点。

### 文件结构

```
src/bootstrap/
└── state.ts    # 全局状态管理（~1500行）
```

### 核心职责

1. **会话状态**: sessionId、cwd、projectRoot 等
2. **成本追踪**: totalCostUSD、modelUsage 等
3. **遥测状态**: Meter、Counter、Logger 等
4. **会话级标志**: 各种 session-only 标志
5. **缓存状态**: 各种缓存和 latch 状态

---

## 2. State 类型定义

### 完整类型

```typescript
type State = {
  // === 路径状态 ===
  originalCwd: string                    // 启动时的工作目录
  projectRoot: string                    // 稳定的项目根目录
  cwd: string                            // 当前工作目录
  
  // === 成本与使用量 ===
  totalCostUSD: number                   // 总成本（美元）
  totalAPIDuration: number               // API 总耗时
  totalAPIDurationWithoutRetries: number // 不含重试的 API 耗时
  totalToolDuration: number              // 工具执行总耗时
  turnHookDurationMs: number             // 当前 turn hook 耗时
  turnToolDurationMs: number             // 当前 turn 工具耗时
  turnClassifierDurationMs: number       // 分类器耗时
  turnToolCount: number                  // 当前 turn 工具数
  turnHookCount: number                  // 当前 turn hook 数
  turnClassifierCount: number            // 分类器调用数
  startTime: number                      // 会话开始时间
  lastInteractionTime: number            // 最后交互时间
  totalLinesAdded: number                // 添加的代码行数
  totalLinesRemoved: number              // 删除的代码行数
  hasUnknownModelCost: boolean           // 是否有未知模型成本
  modelUsage: { [modelName: string]: ModelUsage }
  
  // === 模型状态 ===
  mainLoopModelOverride: ModelSetting | undefined
  initialMainLoopModel: ModelSetting
  modelStrings: ModelStrings | null
  
  // === 会话状态 ===
  sessionId: SessionId                   // 当前会话 ID
  parentSessionId: SessionId | undefined // 父会话 ID（会话链）
  isInteractive: boolean                 // 是否交互模式
  kairosActive: boolean                  // Kairos 激活状态
  strictToolResultPairing: boolean       // 严格工具结果配对
  clientType: string                     // 客户端类型
  sessionSource: string | undefined      // 会话来源
  
  // === 认证状态 ===
  sessionIngressToken: string | null | undefined
  oauthTokenFromFd: string | null | undefined
  apiKeyFromFd: string | null | undefined
  
  // === 设置状态 ===
  allowedSettingSources: SettingSource[]
  flagSettingsPath: string | undefined
  flagSettingsInline: Record<string, unknown> | null
  
  // === 遥测状态 ===
  meter: Meter | null
  sessionCounter: AttributedCounter | null
  locCounter: AttributedCounter | null
  prCounter: AttributedCounter | null
  commitCounter: AttributedCounter | null
  costCounter: AttributedCounter | null
  tokenCounter: AttributedCounter | null
  codeEditToolDecisionCounter: AttributedCounter | null
  activeTimeCounter: AttributedCounter | null
  statsStore: { observe(name: string, value: number): void } | null
  
  // === 日志状态 ===
  loggerProvider: LoggerProvider | null
  eventLogger: ReturnType<typeof logs.getLogger> | null
  meterProvider: MeterProvider | null
  tracerProvider: BasicTracerProvider | null
  
  // === Agent 状态 ===
  agentColorMap: Map<string, AgentColorName>
  agentColorIndex: number
  
  // === 请求追踪 ===
  lastAPIRequest: Omit<BetaMessageStreamParams, 'messages'> | null
  lastAPIRequestMessages: BetaMessageStreamParams['messages'] | null
  lastClassifierRequests: unknown[] | null
  lastMainRequestId: string | undefined
  lastApiCompletionTimestamp: number | null
  promptId: string | null
  
  // === 缓存状态 ===
  cachedClaudeMdContent: string | null
  systemPromptSectionCache: Map<string, string | null>
  planSlugCache: Map<string, string>
  
  // === 会话级标志 ===
  inMemoryErrorLog: Array<{ error: string; timestamp: string }>
  inlinePlugins: Array<string>
  chromeFlagOverride: boolean | undefined
  useCoworkPlugins: boolean
  sessionBypassPermissionsMode: boolean
  scheduledTasksEnabled: boolean
  sessionCronTasks: SessionCronTask[]
  sessionCreatedTeams: Set<string>
  sessionTrustAccepted: boolean
  sessionPersistenceDisabled: boolean
  hasExitedPlanMode: boolean
  needsPlanModeExitAttachment: boolean
  needsAutoModeExitAttachment: boolean
  lspRecommendationShownThisSession: boolean
  registeredHooks: Partial<Record<HookEvent, RegisteredHookMatcher[]>> | null
  invokedSkills: Map<string, {...}>
  slowOperations: Array<{ operation: string; durationMs: number; timestamp: number }>
  
  // === SDK 状态 ===
  sdkAgentProgressSummariesEnabled: boolean
  userMsgOptIn: boolean
  questionPreviewFormat: 'markdown' | 'html' | undefined
  sdkBetas: string[] | undefined
  mainThreadAgentType: string | undefined
  initJsonSchema: Record<string, unknown> | null
  
  // === 远程状态 ===
  isRemoteMode: boolean
  directConnectServerUrl: string | undefined
  teleportedSessionInfo: {...} | null
  
  // === Channel 状态 ===
  allowedChannels: ChannelEntry[]
  hasDevChannels: boolean
  
  // === 项目状态 ===
  sessionProjectDir: string | null
  additionalDirectoriesForClaudeMd: string[]
  
  // === Prompt Cache 状态 ===
  promptCache1hAllowlist: string[] | null
  promptCache1hEligible: boolean | null
  afkModeHeaderLatched: boolean | null
  fastModeHeaderLatched: boolean | null
  cacheEditingHeaderLatched: boolean | null
  thinkingClearLatched: boolean | null
  pendingPostCompaction: boolean
  
  // === 其他 ===
  lastEmittedDate: string | null
}
```

### Channel Entry 类型

```typescript
export type ChannelEntry =
  | { kind: 'plugin'; name: string; marketplace: string; dev?: boolean }
  | { kind: 'server'; name: string; dev?: boolean }
```

### AttributedCounter 类型

```typescript
export type AttributedCounter = {
  add(value: number, additionalAttributes?: Attributes): void
}
```

---

## 3. 状态分类

### 3.1 路径状态

| 字段 | 描述 | 何时设置 |
|------|------|---------|
| `originalCwd` | 启动时的工作目录 | 启动时 |
| `projectRoot` | 稳定的项目根目录 | 启动时（不受 EnterWorktree 影响） |
| `cwd` | 当前工作目录 | 可被 setCwdState 更新 |

**设计说明**:
- `originalCwd` 和 `projectRoot` 通常相同
- `projectRoot` 用于项目身份（history、skills、sessions）
- `cwd` 用于文件操作，可被 EnterWorktreeTool 更新

### 3.2 成本与使用量

| 字段 | 描述 |
|------|------|
| `totalCostUSD` | 累计 API 成本 |
| `totalAPIDuration` | API 调用总时间（含重试） |
| `totalAPIDurationWithoutRetries` | API 调用时间（不含重试） |
| `totalToolDuration` | 工具执行总时间 |
| `modelUsage` | 每个模型的使用量 |

### 3.3 会话状态

| 字段 | 描述 |
|------|------|
| `sessionId` | 当前会话 UUID |
| `parentSessionId` | 父会话 ID（用于跟踪会话链） |
| `sessionProjectDir` | 会话文件所在目录 |
| `isInteractive` | 是否交互模式 |
| `clientType` | 客户端类型（'cli' 等） |

### 3.4 遥测状态

| 字段 | 描述 |
|------|------|
| `meter` | OpenTelemetry Meter |
| `sessionCounter` | 会话计数器 |
| `locCounter` | 代码行计数器 |
| `prCounter` | PR 计数器 |
| `commitCounter` | Commit 计数器 |
| `costCounter` | 成本计数器 |
| `tokenCounter` | Token 计数器 |
| `activeTimeCounter` | 活跃时间计数器 |

### 3.5 会话级标志

这些标志仅在当前会话有效，不持久化：

| 字段 | 描述 |
|------|------|
| `sessionBypassPermissionsMode` | 绕过权限模式 |
| `sessionTrustAccepted` | 会话级信任已接受 |
| `sessionPersistenceDisabled` | 禁用会话持久化 |
| `hasExitedPlanMode` | 已退出计划模式 |
| `scheduledTasksEnabled` | 定时任务已启用 |
| `lspRecommendationShownThisSession` | LSP 推荐已显示 |

### 3.6 Latch 状态

"Latch" 状态是一次触发后保持的状态：

| 字段 | 描述 |
|------|------|
| `afkModeHeaderLatched` | AFK 模式 header 已 latch |
| `fastModeHeaderLatched` | Fast 模式 header 已 latch |
| `cacheEditingHeaderLatched` | Cache editing header 已 latch |
| `thinkingClearLatched` | Thinking clear 已 latch |

**目的**: 保持 beta header 稳定，避免中途切换导致 prompt cache 失效。

---

## 4. Getter/Setter API

### 4.1 会话 ID

```typescript
// 获取当前会话 ID
export function getSessionId(): SessionId

// 重新生成会话 ID
export function regenerateSessionId(
  options: { setCurrentAsParent?: boolean } = {},
): SessionId

// 获取父会话 ID
export function getParentSessionId(): SessionId | undefined

// 切换会话（原子操作）
export function switchSession(
  sessionId: SessionId,
  projectDir: string | null = null,
): void
```

### 4.2 路径

```typescript
// 工作目录
export function getOriginalCwd(): string
export function setOriginalCwd(cwd: string): void
export function getCwdState(): string
export function setCwdState(cwd: string): void

// 项目根目录
export function getProjectRoot(): string
export function setProjectRoot(cwd: string): void  // 仅用于 --worktree

// 会话项目目录
export function getSessionProjectDir(): string | null
```

### 4.3 成本与使用量

```typescript
// 成本
export function getTotalCostUSD(): number
export function addToTotalCostState(cost: number, modelUsage: ModelUsage, model: string): void

// 时间
export function getTotalAPIDuration(): number
export function getTotalAPIDurationWithoutRetries(): number
export function getTotalToolDuration(): number
export function getTotalDuration(): number
export function addToTotalDurationState(duration: number, durationWithoutRetries: number): void
export function addToToolDuration(duration: number): void

// 代码行
export function getTotalLinesAdded(): number
export function getTotalLinesRemoved(): number
export function addToTotalLinesChanged(added: number, removed: number): void

// Token
export function getTotalInputTokens(): number
export function getTotalOutputTokens(): number
export function getTotalCacheReadInputTokens(): number
export function getTotalCacheCreationInputTokens(): number
export function getTotalWebSearchRequests(): number

// 模型使用
export function getModelUsage(): { [modelName: string]: ModelUsage }
export function getUsageForModel(model: string): ModelUsage | undefined
```

### 4.4 模型

```typescript
export function getMainLoopModelOverride(): ModelSetting | undefined
export function setMainLoopModelOverride(model: ModelSetting | undefined): void
export function getInitialMainLoopModel(): ModelSetting
export function setInitialMainLoopModel(model: ModelSetting): void
export function getModelStrings(): ModelStrings | null
export function setModelStrings(modelStrings: ModelStrings): void
export function getSdkBetas(): string[] | undefined
export function setSdkBetas(betas: string[] | undefined): void
```

### 4.5 遥测

```typescript
export function getMeter(): Meter | null
export function setMeter(meter: Meter, createCounter: ...): void
export function getSessionCounter(): AttributedCounter | null
export function getLocCounter(): AttributedCounter | null
export function getPrCounter(): AttributedCounter | null
export function getCommitCounter(): AttributedCounter | null
export function getCostCounter(): AttributedCounter | null
export function getTokenCounter(): AttributedCounter | null
export function getActiveTimeCounter(): AttributedCounter | null
export function getStatsStore(): { observe(name: string, value: number): void } | null
export function setStatsStore(store: ...): void
```

### 4.6 交互时间

```typescript
// 更新最后交互时间
export function updateLastInteractionTime(immediate?: boolean): void

// 刷新交互时间（Ink render 前调用）
export function flushInteractionTime(): void

// 获取最后交互时间
export function getLastInteractionTime(): number
```

### 4.7 Turn 统计

```typescript
// Hook 统计
export function getTurnHookDurationMs(): number
export function addToTurnHookDuration(duration: number): void
export function resetTurnHookDuration(): void
export function getTurnHookCount(): number

// Tool 统计
export function getTurnToolDurationMs(): number
export function addToTurnToolDuration(duration: number): void
export function resetTurnToolDuration(): void
export function getTurnToolCount(): number

// Classifier 统计
export function getTurnClassifierDurationMs(): number
export function addToTurnClassifierDuration(duration: number): void
export function resetTurnClassifierDuration(): void
export function getTurnClassifierCount(): number
```

### 4.8 Token Budget

```typescript
export function getTurnOutputTokens(): number
export function getCurrentTurnTokenBudget(): number | null
export function snapshotOutputTokensForTurn(budget: number | null): void
export function getBudgetContinuationCount(): number
export function incrementBudgetContinuationCount(): void
```

### 4.9 请求追踪

```typescript
export function getLastMainRequestId(): string | undefined
export function setLastMainRequestId(requestId: string): void
export function getLastApiCompletionTimestamp(): number | null
export function setLastApiCompletionTimestamp(timestamp: number): void
```

### 4.10 Compaction 标记

```typescript
// 标记刚完成 compaction
export function markPostCompaction(): void

// 消费 post-compaction 标记
export function consumePostCompaction(): boolean
```

---

## 5. 核心功能

### 5.1 状态初始化

```typescript
function getInitialState(): State {
  // 解析 cwd（处理 symlink）
  let resolvedCwd = ''
  if (typeof process !== 'undefined' && ...) {
    const rawCwd = cwd()
    try {
      resolvedCwd = realpathSync(rawCwd).normalize('NFC')
    } catch {
      // CloudStorage EPERM fallback
      resolvedCwd = rawCwd.normalize('NFC')
    }
  }
  
  return {
    originalCwd: resolvedCwd,
    projectRoot: resolvedCwd,
    sessionId: randomUUID() as SessionId,
    // ... 其他默认值
  }
}

// 全局状态实例
const STATE: State = getInitialState()
```

### 5.2 会话切换

```typescript
// 原子切换会话
export function switchSession(
  sessionId: SessionId,
  projectDir: string | null = null,
): void {
  // 清理旧会话的 plan slug
  STATE.planSlugCache.delete(STATE.sessionId)
  
  // 设置新会话
  STATE.sessionId = sessionId
  STATE.sessionProjectDir = projectDir
  
  // 触发信号
  sessionSwitched.emit(sessionId)
}

// 订阅会话切换
export const onSessionSwitch = sessionSwitched.subscribe
```

### 5.3 成本恢复

```typescript
// 恢复会话成本状态（用于 /resume）
export function setCostStateForRestore({
  totalCostUSD,
  totalAPIDuration,
  totalAPIDurationWithoutRetries,
  totalToolDuration,
  totalLinesAdded,
  totalLinesRemoved,
  lastDuration,
  modelUsage,
}): void
```

### 5.4 Scroll Draining

```typescript
// 标记滚动活动（后台 interval 检查此标志）
export function markScrollActivity(): void

// 检查是否正在滚动
export function getIsScrollDraining(): boolean

// 等待滚动空闲
export async function waitForScrollIdle(): Promise<void>
```

### 5.5 测试重置

```typescript
// 仅测试环境可用
export function resetStateForTests(): void {
  if (process.env.NODE_ENV !== 'test') {
    throw new Error('resetStateForTests can only be called in tests')
  }
  // 重置所有状态到初始值
  Object.entries(getInitialState()).forEach(([key, value]) => {
    STATE[key as keyof State] = value as never
  })
  // 重置模块级变量
  outputTokensAtTurnStart = 0
  currentTurnTokenBudget = null
  budgetContinuationCount = 0
  sessionSwitched.clear()
}
```

---

## 6. 设计原则

### 6.1 Bootstrap Isolation

Bootstrap 模块必须保持为依赖图的叶子节点：

```
                    ┌─────────────┐
                    │   其他模块   │
                    └──────┬──────┘
                           │ import
                           ▼
                    ┌─────────────┐
                    │  bootstrap  │  ← 不依赖其他业务模块
                    └─────────────┘
```

**规则**:
- 不能导入 `src/utils/` 下的模块
- 不能导入 `src/services/` 下的模块
- 只能导入标准库和外部依赖

### 6.2 谨慎添加状态

```typescript
// DO NOT ADD MORE STATE HERE - BE JUDICIOUS WITH GLOBAL STATE
```

全局状态应该最小化，每次添加新状态都需要仔细考虑。

### 6.3 会话级 vs 持久化

| 类型 | 存储 | 生命周期 |
|------|------|---------|
| 会话级标志 | 内存 | 会话结束即清除 |
| 持久化设置 | 磁盘 | 跨会话保持 |

会话级标志（如 `sessionBypassPermissionsMode`）不会写入磁盘，适合临时状态。

### 6.4 信号机制

使用 `createSignal` 实现发布/订阅：

```typescript
const sessionSwitched = createSignal<[id: SessionId]>()

// 触发
sessionSwitched.emit(sessionId)

// 订阅
export const onSessionSwitch = sessionSwitched.subscribe
```

### 6.5 防抖优化

```typescript
// 交互时间更新延迟到 Ink render 帧
let interactionTimeDirty = false

export function updateLastInteractionTime(immediate?: boolean): void {
  if (immediate) {
    flushInteractionTime_inner()
  } else {
    interactionTimeDirty = true  // 标记 dirty
  }
}

export function flushInteractionTime(): void {
  if (interactionTimeDirty) {
    flushInteractionTime_inner()
  }
}
```

### 6.6 Turn 级统计

Turn 级统计在每个 API turn 结束后重置：

```typescript
// Turn 开始时调用
snapshotOutputTokensForTurn(budget)

// Turn 结束后重置
resetTurnHookDuration()
resetTurnToolDuration()
resetTurnClassifierDuration()
```

---

## 附录: 状态字段完整索引

### 路径状态
- `originalCwd`, `projectRoot`, `cwd`, `sessionProjectDir`, `additionalDirectoriesForClaudeMd`

### 成本状态
- `totalCostUSD`, `totalAPIDuration`, `totalAPIDurationWithoutRetries`, `totalToolDuration`, `modelUsage`

### 时间状态
- `startTime`, `lastInteractionTime`, `lastApiCompletionTimestamp`

### 代码统计
- `totalLinesAdded`, `totalLinesRemoved`

### 模型状态
- `mainLoopModelOverride`, `initialMainLoopModel`, `modelStrings`, `sdkBetas`

### 会话状态
- `sessionId`, `parentSessionId`, `isInteractive`, `clientType`, `sessionSource`

### 遥测状态
- `meter`, `sessionCounter`, `locCounter`, `prCounter`, `commitCounter`, `costCounter`, `tokenCounter`, `activeTimeCounter`, `statsStore`

### 日志状态
- `loggerProvider`, `eventLogger`, `meterProvider`, `tracerProvider`

### Agent 状态
- `agentColorMap`, `agentColorIndex`, `mainThreadAgentType`

### 请求状态
- `lastAPIRequest`, `lastAPIRequestMessages`, `lastClassifierRequests`, `lastMainRequestId`, `promptId`

### 缓存状态
- `cachedClaudeMdContent`, `systemPromptSectionCache`, `planSlugCache`

### 会话标志
- `sessionBypassPermissionsMode`, `sessionTrustAccepted`, `sessionPersistenceDisabled`, `hasExitedPlanMode`, `scheduledTasksEnabled`, `lspRecommendationShownThisSession`

### Latch 状态
- `afkModeHeaderLatched`, `fastModeHeaderLatched`, `cacheEditingHeaderLatched`, `thinkingClearLatched`, `promptCache1hEligible`

---

## 总结

Bootstrap 模块是 Claude Code 的全局状态管理中心，具有以下特点：

1. **单一文件**: 所有状态集中在 `state.ts`
2. **叶子节点**: 不依赖其他业务模块（bootstrap isolation）
3. **最小化状态**: 谨慎添加全局状态
4. **类型安全**: 完整的 TypeScript 类型定义
5. **Getter/Setter 模式**: 通过函数访问状态，而非直接访问对象
6. **信号机制**: 使用 `createSignal` 实现发布/订阅
7. **测试友好**: `resetStateForTests` 支持测试隔离
8. **会话级隔离**: 区分会话级标志和持久化设置