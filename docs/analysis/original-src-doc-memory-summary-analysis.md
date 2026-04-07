# 原始 `src/` 中“文档检索 / 记忆 / 会话总结”设计解析

本文只基于当前仓库原始 TypeScript `src/` 源码整理，不基于 Go 版本推测，也不讨论迁移方案。重点不是“有哪些命令”，而是原码里和 RAG 类似的上下文组织机制：

1. 检索源从哪里来
2. 候选集如何构造
3. 什么时候触发召回
4. 如何筛选、限流、去重
5. 最终如何注入模型上下文
6. 会话总结和记忆系统如何互相配合

---

## 1. 先给结论：原码不是单一 RAG，而是“多层上下文检索架构”

从 `src/` 的实现看，这个 CLI 没有一个统一名叫 “RAG” 的模块，但实际上存在一套明显的“分层检索 + 分层注入”设计。可以概括成 4 层：

### 1.1 静态长期上下文层

这层在每次会话开始就注入，偏“永远要带着的说明”：

- `Managed` memory
- `User` memory
- `Project` memory
- `Local` memory
- `AutoMem` / `TeamMem` 入口文件（在特定 gate 下）

对应源码：

- `src/utils/claudemd.ts`
- `src/context.ts`
- `src/memdir/memdir.ts`

### 1.2 路径触发的局部上下文层

这层不是每次都注入，而是当模型碰到某个文件或目录时，沿路径向上做“局部规则发现”：

- 嵌套 `CLAUDE.md`
- `.claude/rules/*.md`
- 带 `paths` frontmatter 的 conditional rules
- 动态 skills 目录

对应源码：

- `src/utils/attachments.ts`
- `src/utils/claudemd.ts`
- `src/skills/loadSkillsDir.ts`

### 1.3 查询时相关召回层

这层最像经典 RAG：

- 用户输入作为 query
- 先构造 memory manifest
- 再让模型从 manifest 中选择最相关 memories
- 最后把选中的 memory 文件正文作为 attachment 注入

对应源码：

- `src/query.ts`
- `src/utils/attachments.ts`
- `src/memdir/findRelevantMemories.ts`
- `src/memdir/memoryScan.ts`

### 1.4 会话后沉淀 / 摘要层

这层不是为了当前 query 检索，而是为了把当前会话的信息变成未来可复用的知识或压缩摘要：

- `extractMemories`：把 durable knowledge 写回 auto-memory
- `Session Memory`：维护当前会话 notes
- `compactConversation`：把长对话压缩成 summary
- `toolUseSummary`：把工具批次变成一条短标签

对应源码：

- `src/services/extractMemories/extractMemories.ts`
- `src/services/SessionMemory/sessionMemory.ts`
- `src/services/compact/compact.ts`
- `src/services/toolUseSummary/toolUseSummaryGenerator.ts`

从设计上看，原码不是“一个记忆模块 + 一个检索模块”，而是：

- 静态上下文注入
- 路径相关规则召回
- 查询相关记忆召回
- 回合后知识沉淀
- 会话压缩总结

这些一起组成了原始 CLI 的“上下文增强系统”。

---

## 2. 语料源设计：原码里有哪些“可被检索/注入的知识库”

如果从 RAG 视角看，第一件事不是看 query，而是看“语料源”。

原码里的语料源主要有 5 类。

### 2.1 `CLAUDE.md` / `.claude/rules`：规则型知识库

`src/utils/claudemd.ts` 规定了 instruction memory 的主层级：

1. `Managed` memory，例如 `/etc/claude-code/CLAUDE.md`
2. `User` memory，例如 `~/.claude/CLAUDE.md`
3. `Project` memory，例如：
   - `CLAUDE.md`
   - `.claude/CLAUDE.md`
   - `.claude/rules/*.md`
4. `Local` memory，例如 `CLAUDE.local.md`

文件头注释写得很清楚：

- 用户级文件从 home 目录加载
- 项目级与本地级从当前目录一路向上遍历到 root
- 离当前目录越近，优先级越高

这类语料更像“规则/约束/协作约定知识库”，不是会话历史。

对应源码：

- `src/utils/claudemd.ts`
- `src/context.ts`

### 2.2 Auto Memory / Team Memory：持久记忆知识库

`src/memdir/memdir.ts` 定义了持久 memory 目录的行为规范。

关键点：

- memory 是文件系统上的持久目录，不是数据库
- 支持 `MEMORY.md` 作为入口索引
- 真正的记忆写在独立 topic 文件里
- `MEMORY.md` 只是索引，不应直接塞大段正文

`buildMemoryLines(...)` 和 `buildMemoryPrompt(...)` 明确告诉模型：

- memory 分类型保存
- 每条 memory 单独成文件
- 再在 `MEMORY.md` 里留一条短索引
- 不要保存可以从代码或项目当前状态推导出来的内容

如果开了 team memory，则还会增加 team 目录。

对应源码：

- `src/memdir/memdir.ts`
- `src/memdir/paths.ts`（调用侧依赖）

### 2.3 Session Memory：当前会话专用知识库

`Session Memory` 在设计上与 auto-memory 不同。

它不是“跨会话可复用知识库”，而是“当前会话工作状态的结构化笔记文件”。

`src/services/SessionMemory/prompts.ts` 的默认模板包含：

- Session Title
- Current State
- Task specification
- Files and Functions
- Workflow
- Errors & Corrections
- Codebase and System Documentation
- Learnings
- Key results
- Worklog

这更像“当前 session 的工作摘要缓存”，作用是会话连续性，而不是长期 recall。

对应源码：

- `src/services/SessionMemory/sessionMemory.ts`
- `src/services/SessionMemory/prompts.ts`

### 2.4 Magic Docs：文档型知识库

带 `# MAGIC DOC: ...` 头的 markdown 文档，在被 `FileReadTool` 读到之后，会被登记到 `trackedMagicDocs`。

后续每轮主线程稳定结束后，后台 agent 会基于最新对话去更新这些文档。

这说明 `Magic Docs` 本质上是一种“活文档知识库”，它的目标不是为当前 turn 召回，而是让项目文档随着会话长期演化。

对应源码：

- `src/services/MagicDocs/magicDocs.ts`
- `src/services/MagicDocs/prompts.ts`

### 2.5 Skills：操作性知识库

skills 在原码里本质上也是一类知识源，只不过不是普通 markdown 说明，而是“可执行工作流说明”。

来源包括：

- bundled skills
- user/project/plugin skills
- 动态发现的嵌套 `.claude/skills`
- MCP skill commands

这些技能既可作为 slash/skill 调用目标，也会以 attachment 形式被提示给模型。

对应源码：

- `src/skills/loadSkillsDir.ts`
- `src/utils/attachments.ts`
- `src/constants/prompts.ts`

---

## 3. 静态注入层：原码如何在会话开始时“预装上下文”

这一层最像“启动时预加载知识库”。

### 3.1 `getUserContext()` 会把 `CLAUDE.md` 体系拼成统一上下文

`src/context.ts` 中：

- `getUserContext()` 会调用 `getMemoryFiles()`
- 然后经过 `filterInjectedMemoryFiles(...)`
- 再调用 `getClaudeMds(...)`
- 最终把结果挂到 user context 里的 `claudeMd`

这意味着：

- `CLAUDE.md` 系列不是 attachment 注入，而是直接拼到 conversation 的前置用户上下文
- 它属于每轮 query 都默认带着的静态上下文前缀

对应源码：

- `src/context.ts`
- `src/utils/claudemd.ts`

### 3.2 `getMemoryFiles()` 的发现顺序体现了“优先级栈”

`src/utils/claudemd.ts` 的 `getMemoryFiles()` 很关键。

它按顺序加载：

1. Managed `CLAUDE.md`
2. Managed `.claude/rules/*.md`
3. User `CLAUDE.md`
4. User `.claude/rules/*.md`
5. 从当前目录向上到 root 的 Project / Local files
6. 可选的 `--add-dir` 目录
7. AutoMem 入口 `MEMORY.md`
8. TeamMem 入口

注意两点：

- 加载顺序是“从低优先到高优先”，后面的更靠近当前工作目录，更容易被模型关注
- 这里加载的不只是单个 `CLAUDE.md`，而是一整个分层 instruction stack

### 3.3 `@include` 让静态层支持“文件级展开”

`processMemoryFile(...)` 和 `extractIncludePathsFromTokens(...)` 支持 `@path` 引入其他文本文件。

设计点很明确：

- 只允许文本扩展名，避免把二进制内容拉进上下文
- 用 `marked` lexer 扫描 markdown token，只在文本节点里处理 `@include`
- 跳过 code block / codespan
- 支持相对路径、绝对路径、`~/`
- 用 `processedPaths` 防止循环引用
- 有最大 include 深度 `MAX_INCLUDE_DEPTH = 5`

这实际上给静态 instruction 层提供了一个“轻量文档展开机制”。

### 3.4 静态层也支持条件规则

`.claude/rules/*.md` 不只是无条件加载。

`parseFrontmatterPaths(...)` 会从 frontmatter 中抽出 `paths`，形成 glob。

后续：

- 无 frontmatter `paths` 的规则可视为 unconditional
- 有 `paths` 的规则属于 conditional rules

这意味着 instruction 体系本身就支持“按路径选择性激活”，不是纯静态全文塞入。

---

## 4. 路径触发检索层：当模型接触文件时，如何做局部规则召回

这部分是原码里最接近“基于操作对象检索局部上下文”的机制。

### 4.1 触发条件：当输入或 IDE 选择带来具体文件路径

`src/utils/attachments.ts` 中，很多输入来源都可能触发文件相关 attachment：

- `@file` at-mention
- IDE 当前打开文件
- IDE 当前选中文本所在文件
- 某些工具把路径写入 `nestedMemoryAttachmentTriggers`

之后会调用 `getNestedMemoryAttachments(...)` / `getNestedMemoryAttachmentsForFile(...)`。

### 4.2 `getNestedMemoryAttachmentsForFile(...)` 的四阶段流程

这段代码几乎可以直接视为“按文件路径做局部检索”的核心流程。

它的顺序在注释里写得非常明确：

1. Managed/User conditional rules matching targetPath
2. Nested directories（CWD 到 target）中的 `CLAUDE.md` + unconditional rules + conditional rules
3. CWD-level directories（root 到 CWD）中的 conditional rules

这样设计的原因是：

- 全局和用户级的 conditional rules 要先匹配
- 越靠近目标文件的目录，越应该优先提供局部约束
- root 到 CWD 之间的上层目录，只补充 conditional rules，避免重复注入已 eager-loaded 的 unconditional rules

对应源码：

- `src/utils/attachments.ts`
- `src/utils/claudemd.ts`

### 4.3 如何计算“需要走哪些目录”

`getDirectoriesToProcess(targetPath, originalCwd)` 会构造两个目录序列：

- `nestedDirs`
  - 从 `originalCwd` 到目标文件目录之间的路径
  - 用来加载 nested `CLAUDE.md` 与 rules
- `cwdLevelDirs`
  - 从 root 到 `originalCwd`
  - 这里只补 conditional rules

这个拆分很重要，因为它把：

- “进入更深目录时新增的局部规则”
- “上层目录原本就存在、但只在特定文件路径下激活的条件规则”

分成了两个不同处理阶段。

### 4.4 `memoryFilesToAttachments(...)` 是局部召回的注入出口

筛出的 memory file 最终不会直接拼到 system prompt，而是转成 `nested_memory` attachment。

它还做了两层 dedup：

- `loadedNestedMemoryPaths`
  - 非淘汰式 Set，防止 session 内反复重复注入
- `readFileState`
  - LRU 级别的“已经看过/已经加载过”缓存

并且会把注入内容写回 `readFileState`，让后续：

- dedup 能识别已加载文件
- 文件改动检测能工作

### 4.5 attachment 注入给模型时的形态

`src/utils/messages.ts` 里，`nested_memory` 最终会被包装成：

`Contents of <path>:\n\n<content>`

并作为 meta user message 包进 system reminder。

也就是说，局部规则的进入方式是：

- 文件级 attachment
- 再转成 meta user message
- 最终进入本轮上下文

这非常像“按路径命中后把文档片段塞进 prompt”。

---

## 5. 查询相关召回层：`relevant_memories` 是原码里最像经典 RAG 的部分

这一层最值得单独展开。

### 5.1 触发时机：query 一开始就启动 prefetch

`src/query.ts` 开头：

- `using pendingMemoryPrefetch = startRelevantMemoryPrefetch(...)`

这说明 memory retrieval 不是同步阻塞式调用，而是“边主模型推理、边后台准备召回结果”的并行 prefetch。

后面到工具执行完成后，如果 prefetch 已经完成，就把结果注入：

- `pendingMemoryPrefetch.settledAt !== null`
- `consumedOnIteration === -1`
- 然后 `filterDuplicateMemoryAttachments(await pendingMemoryPrefetch.promise, readFileState)`

所以这条链路是：

1. query 开始即并发检索
2. 不阻塞主 turn
3. 如果检索赶得上，就在同一轮后段注入
4. 如果赶不上，这轮直接跳过，不等待

这是典型的 latency-sensitive retrieval 设计。

### 5.2 查询词不是整段历史，而是“最后一条真实用户消息”

`startRelevantMemoryPrefetch(...)` 的 query 构造逻辑比较保守：

- 只找最后一条 `user && !isMeta` 的消息
- 把这条消息文本当作 retrieval query
- 如果只有一个词，直接跳过，不做 recall

这意味着原码不尝试基于整段会话 embedding 或 multi-turn summary 做召回，而是用“当前用户意图”做即时检索。

### 5.3 候选集构造：不是读全量正文，而是先扫 header/frontmatter

`findRelevantMemories(...)` 的第一步不是读全量 memory 文件内容，而是：

- `scanMemoryFiles(memoryDir, signal)`

`scanMemoryFiles(...)` 做的是：

- 遍历 memory 目录里的 `.md`
- 排除 `MEMORY.md`
- 只读取前若干行 `FRONTMATTER_MAX_LINES = 30`
- 解析 frontmatter 中的：
  - `description`
  - `type`
- 取 `mtimeMs`
- 返回按新旧排序、最多 `MAX_MEMORY_FILES = 200` 个 header

也就是说：

- 候选集建立阶段只读“文件头摘要”
- 不读全文
- 这是明显的两阶段检索设计：先便宜筛候选，再决定读正文

### 5.4 候选选择器：再调用一次模型做相关性选择

`selectRelevantMemories(...)` 会把 header manifest 格式化后交给 `sideQuery(...)`。

prompt 明确要求模型：

- 最多返回 5 个 memory
- 只有“明确有帮助”才选
- 不确定就别选
- 如果最近已经成功调用某些工具，不要再选这些工具的使用手册型 memory
- 但如果 memory 里是 gotcha / known issues，则仍应保留

这说明选择器并不是简单关键词匹配，而是二次模型判断。

从结构上看，它就是：

1. 文件系统扫描得到候选 header
2. LLM selector 从 header list 中选 top-k

这已经非常接近轻量 RAG selector 的标准形态。

### 5.5 检索范围还能按 agent mention 做“子库切换”

`getRelevantMemoryAttachments(...)` 有一个很关键的行为：

- 如果用户输入里提到了某个 agent
- 那就不查默认 auto-memory 目录
- 而是只查该 agent 对应的 memory dir

这等于做了“命名空间级 retrieval routing”：

- 默认查主 memory 库
- 提到特定 agent 时切换到子 memory 库

这是一个很明显的多库检索设计。

### 5.6 读取正文发生在候选选中之后

只有被选中的 memory，才会进一步进入 `readMemoriesForSurfacing(...)`。

这一步会：

- 真正读取 memory 正文
- 给每个 memory 生成 header，包含 freshness / mtime 信息
- 对过长内容做截断说明

所以正文读取是第二阶段，成本只花在 top-k 上。

### 5.7 去重策略：三层去重

这一层的去重设计很细。

#### 第一层：`alreadySurfaced`

`collectSurfacedMemories(messages)` 会扫描历史消息里的 `relevant_memories` attachment：

- 收集已经 surfacing 过的 path
- 顺便计算累计 bytes

后续 selector 在进入模型前就过滤这些 path，避免把名额浪费在马上会被去重掉的旧结果上。

#### 第二层：`readFileState`

即使 selector 选中了某 memory，如果这文件本轮或之前已经被显式读过、写过、编辑过，也会被过滤。

这样可以避免：

- 用户已经 `Read` 过的 memory
- 再次以 attachment 形式被重复塞回去

#### 第三层：session 总量限流

`startRelevantMemoryPrefetch(...)` 会先看 `surfaced.totalBytes`。

如果超过 `RELEVANT_MEMORIES_CONFIG.MAX_SESSION_BYTES`，本轮就不再 recall。

这不是单次 top-k 限制，而是整个 session 级别的 recall budget。

### 5.8 注入形态：不是一句摘要，而是“带 header 的原文片段”

`src/utils/messages.ts` 里，`relevant_memories` 会被转成一组 meta user messages：

- header 里带 path 和时间信息
- 后面直接接 memory 正文

这意味着召回结果不是摘要化标签，而是“半原文上下文块”。

这和真正的文档片段 RAG 很接近。

### 5.9 UI 侧还有“Recall N memories”的折叠显示

虽然底层注入的是原文片段，但 UI 会尽量把它表现得简洁：

- `AttachmentMessage.tsx` 里显示 `Recalled N memories`
- `collapseReadSearch.ts` 会把这些 `relevant_memories` 吸收到 collapsed group 里

也就是：

- prompt 侧是富上下文
- UI 侧是轻展示

---

## 6. 静态 memory 与动态 memory recall 的关系

这是原码里最容易混淆、但其实边界最清楚的一点。

### 6.1 静态层：`CLAUDE.md` / `MEMORY.md` 入口是“始终注入”

这些内容进入 `getUserContext()`，属于 query 前缀的一部分。

特点：

- 每轮都带
- 不做相关性选择
- 主要是规则、指令、索引

### 6.2 动态 recall：`relevant_memories` 是“按 query 召回”

这些内容进入 attachment。

特点：

- 只在需要时召回
- 先筛 candidate 再选 top-k
- 主要是 topic memory 正文

### 6.3 原码故意把入口索引与正文 recall 分开

这点在 `claudemd.ts` 与 `memdir.ts` 的配合里很明显：

- `MEMORY.md` 入口是静态注入
- 真正的 topic files 正文不静态全量注入
- query 时再通过 `findRelevantMemories(...)` 做选择性 surfacing

这是非常典型的“索引常驻、正文按需召回”的设计。

---

## 7. 写回层：原码如何把当前会话信息变成未来可召回知识

如果只有 recall 没有 write-back，这套系统就不是闭环。

### 7.1 `extractMemories`：durable memory 的后台写回器

`src/query/stopHooks.ts` 在 turn 结束后会 fire-and-forget 调用 `executeExtractMemories(...)`。

`extractMemories.ts` 文件头说明了它的职责：

- 从当前 session transcript 提取 durable memories
- 写到 `~/.claude/projects/<path>/memory/`

它有几个关键保护：

- 只在主线程跑
- 如果主 agent 本轮已经直接写了 memory 文件，就跳过
- 用 closure-scoped 状态防止并发提取冲突
- 如果正在提取，则把新的上下文 stash 起来，等下一次 trailing run

### 7.2 写回 agent 的工具权限是强约束的

`createAutoMemCanUseTool(memoryDir)` 限定：

- 允许：Read / Grep / Glob
- 允许：只读 Bash
- 允许：仅对 memoryDir 下的文件执行 Edit / Write
- 其他全部 deny

这说明 durable memory 写回虽然借助 agent 做抽取，但文件权限被压得很窄。

### 7.3 `extractMemories` 也复用了扫描层

源码注释明确说：

- 它先扫描 memory 目录
- 复用 `findRelevantMemories` 的 frontmatter scan
- 避免写回 agent 还要先浪费一轮去 `ls`

这说明 recall 和 write-back 并不是两套完全独立的基础设施，而是共用 memory scan 基座。

---

## 8. Session Memory：另一种“当前会话 RAG 缓存”

如果把 RAG 理解为“把过去上下文压缩成未来仍可用的中间表示”，那 `Session Memory` 其实也是一条独立链路。

### 8.1 它不是 durable memory，而是“会话态摘要缓存”

`Session Memory` 只在：

- `querySource === 'repl_main_thread'`
- gate 开启
- auto-compact 开启

时工作。

它的触发阈值依赖：

- 当前消息 token 总量
- 自上次提取后的 token 增量
- 自上次提取后的 tool call 数

这说明它是一个“代价敏感、按增长触发”的增量摘要器。

### 8.2 它的输出是结构化 notes，不是短摘要

和 `compactConversation` 最大的不同是：

- compact 目标是“上下文缩减”
- session memory 目标是“任务连续性保持”

因此 session memory 的模板里会保留：

- 当前状态
- 关键文件
- 工作流
- 报错与纠正
- worklog

这类信息不像 durable memory 那样强调跨会话通用，也不像 compact summary 那样强调压缩。

### 8.3 它与 compact 还有直接关系

`src/services/compact/sessionMemoryCompact.ts` 会先：

- `waitForSessionMemoryExtraction()`
- `getSessionMemoryContent()`

说明在某些 compact 路径下，session memory 会作为更高质量、结构化的会话摘要输入。

也就是说，session memory 既是独立笔记系统，也是 compact 的辅助摘要来源。

---

## 9. 文档维护层：`Magic Docs` 不是 recall，但属于知识演化机制

虽然 `Magic Docs` 不直接参与当前 query 的 recall，但它在整体知识体系里很重要。

### 9.1 触发条件是“先读到文档”

只有当文件被 `FileReadTool` 读过，且内容里有 `# MAGIC DOC:` 头，才会被登记。

这说明它不是全仓扫描，而是“按用户/模型显式接触过的文档开始跟踪”。

### 9.2 更新时机是“回合稳定后”

只有最后一轮 assistant 没有 tool call 时，才会跑 Magic Docs 更新。

这等于把文档维护放在“当前回合已经收束”的时机，避免和主任务争资源。

### 9.3 更新策略是 current-state rewrite

prompt 明确要求：

- 文档要反映当前状态
- 不写 changelog
- 删除过时内容
- 重点写 overview / architecture / entry points / gotchas

所以从知识演化角度看，Magic Docs 更像“把会话中新学到的信息沉淀成长期项目文档”。

---

## 10. 技能发现层：原码里另一种“文档检索”

虽然技能不是传统文档，但它们在原码里承担了“把相关操作知识推到当前上下文”的作用。

### 10.1 turn-0 与 inter-turn 两种发现路径

`src/utils/attachments.ts` 中：

- 用户输入阶段会做 turn-0 skill discovery
- `src/query.ts` 中每轮还会启动 `startSkillDiscoveryPrefetch(...)`

说明技能发现也采用了：

- 初始同步发现
- 后续并发预取

的双路径设计。

### 10.2 skills 的来源不止一处

除了本来就已加载的 skill set，`src/skills/loadSkillsDir.ts` 还支持动态发现：

- 从当前触碰文件路径往上找 `.claude/skills`
- 越靠近文件的目录优先级越高

这和路径规则召回的思路高度一致，只不过知识单位从 rules 变成了 skills。

### 10.3 skill attachment 的注入语义是“提醒 + 入口”

`src/utils/messages.ts` 里，`skill_discovery` 最终会转成：

- `Skills relevant to your task:`
- 每条 skill 只有名字和 description
- 明示“Invoke via Skill("<name>") for complete instructions”

也就是说：

- 发现阶段只给候选技能摘要
- 真正完整内容在用户或模型调用 Skill 时再展开

这本质上也是一种“索引先行、正文按需展开”的设计。

---

## 11. 会话总结层：原码里有哪些“摘要”以及它们分别服务什么

源码里至少有四种摘要，不应混为一类。

### 11.1 `tool_use_summary`

作用：

- 给客户端显示一条短进度标签

输入：

- tool name / input / output
- 最后一段 assistant 文本

输出：

- 30 字左右、类似 commit subject 的短标签

对应源码：

- `src/query.ts`
- `src/services/toolUseSummary/toolUseSummaryGenerator.ts`

### 11.2 `Session Memory`

作用：

- 维护当前会话结构化工作笔记

输出：

- 一份 markdown notes 文件

对应源码：

- `src/services/SessionMemory/sessionMemory.ts`

### 11.3 `compactConversation`

作用：

- 在上下文过长时，用 summary 替换原历史

输出：

- summary messages
- boundary marker
- 必要 attachments / kept messages

对应源码：

- `src/services/compact/compact.ts`
- `src/commands/compact/compact.ts`

### 11.4 `extractMemories`

作用：

- 从当前会话中提取 durable knowledge

输出：

- 未来可被 `findRelevantMemories(...)` 召回的 topic memory 文件

对应源码：

- `src/services/extractMemories/extractMemories.ts`

所以：

- `tool_use_summary` 是 UI 摘要
- `Session Memory` 是会话笔记摘要
- `compactConversation` 是上下文压缩摘要
- `extractMemories` 是长期知识摘要

---

## 12. 用 RAG 术语重新描述原码架构

如果强行用 RAG 术语去映射，原始 `src/` 大致相当于：

### 12.1 Corpus / Knowledge Sources

- `CLAUDE.md` / `.claude/rules`
- Auto memory topic files
- Team memory topic files
- Session memory file
- Magic Docs
- Skills / nested skills

### 12.2 Index / Metadata Layer

- `MEMORY.md` 作为 memory index
- frontmatter `description`
- frontmatter `type`
- frontmatter `paths`
- 文件路径层级
- `mtimeMs`

### 12.3 Retrieval Triggers

- 会话启动
- 用户输入
- 文件 at-mention
- IDE 打开文件
- 工具碰到新文件
- 回合结束
- 上下文过长

### 12.4 Retrieval Strategy

- eager load：静态 `CLAUDE.md`
- path-based retrieval：nested memory / conditional rules / dynamic skills
- query-based retrieval：`relevant_memories`
- model-based reranking：`selectRelevantMemories(...)`
- scoped routing：agent mention -> agent memory dir

### 12.5 Injection Form

- user context 前缀
- attachment -> meta user message
- system reminder wrapper
- compact summary replacement

### 12.6 Write-back / Knowledge Distillation

- `extractMemories`
- `Session Memory`
- `Magic Docs`
- `compactConversation`

换句话说，原码并不是“直接把所有记忆都塞给模型”，而是：

- 静态规则常驻
- 目录/路径规则按需激活
- memory topic 用 header 做候选、正文做按需召回
- 当前会话信息不断被蒸馏回结构化存储

这就是它最像 RAG 的地方。

---

## 13. 最终判断

如果只看“类似 RAG 的设计”，原始 `src/` 里最核心的不是某一个单独模块，而是下面这组组合：

1. `getMemoryFiles()` + `getClaudeMds()`
   - 负责静态 instruction corpus 装载
2. `getNestedMemoryAttachmentsForFile(...)`
   - 负责基于目标路径的局部规则检索
3. `findRelevantMemories(...)`
   - 负责基于当前用户 query 的相关记忆召回
4. `extractMemories(...)`
   - 负责把会话内容反写为未来可召回知识
5. `Session Memory`
   - 负责把当前会话压成结构化工作记忆
6. `Magic Docs` / skill discovery
   - 负责把文档与操作知识持续演化并在需要时注入

因此，从源码设计角度看，这个 CLI 的“RAG 部分”不是一个中心化检索器，而是一套分层、分触发条件、分注入形态的上下文增强系统。

