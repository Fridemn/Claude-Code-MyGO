# 原始 `src` TUI 交互整理

本文只基于当前仓库里的原始 TypeScript `src/` 实现，不基于 Go 迁移版推测。重点回答：

1. TUI 消息如何进入渲染链路
2. 什么时候输出
3. 什么时候隐藏
4. 什么时候自动折叠/合并
5. `verbose` / transcript(`Ctrl+O`) / fullscreen 对行为的影响

---

## 1. 总体渲染链路

核心入口在 `src/components/Messages.tsx`。

- 先对消息做归一化：`normalizeMessages(messages).filter(isNotEmptyMessage)`。
  证据：`src/components/Messages.tsx:379`
- 再识别“最后一个 thinking block”和“最近一次 bash 输出”，这两者后面分别决定 thinking 隐藏策略和 shell 输出展开策略。
  证据：`src/components/Messages.tsx:381-441`
- 再做 UI 侧过滤、分组、折叠：
  - `applyGrouping(...)`
  - `collapseReadSearchGroups(...)`
  - `collapseTeammateShutdowns(...)`
  - `collapseHookSummaries(...)`
  - `collapseBackgroundBashNotifications(...)`
  证据：`src/components/Messages.tsx:517-521`
- 最终每条 `RenderableMessage` 交给 `MessageRow -> Message` 渲染。
  证据：`src/components/Messages.tsx:614-624`，`src/components/MessageRow.tsx:102-154`

可以把原始 TUI 理解成 4 层：

- 数据层：原始 message/progress/attachment/system
- 变换层：归一化、过滤、group、collapse
- 行层：`MessageRow` 判断是否激活、是否动画、是否显示 metadata
- 块层：`Message` / `AssistantToolUseMessage` / `UserToolResultMessage` / `CollapsedReadSearchContent` 等组件决定具体输出

---

## 2. 默认视图、Verbose、Transcript 的关系

### 2.1 默认视图

默认不是“完全展示原始消息”，而是经过一轮 UI 过滤和折叠后的结果。

- 非 transcript 模式下，会先做 `shouldShowUserMessage(...)` 过滤。
  证据：`src/components/Messages.tsx:499-504`
- 非 verbose 时，会启用 `applyGrouping` 和各类 collapse。
  证据：`src/components/Messages.tsx:517-521`

### 2.2 verbose 的作用

`verbose` 基本相当于“尽量不要帮我收起来，按原位展开更多细节”。

- `applyGrouping` 在 verbose 下直接跳过，保留原始 tool use 位置。
  证据：`src/utils/groupToolUses.ts:52-64`
- `collapseBackgroundBashNotifications` 在 verbose 下直接 passthrough。
  证据：`src/utils/collapseBackgroundBashNotifications.ts:39-46`
- `collapsed_read_search` 在 `Message` 中只要 `verbose || isTranscriptMode` 就走详细渲染。
  证据：`src/components/Message.tsx:336-343`

### 2.3 transcript(`Ctrl+O`) 的作用

transcript 模式比 verbose 更接近“历史全貌”。

- 注释直接写明：`Transcript mode (ctrl+o screen) is truly unfiltered.`
  证据：`src/components/Messages.tsx:505-514`
- `collapsed_read_search` 在 transcript 下也会强制按 verbose 详细展示。
  证据：`src/components/Message.tsx:338-343`
- thinking/redacted thinking 只有 transcript 或 verbose 才显示。
  证据：`src/components/Message.tsx:529-557`

结论：

- 默认视图：强调简洁
- verbose：强调当前屏更详细
- transcript：强调历史可追溯，最接近原始记录

---

## 3. 什么时候“输出”

这里的“输出”可以分成几类：assistant tool use、tool result、用户 bash 输出、collapsed summary、system/attachment 提示。

### 3.1 assistant 的 tool_use 何时输出

入口在 `AssistantToolUseMessage`。

- 工具存在、schema 能解析、`userFacingToolName !== ""`、`renderToolUseMessage(...) !== null` 时，才会渲染工具调用行。
  证据：`src/components/messages/AssistantToolUseMessage.tsx:61-92, 158-180`
- queued 状态显示黑点，非 queued 则显示 `ToolUseLoader`。
  证据：`src/components/messages/AssistantToolUseMessage.tsx:184-193`
- 未 resolved、未 queued 时，会显示 progress 区域；里面还会附带 `PreToolUse` hook 进度。
  证据：`src/components/messages/AssistantToolUseMessage.tsx:238-245, 328-354`
- queued 时还会额外显示工具自己的 queued message。
  证据：`src/components/messages/AssistantToolUseMessage.tsx:263-279`

### 3.2 user 的 tool_result 何时输出

入口在 `UserToolResultMessage`。

- 先通过 `tool_use_id` 回查对应工具；找不到就不渲染。
  证据：`src/components/messages/UserToolResultMessage/UserToolResultMessage.tsx:34-37`
- 根据内容分流：
  - cancel -> `UserToolCanceledMessage`
  - reject / interrupt -> `UserToolRejectMessage`
  - `is_error` -> `UserToolErrorMessage`
  - 其余 -> `UserToolSuccessMessage`
  证据：`src/components/messages/UserToolResultMessage/UserToolResultMessage.tsx:38-84`

### 3.3 成功结果何时真正展示

`UserToolSuccessMessage` 还有一层保护。

- `message.toolUseResult` 或 `tool` 缺失时，不渲染。
  证据：`src/components/messages/UserToolResultMessage/UserToolSuccessMessage.tsx:47-50`
- 如果 `outputSchema.safeParse(...)` 失败，也不渲染，避免恢复会话时损坏结果把 UI 弄崩。
  证据：`src/components/messages/UserToolResultMessage/UserToolSuccessMessage.tsx:56-61`
- 如果工具的 `renderToolResultMessage(...)` 返回 `null`，整条成功结果也不渲染。
  证据：`src/components/messages/UserToolResultMessage/UserToolSuccessMessage.tsx:63-78`

这意味着：不是所有 tool_result 都一定有可见 UI。

### 3.4 用户 `!` 命令的 bash 输出何时输出

这条链路不是普通 tool_result，而是 user text 分支：

- `UserTextMessage` 遇到 `<bash-stdout>` / `<bash-stderr>` 时转给 `UserBashOutputMessage`。
  证据：`src/components/messages/UserTextMessage.tsx:60-70`
- `UserBashOutputMessage` 解包 `<persisted-output>` 后，交给 `BashToolResultMessage`。
  证据：`src/components/messages/UserBashOutputMessage.tsx:12-15, 42-46`

### 3.5 collapsed summary 何时输出

当消息序列被 `collapseReadSearchGroups` 识别为一组可折叠操作时，原本多条 assistant/user/tool_result 会变成一条 `collapsed_read_search`。

- 入口：`collapseReadSearchGroups(...)`
  证据：`src/utils/collapseReadSearch.ts:762-949`
- 渲染器：`CollapsedReadSearchContent`
  证据：`src/components/Message.tsx:336-343`

---

## 4. 什么时候“隐藏”

这里的隐藏包括：

- 直接返回 `null`
- 默认视图不显示，只在 verbose/transcript 显示
- 被 summary 吸收，不单独出现

### 4.1 thinking / redacted thinking

- 非 transcript 且非 verbose 时，直接 `return null`。
  证据：`src/components/Message.tsx:529-545`
- thinking 在 transcript 下还会隐藏“不是最后一个”的旧 thinking。
  证据：
  - 最后一个 thinking 的判定：`src/components/Messages.tsx:391-419`
  - `hideInTranscript` 传入：`src/components/Message.tsx:547-557`

补充：

- 如果 streaming thinking 仍可见，就把 `lastThinkingBlockId` 设成一个不会命中的值 `streaming`，从而隐藏所有已完成 thinking。
  证据：`src/components/Messages.tsx:381-399`
- streaming thinking 结束后 30 秒内仍算可见。
  证据：`src/components/Messages.tsx:381-389`

### 4.2 compact boundary / microcompact boundary / snip marker

- fullscreen 下 `compact_boundary` 直接不显示。
  证据：`src/components/Message.tsx:234-237`
- `microcompact_boundary` 直接不显示。
  证据：`src/components/Message.tsx:247-249`
- `snip marker` 直接不显示。
  证据：`src/components/Message.tsx:250-280`

### 4.3 Assistant tool use 自己选择不显示

`AssistantToolUseMessage` 有几种“主动隐藏”条件。

- 工具找不到，直接不显示。
  证据：`src/components/messages/AssistantToolUseMessage.tsx:61-92`
- `isTransparentWrapper` 且当前 queued/resolved 时，直接不显示整行；只在真正运行中显示 progress 区。
  证据：`src/components/messages/AssistantToolUseMessage.tsx:123-157`
- `userFacingToolName === ""` 时不显示整行。
  证据：`src/components/messages/AssistantToolUseMessage.tsx:158-160`
- `renderToolUseMessage(...) === null` 时不显示整行。
  证据：`src/components/messages/AssistantToolUseMessage.tsx:161-180`

区别要注意：

- 返回 `null`：整条 tool row 消失
- 返回 `""`：保留工具标题，但不显示括号里的参数描述
  证据：`src/components/messages/AssistantToolUseMessage.tsx:208-215, 304-326`

### 4.4 user text 中一些“纯控制信息”被隐藏

`UserTextMessage` 里有很多短路隐藏：

- `NO_CONTENT_MESSAGE` -> 不显示
- `<tick>` -> 不显示
- `<local-command-caveat>` -> 不显示
  证据：`src/components/messages/UserTextMessage.tsx:39-59`

### 4.5 工具成功结果也可能不显示

如上所述，以下情况成功结果会消失：

- tool/result 缺失
- output schema 校验失败
- 工具 result renderer 返回 `null`
  证据：`src/components/messages/UserToolResultMessage/UserToolSuccessMessage.tsx:47-78`

### 4.6 collapsed group 的“默认隐藏，verbose 才看见”

有些操作会被吸收到 collapsed group，但默认不体现在 summary 文案里：

- REPL wrapper
- Snip
- ToolSearch
  证据：`src/utils/collapseReadSearch.ts:148-193`

其中注释明确写了：

- 它们不会增加 summary count
- 默认视图里被隐藏
- 但 verbose/transcript 下通过 `groupMessages` 迭代仍能看到
  证据：`src/utils/collapseReadSearch.ts:55-60, 177-179, 799-802`

### 4.7 null-rendering attachments 会在更早阶段被过滤

`Messages.tsx` 会先过滤掉那些 `AttachmentMessage` 本来就会 render `null` 的 attachment，避免它们占 transcript 配额或消息数。

证据：`src/components/Messages.tsx:499-504`

---

## 5. 自动折叠 / 自动合并规则

这部分是 TUI 行为最复杂的地方。

### 5.1 同一响应里的同类工具可先 group

`applyGrouping(...)` 的规则：

- 只有实现了 `renderGroupedToolUse` 的工具才参与分组
  证据：`src/utils/groupToolUses.ts:19-31`
- 按 `message.id + toolName` 分组，也就是同一个 assistant API 响应里的同类 tool use
  证据：`src/utils/groupToolUses.ts:67-80`
- 只有 2 条及以上才生成 `grouped_tool_use`
  证据：`src/utils/groupToolUses.ts:83-100`
- 对应的 user tool_result 会被一起挂到 grouped message 上
  证据：`src/utils/groupToolUses.ts:102-157`
- 如果某条 user message 里的 tool_result 全都属于 grouped tool use，那条 user message 会被跳过
  证据：`src/utils/groupToolUses.ts:163-176`
- verbose 下完全不做 group
  证据：`src/utils/groupToolUses.ts:52-64`

### 5.2 Read/Search/Bash/MCP/Memory 会再 collapse 成摘要

`collapseReadSearchGroups(...)` 把“连续的一串可折叠工具”合成一条 `collapsed_read_search`。

#### 5.2.1 哪些会被认为是 collapsible

`getToolSearchOrReadInfo(...)` 把以下操作标成 collapsible：

- 普通 search/read/list
- REPL wrapper
- memory file write/edit
- Snip / ToolSearch
- fullscreen 下非 search/read 的普通 Bash 命令
  证据：`src/utils/collapseReadSearch.ts:143-237`

#### 5.2.2 summary 会统计哪些类别

collapsed group 里会分别统计：

- `searchCount`
- `readCount`
- `listCount`
- `memorySearchCount`
- `memoryReadCount`
- `memoryWriteCount`
- `mcpCallCount`
- `bashCount`
- git 派生结果：commit/push/branch/pr
- `hookCount/hookTotalMs`
  证据：`src/utils/collapseReadSearch.ts:581-623, 697-750`

#### 5.2.3 哪些事件不会打断 group

group 进行中时，这些消息不会立刻打断，而是跳过或延后：

- assistant thinking / redacted thinking
- attachment
- system
  证据：`src/utils/collapseReadSearch.ts:384-400, 918-932`

更细一点：

- `PreToolUse` hook summary 会被吸收到当前 group
  证据：`src/utils/collapseReadSearch.ts:897-903`
- `relevant_memories` attachment 会被吸收到当前 group
  证据：`src/utils/collapseReadSearch.ts:904-917`
- 其他 skippable message 会先 defer，等 collapsed badge 输出后再跟上，目的是让 badge 保持在第一条 tool use 的位置
  证据：`src/utils/collapseReadSearch.ts:919-929`

#### 5.2.4 哪些会打断 group

- assistant 文本消息
- 非 collapsible tool_use
- 非 collapsible 的 user tool_result / 其他普通消息
  证据：`src/utils/collapseReadSearch.ts:331-367, 933-944`

#### 5.2.5 hint 是怎么来的

collapsed group 下方 `⎿` 的 hint 来源优先级大致是：

- `latestDisplayHint`
- 最后一次 search pattern
- 最后一次 read file path
- active REPL 的 progress 里当前 inner tool 输入
  证据：`src/components/messages/CollapsedReadSearchContent.tsx:191-218`

另外，hint 至少显示 700ms，避免“一闪而过看不清”。

证据：`src/components/messages/CollapsedReadSearchContent.tsx:26-29, 218`

#### 5.2.6 活跃 collapsed group 的定义

在 `MessageRow` 中：

- 当前消息是 collapsed group
- 且组内有 tool 还在 in-progress
- 或者当前整体 still loading 且后面已无内容

则该 group 被视为 active，用于控制动画、现在时文案、hint、progress suffix。

证据：`src/components/MessageRow.tsx:113-121`

### 5.3 collapsed group 默认只显示摘要，verbose/transcript 显示全明细

`CollapsedReadSearchContent`：

- verbose 模式：遍历组内每一个 tool use，逐条显示工具名、参数、成功结果摘要、hook 细节、relevant memories 内容
  证据：`src/components/messages/CollapsedReadSearchContent.tsx:220-257`
- 默认模式：只显示一行 summary，加一个 `CtrlOToExpand`
  证据：`src/components/messages/CollapsedReadSearchContent.tsx:260-267, 294-305`

### 5.4 背景 bash 完成通知会进一步合并

`collapseBackgroundBashNotifications(...)`：

- 只在 fullscreen 下启用
- 只在非 verbose 下启用
- 只折叠“成功完成”的后台 bash 通知
- failed/killed 不折叠
- agent/workflow/monitor 类通知不折叠
  证据：`src/utils/collapseBackgroundBashNotifications.ts:21-30, 33-46`

合并结果是合成一条：

- `"N background commands completed"`
  证据：`src/utils/collapseBackgroundBashNotifications.ts:61-75`

### 5.5 hook summary 会对同标签连续消息做合并

`collapseHookSummaries(...)`：

- 只处理 `hookLabel !== undefined` 的 `stop_hook_summary`
- 同标签连续消息会 merge
- `hookCount` 求和
- `hookInfos` / `hookErrors` 扁平合并
- `preventedContinuation` / `hasOutput` 取 any
- `totalDurationMs` 取 max，而不是 sum
  证据：`src/utils/collapseHookSummaries.ts:16-21, 29-50`

### 5.6 teammate shutdown 也会批量折叠

`collapseTeammateShutdowns(...)`：

- 只针对 `attachment.type=task_status`
- 且 `taskType=in_process_teammate`
- 且 `status=completed`
  证据：`src/utils/collapseTeammateShutdowns.ts:3-11`

连续多条会变成一个 `teammate_shutdown_batch`。

证据：`src/utils/collapseTeammateShutdowns.ts:14-47`

---

## 6. Bash 相关的 TUI 特例

### 6.1 tool_use 行里，命令默认会截断

`BashTool.renderToolUseMessage(...)`：

- 无 command -> `null`
- `sed -i` 风格编辑只显示文件路径
- 非 verbose 时：
  - fullscreen 下如果命令里带可提取的注释标签，优先显示标签
  - 否则最多 2 行、160 字符
  - 超出则加 `…`
- verbose 时显示完整命令
  证据：`src/tools/BashTool/UI.tsx:85-130`

### 6.2 tool_use 运行中显示 progress

- 没有 progress 数据时显示 `Running…`
- 有数据时显示 `ShellProgressMessage`
  证据：`src/tools/BashTool/UI.tsx:131-153`

### 6.3 queued 时显示 `Waiting…`

证据：`src/tools/BashTool/UI.tsx:154-157`

### 6.4 tool_result 的显示规则

`BashToolResultMessage`：

- `stdout` 有内容 -> 显示 stdout
- `stderr` 有内容 -> 以 error 色显示 stderr
- `sandbox_violations` 会先从 stderr 中剥离，不直接堆给用户
  证据：`src/tools/BashTool/BashToolResultMessage.tsx:24-39, 93-99, 121`
- `"Shell cwd was reset to ..."` 会从 stderr 中剥离后单独当 warning 风格提示
  证据：`src/tools/BashTool/BashToolResultMessage.tsx:45-64, 146-153`
- `isImage` 时，不渲染原始数据，只提示 `[Image data detected and sent to Claude]`
  证据：`src/tools/BashTool/BashToolResultMessage.tsx:100-109`
- stdout/stderr 都空时：
  - 后台任务：显示 `Running in the background`
  - 否则优先 `returnCodeInterpretation`
  - 再否则 `Done` 或 `(No output)`
  证据：`src/tools/BashTool/BashToolResultMessage.tsx:154-166`
- 如果有 `timeoutMs`，额外显示 `ShellTimeDisplay`
  证据：`src/tools/BashTool/BashToolResultMessage.tsx:167-174`

### 6.5 最近一次用户 `!` 输出会自动展开，旧输出默认截断

这是一个很关键的 TUI 细节。

- `Messages.tsx` 会从后往前找到“最近一条带 `<bash-stdout>` / `<bash-stderr>` 的 user message”，记下其 UUID。
  证据：`src/components/Messages.tsx:421-441`
- `Message.tsx` 在渲染 user message 时，如果这条就是最近一次 bash 输出，就包上 `ExpandShellOutputProvider`
  证据：`src/components/Message.tsx:192-223`
- `ExpandShellOutputContext` 注释明确写着：用于“auto-expand the most recent user ! command output”
  证据：`src/components/shell/ExpandShellOutputContext.tsx:5-12`
- `OutputLine` 里 `shouldShowFull = verbose || expandShellOutput`
  证据：`src/components/shell/OutputLine.tsx:59-74`

所以行为是：

- 最近一次用户 shell 输出：即使不 verbose，也按全文展示
- 更早的 shell 输出：默认按终端宽度截断
- 一旦进入 verbose/transcript：都展示完整内容

### 6.6 shell 输出的截断真正发生在 `OutputLine`

`OutputLine` 的规则：

- 会先尝试做 JSON pretty format，但超过 10k 长度就不格式化
  证据：`src/components/shell/OutputLine.tsx:32-39`
- `shouldShowFull` 为假时，通过 `renderTruncatedContent(...)` 按当前终端宽度截断
  证据：`src/components/shell/OutputLine.tsx:61-74`
- `shouldShowFull` 为真时直接全文显示
  证据：`src/components/shell/OutputLine.tsx:61-74`

---

## 7. collapsed summary 具体长什么样

`CollapsedReadSearchContent` 在非 verbose 下会根据 group 内容拼一行自然语言摘要：

- search: `Searching for N patterns` / `Searched for N patterns`
- read: `Reading N files` / `Read N files`
- list: `Listing N directories` / `Listed N directories`
- mcp: `Querying <server>` / `Queried <server>`
- bash: `Running N bash commands` / `Ran N bash commands`
- memory: `Recalled N memories` / `Writing N memories` 等
- git 结果会优先顶到前面，如 `committed abc123`, `created PR #42`
  证据：`src/components/messages/CollapsedReadSearchContent.tsx:294-320` 以及后续 summary 拼接逻辑

补充特征：

- active group 用现在时，并带 `...`
- 完成后的 group 用过去时
- active shell 超过 2 秒时，会在 hint 行末尾追加 `(xxs · N lines)` 进度尾缀
  证据：`src/components/messages/CollapsedReadSearchContent.tsx:269-291`

---

## 8. metadata、动画、当前活跃态

### 8.1 哪些消息会动画

`MessageRow` 里：

- grouped message：组内任一 tool in-progress 就动画
- collapsed group：组内任一 tool in-progress 就动画
- 普通消息：只要 tool use id 缺失或它仍 in-progress，就允许动画
  证据：`src/components/MessageRow.tsx:155-209`

### 8.2 transcript 下 assistant 消息会显示 metadata

- transcript 模式下，assistant 且含 text/tool_use 等内容的消息，如果有 timestamp/model，就在上方显示 metadata 行
  证据：`src/components/MessageRow.tsx:210-248`

### 8.3 collapsed group 的 active 判定会影响文案与 loader

- active -> `ToolUseLoader`、现在时、hint、shell progress suffix
- inactive -> 不闪动，只显示完成后的摘要
  证据：`src/components/MessageRow.tsx:113-121`，`src/components/messages/CollapsedReadSearchContent.tsx:260-291`

---

## 9. 对 Go 迁移时最值得对照的行为点

如果要在 Go 版复刻原版 TUI，最关键的不是“逐组件长得一样”，而是这些行为语义：

1. `Messages.tsx` 的变换顺序不能乱：normalize -> filter -> group -> collapse -> lookups -> render。
   证据：`src/components/Messages.tsx:379-521`
2. `verbose` 和 transcript 不只是“显示更多”，还会关闭 group/collapse 的多个阶段。
3. thinking 不是简单显示/不显示，而是“只保留本轮最后一个 completed thinking，且 streaming 30 秒内仍压住旧 thinking”。
4. search/read/bash/memory/mcp 的 collapse 不是单纯计数，还会吸收 hook、memory attachment、bash git 结果。
5. bash 输出有“最近一次自动展开”的 UX 特例，旧输出默认截断。
6. 某些 tool use / tool result 会主动返回 `null`，迁移时不能默认所有消息都渲染成一行。

---

## 10. 关键文件索引

- 总入口：`src/components/Messages.tsx`
- 行级控制：`src/components/MessageRow.tsx`
- 块级渲染：`src/components/Message.tsx`
- assistant tool use：`src/components/messages/AssistantToolUseMessage.tsx`
- user tool result：`src/components/messages/UserToolResultMessage/*.tsx`
- read/search 折叠：`src/utils/collapseReadSearch.ts`
- 折叠摘要渲染：`src/components/messages/CollapsedReadSearchContent.tsx`
- 同响应工具分组：`src/utils/groupToolUses.ts`
- 背景 bash 通知折叠：`src/utils/collapseBackgroundBashNotifications.ts`
- hook summary 折叠：`src/utils/collapseHookSummaries.ts`
- teammate shutdown 折叠：`src/utils/collapseTeammateShutdowns.ts`
- bash tool UI：`src/tools/BashTool/UI.tsx`
- bash result 渲染：`src/tools/BashTool/BashToolResultMessage.tsx`
- shell 行截断与自动展开：`src/components/shell/OutputLine.tsx`、`src/components/shell/ExpandShellOutputContext.tsx`

