package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/orchestrator"
	"github.com/user/agenterm/internal/playbook"
	"github.com/user/agenterm/internal/registry"
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
	agentRegistry, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	playbookRegistry, err := playbook.NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("new playbook registry: %v", err)
	}
	return NewRouter(database.SQL(), gw, nil, nil, nil, nil, nil, "test-token", "configured-session", agentRegistry, playbookRegistry), database
}

func openAPIWithManager(t *testing.T, gw gateway, manager sessionManager) (http.Handler, *db.DB) {
	t.Helper()
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	agentRegistry, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	playbookRegistry, err := playbook.NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("new playbook registry: %v", err)
	}
	return NewRouter(database.SQL(), gw, manager, nil, nil, nil, nil, "test-token", "configured-session", agentRegistry, playbookRegistry), database
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

func TestCreateProjectRollsBackWhenDefaultOrchestratorInitFails(t *testing.T) {
	h, database := openAPI(t, &fakeGateway{})
	ctx := context.Background()

	if _, err := database.SQL().ExecContext(ctx, `DELETE FROM workflow_phases`); err != nil {
		t.Fatalf("delete workflow_phases: %v", err)
	}
	if _, err := database.SQL().ExecContext(ctx, `DELETE FROM workflows`); err != nil {
		t.Fatalf("delete workflows: %v", err)
	}

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "P1", "repo_path": t.TempDir(),
	}, true)
	if createProject.Code != http.StatusInternalServerError {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}

	var count int
	if err := database.SQL().QueryRowContext(ctx, `SELECT count(1) FROM projects`).Scan(&count); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if count != 0 {
		t.Fatalf("project should be rolled back, count=%d", count)
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

func TestWorktreeMergeEndpointAndResolveConflict(t *testing.T) {
	gw := &fakeGateway{}
	h, _ := openAPI(t, gw)
	repo := initGitRepo(t)
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
		}
	}
	getBranch := func() string {
		cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git current branch failed: %v\n%s", err, string(out))
		}
		return strings.TrimSpace(string(out))
	}
	defaultBranch := getBranch()

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "Repo", "repo_path": repo,
	}, true)
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createTask := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/tasks", map[string]any{
		"title": "Task merge", "description": "D",
	}, true)
	var task map[string]any
	decodeBody(t, createTask, &task)
	taskID := task["id"].(string)

	wtPath := filepath.Join(repo, ".worktrees", "merge-task")
	createWT := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/worktrees", map[string]any{
		"branch_name": "feature/merge-task",
		"path":        wtPath,
		"task_id":     taskID,
	}, true)
	if createWT.Code != http.StatusCreated {
		t.Fatalf("create worktree status=%d body=%s", createWT.Code, createWT.Body.String())
	}
	var wt map[string]any
	decodeBody(t, createWT, &wt)
	worktreeID := wt["id"].(string)

	// Create mergeable change in worktree branch.
	if err := os.WriteFile(filepath.Join(wtPath, "feature.txt"), []byte("feature\n"), 0o644); err != nil {
		t.Fatalf("write feature file: %v", err)
	}
	runWT := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = wtPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git(wt) %v failed: %v\n%s", args, err, string(out))
		}
	}
	runWT("add", "feature.txt")
	runWT("commit", "-m", "feature work")

	mergeResp := apiRequest(t, h, http.MethodPost, "/api/worktrees/"+worktreeID+"/merge", map[string]any{
		"target_branch": defaultBranch,
	}, true)
	if mergeResp.Code != http.StatusOK {
		t.Fatalf("merge status=%d body=%s", mergeResp.Code, mergeResp.Body.String())
	}
	var mergeBody map[string]any
	decodeBody(t, mergeResp, &mergeBody)
	if mergeBody["status"] != "merged" {
		t.Fatalf("merge status=%v want merged", mergeBody["status"])
	}

	// Prepare base file for conflict scenario.
	if err := os.WriteFile(filepath.Join(repo, "shared.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatalf("write shared base: %v", err)
	}
	run("add", "shared.txt")
	run("commit", "-m", "base shared")

	createTask2 := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/tasks", map[string]any{
		"title": "Task conflict", "description": "D",
	}, true)
	var task2 map[string]any
	decodeBody(t, createTask2, &task2)
	taskID2 := task2["id"].(string)
	createWT2 := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/worktrees", map[string]any{
		"branch_name": "feature/conflict",
		"path":        filepath.Join(repo, ".worktrees", "conflict-task"),
		"task_id":     taskID2,
	}, true)
	if createWT2.Code != http.StatusCreated {
		t.Fatalf("create worktree2 status=%d body=%s", createWT2.Code, createWT2.Body.String())
	}
	var wt2 map[string]any
	decodeBody(t, createWT2, &wt2)
	worktreeID2 := wt2["id"].(string)
	wtPath2 := filepath.Join(repo, ".worktrees", "conflict-task")
	runWT2 := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = wtPath2
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git(wt2) %v failed: %v\n%s", args, err, string(out))
		}
	}
	if err := os.WriteFile(filepath.Join(wtPath2, "shared.txt"), []byte("feature-change\n"), 0o644); err != nil {
		t.Fatalf("write shared in wt2: %v", err)
	}
	runWT2("add", "shared.txt")
	runWT2("commit", "-m", "feature conflict")
	if err := os.WriteFile(filepath.Join(repo, "shared.txt"), []byte("main-change\n"), 0o644); err != nil {
		t.Fatalf("write shared in main: %v", err)
	}
	run("add", "shared.txt")
	run("commit", "-m", "main conflict")

	mergeConflict := apiRequest(t, h, http.MethodPost, "/api/worktrees/"+worktreeID2+"/merge", map[string]any{
		"target_branch": defaultBranch,
	}, true)
	if mergeConflict.Code != http.StatusOK {
		t.Fatalf("merge conflict status=%d body=%s", mergeConflict.Code, mergeConflict.Body.String())
	}
	var mergeConflictBody map[string]any
	decodeBody(t, mergeConflict, &mergeConflictBody)
	if mergeConflictBody["status"] != "conflict" {
		t.Fatalf("merge conflict status=%v want conflict", mergeConflictBody["status"])
	}

	createSession := apiRequest(t, h, http.MethodPost, "/api/tasks/"+taskID2+"/sessions", map[string]any{
		"agent_type": "codex", "role": "coder",
	}, true)
	if createSession.Code != http.StatusCreated {
		t.Fatalf("create coder session status=%d body=%s", createSession.Code, createSession.Body.String())
	}
	var session map[string]any
	decodeBody(t, createSession, &session)

	resolveResp := apiRequest(t, h, http.MethodPost, "/api/worktrees/"+worktreeID2+"/resolve-conflict", map[string]any{
		"message": "resolve and resubmit",
	}, true)
	if resolveResp.Code != http.StatusOK {
		t.Fatalf("resolve conflict status=%d body=%s", resolveResp.Code, resolveResp.Body.String())
	}
	var resolveBody map[string]any
	decodeBody(t, resolveResp, &resolveBody)
	if resolveBody["status"] != "resolution_requested" {
		t.Fatalf("resolve status=%v want resolution_requested", resolveBody["status"])
	}
	if resolveBody["session_id"] != session["id"] {
		t.Fatalf("resolve session_id=%v want %v", resolveBody["session_id"], session["id"])
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

func TestSessionCreateRejectsUnknownAgentType(t *testing.T) {
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
		"agent_type": "missing-agent", "role": "coder",
	}, true)
	if createSession.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", createSession.Code, createSession.Body.String())
	}
}

func TestSessionSendAndTakeoverEndpoints(t *testing.T) {
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
	var session map[string]any
	decodeBody(t, createSession, &session)
	sessionID := session["id"].(string)

	send := apiRequest(t, h, http.MethodPost, "/api/sessions/"+sessionID+"/send", map[string]any{
		"text": "echo hello\\n",
	}, true)
	if send.Code != http.StatusOK {
		t.Fatalf("send status=%d body=%s", send.Code, send.Body.String())
	}
	if len(gw.sentRaw) == 0 {
		t.Fatalf("expected command to be sent to gateway")
	}
	beforeBlocked := len(gw.sentRaw)
	blocked := apiRequest(t, h, http.MethodPost, "/api/sessions/"+sessionID+"/send", map[string]any{
		"text": "rm -rf /tmp/unsafe\\n",
	}, true)
	if blocked.Code != http.StatusForbidden {
		t.Fatalf("blocked send status=%d body=%s", blocked.Code, blocked.Body.String())
	}
	if len(gw.sentRaw) != beforeBlocked {
		t.Fatalf("blocked command should not be forwarded to gateway")
	}

	take := apiRequest(t, h, http.MethodPatch, "/api/sessions/"+sessionID+"/takeover", map[string]any{
		"human_takeover": true,
	}, true)
	if take.Code != http.StatusOK {
		t.Fatalf("takeover status=%d body=%s", take.Code, take.Body.String())
	}
	var takeoverSession map[string]any
	decodeBody(t, take, &takeoverSession)
	if takeoverSession["status"] != "human_takeover" {
		t.Fatalf("status=%v want human_takeover", takeoverSession["status"])
	}

	idle := apiRequest(t, h, http.MethodGet, "/api/sessions/"+sessionID+"/idle", nil, true)
	if idle.Code != http.StatusOK {
		t.Fatalf("idle status=%d body=%s", idle.Code, idle.Body.String())
	}
	var idleResp map[string]any
	decodeBody(t, idle, &idleResp)
	if v, ok := idleResp["idle"].(bool); !ok || v {
		t.Fatalf("expected idle=false for human_takeover, got=%v", idleResp["idle"])
	}
	if v, ok := idleResp["human_takeover"].(bool); !ok || !v {
		t.Fatalf("expected human_takeover=true, got=%v", idleResp["human_takeover"])
	}
}

func TestSessionDeleteWithoutLifecycleManager(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})
	resp := apiRequest(t, h, http.MethodDelete, "/api/sessions/missing", nil, true)
	if resp.Code != http.StatusNotImplemented {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
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

func TestAgentRegistryCRUDEndpoints(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	create := apiRequest(t, h, http.MethodPost, "/api/agents", map[string]any{
		"id":                  "custom-agent",
		"name":                "Custom Agent",
		"model":               "qwen3-coder",
		"command":             "custom run",
		"max_parallel_agents": 3,
		"notes":               "good at fast iteration",
	}, true)
	if create.Code != http.StatusCreated {
		t.Fatalf("create agent status=%d body=%s", create.Code, create.Body.String())
	}

	get := apiRequest(t, h, http.MethodGet, "/api/agents/custom-agent", nil, true)
	if get.Code != http.StatusOK {
		t.Fatalf("get agent status=%d body=%s", get.Code, get.Body.String())
	}
	var got map[string]any
	decodeBody(t, get, &got)
	if got["model"] != "qwen3-coder" {
		t.Fatalf("model=%v want qwen3-coder", got["model"])
	}

	update := apiRequest(t, h, http.MethodPut, "/api/agents/custom-agent", map[string]any{
		"name":                "Custom Agent Updated",
		"model":               "glm5",
		"command":             "custom run",
		"max_parallel_agents": 4,
	}, true)
	if update.Code != http.StatusOK {
		t.Fatalf("update agent status=%d body=%s", update.Code, update.Body.String())
	}

	del := apiRequest(t, h, http.MethodDelete, "/api/agents/custom-agent", nil, true)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete agent status=%d body=%s", del.Code, del.Body.String())
	}

	getMissing := apiRequest(t, h, http.MethodGet, "/api/agents/custom-agent", nil, true)
	if getMissing.Code != http.StatusNotFound {
		t.Fatalf("get deleted status=%d body=%s", getMissing.Code, getMissing.Body.String())
	}
}

func TestPlaybookCRUDEndpoints(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	create := apiRequest(t, h, http.MethodPost, "/api/playbooks", map[string]any{
		"id":          "custom-playbook",
		"name":        "Custom Playbook",
		"description": "test",
		"phases": []map[string]any{
			{"name": "Plan", "agent": "codex", "role": "planner", "description": "review scope"},
			{"name": "Ship", "agent": "claude-code", "role": "implementer", "description": "deliver feature"},
		},
	}, true)
	if create.Code != http.StatusCreated {
		t.Fatalf("create playbook status=%d body=%s", create.Code, create.Body.String())
	}

	get := apiRequest(t, h, http.MethodGet, "/api/playbooks/custom-playbook", nil, true)
	if get.Code != http.StatusOK {
		t.Fatalf("get playbook status=%d body=%s", get.Code, get.Body.String())
	}
	var got map[string]any
	decodeBody(t, get, &got)
	if got["name"] != "Custom Playbook" {
		t.Fatalf("name=%v want Custom Playbook", got["name"])
	}

	update := apiRequest(t, h, http.MethodPut, "/api/playbooks/custom-playbook", map[string]any{
		"name":        "Custom Playbook v2",
		"description": "updated",
		"phases": []map[string]any{
			{"name": "Implement", "agent": "codex", "role": "implementer", "description": "write code"},
		},
	}, true)
	if update.Code != http.StatusOK {
		t.Fatalf("update playbook status=%d body=%s", update.Code, update.Body.String())
	}

	list := apiRequest(t, h, http.MethodGet, "/api/playbooks", nil, true)
	if list.Code != http.StatusOK {
		t.Fatalf("list playbooks status=%d body=%s", list.Code, list.Body.String())
	}

	del := apiRequest(t, h, http.MethodDelete, "/api/playbooks/custom-playbook", nil, true)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete playbook status=%d body=%s", del.Code, del.Body.String())
	}

	getMissing := apiRequest(t, h, http.MethodGet, "/api/playbooks/custom-playbook", nil, true)
	if getMissing.Code != http.StatusNotFound {
		t.Fatalf("get deleted playbook status=%d body=%s", getMissing.Code, getMissing.Body.String())
	}
}

func TestDemandPoolEndpoints(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "Demand P1", "repo_path": t.TempDir(),
	}, true)
	if createProject.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createItem := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/demand-pool", map[string]any{
		"title":       "Add roadmap page",
		"description": "Need roadmap and prioritization",
		"status":      "captured",
		"priority":    3,
		"tags":        []string{"product", "ux"},
	}, true)
	if createItem.Code != http.StatusCreated {
		t.Fatalf("create demand item status=%d body=%s", createItem.Code, createItem.Body.String())
	}
	var item map[string]any
	decodeBody(t, createItem, &item)
	itemID := item["id"].(string)

	list := apiRequest(t, h, http.MethodGet, "/api/projects/"+projectID+"/demand-pool?status=captured", nil, true)
	if list.Code != http.StatusOK {
		t.Fatalf("list demand items status=%d body=%s", list.Code, list.Body.String())
	}
	var listed []map[string]any
	decodeBody(t, list, &listed)
	if len(listed) != 1 {
		t.Fatalf("listed len=%d want 1", len(listed))
	}

	update := apiRequest(t, h, http.MethodPatch, "/api/demand-pool/"+itemID, map[string]any{
		"status":   "triaged",
		"priority": 9,
	}, true)
	if update.Code != http.StatusOK {
		t.Fatalf("update demand item status=%d body=%s", update.Code, update.Body.String())
	}

	reprioritize := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/demand-pool/reprioritize", map[string]any{
		"items": []map[string]any{
			{"id": itemID, "priority": 7},
		},
	}, true)
	if reprioritize.Code != http.StatusOK {
		t.Fatalf("reprioritize status=%d body=%s", reprioritize.Code, reprioritize.Body.String())
	}

	promote := apiRequest(t, h, http.MethodPost, "/api/demand-pool/"+itemID+"/promote", map[string]any{
		"title":       "Roadmap feature task",
		"description": "Implement roadmap page",
		"status":      "pending",
		"depends_on":  []string{},
	}, true)
	if promote.Code != http.StatusOK {
		t.Fatalf("promote status=%d body=%s", promote.Code, promote.Body.String())
	}
	var promoted map[string]any
	decodeBody(t, promote, &promoted)
	taskRaw, ok := promoted["task"].(map[string]any)
	if !ok || taskRaw["id"] == "" {
		t.Fatalf("promoted task missing: %v", promoted)
	}

	del := apiRequest(t, h, http.MethodDelete, "/api/demand-pool/"+itemID, nil, true)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete demand item status=%d body=%s", del.Code, del.Body.String())
	}

	getMissing := apiRequest(t, h, http.MethodGet, "/api/demand-pool/"+itemID, nil, true)
	if getMissing.Code != http.StatusNotFound {
		t.Fatalf("get deleted demand item status=%d body=%s", getMissing.Code, getMissing.Body.String())
	}
}

func TestDemandOrchestratorReportEndpoint(t *testing.T) {
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	agentRegistry, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}
	playbookRegistry, err := playbook.NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("new playbook registry: %v", err)
	}
	demandOrchestratorInst := orchestrator.New(orchestrator.Options{Lane: "demand"})
	h := NewRouter(database.SQL(), &fakeGateway{}, nil, nil, nil, nil, demandOrchestratorInst, "test-token", "configured-session", agentRegistry, playbookRegistry)

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "Demand Report P1", "repo_path": t.TempDir(),
	}, true)
	if createProject.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	for _, payload := range []map[string]any{
		{"title": "Add release checklist", "status": "captured", "priority": 3},
		{"title": "Improve onboarding", "status": "shortlisted", "priority": 8},
	} {
		resp := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/demand-pool", payload, true)
		if resp.Code != http.StatusCreated {
			t.Fatalf("create demand item status=%d body=%s", resp.Code, resp.Body.String())
		}
	}

	report := apiRequest(t, h, http.MethodGet, "/api/demand-orchestrator/report?project_id="+projectID, nil, true)
	if report.Code != http.StatusOK {
		t.Fatalf("demand report status=%d body=%s", report.Code, report.Body.String())
	}
	var got map[string]any
	decodeBody(t, report, &got)
	if got["demand_items_total"] != float64(2) {
		t.Fatalf("demand_items_total=%v want 2", got["demand_items_total"])
	}
	counts, ok := got["demand_status_counts"].(map[string]any)
	if !ok {
		t.Fatalf("demand_status_counts type=%T", got["demand_status_counts"])
	}
	if counts["captured"] != float64(1) || counts["shortlisted"] != float64(1) {
		t.Fatalf("unexpected status counts: %+v", counts)
	}
}

func TestOrchestratorEndpoints(t *testing.T) {
	gw := &fakeGateway{}
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	agentRegistry, err := registry.NewRegistry(filepath.Join(t.TempDir(), "agents"))
	if err != nil {
		t.Fatalf("new registry: %v", err)
	}

	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	reviewRepo := db.NewReviewRepo(database.SQL())
	project := &db.Project{Name: "P", RepoPath: t.TempDir(), Status: "active"}
	if err := projectRepo.Create(context.Background(), project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{ProjectID: project.ID, Title: "Review T", Description: "D", Status: "running"}
	if err := taskRepo.Create(context.Background(), task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	cycle := &db.ReviewCycle{TaskID: task.ID, Status: "review_changes_requested", CommitHash: "abc123"}
	if err := reviewRepo.CreateCycle(context.Background(), cycle); err != nil {
		t.Fatalf("create review cycle: %v", err)
	}
	if err := reviewRepo.CreateIssue(context.Background(), &db.ReviewIssue{
		CycleID: cycle.ID, Summary: "fix style", Status: "open",
	}); err != nil {
		t.Fatalf("create review issue: %v", err)
	}

	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			if req.URL.Path == "/v1/messages" && req.Method == http.MethodPost {
				return mockJSONResponse(map[string]any{
					"content": []any{map[string]any{"type": "text", "text": "ok"}},
				}), nil
			}
			if req.URL.Path == "/api/projects/"+project.ID && req.Method == http.MethodGet {
				return mockJSONResponse(map[string]any{
					"project": map[string]any{"id": project.ID, "name": project.Name, "repo_path": project.RepoPath, "status": project.Status},
					"tasks": []any{
						map[string]any{"id": "t1", "status": "pending"},
						map[string]any{"id": "t2", "status": "blocked"},
					},
					"worktrees": []any{},
					"sessions": []any{
						map[string]any{"id": "s1", "status": "failed"},
					},
				}), nil
			}
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(`{"error":"not found"}`)),
			}, nil
		}),
	}

	orchestratorInst := orchestrator.New(orchestrator.Options{
		APIKey:           "test-key",
		AnthropicBaseURL: "http://mock/v1/messages",
		APIToolBaseURL:   "http://mock",
		HTTPClient:       httpClient,
		ProjectRepo:      projectRepo,
		TaskRepo:         taskRepo,
		WorktreeRepo:     db.NewWorktreeRepo(database.SQL()),
		SessionRepo:      db.NewSessionRepo(database.SQL()),
		HistoryRepo:      db.NewOrchestratorHistoryRepo(database.SQL()),
		Registry:         agentRegistry,
	})

	playbookRegistry, err := playbook.NewRegistry(filepath.Join(t.TempDir(), "playbooks"))
	if err != nil {
		t.Fatalf("new playbook registry: %v", err)
	}
	h := NewRouter(database.SQL(), gw, nil, nil, nil, orchestratorInst, nil, "test-token", "configured-session", agentRegistry, playbookRegistry)

	chat := apiRequest(t, h, http.MethodPost, "/api/orchestrator/chat", map[string]any{
		"project_id": project.ID,
		"message":    "hello",
	}, true)
	if chat.Code != http.StatusOK {
		t.Fatalf("chat status=%d body=%s", chat.Code, chat.Body.String())
	}

	history := apiRequest(t, h, http.MethodGet, "/api/orchestrator/history?project_id="+project.ID+"&limit=10", nil, true)
	if history.Code != http.StatusOK {
		t.Fatalf("history status=%d body=%s", history.Code, history.Body.String())
	}
	var historyItems []map[string]any
	decodeBody(t, history, &historyItems)
	if len(historyItems) < 2 {
		t.Fatalf("history len=%d want >=2", len(historyItems))
	}
	firstRole, _ := historyItems[0]["role"].(string)
	lastRole, _ := historyItems[len(historyItems)-1]["role"].(string)
	if firstRole != "user" {
		t.Fatalf("history first role=%q want user", firstRole)
	}
	if lastRole != "assistant" {
		t.Fatalf("history last role=%q want assistant", lastRole)
	}

	historyMissingProject := apiRequest(t, h, http.MethodGet, "/api/orchestrator/history", nil, true)
	if historyMissingProject.Code != http.StatusBadRequest {
		t.Fatalf("history missing project status=%d body=%s", historyMissingProject.Code, historyMissingProject.Body.String())
	}

	historyInvalidLimit := apiRequest(t, h, http.MethodGet, "/api/orchestrator/history?project_id="+project.ID+"&limit=0", nil, true)
	if historyInvalidLimit.Code != http.StatusBadRequest {
		t.Fatalf("history invalid limit status=%d body=%s", historyInvalidLimit.Code, historyInvalidLimit.Body.String())
	}

	report := apiRequest(t, h, http.MethodGet, "/api/orchestrator/report?project_id="+project.ID, nil, true)
	if report.Code != http.StatusOK {
		t.Fatalf("report status=%d body=%s", report.Code, report.Body.String())
	}
	var reportBody map[string]any
	decodeBody(t, report, &reportBody)
	if reportBody["phase"] != "blocked" {
		t.Fatalf("report phase=%v want blocked", reportBody["phase"])
	}
	if reportBody["queue_depth"] == nil {
		t.Fatalf("report missing queue_depth")
	}
	blockers, ok := reportBody["blockers"].([]any)
	if !ok || len(blockers) == 0 {
		t.Fatalf("report blockers=%T %#v want non-empty blockers list", reportBody["blockers"], reportBody["blockers"])
	}
	if reportBody["review_state"] != "changes_requested" {
		t.Fatalf("report review_state=%v want changes_requested", reportBody["review_state"])
	}
	reviewVerdict, ok := reportBody["review_verdict"].(map[string]any)
	if !ok {
		t.Fatalf("report review_verdict=%T want object", reportBody["review_verdict"])
	}
	if reviewVerdict["status"] != "changes_requested" {
		t.Fatalf("review_verdict.status=%v want changes_requested", reviewVerdict["status"])
	}
	requiredChecks, ok := reportBody["required_checks"].(map[string]any)
	if !ok {
		t.Fatalf("report required_checks=%T want object", reportBody["required_checks"])
	}
	if requiredChecks["finalize_ready"] != nil {
		t.Fatalf("required_checks should not contain finalize_ready directly")
	}
	if got, ok := reportBody["finalize_ready"].(bool); !ok || got {
		t.Fatalf("finalize_ready=%v want false", reportBody["finalize_ready"])
	}
	if reportBody["review_latest_iteration"] != float64(1) {
		t.Fatalf("report review_latest_iteration=%v want 1", reportBody["review_latest_iteration"])
	}
	summaries, ok := reportBody["review_task_summaries"].([]any)
	if !ok || len(summaries) == 0 {
		t.Fatalf("report review_task_summaries=%T %#v want non-empty", reportBody["review_task_summaries"], reportBody["review_task_summaries"])
	}
}

func TestSessionCloseCheckGate(t *testing.T) {
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
	var session map[string]any
	decodeBody(t, createSession, &session)
	sessionID := session["id"].(string)

	beforeReview := apiRequest(t, h, http.MethodGet, "/api/sessions/"+sessionID+"/close-check", nil, true)
	if beforeReview.Code != http.StatusOK {
		t.Fatalf("close-check before review status=%d body=%s", beforeReview.Code, beforeReview.Body.String())
	}
	var beforeBody map[string]any
	decodeBody(t, beforeReview, &beforeBody)
	if canClose, _ := beforeBody["can_close"].(bool); canClose {
		t.Fatalf("expected can_close=false before review pass")
	}

	cycleResp := apiRequest(t, h, http.MethodPost, "/api/tasks/"+taskID+"/review-cycles", map[string]any{
		"commit_hash": "abc123",
	}, true)
	if cycleResp.Code != http.StatusCreated {
		t.Fatalf("create review cycle status=%d body=%s", cycleResp.Code, cycleResp.Body.String())
	}
	var cycle map[string]any
	decodeBody(t, cycleResp, &cycle)
	cycleID := cycle["id"].(string)

	passCycle := apiRequest(t, h, http.MethodPatch, "/api/review-cycles/"+cycleID, map[string]any{
		"status": "review_passed",
	}, true)
	if passCycle.Code != http.StatusOK {
		t.Fatalf("pass cycle status=%d body=%s", passCycle.Code, passCycle.Body.String())
	}

	afterReview := apiRequest(t, h, http.MethodGet, "/api/sessions/"+sessionID+"/close-check", nil, true)
	if afterReview.Code != http.StatusOK {
		t.Fatalf("close-check after review status=%d body=%s", afterReview.Code, afterReview.Body.String())
	}
	var afterBody map[string]any
	decodeBody(t, afterReview, &afterBody)
	if canClose, _ := afterBody["can_close"].(bool); !canClose {
		t.Fatalf("expected can_close=true after review pass")
	}
}

func TestOrchestratorGovernanceEndpoints(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})
	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "Governance", "repo_path": t.TempDir(),
	}, true)
	if createProject.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	getProfile := apiRequest(t, h, http.MethodGet, "/api/projects/"+projectID+"/orchestrator", nil, true)
	if getProfile.Code != http.StatusOK {
		t.Fatalf("get profile status=%d body=%s", getProfile.Code, getProfile.Body.String())
	}

	updateProfile := apiRequest(t, h, http.MethodPatch, "/api/projects/"+projectID+"/orchestrator", map[string]any{
		"workflow_id":   "workflow-fast",
		"max_parallel":  2,
		"review_policy": "strict",
	}, true)
	if updateProfile.Code != http.StatusOK {
		t.Fatalf("update profile status=%d body=%s", updateProfile.Code, updateProfile.Body.String())
	}

	workflows := apiRequest(t, h, http.MethodGet, "/api/workflows", nil, true)
	if workflows.Code != http.StatusOK {
		t.Fatalf("list workflows status=%d body=%s", workflows.Code, workflows.Body.String())
	}

	createWorkflow := apiRequest(t, h, http.MethodPost, "/api/workflows", map[string]any{
		"id":          "workflow-test-custom",
		"name":        "Custom",
		"description": "custom flow",
		"scope":       "project",
		"phases": []map[string]any{
			{"ordinal": 1, "phase_type": "scan", "role": "planner", "max_parallel": 1},
		},
	}, true)
	if createWorkflow.Code != http.StatusCreated {
		t.Fatalf("create workflow status=%d body=%s", createWorkflow.Code, createWorkflow.Body.String())
	}
	createWorkflowInvalid := apiRequest(t, h, http.MethodPost, "/api/workflows", map[string]any{
		"id":          "workflow-test-invalid",
		"name":        "Invalid",
		"description": "invalid flow",
		"scope":       "project",
		"phases": []map[string]any{
			{"ordinal": 1, "phase_type": "unknown", "role": "nonexistent", "max_parallel": 1},
		},
	}, true)
	if createWorkflowInvalid.Code != http.StatusBadRequest {
		t.Fatalf("create invalid workflow status=%d body=%s", createWorkflowInvalid.Code, createWorkflowInvalid.Body.String())
	}

	updateWorkflow := apiRequest(t, h, http.MethodPut, "/api/workflows/workflow-test-custom", map[string]any{
		"name":        "Custom V2",
		"description": "updated",
		"scope":       "project",
		"version":     2,
		"phases": []map[string]any{
			{"ordinal": 1, "phase_type": "planning", "role": "planner", "max_parallel": 1},
		},
	}, true)
	if updateWorkflow.Code != http.StatusOK {
		t.Fatalf("update workflow status=%d body=%s", updateWorkflow.Code, updateWorkflow.Body.String())
	}

	deleteWorkflow := apiRequest(t, h, http.MethodDelete, "/api/workflows/workflow-test-custom", nil, true)
	if deleteWorkflow.Code != http.StatusNoContent {
		t.Fatalf("delete workflow status=%d body=%s", deleteWorkflow.Code, deleteWorkflow.Body.String())
	}

	createKnowledge := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/knowledge", map[string]any{
		"kind":       "design",
		"title":      "Project Design",
		"content":    "requirements and UX",
		"source_uri": "https://example.com/design",
	}, true)
	if createKnowledge.Code != http.StatusCreated {
		t.Fatalf("create knowledge status=%d body=%s", createKnowledge.Code, createKnowledge.Body.String())
	}

	listKnowledge := apiRequest(t, h, http.MethodGet, "/api/projects/"+projectID+"/knowledge", nil, true)
	if listKnowledge.Code != http.StatusOK {
		t.Fatalf("list knowledge status=%d body=%s", listKnowledge.Code, listKnowledge.Body.String())
	}

	replaceBindings := apiRequest(t, h, http.MethodPut, "/api/projects/"+projectID+"/role-bindings", map[string]any{
		"bindings": []map[string]any{
			{"role": "planner", "provider": "anthropic", "model": "claude-sonnet-4-5", "max_parallel": 1},
			{"role": "coder", "provider": "openai", "model": "gpt-5-codex", "max_parallel": 4},
		},
	}, true)
	if replaceBindings.Code != http.StatusOK {
		t.Fatalf("replace role bindings status=%d body=%s", replaceBindings.Code, replaceBindings.Body.String())
	}

	listBindings := apiRequest(t, h, http.MethodGet, "/api/projects/"+projectID+"/role-bindings", nil, true)
	if listBindings.Code != http.StatusOK {
		t.Fatalf("list role bindings status=%d body=%s", listBindings.Code, listBindings.Body.String())
	}

	createTask := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/tasks", map[string]any{
		"title": "Needs review", "description": "loop",
	}, true)
	if createTask.Code != http.StatusCreated {
		t.Fatalf("create task status=%d body=%s", createTask.Code, createTask.Body.String())
	}
	var task map[string]any
	decodeBody(t, createTask, &task)
	taskID := task["id"].(string)

	createCycle := apiRequest(t, h, http.MethodPost, "/api/tasks/"+taskID+"/review-cycles", map[string]any{
		"commit_hash": "abc123",
	}, true)
	if createCycle.Code != http.StatusCreated {
		t.Fatalf("create review cycle status=%d body=%s", createCycle.Code, createCycle.Body.String())
	}
	var cycle map[string]any
	decodeBody(t, createCycle, &cycle)
	cycleID := cycle["id"].(string)

	createIssue := apiRequest(t, h, http.MethodPost, "/api/review-cycles/"+cycleID+"/issues", map[string]any{
		"severity": "high",
		"summary":  "fix this",
	}, true)
	if createIssue.Code != http.StatusCreated {
		t.Fatalf("create review issue status=%d body=%s", createIssue.Code, createIssue.Body.String())
	}
	var issue map[string]any
	decodeBody(t, createIssue, &issue)
	issueID := issue["id"].(string)

	cyclesAfterCreateIssue := apiRequest(t, h, http.MethodGet, "/api/tasks/"+taskID+"/review-cycles", nil, true)
	if cyclesAfterCreateIssue.Code != http.StatusOK {
		t.Fatalf("list review cycles after issue status=%d body=%s", cyclesAfterCreateIssue.Code, cyclesAfterCreateIssue.Body.String())
	}
	var listedCycles []map[string]any
	decodeBody(t, cyclesAfterCreateIssue, &listedCycles)
	if len(listedCycles) == 0 {
		t.Fatalf("expected review cycle to be present")
	}
	if listedCycles[len(listedCycles)-1]["status"] != "review_changes_requested" {
		t.Fatalf("cycle status after issue create=%v want review_changes_requested", listedCycles[len(listedCycles)-1]["status"])
	}

	passWithOpenIssues := apiRequest(t, h, http.MethodPatch, "/api/review-cycles/"+cycleID, map[string]any{
		"status": "review_passed",
	}, true)
	if passWithOpenIssues.Code != http.StatusBadRequest {
		t.Fatalf("set review_passed with open issues status=%d body=%s", passWithOpenIssues.Code, passWithOpenIssues.Body.String())
	}

	completeBlocked := apiRequest(t, h, http.MethodPatch, "/api/tasks/"+taskID, map[string]any{
		"status": "done",
	}, true)
	if completeBlocked.Code != http.StatusConflict {
		t.Fatalf("complete with open review issue status=%d body=%s", completeBlocked.Code, completeBlocked.Body.String())
	}

	resolveIssue := apiRequest(t, h, http.MethodPatch, "/api/review-issues/"+issueID, map[string]any{
		"status":     "resolved",
		"resolution": "fixed in followup commit",
	}, true)
	if resolveIssue.Code != http.StatusOK {
		t.Fatalf("resolve review issue status=%d body=%s", resolveIssue.Code, resolveIssue.Body.String())
	}

	completeTask := apiRequest(t, h, http.MethodPatch, "/api/tasks/"+taskID, map[string]any{
		"status": "done",
	}, true)
	if completeTask.Code != http.StatusOK {
		t.Fatalf("complete task after resolving issues status=%d body=%s", completeTask.Code, completeTask.Body.String())
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

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func mockJSONResponse(v any) *http.Response {
	buf, _ := json.Marshal(v)
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(string(buf))),
	}
}
