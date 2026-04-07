# Core 模块详细文档

## 目录

1. [Tool.ts](#toolts) - 工具接口定义
2. [Task.ts](#taskts) - 任务系统
3. [QueryEngine.ts](#queryenginets) - 查询引擎
4. [commands.ts](#commandsts) - 命令系统
5. [tools.ts](#toolsts) - 工具注册
6. [types/command.ts](#typescommandts) - 命令类型
7. [types/message.ts](#typesmessagets) - 消息类型
8. [AppStateStore.ts](#appstatestoretts) - 状态存储
9. [AppState.tsx](#appstatetsx) - 状态提供器

---

## Tool.ts

**文件路径**: `src/Tool.ts`

### 功能概述

Tool.ts 是 Claude Code 工具系统的核心定义文件，定义了 AI 模型可以执行的所有操作的接口标准。

### 核心类型

#### Tool 接口

```typescript
export type Tool<
  Input extends AnyObject = AnyObject,
  Output = unknown,
  P extends ToolProgressData = ToolProgressData,
> = {
  name: string                           // 工具唯一名称
  aliases?: string[]                     // 别名列表（用于向后兼容）
  searchHint?: string                    // 工具搜索提示（3-10个词，无句号）
  inputSchema: Input                     // 输入验证模式（Zod schema）
  inputJSONSchema?: ToolInputJSONSchema  // MCP JSON Schema 格式
  outputSchema?: z.ZodType<unknown>      // 输出验证模式
  maxResultSizeChars: number             // 结果最大字符数，超出则持久化到磁盘
  strict?: boolean                       // 严格模式（tengu_tool_pear 启用时）

  // 核心执行方法
  call(args, context, canUseTool, parentMessage, onProgress?): Promise<ToolResult<Output>>

  // 描述生成
  description(input, options): Promise<string>
  prompt(options): Promise<string>

  // 状态判断方法
  isEnabled(): boolean
  isConcurrencySafe(input): boolean      // 是否可并发执行
  isReadOnly(input): boolean            // 是否只读操作
  isDestructive?(input): boolean        // 是否破坏性操作
  interruptBehavior?(): 'cancel' | 'block'  // 用户中断时的行为
  isSearchOrReadCommand?(input): { isSearch, isRead, isList? }  // 是否搜索/读取操作
  requiresUserInteraction?(): boolean   // 是否需要用户交互

  // 权限相关
  validateInput?(input, context): Promise<ValidationResult>
  checkPermissions(input, context): Promise<PermissionResult>
  preparePermissionMatcher?(input): Promise<(pattern: string) => boolean>

  // MCP 相关
  isMcp?: boolean
  isLsp?: boolean
  mcpInfo?: { serverName: string; toolName: string }

  // 延迟加载
  readonly shouldDefer?: boolean   // 延迟加载，需要 ToolSearch 激活
  readonly alwaysLoad?: boolean    // 总是加载，忽略延迟加载

  // 安全分类
  toAutoClassifierInput(input): unknown  // 用于安全分类器
  getPath?(input): string                // 获取文件路径（用于权限规则匹配）

  // UI 渲染方法
  renderToolUseMessage(input, options): React.ReactNode
  renderToolResultMessage?(content, progressMessages, options): React.ReactNode
  renderToolUseProgressMessage?(progressMessages, options): React.ReactNode
  renderToolUseRejectedMessage?(input, options): React.ReactNode
  renderToolUseErrorMessage?(result, options): React.ReactNode
  renderToolUseTag?(input): React.ReactNode
  renderToolUseQueuedMessage?(): React.ReactNode
  renderGroupedToolUse?(toolUses, options): React.ReactNode | null

  // 结果处理
  mapToolResultToToolResultBlockParam(content, toolUseID): ToolResultBlockParam
  extractSearchText?(out): string         // 提取用于搜索的文本
  isResultTruncated?(output): boolean     // 结果是否被截断
  isTransparentWrapper?(): boolean        // 是否为透明包装器

  // 用户显示
  userFacingName(input): string
  userFacingNameBackgroundColor?(input): keyof Theme | undefined
  getToolUseSummary?(input): string | null      // 简洁摘要
  getActivityDescription?(input): string | null  // 活动描述（用于旋转动画）

  // 输入处理
  backfillObservableInput?(input): void    // 填充派生的输入字段
  inputsEquivalent?(a, b): boolean         // 判断两个输入是否等效
}
```

#### ToolResult 类型

```typescript
export type ToolResult<T> = {
  data: T                           // 工具执行结果
  newMessages?: (                  // 可选的附加消息
    | UserMessage
    | AssistantMessage
    | AttachmentMessage
    | SystemMessage
  )[]
  // 上下文修改器，用于非并发安全的工具
  contextModifier?: (context: ToolUseContext) => ToolUseContext
  // MCP 元数据
  mcpMeta?: {
    _meta?: Record<string, unknown>
    structuredContent?: Record<string, unknown>
  }
}
```

#### ToolUseContext 类型

工具执行时的上下文信息，包含：

- **options**: 配置选项（commands, debug, tools, verbose, thinkingConfig, mcpClients 等）
- **abortController**: 中断控制器
- **readFileState**: 文件读取状态缓存
- **getAppState / setAppState**: 状态读写函数
- **setAppStateForTasks**: 用于后台任务的共享状态更新
- **handleElicitation**: URL 诱导请求处理器
- **setToolJSX**: JSX 设置函数
- **addNotification**: 添加通知
- **appendSystemMessage**: 添加系统消息
- **sendOSNotification**: 发送系统通知
- **nestedMemoryAttachmentTriggers**: 嵌套内存附件触发器
- **discoveredSkillNames**: 发现的技能名称
- **updateFileHistoryState**: 更新文件历史状态
- **updateAttributionState**: 更新归属状态
- **toolDecisions**: 工具决策追踪
- **queryTracking**: 查询链追踪
- **requestPrompt**: 请求用户输入
- **contentReplacementState**: 内容替换状态
- **renderedSystemPrompt**: 渲染后的系统提示

#### ValidationResult 类型

```typescript
export type ValidationResult =
  | { result: true }
  | { result: false; message: string; errorCode: number }
```

### 工厂函数

#### buildTool

```typescript
export function buildTool<D extends AnyToolDef>(def: D): BuiltTool<D>
```

使用默认值为不完整的工具定义填充，生成完整的 `Tool` 对象。

**默认行为**:
- `isEnabled`: `true`
- `isConcurrencySafe`: `false`
- `isReadOnly`: `false`
- `isDestructive`: `false`
- `checkPermissions`: `{ behavior: 'allow', updatedInput }`
- `toAutoClassifierInput`: `''` (跳过分类器)
- `userFacingName`: `name`

### 工具集合类型

```typescript
export type Tools = readonly Tool[]
```

### 工具查找函数

```typescript
export function toolMatchesName(
  tool: { name: string; aliases?: string[] },
  name: string,
): boolean

export function findToolByName(tools: Tools, name: string): Tool | undefined
```

### 进度类型

```typescript
export type ToolProgressData =
  | BashProgress
  | AgentToolProgress
  | MCPProgress
  | REPLToolProgress
  | SkillToolProgress
  | TaskOutputProgress
  | WebSearchProgress

export type ToolProgress<P extends ToolProgressData> = {
  toolUseID: string
  data: P
}

export type ToolCallProgress<P extends ToolProgressData> = (
  progress: ToolProgress<P>
) => void
```

### 关键设计模式

1. **泛型设计**: Tool 使用三个泛型参数（Input, Output, Progress）实现类型安全
2. **工厂模式**: buildTool 工厂函数简化工具创建
3. **策略模式**: 不同工具实现不同的权限检查、验证策略
4. **观察者模式**: 通过 onProgress 回调报告进度
5. **延迟加载**: shouldDefer 支持按需加载工具

---

## Task.ts

**文件路径**: `src/Task.ts`

### 功能概述

Task.ts 定义了 Claude Code 的任务系统，用于管理后台运行的子任务。

### 核心类型

#### TaskType 枚举

```typescript
export type TaskType =
  | 'local_bash'           // 本地 Shell 命令
  | 'local_agent'          // 本地子代理
  | 'remote_agent'         // 远程代理
  | 'in_process_teammate'  // 进程内队友
  | 'local_workflow'       // 本地工作流
  | 'monitor_mcp'          // MCP 监控
  | 'dream'                // 后台处理任务
```

#### TaskStatus 枚举

```typescript
export type TaskStatus =
  | 'pending'      // 待执行
  | 'running'      // 运行中
  | 'completed'    // 已完成
  | 'failed'       // 失败
  | 'killed'       // 已终止
```

#### 状态判断函数

```typescript
export function isTerminalTaskStatus(status: TaskStatus): boolean
// 返回 status === 'completed' || status === 'failed' || status === 'killed'
```

#### TaskStateBase 类型

```typescript
export type TaskStateBase = {
  id: string               // 任务唯一 ID
  type: TaskType           // 任务类型
  status: TaskStatus        // 任务状态
  description: string       // 任务描述
  toolUseId?: string       // 关联的工具调用 ID
  startTime: number        // 开始时间戳
  endTime?: number          // 结束时间戳
  totalPausedMs?: number    // 总暂停时间
  outputFile: string        // 输出文件路径
  outputOffset: number      // 输出偏移量
  notified: boolean        // 是否已通知
}
```

#### TaskHandle 类型

```typescript
export type TaskHandle = {
  taskId: string
  cleanup?: () => void
}
```

#### TaskContext 类型

```typescript
export type TaskContext = {
  abortController: AbortController
  getAppState: () => AppState
  setAppState: SetAppState
}
```

#### Task 接口

```typescript
export type Task = {
  name: string
  type: TaskType
  kill(taskId: string, setAppState: SetAppState): Promise<void>
}
```

#### LocalShellSpawnInput 类型

```typescript
export type LocalShellSpawnInput = {
  command: string
  description: string
  timeout?: number
  toolUseId?: string
  agentId?: AgentId
  /** UI 显示变体: description-as-label, dialog title, status栏 */
  kind?: 'bash' | 'monitor'
}
```

### 工具函数

#### 任务 ID 生成

```typescript
const TASK_ID_PREFIXES: Record<string, string> = {
  local_bash: 'b',           // 保持 'b' 以兼容
  local_agent: 'a',
  remote_agent: 'r',
  in_process_teammate: 't',
  local_workflow: 'w',
  monitor_mcp: 'm',
  dream: 'd',
}

export function generateTaskId(type: TaskType): string
// 格式: {prefix}{8个随机字符}
// 示例: 'b1a2b3c4d5'

export function createTaskStateBase(
  id: string,
  type: TaskType,
  description: string,
  toolUseId?: string,
): TaskStateBase
```

**安全设计**: 使用 36^8 ≈ 2.8 万亿种组合，可抵抗暴力破解符号链接攻击。

### 设计特点

1. **类型安全**: 任务类型和状态通过 TypeScript 枚举严格定义
2. **统一接口**: 所有任务类型实现统一的 kill 方法
3. **输出持久化**: 大输出写入磁盘，内存中只保存偏移量
4. **状态追踪**: 支持暂停时间累计和通知状态追踪

---

## QueryEngine.ts

**文件路径**: `src/QueryEngine.ts`

### 功能概述

QueryEngine 是查询生命周期管理的核心类，处理用户输入、API 调用、工具执行和响应生成。

### 类定义

```typescript
export class QueryEngine {
  private config: QueryEngineConfig
  private mutableMessages: Message[]
  private abortController: AbortController
  private permissionDenials: SDKPermissionDenial[]
  private totalUsage: NonNullableUsage
  private hasHandledOrphanedPermission: boolean
  private readFileState: FileStateCache
  private discoveredSkillNames: Set<string>
  private loadedNestedMemoryPaths: Set<string>
}
```

### 构造函数

```typescript
constructor(config: QueryEngineConfig)
```

### 核心方法

#### submitMessage

```typescript
async *submitMessage(
  prompt: string | ContentBlockParam[],
  options?: { uuid?: string; isMeta?: boolean },
): AsyncGenerator<SDKMessage, void, unknown>
```

**执行流程**:

1. **初始化**
   - 清除技能发现追踪
   - 设置工作目录
   - 创建包装的 canUseTool（追踪权限拒绝）

2. **获取系统提示**
   ```typescript
   const { defaultSystemPrompt, userContext, systemContext } =
     await fetchSystemPromptParts({ tools, mainLoopModel, ... })
   ```

3. **处理用户输入**
   ```typescript
   const { messages, shouldQuery, allowedTools, model, resultText } =
     await processUserInput({ input: prompt, ... })
   ```

4. **权限上下文更新**
   - 根据处理结果更新 `alwaysAllowRules`

5. **API 调用循环**
   ```typescript
   for await (const message of query({ messages, systemPrompt, ... })) {
     // 处理不同类型的消息
   }
   ```

6. **结果处理**
   - 追踪 token 使用量
   - 检查预算限制（USD、最大轮次等）
   - 生成最终结果消息

#### interrupt

```typescript
interrupt(): void
// 中断当前查询
```

#### getMessages

```typescript
getMessages(): readonly Message[]
// 获取当前消息列表
```

#### getReadFileState

```typescript
getReadFileState(): FileStateCache
// 获取文件读取状态缓存
```

#### getSessionId

```typescript
getSessionId(): string
// 获取会话 ID
```

#### setModel

```typescript
setModel(model: string): void
// 设置使用的模型
```

### QueryEngineConfig 配置

```typescript
export type QueryEngineConfig = {
  cwd: string                          // 工作目录
  tools: Tools                         // 可用工具列表
  commands: Command[]                   // 可用命令列表
  mcpClients: MCPServerConnection[]    // MCP 客户端
  agents: AgentDefinition[]            // 代理定义
  canUseTool: CanUseToolFn             // 权限检查函数
  getAppState: () => AppState          // 获取状态
  setAppState: (f) => void             // 设置状态
  initialMessages?: Message[]          // 初始消息
  readFileCache: FileStateCache        // 文件缓存
  customSystemPrompt?: string          // 自定义系统提示
  appendSystemPrompt?: string          // 附加系统提示
  userSpecifiedModel?: string          // 用户指定模型
  fallbackModel?: string               // 备用模型
  thinkingConfig?: ThinkingConfig      // 思考配置
  maxTurns?: number                    // 最大轮次
  maxBudgetUsd?: number                // 最大 USD 预算
  taskBudget?: { total: number }       // 任务预算
  jsonSchema?: Record<string, unknown> // JSON Schema（结构化输出）
  verbose?: boolean                    // 详细模式
  replayUserMessages?: boolean         // 重放用户消息
  handleElicitation?: ToolUseContext['handleElicitation']
  includePartialMessages?: boolean     // 包含部分消息
  setSDKStatus?: (status: SDKStatus) => void
  abortController?: AbortController
  orphanedPermission?: OrphanedPermission
  snipReplay?: (yieldedSystemMsg, store) => { messages: Message[]; executed: boolean } | undefined
}
```

### 便捷函数

#### ask

```typescript
export async function* ask({ ... }): AsyncGenerator<SDKMessage, void, unknown>
```

`QueryEngine` 的便捷包装器，用于单次查询场景。

### 消息处理

QueryEngine 处理多种 SDKMessage 类型：

- `assistant`: 助手消息
- `user`: 用户消息
- `progress`: 进度消息
- `attachment`: 附件消息
- `stream_event`: 流式事件
- `system`: 系统消息（包括 `compact_boundary`, `api_error`）
- `tool_use_summary`: 工具使用摘要

### 错误处理

QueryEngine 追踪多种错误情况：

- `error_during_execution`: 执行期间错误
- `error_max_turns`: 超过最大轮次
- `error_max_budget_usd`: 超出 USD 预算
- `error_max_structured_output_retries`: 结构化输出重试超限

### 设计特点

1. **AsyncGenerator**: 使用 AsyncGenerator 实现流式响应
2. **状态持久化**: mutableMessages 跨轮次持久化
3. **权限追踪**: 包装 canUseTool 追踪所有权限拒绝
4. **预算管理**: 支持 USD 预算、最大轮次、任务预算
5. **会话管理**: 支持消息重放和会话恢复

---

## commands.ts

**文件路径**: `src/commands.ts`

### 功能概述

commands.ts 管理所有斜杠命令的注册、加载和查找。

### 命令来源

1. **内置命令** (`COMMANDS()`): 系统内置命令
2. **技能目录命令** (`skillDirCommands`): 从 `.claude/skills/` 加载
3. **插件技能** (`pluginSkills`): 从插件加载
4. **打包技能** (`bundledSkills`): 预打包技能
5. **内置插件技能** (`builtinPluginSkills`): 内置插件提供的技能
6. **工作流命令** (`workflowCommands`): 工作流脚本

### 核心函数

#### getCommands

```typescript
export async function getCommands(cwd: string): Promise<Command[]>
// 获取所有可用命令（考虑可用性要求和启用状态）
```

#### getSkillToolCommands

```typescript
export const getSkillToolCommands = memoize(
  async (cwd: string): Promise<Command[]>
)
// 获取模型可调用的技能命令
// 过滤条件: type='prompt', !disableModelInvocation, 有描述
```

#### getSlashCommandToolSkills

```typescript
export const getSlashCommandToolSkills = memoize(
  async (cwd: string): Promise<Command[]>
)
// 获取斜杠命令技能（供模型使用）
```

#### findCommand

```typescript
export function findCommand(
  commandName: string,
  commands: Command[],
): Command | undefined
// 按名称或别名查找命令
```

#### getCommand

```typescript
export function getCommand(
  commandName: string,
  commands: Command[],
): Command
// 获取命令，找不到则抛出错误
```

#### isBridgeSafeCommand

```typescript
export function isBridgeSafeCommand(cmd: Command): boolean
// 检查命令是否可安全通过桥接远程执行
```

#### filterCommandsForRemoteMode

```typescript
export function filterCommandsForRemoteMode(commands: Command[]): Command[]
// 过滤出远程模式安全的命令
```

#### meetsAvailabilityRequirement

```typescript
export function meetsAvailabilityRequirement(cmd: Command): boolean
// 检查命令的可用性要求是否满足
// 考虑: claude-ai 订阅者、Console API 用户等
```

#### getMcpSkillCommands

```typescript
export function getMcpSkillCommands(mcpCommands: readonly Command[]): readonly Command[]
// 获取 MCP 提供的技能命令
```

### 命令缓存管理

```typescript
export function clearCommandMemoizationCaches(): void
// 清除命令缓存（保留技能缓存）

export function clearCommandsCache(): void
// 清除所有命令和技能缓存
```

### 预定义命令集

#### REMOTE_SAFE_COMMANDS

在 `--remote` 模式下安全的命令：
- session, exit, clear, help, theme, color, vim, cost, usage
- copy, btw, feedback, plan, keybindings, statusline, stickers, mobile

#### BRIDGE_SAFE_COMMANDS

可安全通过桥接远程执行的本地命令：
- compact, clear, cost, summary, releaseNotes, files

#### INTERNAL_ONLY_COMMANDS

仅供内部使用的命令（ANT 构建）：
- backfillSessions, breakCache, bughunter, commit, ctx_viz
- mockLimits, bridgeKick, version, resetLimits, onboarding
- share, summary, teleport, antTrace, perfIssue, env, oauthRefresh
- debugToolCall, agentsPlatform, autofixPr 等

### 命令格式辅助

```typescript
export function formatDescriptionWithSource(cmd: Command): string
// 格式化命令描述，添加来源标注
// 示例: "搜索代码 (bundled)" 或 "(my-plugin) 描述"
```

### 主要内置命令

| 命令 | 功能 |
|------|------|
| addDir | 添加目录到上下文 |
| agents | 管理代理 |
| branch | Git 分支操作 |
| clear | 清除会话 |
| compact | 压缩上下文 |
| config | 配置管理 |
| diff | Git 差异查看 |
| doctor | 健康检查 |
| exit | 退出 |
| files | 文件列表 |
| help | 帮助信息 |
| mcp | MCP 管理 |
| memory | 记忆管理 |
| model | 模型选择 |
| permissions | 权限管理 |
| plan | 计划模式 |
| session | 会话管理 |
| skills | 技能管理 |
| stats | 统计信息 |
| status | 状态显示 |
| theme | 主题切换 |
| usage | 使用统计 |

### 设计特点

1. **延迟加载**: 动态导入昂贵的命令模块
2. **多来源聚合**: 统一处理多种命令来源
3. **可用性检查**: 基于用户认证状态过滤命令
4. **条件编译**: 使用 `feature()` 实现 Dead Code Elimination
5. **记忆化**: 使用 memoize 缓存命令加载结果

---

## tools.ts

**文件路径**: `src/tools.ts`

### 功能概述

tools.ts 负责工具的注册、过滤和组合。

### 工具预设

```typescript
export const TOOL_PRESETS = ['default'] as const
export type ToolPreset = (typeof TOOL_PRESETS)[number]

export function parseToolPreset(preset: string): ToolPreset | null
export function getToolsForDefaultPreset(): string[]
```

### 核心函数

#### getAllBaseTools

```typescript
export function getAllBaseTools(): Tools
// 获取所有基础工具（不考虑条件过滤）
// 这是所有工具的真实来源
```

**包含的工具**:

| 工具 | 条件 | 功能 |
|------|------|------|
| AgentTool | 始终 | 子代理执行 |
| TaskOutputTool | 始终 | 任务输出获取 |
| BashTool | 始终 | Shell 命令执行 |
| GlobTool | 无嵌入式搜索工具 | 文件模式匹配 |
| GrepTool | 无嵌入式搜索工具 | 内容搜索 |
| ExitPlanModeV2Tool | 始终 | 退出计划模式 |
| FileReadTool | 始终 | 文件读取 |
| FileEditTool | 始终 | 文件编辑 |
| FileWriteTool | 始终 | 文件写入 |
| NotebookEditTool | 始终 | Jupyter 编辑 |
| WebFetchTool | 始终 | 网页获取 |
| TodoWriteTool | 始终 | Todo 写入 |
| WebSearchTool | 始终 | 网络搜索 |
| TaskStopTool | 始终 | 停止任务 |
| AskUserQuestionTool | 始终 | 请求用户输入 |
| SkillTool | 始终 | 技能执行 |
| EnterPlanModeTool | 始终 | 进入计划模式 |
| TaskCreateTool | todoV2Enabled | 创建任务 |
| TaskGetTool | todoV2Enabled | 获取任务 |
| TaskUpdateTool | todoV2Enabled | 更新任务 |
| TaskListTool | todoV2Enabled | 列出任务 |
| LSPTool | ENABLE_LSP_TOOL | LSP 语言服务 |
| EnterWorktreeTool | worktreeModeEnabled | 进入工作树 |
| ExitWorktreeTool | worktreeModeEnabled | 退出工作树 |
| WorkflowTool | WORKFLOW_SCRIPTS | 工作流执行 |
| CronCreateTool | AGENT_TRIGGERS | 定时任务创建 |
| CronDeleteTool | AGENT_TRIGGERS | 定时任务删除 |
| CronListTool | AGENT_TRIGGERS | 定时任务列表 |
| RemoteTriggerTool | AGENT_TRIGGERS_REMOTE | 远程触发器 |
| MonitorTool | MONITOR_TOOL | 监控工具 |
| SendUserFileTool | KAIROS | 发送用户文件 |
| PushNotificationTool | KAIROS_PUSH_NOTIFICATION | 推送通知 |
| SubscribePRTool | KAIROS_GITHUB_WEBHOOKS | 订阅 PR |
| SnipTool | HISTORY_SNIP | 历史裁剪 |
| ListPeersTool | UDS_INBOX | 列出对等节点 |
| REPLTool | ANT 构建 | REPL 执行 |
| ConfigTool | ANT 构建 | 配置工具 |
| TungstenTool | ANT 构建 | Tungsten 工具 |
| ToolSearchTool | toolSearchEnabled | 工具搜索 |

#### getTools

```typescript
export function getTools(permissionContext: ToolPermissionContext): Tools
// 获取考虑权限过滤的工具
// 排除被拒绝规则匹配的工具
```

**特殊模式**:

1. **SIMPLE 模式** (`CLAUDE_CODE_SIMPLE`):
   - 仅包含 BashTool, FileReadTool, FileEditTool
   - 协调器模式下添加 AgentTool, TaskStopTool, SendMessageTool

2. **REPL 模式**: 隐藏原始工具，通过 REPL 访问

#### filterToolsByDenyRules

```typescript
export function filterToolsByDenyRules<T extends {...}>(
  tools: readonly T[],
  permissionContext: ToolPermissionContext
): T[]
// 根据拒绝规则过滤工具
// 包括 MCP 服务器前缀规则
```

#### assembleToolPool

```typescript
export function assembleToolPool(
  permissionContext: ToolPermissionContext,
  mcpTools: Tools,
): Tools
// 组合内置工具和 MCP 工具
// 去除重复名称，内置工具优先
```

#### getMergedTools

```typescript
export function getMergedTools(
  permissionContext: ToolPermissionContext,
  mcpTools: Tools,
): Tools
// 获取合并后的工具列表
```

### 导出常量

```typescript
export const ALL_AGENT_DISALLOWED_TOOLS
export const CUSTOM_AGENT_DISALLOWED_TOOLS
export const ASYNC_AGENT_ALLOWED_TOOLS
export const COORDINATOR_MODE_ALLOWED_TOOLS
export const REPL_ONLY_TOOLS
```

### 设计特点

1. **条件编译**: 使用 `feature()` 和环境变量控制工具可用性
2. **权限驱动**: 根据权限上下文过滤工具
3. **按名称去重**: 组合工具时内置工具优先
4. **REPL 封装**: REPL 模式隐藏原始工具，包装在 VM 中
5. **MCP 集成**: MCP 工具与内置工具统一管理

---

## types/command.ts

**文件路径**: `src/types/command.ts`

### 功能概述

定义所有命令类型的 TypeScript 接口。

### 命令类型

#### PromptCommand

```typescript
export type PromptCommand = {
  type: 'prompt'
  progressMessage: string                    // 进度消息
  contentLength: number                     // 内容长度（用于 token 估算）
  argNames?: string[]                       // 参数名称
  allowedTools?: string[]                   // 允许的工具列表
  model?: string                            // 指定模型
  source: SettingSource | 'builtin' | 'mcp' | 'plugin' | 'bundled'
  pluginInfo?: { pluginManifest: PluginManifest; repository: string }
  disableNonInteractive?: boolean
  hooks?: HooksSettings                     // 执行时注册的钩子
  skillRoot?: string                        // 技能资源根目录
  context?: 'inline' | 'fork'              // 执行上下文
  agent?: string                            // fork 时使用的代理类型
  effort?: EffortValue
  paths?: string[]                          // 适用的文件模式
  getPromptForCommand(args, context): Promise<ContentBlockParam[]>
}
```

#### LocalCommand

```typescript
export type LocalCommand = {
  type: 'local'
  supportsNonInteractive: boolean
  load: () => Promise<LocalCommandModule>
}

export type LocalCommandModule = {
  call: LocalCommandCall
}

export type LocalCommandCall = (
  args: string,
  context: LocalJSXCommandContext,
) => Promise<LocalCommandResult>

export type LocalCommandResult =
  | { type: 'text'; value: string }
  | { type: 'compact'; compactionResult: CompactionResult; displayText?: string }
  | { type: 'skip' }
```

#### LocalJSXCommand

```typescript
export type LocalJSXCommand = {
  type: 'local-jsx'
  load: () => Promise<LocalJSXCommandModule>
}

export type LocalJSXCommandModule = {
  call: LocalJSXCommandCall
}

export type LocalJSXCommandCall = (
  onDone: LocalJSXCommandOnDone,
  context: ToolUseContext & LocalJSXCommandContext,
  args: string,
) => Promise<React.ReactNode>

export type LocalJSXCommandOnDone = (
  result?: string,
  options?: {
    display?: CommandResultDisplay
    shouldQuery?: boolean
    metaMessages?: string[]
    nextInput?: string
    submitNextInput?: boolean
  },
) => void
```

#### CommandBase

```typescript
export type CommandBase = {
  availability?: CommandAvailability[]        // 可用性要求
  description: string                       // 描述
  hasUserSpecifiedDescription?: boolean
  isEnabled?: () => boolean                 // 是否启用
  isHidden?: boolean                        // 是否隐藏
  name: string                              // 命令名称
  aliases?: string[]                        // 别名
  isMcp?: boolean
  argumentHint?: string                     // 参数提示
  whenToUse?: string                        // 使用场景
  version?: string                          // 版本
  disableModelInvocation?: boolean           // 禁用模型调用
  userInvocable?: boolean                  // 用户可调用
  loadedFrom?: 'commands_DEPRECATED' | 'skills' | 'plugin' | 'managed' | 'bundled' | 'mcp'
  kind?: 'workflow'                         // 工作流标识
  immediate?: boolean                       // 立即执行
  isSensitive?: boolean                     // 敏感命令
  userFacingName?: () => string             // 显示名称
}

export type CommandAvailability =
  | 'claude-ai'    // claude.ai 订阅者
  | 'console'      // Console API 用户
```

#### Command 联合类型

```typescript
export type Command = CommandBase &
  (PromptCommand | LocalCommand | LocalJSXCommand)
```

### 辅助函数

```typescript
export function getCommandName(cmd: CommandBase): string
// 获取命令显示名称

export function isCommandEnabled(cmd: CommandBase): boolean
// 检查命令是否启用
```

### Context 类型

```typescript
export type LocalJSXCommandContext = ToolUseContext & {
  canUseTool?: CanUseToolFn
  setMessages: (updater: (prev: Message[]) => Message[]) => void
  options: {
    dynamicMcpConfig?: Record<string, ScopedMcpServerConfig>
    ideInstallationStatus: IDEExtensionInstallationStatus | null
    theme: ThemeName
  }
  onChangeAPIKey: () => void
  onChangeDynamicMcpConfig?: (config) => void
  onInstallIDEExtension?: (ide: IdeType) => void
  resume?: (sessionId: UUID, log: LogOption, entrypoint: ResumeEntrypoint) => Promise<void>
}

export type ResumeEntrypoint =
  | 'cli_flag'
  | 'slash_command_picker'
  | 'slash_command_session_id'
  | 'slash_command_title'
  | 'fork'
```

---

## types/message.ts

**文件路径**: `src/types/message.ts`

### 功能概述

定义所有消息类型的 TypeScript 接口。

### 消息类型

#### AssistantMessage

```typescript
export interface AssistantMessage {
  type: 'assistant'
  uuid: UUID
  timestamp: string
  message: BetaMessage              // 来自 SDK
  requestId?: string
  isMeta?: true
  isVirtual?: true
  isApiErrorMessage?: boolean
  apiError?: string
  error?: unknown
  errorDetails?: string
  advisorModel?: string
  agentId?: string                 // 代理 ID
  caller?: string                   // 调用者信息
}
```

#### UserMessage

```typescript
export interface UserMessage {
  type: 'user'
  message: {
    role: 'user'
    content: string | ContentBlockParam[]
  }
  uuid: UUID
  timestamp: string
  isMeta?: true                     // 元消息（模型可见但隐藏）
  isVisibleInTranscriptOnly?: true
  isVirtual?: true
  isCompactSummary?: true           // 压缩摘要
  toolUseResult?: unknown
  mcpMeta?: {
    _meta?: Record<string, unknown>
    structuredContent?: Record<string, unknown>
  }
  imagePasteIds?: number[]
  sourceToolAssistantUUID?: UUID
  permissionMode?: PermissionMode
  summarizeMetadata?: {
    messagesSummarized: number
    userContext?: string
    direction?: PartialCompactDirection
  }
  origin?: MessageOrigin
}

export type MessageOrigin =
  | 'agent'
  | 'teammate'
  | 'command'
  | 'system'
  | 'hook'
  | undefined
```

#### SystemMessage

```typescript
interface SystemMessageBase {
  type: 'system'
  uuid: UUID
  timestamp: string
  isMeta?: boolean
  content?: string
  level?: SystemMessageLevel
  toolUseID?: string
}

// 子类型联合
export type SystemMessage =
  | SystemInformationalMessage     // 'informational'
  | SystemAPIErrorMessage          // 'api_error'
  | SystemLocalCommandMessage      // 'local_command'
  | SystemStopHookSummaryMessage   // 'stop_hook_summary'
  | SystemBridgeStatusMessage      // 'bridge_status'
  | SystemTurnDurationMessage      // 'turn_duration'
  | SystemThinkingMessage          // 'thinking'
  | SystemMemorySavedMessage       // 'memory_saved'
  | SystemAwaySummaryMessage       // 'away_summary'
  | SystemAgentsKilledMessage      // 'agents_killed'
  | SystemCompactBoundaryMessage   // 'compact_boundary'
  | SystemMicrocompactBoundaryMessage  // 'microcompact_boundary'
  | SystemPermissionRetryMessage   // 'permission_retry'
  | SystemScheduledTaskFireMessage // 'scheduled_task_fire'
  | SystemApiMetricsMessage        // 'api_metrics'
```

#### AttachmentMessage

```typescript
export interface AttachmentMessage<A extends Record<string, unknown> = Record<string, unknown>> {
  type: 'attachment'
  attachment: A & { type: string }
  uuid: UUID
  timestamp: string
  isMeta?: true
}
```

#### ProgressMessage

```typescript
export interface ProgressMessage<P extends Progress = Progress> {
  type: 'progress'
  data: P
  toolUseID: string
  parentToolUseID: string
  uuid: UUID
  timestamp: string
}
```

#### TombstoneMessage

```typescript
export interface TombstoneMessage {
  type: 'tombstone'
  originalType: 'assistant' | 'user' | 'system'
  uuid: UUID
  timestamp: string
}
// 用于消息删除的墓碑标记
```

#### ToolUseSummaryMessage

```typescript
export interface ToolUseSummaryMessage {
  type: 'tool_use_summary'
  summary: string
  precedingToolUseIds: string[]
  uuid: UUID
  timestamp: string
}
```

### 联合类型

```typescript
export type Message =
  | AssistantMessage
  | UserMessage
  | SystemMessage
  | AttachmentMessage
  | ProgressMessage
  | TombstoneMessage

export type RenderableMessage =
  | AssistantMessage
  | UserMessage
  | SystemMessage
  | AttachmentMessage
  | GroupedToolUseMessage
  | CollapsedReadSearchGroup

export type NormalizedMessage =
  | NormalizedAssistantMessage  // 单内容块
  | NormalizedUserMessage       // content 始终为数组
  | SystemMessage
  | AttachmentMessage
  | ProgressMessage
  | TombstoneMessage
```

### 辅助类型

```typescript
export type SystemMessageLevel = 'info' | 'warning' | 'error'
export type PartialCompactDirection = 'earlier' | 'later'

export interface StopHookInfo {
  hookName: string
  executionTime?: number
  success: boolean
  error?: string
}

export interface RequestStartEvent {
  type: 'stream_request_start'
}

export interface StreamEvent {
  type: 'stream_event'
  event: BetaRawMessageStreamEvent
  ttftMs?: number
}
```

---

## AppStateStore.ts

**文件路径**: `src/state/AppStateStore.ts`

### 功能概述

AppStateStore.ts 定义了应用状态类型和默认值。

### 核心类型

#### AppState

```typescript
export type AppState = DeepImmutable<{
  // 设置和 UI
  settings: SettingsJson
  verbose: boolean
  mainLoopModel: ModelSetting
  mainLoopModelForSession: ModelSetting
  statusLineText: string | undefined
  expandedView: 'none' | 'tasks' | 'teammates'
  isBriefOnly: boolean

  // 代理和团队
  agentDefinitions: AgentDefinitionsResult
  agentNameRegistry: Map<string, AgentId>    // 名称→ID 注册表
  foregroundedTaskId?: string                // 前景任务 ID
  viewingAgentTaskId?: string                // 查看的代理任务 ID

  // 权限
  toolPermissionContext: ToolPermissionContext

  // 任务状态
  tasks: { [taskId: string]: TaskState }

  // MCP
  mcp: {
    clients: MCPServerConnection[]
    tools: Tool[]
    commands: Command[]
    resources: Record<string, ServerResource[]>
    pluginReconnectKey: number
  }

  // 插件
  plugins: {
    enabled: LoadedPlugin[]
    disabled: LoadedPlugin[]
    commands: Command[]
    errors: PluginError[]
    installationStatus: {...}
    needsRefresh: boolean
  }

  // 文件历史和归属
  fileHistory: FileHistoryState
  attribution: AttributionState

  // Todo
  todos: { [agentId: string]: TodoList }

  // 通知
  notifications: {
    current: Notification | null
    queue: Notification[]
  }
  elicitation: { queue: ElicitationRequestEvent[] }

  // 提示建议
  promptSuggestion: {
    text: string | null
    promptId: 'user_intent' | 'stated_intent' | null
    shownAt: number
    acceptedAt: number
    generationRequestId: string | null
  }

  // 推测执行
  speculation: SpeculationState
  speculationSessionTimeSavedMs: number

  // 技能改进
  skillImprovement: {
    suggestion: {
      skillName: string
      updates: { section, change, reason }[]
    } | null
  }

  // 认证版本
  authVersion: number

  // 初始消息
  initialMessage: {
    message: UserMessage
    clearContext?: boolean
    mode?: PermissionMode
    allowedPrompts?: AllowedPrompt[]
  } | null

  // 其他
  thinkingEnabled: boolean | undefined
  promptSuggestionEnabled: boolean
  sessionHooks: SessionHooksState
  denialTracking?: DenialTrackingState
  activeOverlays: ReadonlySet<string>
  fastMode?: boolean
  effortValue?: EffortValue
  advisorModel?: string

  // 桥接状态
  replBridgeEnabled: boolean
  replBridgeConnected: boolean
  replBridgeSessionActive: boolean
  replBridgeSessionUrl: string | undefined
  // ... 更多桥接相关字段

  // 团队上下文
  teamContext?: {
    teamName: string
    teamFilePath: string
    leadAgentId: string
    selfAgentId?: string
    selfAgentName?: string
    isLeader?: boolean
    teammates: {...}
  }

  // 独立代理上下文
  standaloneAgentContext?: { name: string; color?: AgentColorName }

  // 收件箱
  inbox: {
    messages: Array<{
      id: string
      from: string
      text: string
      timestamp: string
      status: 'pending' | 'processing' | 'processed'
    }>
  }
}>
```

#### SpeculationState

```typescript
export type SpeculationState =
  | { status: 'idle' }
  | {
      status: 'active'
      id: string
      abort: () => void
      startTime: number
      messagesRef: { current: Message[] }
      writtenPathsRef: { current: Set<string> }
      boundary: CompletionBoundary | null
      suggestionLength: number
      toolUseCount: number
      isPipelined: boolean
      pipelinedSuggestion?: {...}
    }

export type CompletionBoundary =
  | { type: 'complete'; completedAt: number; outputTokens: number }
  | { type: 'bash'; command: string; completedAt: number }
  | { type: 'edit'; toolName: string; filePath: string; completedAt: number }
  | { type: 'denied_tool'; toolName: string; detail: string; completedAt: number }
```

### 默认值

```typescript
export function getDefaultAppState(): AppState
// 返回默认状态对象
```

---

## AppState.tsx

**文件路径**: `src/state/AppState.tsx`

### 功能概述

AppState.tsx 提供 React 上下文和 hooks 用于访问全局状态。

### React Context

```typescript
export const AppStoreContext = React.createContext<AppStateStore | null>(null)
const HasAppStateContext = React.createContext<boolean>(false)
```

### Provider 组件

```typescript
export function AppStateProvider({
  children,
  initialState,
  onChangeAppState,
}: Props): React.ReactNode
```

**功能**:
- 创建 Store 实例
- 设置权限绕过检查
- 监听外部设置变更
- 提供多层上下文（Store、Mailbox、Voice）

### React Hooks

#### useAppState

```typescript
export function useAppState<T>(selector: (state: AppState) => T): T
// 订阅状态切片，仅在所选值变化时重渲染
```

**使用示例**:
```typescript
const verbose = useAppState(s => s.verbose)
const model = useAppState(s => s.mainLoopModel)
const { text, promptId } = useAppState(s => s.promptSuggestion)
```

#### useSetAppState

```typescript
export function useSetAppState(): (
  updater: (prev: AppState) => AppState
) => void
// 获取状态更新函数，不订阅任何状态
```

#### useAppStateStore

```typescript
export function useAppStateStore(): AppStateStore
// 获取 Store 直接引用
```

#### useAppStateMaybeOutsideOfProvider

```typescript
export function useAppStateMaybeOutsideOfProvider<T>(
  selector: (state: AppState) => T
): T | undefined
// 安全版本，Provider 外返回 undefined
```

### 设计特点

1. **useSyncExternalStore**: 使用 React 18 的 store 订阅机制
2. **Object.is 比较**: 优化重渲染，仅在所选值变化时触发
3. **多层上下文**: Store + Mailbox + Voice 分离关注点
4. **不可变性**: 状态通过 DeepImmutable 类型强制不可变
5. **外部同步**: 监听外部设置文件变化并同步

---

## 总结

Core 模块构成了 Claude Code 的核心基础设施：

| 模块 | 职责 | 关键概念 |
|------|------|----------|
| Tool | 工具接口定义 | 泛型设计、工厂模式、延迟加载 |
| Task | 任务管理 | 状态机、输出持久化、安全 ID |
| QueryEngine | 查询生命周期 | AsyncGenerator、权限追踪、预算管理 |
| commands | 命令系统 | 多来源聚合、条件编译、记忆化 |
| tools | 工具注册 | 权限过滤、REPL 封装、MCP 集成 |
| AppState | 全局状态 | 不可变性、订阅优化、外部同步 |
