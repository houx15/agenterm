package api

import (
	"net/http"
	"testing"
)

func TestPermissionTemplateCRUD(t *testing.T) {
	h, _ := openAPI(t, &fakeGateway{})

	// Create — missing agent_type.
	badCreate := apiRequest(t, h, http.MethodPost, "/api/permission-templates", map[string]any{
		"name":   "Default",
		"config": `{"allow": true}`,
	}, true)
	if badCreate.Code != http.StatusBadRequest {
		t.Fatalf("bad create status=%d want %d", badCreate.Code, http.StatusBadRequest)
	}

	// Create — missing name.
	badName := apiRequest(t, h, http.MethodPost, "/api/permission-templates", map[string]any{
		"agent_type": "claude",
		"config":     `{"allow": true}`,
	}, true)
	if badName.Code != http.StatusBadRequest {
		t.Fatalf("bad name status=%d want %d", badName.Code, http.StatusBadRequest)
	}

	// Create — missing config.
	badConfig := apiRequest(t, h, http.MethodPost, "/api/permission-templates", map[string]any{
		"agent_type": "claude",
		"name":       "Default",
	}, true)
	if badConfig.Code != http.StatusBadRequest {
		t.Fatalf("bad config status=%d want %d", badConfig.Code, http.StatusBadRequest)
	}

	// Create — success.
	create := apiRequest(t, h, http.MethodPost, "/api/permission-templates", map[string]any{
		"agent_type": "claude",
		"name":       "Claude Default",
		"config":     `{"permissions": {"allow_all": true}}`,
	}, true)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status=%d body=%s", create.Code, create.Body.String())
	}
	var tmpl map[string]any
	decodeBody(t, create, &tmpl)
	tmplID := tmpl["id"].(string)
	if tmpl["agent_type"] != "claude" {
		t.Fatalf("agent_type=%v want claude", tmpl["agent_type"])
	}
	if tmpl["name"] != "Claude Default" {
		t.Fatalf("name=%v want Claude Default", tmpl["name"])
	}

	// Create second template.
	create2 := apiRequest(t, h, http.MethodPost, "/api/permission-templates", map[string]any{
		"agent_type": "codex",
		"name":       "Codex Default",
		"config":     "allow read\nallow write",
	}, true)
	if create2.Code != http.StatusCreated {
		t.Fatalf("create2 status=%d body=%s", create2.Code, create2.Body.String())
	}

	// List all.
	list := apiRequest(t, h, http.MethodGet, "/api/permission-templates", nil, true)
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var listed []map[string]any
	decodeBody(t, list, &listed)
	if len(listed) != 2 {
		t.Fatalf("listed len=%d want 2", len(listed))
	}

	// List by agent type.
	listClaude := apiRequest(t, h, http.MethodGet, "/api/permission-templates/claude", nil, true)
	if listClaude.Code != http.StatusOK {
		t.Fatalf("list claude status=%d body=%s", listClaude.Code, listClaude.Body.String())
	}
	var claudeTemplates []map[string]any
	decodeBody(t, listClaude, &claudeTemplates)
	if len(claudeTemplates) != 1 {
		t.Fatalf("claude templates len=%d want 1", len(claudeTemplates))
	}
	if claudeTemplates[0]["name"] != "Claude Default" {
		t.Fatalf("claude template name=%v want Claude Default", claudeTemplates[0]["name"])
	}

	// Update.
	update := apiRequest(t, h, http.MethodPut, "/api/permission-templates/"+tmplID, map[string]any{
		"name": "Claude Updated",
	}, true)
	if update.Code != http.StatusOK {
		t.Fatalf("update status=%d body=%s", update.Code, update.Body.String())
	}
	var updated map[string]any
	decodeBody(t, update, &updated)
	if updated["name"] != "Claude Updated" {
		t.Fatalf("updated name=%v want Claude Updated", updated["name"])
	}

	// Update with empty name.
	badUpdate := apiRequest(t, h, http.MethodPut, "/api/permission-templates/"+tmplID, map[string]any{
		"name": "   ",
	}, true)
	if badUpdate.Code != http.StatusBadRequest {
		t.Fatalf("empty name update status=%d want %d", badUpdate.Code, http.StatusBadRequest)
	}

	// Update non-existent.
	updateMissing := apiRequest(t, h, http.MethodPut, "/api/permission-templates/nonexistent", map[string]any{
		"name": "Test",
	}, true)
	if updateMissing.Code != http.StatusNotFound {
		t.Fatalf("update missing status=%d want %d", updateMissing.Code, http.StatusNotFound)
	}

	// Delete.
	del := apiRequest(t, h, http.MethodDelete, "/api/permission-templates/"+tmplID, nil, true)
	if del.Code != http.StatusNoContent {
		t.Fatalf("delete status=%d body=%s", del.Code, del.Body.String())
	}

	// Delete non-existent.
	delMissing := apiRequest(t, h, http.MethodDelete, "/api/permission-templates/nonexistent", nil, true)
	if delMissing.Code != http.StatusNotFound {
		t.Fatalf("delete missing status=%d want %d", delMissing.Code, http.StatusNotFound)
	}

	// Verify deletion — list should have 1.
	listAfter := apiRequest(t, h, http.MethodGet, "/api/permission-templates", nil, true)
	if listAfter.Code != http.StatusOK {
		t.Fatalf("list after delete status=%d body=%s", listAfter.Code, listAfter.Body.String())
	}
	var listedAfter []map[string]any
	decodeBody(t, listAfter, &listedAfter)
	if len(listedAfter) != 1 {
		t.Fatalf("listed after delete len=%d want 1", len(listedAfter))
	}
}
