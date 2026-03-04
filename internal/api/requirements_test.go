package api

import (
	"net/http"
	"testing"
)

func TestRequirementsCRUD(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	// Create project first.
	createProject := apiRequest(t, h, http.MethodPost, "/api/projects", map[string]any{
		"name": "ReqProject", "repo_path": t.TempDir(),
	}, true)
	if createProject.Code != http.StatusCreated {
		t.Fatalf("create project status=%d body=%s", createProject.Code, createProject.Body.String())
	}
	var project map[string]any
	decodeBody(t, createProject, &project)
	projectID := project["id"].(string)

	// Create requirement — missing title.
	badReq := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements", map[string]any{
		"description": "no title",
	}, true)
	if badReq.Code != http.StatusBadRequest {
		t.Fatalf("bad create status=%d want %d", badReq.Code, http.StatusBadRequest)
	}

	// Create requirement — invalid status.
	badStatus := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements", map[string]any{
		"title":  "R1",
		"status": "invalid",
	}, true)
	if badStatus.Code != http.StatusBadRequest {
		t.Fatalf("bad status create status=%d want %d", badStatus.Code, http.StatusBadRequest)
	}

	// Create requirement — success.
	create := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements", map[string]any{
		"title":       "Requirement 1",
		"description": "Description 1",
		"priority":    1,
		"status":      "draft",
	}, true)
	if create.Code != http.StatusCreated {
		t.Fatalf("create requirement status=%d body=%s", create.Code, create.Body.String())
	}
	var req1 map[string]any
	decodeBody(t, create, &req1)
	req1ID := req1["id"].(string)
	if req1["title"] != "Requirement 1" {
		t.Fatalf("title=%v want Requirement 1", req1["title"])
	}

	// Create second requirement with default status.
	create2 := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements", map[string]any{
		"title":    "Requirement 2",
		"priority": 2,
	}, true)
	if create2.Code != http.StatusCreated {
		t.Fatalf("create req2 status=%d body=%s", create2.Code, create2.Body.String())
	}
	var req2 map[string]any
	decodeBody(t, create2, &req2)
	req2ID := req2["id"].(string)
	if req2["status"] != "draft" {
		t.Fatalf("default status=%v want draft", req2["status"])
	}

	// List requirements.
	list := apiRequest(t, h, http.MethodGet, "/api/projects/"+projectID+"/requirements", nil, true)
	if list.Code != http.StatusOK {
		t.Fatalf("list requirements status=%d body=%s", list.Code, list.Body.String())
	}
	var listed []map[string]any
	decodeBody(t, list, &listed)
	if len(listed) != 2 {
		t.Fatalf("listed len=%d want 2", len(listed))
	}

	// Get requirement.
	get := apiRequest(t, h, http.MethodGet, "/api/requirements/"+req1ID, nil, true)
	if get.Code != http.StatusOK {
		t.Fatalf("get requirement status=%d body=%s", get.Code, get.Body.String())
	}
	var gotReq map[string]any
	decodeBody(t, get, &gotReq)
	if gotReq["title"] != "Requirement 1" {
		t.Fatalf("get title=%v want Requirement 1", gotReq["title"])
	}

	// Get non-existent requirement.
	getMissing := apiRequest(t, h, http.MethodGet, "/api/requirements/nonexistent", nil, true)
	if getMissing.Code != http.StatusNotFound {
		t.Fatalf("get missing status=%d want %d", getMissing.Code, http.StatusNotFound)
	}

	// Update requirement.
	update := apiRequest(t, h, http.MethodPatch, "/api/requirements/"+req1ID, map[string]any{
		"title":  "Updated Requirement 1",
		"status": "planning",
	}, true)
	if update.Code != http.StatusOK {
		t.Fatalf("update requirement status=%d body=%s", update.Code, update.Body.String())
	}
	var updated map[string]any
	decodeBody(t, update, &updated)
	if updated["title"] != "Updated Requirement 1" {
		t.Fatalf("updated title=%v want Updated Requirement 1", updated["title"])
	}
	if updated["status"] != "planning" {
		t.Fatalf("updated status=%v want planning", updated["status"])
	}

	// Update with empty title.
	badUpdate := apiRequest(t, h, http.MethodPatch, "/api/requirements/"+req1ID, map[string]any{
		"title": "   ",
	}, true)
	if badUpdate.Code != http.StatusBadRequest {
		t.Fatalf("empty title update status=%d want %d", badUpdate.Code, http.StatusBadRequest)
	}

	// Reorder requirements.
	reorder := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements/reorder", map[string]any{
		"ids": []string{req2ID, req1ID},
	}, true)
	if reorder.Code != http.StatusOK {
		t.Fatalf("reorder status=%d body=%s", reorder.Code, reorder.Body.String())
	}
	var reordered []map[string]any
	decodeBody(t, reorder, &reordered)
	if len(reordered) != 2 {
		t.Fatalf("reordered len=%d want 2", len(reordered))
	}
	// After reorder, req2 should come first (priority 0) and req1 second (priority 1).
	if reordered[0]["id"] != req2ID {
		t.Fatalf("first after reorder id=%v want %v", reordered[0]["id"], req2ID)
	}

	// Reorder with empty IDs.
	badReorder := apiRequest(t, h, http.MethodPost, "/api/projects/"+projectID+"/requirements/reorder", map[string]any{
		"ids": []string{},
	}, true)
	if badReorder.Code != http.StatusBadRequest {
		t.Fatalf("empty reorder status=%d want %d", badReorder.Code, http.StatusBadRequest)
	}

	// Delete requirement.
	del := apiRequest(t, h, http.MethodDelete, "/api/requirements/"+req1ID, nil, true)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", del.Code, del.Body.String())
	}

	// Verify deleted.
	getDeleted := apiRequest(t, h, http.MethodGet, "/api/requirements/"+req1ID, nil, true)
	if getDeleted.Code != http.StatusNotFound {
		t.Fatalf("get deleted status=%d want %d", getDeleted.Code, http.StatusNotFound)
	}

	// List after delete.
	listAfter := apiRequest(t, h, http.MethodGet, "/api/projects/"+projectID+"/requirements", nil, true)
	if listAfter.Code != http.StatusOK {
		t.Fatalf("list after delete status=%d body=%s", listAfter.Code, listAfter.Body.String())
	}
	var listedAfter []map[string]any
	decodeBody(t, listAfter, &listedAfter)
	if len(listedAfter) != 1 {
		t.Fatalf("listed after delete len=%d want 1", len(listedAfter))
	}
}

func TestRequirementsRequireValidProject(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	resp := apiRequest(t, h, http.MethodPost, "/api/projects/nonexistent/requirements", map[string]any{
		"title": "R1",
	}, true)
	if resp.Code != http.StatusNotFound {
		t.Fatalf("create requirement bad project status=%d want %d", resp.Code, http.StatusNotFound)
	}

	list := apiRequest(t, h, http.MethodGet, "/api/projects/nonexistent/requirements", nil, true)
	if list.Code != http.StatusNotFound {
		t.Fatalf("list requirements bad project status=%d want %d", list.Code, http.StatusNotFound)
	}
}
