# 其他核心模块详细文档

## 目录

1. [概述](#1-概述)
2. [Types 模块](#2-types-模块)
3. [Constants 模块](#3-constants-模块)
4. [State 模块](#4-state-模块)
5. [Hooks 模块](#5-hooks-模块)
6. [其他重要模块](#6-其他重要模块)

---

## 1. 概述

本模块涵盖 Claude Code 中不属于主要模块（如 Tools、Commands、Bridge 等）的核心功能模块。

---

## 2. Types 模块

### 2.1 概述

```
src/types/
├── ids.ts              # 品牌类型定义
├── message.ts          # 消息类型
├── command.ts          # 命令类型
├── hooks.ts            # Hook 类型
├── permissions.ts      # 权限类型
├── plugin.ts           # 插件类型
├── textInputTypes.ts   # 文本输入类型
├── logs.ts             # 日志类型
└── generated/          # 生成的类型
```

### 2.2 ids.ts - 品牌类型

使用 TypeScript 品牌类型防止 ID 混淆：

```typescript
// Session ID
export type SessionId = string & { readonly __brand: 'SessionId' }
export function asSessionId(id: string): SessionId

// Agent ID
export type AgentId = string & { readonly __brand: 'AgentId' }
export function asAgentId(id: string): AgentId
export function toAgentId(s: string): AgentId | null

// 验证模式
const AGENT_ID_PATTERN = /^a(?:.+-)?[0-9a-f]{16}$/
```

### 2.3 message.ts - 消息类型

```typescript
// 消息联合类型
export type Message =
  | AssistantMessage
  | UserMessage
  | SystemMessage
  | ToolMessage

// 消息内容
export type MessageContent =
  | TextContent
  | ToolUseContent
  | ToolResultContent

// Assistant Message
export interface AssistantMessage {
  type: 'assistant'
  id: string
  content: MessageContent[]
  role: 'assistant'
}

// User Message
export interface UserMessage {
  type: 'user'
  id: string
  content: MessageContent[]
  role: 'user'
}
```

### 2.4 permissions.ts - 权限类型

```typescript
// 权限模式
export type PermissionMode =
  | 'acceptEdits'
  | 'limitTools'
  | 'bypassPermissions'
  | 'ask'

// 权限决策
export type PermissionDecision =
  | { type: 'allow' }
  | { type: 'deny'; reason: string }
  | { type: 'escalate' }
```

---

## 3. Constants 模块

### 3.1 概述

```
src/constants/
├── apiLimits.ts           # API 限制
├── betas.ts               # Beta 功能
├── common.ts              # 通用常量
├── files.ts               # 文件相关常量
├── keys.ts                # 键位常量
├── messages.ts            # 消息常量
├── oauth.ts               # OAuth 常量
├── product.ts             # 产品信息
├── prompts.ts             # 提示常量
├── xml.ts                 # XML 常量
├── tools.ts               # 工具常量
├── outputStyles.ts        # 输出风格
├── systemPromptSections.ts # 系统提示部分
└── notifs/               # 通知常量
```

### 3.2 apiLimits.ts

```typescript
// API 限制常量
export const MAX_TOKENS = 8192
export const MAX_RETRIES = 3
export const REQUEST_TIMEOUT_MS = 60_000
```

### 3.3 xml.ts

```typescript
// XML 标签常量
export const XML_TAGS = {
  TOOL_USE: 'tool_use',
  TOOL_RESULT: 'tool_result',
  TEXT: 'text',
  THINKING: 'thinking',
  REDACTED: 'redacted',
} as const

// 标签正则
export const TOOL_USE_REGEX = /<tool_use>[\s\S]*?<\/tool_use>/g
```

### 3.4 betas.ts

```typescript
// Beta 特性常量
export const BETA_HEADERS = {
  PROMPT_CACHE: 'prompt-cache-1m-2025-08-07',
  AFK_MODE: 'afk-mode-2025-11-01',
  FAST_MODE: 'fast-mode-2025-06-01',
} as const
```

---

## 4. State 模块

### 4.1 概述

```
src/state/
├── AppStateStore.ts       # AppState Store
├── AppState.tsx           # AppState Provider
├── onChangeAppState.ts    # 状态变更处理
├── selectors.ts           # 选择器
├── store.ts               # Store 工具
└── teammateViewHelpers.ts  # 队友视图辅助
```

### 4.2 AppState.tsx

React Context 提供应用状态：

```typescript
// AppState 类型（100+ 字段）
type AppState = {
  // 会话状态
  sessionId: string
  messages: Message[]

  // UI 状态
  isLoading: boolean
  isThinking: boolean

  // 设置状态
  model: string
  permissionMode: PermissionMode

  // ... 更多字段
}

// Provider
export function AppStateProvider({
  initialState,
  onChangeAppState,
  children,
}: {
  initialState: AppState
  onChangeAppState: (state: AppState, changed: Partial<AppState>) => void
  children: React.ReactNode
}): React.ReactNode

// Hooks
export function useAppState(): AppState
export function useSetAppState(): (changes: Partial<AppState>) => void
```

### 4.3 onChangeAppState.ts

状态变更处理：

```typescript
export function onChangeAppState(
  state: AppState,
  changed: Partial<AppState>
): void {
  // 处理各种状态变更
  // 持久化、通知、副作用等
}
```

---

## 5. Hooks 模块

### 5.1 概述

```
src/hooks/
├── useAppState.ts          # 应用状态 Hook
├── useCanUseTool.tsx      # 工具权限 Hook
├── useCancelRequest.ts    # 取消请求 Hook
├── useCommandQueue.ts     # 命令队列 Hook
├── useDiffData.ts         # Diff 数据 Hook
├── useIDEIntegration.tsx  # IDE 集成 Hook
├── useInputBuffer.ts      # 输入缓冲 Hook
├── useMainLoopModel.ts    # 主循环模型 Hook
├── useMergedCommands.ts   # 合并命令 Hook
├── useMergedTools.ts      # 合并工具 Hook
├── usePromptSuggestion.ts # 提示建议 Hook
├── useSettings.ts         # 设置 Hook
├── useTasksV2.ts          # 任务 V2 Hook
├── useTextInput.ts        # 文本输入 Hook
├── useVirtualScroll.ts    # 虚拟滚动 Hook
└── [其他 Hook]
```

### 5.2 核心 Hook

#### useAppState.ts

```typescript
// 使用应用状态
export function useAppState(): AppState {
  const context = useContext(AppStateContext)
  if (!context) {
    throw new Error('useAppState must be used within AppStateProvider')
  }
  return context
}

// 更新应用状态
export function useSetAppState(): (changes: Partial<AppState>) => void {
  const { onChangeAppState } = useContext(AppStateContext)
  return useCallback(
    (changes: Partial<AppState>) => onChangeAppState?.(null as any, changes),
    [onChangeAppState]
  )
}
```

#### useCanUseTool.tsx

```typescript
// 工具权限检查
export function useCanUseTool(): {
  canUseTool: (toolName: string, input?: Record<string, unknown>) => Promise<boolean>
  pendingRequests: PermissionRequest[]
} {
  // 检查权限、显示提示等
}
```

#### useCommandQueue.ts

```typescript
// 命令队列管理
export function useCommandQueue(): {
  enqueue: (command: QueuedCommand) => void
  dequeue: () => QueuedCommand | undefined
  peek: () => QueuedCommand | undefined
  clear: () => void
  size: number
}
```

#### useDiffData.ts

```typescript
// Diff 数据管理
export function useDiffData(): {
  diffs: Diff[]
  addDiff: (diff: Diff) => void
  removeDiff: (id: string) => void
  clearDiffs: () => void
}
```

#### useMergedCommands.ts

```typescript
// 合并命令
export function useMergedCommands(): Command[] {
  // 合并内置命令和插件命令
  // 处理优先级、去重等
}
```

#### useMergedTools.ts

```typescript
// 合并工具
export function useMergedTools(): Tool[] {
  // 合并内置工具和插件工具
  // 处理可见性、权限等
}
```

---

## 6. 其他重要模块

### 6.1 coordinator/

协调器模块：

```typescript
// 协调多个 Agent/工具的执行
export class Coordinator {
  // 管理主循环和子代理
  // 处理并发和依赖
}
```

### 6.2 entrypoints/

入口点模块：

```
src/entrypoints/
├── main.tsx          # 主入口
├── agent.tsx        # Agent 入口
├── agentSdkTypes.ts # SDK 类型
└── sdk/            # SDK 实现
```

### 6.3 context/

上下文模块：

```typescript
// 管理 Claude 请求的上下文
export function buildContext(): Context {
  // 构建系统提示、工具列表等
}
```

### 6.4 skills/

Skills 模块：

```
src/skills/
├── skills.ts        # Skills 主模块
├── loadSkills.ts    # 加载 Skills
├── executeSkill.ts  # 执行 Skill
└── [其他]
```

### 6.5 migrations/

迁移模块：

```typescript
// 数据库/存储迁移
export async function runMigrations(): Promise<void> {
  // 执行待处理的迁移
}
```

### 6.6 plugins/

插件模块：

```
src/plugins/
├── pluginManager.ts    # 插件管理
├── loadPlugins.ts      # 加载插件
└── [其他]
```

### 6.7 services/

服务模块（已文档化）

### 6.8 server/

服务器模块：

```typescript
// MCP 服务器实现
export class MCPServer {
  // 处理 MCP 协议
}
```

### 6.9 outputStyles/

输出风格模块：

```typescript
// 定义不同的输出格式
export const OUTPUT_STYLES = {
  compact: { ... },
  verbose: { ... },
  markdown: { ... },
} as const
```

### 6.10 screens/

屏幕模块：

```typescript
// 终端屏幕管理
export class ScreenManager {
  // 管理多个屏幕/视图
  // 处理切换、堆栈等
}
```

### 6.11 ink/

Ink 渲染模块：

```typescript
// Ink 组件包装
export function renderWithInk(element: React.ReactNode): void {
  // 使用 Ink 渲染
}
```

### 6.12 tasks/

任务管理模块：

```typescript
// 任务状态和操作
export type TaskStatus =
  | 'pending'
  | 'running'
  | 'completed'
  | 'failed'
  | 'cancelled'
```

---

## 附录: 模块索引

| 模块 | 目录 | 描述 |
|------|------|------|
| Types | `src/types/` | 类型定义 |
| Constants | `src/constants/` | 常量定义 |
| State | `src/state/` | 状态管理 |
| Hooks | `src/hooks/` | React Hooks |
| Coordinator | `src/coordinator/` | 协调器 |
| Entrypoints | `src/entrypoints/` | 入口点 |
| Context | `src/context/` | 上下文 |
| Skills | `src/skills/` | Skills |
| Migrations | `src/migrations/` | 迁移 |
| Plugins | `src/plugins/` | 插件 |
| Server | `src/server/` | 服务器 |
| OutputStyles | `src/outputStyles/` | 输出风格 |
| Screens | `src/screens/` | 屏幕 |
| Ink | `src/ink/` | Ink 渲染 |

---

## 总结

其他核心模块涵盖了：

1. **Types**: 类型定义和品牌类型
2. **Constants**: 应用常量
3. **State**: React 状态管理
4. **Hooks**: React Hooks 库
5. **协调器**: 多组件协调
6. **入口点**: 应用入口
7. **插件/技能**: 扩展系统