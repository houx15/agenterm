package api

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/user/agenterm/internal/db"
	gitops "github.com/user/agenterm/internal/git"
)

var branchSegmentRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type createWorktreeRequest struct {
	BranchName string `json:"branch_name"`
	Path       string `json:"path"`
	TaskID     string `json:"task_id"`
}

type mergeWorktreeRequest struct {
	TargetBranch string `json:"target_branch"`
}

type resolveWorktreeConflictRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

func (h *handler) createWorktree(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("id")
	project, err := h.projectRepo.Get(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}

	var req createWorktreeRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	repoRoot, err := gitops.GetRepoRoot(project.RepoPath)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid project repo_path")
		return
	}

	slug := sanitizeBranchSegment(req.TaskID)
	if slug == "" {
		slug = "task"
	}
	branchName := req.BranchName
	if branchName == "" {
		branchName = "feature/" + slug
	}
	branchName = sanitizeBranchName(branchName)
	if branchName == "" {
		jsonError(w, http.StatusBadRequest, "invalid branch_name")
		return
	}

	worktreePath := req.Path
	if worktreePath == "" {
		worktreePath = filepath.Join(repoRoot, ".worktrees", slug)
	}
	if !filepath.IsAbs(worktreePath) {
		worktreePath = filepath.Join(repoRoot, worktreePath)
	}
	absWorktreePath, err := filepath.Abs(worktreePath)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid worktree path")
		return
	}
	worktreePath = filepath.Clean(absWorktreePath)
	if !pathWithinBase(repoRoot, worktreePath) {
		jsonError(w, http.StatusBadRequest, "worktree path must be inside project repo")
		return
	}
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create worktree parent directory: %v", err))
		return
	}

	if err := gitops.CreateWorktree(repoRoot, worktreePath, branchName); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	worktree := &db.Worktree{
		ProjectID:  projectID,
		BranchName: branchName,
		Path:       worktreePath,
		TaskID:     req.TaskID,
		Status:     "active",
	}
	if err := h.worktreeRepo.Create(r.Context(), worktree); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if req.TaskID != "" {
		task, err := h.taskRepo.Get(r.Context(), req.TaskID)
		if err == nil && task != nil {
			task.WorktreeID = worktree.ID
			_ = h.taskRepo.Update(r.Context(), task)
		}
	}

	jsonResponse(w, http.StatusCreated, worktree)
}

func pathWithinBase(base string, target string) bool {
	base = canonicalPathForCompare(base)
	target = canonicalPathForCompare(target)
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	if rel == "." {
		return true
	}
	rel = filepath.Clean(rel)
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func canonicalPathForCompare(path string) string {
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return filepath.Clean(resolved)
	}
	ancestor := path
	suffix := make([]string, 0, 4)
	for {
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			break
		}
		suffix = append([]string{filepath.Base(ancestor)}, suffix...)
		ancestor = parent
		if resolved, err := filepath.EvalSymlinks(ancestor); err == nil {
			parts := append([]string{filepath.Clean(resolved)}, suffix...)
			return filepath.Join(parts...)
		}
	}
	return path
}

func (h *handler) getWorktreeGitStatus(w http.ResponseWriter, r *http.Request) {
	worktree, ok := h.mustGetWorktree(w, r)
	if !ok {
		return
	}
	status, err := gitops.GetStatus(worktree.Path)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, status)
}

func (h *handler) getWorktreeGitLog(w http.ResponseWriter, r *http.Request) {
	worktree, ok := h.mustGetWorktree(w, r)
	if !ok {
		return
	}
	count := 20
	if raw := r.URL.Query().Get("n"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			jsonError(w, http.StatusBadRequest, "invalid n query parameter")
			return
		}
		if n > 100 {
			n = 100
		}
		count = n
	}
	commits, err := gitops.GetLog(worktree.Path, count)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, commits)
}

func (h *handler) deleteWorktree(w http.ResponseWriter, r *http.Request) {
	worktree, ok := h.mustGetWorktree(w, r)
	if !ok {
		return
	}
	project, err := h.projectRepo.Get(r.Context(), worktree.ProjectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if project == nil {
		jsonError(w, http.StatusBadRequest, "project for worktree not found")
		return
	}
	if err := gitops.RemoveWorktree(project.RepoPath, worktree.Path); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	if worktree.TaskID != "" {
		task, err := h.taskRepo.Get(r.Context(), worktree.TaskID)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if task != nil && task.WorktreeID == worktree.ID {
			task.WorktreeID = ""
			if err := h.taskRepo.Update(r.Context(), task); err != nil {
				jsonError(w, http.StatusInternalServerError, err.Error())
				return
			}
		}
	}
	if err := h.worktreeRepo.Delete(r.Context(), worktree.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusNoContent, nil)
}

func (h *handler) mergeWorktree(w http.ResponseWriter, r *http.Request) {
	worktree, ok := h.mustGetWorktree(w, r)
	if !ok {
		return
	}
	project, err := h.projectRepo.Get(r.Context(), worktree.ProjectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if project == nil {
		jsonError(w, http.StatusBadRequest, "project for worktree not found")
		return
	}
	var req mergeWorktreeRequest
	if err := decodeJSON(r, &req); err != nil && err != io.EOF {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	sourceBranch := strings.TrimSpace(worktree.BranchName)
	if sourceBranch == "" {
		jsonError(w, http.StatusBadRequest, "worktree branch_name is required")
		return
	}
	targetBranch := strings.TrimSpace(req.TargetBranch)
	if targetBranch == "" {
		targetBranch = detectDefaultBranch(project.RepoPath)
	}

	result, err := mergeBranch(project.RepoPath, sourceBranch, targetBranch)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := map[string]any{
		"worktree_id":    worktree.ID,
		"source_branch":  sourceBranch,
		"target_branch":  targetBranch,
		"source_commit":  result.SourceCommit,
		"merged":         result.Merged,
		"conflict":       result.Conflict,
		"conflict_files": result.ConflictFiles,
	}
	if result.Merged {
		worktree.Status = "merged"
		_ = h.worktreeRepo.Update(r.Context(), worktree)
		resp["status"] = "merged"
		if h.hub != nil {
			h.hub.BroadcastProjectEvent(worktree.ProjectID, "worktree_merge_succeeded", map[string]any{
				"worktree_id":   worktree.ID,
				"source_branch": sourceBranch,
				"target_branch": targetBranch,
				"commit_hash":   result.SourceCommit,
			})
		}
		jsonResponse(w, http.StatusOK, resp)
		return
	}
	if result.Conflict {
		worktree.Status = "conflict"
		_ = h.worktreeRepo.Update(r.Context(), worktree)
		if strings.TrimSpace(worktree.TaskID) != "" {
			task, err := h.taskRepo.Get(r.Context(), worktree.TaskID)
			if err == nil && task != nil {
				task.Status = "pending"
				_ = h.taskRepo.Update(r.Context(), task)
			}
		}
		resp["status"] = "conflict"
		if h.hub != nil {
			h.hub.BroadcastProjectEvent(worktree.ProjectID, "worktree_merge_conflict", map[string]any{
				"worktree_id":      worktree.ID,
				"source_branch":    sourceBranch,
				"target_branch":    targetBranch,
				"commit_hash":      result.SourceCommit,
				"conflicted_files": result.ConflictFiles,
			})
		}
		jsonResponse(w, http.StatusOK, resp)
		return
	}
	resp["status"] = "unchanged"
	jsonResponse(w, http.StatusOK, resp)
}

func (h *handler) resolveWorktreeConflict(w http.ResponseWriter, r *http.Request) {
	worktree, ok := h.mustGetWorktree(w, r)
	if !ok {
		return
	}
	var req resolveWorktreeConflictRequest
	if err := decodeJSON(r, &req); err != nil && err != io.EOF {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	message := strings.TrimSpace(req.Message)
	if message == "" {
		message = "Please resolve merge conflicts in this worktree, commit the fix, and finish with [READY_FOR_REVIEW]."
	}

	sessionID := strings.TrimSpace(req.SessionID)
	if sessionID == "" && strings.TrimSpace(worktree.TaskID) != "" {
		sessions, _ := h.sessionRepo.ListByTask(r.Context(), worktree.TaskID)
		sessionID = pickCoderSessionID(sessions)
	}
	if strings.TrimSpace(worktree.TaskID) != "" {
		task, err := h.taskRepo.Get(r.Context(), worktree.TaskID)
		if err == nil && task != nil {
			task.Status = "pending"
			_ = h.taskRepo.Update(r.Context(), task)
		}
	}
	worktree.Status = "active"
	_ = h.worktreeRepo.Update(r.Context(), worktree)

	sent := false
	if sessionID != "" && h.lifecycle != nil {
		if err := h.lifecycle.SendCommand(r.Context(), sessionID, message+"\n"); err == nil {
			sent = true
		}
	}

	if h.hub != nil {
		h.hub.BroadcastProjectEvent(worktree.ProjectID, "worktree_conflict_resolution_requested", map[string]any{
			"worktree_id": worktree.ID,
			"task_id":     worktree.TaskID,
			"session_id":  sessionID,
			"sent":        sent,
		})
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"worktree_id": worktree.ID,
		"task_id":     worktree.TaskID,
		"session_id":  sessionID,
		"sent":        sent,
		"status":      "resolution_requested",
	})
}

func (h *handler) mustGetWorktree(w http.ResponseWriter, r *http.Request) (*db.Worktree, bool) {
	id := r.PathValue("id")
	worktree, err := h.worktreeRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if worktree == nil {
		jsonError(w, http.StatusNotFound, "worktree not found")
		return nil, false
	}
	return worktree, true
}

func sanitizeBranchSegment(v string) string {
	v = branchSegmentRe.ReplaceAllString(strings.ToLower(strings.TrimSpace(v)), "-")
	v = strings.Trim(v, "-._/")
	if v == "" {
		return ""
	}
	return v
}

func sanitizeBranchName(v string) string {
	parts := strings.Split(v, "/")
	clean := make([]string, 0, len(parts))
	for _, part := range parts {
		if p := sanitizeBranchSegment(part); p != "" {
			clean = append(clean, p)
		}
	}
	return strings.Join(clean, "/")
}

type mergeResult struct {
	SourceCommit  string
	Merged        bool
	Conflict      bool
	ConflictFiles []string
}

func mergeBranch(repoPath, sourceBranch, targetBranch string) (mergeResult, error) {
	repoPath = strings.TrimSpace(repoPath)
	sourceBranch = strings.TrimSpace(sourceBranch)
	targetBranch = strings.TrimSpace(targetBranch)
	if repoPath == "" || sourceBranch == "" || targetBranch == "" {
		return mergeResult{}, fmt.Errorf("repo path and branches are required")
	}
	sourceCommit, err := gitOut(repoPath, "rev-parse", "--verify", sourceBranch)
	if err != nil {
		return mergeResult{}, err
	}
	result := mergeResult{SourceCommit: strings.TrimSpace(sourceCommit)}

	if already, err := commitMerged(repoPath, result.SourceCommit, targetBranch); err == nil && already {
		result.Merged = true
		return result, nil
	}

	original, _ := gitOut(repoPath, "rev-parse", "--abbrev-ref", "HEAD")
	origBranch := strings.TrimSpace(original)
	defer func() {
		if origBranch != "" && origBranch != "HEAD" {
			_, _ = gitOut(repoPath, "checkout", origBranch)
		}
	}()

	if _, err := gitOut(repoPath, "checkout", targetBranch); err != nil {
		return result, err
	}
	if _, err := gitOut(repoPath, "merge", "--no-ff", "--no-edit", sourceBranch); err != nil {
		conflictFiles, listErr := gitOut(repoPath, "diff", "--name-only", "--diff-filter=U")
		if listErr == nil && strings.TrimSpace(conflictFiles) != "" {
			result.Conflict = true
			result.ConflictFiles = splitNonEmptyLines(conflictFiles)
		}
		_, _ = gitOut(repoPath, "merge", "--abort")
		return result, nil
	}
	result.Merged = true
	return result, nil
}

func detectDefaultBranch(repoPath string) string {
	if out, err := gitOut(repoPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		b := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(out), "origin/"))
		if b != "" {
			return b
		}
	}
	if out, err := gitOut(repoPath, "rev-parse", "--abbrev-ref", "HEAD"); err == nil {
		b := strings.TrimSpace(out)
		if b != "" && b != "HEAD" {
			return b
		}
	}
	return "main"
}

func commitMerged(repoPath, commitHash, targetBranch string) (bool, error) {
	cmd := exec.Command("git", "-C", repoPath, "merge-base", "--is-ancestor", strings.TrimSpace(commitHash), strings.TrimSpace(targetBranch))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return false, nil
		}
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return false, fmt.Errorf("git merge-base --is-ancestor failed: %s", msg)
	}
	return true, nil
}

func gitOut(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return string(out), nil
}

func pickCoderSessionID(sessions []*db.Session) string {
	var fallback string
	for _, sess := range sessions {
		if sess == nil || !strings.EqualFold(strings.TrimSpace(sess.Role), "coder") {
			continue
		}
		if fallback == "" {
			fallback = sess.ID
		}
		st := strings.ToLower(strings.TrimSpace(sess.Status))
		if st == "completed" || st == "failed" {
			continue
		}
		return sess.ID
	}
	return fallback
}

func splitNonEmptyLines(raw string) []string {
	parts := strings.Split(raw, "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(part)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
