package api

import (
	"net/http"

	"github.com/user/agenterm/internal/db"
)

type createProjectRequest struct {
	Name     string `json:"name"`
	RepoPath string `json:"repo_path"`
	Playbook string `json:"playbook"`
	Status   string `json:"status"`
}

type updateProjectRequest struct {
	Name     *string `json:"name"`
	RepoPath *string `json:"repo_path"`
	Playbook *string `json:"playbook"`
	Status   *string `json:"status"`
}

type projectDetailResponse struct {
	Project   *db.Project    `json:"project"`
	Tasks     []*db.Task     `json:"tasks"`
	Worktrees []*db.Worktree `json:"worktrees"`
	Sessions  []*db.Session  `json:"sessions"`
}

func (h *handler) createProject(w http.ResponseWriter, r *http.Request) {
	var req createProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Name == "" || req.RepoPath == "" {
		jsonError(w, http.StatusBadRequest, "name and repo_path are required")
		return
	}
	status := req.Status
	if status == "" {
		status = "active"
	}

	project := &db.Project{
		Name:     req.Name,
		RepoPath: req.RepoPath,
		Status:   status,
		Playbook: req.Playbook,
	}
	if err := h.projectRepo.Create(r.Context(), project); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if h.projectOrchestratorRepo != nil {
		if err := h.projectOrchestratorRepo.EnsureDefaultForProject(r.Context(), project.ID); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	jsonResponse(w, http.StatusCreated, project)
}

func (h *handler) listProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := h.projectRepo.List(r.Context(), db.ProjectFilter{Status: r.URL.Query().Get("status")})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, projects)
}

func (h *handler) getProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := h.projectRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}

	tasks, err := h.taskRepo.ListByProject(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	worktrees, err := h.worktreeRepo.ListByProject(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	sessions := make([]*db.Session, 0)
	for _, task := range tasks {
		taskSessions, err := h.sessionRepo.ListByTask(r.Context(), task.ID)
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		sessions = append(sessions, taskSessions...)
	}

	jsonResponse(w, http.StatusOK, projectDetailResponse{
		Project:   project,
		Tasks:     tasks,
		Worktrees: worktrees,
		Sessions:  sessions,
	})
}

func (h *handler) updateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := h.projectRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}

	var req updateProjectRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.RepoPath != nil {
		project.RepoPath = *req.RepoPath
	}
	if req.Playbook != nil {
		project.Playbook = *req.Playbook
	}
	if req.Status != nil {
		project.Status = *req.Status
	}

	if project.Name == "" || project.RepoPath == "" {
		jsonError(w, http.StatusBadRequest, "name and repo_path cannot be empty")
		return
	}

	if err := h.projectRepo.Update(r.Context(), project); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, project)
}

func (h *handler) deleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	project, err := h.projectRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if project == nil {
		jsonError(w, http.StatusNotFound, "project not found")
		return
	}

	project.Status = "archived"
	if err := h.projectRepo.Update(r.Context(), project); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusNoContent, nil)
}
