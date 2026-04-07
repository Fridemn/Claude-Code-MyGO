# Utils 模块详细文档

## 目录

1. [概述](#1-概述)
2. [目录结构](#2-目录结构)
3. [核心工具分类](#3-核心工具分类)
4. [主要工具详解](#4-主要工具详解)
5. [子目录模块](#5-子目录模块)

---

## 1. 概述

Utils 模块是 Claude Code 的工具函数库，包含 300+ 个文件，涵盖各种辅助功能。

### 设计原则

1. **单一职责**: 每个文件专注于一个功能
2. **叶子模块**: 避免循环依赖
3. **可测试性**: 独立的工具函数易于测试
4. **可复用性**: 跨模块共享

---

## 2. 目录结构

```
src/utils/           # ~331 个文件
├── bash/           # Bash 执行相关
├── git/            # Git 操作
├── hooks/          # Hook 系统
├── memory/         # 内存管理
├── messages/       # 消息处理
├── model/          # 模型配置
├── mcp/            # MCP 协议
├── permissions/    # 权限管理
├── plugins/        # 插件系统
├── settings/       # 设置管理
├── skills/         # Skills 系统
├── task/           # 任务管理
├── telemetry/      # 遥测
├── ultraplan/      # 计划模式
├── background/     # 后台任务
├── sandbox/        # 沙箱
├── swarm/          # Swarm
└── [其他散列文件]  # 独立工具
```

---

## 3. 核心工具分类

### 3.1 核心基础

| 文件 | 描述 |
|------|------|
| `errors.ts` | 错误类型定义 |
| `sleep.ts` | 延迟函数 |
| `uuid.ts` | UUID 生成 |
| `array.ts` | 数组工具 |
| `stringUtils.ts` | 字符串工具 |
| `json.ts` | JSON 解析/序列化 |
| `slowOperations.ts` | 慢操作优化 |
| `signal.ts` | 信号管理 |
| `stream.ts` | 流处理 |

### 3.2 文件系统

| 文件 | 描述 |
|------|------|
| `path.ts` | 路径处理 |
| `fileStateCache.ts` | 文件状态缓存 |
| `filePersistence/` | 文件持久化 |
| `tempfile.ts` | 临时文件 |

### 3.3 网络

| 文件 | 描述 |
|------|------|
| `api.ts` | API 调用 |
| `apiPreconnect.ts` | API 预连接 |
| `fetchWithRetry.ts` | 重试请求 |

### 3.4 进程/Shell

| 文件 | 描述 |
|------|------|
| `Shell.ts` | Shell 执行 |
| `process.ts` | 进程管理 |
| `bash/` | Bash 相关 |
| `shell/` | Shell 配置 |
| `tmuxSocket.ts` | Tmux 集成 |

### 3.5 Git

| 文件 | 描述 |
|------|------|
| `git/` | Git 操作 |
| `detectRepository.ts` | 仓库检测 |
| `git.ts` | Git 工具 |

### 3.6 认证

| 文件 | 描述 |
|------|------|
| `auth.ts` | 认证管理 |
| `authPortable.ts` | 便携认证 |
| `authFileDescriptor.ts` | 文件描述符认证 |
| `oauth/` | OAuth 支持 |

### 3.7 设置

| 文件 | 描述 |
|------|------|
| `settings/` | 设置管理 |
| `settingsCache.ts` | 设置缓存 |
| `config.ts` | 全局配置 |

### 3.8 任务系统

| 文件 | 描述 |
|------|------|
| `task/` | 任务管理 |
| `tasks.ts` | 任务工具 |
| `cronTasks.ts` | 定时任务 |

### 3.9 模型/AI

| 文件 | 描述 |
|------|------|
| `model/` | 模型配置 |
| `model.ts` | 模型工具 |
| `thinking.ts` | 思考配置 |

### 3.10 Hooks

| 文件 | 描述 |
|------|------|
| `hooks/` | Hook 系统 |
| `hookEvents.ts` | Hook 事件 |
| `AsyncHookRegistry.ts` | 异步 Hook |

---

## 4. 主要工具详解

### 4.1 errors.ts

```typescript
// 核心错误类型
export class AbortError extends Error
export class ToolError extends Error
export class APIError extends Error

// 错误处理
export function errorMessage(error: unknown): string
export function isAbortError(error: unknown): boolean
```

### 4.2 signal.ts

```typescript
// 信号创建
export function createSignal<T>(): Signal<T>

interface Signal<T> {
  get(): T
  set(value: T): void
  update(fn: (current: T) => T): void
  emit(value: T): void
  subscribe(callback: (value: T) => void): () => void
  clear(): void
}
```

### 4.3 stream.ts

```typescript
// 流处理
export class Stream<T> {
  static fromAsync<T>(asyncIterable: AsyncIterable<T>): Stream<T>
  filter(predicate: (item: T) => boolean): Stream<T>
  map<U>(fn: (item: T) => U): Stream<U>
  take(count: number): Stream<T>
  forEach(fn: (item: T) => void): Promise<void>
  toArray(): Promise<T[]>
}
```

### 4.4 slowOperations.ts

```typescript
// JSON 操作
export function jsonParse(text: string): unknown
export function jsonStringify(value: unknown): string

// 防抖 JSON 解析
export function debouncedJsonParse(text: string): unknown
```

### 4.5 CircularBuffer.ts

```typescript
// 环形缓冲区
export class CircularBuffer<T> {
  constructor(private capacity: number)
  push(item: T): void
  toArray(): T[]
  get length(): number
  clear(): void
}
```

### 4.6 shell/Shell.ts

```typescript
export class Shell {
  constructor(options?: ShellOptions)

  // 执行命令
  async run(command: string): Promise<ShellResult>

  // 获取/设置工作目录
  getCwd(): string
  setCwd(path: string): void

  // 环境变量
  getEnv(): NodeJS.ProcessEnv
  setEnv(env: NodeJS.ProcessEnv): void
}

interface ShellOptions {
  cwd?: string
  env?: NodeJS.ProcessEnv
  shell?: string
}
```

### 4.7 process.ts

```typescript
// 进程工具
export function writeToStdout(data: string): void
export function writeToStderr(data: string): void
export function registerProcessOutputErrorHandlers(): void
```

### 4.8 path.ts

```typescript
// 路径操作
export function expandPath(path: string): string
export function normalizePath(path: string): string
export function resolvePath(path: string): string
export function isAbsolutePath(path: string): boolean
export function joinPath(...parts: string[]): string
```

### 4.9 fileStateCache.ts

```typescript
// 文件状态缓存
export function createFileStateCacheWithSizeLimit(
  maxEntries?: number
): FileStateCache

interface FileStateCache {
  get(path: string): FileState | undefined
  set(path: string, state: FileState): void
  delete(path: string): void
  clear(): void
  get size(): number
}
```

### 4.10 cleanupRegistry.ts

```typescript
// 清理注册表
export function registerCleanup(fn: () => void | Promise<void>): void
export function runCleanup(): Promise<void>
```

### 4.11 gracefulShutdown.ts

```typescript
// 优雅关闭
export async function gracefulShutdown(exitCode?: number): Promise<void>
export function gracefulShutdownSync(exitCode?: number): void
export function isShuttingDown(): boolean
export function registerShutdownHandler(
  handler: () => void | Promise<void>,
  priority?: number
): void
```

---

## 5. 子目录模块

### 5.1 bash/

```
src/utils/bash/
├── BashTool.ts          # Bash 工具实现
├── BashToolImpl.ts     # Bash 工具实现
├── BashResult.ts        # 结果类型
├── BashError.ts        # 错误类型
├── specs/              # Bash 规格
└── index.ts            # 导出
```

**核心类型**:
```typescript
export interface BashResult {
  stdout: string
  stderr: string
  exitCode: number
  durationMs: number
}

export class BashError extends Error {
  constructor(
    message: string,
    public readonly exitCode: number,
    public readonly stderr: string
  )
}
```

### 5.2 git/

```
src/utils/git/
├── git.ts              # Git 操作
├── detectRepository.ts # 仓库检测
├── branch.ts          # 分支操作
├── commit.ts          # 提交操作
├── diff.ts            # Diff 操作
└── status.ts          # 状态操作
```

**核心函数**:
```typescript
// 仓库检测
export function detectGitRoot(dir?: string): string | null
export function getGitBranch(dir?: string): string | null
export function getGitRemoteUrl(dir?: string): string | null

// Git 操作
export async function gitStatus(dir?: string): Promise<GitStatus>
export async function gitLog(options?: GitLogOptions): Promise<GitLogEntry[]>
export async function gitDiff(options?: GitDiffOptions): Promise<string>
```

### 5.3 hooks/

```
src/utils/hooks/
├── hookEvents.ts        # Hook 事件定义
├── hookSchemas.ts       # Hook schema
├── AsyncHookRegistry.ts # 异步 Hook 注册表
├── useCanUseTool.ts     # 权限 Hook
└── [其他 Hook 实现]
```

**Hook 类型**:
```typescript
export type HookEvent =
  | 'onToolUse'
  | 'onToolResult'
  | 'onMessage'
  | 'onCompletion'
  | 'onError'

export interface HookCallback {
  event: HookEvent
  callback: (context: HookContext) => HookResult | Promise<HookResult>
}
```

### 5.4 model/

```
src/utils/model/
├── model.ts              # 模型配置
├── modelStrings.ts       # 模型字符串
├── providers.ts          # 模型提供者
├── ModelSetting.ts       # 模型设置
└── [其他模型相关]
```

**核心函数**:
```typescript
// 模型选择
export function getMainLoopModel(): ModelSetting
export function setMainLoopModel(model: ModelSetting): void

// 模型提供者
export function getModelProvider(): 'anthropic' | 'bedrock' | 'vertex'
export function getApiKey(): string | undefined

// Token 预算
export function getTokenBudget(): TokenBudget
export function calculateTokenUsage(messages: Message[]): TokenUsage
```

### 5.5 permissions/

```
src/utils/permissions/
├── permissions.ts        # 权限检查
├── PermissionMode.ts     # 权限模式
├── PermissionResult.ts   # 权限结果
├── PermissionPrompt.ts   # 权限提示
├── PermissionUpdate.ts   # 权限更新
└── PermissionPromptToolResultSchema.ts
```

**权限模式**:
```typescript
export type PermissionMode =
  | 'acceptEdits'    // 接受所有编辑
  | 'limitTools'     // 限制工具
  | 'bypassPermissions' // 绕过权限
  | 'ask'           // 询问

export type PermissionDecision =
  | { type: 'allow' }
  | { type: 'deny'; reason: string }
  | { type: 'escalate' }
```

### 5.6 settings/

```
src/utils/settings/
├── settings.ts           # 设置管理
├── settingsCache.ts      # 设置缓存
├── constants.ts          # 设置常量
├── types.ts             # 设置类型
├── changeDetector.ts     # 变更检测
├── applySettingsChange.ts # 应用变更
└── mdm/                 # MDM 支持
```

**设置类型**:
```typescript
export interface Settings {
  model?: ModelSetting
  maxTokens?: number
  permissionMode?: PermissionMode
  autoUpdates?: boolean
  // ... 更多设置
}

export function getSettings(): Promise<Settings>
export function updateSettings(changes: Partial<Settings>): Promise<void>
```

### 5.7 task/

```
src/utils/task/
├── taskTypes.ts          # 任务类型
├── taskManager.ts        # 任务管理
├── taskRunner.ts         # 任务运行
├── cronTasks.ts          # 定时任务
└── taskStorage.ts        # 任务存储
```

**任务类型**:
```typescript
export type TaskType =
  | 'UserAttendedTask'
  | 'BackgroundTask'
  | 'CronTask'
  | 'SubAgentTask'

export interface Task {
  id: string
  type: TaskType
  status: TaskStatus
  createdAt: number
  // ...
}
```

### 5.8 memory/

```
src/utils/memory/
├── memoryManager.ts      # 内存管理
├── memoryStore.ts        # 内存存储
└── [其他内存相关]
```

### 5.9 messages/

```
src/utils/messages/
├── messageQueue.ts       # 消息队列
├── messageParser.ts      # 消息解析
├── messageFormatter.ts   # 消息格式化
└── mappers.ts           # 消息映射
```

### 5.10 mcp/

```
src/utils/mcp/
├── mcpClient.ts         # MCP 客户端
├── mcpServer.ts         # MCP 服务器
├── mcpTypes.ts          # MCP 类型
└── [其他 MCP 相关]
```

### 5.11 skills/

```
src/utils/skills/
├── skillManager.ts       # Skill 管理
├── skillLoader.ts       # Skill 加载
├── skillExecutor.ts     # Skill 执行
└── [其他 Skill 相关]
```

### 5.12 telemetry/

```
src/utils/telemetry/
├── telemetry.ts          # 遥测主模块
├── telemetryEvents.ts   # 遥测事件
├── metrics.ts           # 指标
└── [其他遥测相关]
```

### 5.13 ultraplan/

```
src/utils/ultraplan/
├── planManager.ts        # 计划管理
├── planExecutor.ts       # 计划执行
├── planStorage.ts        # 计划存储
└── [其他计划相关]
```

### 5.14 background/

```
src/utils/background/
├── backgroundTask.ts     # 后台任务
├── backgroundManager.ts  # 后台管理
├── BackgroundTask.ts    # 任务类
└── remote/              # 远程后台
```

### 5.15 sandbox/

```
src/utils/sandbox/
├── sandbox.ts           # 沙箱主模块
├── sandboxExecutor.ts  # 沙箱执行
├── sandboxConfig.ts     # 沙箱配置
└── [其他沙箱相关]
```

### 5.16 swarm/

```
src/utils/swarm/
├── swarm.ts             # Swarm 主模块
├── swarmManager.ts      # Swarm 管理
├── backends/            # 后端实现
└── [其他 Swarm 相关]
```

### 5.17 plugins/

```
src/utils/plugins/
├── pluginManager.ts     # 插件管理
├── pluginLoader.ts     # 插件加载
├── pluginIdentifier.ts # 插件标识
└── [其他插件相关]
```

### 5.18 settings/mdm/

```
src/utils/settings/mdm/
├── mdm.ts              # MDM 支持
├── mdmConfig.ts        # MDM 配置
└── [其他 MDM 相关]
```

---

## 附录: 常用工具速查

### 异步工具

```typescript
import { sleep } from 'src/utils/sleep.ts'
import { withResolvers } from 'src/utils/withResolvers.ts'
import { createAbortController } from 'src/utils/abortController.ts'
import { createCombinedAbortSignal } from 'src/utils/combinedAbortSignal.ts'
```

### 数据结构

```typescript
import { CircularBuffer } from 'src/utils/CircularBuffer.ts'
import { createSignal } from 'src/utils/signal.ts'
```

### 错误处理

```typescript
import { errorMessage, AbortError, ToolError } from 'src/utils/errors.ts'
import { registerCleanup } from 'src/utils/cleanupRegistry.ts'
```

### 文件/路径

```typescript
import { expandPath } from 'src/utils/path.ts'
import { createFileStateCacheWithSizeLimit } from 'src/utils/fileStateCache.ts'
```

### Git

```typescript
import { detectGitRoot, getGitBranch } from 'src/utils/git.ts'
```

### 模型

```typescript
import { getMainLoopModel, setMainLoopModel } from 'src/utils/model/model.ts'
```

### 权限

```typescript
import { hasPermissionsToUseTool } from 'src/utils/permissions/permissions.ts'
```

---

## 总结

Utils 模块是 Claude Code 的工具函数库，特点：

1. **文件数量**: 300+ 个文件
2. **功能全面**: 涵盖文件、网络、Git、Shell、认证等
3. **模块化**: 子目录组织相关功能
4. **可测试**: 独立函数易于测试
5. **可复用**: 跨模块共享
6. **类型安全**: TypeScript 完整类型定义

主要子模块：
- **bash/**: Bash 执行
- **git/**: Git 操作
- **hooks/**: Hook 系统
- **model/**: 模型配置
- **permissions/**: 权限管理
- **settings/**: 设置管理
- **task/**: 任务系统