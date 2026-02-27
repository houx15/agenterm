  P0 (must decide before implementation starts)

  1. What is the exact Definition of Done for each stage (brainstorm, plan, build, test, summarize)?
  - definition of done = give a result, ask the user to adopt (write a doc, clear context and move to the next phase)
  - brainstorm: generate 3-5 solutions with design motivations and technical solutions
  - plan: a step-by-step plan with parallelable worktree design. generate spec docs
  - build: each worktree. first step - build test against spec. second step - implement spec and pass tests. third step - review and give feedbacks. finish until reviewer think the code is good. make sure that git commit and git push after every changes.
  - test: build e2e test and run
  - summarize: summarize the things we've done.


  2. What minimum artifacts are mandatory after plan (worktree graph, spec docs, role assignment proposal, risk list)?
  - worktree graph, spec docs, role assignment proposal.

  3. What exact TDD contract do we enforce in build (test-first required always, or role/template dependent)?
  - template dependent. tdd is suggested.

  4. Which test levels are required per task type (unit, integration, e2e, smoke)?
  - unit, e2e and smoke

  5. What is the gate rule for merge: “all tests green” only, or plus reviewer sign-off and zero unresolved critical findings?
  - all tests green + reviewer sign-off and zero unresolved critical findings.

  6. Which actions are fully autonomous vs approval-required in each stage?
  - all edits inside the project folder is fully autonomous.
  - all readonly even outside the project folder is fully autonomous.
  - write/edit/delete outside the project folder is approval-required
  - web-fetch related commands, mcp commands are fully autonomous.

  7. What is the escalation policy when agent asks confirmation repeatedly or stalls?
    - first step: orchestrator handles. then human attention needs - send notice and wait human's reply

  8. What is the maximum retry policy per role loop before hard stop?
  - for the code build - review loop, one build one review is a loop, and we accept maximum 20 loops.
  - I don't think there would be other loops.

  9.  What is the canonical worktree/branch naming scheme?
    - feature/xxx
    - bug/xxx 


  10. What is the canonical spec document schema and location (docs/specs/...)?
    - docs/task-xxx.md 
  11. How should orchestrator choose among multiple eligible agents for the same role (priority, load, cost, model quality)?
      - we write some specifications for agents. decide based on this. but always let the human do the final decision.  
  12. What is the policy if required role inputs are missing: auto-derive, ask user, or block stage?
      1.  first auto-derive
      2.  if cannot then ask the user.
      3.  we can do our best (like designing apis) to let this required inputs are good.
  13. What is the exact project bootstrap behavior: always run git init/remote checks or detect-and-skip?
      1.  always check git status, if no git, ask the user whether init or clone from remote. if init, git init and tell the user a warning.
  14. What is the conflict policy: auto-resolve allowed, or always reviewer-assisted?
      1.  auto-resolve, agent-resolve. cannot resolve then ask human to review.
  15. What are the hard safety boundaries (forbidden commands, forbidden paths, destructive git actions)?
      1.  forbidden commands: rm -rf
      2.  1. 系统级破坏性命令
        这些命令可能导致系统损坏、数据丢失或不可逆更改，因此在自动执行模式里通常会被拒绝或要求人工确认：
            •	删除整个文件系统或目录（如 rm -rf /）
            •	重新格式化磁盘（如 mkfs, fdisk）
            •	低层设备写入（如 dd if=）
        这些会被标记为“危险命令”，在 approval_policy 配置里会被要求确认或直接 deny（禁止）执行。 ￼

        2. 高风险 shell 语句 / 远程执行
        允许命令执行外部脚本或从外部不受信任来源拉取执行代码也被视作危险，通常需要审批或禁止：
            •	curl … | bash
            •	wget … | sh
            •	任意未审查的远程脚本执行
        这类命令可能导致恶意代码执行或凭据泄露。 ￼

        3. 超出当前工作区以外的操作
        Codex 的默认 sandbox 模式限制它不能访问工作区之外的文件、网络或系统敏感资源：
            •	网络访问（默认被屏蔽）除非配置了白名单或明确允许
            •	访问用户主目录外的敏感文件
            •	访问系统目录如 /etc, /usr/bin 等不属于当前项目的地方
        这些限制是防止意外泄露或破坏用户环境。 ￼

        4. Git 破坏性操作
        在 CLI 模式下，如果任务明确要求 Codex 只修改特定文件，那么对版本库进行破坏性操作（如强制 reset、清理其他分支、删除分支等）也会被规则限制或要求人工确认。实际使用中用户配置的规则文件（.rules 或 approval_policy）决定哪些 git 操作是被禁止的。 ￼


  P1 (must decide during Slice 1–2)
  16. Do we keep tmux as execution substrate in vNext, or move to native PTY ownership in Tauri runtime?
      1.  actually all the reason I want tmux is that we can know the progress or give some confirmation when I'm outside, I can use tailscale to connect my computer and do something. but I think, maybe I don't need tmux. since we have orchestrator.
      2.  chatgpt's suggestion:
      3.  我会给你一个实用的落地建议：默认走“非交互 job 模式”，只在需要时进入“救援交互模式”。
            •	默认（90%）：agent 以非交互方式运行命令，stdout/stderr 持久化；orchestrator 做日志裁剪 + 关键行提取 + 定期总结；你远程只看总结/关键片段。
            •	救援（10%）：当任务需要 TUI/密码输入/复杂交互时，再启用交互通道（可以是原生 PTY，也可以是 tmux attach，甚至就是 SSH 手工进）。

        这样你就能同时满足“多 agent 管理、远程对话、低耦合、跨平台”，并且把终端的复杂性降到最低。
  17. What is the final command protocol schema (send_text, send_key, interrupt, resize, close)?
      1.  send text, send key, interrupt, resize, close
  18. How do we define session readiness per agent type (prompt signatures, timeout, fallback)?
      1.  I'm not quite sure... usually, we can see that we can enter something - this is for enter. sometimes it is several options for selection.
  19. What is the final desktop information architecture (pane defaults, collapse behavior, keyboard-first flows)?
      1.  统计 - 最近的agent占用情况、任务处理情况、平均需要响应时间
      2.  工作区 - 项目（有新消息会提示） -> 置顶orchestrator和下面不同的agent终端
          1.  这里可以创建项目，创建项目需要选择文件夹。设置orchestrator，然后确认团队配置。
          2.  项目页面顶部可以有一个项目路线图。项目当前的plan可能分割了多个feature，哪些在working，怎么样了。
      3.  settings - agent registry（名字，启动命令，specifications，最大并行数）. core workflow 编排（类似于节点拖拽）
  20. Which mobile actions are allowed besides approvals/status/reports?
      1.  only need message and approval.
  21. How are notifications delivered (in-app only vs push/email/webhook)?
      1.  in-app only.
  22. What metrics define “successful adoption” (cycle time, interruption count, autonomous completion rate, failure rate)?
      1.  cycle time, interruption count, failure rate
  23. What is the release strategy (internal alpha, feature flags, per-project opt-in)?
      1.  this is an opensource app.
  24. What is the support/debug workflow when a run fails mid-stage (one-click diagnostic bundle, replay, resume)?
      1.  yes we need diagnostic bundle.