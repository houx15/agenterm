package api

import (
	"bytes"
	"fmt"
	"net/http"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/tmux"
)

type createSessionRequest struct {
	AgentType string `json:"agent_type"`
	Role      string `json:"role"`
}

type sendCommandRequest struct {
	Text string `json:"text"`
}

type patchTakeoverRequest struct {
	HumanTakeover bool `json:"human_takeover"`
}

type sessionOutputLine struct {
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

func (h *handler) createSession(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")
	task, err := h.taskRepo.Get(r.Context(), taskID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if task == nil {
		jsonError(w, http.StatusNotFound, "task not found")
		return
	}
	project, err := h.projectRepo.Get(r.Context(), task.ProjectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if project == nil {
		jsonError(w, http.StatusBadRequest, "project for task not found")
		return
	}

	var req createSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.AgentType == "" || req.Role == "" {
		jsonError(w, http.StatusBadRequest, "agent_type and role are required")
		return
	}
	if h.gw == nil {
		jsonError(w, http.StatusInternalServerError, "tmux gateway unavailable")
		return
	}

	workingDir := project.RepoPath
	if task.WorktreeID != "" {
		if wt, err := h.worktreeRepo.Get(r.Context(), task.WorktreeID); err == nil && wt != nil {
			workingDir = wt.Path
		}
	}

	session := &db.Session{
		TaskID:          taskID,
		TmuxSessionName: "agenterm",
		AgentType:       req.AgentType,
		Role:            req.Role,
		Status:          "running",
		HumanAttached:   false,
	}
	if err := h.sessionRepo.Create(r.Context(), session); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	windowName := "session-" + session.ID[:8]
	before := h.gw.ListWindows()
	if err := h.gw.NewWindow(windowName, workingDir); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	after := h.gw.ListWindows()
	windowID := findWindowID(before, after, windowName)
	if windowID == "" {
		windowID = findWindowID(nil, after, windowName)
	}
	session.TmuxWindowID = windowID
	if err := h.sessionRepo.Update(r.Context(), session); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if session.TmuxWindowID != "" {
		_ = h.gw.SendKeys(session.TmuxWindowID, "cd "+shellQuotePath(workingDir)+"\n")
	}

	jsonResponse(w, http.StatusCreated, session)
}

func (h *handler) listSessions(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	taskID := r.URL.Query().Get("task_id")
	projectID := r.URL.Query().Get("project_id")

	if projectID == "" {
		sessions, err := h.sessionRepo.List(r.Context(), db.SessionFilter{TaskID: taskID, Status: status})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, sessions)
		return
	}

	tasks, err := h.taskRepo.ListByProject(r.Context(), projectID)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if taskID != "" {
		matched := false
		for _, t := range tasks {
			if t.ID == taskID {
				matched = true
				break
			}
		}
		if !matched {
			jsonResponse(w, http.StatusOK, []*db.Session{})
			return
		}
	}

	all := make([]*db.Session, 0)
	for _, t := range tasks {
		if taskID != "" && t.ID != taskID {
			continue
		}
		items, err := h.sessionRepo.List(r.Context(), db.SessionFilter{TaskID: t.ID, Status: status})
		if err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		all = append(all, items...)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	jsonResponse(w, http.StatusOK, all)
}

func (h *handler) getSession(w http.ResponseWriter, r *http.Request) {
	session, ok := h.mustGetSession(w, r)
	if !ok {
		return
	}
	jsonResponse(w, http.StatusOK, session)
}

func (h *handler) sendSessionCommand(w http.ResponseWriter, r *http.Request) {
	session, ok := h.mustGetSession(w, r)
	if !ok {
		return
	}
	if h.gw == nil {
		jsonError(w, http.StatusInternalServerError, "tmux gateway unavailable")
		return
	}
	if session.TmuxWindowID == "" {
		jsonError(w, http.StatusBadRequest, "session has no tmux window")
		return
	}

	var req sendCommandRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Text == "" {
		jsonError(w, http.StatusBadRequest, "text is required")
		return
	}

	if err := h.gw.SendRaw(session.TmuxWindowID, req.Text); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	session.Status = "running"
	if err := h.sessionRepo.Update(r.Context(), session); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *handler) getSessionOutput(w http.ResponseWriter, r *http.Request) {
	session, ok := h.mustGetSession(w, r)
	if !ok {
		return
	}
	if session.TmuxWindowID == "" {
		jsonResponse(w, http.StatusOK, []sessionOutputLine{})
		return
	}

	lines := 200
	if raw := r.URL.Query().Get("lines"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 {
			jsonError(w, http.StatusBadRequest, "invalid lines query parameter")
			return
		}
		if n > 2000 {
			n = 2000
		}
		lines = n
	}

	since, err := parseSince(r.URL.Query().Get("since"))
	if err != nil {
		jsonError(w, http.StatusBadRequest, "invalid since query parameter")
		return
	}

	out, err := capturePane(session.TmuxWindowID, lines)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	now := time.Now().UTC()
	if !since.IsZero() && since.After(now) {
		jsonResponse(w, http.StatusOK, []sessionOutputLine{})
		return
	}

	result := make([]sessionOutputLine, 0, len(out))
	for _, line := range out {
		if strings.TrimSpace(line) == "" {
			continue
		}
		result = append(result, sessionOutputLine{Text: line, Timestamp: now})
	}
	jsonResponse(w, http.StatusOK, result)
}

func (h *handler) getSessionIdle(w http.ResponseWriter, r *http.Request) {
	session, ok := h.mustGetSession(w, r)
	if !ok {
		return
	}
	idle := time.Since(session.LastActivityAt) > 30*time.Second || session.Status == "idle"
	jsonResponse(w, http.StatusOK, map[string]any{
		"idle":          idle,
		"last_activity": session.LastActivityAt,
		"status":        session.Status,
	})
}

func (h *handler) patchSessionTakeover(w http.ResponseWriter, r *http.Request) {
	session, ok := h.mustGetSession(w, r)
	if !ok {
		return
	}
	var req patchTakeoverRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	session.HumanAttached = req.HumanTakeover
	if req.HumanTakeover {
		session.Status = "human_takeover"
	} else {
		session.Status = "idle"
	}
	if err := h.sessionRepo.Update(r.Context(), session); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, session)
}

func (h *handler) mustGetSession(w http.ResponseWriter, r *http.Request) (*db.Session, bool) {
	id := r.PathValue("id")
	session, err := h.sessionRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if session == nil {
		jsonError(w, http.StatusNotFound, "session not found")
		return nil, false
	}
	return session, true
}

func findWindowID(before []tmux.Window, after []tmux.Window, windowName string) string {
	seen := make(map[string]struct{}, len(before))
	for _, w := range before {
		seen[w.ID] = struct{}{}
	}
	for _, w := range after {
		if w.Name == windowName {
			if len(before) == 0 {
				return w.ID
			}
			if _, ok := seen[w.ID]; !ok {
				return w.ID
			}
		}
	}
	return ""
}

func capturePane(windowID string, lines int) ([]string, error) {
	cmd := exec.Command("tmux", "capture-pane", "-p", "-t", windowID, "-S", fmt.Sprintf("-%d", lines))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if stderr.Len() > 0 {
			return nil, fmt.Errorf("tmux capture-pane failed: %s", strings.TrimSpace(stderr.String()))
		}
		return nil, fmt.Errorf("tmux capture-pane failed: %w", err)
	}
	return strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n"), nil
}

func parseSince(raw string) (time.Time, error) {
	if raw == "" {
		return time.Time{}, nil
	}
	if n, err := strconv.ParseInt(raw, 10, 64); err == nil {
		return time.Unix(n, 0).UTC(), nil
	}
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func shellQuotePath(path string) string {
	if strings.ContainsAny(path, " \t\n\"'") {
		return strconv.Quote(path)
	}
	return path
}
