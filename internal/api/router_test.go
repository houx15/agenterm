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
	"testing"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/tmux"
)

type fakeGateway struct {
	windows        []tmux.Window
	nextID         int
	newWindowCalls int
	sentRaw        []string
}

func (f *fakeGateway) NewWindow(name, defaultDir string) error {
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

func openAPI(t *testing.T, gw gateway) (http.Handler, *db.DB) {
	t.Helper()
	database, err := db.Open(context.Background(), filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	return NewRouter(database.SQL(), gw, nil, "test-token"), database
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

	wtPath := filepath.Join(repo, ".worktrees", "t1")
	createWT := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/worktrees", map[string]any{
		"branch_name": "feature/t1",
		"path":        wtPath,
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
