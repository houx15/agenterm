package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/user/agenterm/internal/db"
	"github.com/user/agenterm/internal/registry"
)

// fakeGateway is kept as a no-op placeholder for test helpers that previously
// required a gateway argument. The type is now unused by the production code
// but maintained here to keep test signatures stable.
type fakeGateway struct{}

func openAPI(t *testing.T, _ *fakeGateway) (http.Handler, *db.DB) {
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
	return NewRouter(database.SQL(), nil, nil, "test-token", agentRegistry), database
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

func TestListDirectoriesDefaultsToHome(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	resp := apiRequest(t, h, http.MethodGet, "/api/fs/directories", nil, true)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}

	var body map[string]any
	decodeBody(t, resp, &body)
	path, _ := body["path"].(string)
	if path == "" || !filepath.IsAbs(path) {
		t.Fatalf("expected absolute path, got=%q", path)
	}
}

func TestListDirectoriesReturnsChildrenWithAbsolutePaths(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "alpha"), 0o755); err != nil {
		t.Fatalf("mkdir alpha: %v", err)
	}
	if err := os.Mkdir(filepath.Join(root, "beta"), 0o755); err != nil {
		t.Fatalf("mkdir beta: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "file.txt"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	resp := apiRequest(t, h, http.MethodGet, "/api/fs/directories?path="+url.QueryEscape(root), nil, true)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}

	var body struct {
		Path        string `json:"path"`
		Parent      string `json:"parent"`
		Directories []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"directories"`
	}
	decodeBody(t, resp, &body)

	if body.Path != filepath.Clean(root) {
		t.Fatalf("path=%q want %q", body.Path, filepath.Clean(root))
	}
	if body.Parent == "" {
		t.Fatalf("expected parent path to be set")
	}
	if len(body.Directories) != 2 {
		t.Fatalf("directories=%d want 2", len(body.Directories))
	}
	for _, dir := range body.Directories {
		if !filepath.IsAbs(dir.Path) {
			t.Fatalf("expected absolute child path, got=%q", dir.Path)
		}
		if dir.Name != "alpha" && dir.Name != "beta" {
			t.Fatalf("unexpected dir name=%q", dir.Name)
		}
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

func TestSessionCreationRequiresLifecycleManager(t *testing.T) {
	// TODO: Re-enable with PTY backend integration test once lifecycle wiring is stable.
	// Without a lifecycle manager, session creation returns 501.
	h, _ := openAPI(t, &fakeGateway{})

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
	if createSession.Code != http.StatusNotImplemented {
		t.Fatalf("create session without lifecycle status=%d want 501 body=%s", createSession.Code, createSession.Body.String())
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
	h, database := openAPI(t, gw)
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

	// Seed session directly in DB since lifecycle manager is nil in tests.
	sessionRepo := db.NewSessionRepo(database.SQL())
	sess := &db.Session{
		TaskID:          taskID2,
		TmuxSessionName: "test-merge-conflict",
		TmuxWindowID:    "test-merge-conflict",
		AgentType:       "codex",
		Role:            "coder",
		Status:          "working",
	}
	if err := sessionRepo.Create(context.Background(), sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

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
	if resolveBody["session_id"] != sess.ID {
		t.Fatalf("resolve session_id=%v want %v", resolveBody["session_id"], sess.ID)
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

// TODO: TestSessionCreateRollsBackWhenTmuxWindowFails removed — tmux fallback no longer exists.
// Equivalent test for PTY backend should be added once integration wiring is complete.

// TODO: TestSessionCreateRejectsUnknownAgentType removed — requires lifecycle manager wiring.
// Without lifecycle, session creation returns 501. Re-add as PTY integration test.

func TestSessionTakeoverAndIdleEndpoints(t *testing.T) {
	h, database := openAPI(t, &fakeGateway{})
	ctx := context.Background()

	projectRepo := db.NewProjectRepo(database.SQL())
	taskRepo := db.NewTaskRepo(database.SQL())
	sessionRepo := db.NewSessionRepo(database.SQL())

	project := &db.Project{Name: "P1", RepoPath: t.TempDir(), Status: "active"}
	if err := projectRepo.Create(ctx, project); err != nil {
		t.Fatalf("create project: %v", err)
	}
	task := &db.Task{ProjectID: project.ID, Title: "T1", Description: "D", Status: "pending"}
	if err := taskRepo.Create(ctx, task); err != nil {
		t.Fatalf("create task: %v", err)
	}
	sess := &db.Session{
		TaskID:          task.ID,
		TmuxSessionName: "test-term",
		TmuxWindowID:    "test-term",
		AgentType:       "codex",
		Role:            "coder",
		Status:          "working",
	}
	if err := sessionRepo.Create(ctx, sess); err != nil {
		t.Fatalf("create session: %v", err)
	}

	take := apiRequest(t, h, http.MethodPatch, "/api/sessions/"+sess.ID+"/takeover", map[string]any{
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

	idle := apiRequest(t, h, http.MethodGet, "/api/sessions/"+sess.ID+"/idle", nil, true)
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

// TODO: TestSessionCommandQueueEndpoints removed — requires lifecycle manager with PTY backend.
// Re-add as PTY integration test once lifecycle wiring is complete.

func TestSessionDeleteWithoutLifecycleManager(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})
	resp := apiRequest(t, h, http.MethodDelete, "/api/sessions/missing", nil, true)
	if resp.Code != http.StatusNotImplemented {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
}

// TODO: TestSessionOutputSinceFiltering removed — tmux capturePaneFn no longer exists.
// Re-add as PTY integration test once lifecycle and monitor wiring is complete.

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

func TestSessionCloseCheckGate(t *testing.T) {
	gw := &fakeGateway{}
	h, database := openAPI(t, gw)

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

	// Seed session directly in DB since lifecycle manager is nil in tests.
	sessionRepo := db.NewSessionRepo(database.SQL())
	sess := &db.Session{
		TaskID:          taskID,
		TmuxSessionName: "test-close-check",
		TmuxWindowID:    "test-close-check",
		AgentType:       "codex",
		Role:            "coder",
		Status:          "working",
	}
	if err := sessionRepo.Create(context.Background(), sess); err != nil {
		t.Fatalf("create session: %v", err)
	}
	sessionID := sess.ID

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

func TestProjectRunStateEndpoints(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})
	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "RunState", "repo_path": t.TempDir(),
	}, true)
	if createProject.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	current := apiRequest(t, h, http.MethodGet, "/api/projects/"+projectID+"/runs/current", nil, true)
	if current.Code != http.StatusOK {
		t.Fatalf("get current run status=%d body=%s", current.Code, current.Body.String())
	}
	var currentBody map[string]any
	decodeBody(t, current, &currentBody)
	run := currentBody["run"].(map[string]any)
	runID := run["id"].(string)
	if run["current_stage"] != "plan" {
		t.Fatalf("current_stage=%v want plan", run["current_stage"])
	}

	transition := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/runs/"+runID+"/transition", map[string]any{
		"to_stage": "build",
		"status":   "active",
		"evidence": map[string]any{"note": "entered build"},
	}, true)
	if transition.Code != http.StatusOK {
		t.Fatalf("transition run status=%d body=%s", transition.Code, transition.Body.String())
	}
	var transitionBody map[string]any
	decodeBody(t, transition, &transitionBody)
	updatedRun := transitionBody["run"].(map[string]any)
	if updatedRun["current_stage"] != "build" {
		t.Fatalf("current_stage=%v want build", updatedRun["current_stage"])
	}

	invalidTransition := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/runs/"+runID+"/transition", map[string]any{
		"to_stage": "plan",
		"status":   "active",
	}, true)
	if invalidTransition.Code != http.StatusConflict {
		t.Fatalf("invalid transition status=%d body=%s", invalidTransition.Code, invalidTransition.Body.String())
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
