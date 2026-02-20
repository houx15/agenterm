package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/registry"
)

func (h *handler) listAgents(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	jsonResponse(w, http.StatusOK, h.registry.List())
}

type agentAssignment struct {
	SessionID      string `json:"session_id"`
	ProjectID      string `json:"project_id,omitempty"`
	ProjectName    string `json:"project_name,omitempty"`
	TaskID         string `json:"task_id,omitempty"`
	TaskTitle      string `json:"task_title,omitempty"`
	Role           string `json:"role"`
	Status         string `json:"status"`
	LastActivityAt string `json:"last_activity_at,omitempty"`
}

type agentRuntimeStatus struct {
	AgentID      string            `json:"agent_id"`
	AgentName    string            `json:"agent_name"`
	Capacity     int               `json:"capacity"`
	Assigned     int               `json:"assigned"`
	Orchestrator int               `json:"orchestrator"`
	Busy         int               `json:"busy"`
	Idle         int               `json:"idle"`
	Overflow     int               `json:"overflow"`
	Assignments  []agentAssignment `json:"assignments"`
}

type agentStatusResponse struct {
	TotalConfigured   int                  `json:"total_configured"`
	TotalCapacity     int                  `json:"total_capacity"`
	TotalBusy         int                  `json:"total_busy"`
	TotalAssigned     int                  `json:"total_assigned"`
	TotalOrchestrator int                  `json:"total_orchestrator"`
	TotalIdle         int                  `json:"total_idle"`
	Items             []agentRuntimeStatus `json:"items"`
}

func (h *handler) listAgentStatuses(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	agents := h.registry.List()
	sessions, err := h.sessionRepo.List(r.Context(), db.SessionFilter{})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	taskByID := map[string]*db.Task{}
	projectByID := map[string]*db.Project{}
	for _, session := range sessions {
		if session == nil || strings.TrimSpace(session.TaskID) == "" {
			continue
		}
		if _, ok := taskByID[session.TaskID]; !ok {
			task, err := h.taskRepo.Get(r.Context(), session.TaskID)
			if err == nil && task != nil {
				taskByID[session.TaskID] = task
			}
		}
		task := taskByID[session.TaskID]
		if task != nil && strings.TrimSpace(task.ProjectID) != "" {
			if _, ok := projectByID[task.ProjectID]; !ok {
				project, err := h.projectRepo.Get(r.Context(), task.ProjectID)
				if err == nil && project != nil {
					projectByID[task.ProjectID] = project
				}
			}
		}
	}

	assignmentsByAgent := map[string][]agentAssignment{}
	for _, session := range sessions {
		if session == nil || !isBusyAgentStatus(session.Status) {
			continue
		}
		agentID := strings.TrimSpace(session.AgentType)
		if agentID == "" {
			continue
		}
		item := agentAssignment{
			SessionID: session.ID,
			TaskID:    session.TaskID,
			Role:      session.Role,
			Status:    session.Status,
		}
		if !session.LastActivityAt.IsZero() {
			item.LastActivityAt = session.LastActivityAt.UTC().Format(time.RFC3339)
		}
		if task := taskByID[session.TaskID]; task != nil {
			item.ProjectID = task.ProjectID
			item.TaskTitle = task.Title
			if project := projectByID[task.ProjectID]; project != nil {
				item.ProjectName = project.Name
			}
		}
		assignmentsByAgent[agentID] = append(assignmentsByAgent[agentID], item)
	}

	resp := agentStatusResponse{
		TotalConfigured: len(agents),
		Items:           make([]agentRuntimeStatus, 0, len(agents)),
	}
	for _, agent := range agents {
		if agent == nil {
			continue
		}
		capacity := agent.MaxParallelAgents
		if capacity <= 0 {
			capacity = 1
		}
		assignments := assignmentsByAgent[agent.ID]
		sort.Slice(assignments, func(i, j int) bool {
			return assignments[i].LastActivityAt > assignments[j].LastActivityAt
		})
		orchestratorCount := 0
		for _, assignment := range assignments {
			if strings.EqualFold(strings.TrimSpace(assignment.Role), "orchestrator") {
				orchestratorCount++
			}
		}
		assignedCount := len(assignments) - orchestratorCount
		if assignedCount < 0 {
			assignedCount = 0
		}
		busy := len(assignments)
		idle := capacity - busy
		overflow := 0
		if idle < 0 {
			overflow = -idle
			idle = 0
		}
		resp.Items = append(resp.Items, agentRuntimeStatus{
			AgentID:      agent.ID,
			AgentName:    agent.Name,
			Capacity:     capacity,
			Assigned:     assignedCount,
			Orchestrator: orchestratorCount,
			Busy:         busy,
			Idle:         idle,
			Overflow:     overflow,
			Assignments:  assignments,
		})
		resp.TotalCapacity += capacity
		resp.TotalBusy += busy
		resp.TotalAssigned += assignedCount
		resp.TotalOrchestrator += orchestratorCount
	}
	resp.TotalIdle = resp.TotalCapacity - resp.TotalBusy
	if resp.TotalIdle < 0 {
		resp.TotalIdle = 0
	}
	sort.Slice(resp.Items, func(i, j int) bool {
		return resp.Items[i].AgentID < resp.Items[j].AgentID
	})
	jsonResponse(w, http.StatusOK, resp)
}

func isBusyAgentStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "idle", "disconnected", "sleeping", "paused", "completed", "failed", "stopped", "terminated", "closed", "dead":
		return false
	default:
		return true
	}
}

func (h *handler) getAgent(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	agent := h.registry.Get(r.PathValue("id"))
	if agent == nil {
		jsonError(w, http.StatusNotFound, "agent not found")
		return
	}
	jsonResponse(w, http.StatusOK, agent)
}

func (h *handler) createAgent(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	var req registry.AgentConfig
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if h.registry.Get(req.ID) != nil {
		jsonError(w, http.StatusConflict, "agent already exists")
		return
	}
	if err := h.registry.Save(&req); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "write agent config") {
			status = http.StatusInternalServerError
		}
		jsonError(w, status, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, h.registry.Get(req.ID))
}

func (h *handler) updateAgent(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	id := r.PathValue("id")
	if h.registry.Get(id) == nil {
		jsonError(w, http.StatusNotFound, "agent not found")
		return
	}
	var req registry.AgentConfig
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ID != "" && req.ID != id {
		jsonError(w, http.StatusBadRequest, "agent id in path and body must match")
		return
	}
	req.ID = id
	if err := h.registry.Save(&req); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "write agent config") {
			status = http.StatusInternalServerError
		}
		jsonError(w, status, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, h.registry.Get(id))
}

func (h *handler) deleteAgent(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	id := r.PathValue("id")
	if h.registry.Get(id) == nil {
		jsonError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err := h.registry.Delete(id); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
