# Claude-Code-Go

> **⚠️ Disclaimer**: This project is for learning and research purposes only. It does not represent the official internal development repository structure.

[English Version](README_EN.md) | 中文版

基于 Go 重构的个人版 Claude Code CLI。

![1](./assets/1.png)
![2](./assets/2.png)

## 说明

只要原仓库存在，本项目将持续同步更新。如有问题或疏漏请见谅，欢迎提交 Issue 或 PR。按原仓库标准持续移植，移除 Anthropic 专有依赖，改为通过 `.env` 配置兼容 OpenAI 风格接口。

## 移植进度

### 核心架构 (已完成)

| 模块 | 状态 | 说明 |
|------|------|------|
| `cmd -> app -> engine -> provider -> session` | ✅ | 主调用链完整 |
| `config` | ✅ | `.env` 配置加载 |
| `session` | ✅ | 会话历史持久化 |
| `engine` | ✅ | 消息循环、工具调用、流式响应 |
| `ui` | ✅ | Bubble Tea TUI、渲染、折叠、滚动 |

### 工具移植 (Tool)

| 原 TS 工具 | Go 实现 | 状态 |
|-----------|---------|------|
| BashTool | `internal/tool/bash` | ✅ |
| PowerShellTool | `internal/tool/bash` | ✅ |
| FileReadTool | `internal/tool/file` | ✅ |
| FileWriteTool | `internal/tool/file` | ✅ |
| FileEditTool | `internal/tool/file` | ✅ |
| GlobTool | `internal/tool/file` + `internal/tool/search` | ✅ |
| GrepTool | `internal/tool/search` | ✅ |
| AgentTool | `internal/tool/agent` | ✅ |
| BriefTool | `internal/tool/agent` | ✅ |
| SendMessageTool | `internal/tool/agent` | ✅ |
| TaskCreateTool | `internal/tool/task` | ✅ |
| TaskGetTool | `internal/tool/task` | ✅ |
| TaskListTool | `internal/tool/task` | ✅ |
| TaskOutputTool | `internal/tool/task` | ✅ |
| TaskStopTool | `internal/tool/task` | ✅ |
| TaskUpdateTool | `internal/tool/task` | ✅ |
| TodoWriteTool | `internal/tool/todo` | ✅ |
| EnterPlanModeTool | `internal/tool/plan` | ✅ |
| ExitPlanModeTool | `internal/tool/plan` | ✅ |
| EnterWorktreeTool | `internal/tool/worktree` | ✅ |
| ExitWorktreeTool | `internal/tool/worktree` | ✅ |
| AskUserQuestionTool | `internal/tool/interaction` | ✅ |
| MCPTool | `internal/tool/mcp` | ✅ |
| ListMcpResourcesTool | `internal/tool/mcp` | ✅ |
| ReadMcpResourceTool | `internal/tool/mcp` | ✅ |
| McpAuthTool | `internal/tool/mcp` | ✅ |
| LSPTool | `internal/tool/lsp` | ✅ |
| NotebookEditTool | `internal/tool/notebook` | ✅ |
| ConfigTool | `internal/tool/config` | ✅ |
| SkillTool | `internal/tool/skill` | ✅ |
| TeamCreateTool | `internal/tool/team` | ✅ |
| TeamDeleteTool | `internal/tool/team` | ✅ |
| SleepTool | `internal/tool/sleep` | ✅ |
| SyntheticOutputTool | `internal/tool/output` | ✅ |
| REPLTool | `internal/tool/repl` | ✅ |
| ScheduleCronTool | `internal/tool/schedule` | ✅ |
| RemoteTriggerTool | `internal/tool/schedule` | ✅ |
| WebFetchTool | `internal/tool/web` | ✅ |
| WebSearchTool | `internal/tool/web` | ✅ |
| ToolSearchTool | `internal/tool/search` | ✅ |

### 斜杠命令 (Slash Commands)

| 原 TS 命令 | Go 实现 | 状态 |
|-----------|---------|------|
| `/help` | `internal/command/help` | ✅ |
| `/files` | `internal/command/files` | ✅ |
| `/memory` | `internal/command/memory` | ✅ |
| `/mcp` | `internal/command/integration` | ✅ |
| `/plugins` | `internal/command/integration` | ✅ |
| `/hooks` | `internal/command/integration` | ✅ |
| `/agents` | `internal/command/agent` | ✅ |
| `/skills` | `internal/command/skills` | ✅ |
| `/session` | `internal/command/session` | ✅ |
| `/compact` | `internal/command/meta` | ✅ |
| `/prompt` | `internal/command/prompt` | ✅ |
| `/doctor` | `internal/command/dev` | ✅ |
| `/diff` | `internal/command/dev` | ✅ |
| `/usage` | `internal/command/stats` | ✅ |
| `/stats` | `internal/command/stats` | ✅ |
| `/login` | - | ❌ (Anthropic 专有) |
| `/logout` | - | ❌ (Anthropic 专有) |
| `/cost` | - | 🔄 待移植 |
| `/model` | - | 🔄 待移植 |
| `/config` | - | 🔄 待移植 |

### 扩展系统

| 模块 | 状态 | 说明 |
|------|------|------|
| MCP | ✅ | 本地 JSON 配置、动态工具 |
| Plugins | ✅ | 本地 JSON 配置、动态命令 |
| Hooks | ✅ | 本地 JSON 配置、事件触发 |
| Skills | ✅ | 本地 Markdown 文件 |
| Memory | ✅ | 持久化记忆系统 |

### 未移植功能

| 模块 | 原因 |
|------|------|
| `login/logout` | Anthropic OAuth 专有 |
| `bridge` | Anthropic 桌面桥接 |
| `remote-env` | Anthropic 远程环境 |
| `voice` | 语音输入依赖 Anthropic 服务 |
| `mobile` | 移动端同步 |
| `insights` | 使用统计 |
| `ultraplan` | 高级计划模式 |

## 目录结构

```text
Claude-Code-Go/
├── cmd/                      # CLI 入口
│   ├── root.go
│   ├── chat.go
│   ├── config.go
│   └── test.go
├── internal/
│   ├── api/                  # OpenAI 兼容 API 客户端
│   ├── app/                  # 应用层
│   ├── agent/                # 子代理管理
│   ├── bootstrap/            # 启动状态
│   ├── bridge/               # 桥接客户端 (预留)
│   ├── command/              # 斜杠命令
│   ├── components/           # TUI 组件
│   ├── config/               # 配置加载
│   ├── engine/               # 消息引擎
│   ├── infra/                # 基础设施
│   ├── memory/               # 记忆系统
│   ├── prompt/               # 系统提示词
│   ├── services/             # 服务容器
│   ├── session/              # 会话管理
│   ├── state/                # 状态管理
│   ├── task/                 # 任务追踪
│   ├── tool/                 # 工具定义
│   ├── types/                # 类型定义
│   ├── ui/                   # 终端 UI
│   └── utils/                # 工具函数
├── tests/                    # 测试文件
├── .env.example
└── main.go
```

## 配置

复制 `.env.example` 为 `.env`：

```env
# 必填
CLAUDE_CODE_API_KEY=your_api_key
CLAUDE_CODE_BASE_URL=https://api.openai.com/v1/chat/completions
CLAUDE_CODE_MODEL=gpt-4.1

# 可选
CLAUDE_CODE_MCP_CONFIG=.claude-code-go/mcp.json
CLAUDE_CODE_PLUGINS_CONFIG=.claude-code-go/plugins.json
CLAUDE_CODE_HOOKS_CONFIG=.claude-code-go/hooks.json
CLAUDE_CODE_SESSION_DIR=.claude-code-go/sessions
CLAUDE_CODE_SYSTEM_PROMPT=
```

### 配置说明

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `CLAUDE_CODE_API_KEY` | API 密钥 | - |
| `CLAUDE_CODE_BASE_URL` | 完整请求地址，不会自动拼接路径 | - |
| `CLAUDE_CODE_MODEL` | 模型名称 | `gpt-4.1` |
| `CLAUDE_CODE_MCP_CONFIG` | MCP 配置文件路径 | - |
| `CLAUDE_CODE_PLUGINS_CONFIG` | 插件配置文件路径 | - |
| `CLAUDE_CODE_HOOKS_CONFIG` | Hooks 配置文件路径 | - |
| `CLAUDE_CODE_SESSION_DIR` | 会话保存目录 | `.claude-code-go/sessions` |

### MCP 配置示例 (`mcp.json`)

```json
{
  "servers": [
    {
      "name": "filesystem",
      "command": "mcp-server-filesystem",
      "args": ["--root", "/path/to/project"],
      "enabled": true
    }
  ]
}
```

### Plugins 配置示例 (`plugins.json`)

```json
{
  "plugins": [
    {
      "name": "my-plugin",
      "path": "./plugins/my-plugin",
      "enabled": true
    }
  ]
}
```

### Hooks 配置示例 (`hooks.json`)

```json
{
  "hooks": [
    {
      "event": "PreToolUse",
      "command": "echo 'Tool about to be used'",
      "blocking": false
    }
  ]
}
```

## 用法

### 启动交互式会话

```bash
go run .
```

### 子命令

```bash
go run . chat      # 启动交互式聊天 (默认)
go run . config    # 显示当前配置
go run . test      # 运行测试
go run . version   # 显示版本
```

### 交互式斜杠命令

在聊天会话中：

| 命令 | 说明 |
|------|------|
| `/help` | 显示帮助 |
| `/files [pattern]` | 列出文件 |
| `/memory` | 管理记忆 |
| `/mcp` | MCP 服务器管理 |
| `/plugins` | 插件管理 |
| `/hooks` | Hooks 管理 |
| `/agents` | 子代理管理 |
| `/skills` | 技能管理 |
| `/session` | 会话管理 |
| `/compact` | 压缩上下文 |
| `/prompt` | 查看/编辑提示词 |
| `/doctor` | 诊断检查 |
| `/diff` | 显示差异 |
| `/usage` | 使用统计 |
| `/stats` | 统计信息 |

### Vim 模式快捷键

| 快捷键 | 说明 |
|--------|------|
| `i` | 进入插入模式 |
| `Esc` | 退出插入模式 |
| `k` | 上翻历史 |
| `j` | 下翻历史 |
| `Ctrl+C` | 中断当前操作 |
| `Ctrl+D` | 退出 |

## 测试

```bash
# 运行所有测试
go test ./tests/...

# 运行特定测试
go test ./tests/... -run TestBashTool

# 通过入口运行
go run . test
```

## 迁移原则

1. **行为优先**：保持与原 CLI 相同的用户体验
2. **模块对应**：保持 `engine / tool / command / session / config` 分层
3. **渐进移植**：先完成主链路，再补充高级功能
4. **移除专有**：去掉 `anthropic`、`oauth`、`bridge` 等依赖

## 与原版差异

| 特性 | 原 TS 版 | Go 版 |
|------|----------|-------|
| 认证 | Anthropic OAuth | API Key 直接配置 |
| API | Anthropic API | OpenAI 兼容 API |
| 模型 | Claude 系列 | 任意兼容模型 |
| 桌面集成 | 完整 | 无 |
| 远程环境 | 支持 | 无 |
| 语音输入 | 支持 | 无 |

## 开发

```bash
# 构建
go build -o claude-code-go .

# 运行
./claude-code-go

# 开发模式 (热重载需要额外工具)
go run . chat
```
