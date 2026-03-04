# agenTerm — Pivot Decision: From AI Orchestrator to Human Orchestrator

## Date: 2026-03-04

## The Problem We're Actually Solving

The core problem is not "how to coordinate multiple AI agents." The core problem is:

**The human's work rhythm and the AI's work rhythm are fundamentally mismatched.**

AI coding agents work fast and demand frequent responses. When running multiple agents in parallel, the human is interrupted every few minutes, forced to make small decisions without adequate context. This creates two compounding problems:

1. **Cognitive fatigue** — constant interruptions fragment the human's time and drain decision-making capacity. The human cannot do their own work while agents are running.
2. **Decision fragmentation** — judgment is scattered across dozens of micro-decisions (variable names, file structures, error handling approaches) that individually seem trivial but collectively define the project's direction, often without the human realizing it.

A better UI does not solve this. A dashboard with projects, hierarchies, and todo lists still requires the human to respond to the same number of interruptions — it just makes the interruptions prettier. **UI is the presentation layer of cognitive load, not the solution layer.**

## What We Tried and Why It Failed

The previous architecture centered on an **AI orchestrator** — an LLM-based "project manager" that would:

- Decompose requirements into tasks
- Spawn and manage agent sessions
- Drive the build → review → test pipeline automatically
- Handle agent coordination and stage transitions

This failed for two reasons:

1. **Reliability** — the orchestrator frequently failed at basic operations (creating TUI sessions, sending commands) and would fall back to doing work itself instead of delegating. An unreliable automation layer is worse than no automation, because now the human must also verify that the automation worked correctly.

2. **Trust** — even when technically functional, the human (me) would never trust an AI orchestrator to make the right decisions unsupervised. An automation system you don't trust is one you either don't use, or use while spending extra effort checking its work. Both outcomes are worse than the baseline.

**Key insight: I will never trust an AI orchestrator. The previous direction cannot solve my core problem.**

## The Pivot: Human as Orchestrator

The new model keeps decision authority with the human but maximizes the efficiency of each human decision.

### The Management Analogy

A human manager can oversee many teams and projects — not because an automated system makes decisions for them, but because:

- Information is pre-processed before it reaches them (summaries, not raw data)
- Their decisions are structural, not operational (direction, not implementation)
- They have tools that make each decision fast to communicate and execute

agenTerm should be the manager's toolkit, not the manager.

### What the Human Does

The human operates at a **higher abstraction level** — not micro-decisions about code, but macro-decisions about workflow:

- **Defines requirements** in a requirement pool, prioritized
- **Conducts planning sessions** with a planner agent (structured Q&A, not open-ended chat)
- **Creates parallel work structures** — worktrees, sessions, agent assignments — with one click
- **Decides when a stage is complete** and triggers the next stage
- **Performs final testing** as the single mandatory human-in-the-loop checkpoint

The human intervenes at **stage boundaries**, not during execution. Between stages, the human is free — to run, write papers, do consulting, or simply think.

### What AI Agents Do

Each agent works within a single TUI session with full context. Agents:

- Receive a well-scoped task with clear completion criteria
- Execute autonomously within their session (no headless mode — TUI preserves memory and iterative capability)
- Follow rules defined in CLAUDE.md / AGENTS.md (safe commands, completion behavior, what to do when blocked)
- Stop cleanly when done or when uncertain

Agents do **not** coordinate with each other. Stage transitions and cross-agent coordination are the human's job, mediated by agenTerm's UI.

### What agenTerm Does

agenTerm is demoted from "AI orchestrator platform" to **"human orchestrator's control plane."** It is a pure execution engine with no autonomous decision-making.

The orchestrator component, if retained, is reduced to a mechanical state machine: the human presses a button, and it creates sessions, injects prompts, starts agents. It makes zero judgment calls.

## Why TUI Over Headless

We considered headless mode (API calls with no persistent session) but rejected it:

- **No memory** — headless agents start fresh every invocation, losing all accumulated understanding of the codebase. Workarounds (context files, codebase summaries) are adequate but inferior to a live session's implicit context.
- **No iteration** — complex tasks require trying approaches, failing, and adjusting. A persistent TUI session supports this naturally; headless requires external orchestration of retry loops.
- **Cross-agent capability gap** — Claude Code has built-in sub-agent capabilities (can self-orchestrate plan → build → review → test within one session). Codex, Kimi, Qwen, and OpenCode do not. agenTerm's stage management gives these simpler agents the equivalent of Claude Code's sub-agent protocol, externally.

For Claude Code specifically, a well-written CLAUDE.md may be sufficient — it can self-orchestrate. For all other agents, agenTerm provides the stage transitions they cannot do themselves.

## Core Feature Set (New Spec)

### 1. Requirement Pool

A prioritized queue of things to build. Each requirement starts as a short description and gets enriched through planning. When one requirement's full cycle completes (plan → build → review → merge → test), the human clicks "next" and begins the planning session for the next one.

### 2. Planning Session

A structured interaction between the human and a planner agent:

- Human describes what they want
- Planner reads the codebase, generates a question checklist
- Human answers the questions (choosing, not creating — reduces cognitive load)
- Planner outputs: worktree structure, task breakdown, parallel/sequential relationships, completion criteria for each task

Output is stored in the database and becomes the blueprint for execution.

### 3. One-Click Parallel Execution Setup

From the planning output, the human can:

- Create all worktrees at once
- Assign agents to each worktree (system knows available agents and their capabilities)
- Define safe command sets per agent
- Launch all sessions with one action

The system knows the total number of available agents, so allocation is fast.

### 4. Execution Monitor

While agents work, the human sees a simple status board:

- Each task: `running` / `idle` / `done` / `blocked`
- No need to watch output in real time
- Blocked tasks show a one-line reason
- Human can optionally enter any TUI to inspect or take over

### 5. Stage Transition Controls

The human decides when to advance:

- "Build is complete → start review" — launches reviewer agents on completed worktrees
- "Review passed → merge" — triggers merge operations
- "All merged → I'll test now" — human takes over for integration testing

Each transition is an explicit human action, not an automated trigger.

### 6. Cycle Completion

After the human finishes testing:

- Mark the requirement as done
- Click "next requirement" to begin planning the next item in the pool
- Previous cycle's context is archived but accessible

## What We're Cutting

- **AI orchestrator intelligence** — no LLM-based decision-making in the orchestration layer
- **Automatic stage transitions** — all stage boundaries require human approval
- **Autonomous agent coordination** — agents don't talk to each other; the human mediates
- **Notification-driven workflow** — the human checks in on their own schedule, not when pushed

## Design Principle

**The human's cognitive load is determined by decision frequency and decision quality, not by information presentation.** No amount of UI polish compensates for being interrupted too often or making decisions without context. agenTerm succeeds when the human can work in focused blocks — a planning block, a monitoring glance, a review block, a testing block — with genuine freedom between them.