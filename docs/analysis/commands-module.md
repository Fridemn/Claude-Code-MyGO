# Commands 模块详细文档

## 目录

1. [概述](#1-概述)
2. [命令类型](#2-命令类型)
3. [命令分类](#3-命令分类)
4. [命令详解](#4-命令详解)
5. [命令实现模式](#5-命令实现模式)
6. [设计特点](#6-设计特点)

---

## 1. 概述

Commands 模块负责实现 Claude Code 的所有斜杠命令（Slash Commands）。命令通过 `/` 前缀在 REPL 中触发，提供丰富的交互功能。

### 命令文件结构

```
src/commands/
├── index.ts              # 入口文件，汇总所有命令
├── command-name/
│   ├── index.ts          # 命令元数据定义
│   └── command.js       # 命令实现（延迟加载）
├── command.jsx           # JSX 命令（直接实现）
└── command.ts          # 纯 TS 命令（直接实现）
```

### 命令元数据格式

```typescript
const myCommand = {
  type: 'local' | 'local-jsx' | 'prompt',
  name: 'command-name',
  description: 'Command description',
  aliases?: ['alias1', 'alias2'],     // 别名
  isEnabled?: () => boolean,           // 启用检查
  isHidden?: boolean,                  // 隐藏命令
  supportsNonInteractive?: boolean,    // 支持非交互模式
  argumentHint?: '<hint>',             // 参数提示
  immediate?: boolean,                 // 立即执行
  load: () => import('./implementation'), // 延迟加载
} satisfies Command
```

---

## 2. 命令类型

### 2.1 local 类型

本地执行命令，返回文本结果。

```typescript
const localCommand = {
  type: 'local',
  name: 'example',
  description: 'Description',
  supportsNonInteractive: true,
  load: () => import('./example.js'),
}
```

**实现文件** (`example.js`):
```typescript
import type { LocalCommandCall } from '../../types/command.js'

export const call: LocalCommandCall = async (args, context) => {
  return {
    type: 'text',
    value: 'Result output',
  }
  // 或返回压缩结果
  // return { type: 'compact', compactionResult: {...} }
  // 或跳过
  // return { type: 'skip' }
}
```

### 2.2 local-jsx 类型

JSX UI 命令，渲染 React/Ink 组件。

```typescript
const jsxCommand = {
  type: 'local-jsx',
  name: 'example',
  description: 'Description',
  load: () => import('./example.jsx'),
}
```

**实现文件** (`example.jsx`):
```typescript
import type { LocalJSXCommandCall } from '../../types/command.js'
import { Box, Text } from 'ink'
import React from 'react'

export const call: LocalJSXCommandCall = async (onDone, context, args) => {
  return (
    <Box>
      <Text>Command UI</Text>
    </Box>
  )
}
```

### 2.3 prompt 类型

提示类命令，生成内容发送给模型。

```typescript
const promptCommand = {
  type: 'prompt',
  name: 'example',
  description: 'Description',
  progressMessage: 'Processing...',
  contentLength: 1000,
  getPromptForCommand: async (args, context) => {
    return [{ type: 'text', text: 'Generated prompt content' }]
  },
}
```

---

## 3. 命令分类

### 3.1 会话管理

| 命令 | 描述 | 类型 |
|------|------|------|
| `/clear` | 清除会话历史 | local |
| `/compact` | 压缩上下文 | local |
| `/session` | 远程会话管理 | local-jsx |
| `/resume` | 恢复会话 | local-jsx |
| `/rewind` | 回退会话 | local-jsx |
| `/summary` | 生成会话摘要 | local-jsx |
| `/export` | 导出会话 | local-jsx |

### 3.2 配置管理

| 命令 | 描述 | 类型 |
|------|------|------|
| `/config` | 配置管理 | local-jsx |
| `/model` | 模型选择 | local-jsx |
| `/theme` | 主题切换 | local-jsx |
| `/color` | 代理颜色 | local-jsx |
| `/fast` | 快速模式 | local-jsx |
| `/output-style` | 输出样式 | local-jsx |
| `/privacy-settings` | 隐私设置 | local-jsx |
| `/permissions` | 权限管理 | local-jsx |

### 3.3 文件和代码

| 命令 | 描述 | 类型 |
|------|------|------|
| `/init` | 初始化项目 | prompt |
| `/diff` | Git 差异 | local-jsx |
| `/branch` | Git 分支 | local-jsx |
| `/commit` | Git 提交 | local-jsx |
| `/review` | 代码审查 | prompt |
| `/files` | 文件列表 | local-jsx |
| `/add-dir` | 添加目录 | local-jsx |

### 3.4 工具和扩展

| 命令 | 描述 | 类型 |
|------|------|------|
| `/mcp` | MCP 服务器管理 | local-jsx |
| `/skills` | 技能管理 | local-jsx |
| `/plugins` | 插件管理 | local-jsx |
| `/hooks` | Hook 管理 | local-jsx |
| `/reload-plugins` | 重新加载插件 | local |

### 3.5 开发工具

| 命令 | 描述 | 类型 |
|------|------|------|
| `/doctor` | 健康检查 | local |
| `/ide` | IDE 集成 | local-jsx |
| `/terminalSetup` | 终端设置 | local-jsx |
| `/keybindings` | 快捷键 | local-jsx |

### 3.6 任务和代理

| 命令 | 描述 | 类型 |
|------|------|------|
| `/tasks` | 任务管理 | local-jsx |
| `/agents` | 代理配置 | local-jsx |
| `/plan` | 计划模式 | local-jsx |
| `/passes` | 验证执行 | local-jsx |

### 3.7 统计和监控

| 命令 | 描述 | 类型 |
|------|------|------|
| `/cost` | 成本统计 | local |
| `/usage` | 使用统计 | local-jsx |
| `/stats` | 详细统计 | local-jsx |
| `/effort` | 工作量估算 | local-jsx |

### 3.8 远程和协作

| 命令 | 描述 | 类型 |
|------|------|------|
| `/remote-setup` | 远程设置 | local-jsx |
| `/teleport` | 远程跳转 | local-jsx |
| `/feedback` | 反馈 | local-jsx |
| `/share` | 分享会话 | local-jsx |
| `/mobile` | 移动端 | local-jsx |

### 3.9 记忆系统

| 命令 | 描述 | 类型 |
|------|------|------|
| `/memory` | 记忆管理 | local-jsx |
| `/ctx_viz` | 上下文可视化 | local-jsx |

### 3.10 实验性命令

| 命令 | 描述 | 类型 |
|------|------|------|
| `/thinkback` | 回放测试 | local-jsx |
| `/thinkback-play` | 回放执行 | local-jsx |
| `/btw` | 快速笔记 | local |
| `/sticker` | 贴纸 | local-jsx |
| `/upgrade` | 版本升级 | local-jsx |

---

## 4. 命令详解

### 4.1 /clear

清除会话历史，释放上下文空间。

```typescript
const clear = {
  type: 'local',
  name: 'clear',
  description: 'Clear conversation history and free up context',
  aliases: ['reset', 'new'],
  supportsNonInteractive: false,
  load: () => import('./clear.js'),
}
```

**功能**:
- 清除消息历史
- 重置文件状态缓存
- 触发 GC

---

### 4.2 /compact

压缩上下文，保留摘要。

```typescript
const compact = {
  type: 'local',
  name: 'compact',
  description: 'Clear conversation history but keep a summary in context',
  isEnabled: () => !isEnvTruthy(process.env.DISABLE_COMPACT),
  supportsNonInteractive: true,
  argumentHint: '<optional custom summarization instructions>',
  load: () => import('./compact.js'),
}
```

**功能**:
- 压缩消息历史
- 生成会话摘要
- 可选自定义摘要指令

---

### 4.3 /model

切换 AI 模型。

```typescript
const model = {
  type: 'local-jsx',
  name: 'model',
  description: 'Set the AI model for Claude Code',
  argumentHint: '[model]',
  immediate: shouldInferenceConfigCommandBeImmediate(),
  load: () => import('./model.js'),
}
```

**功能**:
- 显示可用模型列表
- 切换当前模型
- 显示当前模型

---

### 4.4 /mcp

管理 MCP (Model Context Protocol) 服务器。

```typescript
const mcp = {
  type: 'local-jsx',
  name: 'mcp',
  description: 'Manage MCP servers',
  immediate: true,
  argumentHint: '[enable|disable [server-name]]',
  load: () => import('./mcp.js'),
}
```

**功能**:
- 列出已配置的 MCP 服务器
- 启用/禁用服务器
- 添加新服务器
- 查看服务器状态

---

### 4.5 /config

配置管理界面。

```typescript
// src/commands/config/index.ts
const config = {
  type: 'local-jsx',
  name: 'config',
  description: 'Configure Claude Code settings',
  load: () => import('./config.js'),
}
```

**功能**:
- 编辑配置文件
- 查看当前配置
- 重置配置

---

### 4.6 /session

远程会话管理。

```typescript
const session = {
  type: 'local-jsx',
  name: 'session',
  aliases: ['remote'],
  description: 'Show remote session URL and QR code',
  isEnabled: () => getIsRemoteMode(),
  isHidden: () => !getIsRemoteMode(),
  load: () => import('./session.js'),
}
```

**功能**:
- 显示远程会话 URL
- 生成 QR 码
- 管理远程连接

---

### 4.7 /init

初始化项目，创建 CLAUDE.md。

```typescript
const init = {
  type: 'prompt',
  name: 'init',
  description: 'Initialize Claude Code for this project',
  progressMessage: 'Analyzing project...',
  contentLength: 5000,
  getPromptForCommand: async (args, context) => {
    // 生成初始化提示
  },
}
```

**功能**:
- 分析项目结构
- 创建 CLAUDE.md
- 可选创建 skills 和 hooks
- 交互式问答

---

### 4.8 /doctor

健康检查命令。

```typescript
const doctor = {
  type: 'local',
  name: 'doctor',
  description: 'Check Claude Code health and configuration',
  supportsNonInteractive: true,
  load: () => import('./doctor.js'),
}
```

**检查项**:
- API 连接
- 认证状态
- 配置文件
- 依赖项
- 环境变量

---

### 4.9 /cost

显示会话成本。

```typescript
const cost = {
  type: 'local',
  name: 'cost',
  description: 'Show the total cost and duration of the current session',
  get isHidden() {
    return isClaudeAISubscriber()
  },
  supportsNonInteractive: true,
  load: () => import('./cost.js'),
}
```

**显示信息**:
- 输入/输出 token 数量
- API 成本
- 会话时长
- 模型信息

---

### 4.10 /tasks

任务管理界面。

```typescript
const tasks = {
  type: 'local-jsx',
  name: 'tasks',
  description: 'Manage background tasks',
  load: () => import('./tasks.js'),
}
```

**功能**:
- 列出后台任务
- 查看任务状态
- 停止任务
- 查看任务输出

---

### 4.11 /agents

代理配置管理。

```typescript
const agents = {
  type: 'local-jsx',
  name: 'agents',
  description: 'Manage agent configurations',
  load: () => import('./agents.js'),
}
```

**功能**:
- 列出可用代理
- 配置代理参数
- 创建自定义代理

---

### 4.12 /skills

技能管理。

```typescript
const skills = {
  type: 'local-jsx',
  name: 'skills',
  description: 'Manage Claude Code skills',
  load: () => import('./skills.js'),
}
```

**功能**:
- 列出已安装技能
- 搜索技能市场
- 安装/卸载技能
- 查看技能详情

---

### 4.13 /review

代码审查。

```typescript
const review = {
  type: 'prompt',
  name: 'review',
  description: 'Review code changes',
  progressMessage: 'Reviewing code...',
  load: () => import('./review.js'),
}
```

---

### 4.14 /diff

Git 差异查看。

```typescript
const diff = {
  type: 'local-jsx',
  name: 'diff',
  description: 'Show git diff',
  argumentHint: '<files>',
  load: () => import('./diff.js'),
}
```

---

### 4.15 /theme

主题切换。

```typescript
const theme = {
  type: 'local-jsx',
  name: 'theme',
  description: 'Change the color theme',
  load: () => import('./theme.js'),
}
```

---

### 4.16 /privacy-settings

隐私设置。

```typescript
const privacySettings = {
  type: 'local-jsx',
  name: 'privacy-settings',
  description: 'Configure privacy settings',
  load: () => import('./privacy-settings.js'),
}
```

---

### 4.17 /permissions

权限管理。

```typescript
const permissions = {
  type: 'local-jsx',
  name: 'permissions',
  description: 'Manage tool permissions',
  load: () => import('./permissions.js'),
}
```

---

### 4.18 /plan

计划模式。

```typescript
const plan = {
  type: 'local-jsx',
  name: 'plan',
  description: 'Enter plan mode for detailed planning',
  load: () => import('./plan.js'),
}
```

---

### 4.19 /hooks

Hook 管理。

```typescript
const hooks = {
  type: 'local-jsx',
  name: 'hooks',
  description: 'Manage Claude Code hooks',
  load: () => import('./hooks.js'),
}
```

---

### 4.20 /ide

IDE 集成设置。

```typescript
const ide = {
  type: 'local-jsx',
  name: 'ide',
  description: 'Configure IDE integration',
  load: () => import('./ide.js'),
}
```

---

### 4.21 /usage

使用统计详情。

```typescript
const usage = {
  type: 'local-jsx',
  name: 'usage',
  description: 'Show detailed usage statistics',
  load: () => import('./usage.js'),
}
```

---

## 5. 命令实现模式

### 5.1 延迟加载模式

大多数命令使用延迟加载减少启动时间：

```typescript
// index.ts - 元数据
const command = {
  type: 'local',
  name: 'example',
  load: () => import('./example.js'),
}

// example.js - 实际实现
export const call = async (args, context) => {
  return { type: 'text', value: 'Result' }
}
```

### 5.2 动态描述

某些命令需要动态生成描述：

```typescript
const model = {
  type: 'local-jsx',
  name: 'model',
  get description() {
    return `Set the AI model (currently ${renderModelName(getMainLoopModel())})`
  },
}
```

### 5.3 条件启用

命令可基于环境或配置有条件启用：

```typescript
const session = {
  type: 'local-jsx',
  name: 'session',
  isEnabled: () => getIsRemoteMode(),
  get isHidden() {
    return !getIsRemoteMode()
  },
}
```

### 5.4 别名支持

命令支持多个别名：

```typescript
const clear = {
  type: 'local',
  name: 'clear',
  aliases: ['reset', 'new'],
}
```

### 5.5 立即执行

某些命令立即执行，不等待模型停止：

```typescript
const mcp = {
  type: 'local-jsx',
  name: 'mcp',
  immediate: true,
}
```

---

## 6. 设计特点

### 6.1 延迟加载

所有命令实现都是动态导入的，减少初始加载时间。

### 6.2 类型安全

使用 `satisfies Command` 确保命令元数据正确。

### 6.3 条件编译

使用 `feature()` 和环境变量实现条件编译：

```typescript
const voice = feature('VOICE_MODE')
  ? require('./commands/voice/index.js').default
  : null
```

### 6.4 统一接口

三种命令类型（local, local-jsx, prompt）有统一的元数据结构。

### 6.5 非交互模式支持

`supportsNonInteractive` 标志控制命令是否可在 headless 模式运行。

### 6.6 桥接安全

`isBridgeSafeCommand()` 和 `BRIDGE_SAFE_COMMANDS` 控制远程可用命令。

---

## 附录: 命令索引

### 按字母顺序

| 命令 | 类型 | 描述 |
|------|------|------|
| `/add-dir` | local-jsx | 添加目录 |
| `/agents` | local-jsx | 代理管理 |
| `/autofix-pr` | local | 自动修复 PR |
| `/backfill-sessions` | local | 填充会话 |
| `/branch` | local-jsx | 分支管理 |
| `/break-cache` | local | 清除缓存 |
| `/btw` | local | 快速笔记 |
| `/bughunter` | local | Bug 猎人 |
| `/chrome` | local-jsx | Chrome 集成 |
| `/clear` | local | 清除会话 |
| `/color` | local-jsx | 颜色设置 |
| `/compact` | local | 压缩上下文 |
| `/commit` | local-jsx | Git 提交 |
| `/config` | local-jsx | 配置管理 |
| `/context` | local-jsx | 上下文管理 |
| `/copy` | local | 复制内容 |
| `/cost` | local | 成本统计 |
| `/ctx_viz` | local | 上下文可视化 |
| `/debug-tool-call` | local | 调试工具调用 |
| `/desktop` | local-jsx | 桌面集成 |
| `/diff` | local-jsx | Git 差异 |
| `/doctor` | local | 健康检查 |
| `/effort` | local-jsx | 工作量估算 |
| `/env` | local | 环境变量 |
| `/exit` | local | 退出 |
| `/export` | local-jsx | 导出会话 |
| `/extra-usage` | local | 额外使用 |
| `/fast` | local-jsx | 快速模式 |
| `/feedback` | local-jsx | 反馈 |
| `/files` | local-jsx | 文件列表 |
| `/good-claude` | local | Claude 评价 |
| `/heapdump` | local | 堆转储 |
| `/help` | local-jsx | 帮助 |
| `/hooks` | local-jsx | Hook 管理 |
| `/ide` | local-jsx | IDE 设置 |
| `/init` | prompt | 初始化项目 |
| `/install-github-app` | local-jsx | 安装 GitHub App |
| `/install-slack-app` | local-jsx | 安装 Slack App |
| `/issue` | local-jsx | Issue 管理 |
| `/keybindings` | local-jsx | 快捷键 |
| `/login` | local-jsx | 登录 |
| `/logout` | local | 登出 |
| `/mcp` | local-jsx | MCP 管理 |
| `/memory` | local-jsx | 记忆管理 |
| `/mobile` | local-jsx | 移动端 |
| `/mock-limits` | local | 模拟限制 |
| `/model` | local-jsx | 模型选择 |
| `/oauth-refresh` | local | OAuth 刷新 |
| `/onboarding` | local | 入门引导 |
| `/output-style` | local-jsx | 输出样式 |
| `/passes` | local-jsx | 验证执行 |
| `/perf-issue` | local | 性能问题 |
| `/permissions` | local-jsx | 权限管理 |
| `/plan` | local-jsx | 计划模式 |
| `/plugin` | local-jsx | 插件管理 |
| `/pr_comments` | local-jsx | PR 评论 |
| `/privacy-settings` | local-jsx | 隐私设置 |
| `/rate-limit-options` | local-jsx | 速率限制 |
| `/release-notes` | local-jsx | 发布说明 |
| `/reload-plugins` | local | 重载插件 |
| `/remote-env` | local | 远程环境 |
| `/remote-setup` | local-jsx | 远程设置 |
| `/rename` | local-jsx | 重命名 |
| `/reset-limits` | local | 重置限制 |
| `/resume` | local-jsx | 恢复会话 |
| `/review` | prompt | 代码审查 |
| `/rewind` | local-jsx | 回退会话 |
| `/sandbox-toggle` | local | 沙箱切换 |
| `/security-review` | prompt | 安全审查 |
| `/session` | local-jsx | 会话管理 |
| `/share` | local-jsx | 分享会话 |
| `/skills` | local-jsx | 技能管理 |
| `/stats` | local-jsx | 统计信息 |
| `/status` | local-jsx | 状态显示 |
| `/stickers` | local-jsx | 贴纸 |
| `/summary` | local-jsx | 会话摘要 |
| `/tag` | local-jsx | 标签管理 |
| `/tasks` | local-jsx | 任务管理 |
| `/teleport` | local-jsx | 远程跳转 |
| `/terminalSetup` | local-jsx | 终端设置 |
| `/theme` | local-jsx | 主题切换 |
| `/thinkback` | local-jsx | 回放测试 |
| `/upgrade` | local-jsx | 版本升级 |
| `/usage` | local-jsx | 使用统计 |
| `/version` | local | 版本信息 |
| `/vim` | local-jsx | Vim 模式 |

---

## 总结

Commands 模块采用统一的元数据定义 + 延迟加载实现模式，支持三种命令类型（local, local-jsx, prompt），具有以下特点：

1. **延迟加载**: 减少启动时间
2. **类型安全**: TypeScript 严格类型检查
3. **条件编译**: 支持功能开关
4. **统一接口**: 三种类型统一管理
5. **桥接安全**: 远程模式安全控制
