# Tools 模块详细文档

## 目录

1. [概述](#1-概述)
2. [工具分类](#2-工具分类)
3. [工具详解](#3-工具详解)
4. [工具实现模式](#4-工具实现模式)
5. [共享模块](#5-共享模块)
6. [权限与安全](#6-权限与安全)
7. [进度报告](#7-进度报告)

---

## 1. 概述

Tools 模块是 Claude Code 的功能实现层，每个工具定义了 AI 模型可以执行的具体操作。

### 目录结构

```
src/tools/
├── AgentTool/               # 子代理执行
├── AskUserQuestionTool/     # 请求用户输入
├── BashTool/                # Shell 命令执行
├── BriefTool/               # 简要信息生成
├── ConfigTool/              # 配置管理
├── EnterPlanModeTool/       # 进入计划模式
├── EnterWorktreeTool/       # 进入工作树
├── ExitPlanModeTool/        # 退出计划模式
├── ExitWorktreeTool/        # 退出工作树
├── FileEditTool/            # 文件编辑
├── FileReadTool/            # 文件读取
├── FileWriteTool/           # 文件写入
├── GlobTool/                # 文件模式匹配
├── GrepTool/                # 内容搜索
├── ListMcpResourcesTool/    # 列出 MCP 资源
├── LSPTool/                 # LSP 语言服务
├── MCPTool/                 # MCP 协议工具
├── NotebookEditTool/        # Jupyter 编辑
├── PowerShellTool/          # PowerShell 执行
├── ReadMcpResourceTool/     # 读取 MCP 资源
├── RemoteTriggerTool/       # 远程触发器
├── REPLTool/                # REPL 执行环境
├── ScheduleCronTool/        # 定时任务
├── SendMessageTool/         # 发送消息
├── shared/                  # 共享模块
├── SkillTool/               # 技能执行
├── SleepTool/               # 睡眠等待
├── SyntheticOutputTool/     # 结构化输出
├── TaskCreateTool/          # 创建任务
├── TaskGetTool/             # 获取任务
├── TaskListTool/            # 列出任务
├── TaskOutputTool/          # 任务输出
├── TaskStopTool/            # 停止任务
├── TaskUpdateTool/          # 更新任务
├── TeamCreateTool/          # 创建团队
├── TeamDeleteTool/          # 删除团队
├── testing/                 # 测试工具
├── TodoWriteTool/           # Todo 写入
├── ToolSearchTool/          # 工具搜索
├── WebFetchTool/            # 网页获取
├── WebSearchTool/           # 网络搜索
└── utils.ts                 # 工具函数
```

### 工具文件组织

每个工具目录通常包含：

```
ToolName/
├── ToolName.ts[x]          # 工具实现
├── prompt.ts               # 提示生成
├── UI.tsx                  # UI 渲染组件
├── [submodules].ts         # 子模块（如权限、验证等）
└── constants.ts            # 常量定义
```

---

## 2. 工具分类

### 2.1 文件操作工具

| 工具 | 功能 | 是否只读 | 是否破坏性 |
|------|------|---------|-----------|
| FileReadTool | 读取文件内容 | ✅ | ❌ |
| FileWriteTool | 创建/覆盖文件 | ❌ | ✅ |
| FileEditTool | 编辑文件 | ❌ | ⚠️ |
| GlobTool | 文件模式匹配 | ✅ | ❌ |

### 2.2 Shell 执行工具

| 工具 | 功能 | 平台 |
|------|------|------|
| BashTool | Shell 命令执行 | Unix/Linux/Mac |
| PowerShellTool | PowerShell 执行 | Windows |

### 2.3 搜索工具

| 工具 | 功能 |
|------|------|
| GrepTool | 正则表达式搜索文件内容 |
| GlobTool | 文件名模式匹配 |

### 2.4 代理和任务工具

| 工具 | 功能 |
|------|------|
| AgentTool | 启动子代理 |
| TaskCreateTool | 创建任务 |
| TaskGetTool | 获取任务详情 |
| TaskListTool | 列出任务 |
| TaskUpdateTool | 更新任务状态 |
| TaskStopTool | 停止任务 |
| TaskOutputTool | 获取任务输出 |

### 2.5 模式切换工具

| 工具 | 功能 |
|------|------|
| EnterPlanModeTool | 进入计划模式 |
| ExitPlanModeTool | 退出计划模式 |
| EnterWorktreeTool | 进入 Git 工作树 |
| ExitWorktreeTool | 退出工作树 |

### 2.6 MCP 工具

| 工具 | 功能 |
|------|------|
| MCPTool | 执行 MCP 服务器提供的工具 |
| ListMcpResourcesTool | 列出 MCP 资源 |
| ReadMcpResourceTool | 读取 MCP 资源 |

### 2.7 通信工具

| 工具 | 功能 |
|------|------|
| AskUserQuestionTool | 请求用户输入 |
| SendMessageTool | 发送消息给代理 |
| SkillTool | 执行技能 |

### 2.8 网络工具

| 工具 | 功能 |
|------|------|
| WebFetchTool | 获取网页内容 |
| WebSearchTool | 网络搜索 |

### 2.9 其他工具

| 工具 | 功能 |
|------|------|
| NotebookEditTool | 编辑 Jupyter Notebook |
| LSPTool | LSP 语言服务 |
| TodoWriteTool | Todo 列表管理 |
| ConfigTool | 配置管理 |
| BriefTool | 简要信息生成 |
| ToolSearchTool | 搜索可用工具 |
| SleepTool | 睡眠等待 |
| SyntheticOutputTool | 结构化输出 |

---

## 3. 工具详解

### 3.1 BashTool

**目录**: `src/tools/BashTool/`

**功能**: 执行 Shell 命令

**文件组成**:
| 文件 | 功能 |
|------|------|
| BashTool.tsx | 主实现 |
| bashPermissions.ts | 权限检查 |
| bashSecurity.ts | 安全验证 |
| pathValidation.ts | 路径验证 |
| readOnlyValidation.ts | 只读验证 |
| sedValidation.ts | sed 命令验证 |
| shouldUseSandbox.ts | 沙箱判断 |
| commandSemantics.ts | 命令语义分析 |
| prompt.ts | 提示生成 |
| UI.tsx | UI 渲染 |

**输入 Schema**:
```typescript
{
  command: string           // 要执行的命令
  description?: string      // 命令描述
  timeout?: number         // 超时时间（毫秒）
  run_in_background?: boolean  // 后台运行
}
```

**安全检查**:
- 路径验证：确保操作在允许的目录内
- 命令解析：检测破坏性命令
- 沙箱模式：危险命令使用沙箱执行
- 只读验证：检测只读命令

**命令分类**:
```typescript
// 搜索命令
const BASH_SEARCH_COMMANDS = ['find', 'grep', 'rg', 'ag', 'ack', 'locate']

// 读取命令
const BASH_READ_COMMANDS = ['cat', 'head', 'tail', 'less', 'more', 'wc', 'stat', 'jq', 'awk']

// 列表命令
const BASH_LIST_COMMANDS = ['ls', 'tree', 'du']

// 静默命令（成功时无输出）
const BASH_SILENT_COMMANDS = ['mv', 'cp', 'rm', 'mkdir', 'chmod', 'touch']
```

---

### 3.2 FileReadTool

**目录**: `src/tools/FileReadTool/`

**功能**: 读取文件内容，支持文本、图片、PDF

**文件组成**:
| 文件 | 功能 |
|------|------|
| FileReadTool.ts | 主实现 |
| imageProcessor.ts | 图片处理 |
| limits.ts | 读取限制 |
| prompt.ts | 提示生成 |
| UI.tsx | UI 渲染 |

**输入 Schema**:
```typescript
{
  file_path: string       // 文件绝对路径
  offset?: number        // 起始行号
  limit?: number         // 读取行数
  pages?: string         // PDF 页码范围（如 "1-5, 10"）
}
```

**特性**:
- 支持文本文件、图片、PDF
- 行号添加
- 分页读取
- 读取限制（token 数、文件大小）
- 设备文件过滤

**设备文件黑名单**:
```typescript
const BLOCKED_DEVICE_PATHS = [
  '/dev/zero',    // 无限输出
  '/dev/random',  // 阻塞输入
  '/dev/urandom', // 无限输出
  '/dev/full',    // 阻塞
]
```

---

### 3.3 FileEditTool

**目录**: `src/tools/FileEditTool/`

**功能**: 精确编辑文件（字符串替换）

**输入 Schema**:
```typescript
{
  file_path: string       // 文件路径
  old_string: string     // 要替换的字符串
  new_string: string     // 替换后的字符串
  replace_all?: boolean  // 替换所有匹配
}
```

**特性**:
- 精确字符串匹配
- 单次或全部替换
- 破坏性操作警告
- 文件编码检测

---

### 3.4 FileWriteTool

**目录**: `src/tools/FileWriteTool/`

**功能**: 创建或覆盖文件

**输入 Schema**:
```typescript
{
  file_path: string    // 文件路径
  content: string     // 文件内容
}
```

---

### 3.5 AgentTool

**目录**: `src/tools/AgentTool/`

**功能**: 启动子代理执行任务

**文件组成**:
| 文件 | 功能 |
|------|------|
| AgentTool.tsx | 主实现 |
| agentColorManager.ts | 代理颜色管理 |
| agentDisplay.ts | 代理显示 |
| agentMemory.ts | 代理记忆 |
| forkSubagent.ts | Fork 子代理 |
| loadAgentsDir.ts | 加载代理定义 |
| runAgent.ts | 运行代理 |
| prompt.ts | 提示生成 |
| built-in/ | 内置代理定义 |
| constants.ts | 常量定义 |
| UI.tsx | UI 渲染 |

**输入 Schema**:
```typescript
{
  description: string           // 任务描述
  prompt: string               // 任务内容
  subagent_type?: string        // 代理类型
  model?: 'sonnet' | 'opus' | 'haiku'  // 模型选择
  run_in_background?: boolean   // 后台运行
  name?: string                 // 代理名称（用于消息路由）
  team_name?: string            // 团队名称
  mode?: PermissionMode         // 权限模式
  isolation?: 'worktree' | 'remote'  // 隔离模式
  cwd?: string                  // 工作目录
}
```

**内置代理**:
- `general-purpose`: 通用代理
- `explore`: 探索代理
- `plan`: 计划代理
- `verification`: 验证代理
- `claude-code-guide`: Claude Code 指导代理

---

### 3.6 GlobTool

**目录**: `src/tools/GlobTool/`

**功能**: 文件名模式匹配

**输入 Schema**:
```typescript
{
  pattern: string    // Glob 模式（如 "**/*.ts"）
  path?: string      // 搜索路径
}
```

---

### 3.7 GrepTool

**目录**: `src/tools/GrepTool/`

**功能**: 正则表达式搜索文件内容

**输入 Schema**:
```typescript
{
  pattern: string                    // 正则表达式
  path?: string                      // 搜索路径
  output_mode?: 'content' | 'files_with_matches' | 'count'
  type?: string                      // 文件类型
  glob?: string                      // Glob 过滤
  context?: number                   // 上下文行数
  head_limit?: number                // 结果限制
  -i?: boolean                       // 忽略大小写
}
```

---

### 3.8 AskUserQuestionTool

**目录**: `src/tools/AskUserQuestionTool/`

**功能**: 请求用户输入

**输入 Schema**:
```typescript
{
  questions: Array<{
    question: string      // 问题文本
    header: string        // 标题标签
    options: Array<{      // 选项
      label: string
      description?: string
      preview?: string    // 预览内容
    }>
    multiSelect?: boolean // 多选
  }>
}
```

---

### 3.9 WebFetchTool

**目录**: `src/tools/WebFetchTool/`

**功能**: 获取网页内容

**输入 Schema**:
```typescript
{
  url: string         // URL 地址
  prompt: string      // 内容处理提示
}
```

---

### 3.10 WebSearchTool

**目录**: `src/tools/WebSearchTool/`

**功能**: 网络搜索

**输入 Schema**:
```typescript
{
  query: string       // 搜索查询
  allowed_domains?: string[]   // 限制域名
  blocked_domains?: string[]   // 屏蔽域名
}
```

---

### 3.11 MCPTool

**目录**: `src/tools/MCPTool/`

**功能**: 执行 MCP 服务器提供的工具

**特性**:
- 动态加载 MCP 工具
- 权限委托
- 结果折叠显示

---

### 3.12 SkillTool

**目录**: `src/tools/SkillTool/`

**功能**: 执行技能

**输入 Schema**:
```typescript
{
  name: string        // 技能名称
  args?: string       // 参数
}
```

---

### 3.13 TodoWriteTool

**目录**: `src/tools/TodoWriteTool/`

**功能**: 管理 Todo 列表

**输入 Schema**:
```typescript
{
  todos: Array<{
    content: string         // Todo 内容
    status: 'pending' | 'in_progress' | 'completed'
    priority?: 'high' | 'medium' | 'low'
  }>
}
```

---

### 3.14 NotebookEditTool

**目录**: `src/tools/NotebookEditTool/`

**功能**: 编辑 Jupyter Notebook

**输入 Schema**:
```typescript
{
  notebook_path: string     // Notebook 路径
  new_source: string        // 新内容
  cell_id?: string          // Cell ID
  cell_type?: 'code' | 'markdown'
  edit_mode?: 'replace' | 'insert' | 'delete'
}
```

---

### 3.15 EnterPlanModeTool / ExitPlanModeTool

**目录**: `src/tools/EnterPlanModeTool/`, `src/tools/ExitPlanModeTool/`

**功能**: 进入/退出计划模式

**计划模式特性**:
- 模型先规划再执行
- 用户审批计划
- 按步骤执行

---

### 3.16 EnterWorktreeTool / ExitWorktreeTool

**目录**: `src/tools/EnterWorktreeTool/`, `src/tools/ExitWorktreeTool/`

**功能**: 创建/退出 Git 工作树隔离环境

**用途**: 隔离代理工作目录，避免冲突

---

## 4. 工具实现模式

### 4.1 基本工具定义

```typescript
import { buildTool, type ToolDef } from '../../Tool.js'
import { z } from 'zod/v4'

const inputSchema = z.object({
  param: z.string().describe('Description'),
})

const myToolDef: ToolDef<typeof inputSchema, MyOutput> = {
  name: 'my_tool',
  inputSchema,
  maxResultSizeChars: 50000,
  
  async call(input, context, canUseTool, parentMessage, onProgress) {
    // 实现逻辑
    return { data: result }
  },
  
  async description(input, options) {
    return `Doing something with ${input.param}`
  },
  
  async prompt(options) {
    return 'Tool description for the model'
  },
  
  isReadOnly: () => false,
  isConcurrencySafe: () => false,
  
  renderToolUseMessage(input, options) {
    return <Text>Doing: {input.param}</Text>
  },
  
  renderToolResultMessage(content, progressMessages, options) {
    return <Text>Result: {content}</Text>
  },
}

export const MyTool = buildTool(myToolDef)
```

### 4.2 延迟 Schema

```typescript
import { lazySchema } from '../../utils/lazySchema.js'

const inputSchema = lazySchema(() => z.object({
  // 延迟定义，减少启动时间
}))
```

### 4.3 权限检查

```typescript
async checkPermissions(input, context) {
  // 自定义权限逻辑
  const hasAccess = await checkAccess(input.path)
  if (!hasAccess) {
    return { behavior: 'deny', message: 'Access denied' }
  }
  return { behavior: 'allow', updatedInput: input }
}
```

### 4.4 输入验证

```typescript
async validateInput(input, context) {
  if (!isValid(input)) {
    return {
      result: false,
      message: 'Invalid input',
      errorCode: 400,
    }
  }
  return { result: true }
}
```

### 4.5 进度报告

```typescript
async call(input, context, canUseTool, parentMessage, onProgress) {
  // 报告进度
  onProgress?.({
    toolUseID: 'xxx',
    data: { type: 'progress', message: 'Processing...' },
  })
  
  // 继续执行
}
```

---

## 5. 共享模块

### 5.1 gitOperationTracking.ts

**功能**: Git 操作追踪

**函数**:
```typescript
export function detectGitOperation(
  command: string,
  output: string,
): {
  commit?: { sha: string; kind: CommitKind }
  push?: { branch: string }
  pr?: { number: number; url?: string; action: PrAction }
}

export function trackGitOperations(
  command: string,
  exitCode: number,
  stdout?: string,
): void
```

**追踪的操作**:
- git commit
- git push
- git merge
- git rebase
- gh pr create/merge/close

### 5.2 spawnMultiAgent.ts

**功能**: 多代理创建共享逻辑

**函数**:
```typescript
export function spawnTeammate(
  options: SpawnOptions,
): Promise<SpawnResult>

export function resolveTeammateModel(
  inputModel: string | undefined,
  leaderModel: string | null,
): string
```

---

## 6. 权限与安全

### 6.1 权限检查流程

```
工具调用请求
    ↓
validateInput()        // 输入验证
    ↓
checkPermissions()    // 权限检查
    ↓
┌─────────────────┐
│ 权限决策        │
│ - allow         │ → 执行
│ - deny          │ → 拒绝
│ - ask           │ → 请求确认
└─────────────────┘
```

### 6.2 安全分类

**只读操作** (`isReadOnly`):
- FileReadTool
- GlobTool
- GrepTool (无输出时)

**破坏性操作** (`isDestructive`):
- FileWriteTool (覆盖文件)
- BashTool (rm, mv 等)

**并发安全** (`isConcurrencySafe`):
- 大多数工具不是并发安全的

### 6.3 沙箱执行

BashTool 支持沙箱模式：

```typescript
export function shouldUseSandbox(command: string): boolean {
  // 检测危险命令
  if (DANGEROUS_COMMANDS.some(cmd => command.includes(cmd))) {
    return true
  }
  return false
}
```

---

## 7. 进度报告

### 7.1 进度类型

```typescript
export type ToolProgressData =
  | BashProgress           // Shell 执行进度
  | AgentToolProgress      // 代理进度
  | MCPProgress            // MCP 进度
  | REPLToolProgress       // REPL 进度
  | SkillToolProgress      // 技能进度
  | TaskOutputProgress     // 任务输出进度
  | WebSearchProgress      // 搜索进度
```

### 7.2 进度消息格式

```typescript
export type ToolProgress<P extends ToolProgressData> = {
  toolUseID: string
  data: P
}

export type ToolCallProgress<P> = (
  progress: ToolProgress<P>
) => void
```

### 7.3 使用示例

```typescript
async call(input, context, canUseTool, parentMessage, onProgress) {
  for (const item of items) {
    // 处理每个项目
    process(item)
    
    // 报告进度
    onProgress?.({
      toolUseID: 'xxx',
      data: {
        type: 'progress',
        processed: items.indexOf(item) + 1,
        total: items.length,
      },
    })
  }
}
```

---

## 总结

Tools 模块采用统一的 Tool 接口设计，具有以下特点：

1. **类型安全**: 使用 Zod schema 验证输入输出
2. **权限控制**: 多层权限检查机制
3. **进度报告**: 支持实时进度回调
4. **UI 渲染**: 自定义 React 渲染方法
5. **安全隔离**: 沙箱模式和破坏性操作检测
6. **扩展性**: 工厂函数简化工具创建
7. **共享逻辑**: 提取公共功能到 shared 目录