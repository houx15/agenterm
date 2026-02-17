# AgenTerm — Cross-Ecosystem AI Agent Orchestrator

> 一个开源的、跨 AI 生态的多 Agent 编排系统。通过 Web UI 管理项目、调度异构 coding agent、自动化 code review 流程，并支持随时人工接管。

---

## 1. 问题陈述

### 1.1 现状痛点

当前使用多个 AI coding agent（Claude Code、Codex CLI、Gemini CLI、OpenCode(GLM5, MiniMax M2.5, Ark-Latest)、Kimi CLI、 Qwen CLI）协作开发时，存在以下问题：

- **手动编排成本高**：需要手动在多个 terminal 之间切换、复制粘贴 commit hash、传递 review 反馈
- **无法远程监控**：离开电脑后无法了解各 agent 的工作进展
- **缺乏统一视图**：没有一个地方能看到所有项目的所有 task 状态
- **重复劳动**：每个项目都需要手动拆分 worktree、分配 agent、设置 review 流程
- **生态割裂**：Claude 有 sub-agents/agent teams，Kimi 有 agent swarm，但它们各自封闭，无法混合调度

### 1.2 目标

构建一个系统，让用户能够：

0. 自主配置自己现有的模型plan，拆成具像化的可并发的员工数目（需要配置供应商、模型名、coding cli列表、启动命令等）
1. 通过自然语言描述需求，由 AI 项目经理自动拆分任务、创建 worktree、分配 agent
2. 在任何设备上（通过 Tailscale）查看项目状态、与项目经理对话、进入具体 session
3. 定义自己的编程模式（playbook），让项目经理按照惯用方式编排
4. 随时人工接管任何 session，接管后自动化流程暂停

---

## 2. 系统架构

### 2.1 分层设计

```
┌─────────────────────────────────────────────────────────────┐
│                        Web UI Layer                         │
│  ┌──────────┐  ┌───────────────────┐  ┌──────────────────┐  │
│  │ Dashboard │  │ Orchestrator Chat │  │ Session Terminal  │  │
│  │ (全局状态) │  │  (项目经理对话)    │  │  (xterm.js)      │  │
│  └──────────┘  └───────────────────┘  └──────────────────┘  │
├─────────────────────────────────────────────────────────────┤
│                     Go Backend (API)                        │
│  Project CRUD · Session Mgmt · Event Bus · WebSocket Hub    │
├─────────────────────────────────────────────────────────────┤
│                    Orchestrator Layer                        │
│  LLM-based PM · Task Decomposition · Agent Dispatch         │
│  Progress Monitor · Report Generation                       │
├─────────────────────────────────────────────────────────────┤
│                    Execution Layer                           │
│  tmux sessions · git worktrees · Agent processes            │
│  Auto-commit hooks · Completion detection                   │
├─────────────────────────────────────────────────────────────┤
│                    Agent Registry                            │
│  Agent configs · Playbooks · Skills (project-planner, etc.) │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 核心组件

**Go Backend**：已有基础（Go + xterm.js），需扩展为完整的 control plane，暴露 RESTful API。

**Orchestrator(项目经理)**：一个 LLM agent(创建项目时可以配置)，通过调用 Go Backend API 来操作一切。它不直接碰 tmux，所有操作通过 API 间接完成。

**Agent Registry**：YAML 配置文件，定义所有可用的 coding agent 及其特性。用户应该可以通过前端来进行配置，在实际创建项目的时候，可以从中选择

**Execution Layer**：tmux session 作为 agent 的运行容器，每个 session 对应一个 agent 实例。

---

## 3. 数据模型

### 3.1 Project

```yaml
id: uuid
name: string
repo_path: string               # 主仓库路径
status: active | paused | archived
created_at: timestamp
updated_at: timestamp
playbook: string                # 关联的 playbook 名称（可选）
worktrees: Worktree[]
tasks: Task[]
```

### 3.2 Task

```yaml
id: uuid
project_id: uuid
title: string
description: string             # TASK.md 内容摘要
status: pending | running | reviewing | done | failed | blocked
depends_on: task_id[]           # 依赖的其他 task
worktree_id: uuid
sessions: Session[]             # 关联的 tmux sessions
spec_path: string               # TASK.md 文件路径
created_at: timestamp
updated_at: timestamp
```

### 3.3 Worktree

```yaml
id: uuid
project_id: uuid
branch_name: string
path: string                    # worktree 在文件系统中的路径
task_id: uuid
status: active | merged | abandoned
```

### 3.4 Session

```yaml
id: uuid
task_id: uuid
tmux_session_name: string
agent_type: string              # 引用 Agent Registry 中的 agent id
role: coder | reviewer | coordinator
status: idle | running | waiting_review | human_takeover | completed
human_attached: boolean
created_at: timestamp
last_activity_at: timestamp
```

### 3.5 Agent（Registry 配置）

```yaml
# ~/.orchestra/agents/claude-sonnet.yaml
id: claude-sonnet
name: "Claude Code (Sonnet)"
command: "claude --model sonnet"
resume_command: "claude --resume {session_id}"  # 支持会话恢复
headless_command: "claude -p '{prompt}' --allowedTools Edit,Write,Bash --output-format json"
capabilities:
  - code
  - review
  - architecture
languages: [typescript, python, go, rust]
cost_tier: medium               # low | medium | high
speed_tier: fast                # slow | medium | fast
supports_session_resume: true
supports_headless: true
auto_accept_mode: "shift+tab"   # 进入自动模式的方式
```

```yaml
# ~/.orchestra/agents/codex.yaml
id: codex
name: "OpenAI Codex CLI"
command: "codex"
headless_command: "codex -q '{prompt}' --full-auto"
capabilities:
  - code
languages: [typescript, python, go]
cost_tier: low
speed_tier: fast
supports_session_resume: false
supports_headless: true
```

### 3.6 Playbook

```yaml
# ~/.orchestra/playbooks/go-backend.yaml
name: go-backend
description: "Go 后端项目的标准编排方式"
match:
  languages: [go]
  project_patterns: ["go.mod"]
phases:
  - name: scaffold
    agent: codex
    role: coder
    description: "用 Codex 快速生成接口骨架和数据结构"
  - name: implementation
    agent: claude-sonnet
    role: coder
    description: "用 Claude 处理业务逻辑、错误处理、边界 case"
  - name: review
    agent: claude-sonnet
    role: reviewer
    description: "交叉 review，关注安全性和 Go idiom"
  - name: test
    agent: codex
    role: coder
    description: "用 Codex 补充测试用例"
parallelism_strategy: |
  scaffold 各模块可并行。
  implementation 在 scaffold 完成后开始，互不依赖的模块可并行。
  review 在每个模块 implementation 完成后立即开始，不等待所有模块。
  test 在 review 通过后开始。
```

---

## 4. Orchestrator 设计

### 4.1 Orchestrator 是什么

Orchestrator 是一个 LLM agent，扮演"项目经理"角色。它通过 Go Backend 暴露的 API（作为 tool/function）来操作系统。

### 4.2 Orchestrator 的 Tool Set

Orchestrator 通过以下 API 与系统交互：

```
# 项目管理
create_project(name, repo_path, playbook?)
list_projects(status_filter?)
get_project_status(project_id)
archive_project(project_id)

# 任务管理
create_task(project_id, title, description, depends_on?)
update_task_status(task_id, status)
get_task_details(task_id)

# Worktree 管理
create_worktree(project_id, branch_name)
delete_worktree(worktree_id)
get_worktree_git_status(worktree_id)
get_worktree_git_log(worktree_id, n?)

# Session 管理
create_session(task_id, agent_type, role)
send_command(session_id, text)
read_session_output(session_id, since_timestamp?)
is_session_idle(session_id)               # 检测 agent 是否完成
is_human_attached(session_id)

# 文件操作
write_task_spec(worktree_id, content)     # 写入 TASK.md
read_file(worktree_id, path)

# 报告
generate_progress_report(project_id)
```
可能还需要考虑merge conflict的工作（亦或者是调用另一个subagent来实现）

### 4.3 Orchestrator 的运行模式

**无状态、事件驱动**。Orchestrator 不是一个长驻进程。它在以下时机被触发：

1. **用户指令**：用户在 Web UI 的对话面板中输入需求
2. **定时轮询**：Go Backend 每 N 秒检查一次各 session 状态，有变化时调用 orchestrator
3. **事件触发**：某个 agent 完成（session 变为 idle）、commit 被检测到、review 反馈产生

每次触发时，orchestrator 读取当前项目/任务/session 的完整状态，做出决策，执行动作，然后退出。所有状态持久化在数据库或文件系统中。

### 4.4 Orchestrator 的 System Prompt 结构

```
你是一个软件项目经理 AI。你的职责是：
1. 理解用户的需求，拆分成可并行执行的 task
2. 为每个 task 创建 worktree 并编写 TASK.md spec
3. 根据 playbook 和 agent registry 分配合适的 agent
4. 监控进度，在 agent 完成后安排 review 或下一步
5. 生成进度报告

当前项目状态：{project_status_json}
可用 Agent：{agent_registry_summary}
适用 Playbook：{matched_playbook}
用户编程偏好：{user_preferences}

重要规则：
- 如果某个 session 处于 human_takeover 状态，不要对该 session 发送任何命令
- 优先并行执行无依赖关系的 task
- 每个 worktree 的 coder session 完成后，自动创建 reviewer session
- 在关键决策点（如合并分支、删除 worktree）向用户确认
```

---

## 5. 执行层详细设计

### 5.1 每个 Worktree 的 Session 编排

每个 worktree 最多创建三个 tmux session：

| Session | 角色 | 职责 |
|---------|------|------|
| `{project}-{task}-coder` | Coder | 在 TUI 模式下写代码，修改后自动 commit |
| `{project}-{task}-reviewer` | Reviewer | 接收 git diff，输出 review 反馈 |
| `{project}-{task}-coord` | Coordinator | 轻量脚本，监控 git log，在 coder 和 reviewer 之间传递信息 |

### 5.2 Coder ↔ Reviewer 自动化流程

```
Coder 在 tmux session 中工作
        │
        ▼
修改文件 → auto-commit hook 自动 commit（消息带 [READY_FOR_REVIEW] 标记）
        │
        ▼
Coordinator 检测到新 commit
        │
        ▼
Coordinator 执行 git diff HEAD~1，将 diff + TASK.md 发给 Reviewer session
        │
        ▼
Reviewer 输出 review 反馈
        │
        ▼
Coordinator 检测 reviewer 输出完成
        │
        ├─ 如果 review 通过 → 通知 orchestrator，标记 task 完成
        │
        └─ 如果有修改建议 → 将反馈发送给 Coder session，Coder 继续修改
```

### 5.3 Agent 完成检测

按优先级选择检测策略：

1. **Shell prompt 检测**：解析 tmux 输出流，检测 shell prompt 出现（表示 agent TUI 退出或等待输入）
2. **Idle 超时检测**：session 输出超过 N 秒无变化，认为 agent idle
3. **标记文件检测**：agent 在 worktree 中写入 `.orchestra/done` 文件（可通过 TASK.md 指示 agent 这样做）
4. **Git commit 检测**：监控 `git log`，检测到带特定标记的 commit

### 5.4 人工接管机制

```
用户在 Web UI 点击进入某个 session 的 terminal
        │
        ▼
Go Backend 检测到 WebSocket 连接（attach 事件）
        │
        ▼
更新 session.human_attached = true
        │
        ▼
通知 Orchestrator：该 session 进入 human_takeover
Orchestrator 暂停对该 session 及其关联 task 的所有自动化操作
        │
        ▼
用户在 terminal 中直接与 agent 交互
        │
        ▼
用户关闭 terminal（detach 事件）
        │
        ▼
更新 session.human_attached = false
通知 Orchestrator：session 恢复自动模式
```

---

## 6. Skill 与插件自动检测

### 6.1 启动时自动检测

系统启动或创建新项目时，自动检测以下依赖：

```yaml
required_skills:
  - name: project-planner
    description: "拆分任务、分析并行关系、创建 worktree、生成 TASK.md"
    check:
      - path: "~/.claude/skills/project-planner/SKILL.md"
      - path: ".claude/skills/project-planner/SKILL.md"
    install:
      source: "github:user/project-planner-skill"
      target: "~/.claude/skills/project-planner/"

  - name: compound-engineering
    description: "Claude Code 的 compound engineering plugin（plan/act/review 模式）"
    check:
      - command: "claude /help | grep -q 'compound'"
      - path: "~/.claude/plugins/compound-engineering/"
    install:
      source: "npm:@anthropic-ai/compound-engineering"
      # 或手动安装指引
```

### 6.2 project-planner vs compound engineering

| 维度 | project-planner (自研 Skill) | compound engineering (Plugin) |
|------|------------------------------|-------------------------------|
| 作用范围 | 跨 worktree 的高层规划 | 单个 worktree 内的开发流程 |
| 输出物 | worktree + TASK.md | plan → act → review 循环 |
| 触发时机 | 项目初始化阶段 | 每个 task 的执行阶段 |
| 关系 | 先执行，产出 task spec | 后执行，在 spec 指导下开发 |

二者互补而非替代：project-planner 负责"拆成什么"，compound engineering 负责"每一块怎么做"。

### 6.3 自动 Commit Hook

系统为每个 worktree 中的 coding session 自动配置 git hook：

```bash
# .orchestra/hooks/auto-commit.sh
# 由 coordinator session 定期执行，或通过 fswatch/inotifywait 触发

cd {worktree_path}

# 检查是否有未提交的更改
if [ -n "$(git status --porcelain)" ]; then
    git add -A
    
    # 生成 commit message（可选：调用 LLM 生成）
    CHANGES=$(git diff --cached --stat)
    COMMIT_MSG="[auto] ${CHANGES}"
    
    git commit -m "$COMMIT_MSG"
fi
```

配置方式（两种策略，用户可选）：

- **定时提交**：coordinator 每 N 秒检查一次，有更改就 commit
- **Agent 主动提交**：在 TASK.md 中指示 agent 每完成一个逻辑单元后 commit（更语义化）

推荐结合使用：Agent 主动 commit 为主，定时 commit 作为兜底。Agent 主动 commit 时使用 `[READY_FOR_REVIEW]` 标记来触发 review 流程。

### 6.4 Claude Code Hook 集成

对于 Claude Code agent，利用其原生 hook 系统实现自动 commit：

```json
// .claude/settings.json（由 Orchestra 自动注入到 worktree 中）
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Write|Edit",
        "hooks": [
          {
            "type": "command",
            "command": ".orchestra/hooks/auto-commit.sh"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": ".orchestra/hooks/on-agent-stop.sh"
          }
        ]
      }
    ]
  }
}
```

---

## 7. Web UI 设计

### 7.1 页面结构

```
┌─────────────────────────────────────────────────────┐
│  Orchestra                        [用户] [设置]      │
├──────────┬──────────────────────────────────────────┤
│ Sidebar  │                                          │
│          │           Main Content Area              │
│ 📊 Dashboard │                                      │
│ 💬 PM Chat   │                                      │
│ ─────────│                                          │
│ Projects │                                          │
│  ├ knoweia   │                                      │
│  ├ orchestra │                                      │
│  └ gce-sim   │                                      │
│ ─────────│                                          │
│ Sessions │                                          │
│  ├ 🟢 active │                                      │
│  ├ 🟡 review │                                      │
│  └ 🔴 failed │                                      │
│          │                                          │
└──────────┴──────────────────────────────────────────┘
```

### 7.2 Dashboard（仪表盘）

显示全局状态概览：

- 当前活跃项目数量、各项目进度百分比
- 正在运行的 session 数量，按状态分类
- 最近完成的 task 列表
- 资源使用（token 消耗估算、各 agent 活跃时间）
- 历史项目存档（可搜索、可恢复）

### 7.3 PM Chat（项目经理对话面板）

这是与 Orchestrator 交互的主界面：

- **左侧**：项目列表 + 当前项目的 task DAG 可视化（显示依赖关系和执行状态）
- **右侧**：与 Orchestrator 的对话窗口
- **输入方式**：
  - 文字输入（默认）
  - 语音输入（按住录音 → 语音转文字 → 可编辑后发送，详见 7.5）
- **操作**：
  - 新建项目：描述需求 → Orchestrator 自动规划
  - 查看进度：Orchestrator 生成报告
  - 调整计划：修改 task 优先级、更换 agent、暂停/恢复
  - 归档项目：完成后存档

对话面板中 Orchestrator 的回复应包含可交互元素：

- task 列表中每个 task 可点击跳转到对应 session
- "创建 worktree" "启动 agent" 等操作前应有确认按钮
- 进度报告中嵌入 session 的实时缩略图

### 7.4 Session Terminal（终端视图）

- 按项目分组展示所有 session
- 每个 session 显示：agent 类型图标、角色标签（coder/reviewer）、状态指示灯
- 点击进入全屏 terminal（xterm.js），此时触发 human_takeover
- 支持分屏查看同一 worktree 的 coder 和 reviewer session
- 退出 terminal 时弹出确认："将控制权交还给项目经理？"

### 7.5 语音输入

在 PM Chat 面板中支持语音输入：

**交互流程**：

1. 用户按住麦克风按钮（或按住快捷键，如空格长按）
2. 录制音频，实时显示波形动画
3. 松开后，音频发送到 STT 服务进行转写
4. 转写文本出现在输入框中，用户可编辑
5. 用户确认后发送（Enter 或点击发送按钮）

**技术选型**：

- 前端：Web Audio API 录制 → 发送到后端
- STT 服务（按优先级）：
  1. 本地 Whisper（如果用户配置了，延迟低、免费）
  2. 云端 STT API（Deepgram / Azure Speech / OpenAI Whisper API）
- 支持中英文混合识别

**配置**：

```yaml
# ~/.orchestra/config.yaml
voice_input:
  enabled: true
  stt_provider: whisper_local    # whisper_local | openai | deepgram | azure
  whisper_model: base            # tiny | base | small | medium | large
  language: zh                   # 主要语言，辅助识别准确率
  hotkey: space                  # 长按触发录音的快捷键
```

---

## 8. API 设计

### 8.1 RESTful API（Go Backend）

```
# 项目
POST   /api/projects                    创建项目
GET    /api/projects                    列出项目
GET    /api/projects/:id                获取项目详情（含 tasks/sessions）
PATCH  /api/projects/:id                更新项目（状态、playbook 等）
DELETE /api/projects/:id                归档项目

# 任务
POST   /api/projects/:id/tasks          创建 task
GET    /api/projects/:id/tasks          列出 tasks
GET    /api/tasks/:id                   获取 task 详情
PATCH  /api/tasks/:id                   更新 task 状态
GET    /api/tasks/:id/spec              获取 TASK.md 内容

# Worktree
POST   /api/projects/:id/worktrees      创建 worktree
GET    /api/worktrees/:id/git-status    获取 git status
GET    /api/worktrees/:id/git-log       获取 git log
DELETE /api/worktrees/:id               删除 worktree

# Session
POST   /api/tasks/:id/sessions          创建 session（指定 agent + role）
GET    /api/sessions                    列出所有 session（支持筛选）
GET    /api/sessions/:id                获取 session 详情
POST   /api/sessions/:id/send           向 session 发送命令
GET    /api/sessions/:id/output         获取 session 最近输出
GET    /api/sessions/:id/idle           检测 session 是否 idle
PATCH  /api/sessions/:id/takeover       标记/取消 human takeover

# Agent Registry
GET    /api/agents                      列出已配置的 agent
GET    /api/agents/:id                  获取 agent 详情

# Playbook
GET    /api/playbooks                   列出 playbook
GET    /api/playbooks/:id               获取 playbook 详情

# Orchestrator
POST   /api/orchestrator/chat           与 orchestrator 对话
GET    /api/orchestrator/report/:project_id   获取项目报告

# 语音
POST   /api/voice/transcribe            上传音频 → 返回文字
```

### 8.2 WebSocket

```
ws://host/ws/session/:id             Session terminal 连接（xterm）
ws://host/ws/orchestrator            Orchestrator 对话流式输出
ws://host/ws/events                  全局事件推送（task 状态变更、session 状态变更等）
```

---

## 9. 配置结构

```
~/.orchestra/
├── config.yaml              # 全局配置（STT、默认模型、Tailscale 等）
├── agents/                  # Agent Registry
│   ├── claude-sonnet.yaml
│   ├── claude-opus.yaml
│   ├── codex.yaml
│   ├── gemini.yaml
│   ├── opencode-glm.yaml
│   ├── opencode-minimax.yaml
│   └── kimi.yaml
├── playbooks/               # 编排 Playbook
│   ├── go-backend.yaml
│   ├── react-frontend.yaml
│   ├── python-research.yaml
│   └── default.yaml
├── skills/                  # 共享 Skills（如 project-planner）
│   └── project-planner/
│       └── SKILL.md
└── data/                    # 持久化数据
    ├── orchestra.db         # SQLite（项目、任务、session 元数据）
    └── logs/                # Orchestrator 决策日志
```

项目级配置：

```
{project_root}/
├── .orchestra/
│   ├── project.yaml         # 项目配置（覆盖全局）
│   ├── tasks/               # Task 状态文件
│   │   ├── task-001.yaml
│   │   └── task-002.yaml
│   └── hooks/               # 自动注入的 hook 脚本
│       ├── auto-commit.sh
│       └── on-agent-stop.sh
├── .claude/                 # Claude Code 配置（自动生成）
│   ├── settings.json        # 含 auto-commit hook
│   └── skills/
│       └── project-planner/ # 自动检测/下载
└── CLAUDE.md                # 自动生成/更新
```

---

## 10. 技术栈

| 组件 | 技术选型 | 理由 |
|------|---------|------|
| Backend | Go | 已有基础，性能好，适合长连接管理 |
| Terminal | xterm.js + tmux | 已有基础 |
| 前端 | React / Next.js | 组件化开发，SSR 可选 |
| 数据库 | SQLite | 轻量，单机部署，无需额外服务 |
| Orchestrator LLM | Claude Sonnet API | 性价比高，function calling 支持好 |
| STT | Whisper (本地) / Deepgram (云端) | Whisper 免费且支持中文混合 |
| 远程访问 | Tailscale | 已有基础，零配置 VPN |
| 进程通信 | tmux send-keys + 输出捕获 | 统一的 agent 交互方式，不依赖特定 agent 的 API |

---

## 11. 实施路线

### Phase 1: 基础框架（1-2 周）

- [ ] 扩展 Go Backend API：Project CRUD、Session 管理（基于现有 tmux 管理代码）
- [ ] 定义 Agent Registry YAML schema，配置 2-3 个常用 agent
- [ ] 实现 worktree 创建/删除 API
- [ ] 实现 session 命令发送和输出读取 API
- [ ] 数据库 schema（SQLite）

### Phase 2: Orchestrator 最小可用版（1-2 周）

- [ ] 编写 Orchestrator 的 system prompt + tool definitions
- [ ] 实现 `/api/orchestrator/chat` 接口（调用 Claude API + function calling）
- [ ] 集成 project-planner skill 的逻辑到 orchestrator（任务拆分 → worktree 创建 → TASK.md 生成）
- [ ] 实现基础的 agent 分派：创建 session → 发送初始 prompt
- [ ] 在终端中验证完整流程：描述需求 → 任务拆分 → agent 启动 → 代码产出

### Phase 3: 自动化流程（1-2 周）

- [ ] 实现 auto-commit hook（定时 + Claude PostToolUse hook）
- [ ] 实现 session idle 检测（输出流监控）
- [ ] 实现 coordinator 脚本（coder → reviewer 的 diff 传递）
- [ ] 实现事件驱动 orchestrator 触发（task 完成 → 自动安排下一步）
- [ ] 实现 human_takeover 机制（WebSocket attach/detach 检测）

### Phase 4: Web UI（2-3 周）

- [ ] Dashboard 页面（项目概览、session 状态面板）
- [ ] PM Chat 页面（对话界面 + task DAG 可视化）
- [ ] Session Terminal 页面（分组列表 + 全屏 xterm + 分屏支持）
- [ ] 语音输入集成（Web Audio API + Whisper 转写）
- [ ] 全局事件 WebSocket（实时状态更新）

### Phase 5: Skill 自动管理与打磨（1 周）

- [ ] 启动时自动检测 project-planner / compound engineering
- [ ] 缺失时自动下载安装（从 GitHub 或 npm）
- [ ] Playbook 编辑界面（Web UI 中可视化编辑）
- [ ] Orchestrator 决策日志查看
- [ ] 项目归档与恢复

---

## 12. 开放问题

1. **Orchestrator 模型选择**：用 Claude Sonnet 够用吗？复杂的任务拆分是否需要 Opus？可以考虑分层：Opus 做初始规划，Sonnet 做日常监控。

2. **跨 agent session 恢复**：Claude Code 支持 `--resume`，但 Codex/Gemini/OpenCode 的 session 恢复支持各不相同。需要为不支持恢复的 agent 设计 fallback（重新注入上下文）。

3. **Token 预算管理**：不同 agent 的 token 消耗差异大，orchestrator 是否需要考虑 token 预算来做调度决策？

4. **冲突处理**：多个 agent 在不同 worktree 上工作，merge 时可能有冲突。Orchestrator 应该自动尝试解决还是升级给人类？

5. **安全性**：tmux send-keys 本质上可以执行任何命令。需要考虑 agent 逃逸（向 tmux 注入非预期命令）的风险。

6. **project-planner 与 compound engineering 的精确边界**：需要实际使用后确定。当前假设是 project-planner 做跨 worktree 规划，compound engineering 做 worktree 内开发循环，但可能存在重叠。

---

## 13. 从现有系统借鉴

### Claude Code Agent Teams

- ✅ 借鉴：subagent 定义格式（markdown + YAML frontmatter）、memory 目录、tool restriction per role
- ❌ 不适用：单一生态封闭调度

### Kimi K2.5 Agent Swarm

- ✅ 借鉴：自主并行判断（orchestrator prompt 中鼓励并行）、serial collapse 问题意识
- ❌ 不适用：PARL 训练方法（需要大量训练数据）

### claude-flow / Agentrooms

- ✅ 借鉴：tmux session 作为 execution runtime 的思路、状态仪表盘设计
- ❌ 不适用：过度复杂的插件体系

### VS Code Multi-Agent（1.109）

- ✅ 借鉴：Agent Sessions view 的 UX（统一查看不同 agent 的 session）、subagent 并行可视化
- ❌ 不适用：绑定 VS Code 生态
