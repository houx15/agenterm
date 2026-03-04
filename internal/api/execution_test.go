package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

// setupRequirementWithBlueprint creates a project, requirement, planning session,
// and saves a blueprint. Returns projectID, requirementID, and planningSessionID.
func setupRequirementWithBlueprint(t *testing.T, h http.Handler) (string, string, string) {
	t.Helper()

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "ExecProject", "repo_path": t.TempDir(),
	}, true)
	if createProject.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createReq := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements", map[string]any{
		"title":       "Exec Requirement",
		"description": "For execution testing",
	}, true)
	if createReq.Code != http.StatusCreated {
		t.Fatalf("create requirement status=%d body=%s", createReq.Code, createReq.Body.String())
	}
	var requirement map[string]any
	decodeBody(t, createReq, &requirement)
	requirementID := requirement["id"].(string)

	createPS := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/planning", nil, true)
	if createPS.Code != http.StatusCreated {
		t.Fatalf("create planning session status=%d body=%s", createPS.Code, createPS.Body.String())
	}
	var ps map[string]any
	decodeBody(t, createPS, &ps)
	psID := ps["id"].(string)

	blueprint := map[string]any{
		"tasks": []map[string]any{
			{
				"id":                  "bp-task-1",
				"title":               "Build backend",
				"description":         "Create API endpoints",
				"completion_criteria": []string{"Tests pass"},
				"worktree_branch":     "feature/backend",
				"agent_type":          "claude",
			},
			{
				"id":                  "bp-task-2",
				"title":               "Build frontend",
				"description":         "Create UI",
				"completion_criteria": []string{"Renders"},
				"worktree_branch":     "feature/frontend",
				"agent_type":          "codex",
				"depends_on":          []string{"bp-task-1"},
			},
		},
	}
	saveResp := apiRequest(t, h, http.MethodPost, "/api/planning-sessions/"+psID+"/blueprint", map[string]any{
		"blueprint": blueprint,
	}, true)
	if saveResp.Code != http.StatusOK {
		t.Fatalf("save blueprint status=%d body=%s", saveResp.Code, saveResp.Body.String())
	}

	return projectID, requirementID, psID
}

func TestLaunchExecution(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	_, requirementID, _ := setupRequirementWithBlueprint(t, h)

	// Launch execution.
	launch := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/launch", nil, true)
	if launch.Code != http.StatusOK {
		t.Fatalf("launch execution status=%d body=%s", launch.Code, launch.Body.String())
	}

	var resp map[string]any
	decodeBody(t, launch, &resp)

	// Check tasks were created.
	tasksRaw, _ := json.Marshal(resp["tasks"])
	var tasks []map[string]any
	_ = json.Unmarshal(tasksRaw, &tasks)
	if len(tasks) != 2 {
		t.Fatalf("tasks len=%d want 2", len(tasks))
	}
	if tasks[0]["title"] != "Build backend" {
		t.Fatalf("task 0 title=%v want Build backend", tasks[0]["title"])
	}
	if tasks[0]["requirement_id"] != requirementID {
		t.Fatalf("task 0 requirement_id=%v want %v", tasks[0]["requirement_id"], requirementID)
	}

	// Check worktrees were created.
	worktreesRaw, _ := json.Marshal(resp["worktrees"])
	var worktrees []map[string]any
	_ = json.Unmarshal(worktreesRaw, &worktrees)
	if len(worktrees) != 2 {
		t.Fatalf("worktrees len=%d want 2", len(worktrees))
	}

	// Check run was created.
	run, ok := resp["run"].(map[string]any)
	if !ok {
		t.Fatal("expected run in response")
	}
	if run["current_stage"] != "build" {
		t.Fatalf("run current_stage=%v want build", run["current_stage"])
	}
	if run["status"] != "active" {
		t.Fatalf("run status=%v want active", run["status"])
	}

	// Verify requirement status updated to "building".
	getReq := apiRequest(t, h, http.MethodGet, "/api/requirements/"+requirementID, nil, true)
	var updatedReq map[string]any
	decodeBody(t, getReq, &updatedReq)
	if updatedReq["status"] != "building" {
		t.Fatalf("requirement status=%v want building", updatedReq["status"])
	}
}

func TestLaunchExecutionRequiresReadyStatus(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	// Create a requirement that is still in "draft" status.
	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "NotReady", "repo_path": t.TempDir(),
	}, true)
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createReq := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements", map[string]any{
		"title": "Draft Req",
	}, true)
	var requirement map[string]any
	decodeBody(t, createReq, &requirement)
	requirementID := requirement["id"].(string)

	launch := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/launch", nil, true)
	if launch.Code != http.StatusConflict {
		t.Fatalf("launch draft status=%d want %d body=%s", launch.Code, http.StatusConflict, launch.Body.String())
	}
}

func TestLaunchExecutionNotFound(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	launch := apiRequest(t, h, http.MethodPost, "/api/requirements/nonexistent/launch", nil, true)
	if launch.Code != http.StatusNotFound {
		t.Fatalf("launch missing status=%d want %d", launch.Code, http.StatusNotFound)
	}
}

func TestTransitionStage(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	_, requirementID, _ := setupRequirementWithBlueprint(t, h)

	// Launch first.
	launch := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/launch", nil, true)
	if launch.Code != http.StatusOK {
		t.Fatalf("launch status=%d body=%s", launch.Code, launch.Body.String())
	}

	// Transition: build -> test (via "review" transition).
	transition := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/transition", map[string]any{
		"transition": "review",
	}, true)
	if transition.Code != http.StatusOK {
		t.Fatalf("transition status=%d body=%s", transition.Code, transition.Body.String())
	}
	var transResp map[string]any
	decodeBody(t, transition, &transResp)
	run := transResp["run"].(map[string]any)
	if run["current_stage"] != "test" {
		t.Fatalf("current_stage=%v want test", run["current_stage"])
	}

	// Transition: test -> done.
	doneTransition := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/transition", map[string]any{
		"transition": "done",
	}, true)
	if doneTransition.Code != http.StatusOK {
		t.Fatalf("done transition status=%d body=%s", doneTransition.Code, doneTransition.Body.String())
	}

	// Verify requirement status is "done".
	getReq := apiRequest(t, h, http.MethodGet, "/api/requirements/"+requirementID, nil, true)
	var doneReq map[string]any
	decodeBody(t, getReq, &doneReq)
	if doneReq["status"] != "done" {
		t.Fatalf("requirement status=%v want done", doneReq["status"])
	}
}

func TestTransitionStageInvalidTransition(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	_, requirementID, _ := setupRequirementWithBlueprint(t, h)

	// Launch first.
	launch := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/launch", nil, true)
	if launch.Code != http.StatusOK {
		t.Fatalf("launch status=%d body=%s", launch.Code, launch.Body.String())
	}

	// Invalid transition value.
	badTransition := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/transition", map[string]any{
		"transition": "invalid",
	}, true)
	if badTransition.Code != http.StatusBadRequest {
		t.Fatalf("invalid transition status=%d want %d", badTransition.Code, http.StatusBadRequest)
	}

	// Merge is invalid from build stage.
	mergeFromBuild := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/transition", map[string]any{
		"transition": "merge",
	}, true)
	if mergeFromBuild.Code != http.StatusConflict {
		t.Fatalf("merge from build status=%d want %d", mergeFromBuild.Code, http.StatusConflict)
	}
}

func TestTransitionStageNoActiveRun(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "NoRunProject", "repo_path": t.TempDir(),
	}, true)
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createReq := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements", map[string]any{
		"title": "No Run Req",
	}, true)
	var requirement map[string]any
	decodeBody(t, createReq, &requirement)
	requirementID := requirement["id"].(string)

	transition := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/transition", map[string]any{
		"transition": "review",
	}, true)
	if transition.Code != http.StatusNotFound {
		t.Fatalf("transition without run status=%d want %d body=%s", transition.Code, http.StatusNotFound, transition.Body.String())
	}
}
