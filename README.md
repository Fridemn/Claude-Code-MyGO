# Claude-Go

> **⚠️ Disclaimer**: This project is for learning and research purposes only. It does not represent the official internal development repository structure.

[English Version](README_EN.md) | 中文版

基于 Go 重构的个人版 Claude Code CLI。

![1](./assets/1.png)
![2](./assets/2.png)

## 说明

只要原仓库存在，本项目将持续同步更新。如有问题或疏漏请见谅，欢迎提交 Issue 或 PR。按原仓库标准持续移植，移除 Anthropic 专有依赖，改为通过 `.env` 配置兼容 OpenAI 风格接口。

## 移植进度概览

| 类别 | 原 TS 数量 | Go 已移植 | 完成率 |
|------|-----------|----------|--------|
| 核心架构 | 8 模块 | 8 模块 | 100% |
| 工具 (Tool) | 42 个 | 42 个 | 100% |
| 斜杠命令 (Command) | ~80 个 | ~30 个 | ~38% |
| 服务层 (Service) | 15+ 子系统 | 12 子系统 | ~80% |
| 扩展系统 | 5 子系统 | 5 子系统 | 100% |

## 核心架构 (已完成)

| 模块 | 状态 | 说明 |
|------|------|------|
| `cmd -> app -> engine -> provider -> session` | ✅ | 主调用链完整 |
| `config` | ✅ | `.env` 配置加载、多级设置合并 |
| `session` | ✅ | 会话历史持久化、恢复加载 |
| `engine` | ✅ | 消息循环、工具调用、流式响应 |
| `ui` | ✅ | Bubble Tea TUI、渲染、折叠、滚动回看 |
| `provider` | ✅ | OpenAI 兼容 LLM Provider |
| `agent` | ✅ | 子代理注册、定义、派生 |
| `types` | ✅ | 消息、权限、附件、Hook 等类型定义 |

## 工具移植 (Tool)

### 文件操作

| 原 TS 工具 | Go 实现 | 状态 |
|-----------|---------|------|
| FileReadTool | `internal/tool/file/read.go` | ✅ |
| FileWriteTool | `internal/tool/file/write.go` | ✅ |
| FileEditTool | `internal/tool/file/edit.go` | ✅ |
| GlobTool | `internal/tool/file` + `internal/tool/search` | ✅ |
| GrepTool | `internal/tool/search` | ✅ |
| NotebookEditTool | `internal/tool/notebook` | ✅ |

### 执行环境

| 原 TS 工具 | Go 实现 | 状态 |
|-----------|---------|------|
| BashTool | `internal/tool/bash` | ✅ |
| PowerShellTool | `internal/tool/bash` (含 powershell_*.go) | ✅ |

### 代理与协作

| 原 TS 工具 | Go 实现 | 状态 |
|-----------|---------|------|
| AgentTool | `internal/tool/agent` | ✅ |
| BriefTool | `internal/tool/agent` | ✅ |
| SendMessageTool | `internal/tool/agent` | ✅ |
| TeamCreateTool | `internal/tool/team` | ✅ |
| TeamDeleteTool | `internal/tool/team` | ✅ |

### 任务管理

| 原 TS 工具 | Go 实现 | 状态 |
|-----------|---------|------|
| TaskCreateTool | `internal/tool/task` | ✅ |
| TaskGetTool | `internal/tool/task` | ✅ |
| TaskListTool | `internal/tool/task` | ✅ |
| TaskOutputTool | `internal/tool/task` | ✅ |
| TaskStopTool | `internal/tool/task` | ✅ |
| TaskUpdateTool | `internal/tool/task` | ✅ |
| TodoWriteTool | `internal/tool/todo` | ✅ |

### 计划与工作树

| 原 TS 工具 | Go 实现 | 状态 |
|-----------|---------|------|
| EnterPlanModeTool | `internal/tool/plan` | ✅ |
| ExitPlanModeTool | `internal/tool/plan` | ✅ |
| EnterWorktreeTool | `internal/tool/worktree` | ✅ |
| ExitWorktreeTool | `internal/tool/worktree` | ✅ |

### 交互与 UI

| 原 TS 工具 | Go 实现 | 状态 |
|-----------|---------|------|
| AskUserQuestionTool | `internal/tool/interaction` | ✅ |
| SleepTool | `internal/tool/sleep` | ✅ |
| SyntheticOutputTool | `internal/tool/output` | ✅ |
| REPLTool | `internal/tool/repl` | ✅ |

### 扩展与集成

| 原 TS 工具 | Go 实现 | 状态 |
|-----------|---------|------|
| MCPTool | `internal/tool/mcp` | ✅ |
| ListMcpResourcesTool | `internal/tool/mcp` | ✅ |
| ReadMcpResourceTool | `internal/tool/mcp` | ✅ |
| McpAuthTool | `internal/tool/mcp` | ✅ |
| LSPTool | `internal/tool/lsp` | ✅ |
| ConfigTool | `internal/tool/config` | ✅ |
| SkillTool | `internal/tool/skill` | ✅ |
| ScheduleCronTool | `internal/tool/schedule` | ✅ |
| RemoteTriggerTool | `internal/tool/schedule` | ✅ |

### 搜索与网络

| 原 TS 工具 | Go 实现 | 状态 |
|-----------|---------|------|
| WebFetchTool | `internal/tool/web` | ✅ |
| WebSearchTool | `internal/tool/web` | ✅ |
| ToolSearchTool | `internal/tool/search` | ✅ |
| ImageProcessor | `internal/tool/image` | ✅ |

## 斜杠命令 (Slash Commands)

### 已完成

| 原 TS 命令 | Go 实现 | 说明 |
|-----------|---------|------|
| `/help` | `internal/command/help` | 帮助 |
| `/files [pattern]` | `internal/command/files` | 文件浏览 |
| `/grep` | `internal/command/files/grep.go` | 内容搜索 |
| `/read` | `internal/command/files/read.go` | 文件读取 |
| `/memory` | `internal/command/memory` | 记忆管理 |
| `/mcp` | `internal/command/integration/mcp.go` | MCP 服务器管理 |
| `/plugins` | `internal/command/integration/plugins.go` | 插件管理 |
| `/hooks` | `internal/command/integration/hooks.go` | Hooks 管理 |
| `/agents` | `internal/command/agent` | 子代理管理 |
| `/skills` | `internal/command/skills` | 技能管理 |
| `/session` | `internal/command/session` | 会话管理 |
| `/compact` | `internal/command/meta` | 上下文压缩 |
| `/doctor` | `internal/command/dev` | 诊断检查 |
| `/diff` | `internal/command/dev` | 差异显示 |
| `/usage` | `internal/command/stats/usage.go` | 使用统计 |
| `/stats` | `internal/command/stats` | 统计面板 |
| `/stats/effort` | `internal/command/stats/effort.go` | 效力控制 |
| `/stats/status` | `internal/command/stats/status.go` | 状态显示 |
| `/stats/tools` | `internal/command/stats/tools.go` | 工具统计 |
| `/model` | `internal/command/model` | 模型切换 |
| `/config` | `internal/command/config` | 配置管理 |
| `/btw` | `internal/command/btw` | 快速备注 |
| `/context` | `internal/command/context` | 上下文管理 |
| `/fast` | `internal/command/fast` | 快速模式切换 |
| `/ide` | `internal/command/ide` | IDE 集成 |
| `/sandbox` | `internal/command/sandbox` | 沙箱管理 |
| `/prompt/commit` | `internal/command/prompt/commit.go` | 提交提示词 |
| `/prompt/review` | `internal/command/prompt/review.go` | 代码审查提示词 |
| `/prompt/insights` | `internal/command/prompt/insights.go` | 使用洞察提示词 |
| `/prompt/pr-comments` | `internal/command/prompt/pr_comments.go` | PR 评论提示词 |
| `/prompt/security-review` | `internal/command/prompt/security_review.go` | 安全审查提示词 |
| `/prompt/shell` | `internal/command/prompt/shell.go` | Shell 提示词 |

### 待移植

| 命令 | 说明 | 优先级 |
|------|------|--------|
| `/commit` | Git 提交 | 高 |
| `/review` | 代码审查 | 高 |
| `/init` | 项目初始化 | 高 |
| `/resume` | 恢复会话 | 高 |
| `/exit` | 退出 | 中 |
| `/clear` | 清屏 | 中 |
| `/copy` | 复制消息 | 中 |
| `/theme` | 主题选择 | 中 |
| `/color` | 代理颜色 | 中 |
| `/plan` | 计划模式 | 中 |
| `/permissions` | 权限管理 | 中 |
| `/branch` | Git 分支 | 中 |
| `/status` | 状态信息 | 中 |
| `/tasks` | 任务管理 | 中 |
| `/export` | 导出记录 | 中 |
| `/rewind` | 回退会话 | 中 |
| `/rename` | 重命名会话 | 低 |
| `/upgrade` | 升级检查 | 低 |
| `/feedback` | 反馈 | 低 |
| `/summary` | 会话摘要 | 低 |
| `/keybindings` | 快捷键设置 | 低 |
| `/advisor` | 顾问 | 低 |
| `/extra-usage` | 额外用量 | 低 |
| `/tag` | 标签 | 低 |
| `/output-style` | 输出风格 | 低 |
| `/env` | 环境变量 | 低 |
| `/release-notes` | 更新日志 | 低 |
| `/terminal-setup` | 终端设置 | 低 |
| `/passes` | Pass 管理 | 低 |
| `/privacy-settings` | 隐私设置 | 低 |
| `/statusline` | 状态栏 | 低 |
| `/cost` | 费用追踪 | 低 |

### 不移植 (Anthropic 专有)

| 命令 | 原因 |
|------|------|
| `/login` | Anthropic OAuth 专有 |
| `/logout` | Anthropic OAuth 专有 |
| `/install-github-app` | Anthropic 平台集成 |
| `/install-slack-app` | Anthropic 平台集成 |
| `/oauth-refresh` | Anthropic OAuth 刷新 |
| `/mobile` | 移动端同步 |
| `/desktop` | 桌面应用集成 |
| `/remote-env` | 远程环境 |

## 服务层 (Services)

| 服务 | Go 实现 | 状态 | 说明 |
|------|---------|------|------|
| API 客户端 | `internal/api/` | ✅ | 适配器、流式、重试、Token 估算 |
| 会话管理 | `internal/session/` | ✅ | 创建、加载、持久化 |
| 记忆系统 | `internal/memory/` | ✅ | CLAUDE.md 解析、frontmatter、扫描 |
| 上下文压缩 | `internal/services/compact*.go` | ✅ | 自动/手动/微压缩 |
| Hooks 执行 | `internal/services/hooks*.go` | ✅ | 事件触发、配置 |
| 权限系统 | `internal/services/permissions.go` | ✅ | 规则匹配、交互 |
| 技能系统 | `internal/services/skills.go` | ✅ | 加载、发现、打包 |
| 插件系统 | `internal/services/plugins.go` | ✅ | 加载、内置插件 |
| MCP 管理 | `internal/services/mcp.go` + `internal/infra/mcp/` | ✅ | 连接、传输、工具动态注册 |
| LSP 集成 | `internal/services/lsp_*.go` + `internal/tool/lsp/` | ✅ | 客户端、诊断 |
| 设置管理 | `internal/settings/` | ✅ | 多源合并、权限规则 |
| 系统提示词 | `internal/prompt/` | ✅ | 提示词构建 |
| Token 估算 | `internal/services/token_estimation.go` | ✅ | 粗略 Token 计数 |
| 消息转换 | `internal/services/message_convert.go` | ✅ | API 格式转换 |
| 工具摘要 | `internal/services/tool_use_summary.go` + `grouping.go` | ✅ | 工具使用结果分组显示 |
| 提示建议 | `internal/services/prompt_suggestion.go` | ✅ | 输入建议 |
| 通知 | `internal/services/notifier.go` | ✅ | OS 级通知 |
| 记忆提取 | `internal/services/extract_memories.go` | ✅ | 自动记忆提取 |
| 状态管理 | `internal/state/` | ✅ | 应用状态 Store |
| 引导启动 | `internal/bootstrap/` | ✅ | 启动状态初始化 |
| JSON/Markdown 加载 | `internal/services/json_loader.go` + `markdown_loader.go` | ✅ | 配置/技能文件加载 |
| 会话记忆 | `internal/services/session_memory.go` | ✅ | 会话级记忆管理 |
| 启动耗时 | `internal/services/load_time.go` | ✅ | 加载时间追踪 |
| 文件读取桩 | `internal/services/file_read_stub.go` | ✅ | 文件状态缓存 |
| API 微压缩 | `internal/services/api_microcompact.go` | ✅ | API 级微压缩 |
| 诊断追踪 | `internal/services/load_time.go` | ✅ | 加载时间追踪 |

### 未移植服务

| 服务 | 原因 |
|------|------|
| `analytics` | Anthropic 分析平台专有 |
| `auth/oauth` | Anthropic OAuth 专有 |
| `rateLimitMessages` | 速率限制消息（依赖 Anthropic 计费） |
| `mockRateLimits` | 速率限制模拟（测试用） |
| `voiceStreamSTT` | 语音输入（依赖 Anthropic 服务） |
| `voiceKeyterms` | 语音关键词 |
| `preventSleep` | 防止系统休眠 |
| `diagnosticTracking` | 诊断追踪 |
| `autoDream` | 自动 Dream 模式 |
| `x402` | x402 支付协议 |
| `settingsSync` | 设置同步 |
| `teamMemorySync` | 团队记忆同步 |
| `remoteManagedSettings` | 远程托管设置 |
| `grove` | Grove 服务 |
| `vcr` | 录制/回放 |
| `SessionMemory/compact` | 会话记忆压缩 |

## 扩展系统

| 模块 | 状态 | 说明 |
|------|------|------|
| MCP | ✅ | 本地 JSON 配置、动态工具注册、多传输协议 (stdio/SSE/HTTP/WebSocket) |
| Plugins | ✅ | 本地 JSON 配置、动态命令、内置插件 |
| Hooks | ✅ | 本地 JSON 配置、PreToolUse/PostToolUse 事件 |
| Skills | ✅ | Markdown 技能文件、打包技能、目录发现 |
| Memory | ✅ | 持久化记忆、CLAUDE.md 解析、条件记忆 |

## UI 组件

| 组件 | Go 实现 | 说明 |
|------|---------|------|
| 聊天界面 | `internal/ui/` | Bubble Tea TUI 主模型 |
| Markdown 渲染 | `internal/ui/markdown_glamour.go` | Glamour 渲染器 |
| 消息渲染 | `internal/ui/messages/` | 消息显示组件 |
| 折叠显示 | `internal/ui/collapse/` | 工具结果折叠、分组 |
| 差异显示 | `internal/ui/diff/` | Diff 可视化 |
| 输入处理 | `internal/ui/input/` | 输入框、粘贴支持 |
| 状态栏 | `internal/ui/status/` | 底部状态 |
| Spinner | `internal/ui/spinner.go` | 加载动画 |
| 终端管理 | `internal/ui/terminal.go` | 终端大小、备用屏 |
| 主题 | `internal/ui/theme.go` | 配色方案 |
| 对话框 | `internal/ui/dialogs/` | 模型选择、权限确认、快速打开、全局搜索 |
| 组件库 | `internal/ui/components/` | 虚拟列表、模糊选择器、进度条、标签页等 |
| 屏幕管理 | `internal/ui/screen.go` | 全屏布局、最近活动加载 |
| Shell 输出 | `internal/ui/shell/` | 外部 Shell 输出 |

## 未移植功能

### Anthropic 专有 (不移植)

| 模块 | 原因 |
|------|------|
| `login/logout` | Anthropic OAuth 专有 |
| `bridge` | Anthropic 桌面桥接 |
| `remote-env` | Anthropic 远程环境 |
| `voice` | 语音输入依赖 Anthropic 服务 |
| `mobile` | 移动端同步 |
| `insights` | 使用统计 (Anthropic 分析平台) |
| `analytics` | 遥测和分析 |
| `x402` | x402 支付协议 |
| `grove` | Grove 服务集成 |
| `settingsSync` | 远程设置同步 |
| `daemon` | 守护进程模式 |
| `chrome` | Chrome 浏览器集成 |
| `computer-use-mcp` | 计算机使用 MCP |
| `ultraplan` | 高级计划模式 |

### 功能性待移植

| 模块 | 说明 |
|------|------|
| `vim` 模式 | 完整 Vim 键绑定系统 |
| `keybindings` | 自定义快捷键 |
| `rewind` | 会话回退功能 |
| `export` | 导出会话记录 |
| `share` | 分享会话 |
| `advisor` | 顾问模式 |
| `buddy` | Buddy 模式 |
| `thinkback` | Thinkback 模式 |
| `proactive` | 主动模式 |
| `workflows` | 工作流脚本 |
| `background tasks` | 后台任务 |
| `ForkSubagent` | 派生子代理 |

## 目录结构

```text
Claude-Go/
├── cmd/                      # CLI 入口
│   ├── root.go              # 根命令
│   ├── chat.go              # 聊天入口
│   ├── cli.go               # CLI 核心逻辑
│   ├── cli_interactive_env_test.go
│   ├── config.go            # 配置显示
│   ├── renderer.go          # 渲染器
│   ├── session_picker.go    # 会话选择器
│   └── test.go              # 测试入口
├── internal/
│   ├── agent/                # 子代理管理 (定义、注册、派生)
│   ├── api/                  # OpenAI 兼容 API 客户端 (适配器、流式、重试)
│   ├── app/                  # 应用层
│   ├── bootstrap/            # 启动状态
│   ├── bridge/               # 桥接客户端 (预留)
│   ├── cli/                  # CLI Runner (交互式循环、Termios)
│   ├── command/              # 斜杠命令
│   │   ├── agent/           # 代理管理命令
│   │   ├── btw/             # 快速备注
│   │   ├── config/          # 配置命令
│   │   ├── context/         # 上下文命令
│   │   ├── dev/             # 开发工具 (doctor/diff)
│   │   ├── files/           # 文件命令 (files/grep/read)
│   │   ├── help/            # 帮助
│   │   ├── integration/     # 集成 (MCP/Plugins/Hooks/权限)
│   │   ├── memory/          # 记忆管理
│   │   ├── meta/            # 元命令 (compact)
│   │   ├── model/           # 模型切换
│   │   ├── prompt/          # 提示词命令 (commit/review/insights等)
│   │   ├── session/         # 会话管理
│   │   ├── skills/          # 技能管理
│   │   ├── stats/           # 统计 (usage/effort/status/tools)
│   │   └── ...              # 其他命令
│   ├── components/           # TUI 聊天组件
│   ├── config/               # 配置加载 (API Profile)
│   ├── constants/            # 常量 (API 限制、消息、产品信息)
│   ├── engine/               # 消息引擎 (循环、重试)
│   ├── infra/                # 基础设施
│   │   └── mcp/             # MCP (管理器、传输协议)
│   ├── memory/               # 记忆系统 (CLAUDE.md、frontmatter)
│   ├── prompt/               # 系统提示词
│   ├── provider/             # LLM Provider 抽象
│   ├── query/                # 查询循环
│   ├── services/             # 服务容器 (压缩、Hook、权限、技能等)
│   ├── session/              # 会话管理 (创建、存储)
│   ├── settings/             # 设置管理 (多源合并)
│   ├── state/                # 应用状态
│   ├── task/                 # 任务追踪 (磁盘输出、Shell)
│   ├── tool/                 # 工具定义
│   │   ├── agent/           # Agent/Brief/SendMessage
│   │   ├── bash/            # Bash/PowerShell
│   │   ├── config/          # Config 工具
│   │   ├── file/            # Read/Write/Edit/Glob
│   │   ├── image/           # 图片处理
│   │   ├── interaction/     # AskUserQuestion
│   │   ├── lsp/             # LSP 工具
│   │   ├── mcp/             # MCP 动态工具
│   │   ├── notebook/        # Notebook 编辑
│   │   ├── output/          # 输出持久化
│   │   ├── plan/            # 计划模式
│   │   ├── repl/            # REPL 工具
│   │   ├── schedule/        # 定时任务
│   │   ├── search/          # 搜索工具 (Grep/Glob/ToolSearch)
│   │   ├── skill/           # 技能工具
│   │   ├── sleep/           # Sleep 工具
│   │   ├── task/            # 任务工具
│   │   ├── team/            # 团队工具
│   │   ├── todo/            # Todo 工具
│   │   ├── web/             # Web 搜索
│   │   └── worktree/        # Worktree 工具
│   ├── types/                # 类型定义 (消息、权限、附件、Hook)
│   ├── ui/                   # 终端 UI (Bubble Tea)
│   │   ├── collapse/        # 折叠显示
│   │   ├── components/      # 组件库
│   │   ├── dialogs/         # 对话框
│   │   ├── diff/            # Diff 显示
│   │   ├── input/           # 输入处理
│   │   ├── messages/        # 消息渲染
│   │   ├── paste/           # 粘贴支持
│   │   ├── shell/           # Shell 输出
│   │   └── status/          # 状态栏
│   └── utils/                # 工具函数
├── tests/                    # 测试文件
├── .env.example
├── main.go
├── build.go
├── Makefile
└── go.mod
```

## 配置

复制 `.env.example` 为 `.env`：

```env
# 必填
CLAUDE_CODE_API_KEY=your_api_key
CLAUDE_CODE_BASE_URL=https://api.openai.com/v1/chat/completions
CLAUDE_CODE_MODEL=gpt-4.1

# 可选
CLAUDE_CODE_MCP_CONFIG=.claude-go/mcp.json
CLAUDE_CODE_PLUGINS_CONFIG=.claude-go/plugins.json
CLAUDE_CODE_HOOKS_CONFIG=.claude-go/hooks.json
CLAUDE_CODE_SESSION_DIR=.claude-go/sessions
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
| `CLAUDE_CODE_SESSION_DIR` | 会话保存目录 | `~/.claude-code-go/projects/<project>` |

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
| `/grep <pattern>` | 内容搜索 |
| `/read <file>` | 读取文件 |
| `/memory` | 管理记忆 |
| `/mcp` | MCP 服务器管理 |
| `/plugins` | 插件管理 |
| `/hooks` | Hooks 管理 |
| `/agents` | 子代理管理 |
| `/skills` | 技能管理 |
| `/session` | 会话管理 |
| `/compact` | 压缩上下文 |
| `/model` | 切换模型 |
| `/config` | 配置管理 |
| `/btw` | 快速备注 |
| `/context` | 上下文管理 |
| `/fast` | 快速模式 |
| `/ide` | IDE 集成 |
| `/sandbox` | 沙箱管理 |
| `/usage` | 使用统计 |
| `/stats` | 统计面板 |
| `/doctor` | 诊断检查 |
| `/diff` | 显示差异 |
| `/prompt/commit` | 提交提示词 |
| `/prompt/review` | 代码审查提示词 |
| `/prompt/insights` | 使用洞察 |
| `/prompt/pr-comments` | PR 评论审查 |
| `/prompt/security-review` | 安全审查 |

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
| 数据目录 | `~/.claude/` | `~/.claude-code-go/` |
| UI 框架 | React (Ink) | Bubble Tea |

## 开发

### 快速开始

```bash
# 构建当前平台
go run build.go

# 运行
./build/claude-go
```

### 跨平台编译

```bash
# 编译全部平台 (linux/darwin/windows x amd64/arm64)
go run build.go -action build-all -version 1.0.0

# 编译指定平台
go run build.go -os darwin -arch arm64
go run build.go -os windows -arch amd64

# 创建发布包 (含压缩包 + SHA256 校验和)
go run build.go -action release -version 1.0.0
```

输出目录：

| 平台 | 文件 |
|------|------|
| Linux amd64 | `dist/claude-go_linux_amd64` |
| Linux arm64 | `dist/claude-go_linux_arm64` |
| macOS Intel | `dist/claude-go_darwin_amd64` |
| macOS Apple Silicon | `dist/claude-go_darwin_arm64` |
| Windows amd64 | `dist/claude-go_windows_amd64.exe` |
| Windows arm64 | `dist/claude-go_windows_arm64.exe` |

### Make（可选）

```bash
make build                    # 当前平台
make build-all VERSION=1.0.0  # 全平台
make release VERSION=1.0.0    # 发布包
make test                     # 测试
make clean                    # 清理
make help                     # 查看所有目标
```

### build.go 完整参数

```
go run build.go [OPTIONS]

  -action VALUE     build | build-all | release | clean | test | info
  -version VERSION  版本号 (默认: 0.1.0-alpha)
  -os OS            目标系统: linux, darwin, windows
  -arch ARCH        目标架构: amd64, arm64
  -skip-tests       跳过测试
  -help             显示帮助
```

### 版本信息

构建时自动注入版本号、Git commit、构建时间：

```bash
# 查看
go run build.go -action info

# 自定义版本
go run build.go -version 1.2.3
```
