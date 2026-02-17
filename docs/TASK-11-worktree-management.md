# Task: worktree-management

## Context
The SPEC requires git worktree management as the isolation mechanism for parallel agent work. Each task gets its own worktree (branch + directory), so multiple agents can work on different features simultaneously without conflicts. The Go backend needs to create, manage, and query git worktrees.

## Objective
Implement git worktree lifecycle management: create worktrees for tasks, query git status/log within worktrees, and clean up worktrees when tasks complete.

## Dependencies
- Depends on: TASK-07 (database-models), TASK-08 (rest-api)
- Branch: feature/worktree-management
- Base: main (after TASK-08 merge)

## Scope

### Files to Create
- `internal/git/worktree.go` — Git worktree operations (create, remove, list, status, log)
- `internal/git/git.go` — Low-level git command execution helpers

### Files to Modify
- `internal/api/worktrees.go` — Wire handlers to git package
- `cmd/agenterm/main.go` — No significant changes (git ops are stateless)

### Files NOT to Touch
- `internal/tmux/` — No changes
- `internal/hub/` — No changes
- `web/` — No frontend changes

## Implementation Spec

### Step 1: Git command helpers
```go
// internal/git/git.go
func runGit(dir string, args ...string) (string, error)  // Execute git command in directory
func IsGitRepo(path string) bool                         // Check if path is a git repo
func GetRepoRoot(path string) (string, error)            // Get repo root from any subpath
```

### Step 2: Worktree operations
```go
// internal/git/worktree.go

// CreateWorktree creates a new git worktree with a new branch
// Runs: git worktree add <path> -b <branch> from the repo root
func CreateWorktree(repoPath, worktreePath, branchName string) error

// RemoveWorktree removes a worktree and optionally deletes the branch
// Runs: git worktree remove <path>
func RemoveWorktree(repoPath, worktreePath string) error

// ListWorktrees returns all worktrees for a repo
// Runs: git worktree list --porcelain
func ListWorktrees(repoPath string) ([]WorktreeInfo, error)

// GetStatus returns parsed git status for a worktree
// Runs: git -C <path> status --porcelain
func GetStatus(worktreePath string) (*GitStatus, error)

// GetLog returns recent commits in a worktree
// Runs: git -C <path> log --oneline -n <count>
func GetLog(worktreePath string, count int) ([]CommitInfo, error)

// GetDiff returns diff between two refs
// Runs: git -C <path> diff <ref1>...<ref2>
func GetDiff(worktreePath, ref1, ref2 string) (string, error)
```

### Step 3: Data types
```go
type WorktreeInfo struct {
    Path   string `json:"path"`
    Branch string `json:"branch"`
    HEAD   string `json:"head"`
    Bare   bool   `json:"bare"`
}

type GitStatus struct {
    Modified  []string `json:"modified"`
    Added     []string `json:"added"`
    Deleted   []string `json:"deleted"`
    Untracked []string `json:"untracked"`
    Clean     bool     `json:"clean"`
}

type CommitInfo struct {
    Hash    string `json:"hash"`
    Message string `json:"message"`
    Author  string `json:"author"`
    Date    string `json:"date"`
}
```

### Step 4: Wire to API handlers
- `POST /api/projects/{id}/worktrees` — Create worktree, save to DB
- `GET /api/worktrees/{id}/git-status` — Read worktree path from DB, run git status
- `GET /api/worktrees/{id}/git-log` — Read worktree path from DB, run git log
- `DELETE /api/worktrees/{id}` — Remove worktree, update DB

### Step 5: Worktree path convention
Worktrees are created at: `{repo_path}/.worktrees/{task-slug}/`
Branch naming: `feature/{task-slug}`

## Testing Requirements
- Test worktree creation in a test git repo
- Test worktree removal cleans up filesystem
- Test git status parsing (modified, added, deleted, untracked files)
- Test git log parsing
- Test error handling (repo doesn't exist, branch already exists)

## Acceptance Criteria
- [x] Worktrees created with correct branch and path
- [x] Git status returns structured data (not raw text)
- [x] Git log returns structured commit list
- [x] Worktree removal cleans up both filesystem and DB
- [x] All git commands sanitize input to prevent injection
- [x] Works with repos that have existing worktrees

## Notes
- ALWAYS sanitize branch names and paths before passing to git commands
- Use `exec.Command("git", args...)` — never shell out with string interpolation
- Worktree paths should be absolute
- Consider: what happens if a worktree directory already exists? Check and error gracefully.
- The `.worktrees/` directory should be added to `.gitignore` of the main repo
