# TASK-11 worktree management

- [x] Read task doc and inspect existing API/db patterns
- [x] Add `internal/git/git.go` git command helpers and repo root checks
- [x] Add `internal/git/worktree.go` worktree lifecycle + status/log/diff operations
- [x] Refactor `internal/api/worktrees.go` to use `internal/git` package
- [x] Add tests for worktree lifecycle, status parsing, log parsing, and error handling
- [x] Run targeted tests for `internal/git` and `internal/api`
- [x] Update task acceptance checklist in `docs/TASK-11-worktree-management.md`
