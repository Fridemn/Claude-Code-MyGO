# Claude Code 模块文档索引

本文档是 Claude Code 源代码模块的完整文档索引。

## 文档目录

1. [核心模块文档](./core-modules.md)
2. [命令模块文档](./commands-module.md)
3. [工具模块文档](./tools-module.md)
4. [服务模块文档](./services-module.md)
5. [Bridge 模块文档](./bridge-module.md)
6. [Bootstrap 模块文档](./bootstrap-module.md)
7. [CLI 模块文档](./cli-module.md)
8. [Utils 模块文档](./utils-module.md)
9. [Components 模块文档](./components-module.md)
10. [其他核心模块文档](./other-modules.md)
11. [额外模块文档](./additional-modules.md)

---

## 模块总览

```
src/
├── core/           # 核心模块文档
│   ├── Tool.ts
│   ├── Task.ts
│   ├── QueryEngine.ts
│   └── commands.ts
├── commands/       # 命令目录 (80+ 命令)
├── tools/         # 工具实现
├── services/      # 服务层
├── bridge/        # 远程控制
├── bootstrap/     # 全局状态
├── cli/           # CLI 交互
├── utils/         # 工具函数 (300+)
├── components/    # UI 组件 (144)
├── assistant/     # 助手功能
├── buddy/         # 伙伴系统
├── state/         # 状态管理
├── hooks/         # React Hooks
├── types/         # 类型定义
├── constants/     # 常量定义
├── coordinator/   # 协调器
├── entrypoints/   # 入口点
├── context/       # 上下文
├── skills/        # Skills
├── plugins/       # 插件
├── server/        # 服务器
└── [其他]
```

---

## 快速索引

### 按功能查找

| 功能 | 文档 |
|------|------|
| 工具调用 | [tools-module.md](./tools-module.md) |
| 命令系统 | [commands-module.md](./commands-module.md) |
| API 调用 | [services-module.md](./services-module.md) |
| 远程控制 | [bridge-module.md](./bridge-module.md) |
| 会话状态 | [bootstrap-module.md](./bootstrap-module.md) |
| CLI I/O | [cli-module.md](./cli-module.md) |
| UI 组件 | [components-module.md](./components-module.md) |
| 工具函数 | [utils-module.md](./utils-module.md) |
| React 状态 | [state 模块](./other-modules.md#4-state-模块) |
| React Hooks | [hooks 模块](./other-modules.md#5-hooks-模块) |
| 类型定义 | [types 模块](./other-modules.md#2-types-模块) |
| 常量定义 | [constants 模块](./other-modules.md#3-constants-模块) |

### 按文件类型查找

| 类型 | 位置 |
|------|------|
| 核心类 | `src/Tool.ts`, `src/Task.ts`, `src/QueryEngine.ts` |
| 工具实现 | `src/tools/` |
| 命令 | `src/commands.ts`, `src/commands/` |
| 服务 | `src/services/` |
| Bridge | `src/bridge/` |
| 工具函数 | `src/utils/` |
| 组件 | `src/components/` |
| React Hooks | `src/hooks/` |

---

## 核心概念

### 1. 工具系统 (Tools)

Claude Code 的核心执行单元。每个工具：
- 继承 `Tool` 基类
- 实现 `call()` 方法
- 定义输入输出类型
- 支持权限检查

文档: [tools-module.md](./tools-module.md)

### 2. 命令系统 (Commands)

用户交互的主要方式：
- `/command` 格式
- 三种类型: `local`, `local-jsx`, `prompt`
- 支持参数和子命令

文档: [commands-module.md](./commands-module.md)

### 3. 查询引擎 (QueryEngine)

核心消息处理循环：
- 接收用户消息
- 调用 API
- 执行工具
- 返回结果

文档: [core-modules.md](./core-modules.md)

### 4. 服务层 (Services)

基础设施服务：
- API 客户端
- MCP 服务器
- Analytics
- 设置同步

文档: [services-module.md](./services-module.md)

### 5. 远程控制 (Bridge)

与 claude.ai 的连接：
- 环境注册
- 会话管理
- 消息传递
- 权限委托

文档: [bridge-module.md](./bridge-module.md)

---

## 类型系统

### 品牌类型

```typescript
// 防止 ID 混淆
type SessionId = string & { __brand: 'SessionId' }
type AgentId = string & { __brand: 'AgentId' }
```

### 消息类型

```typescript
type Message =
  | AssistantMessage
  | UserMessage
  | SystemMessage
  | ToolMessage
```

### 权限类型

```typescript
type PermissionMode =
  | 'acceptEdits'
  | 'limitTools'
  | 'bypassPermissions'
  | 'ask'
```

---

## 状态管理

### Bootstrap 状态

全局单例状态（`src/bootstrap/state.ts`）：
- 会话 ID
- 工作目录
- 成本统计
- 模型配置

### AppState

React Context 状态：
- UI 状态
- 消息列表
- 设置

### Hooks

自定义 React Hooks：
- `useAppState()`
- `useCanUseTool()`
- `useCommandQueue()`
- `useMergedCommands()`
- `useMergedTools()`

---

## 设计模式

### 1. 工具接口模式

```typescript
abstract class Tool<Input, Output, Progress = void> {
  abstract name: string
  abstract call(input: Input, context: ToolUseContext): Promise<ToolResult<Output, Progress>>
}
```

### 2. 命令注册模式

```typescript
// 命令定义
const commands: Command[] = [
  {
    name: 'example',
    description: 'Example command',
    handler: async () => {},
  }
]

// 注册
export function registerCommands() {
  for (const cmd of commands) {
    commandsMap.set(cmd.name, cmd)
  }
}
```

### 3. 依赖注入模式

```typescript
// 依赖注入
function createService(deps: ServiceDependencies): Service {
  return {
    method: () => deps.api.call()
  }
}
```

### 4. Feature Flag 模式

```typescript
// 编译时优化
if (feature('FEATURE_NAME')) {
  // 仅在启用时包含
}
```

---

## 关键常量

### Beta Headers

```typescript
const BETA_HEADERS = {
  PROMPT_CACHE: 'prompt-cache-1m-2025-08-07',
  AFK_MODE: 'afk-mode-2025-11-01',
}
```

### API Limits

```typescript
const API_LIMITS = {
  MAX_TOKENS: 8192,
  MAX_RETRIES: 3,
  TIMEOUT: 60_000,
}
```

---

## 贡献指南

### 添加新工具

1. 在 `src/tools/` 创建文件
2. 继承 `Tool` 基类
3. 实现必需方法
4. 注册到 `getAllBaseTools()`

### 添加新命令

1. 在 `src/commands/` 创建文件
2. 定义命令对象
3. 导出并注册
4. 更新文档

### 添加新服务

1. 在 `src/services/` 创建模块
2. 定义接口和实现
3. 注册到服务容器
4. 更新文档

---

## 文档维护

- 文档位置: `Claude-Code-Go/temp/docs/`
- 格式: Markdown
- 更新: 随代码变更同步更新
- 审核: PR 时检查文档准确性

---

## 附录

### 目录结构速查

```
Claude-Code-Go/temp/
├── docs/                  # 文档目录
│   ├── core-modules.md
│   ├── commands-module.md
│   ├── tools-module.md
│   ├── services-module.md
│   ├── bridge-module.md
│   ├── bootstrap-module.md
│   ├── cli-module.md
│   ├── utils-module.md
│   ├── components-module.md
│   ├── other-modules.md
│   └── README.md          # 本文件
└── README.md              # 项目说明
```

### 外部资源

- Claude Code 源码: `/home/fridemn/Projects/Claude-Code/src/`
- 工具目录: `src/tools/`
- 命令目录: `src/commands/`
- 服务目录: `src/services/`