# Bridge 模块详细文档

## 目录

1. [概述](#1-概述)
2. [核心概念](#2-核心概念)
3. [文件结构](#3-文件结构)
4. [核心组件详解](#4-核心组件详解)
5. [两种 Bridge 架构](#5-两种-bridge-架构)
6. [会话管理](#6-会话管理)
7. [权限与安全](#7-权限与安全)
8. [错误处理](#8-错误处理)
9. [设计特点](#9-设计特点)

---

## 1. 概述

Bridge 模块是 Claude Code 的远程控制核心，实现了与 claude.ai 云端的连接，允许用户通过网页或移动端远程操控本地 CLI 会话。

### 功能特性

- **远程会话管理**: 创建、连接、断开远程会话
- **双向通信**: 本地 CLI ↔ 云端服务器的实时消息传递
- **权限委托**: 云端用户可批准本地工具调用
- **会话恢复**: 断线重连、会话持久化
- **多模式支持**: Env-based 和 Env-less 两种架构

### 核心概念

```
┌─────────────┐        ┌─────────────┐        ┌─────────────┐
│ claude.ai   │  ←──→  │ CCR Server  │  ←──→  │ Claude Code │
│ (Web/Mobile)│        │ (Cloud)     │        │ (Local CLI) │
└─────────────┘        └─────────────┘        └─────────────┘
       ↓                      ↓                      ↓
   用户界面              会话调度              本地执行
   权限审批              消息路由              工具调用
```

---

## 2. 核心概念

### 2.1 Remote Control

远程控制功能，允许用户通过 claude.ai 网页或移动应用：
- 查看本地 CLI 会话实时状态
- 发送消息给本地 Claude
- 批准/拒绝工具调用权限请求
- 管理多个远程会话

### 2.2 Bridge Session

Bridge 会话是一个远程连接实例，包含：
- `sessionId`: 会话唯一标识
- `environmentId`: 环境标识（Env-based 模式）
- `accessToken`: 认证令牌
- `sessionIngressUrl`: WebSocket 连接地址

### 2.3 Environment (环境)

Env-based 模式下的概念：
- 一个 Environment 代表一个可运行会话的本地实例
- 支持多会话并发（`maxSessions`）
- 有 `spawnMode`: `single-session` | `worktree` | `same-dir`

### 2.4 Worker

Bridge 实例作为 Worker 注册到服务器：
- `workerType`: 标识来源（`claude_code` | `claude_code_assistant`）
- `workerEpoch`: Worker 版本号（用于心跳验证）

---

## 3. 文件结构

```
src/bridge/
├── types.ts                  # 类型定义
├── bridgeConfig.ts           # 认证配置
├── bridgeEnabled.ts          # 功能开关检查
├── bridgeApi.ts              # Environments API 客户端
├── bridgeMain.ts             # 主入口（100KB+）
├── bridgeMessaging.ts        # 消息处理
├── bridgeUI.ts               # UI 渲染
├── bridgeStatusUtil.ts       # 状态工具
├── bridgePointer.ts          # 持久化指针
├── bridgeDebug.ts            # 调试工具
├── bridgePermissionCallbacks.ts # 权限回调
├── replBridge.ts             # REPL Bridge 核心
├── replBridgeTransport.ts    # Transport 实现
├── replBridgeHandle.ts       # Handle 定义
├── initReplBridge.ts         # Bridge 初始化
├── remoteBridgeCore.ts       # Env-less Bridge 核心
├── envLessBridgeConfig.ts    # Env-less 配置
├── sessionRunner.ts          # 会话运行器
├── createSession.ts          # 会话创建
├── workSecret.ts             # Work Secret 解码
├── jwtUtils.ts               # JWT 工具
├── trustedDevice.ts          # 可信设备认证
├── sessionIdCompat.ts        # Session ID 兼容层
├── pollConfig.ts             # Poll 配置
├── pollConfigDefaults.ts     # 默认配置
├── capacityWake.ts           # 容量唤醒
├── flushGate.ts              # Flush 门控
├── inboundMessages.ts        # 入站消息
├── inboundAttachments.ts     # 入站附件
├── codeSessionApi.ts         # Code Session API
├── debugUtils.ts             # 调试工具
├── stub.ts                   # Stub 实现
```

---

## 4. 核心组件详解

### 4.1 类型定义 (`types.ts`)

#### 核心类型

```typescript
// 默认会话超时（24小时）
export const DEFAULT_SESSION_TIMEOUT_MS = 24 * 60 * 60 * 1000

// Work 数据类型
export type WorkData = {
  type: 'session' | 'healthcheck'
  id: string
}

// Work 响应
export type WorkResponse = {
  id: string
  type: 'work'
  environment_id: string
  state: string
  data: WorkData
  secret: string  // base64url-encoded JSON
  created_at: string
}

// Work Secret（解码后）
export type WorkSecret = {
  version: number
  session_ingress_token: string
  api_base_url: string
  sources: Array<{
    type: string
    git_info?: { type: string; repo: string; ref?: string; token?: string }
  }>
  auth: Array<{ type: string; token: string }>
  claude_code_args?: Record<string, string> | null
  mcp_config?: unknown | null
  environment_variables?: Record<string, string> | null
  use_code_sessions?: boolean
}

// 会话状态
export type SessionDoneStatus = 'completed' | 'failed' | 'interrupted'

// 会话活动类型
export type SessionActivityType = 'tool_start' | 'text' | 'result' | 'error'

// 会话活动
export type SessionActivity = {
  type: SessionActivityType
  summary: string  // 如 "Editing src/foo.ts"
  timestamp: number
}
```

#### Spawn Mode

```typescript
// 启动模式
export type SpawnMode = 'single-session' | 'worktree' | 'same-dir'
// single-session: 单会话，会话结束后 Bridge 退出
// worktree: 每个会话使用独立 Git worktree
// same-dir: 所有会话共享 cwd（可能冲突）
```

#### Bridge Config

```typescript
export type BridgeConfig = {
  dir: string                 // 工作目录
  machineName: string         // 机器名
  branch: string              // Git 分支
  gitRepoUrl: string | null   // Git 仓库 URL
  maxSessions: number         // 最大会话数
  spawnMode: SpawnMode        // 启动模式
  verbose: boolean            // 详细输出
  sandbox: boolean            // 沙箱模式
  bridgeId: string            // Bridge UUID
  workerType: string          // Worker 类型
  environmentId: string       // Environment UUID
  reuseEnvironmentId?: string // 重用的 Environment ID
  apiBaseUrl: string          // API Base URL
  sessionIngressUrl: string   // Session Ingress URL
  debugFile?: string          // 调试文件路径
  sessionTimeoutMs?: number   // 会话超时
}
```

#### 依赖接口

```typescript
// Bridge API 客户端接口
export type BridgeApiClient = {
  registerBridgeEnvironment(config: BridgeConfig): Promise<{
    environment_id: string
    environment_secret: string
  }>
  pollForWork(
    environmentId: string,
    environmentSecret: string,
    signal?: AbortSignal,
    reclaimOlderThanMs?: number,
  ): Promise<WorkResponse | null>
  acknowledgeWork(
    environmentId: string,
    workId: string,
    sessionToken: string,
  ): Promise<void>
  stopWork(environmentId: string, workId: string, force: boolean): Promise<void>
  deregisterEnvironment(environmentId: string): Promise<void>
  sendPermissionResponseEvent(
    sessionId: string,
    event: PermissionResponseEvent,
    sessionToken: string,
  ): Promise<void>
  archiveSession(sessionId: string): Promise<void>
  reconnectSession(environmentId: string, sessionId: string): Promise<void>
  heartbeatWork(
    environmentId: string,
    workId: string,
    sessionToken: string,
  ): Promise<{ lease_extended: boolean; state: string }>
}

// Session Handle
export type SessionHandle = {
  sessionId: string
  done: Promise<SessionDoneStatus>
  kill(): void
  forceKill(): void
  activities: SessionActivity[]       // 活动环形缓冲（~10）
  currentActivity: SessionActivity | null
  accessToken: string
  lastStderr: string[]                // stderr 环形缓冲
  writeStdin(data: string): void
  updateAccessToken(token: string): void
}

// Session Spawner
export type SessionSpawner = {
  spawn(opts: SessionSpawnOpts, dir: string): SessionHandle
}

// Bridge Logger
export type BridgeLogger = {
  printBanner(config: BridgeConfig, environmentId: string): void
  logSessionStart(sessionId: string, prompt: string): void
  logSessionComplete(sessionId: string, durationMs: number): void
  logSessionFailed(sessionId: string, error: string): void
  logStatus(message: string): void
  logVerbose(message: string): void
  logError(message: string): void
  logReconnected(disconnectedMs: number): void
  updateIdleStatus(): void
  updateReconnectingStatus(delayStr: string, elapsedStr: string): void
  updateSessionStatus(
    sessionId: string,
    elapsed: string,
    activity: SessionActivity,
    trail: string[],
  ): void
  // ... 更多 UI 方法
}
```

---

### 4.2 认证配置 (`bridgeConfig.ts`)

```typescript
// Ant-only 开发覆盖：CLAUDE_BRIDGE_OAUTH_TOKEN
export function getBridgeTokenOverride(): string | undefined

// Ant-only 开发覆盖：CLAUDE_BRIDGE_BASE_URL
export function getBridgeBaseUrlOverride(): string | undefined

// Bridge API 访问令牌
export function getBridgeAccessToken(): string | undefined {
  return getBridgeTokenOverride() ?? getClaudeAIOAuthTokens()?.accessToken
}

// Bridge API Base URL
export function getBridgeBaseUrl(): string {
  return getBridgeBaseUrlOverride() ?? getOauthConfig().BASE_API_URL
}
```

---

### 4.3 功能开关 (`bridgeEnabled.ts`)

```typescript
// Runtime 检查 Bridge 是否启用
export function isBridgeEnabled(): boolean {
  return feature('BRIDGE_MODE')
    ? isClaudeAISubscriber() &&
        getFeatureValue_CACHED_MAY_BE_STALE('tengu_ccr_bridge', false)
    : false
}

// Blocking 检查（等待 GrowthBook 初始化）
export async function isBridgeEnabledBlocking(): Promise<boolean>

// 获取禁用原因（用于错误提示）
export async function getBridgeDisabledReason(): Promise<string | null>

// Env-less (v2) 模式检查
export function isEnvLessBridgeEnabled(): boolean

// CSE Shim 开关
export function isCseShimEnabled(): boolean

// 版本检查
export function checkBridgeMinVersion(): string | null

// CCR Auto-Connect 默认值
export function getCcrAutoConnectDefault(): boolean

// CCR Mirror 模式
export function isCcrMirrorEnabled(): boolean
```

---

### 4.4 Bridge API (`bridgeApi.ts`)

#### 错误类型

```typescript
// 致命错误（不可重试）
export class BridgeFatalError extends Error {
  readonly status: number
  readonly errorType: string | undefined
  constructor(message: string, status: number, errorType?: string)
}

// ID 验证（防止路径注入）
export function validateBridgeId(id: string, label: string): string {
  if (!id || !SAFE_ID_PATTERN.test(id)) {
    throw new Error(`Invalid ${label}: contains unsafe characters`)
  }
  return id
}
```

#### API 客户端创建

```typescript
export function createBridgeApiClient(deps: BridgeApiDeps): BridgeApiClient {
  // deps 包含：
  // - baseUrl: API 地址
  // - getAccessToken: 令牌获取函数
  // - runnerVersion: 版本号
  // - onDebug: 调试日志
  // - onAuth401: 401 处理（OAuth refresh）
  // - getTrustedDeviceToken: 可信设备令牌
}
```

#### API 方法实现

```typescript
// 注册 Environment
async registerBridgeEnvironment(config: BridgeConfig): Promise<{
  environment_id: string
  environment_secret: string
}>

// Poll 等待 Work
async pollForWork(
  environmentId: string,
  environmentSecret: string,
  signal?: AbortSignal,
  reclaimOlderThanMs?: number,
): Promise<WorkResponse | null>

// Acknowledge Work
async acknowledgeWork(
  environmentId: string,
  workId: string,
  sessionToken: string,
): Promise<void>

// Stop Work
async stopWork(environmentId: string, workId: string, force: boolean): Promise<void>

// Deregister Environment
async deregisterEnvironment(environmentId: string): Promise<void>

// Archive Session
async archiveSession(sessionId: string): Promise<void>

// Reconnect Session
async reconnectSession(environmentId: string, sessionId: string): Promise<void>

// Heartbeat Work
async heartbeatWork(
  environmentId: string,
  workId: string,
  sessionToken: string,
): Promise<{ lease_extended: boolean; state: string }>

// Send Permission Response
async sendPermissionResponseEvent(
  sessionId: string,
  event: PermissionResponseEvent,
  sessionToken: string,
): Promise<void>
```

---

### 4.5 Session Runner (`sessionRunner.ts`)

#### Permission Request

```typescript
export type PermissionRequest = {
  type: 'control_request'
  request_id: string
  request: {
    subtype: 'can_use_tool'
    tool_name: string
    input: Record<string, unknown>
    tool_use_id: string
  }
}
```

#### Tool Summary

```typescript
const TOOL_VERBS: Record<string, string> = {
  Read: 'Reading',
  Write: 'Writing',
  Edit: 'Editing',
  MultiEdit: 'Editing',
  Bash: 'Running',
  Glob: 'Searching',
  Grep: 'Searching',
  WebFetch: 'Fetching',
  WebSearch: 'Searching',
  Task: 'Running task',
  // ...
}

function toolSummary(name: string, input: Record<string, unknown>): string {
  const verb = TOOL_VERBS[name] ?? name
  const target = input.file_path ?? input.pattern ?? input.command?.slice(0, 60) ?? ''
  return target ? `${verb} ${target}` : verb
}
```

#### Activity Extraction

```typescript
function extractActivities(
  line: string,
  sessionId: string,
  onDebug: (msg: string) => void,
): SessionActivity[]
// 从 NDJSON 行提取活动信息
// 处理 assistant (tool_use, text) 和 result 类型
```

#### Session Spawner

```typescript
export function createSessionSpawner(deps: SessionSpawnerDeps): SessionSpawner {
  return {
    spawn(opts: SessionSpawnOpts, dir: string): SessionHandle {
      // 1. 解析 debugFile 配置
      // 2. 构建 CLI 参数
      // 3. 设置环境变量
      // 4. spawn 子进程
      // 5. 创建 SessionHandle
    }
  }
}
```

#### 环境变量设置

```typescript
const env: NodeJS.ProcessEnv = {
  ...deps.env,
  CLAUDE_CODE_OAUTH_TOKEN: undefined,  // 剥离 Bridge 的 OAuth
  CLAUDE_CODE_ENVIRONMENT_KIND: 'bridge',
  ...(deps.sandbox && { CLAUDE_CODE_FORCE_SANDBOX: '1' }),
  CLAUDE_CODE_SESSION_ACCESS_TOKEN: opts.accessToken,
  CLAUDE_CODE_POST_FOR_SESSION_INGRESS_V2: '1',
  // v2 模式
  ...(opts.useCcrV2 && {
    CLAUDE_CODE_USE_CCR_V2: '1',
    CLAUDE_CODE_WORKER_EPOCH: String(opts.workerEpoch),
  }),
}
```

---

### 4.6 Work Secret (`workSecret.ts`)

```typescript
// 解码 Work Secret
export function decodeWorkSecret(secret: string): WorkSecret {
  const json = Buffer.from(secret, 'base64url').toString('utf-8')
  const parsed = jsonParse(json)
  // 验证 version === 1
  // 验证 session_ingress_token 存在
  return parsed as WorkSecret
}

// 构建 SDK URL
export function buildSdkUrl(apiBaseUrl: string, sessionId: string): string {
  // localhost: ws:// + /v2/
  // production: wss:// + /v1/
}

// 构建 CCR v2 SDK URL
export function buildCCRv2SdkUrl(apiBaseUrl: string, sessionId: string): string {
  return `${base}/v1/code/sessions/${sessionId}`
}

// Session ID 比较（忽略 tag prefix）
export function sameSessionId(a: string, b: string): boolean {
  if (a === b) return true
  const aBody = a.slice(a.lastIndexOf('_') + 1)
  const bBody = b.slice(b.lastIndexOf('_') + 1)
  return aBody.length >= 4 && aBody === bBody
}

// Register Worker
export async function registerWorker(
  sessionUrl: string,
  accessToken: string,
): Promise<number> {
  // POST /worker/register → worker_epoch
}
```

---

### 4.7 Session Management (`createSession.ts`)

```typescript
// 创建 Bridge Session
export async function createBridgeSession({
  environmentId,
  title,
  events,
  gitRepoUrl,
  branch,
  signal,
  baseUrl: baseUrlOverride,
  getAccessToken,
  permissionMode,
}): Promise<string | null>
// POST /v1/sessions
// 返回 session ID

// 获取 Bridge Session
export async function getBridgeSession(
  sessionId: string,
  opts?: { baseUrl?: string; getAccessToken?: () => string | undefined },
): Promise<{ environment_id?: string; title?: string } | null>
// GET /v1/sessions/{id}

// Archive Bridge Session
export async function archiveBridgeSession(
  sessionId: string,
  opts?: { baseUrl?: string; getAccessToken?: () => string | undefined; timeoutMs?: number },
): Promise<void>
// POST /v1/sessions/{id}/archive

// Update Bridge Session Title
export async function updateBridgeSessionTitle(
  sessionId: string,
  title: string,
  opts?: { baseUrl?: string; getAccessToken?: () => string | undefined },
): Promise<void>
// PATCH /v1/sessions/{id}
```

---

## 5. 两种 Bridge 架构

### 5.1 Env-based (v1)

**路径**: `replBridge.ts` → `initBridgeCore()`

**流程**:
```
1. POST /v1/environments/bridge     → environment_id + environment_secret
2. POST /v1/sessions                → session_id
3. GET  /v1/environments/{id}/work/poll  → work_response (with secret)
4. Decode work_secret               → session_ingress_token + api_base_url
5. WebSocket 连接                   → 消息传递
6. Heartbeat                        → 延长 lease
7. Teardown                         → deregister + archive
```

**特点**:
- 需要 Environments API 层
- Poll-Dispatch 模式
- 支持 `--session-id` 恢复
- 适合 `claude remote-control` 命令

### 5.2 Env-less (v2)

**路径**: `remoteBridgeCore.ts` → `initEnvLessBridgeCore()`

**流程**:
```
1. POST /v1/code/sessions           → session_id (无 env_id)
2. POST /v1/code/sessions/{id}/bridge → worker_jwt + expires_in + worker_epoch
3. SSE 连接 + CCRClient            → 消息传递
4. Token Refresh Scheduler         → 定期刷新 JWT
5. 401 Recovery                     → 重建 transport
```

**特点**:
- 无 Environments API 层
- 直接连接 Session Ingress
- OAuth → Worker JWT 交换
- 更简单、更快
- REPL `/remote-control` 优先使用

### 5.3 对比

| 特性 | Env-based | Env-less |
|------|-----------|----------|
| API 层 | Environments + Sessions | Sessions only |
| 连接方式 | WebSocket (HybridTransport) | SSE + CCRClient |
| Auth | Environment Secret | Worker JWT |
| 恢复 | `--session-id` resume | 自带 perpetual 模式 |
| 复杂度 | 较高 (~2400 lines) | 较低 (~500 lines) |
| 适用场景 | standalone bridge | REPL |

---

## 6. 会话管理

### 6.1 会话生命周期

```
┌─────────────────────────────────────────────────────────────┐
│                    Session Lifecycle                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. Create Session                                          │
│     POST /v1/sessions → sessionId                           │
│         ↓                                                   │
│  2. Connect                                                 │
│     WebSocket/SSE → Bidirectional messaging                 │
│         ↓                                                   │
│  3. Poll/Heartbeat                                          │
│     Keep lease alive                                        │
│         ↓                                                   │
│  4. Activity                                                │
│     Tool calls, text output, results                        │
│         ↓                                                   │
│  5. Disconnect                                              │
│     SIGTERM → Graceful shutdown                             │
│         ↓                                                   │
│  6. Archive                                                 │
│     POST /v1/sessions/{id}/archive                          │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### 6.2 会话状态

```typescript
export type BridgeState = 'ready' | 'connected' | 'reconnecting' | 'failed'

// Session Done Status
export type SessionDoneStatus = 'completed' | 'failed' | 'interrupted'
```

### 6.3 权限请求处理

```typescript
// Permission Request (from child CLI)
export type PermissionRequest = {
  type: 'control_request'
  request_id: string
  request: {
    subtype: 'can_use_tool'
    tool_name: string
    input: Record<string, unknown>
    tool_use_id: string
  }
}

// Permission Response (to session)
export type PermissionResponseEvent = {
  type: 'control_response'
  response: {
    subtype: 'success'
    request_id: string
    response: Record<string, unknown>  // { behavior: 'allow' | 'deny' }
  }
}
```

---

## 7. 权限与安全

### 7.1 认证层

```typescript
// OAuth Token（用户登录）
CLAUDE_CODE_OAUTH_TOKEN → getClaudeAIOAuthTokens()

// Session Access Token（会话级别）
CLAUDE_CODE_SESSION_ACCESS_TOKEN → work_secret.session_ingress_token

// Worker JWT（v2 模式）
POST /bridge → worker_jwt + expires_in

// Environment Secret（v1 模式）
POST /environments/bridge → environment_secret
```

### 7.2 可信设备认证

```typescript
// Elevated Auth（高安全级别）
export function getTrustedDeviceToken(): string | undefined

// Header
X-Trusted-Device-Token: <token>
```

### 7.3 ID 验证

```typescript
// Safe ID Pattern（防止路径注入）
const SAFE_ID_PATTERN = /^[a-zA-Z0-9_-]+$/

export function validateBridgeId(id: string, label: string): string {
  if (!id || !SAFE_ID_PATTERN.test(id)) {
    throw new Error(`Invalid ${label}: contains unsafe characters`)
  }
  return id
}
```

### 7.4 401 处理

```typescript
// OAuth Refresh
async function withOAuthRetry<T>(
  fn: (accessToken: string) => Promise<{ status: number; data: T }>,
  context: string,
): Promise<{ status: number; data: T }>
// 401 → onAuth401 → refresh → retry
```

---

## 8. 错误处理

### 8.1 BridgeFatalError

```typescript
export class BridgeFatalError extends Error {
  readonly status: number
  readonly errorType: string | undefined  // 如 "environment_expired"
}

// 错误类型检查
export function isExpiredErrorType(errorType: string | undefined): boolean
export function isSuppressible403(err: BridgeFatalError): boolean
```

### 8.2 状态码处理

```typescript
function handleErrorStatus(status: number, data: unknown, context: string): void {
  switch (status) {
    case 401: throw new BridgeFatalError('Auth failed', 401)
    case 403: throw new BridgeFatalError('Access denied', 403)
    case 404: throw new BridgeFatalError('Not found', 404)
    case 410: throw new BridgeFatalError('Expired', 410)
    case 429: throw new Error('Rate limited')
    default: throw new Error(`Failed with status ${status}`)
  }
}
```

### 8.3 Poll Error Recovery

```typescript
const POLL_ERROR_INITIAL_DELAY_MS = 2_000
const POLL_ERROR_MAX_DELAY_MS = 60_000
const POLL_ERROR_GIVE_UP_MS = 15 * 60 * 1000  // 15分钟后放弃

// Exponential backoff on poll failure
```

---

## 9. 设计特点

### 9.1 依赖注入

所有核心函数接收参数对象而非直接导入依赖：

```typescript
export async function initBridgeCore(params: BridgeCoreParams): Promise<BridgeCoreHandle | null>
export function createBridgeApiClient(deps: BridgeApiDeps): BridgeApiClient
export function createSessionSpawner(deps: SessionSpawnerDeps): SessionSpawner
```

### 9.2 Feature Flags

通过 GrowthBook 控制功能：

```typescript
// Bridge 模式启用
feature('BRIDGE_MODE') && getFeatureValue('tengu_ccr_bridge', false)

// v2 模式
getFeatureValue('tengu_bridge_repl_v2', false)

// Auto-connect
getFeatureValue('tengu_cobalt_harbor', false)

// Mirror mode
getFeatureValue('tengu_ccr_mirror', false)
```

### 9.3 OAuth 401 处理

```typescript
// 模式：401 → refresh → retry
async function withOAuthRetry<T>(fn, context): Promise<T>
```

### 9.4 Crash Recovery

```typescript
// Bridge Pointer（持久化）
writeBridgePointer(dir, { environmentId, sessionId, source: 'repl' })
readBridgePointer(dir) → prior state
clearBridgePointer(dir) → cleanup

// Perpetual mode
perpetual: true → 不清除 pointer，允许 crash 后恢复
```

### 9.5 子进程管理

```typescript
// Session Handle
kill(): void        // SIGTERM (graceful)
forceKill(): void   // SIGKILL (force)
writeStdin(data): void  // 直接写入 stdin
updateAccessToken(token): void  // 刷新令牌
```

### 9.6 消息格式

```typescript
// NDJSON（子进程 stdout）
{ type: 'assistant', message: { content: [{ type: 'tool_use', ... }] } }
{ type: 'result', subtype: 'success' }
{ type: 'control_request', request: { subtype: 'can_use_tool', ... } }

// SDK Message（WebSocket/SSE）
{ type: 'text', text: '...' }
{ type: 'tool_use', name: '...', input: {...} }
{ type: 'control_response', response: {...} }
```

---

## 附录: 关键类型索引

### API 类型

| 类型 | 文件 | 描述 |
|------|------|------|
| `BridgeConfig` | types.ts | Bridge 配置 |
| `BridgeApiClient` | types.ts | API 客户端接口 |
| `WorkResponse` | types.ts | Work Poll 响应 |
| `WorkSecret` | types.ts | Work Secret 解码后 |
| `PermissionResponseEvent` | types.ts | 权限响应事件 |

### Session 类型

| 类型 | 文件 | 描述 |
|------|------|------|
| `SessionHandle` | types.ts | 会话 Handle |
| `SessionSpawner` | types.ts | 会话 Spawner |
| `SessionSpawnOpts` | types.ts | Spawn 选项 |
| `SessionActivity` | types.ts | 会话活动 |
| `SessionDoneStatus` | types.ts | 会话完成状态 |

### Bridge Handle 类型

| 类型 | 文件 | 描述 |
|------|------|------|
| `ReplBridgeHandle` | replBridge.ts | REPL Bridge Handle |
| `BridgeCoreHandle` | replBridge.ts | Core Bridge Handle |
| `BridgeCoreParams` | replBridge.ts | Core 初始化参数 |
| `EnvLessBridgeParams` | remoteBridgeCore.ts | Env-less 参数 |
| `BridgeState` | replBridge.ts | Bridge 状态 |

---

## 总结

Bridge 模块是 Claude Code 远程控制的核心，具有以下特点：

1. **双架构设计**: Env-based (poll-dispatch) 和 Env-less (direct) 两种模式
2. **依赖注入**: 所有核心函数接收参数对象
3. **Feature Flags**: GrowthBook 控制功能开关
4. **OAuth 处理**: 401 自动 refresh + retry
5. **Crash Recovery**: Bridge Pointer 持久化 + perpetual 模式
6. **子进程管理**: SIGTERM/SIGKILL + stdin 写入 + token 刷新
7. **安全验证**: ID 验证 + 可信设备认证
8. **错误分类**: BridgeFatalError + 状态码处理