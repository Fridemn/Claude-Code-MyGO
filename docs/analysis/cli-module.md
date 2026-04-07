# CLI 模块详细文档

## 目录

1. [概述](#1-概述)
2. [目录结构](#2-目录结构)
3. [核心组件](#3-核心组件)
4. [StructuredIO](#4-structuredio)
5. [RemoteIO](#5-remoteio)
6. [Transports](#6-transports)
7. [Handlers](#7-handlers)
8. [设计模式](#8-设计模式)

---

## 1. 概述

CLI 模块是 Claude Code 的命令行交互核心，负责：
- 处理用户输入输出
- 管理 SDK 协议通信
- 实现多种传输协议
- 提供子命令处理器

### 核心概念

```
┌─────────────────────────────────────────────────────────────┐
│                      Claude Code CLI                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  User Input ──► StructuredIO ──► QueryEngine              │
│                         │                                   │
│                         ▼                                   │
│              ┌─────────────────────┐                        │
│              │  SDK Message Loop   │                        │
│              └─────────────────────┘                        │
│                         │                                   │
│         ┌───────────────┼───────────────┐                   │
│         ▼               ▼               ▼                   │
│    StdoutWriter    RemoteIO         Transport              │
│         │               │               │                   │
│         ▼               ▼               ▼                   │
│    Terminal        Remote Mode     WebSocket/SSE            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 目录结构

```
src/cli/
├── print.ts              # 打印和输出管理 (212KB)
├── structuredIO.ts       # StructuredIO 类
├── remoteIO.ts          # Remote IO 管理
├── update.ts            # 版本更新逻辑
├── exit.ts              # CLI 退出辅助函数
├── ndjsonSafeStringify.ts # NDJSON 安全序列化
├── handlers/            # 子命令处理器
│   ├── auth.ts          # 认证处理器
│   ├── agents.ts        # Agent 处理器
│   ├── autoMode.ts      # Auto Mode 处理器
│   ├── mcp.tsx          # MCP 处理器
│   ├── plugins.ts       # 插件处理器
│   └── util.tsx         # 工具处理器
└── transports/          # 传输层实现
    ├── HybridTransport.ts    # 混合传输（WS+HTTP）
    ├── SSETransport.ts       # SSE 传输
    ├── WebSocketTransport.ts # WebSocket 传输
    ├── SerialBatchEventUploader.ts # 批量事件上传
    ├── WorkerStateUploader.ts # Worker 状态上传
    └── transportUtils.ts     # 传输工具
```

---

## 3. 核心组件

### 3.1 print.ts (212KB)

主要的输出管理文件，处理：
- SDK 消息打印
- 权限请求响应
- 控制流消息处理

**主要类**: `PrintManager`

**核心功能**:
- `printMessage(message)`: 打印 SDK 消息
- `printPermissionRequest(request)`: 打印权限请求
- `printError(error)`: 打印错误

### 3.2 exit.ts

CLI 退出辅助函数：

```typescript
/** 写入错误消息到 stderr 并以代码 1 退出 */
export function cliError(msg?: string): never

/** 写入消息到 stdout 并以代码 0 退出 */
export function cliOk(msg?: string): never
```

**使用模式**:
```typescript
// 在子命令处理器中使用
export async function handleCommand(): Promise<void> {
  if (error) {
    return cliError('Error message')
  }
  return cliOk('Success message')
}
```

### 3.3 ndjsonSafeStringify.ts

NDJSON（Newline-Delimited JSON）安全序列化：

```typescript
/**
 * 安全序列化对象为 NDJSON 格式。
 * 处理特殊字符和换行符。
 */
export function ndjsonSafeStringify(obj: unknown): string
```

---

## 4. StructuredIO

### 4.1 概述

`StructuredIO` 是 Claude Code 的核心 IO 类，处理与 SDK 主机的标准输入输出通信。

### 4.2 类定义

```typescript
export class StructuredIO {
  readonly structuredInput: AsyncGenerator<StdinMessage | SDKMessage>
  private readonly pendingRequests = new Map<string, PendingRequest<unknown>>()

  // 恢复的 Worker 状态
  restoredWorkerState: Promise<SessionExternalMetadata | null>

  private inputClosed = false
  private unexpectedResponseCallback?: (
    response: SDKControlResponse,
  ) => Promise<void>

  // 追踪已解析的 tool_use ID
  private resolvedToolUseIds = new Set<string>()
}
```

### 4.3 消息类型

```typescript
// SDK Message Types
type SDKMessage =
  | AssistantMessage
  | UserMessage
  | SystemMessage
  | ToolMessage

// Control Types
type SDKControlRequest =
  | { type: 'control_request'; subtype: 'initialize'; ... }
  | { type: 'control_request'; subtype: 'can_use_tool'; ... }
  | { type: 'control_request'; subtype: 'interrupt'; ... }

type SDKControlResponse =
  | { type: 'control_response'; response: { subtype: 'success'; ... } }
  | { type: 'control_response'; response: { subtype: 'error'; ... } }
```

### 4.4 核心方法

```typescript
// 写入消息到 stdout
write(message: SDKMessage | StdoutMessage): void

// 写入批量消息
writeBatch(messages: SDKMessage[]): void

// 发送控制请求并等待响应
sendControlRequest<T>(
  request: SDKControlRequest,
  schema?: z.Schema,
): Promise<T>

// 发送取消请求
sendControlCancelRequest(requestId: string): void

// 发送结果
sendResult(): void

// 获取待处理请求
getPendingRequests(): Map<string, PendingRequest<unknown>>

// 注册意外响应回调
onUnexpectedResponse(
  callback: (response: SDKControlResponse) => Promise<void>
): void
```

### 4.5 权限请求处理

```typescript
// 处理工具权限请求
async handleToolPermissionRequest(
  request: SDKControlRequest,
): Promise<SDKControlResponse>

// 构建 RequiresActionDetails
function buildRequiresActionDetails(
  tool: Tool,
  input: Record<string, unknown>,
  toolUseID: string,
  requestId: string,
): RequiresActionDetails

// 权限决策序列化
function serializeDecisionReason(
  reason: PermissionDecisionReason | undefined,
): string | undefined
```

---

## 5. RemoteIO

### 5.1 概述

`RemoteIO` 管理远程会话的 IO 操作，用于远程控制场景。

### 5.2 类定义

```typescript
export class RemoteIO {
  // StructuredIO 实例
  structuredIO: StructuredIO

  // 远程传输
  transport: Transport

  // Bridge Handle
  bridgeHandle: ReplBridgeHandle | null

  // 会话元数据
  sessionMetadata: SessionExternalMetadata | null
}
```

### 5.3 核心功能

```typescript
// 初始化 RemoteIO
async function initRemoteIO(params: RemoteIOParams): Promise<RemoteIO>

// 读取远程消息
async function readRemoteMessage(): Promise<SDKMessage | null>

// 写入远程消息
async function writeRemoteMessage(message: SDKMessage): Promise<void>

// 关闭连接
async function closeRemoteIO(): Promise<void>
```

---

## 6. Transports

### 6.1 概述

传输层实现多种通信协议，支持不同的连接场景。

### 6.2 WebSocketTransport

基础 WebSocket 传输：

```typescript
export class WebSocketTransport implements Transport {
  constructor(
    url: URL,
    headers?: Record<string, string>,
    sessionId?: string,
    refreshHeaders?: () => Record<string, string>,
    options?: WebSocketTransportOptions,
  )

  // 连接状态
  readonly state: 'connecting' | 'open' | 'closing' | 'closed'

  // 异步消息生成器
  readonly messages: AsyncGenerator<StdoutMessage>

  // 写入消息
  write(message: StdoutMessage | StdoutMessage[]): void

  // 关闭连接
  close(): void
}
```

**配置选项**:
```typescript
type WebSocketTransportOptions = {
  maxReconnectDelayMs?: number
  reconnectBaseDelayMs?: number
  onReconnect?: (attempt: number) => void
  onClose?: (code: number, reason: string) => void
}
```

### 6.3 HybridTransport

混合传输：WebSocket 读取 + HTTP POST 写入

```
Write 流程:
  write(stream_event) ─┐
                        │ (100ms timer)
                        ▼
  write(other) ────► uploader.enqueue()
                        ▲    │
  writeBatch() ─────────┘    │ serial, batched, retries
                             ▼
                        postOnce() (HTTP POST)
```

**特点**:
- 流事件缓冲（100ms）减少 POST 次数
- 串行化 + 重试 + 背压
- 最多一个 POST 在飞

```typescript
export class HybridTransport extends WebSocketTransport {
  constructor(
    url: URL,
    headers?: Record<string, string>,
    sessionId?: string,
    refreshHeaders?: () => Record<string, string>,
    options?: WebSocketTransportOptions & {
      maxConsecutiveFailures?: number
      onBatchDropped?: (batchSize: number, failures: number) => void
    },
  )
}
```

### 6.4 SSETransport

Server-Sent Events 传输：

```typescript
export class SSETransport implements Transport {
  constructor(
    url: URL,
    headers?: Record<string, string>,
    refreshHeaders?: () => Record<string, string>,
  )

  readonly state: 'connecting' | 'open' | 'closing' | 'closed'
  readonly messages: AsyncGenerator<StdoutMessage>

  write(message: StdoutMessage | StdoutMessage[]): void
  close(): void
}
```

**SSE 帧解析**:
```typescript
type SSEFrame = {
  event?: string
  id?: string
  data?: string
}

export function parseSSEFrames(buffer: string): {
  frames: SSEFrame[]
  remaining: string
}
```

### 6.5 SerialBatchEventUploader

串行批量事件上传器：

```typescript
export class SerialBatchEventUploader<T> {
  constructor(options: {
    maxBatchSize: number
    maxQueueSize: number
    baseDelayMs: number
    maxDelayMs: number
    jitterMs: number
    maxConsecutiveFailures?: number
    onBatchDropped?: (batchSize: number, failures: number) => void
  })

  enqueue(event: T): void
  enqueueBatch(events: T[]): void
  flush(): Promise<void>
  close(): void
}
```

### 6.6 WorkerStateUploader

Worker 状态上传器：

```typescript
export class WorkerStateUploader {
  constructor(options: {
    uploadUrl: string
    headers?: Record<string, string>
    intervalMs?: number
  })

  // 上传状态
  async upload(state: WorkerState): Promise<void>

  // 启动定期上传
  start(): void

  // 停止上传
  stop(): void
}
```

---

## 7. Handlers

### 7.1 概述

Handlers 目录包含各种子命令的处理器实现。

### 7.2 auth.ts

认证相关处理器：

```typescript
// 安装 OAuth Token
export async function installOAuthTokens(
  tokens: OAuthTokens,
): Promise<void>

// 处理登录
export async function handleLogin(): Promise<void>

// 处理登出
export async function handleLogout(): Promise<void>
```

### 7.3 agents.ts

Agent 管理处理器：

```typescript
// 列出 Agents
export async function listAgents(): Promise<void>

// 获取 Agent 配置
export async function getAgent(name: string): Promise<AgentConfig | null>

// 更新 Agent
export async function updateAgent(
  name: string,
  config: Partial<AgentConfig>,
): Promise<void>
```

### 7.4 autoMode.ts

Auto Mode 处理器：

```typescript
// 进入 Auto Mode
export async function enterAutoMode(): Promise<void>

// 退出 Auto Mode
export async function exitAutoMode(): Promise<void>

// 获取 Auto Mode 状态
export async function getAutoModeStatus(): Promise<AutoModeStatus>
```

### 7.5 mcp.tsx

MCP 服务器管理处理器：

```typescript
// 列出 MCP 服务器
export async function listMCPServers(): Promise<void>

// 添加 MCP 服务器
export async function addMCPServer(config: McpServerConfig): Promise<void>

// 移除 MCP 服务器
export async function removeMCPServer(name: string): Promise<void>

// 重启 MCP 服务器
export async function restartMCPServer(name: string): Promise<void>
```

### 7.6 plugins.ts

插件管理处理器：

```typescript
// 列出插件
export async function listPlugins(): Promise<void>

// 安装插件
export async function installPlugin(source: string): Promise<void>

// 卸载插件
export async function uninstallPlugin(name: string): Promise<void>

// 重新加载插件
export async function reloadPlugins(): Promise<void>
```

---

## 8. 设计模式

### 8.1 SDK 协议模式

```
┌─────────────┐    stdin     ┌─────────────┐
│   SDK Host  │ ───────────► │  Claude     │
│   (CLI/App) │              │  Code       │
└─────────────┘              └─────────────┘
      ▲                            │
      │         stdout             │
      └────────────────────────────┘

Control Flow:
  Host ──► control_request (can_use_tool) ──► Code
  Host ◄── control_response (allow/deny) ──── Code
```

### 8.2 传输层模式

```typescript
interface Transport {
  readonly state: 'connecting' | 'open' | 'closing' | 'closed'
  readonly messages: AsyncGenerator<StdoutMessage>
  write(message: StdoutMessage | StdoutMessage[]): void
  close(): void
}
```

### 8.3 混合传输模式

```typescript
class HybridTransport extends WebSocketTransport {
  private postUrl: string
  private uploader: SerialBatchEventUploader<StdoutMessage>

  // 流事件缓冲
  private streamEventBuffer: StdoutMessage[] = []
  private streamEventTimer: ReturnType<typeof setTimeout> | null = null

  write(message: StdoutMessage): void {
    if (isStreamEvent(message)) {
      // 缓冲流事件
      this.bufferStreamEvent(message)
    } else {
      // 立即写入
      this.uploader.enqueue(message)
    }
  }
}
```

### 8.4 错误处理

```typescript
// 永久性错误（不重试）
const PERMANENT_HTTP_CODES = new Set([401, 403, 404])

// 重试配置
const POST_MAX_RETRIES = 10
const POST_BASE_DELAY_MS = 500
const POST_MAX_DELAY_MS = 8000
```

### 8.5 背压机制

```typescript
// SerialBatchEventUploader 背压
enqueue(event: T): void {
  if (this.queue.length >= this.maxQueueSize) {
    // 阻塞直到队列有空位
    await this.waitForSpace()
  }
  this.queue.push(event)
}
```

---

## 附录: 常量定义

### 重连配置

```typescript
const RECONNECT_BASE_DELAY_MS = 1000
const RECONNECT_MAX_DELAY_MS = 30_000
const RECONNECT_GIVE_UP_MS = 600_000  // 10分钟

const LIVENESS_TIMEOUT_MS = 45_000  // 45秒无活动认为断开
```

### POST 配置

```typescript
const POST_MAX_RETRIES = 10
const POST_BASE_DELAY_MS = 500
const POST_MAX_DELAY_MS = 8000
const BATCH_FLUSH_INTERVAL_MS = 100
const POST_TIMEOUT_MS = 15_000
```

### 工具 Use ID 缓存

```typescript
const MAX_RESOLVED_TOOL_USE_IDS = 1000
```

---

## 总结

CLI 模块是 Claude Code 的交互核心，具有以下特点：

1. **StructuredIO**: 统一的 SDK 协议处理
2. **多传输支持**: WebSocket、SSE、Hybrid 多种协议
3. **背压控制**: 防止过载和内存溢出
4. **权限委托**: 支持远程权限请求/响应
5. **子命令处理器**: 模块化的命令处理
6. **远程模式**: 支持 Remote Control 场景