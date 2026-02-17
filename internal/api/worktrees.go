package api

import (
	"fmt"
	"net/http"
	"os"
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
	if err := h.worktreeRepo.Delete(r.Context(), worktree.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusNoContent, nil)
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
