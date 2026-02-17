package api

import (
	"bufio"
	"bytes"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/user/agenterm/internal/db"
)

var branchSegmentRe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

type createWorktreeRequest struct {
	BranchName string `json:"branch_name"`
	Path       string `json:"path"`
	TaskID     string `json:"task_id"`
}

type gitStatusResponse struct {
	Modified  []string `json:"modified"`
	Added     []string `json:"added"`
	Deleted   []string `json:"deleted"`
	Untracked []string `json:"untracked"`
	Clean     bool     `json:"clean"`
}

type gitCommit struct {
	Hash    string `json:"hash"`
	Message string `json:"message"`
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
	repoRoot, err := filepath.Abs(project.RepoPath)
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid project repo_path")
		return
	}
	if worktreePath == "" {
		worktreePath = filepath.Join(repoRoot, ".worktrees", slug)
	}
	if !filepath.IsAbs(worktreePath) {
		worktreePath = filepath.Join(repoRoot, worktreePath)
	}
	worktreePath = filepath.Clean(worktreePath)
	if !pathWithinBase(repoRoot, worktreePath) {
		jsonError(w, http.StatusBadRequest, "worktree path must be inside project repo")
		return
	}
	if err := os.MkdirAll(filepath.Dir(worktreePath), 0o755); err != nil {
		jsonError(w, http.StatusInternalServerError, fmt.Sprintf("failed to create worktree parent directory: %v", err))
		return
	}

	if err := runGit(repoRoot, "worktree", "add", worktreePath, "-b", branchName); err != nil {
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

func (h *handler) getWorktreeGitStatus(w http.ResponseWriter, r *http.Request) {
	worktree, ok := h.mustGetWorktree(w, r)
	if !ok {
		return
	}
	out, err := runGitOutput(worktree.Path, "-C", worktree.Path, "status", "--porcelain")
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	status := parseGitStatus(out)
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
	out, err := runGitOutput(worktree.Path, "-C", worktree.Path, "log", "--oneline", "-n", strconv.Itoa(count))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	commits := parseGitLog(out)
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
	if err := runGit(project.RepoPath, "-C", project.RepoPath, "worktree", "remove", worktree.Path); err != nil {
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

func runGit(dir string, args ...string) error {
	_, err := runGitOutput(dir, args...)
	return err
}

func runGitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return "", fmt.Errorf("git %s failed: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
		}
		return "", fmt.Errorf("git %s failed: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

func parseGitStatus(output string) gitStatusResponse {
	status := gitStatusResponse{}
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) < 3 {
			continue
		}
		code := line[:2]
		file := strings.TrimSpace(line[3:])
		switch {
		case strings.HasPrefix(code, "??"):
			status.Untracked = append(status.Untracked, file)
		case strings.Contains(code, "A"):
			status.Added = append(status.Added, file)
		case strings.Contains(code, "D"):
			status.Deleted = append(status.Deleted, file)
		default:
			status.Modified = append(status.Modified, file)
		}
	}
	status.Clean = len(status.Modified) == 0 && len(status.Added) == 0 && len(status.Deleted) == 0 && len(status.Untracked) == 0
	return status
}

func parseGitLog(output string) []gitCommit {
	commits := make([]gitCommit, 0)
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		entry := gitCommit{Hash: parts[0]}
		if len(parts) > 1 {
			entry.Message = parts[1]
		}
		commits = append(commits, entry)
	}
	return commits
}
