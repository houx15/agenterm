package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/tmux"
)

type fakeGateway struct {
	windows        []tmux.Window
	nextID         int
	newWindowCalls int
	sentRaw        []string
	failNewWindow  bool
}

func (f *fakeGateway) NewWindow(name, defaultDir string) error {
	if f.failNewWindow {
		return errFakeGateway
	}
	f.newWindowCalls++
	f.nextID++
	f.windows = append(f.windows, tmux.Window{ID: "@" + strconv.Itoa(f.nextID), Name: name})
	return nil
}

func (f *fakeGateway) ListWindows() []tmux.Window {
	out := make([]tmux.Window, len(f.windows))
	copy(out, f.windows)
	return out
}

func (f *fakeGateway) SendKeys(windowID string, keys string) error {
	return nil
}

func (f *fakeGateway) SendRaw(windowID string, keys string) error {
	f.sentRaw = append(f.sentRaw, windowID+":"+keys)
	return nil
}

var errFakeGateway = &fakeGatewayError{"new window failed"}

type fakeGatewayError struct{ msg string }

func (e *fakeGatewayError) Error() string { return e.msg }

func openAPI(t *testing.T, gw gateway) (http.Handler, *db.DB) {
	t.Helper()
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return NewRouter(database.SQL(), gw, nil, nil, "test-token", "configured-session"), database
}

func openAPIWithManager(t *testing.T, gw gateway, manager sessionManager) (http.Handler, *db.DB) {
	t.Helper()
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return NewRouter(database.SQL(), gw, manager, nil, "test-token", "configured-session"), database
}

func apiRequest(t *testing.T, h http.Handler, method, path string, body any, auth bool) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(payload)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("Authorization", "Bearer test-token")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func decodeBody(t *testing.T, rr *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if rr.Body.Len() == 0 {
		return
	}
	if err := json.Unmarshal(rr.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode body: %v body=%s", err, rr.Body.String())
	}
}

func TestAuthMiddleware(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})
	unauth := apiRequest(t, h, http.MethodGet, "/api/projects", nil, false)
	if unauth.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d want %d", unauth.Code, http.StatusUnauthorized)
	}
	wrong := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	wrong.Header.Set("Authorization", "Bearer wrong-token")
	wrongRR := httptest.NewRecorder()
	h.ServeHTTP(wrongRR, wrong)
	if wrongRR.Code != http.StatusUnauthorized {
		t.Fatalf("wrong token status=%d want %d", wrongRR.Code, http.StatusUnauthorized)
	}
	auth := apiRequest(t, h, http.MethodGet, "/api/projects", nil, true)
	if auth.Code != http.StatusOK {
		t.Fatalf("status=%d want %d", auth.Code, http.StatusOK)
	}
}

func TestProjectAndTaskLifecycle(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	bad := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{"name": "x"}, true)
	if bad.Code != http.StatusBadRequest {
		t.Fatalf("bad create status=%d want %d", bad.Code, http.StatusBadRequest)
	}

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "P1", "repo_path": t.TempDir(),
	}, true)
	if createProject.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createTask := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/tasks", map[string]any{
		"title": "T1", "description": "D", "depends_on": []string{"task-a"},
	}, true)
	if createTask.Code != http.StatusCreated {
		t.Fatalf("create task status=%d body=%s", createTask.Code, createTask.Body.String())
	}
	var task map[string]any
	decodeBody(t, createTask, &task)
	taskID := task["id"].(string)

	getTask := apiRequest(t, h, http.MethodGet, "/api/tasks/"+taskID, nil, true)
	if getTask.Code != http.StatusOK {
		t.Fatalf("get task status=%d", getTask.Code)
	}

	archive := apiRequest(t, h, http.MethodDelete, "/api/projects/"+projectID, nil, true)
	if archive.Code != http.StatusNoContent {
		t.Fatalf("archive status=%d", archive.Code)
	}

	getProject := apiRequest(t, h, http.MethodGet, "/api/projects/"+projectID, nil, true)
	if getProject.Code != http.StatusOK {
		t.Fatalf("get project status=%d", getProject.Code)
	}
	var detail map[string]any
	decodeBody(t, getProject, &detail)
	p := detail["project"].(map[string]any)
	if p["status"] != "archived" {
		t.Fatalf("project status=%v want archived", p["status"])
	}
}

func TestSessionCreationCreatesTmuxWindow(t *testing.T) {
	gw := &fakeGateway{}
	h, _ := openAPI(t, gw)

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "P1", "repo_path": t.TempDir(),
	}, true)
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createTask := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/tasks", map[string]any{
		"title": "T1", "description": "D",
	}, true)
	var task map[string]any
	decodeBody(t, createTask, &task)
	taskID := task["id"].(string)

	createSession := apiRequest(t, h, http.MethodPost, "/api/tasks/"+taskID+"/sessions", map[string]any{
		"agent_type": "codex", "role": "coder",
	}, true)
	if createSession.Code != http.StatusCreated {
		t.Fatalf("create session status=%d body=%s", createSession.Code, createSession.Body.String())
	}
	if gw.newWindowCalls != 1 {
		t.Fatalf("newWindowCalls=%d want 1", gw.newWindowCalls)
	}

	var session map[string]any
	decodeBody(t, createSession, &session)
	if session["tmux_window_id"] == "" {
		t.Fatalf("expected tmux_window_id in response: %v", session)
	}
	if session["tmux_session_name"] != "configured-session" {
		t.Fatalf("tmux_session_name=%v want configured-session", session["tmux_session_name"])
	}
}

func TestSessionCreationUsesManagerWhenAvailable(t *testing.T) {
	if os.Getenv("TMUX_INTEGRATION_TEST") == "" {
		t.Skip("skipping integration test; set TMUX_INTEGRATION_TEST=1 to run")
	}

	gw := &fakeGateway{}
	manager := tmux.NewManager(t.TempDir())
	h, _ := openAPIWithManager(t, gw, manager)
	t.Cleanup(func() { manager.Close() })

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "My App", "repo_path": t.TempDir(),
	}, true)
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createTask := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/tasks", map[string]any{
		"title": "Auth Flow", "description": "D",
	}, true)
	var task map[string]any
	decodeBody(t, createTask, &task)
	taskID := task["id"].(string)

	createSession := apiRequest(t, h, http.MethodPost, "/api/tasks/"+taskID+"/sessions", map[string]any{
		"agent_type": "codex", "role": "coder",
	}, true)
	if createSession.Code != http.StatusCreated {
		t.Fatalf("create session status=%d body=%s", createSession.Code, createSession.Body.String())
	}

	if gw.newWindowCalls != 0 {
		t.Fatalf("expected legacy gateway NewWindow not to be used; calls=%d", gw.newWindowCalls)
	}

	var session map[string]any
	decodeBody(t, createSession, &session)
	tmuxSessionName, _ := session["tmux_session_name"].(string)
	if tmuxSessionName == "" {
		t.Fatalf("expected tmux_session_name in response: %v", session)
	}
	if !strings.Contains(tmuxSessionName, "my-app-auth-flow-coder") {
		t.Fatalf("tmux_session_name=%q want slugged project-task-role", tmuxSessionName)
	}

	if _, err := manager.GetGateway(tmuxSessionName); err != nil {
		t.Fatalf("expected manager to have attached gateway for %s: %v", tmuxSessionName, err)
	}

	if err := manager.DestroySession(tmuxSessionName); err != nil {
		t.Fatalf("cleanup destroy session failed: %v", err)
	}
}

func TestWorktreeGitEndpoints(t *testing.T) {
	gw := &fakeGateway{}
	h, _ := openAPI(t, gw)
	repo := initGitRepo(t)

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "Repo", "repo_path": repo,
	}, true)
	if createProject.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createTask := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/tasks", map[string]any{
		"title": "Task for worktree", "description": "D",
	}, true)
	if createTask.Code != http.StatusCreated {
		t.Fatalf("create task status=%d body=%s", createTask.Code, createTask.Body.String())
	}
	var task map[string]any
	decodeBody(t, createTask, &task)
	taskID := task["id"].(string)

	wtPath := filepath.Join(repo, ".worktrees", "t1")
	createWT := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/worktrees", map[string]any{
		"branch_name": "feature/t1",
		"path":        wtPath,
		"task_id":     taskID,
	}, true)
	if createWT.Code != http.StatusCreated {
		t.Fatalf("create worktree status=%d body=%s", createWT.Code, createWT.Body.String())
	}
	var wt map[string]any
	decodeBody(t, createWT, &wt)
	worktreeID := wt["id"].(string)

	status := apiRequest(t, h, http.MethodGet, "/api/worktrees/"+worktreeID+"/git-status", nil, true)
	if status.Code != http.StatusOK {
		t.Fatalf("git-status code=%d body=%s", status.Code, status.Body.String())
	}
	logResp := apiRequest(t, h, http.MethodGet, "/api/worktrees/"+worktreeID+"/git-log?n=1", nil, true)
	if logResp.Code != http.StatusOK {
		t.Fatalf("git-log code=%d body=%s", logResp.Code, logResp.Body.String())
	}

	del := apiRequest(t, h, http.MethodDelete, "/api/worktrees/"+worktreeID, nil, true)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete worktree code=%d body=%s", del.Code, del.Body.String())
	}

	taskAfterDelete := apiRequest(t, h, http.MethodGet, "/api/tasks/"+taskID, nil, true)
	if taskAfterDelete.Code != http.StatusOK {
		t.Fatalf("get task after delete status=%d body=%s", taskAfterDelete.Code, taskAfterDelete.Body.String())
	}
	var updatedTask map[string]any
	decodeBody(t, taskAfterDelete, &updatedTask)
	if v, ok := updatedTask["worktree_id"]; ok && v != "" {
		t.Fatalf("expected empty worktree_id after worktree delete, got=%v", v)
	}
}

func TestWorktreeRejectsPathOutsideProjectRepo(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})
	repo := initGitRepo(t)
	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "Repo", "repo_path": repo,
	}, true)
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	outside := filepath.Join(t.TempDir(), "outside-worktree")
	createWT := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/worktrees", map[string]any{
		"branch_name": "feature/outside",
		"path":        outside,
	}, true)
	if createWT.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", createWT.Code, createWT.Body.String())
	}
}

func TestSessionCreateRollsBackWhenTmuxWindowFails(t *testing.T) {
	gw := &fakeGateway{failNewWindow: true}
	h, _ := openAPI(t, gw)

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "P1", "repo_path": t.TempDir(),
	}, true)
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)
	createTask := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/tasks", map[string]any{
		"title": "T1", "description": "D",
	}, true)
	var task map[string]any
	decodeBody(t, createTask, &task)
	taskID := task["id"].(string)

	createSession := apiRequest(t, h, http.MethodPost, "/api/tasks/"+taskID+"/sessions", map[string]any{
		"agent_type": "codex", "role": "coder",
	}, true)
	if createSession.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", createSession.Code, createSession.Body.String())
	}
	list := apiRequest(t, h, http.MethodGet, "/api/sessions?task_id="+taskID, nil, true)
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d", list.Code)
	}
	var sessions []map[string]any
	decodeBody(t, list, &sessions)
	if len(sessions) != 0 {
		t.Fatalf("expected no persisted sessions after failure, got %d", len(sessions))
	}
}

func TestSessionOutputSinceFiltering(t *testing.T) {
	gw := &fakeGateway{}
	h, _ := openAPI(t, gw)

	origCapture := capturePaneFn
	defer func() { capturePaneFn = origCapture }()
	capturePaneFn = func(windowID string, lines int) ([]string, error) {
		return []string{"line-a", "line-b"}, nil
	}

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "P1", "repo_path": t.TempDir(),
	}, true)
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)
	createTask := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/tasks", map[string]any{
		"title": "T1", "description": "D",
	}, true)
	var task map[string]any
	decodeBody(t, createTask, &task)
	taskID := task["id"].(string)
	createSession := apiRequest(t, h, http.MethodPost, "/api/tasks/"+taskID+"/sessions", map[string]any{
		"agent_type": "codex", "role": "coder",
	}, true)
	var session map[string]any
	decodeBody(t, createSession, &session)
	sessionID := session["id"].(string)

	first := apiRequest(t, h, http.MethodGet, "/api/sessions/"+sessionID+"/output?lines=10", nil, true)
	if first.Code != http.StatusOK {
		t.Fatalf("first output status=%d body=%s", first.Code, first.Body.String())
	}
	var lines []map[string]any
	decodeBody(t, first, &lines)
	if len(lines) != 2 {
		t.Fatalf("len(lines)=%d want 2", len(lines))
	}
	lastTS := lines[len(lines)-1]["timestamp"].(string)

	second := apiRequest(t, h, http.MethodGet, "/api/sessions/"+sessionID+"/output?since="+lastTS+"&lines=10", nil, true)
	if second.Code != http.StatusOK {
		t.Fatalf("second output status=%d body=%s", second.Code, second.Body.String())
	}
	var lines2 []map[string]any
	decodeBody(t, second, &lines2)
	if len(lines2) != 1 {
		t.Fatalf("len(lines2)=%d want 1", len(lines2))
	}
}

func TestSessionAndAgentNotFoundErrors(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})
	if got := apiRequest(t, h, http.MethodGet, "/api/sessions/missing", nil, true).Code; got != http.StatusNotFound {
		t.Fatalf("session not found status=%d", got)
	}
	if got := apiRequest(t, h, http.MethodGet, "/api/agents/missing", nil, true).Code; got != http.StatusNotFound {
		t.Fatalf("agent not found status=%d", got)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()
	repo := filepath.Join(t.TempDir(), "repo")
	if err := os.MkdirAll(repo, 0o755); err != nil {
		t.Fatalf("mkdir repo: %v", err)
	}
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
		}
	}
	run("init")
	run("config", "user.email", "test@example.com")
	run("config", "user.name", "tester")
	if err := os.WriteFile(filepath.Join(repo, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "init")
	return repo
}
