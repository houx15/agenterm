---
status: complete
priority: p2
issue_id: "019"
tags: [code-review, orchestrator, scheduling, human-takeover]
dependencies: []
---

# Human takeover sessions are excluded from capacity accounting

## Problem Statement

Sessions in `human_takeover` status are treated as inactive for scheduling. This can oversubscribe project/model capacity while those sessions are still consuming real resources.

## Findings

- `internal/orchestrator/scheduler.go:137` marks `human_takeover` as inactive.
- Scheduler checks use `isActiveSessionStatus()` for all capacity counters.

## Proposed Solutions

### Option 1: Count `human_takeover` as active for capacity

Pros:
- Reflects actual resource occupancy.
- Prevents silent oversubscription.

Cons:
- May reduce automation throughput while humans are attached.

Effort: Small
Risk: Low

### Option 2: Add separate configurable policy

Pros:
- Flexible for teams wanting different behavior.

Cons:
- More complexity and configuration burden.

Effort: Medium
Risk: Medium

## Recommended Action

## Technical Details

Affected files:
- `internal/orchestrator/scheduler.go`

## Acceptance Criteria

- [x] `human_takeover` sessions are accounted for by default in capacity checks.
- [x] Tests cover takeover session blocking additional session creation at limit.

## Work Log

### 2026-02-17 - Reported during workflows-review

- Found status classification mismatch with practical capacity behavior.

### 2026-02-17 - Fixed

- `human_takeover` now counts as active in scheduler capacity checks.
- Added regression test proving takeover session blocks new work when at project cap.

## Resources

- `internal/orchestrator/scheduler.go:137`
