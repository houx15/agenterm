package api

import (
	"net/http"
	"testing"
)

func TestPlanningSessionLifecycle(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	// Create project and requirement.
	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "PlanProject", "repo_path": t.TempDir(),
	}, true)
	if createProject.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createReq := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements", map[string]any{
		"title":       "Plan Requirement",
		"description": "Needs planning",
		"status":      "draft",
	}, true)
	if createReq.Code != http.StatusCreated {
		t.Fatalf("create requirement status=%d body=%s", createReq.Code, createReq.Body.String())
	}
	var requirement map[string]any
	decodeBody(t, createReq, &requirement)
	requirementID := requirement["id"].(string)

	// Create planning session.
	createPS := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/planning", nil, true)
	if createPS.Code != http.StatusCreated {
		t.Fatalf("create planning session status=%d body=%s", createPS.Code, createPS.Body.String())
	}
	var ps map[string]any
	decodeBody(t, createPS, &ps)
	psID := ps["id"].(string)
	if ps["status"] != "active" {
		t.Fatalf("planning session status=%v want active", ps["status"])
	}
	if ps["requirement_id"] != requirementID {
		t.Fatalf("requirement_id=%v want %v", ps["requirement_id"], requirementID)
	}

	// Verify requirement status updated to "planning".
	getReq := apiRequest(t, h, http.MethodGet, "/api/requirements/"+requirementID, nil, true)
	if getReq.Code != http.StatusOK {
		t.Fatalf("get requirement status=%d body=%s", getReq.Code, getReq.Body.String())
	}
	var updatedReq map[string]any
	decodeBody(t, getReq, &updatedReq)
	if updatedReq["status"] != "planning" {
		t.Fatalf("requirement status=%v want planning", updatedReq["status"])
	}

	// Get planning session by requirement.
	getPS := apiRequest(t, h, http.MethodGet, "/api/requirements/"+requirementID+"/planning", nil, true)
	if getPS.Code != http.StatusOK {
		t.Fatalf("get planning session status=%d body=%s", getPS.Code, getPS.Body.String())
	}
	var gotPS map[string]any
	decodeBody(t, getPS, &gotPS)
	if gotPS["id"] != psID {
		t.Fatalf("planning session id=%v want %v", gotPS["id"], psID)
	}

	// Get planning session for non-existent requirement.
	getMissing := apiRequest(t, h, http.MethodGet, "/api/requirements/nonexistent/planning", nil, true)
	if getMissing.Code != http.StatusNotFound {
		t.Fatalf("get missing planning session status=%d want %d", getMissing.Code, http.StatusNotFound)
	}

	// Update planning session — change status.
	updatePS := apiRequest(t, h, http.MethodPatch, "/api/planning-sessions/"+psID, map[string]any{
		"status": "completed",
	}, true)
	if updatePS.Code != http.StatusOK {
		t.Fatalf("update planning session status=%d body=%s", updatePS.Code, updatePS.Body.String())
	}
	var updatedPS map[string]any
	decodeBody(t, updatePS, &updatedPS)
	if updatedPS["status"] != "completed" {
		t.Fatalf("status=%v want completed", updatedPS["status"])
	}

	// Update with invalid status.
	badUpdate := apiRequest(t, h, http.MethodPatch, "/api/planning-sessions/"+psID, map[string]any{
		"status": "invalid",
	}, true)
	if badUpdate.Code != http.StatusBadRequest {
		t.Fatalf("bad update status=%d want %d", badUpdate.Code, http.StatusBadRequest)
	}

	// Update non-existent planning session.
	updateMissing := apiRequest(t, h, http.MethodPatch, "/api/planning-sessions/nonexistent", map[string]any{
		"status": "completed",
	}, true)
	if updateMissing.Code != http.StatusNotFound {
		t.Fatalf("update missing status=%d want %d", updateMissing.Code, http.StatusNotFound)
	}
}

func TestSaveBlueprintLifecycle(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	// Create project, requirement, and planning session.
	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "BlueprintProject", "repo_path": t.TempDir(),
	}, true)
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	createReq := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements", map[string]any{
		"title": "Blueprint Req",
	}, true)
	var requirement map[string]any
	decodeBody(t, createReq, &requirement)
	requirementID := requirement["id"].(string)

	createPS := apiRequest(t, h, http.MethodPost, "/api/requirements/"+requirementID+"/planning", nil, true)
	var ps map[string]any
	decodeBody(t, createPS, &ps)
	psID := ps["id"].(string)

	// Save blueprint with empty body — missing blueprint field.
	emptyBlueprint := apiRequest(t, h, http.MethodPost, "/api/planning-sessions/"+psID+"/blueprint", map[string]any{}, true)
	if emptyBlueprint.Code != http.StatusBadRequest {
		t.Fatalf("empty blueprint status=%d want %d", emptyBlueprint.Code, http.StatusBadRequest)
	}

	// Save blueprint successfully.
	blueprint := map[string]any{
		"tasks": []map[string]any{
			{
				"id":                  "task-1",
				"title":               "Build feature",
				"description":         "Implement the feature",
				"completion_criteria": []string{"Tests pass", "Code reviewed"},
				"worktree_branch":     "feature/build",
				"agent_type":          "claude",
			},
		},
	}
	saveResp := apiRequest(t, h, http.MethodPost, "/api/planning-sessions/"+psID+"/blueprint", map[string]any{
		"blueprint": blueprint,
	}, true)
	if saveResp.Code != http.StatusOK {
		t.Fatalf("save blueprint status=%d body=%s", saveResp.Code, saveResp.Body.String())
	}
	var savedPS map[string]any
	decodeBody(t, saveResp, &savedPS)
	if savedPS["status"] != "completed" {
		t.Fatalf("planning session status=%v want completed", savedPS["status"])
	}
	if savedPS["blueprint"] == nil || savedPS["blueprint"] == "" {
		t.Fatal("blueprint should not be empty after save")
	}

	// Verify requirement status updated to "ready".
	getReq := apiRequest(t, h, http.MethodGet, "/api/requirements/"+requirementID, nil, true)
	var updatedReq map[string]any
	decodeBody(t, getReq, &updatedReq)
	if updatedReq["status"] != "ready" {
		t.Fatalf("requirement status=%v want ready", updatedReq["status"])
	}

	// Save blueprint to non-existent planning session.
	saveMissing := apiRequest(t, h, http.MethodPost, "/api/planning-sessions/nonexistent/blueprint", map[string]any{
		"blueprint": blueprint,
	}, true)
	if saveMissing.Code != http.StatusNotFound {
		t.Fatalf("save missing status=%d want %d", saveMissing.Code, http.StatusNotFound)
	}
}
