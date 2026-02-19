package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/user/agenterm/internal/db"
	sessionpkg "github.com/user/agenterm/internal/session"
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

type sessionCloseCheckResponse struct {
	CanClose       bool           `json:"can_close"`
	Reason         string         `json:"reason"`
	ReviewVerdict  map[string]any `json:"review_verdict"`
	RequiredChecks map[string]any `json:"required_checks"`
}

type sessionOutputLine struct {
	Text      string    `json:"text"`
	Timestamp time.Time `json:"timestamp"`
}

type windowOutputState struct {
	snapshot []string
	entries  []sessionOutputLine
}

var capturePaneFn = capturePane

const maxSessionOutputEntries = 5000

func (h *handler) createSession(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("id")

	var req createSessionRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.AgentType == "" || req.Role == "" {
		jsonError(w, http.StatusBadRequest, "agent_type and role are required")
		return
	}

	if h.lifecycle != nil {
		sess, err := h.lifecycle.CreateSession(r.Context(), sessionpkg.CreateSessionRequest{
			TaskID:    taskID,
			AgentType: req.AgentType,
			Role:      req.Role,
		})
		if err != nil {
			status, msg := mapSessionError(err)
			jsonError(w, status, msg)
			return
		}
		jsonResponse(w, http.StatusCreated, sess)
		return
	}

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

	if h.registry != nil && h.registry.Get(req.AgentType) == nil {
		jsonError(w, http.StatusBadRequest, fmt.Sprintf("unknown agent type %q", req.AgentType))
		return
	}

	workingDir := project.RepoPath
	if task.WorktreeID != "" {
		if wt, err := h.worktreeRepo.Get(r.Context(), task.WorktreeID); err == nil && wt != nil {
			workingDir = wt.Path
		}
	}

	tmuxSessionName := h.tmuxSessionName()
	var windowID string

	if h.manager != nil {
		tmuxSessionName = buildTmuxSessionName(project.Name, task.Title, req.Role)
		gw, err := h.manager.CreateSession(tmuxSessionName, workingDir)
		if err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		windows := gw.ListWindows()
		if len(windows) == 0 {
			_ = h.manager.DestroySession(tmuxSessionName)
			jsonError(w, http.StatusInternalServerError, "created tmux session has no windows")
			return
		}
		windowID = windows[0].ID
	} else {
		if h.gw == nil {
			jsonError(w, http.StatusInternalServerError, "tmux gateway unavailable")
			return
		}
		windowName := "session-" + task.ID[:8]
		before := h.gw.ListWindows()
		if err := h.gw.NewWindow(windowName, workingDir); err != nil {
			jsonError(w, http.StatusBadRequest, err.Error())
			return
		}
		after := h.gw.ListWindows()
		windowID = findWindowID(before, after, windowName)
		if windowID == "" {
			windowID = findWindowID(nil, after, windowName)
		}
		if windowID == "" {
			jsonError(w, http.StatusInternalServerError, "failed to resolve tmux window id")
			return
		}
	}

	session := &db.Session{
		TaskID:          taskID,
		TmuxSessionName: tmuxSessionName,
		TmuxWindowID:    windowID,
		AgentType:       req.AgentType,
		Role:            req.Role,
		Status:          "running",
		HumanAttached:   false,
	}
	if err := h.sessionRepo.Create(r.Context(), session); err != nil {
		if h.manager != nil {
			_ = h.manager.DestroySession(tmuxSessionName)
		}
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
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
	var req sendCommandRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.Text == "" {
		jsonError(w, http.StatusBadRequest, "text is required")
		return
	}

	if h.lifecycle != nil {
		if err := h.lifecycle.SendCommand(r.Context(), r.PathValue("id"), req.Text); err != nil {
			status, msg := mapSessionError(err)
			jsonError(w, status, msg)
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "sent"})
		return
	}

	session, ok := h.mustGetSession(w, r)
	if !ok {
		return
	}
	workDir := h.resolveSessionWorkDir(r.Context(), session)
	if err := sessionpkg.ValidateCommandPolicy(req.Text, workDir); err != nil {
		if policyErr, ok := err.(*sessionpkg.CommandPolicyError); ok {
			sessionpkg.AuditCommandPolicyViolation(workDir, session.ID, req.Text, policyErr)
		}
		jsonError(w, http.StatusForbidden, err.Error())
		return
	}
	gw, err := h.gatewayForSession(session.TmuxSessionName)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, "tmux gateway unavailable")
		return
	}
	if session.TmuxWindowID == "" {
		jsonError(w, http.StatusBadRequest, "session has no tmux window")
		return
	}

	if err := gw.SendRaw(session.TmuxWindowID, req.Text); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	session.Status = "working"
	if err := h.sessionRepo.Update(r.Context(), session); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"status": "sent"})
}

func (h *handler) resolveSessionWorkDir(ctx context.Context, sess *db.Session) string {
	if h == nil || sess == nil || strings.TrimSpace(sess.TaskID) == "" {
		return ""
	}
	task, err := h.taskRepo.Get(ctx, sess.TaskID)
	if err != nil || task == nil {
		return ""
	}
	project, err := h.projectRepo.Get(ctx, task.ProjectID)
	if err != nil || project == nil {
		return ""
	}
	if strings.TrimSpace(task.WorktreeID) != "" {
		wt, err := h.worktreeRepo.Get(ctx, task.WorktreeID)
		if err == nil && wt != nil && strings.TrimSpace(wt.Path) != "" {
			return wt.Path
		}
	}
	return project.RepoPath
}

func (h *handler) getSessionOutput(w http.ResponseWriter, r *http.Request) {
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

	if h.lifecycle != nil {
		entries, err := h.lifecycle.GetOutput(r.Context(), r.PathValue("id"), since)
		if err != nil {
			status, msg := mapSessionError(err)
			jsonError(w, status, msg)
			return
		}
		result := make([]sessionOutputLine, 0, len(entries))
		for _, entry := range entries {
			result = append(result, sessionOutputLine{Text: entry.Text, Timestamp: entry.Timestamp})
		}
		if lines > 0 && len(result) > lines {
			result = result[len(result)-lines:]
		}
		jsonResponse(w, http.StatusOK, result)
		return
	}

	session, ok := h.mustGetSession(w, r)
	if !ok {
		return
	}
	if session.TmuxWindowID == "" {
		jsonResponse(w, http.StatusOK, []sessionOutputLine{})
		return
	}

	out, err := capturePaneFn(session.TmuxWindowID, lines)
	if err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}

	result := h.recordAndReadSessionOutput(session.TmuxWindowID, out, since, lines)
	jsonResponse(w, http.StatusOK, result)
}

func (h *handler) getSessionIdle(w http.ResponseWriter, r *http.Request) {
	session, ok := h.mustGetSession(w, r)
	if !ok {
		return
	}
	idle := session.Status == "idle" || session.Status == "waiting_review" || session.Status == "human_takeover"
	jsonResponse(w, http.StatusOK, map[string]any{
		"idle":          idle,
		"last_activity": session.LastActivityAt,
		"status":        session.Status,
	})
}

func (h *handler) getSessionCloseCheck(w http.ResponseWriter, r *http.Request) {
	session, ok := h.mustGetSession(w, r)
	if !ok {
		return
	}
	check, err := h.evaluateSessionCloseCheck(r.Context(), session)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, check)
}

func (h *handler) patchSessionTakeover(w http.ResponseWriter, r *http.Request) {
	var req patchTakeoverRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if h.lifecycle != nil {
		if err := h.lifecycle.SetTakeover(r.Context(), r.PathValue("id"), req.HumanTakeover); err != nil {
			status, msg := mapSessionError(err)
			jsonError(w, status, msg)
			return
		}
		session, ok := h.mustGetSession(w, r)
		if !ok {
			return
		}
		jsonResponse(w, http.StatusOK, session)
		return
	}

	session, ok := h.mustGetSession(w, r)
	if !ok {
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

func (h *handler) deleteSession(w http.ResponseWriter, r *http.Request) {
	if h.lifecycle == nil {
		jsonError(w, http.StatusNotImplemented, "session lifecycle manager unavailable")
		return
	}
	session, ok := h.mustGetSession(w, r)
	if !ok {
		return
	}
	check, err := h.evaluateSessionCloseCheck(r.Context(), session)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !check.CanClose {
		jsonResponse(w, http.StatusConflict, map[string]any{
			"error": "session close blocked by review gate",
			"gate":  check,
		})
		return
	}
	if err := h.lifecycle.DestroySession(r.Context(), session.ID); err != nil {
		status, msg := mapSessionError(err)
		jsonError(w, status, msg)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *handler) evaluateSessionCloseCheck(ctx context.Context, session *db.Session) (sessionCloseCheckResponse, error) {
	result := sessionCloseCheckResponse{
		CanClose: true,
		Reason:   "ok",
		ReviewVerdict: map[string]any{
			"status":              "not_applicable",
			"open_issues_total":   0,
			"latest_cycle_status": "",
			"latest_iteration":    0,
		},
		RequiredChecks: map[string]any{
			"task_completed":              true,
			"latest_review_cycle_passed":  true,
			"open_review_issues_zero":     true,
			"requires_strict_review_gate": false,
		},
	}
	if h == nil || session == nil || strings.TrimSpace(session.TaskID) == "" {
		return result, nil
	}
	task, err := h.taskRepo.Get(ctx, session.TaskID)
	if err != nil {
		return result, err
	}
	if task == nil {
		return result, nil
	}
	taskDone := isDoneStatus(task.Status)
	result.RequiredChecks["task_completed"] = taskDone

	role := strings.ToLower(strings.TrimSpace(session.Role))
	requiresStrict := role == "coder" || role == "reviewer" || role == "qa"
	result.RequiredChecks["requires_strict_review_gate"] = requiresStrict
	if !requiresStrict {
		return result, nil
	}

	if h.reviewRepo == nil {
		result.CanClose = taskDone
		result.RequiredChecks["latest_review_cycle_passed"] = false
		result.Reason = "review data unavailable and task is not completed"
		return result, nil
	}

	cycles, err := h.reviewRepo.ListCyclesByTask(ctx, task.ID)
	if err != nil {
		return result, err
	}
	latestStatus := ""
	latestIteration := 0
	if len(cycles) > 0 {
		latest := cycles[len(cycles)-1]
		latestStatus = strings.ToLower(strings.TrimSpace(latest.Status))
		latestIteration = latest.Iteration
	}

	openIssues, err := h.reviewRepo.CountOpenIssuesByTask(ctx, task.ID)
	if err != nil {
		return result, err
	}
	reviewPassed := latestStatus == "review_passed"

	result.ReviewVerdict = map[string]any{
		"status":              closeGateReviewStatus(latestStatus, openIssues),
		"open_issues_total":   openIssues,
		"latest_cycle_status": latestStatus,
		"latest_iteration":    latestIteration,
	}
	result.RequiredChecks["latest_review_cycle_passed"] = reviewPassed
	result.RequiredChecks["open_review_issues_zero"] = openIssues == 0

	if taskDone {
		return result, nil
	}
	if !reviewPassed || openIssues > 0 {
		result.CanClose = false
		result.Reason = "task is not completed and review gate has not passed"
	}
	return result, nil
}

func isDoneStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done", "completed":
		return true
	default:
		return false
	}
}

func closeGateReviewStatus(latestCycleStatus string, openIssues int) string {
	if openIssues > 0 {
		return "changes_requested"
	}
	switch latestCycleStatus {
	case "review_passed":
		return "pass"
	case "review_running", "review_pending":
		return "in_review"
	case "review_changes_requested":
		return "changes_requested"
	default:
		return "not_started"
	}
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

func (h *handler) tmuxSessionName() string {
	if h.tmuxSession != "" {
		return h.tmuxSession
	}
	return "agenterm"
}

func (h *handler) recordAndReadSessionOutput(windowID string, captured []string, since time.Time, lines int) []sessionOutputLine {
	filteredSnapshot := make([]string, 0, len(captured))
	for _, line := range captured {
		if strings.TrimSpace(line) != "" {
			filteredSnapshot = append(filteredSnapshot, line)
		}
	}

	h.outputMu.Lock()
	defer h.outputMu.Unlock()

	state, ok := h.outputState[windowID]
	if !ok {
		state = &windowOutputState{}
		h.outputState[windowID] = state
	}

	newLines := diffNewLines(state.snapshot, filteredSnapshot)
	baseTime := time.Now().UTC()
	for i, line := range newLines {
		state.entries = append(state.entries, sessionOutputLine{
			Text:      line,
			Timestamp: baseTime.Add(time.Duration(i) * time.Microsecond),
		})
	}
	if len(state.entries) > maxSessionOutputEntries {
		state.entries = state.entries[len(state.entries)-maxSessionOutputEntries:]
	}
	state.snapshot = filteredSnapshot

	filteredEntries := make([]sessionOutputLine, 0, len(state.entries))
	for _, entry := range state.entries {
		if since.IsZero() || entry.Timestamp.After(since) || entry.Timestamp.Equal(since) {
			filteredEntries = append(filteredEntries, entry)
		}
	}
	if lines > 0 && len(filteredEntries) > lines {
		filteredEntries = filteredEntries[len(filteredEntries)-lines:]
	}
	return filteredEntries
}

func (h *handler) gatewayForSession(sessionName string) (sessionGateway, error) {
	if h.manager != nil && sessionName != "" {
		gw, err := h.manager.GetGateway(sessionName)
		if err == nil {
			return gw, nil
		}
		return h.manager.AttachSession(sessionName)
	}
	if h.gw != nil {
		return h.gw, nil
	}
	return nil, fmt.Errorf("gateway unavailable")
}

func buildTmuxSessionName(projectName string, taskTitle string, role string) string {
	base := []string{
		slugPart(projectName),
		slugPart(taskTitle),
		slugPart(role),
	}
	for i := range base {
		if base[i] == "" {
			base[i] = "x"
		}
	}
	return strings.Join(base, "-")
}

func slugPart(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	var b strings.Builder
	lastDash := false
	for _, r := range v {
		isAlphaNum := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAlphaNum {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 36 {
		s = s[:36]
	}
	return s
}

func diffNewLines(previous []string, current []string) []string {
	if len(previous) == 0 {
		return current
	}
	maxOverlap := min(len(previous), len(current))
	for overlap := maxOverlap; overlap > 0; overlap-- {
		if slices.Equal(previous[len(previous)-overlap:], current[:overlap]) {
			return current[overlap:]
		}
	}
	return current
}

func mapSessionError(err error) (int, string) {
	if err == nil {
		return http.StatusOK, ""
	}
	switch {
	case sessionpkg.IsNotFound(err):
		return http.StatusNotFound, err.Error()
	case sessionpkg.IsCommandPolicyError(err):
		return http.StatusForbidden, err.Error()
	case strings.Contains(err.Error(), "required"), strings.Contains(err.Error(), "unknown agent type"):
		return http.StatusBadRequest, err.Error()
	default:
		return http.StatusInternalServerError, err.Error()
	}
}
