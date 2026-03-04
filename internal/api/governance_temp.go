package api

import (
	"net/http"
)

// Temporary stubs for governance routes that will be removed in Step 6.

func (h *handler) listWorkflows(w http.ResponseWriter, r *http.Request) {
	jsonError(w, http.StatusServiceUnavailable, "workflow repo unavailable")
}

func (h *handler) createWorkflow(w http.ResponseWriter, r *http.Request) {
	jsonError(w, http.StatusServiceUnavailable, "workflow repo unavailable")
}

func (h *handler) updateWorkflow(w http.ResponseWriter, r *http.Request) {
	jsonError(w, http.StatusServiceUnavailable, "workflow repo unavailable")
}

func (h *handler) deleteWorkflow(w http.ResponseWriter, r *http.Request) {
	jsonError(w, http.StatusServiceUnavailable, "workflow repo unavailable")
}

func (h *handler) listProjectRoleBindings(w http.ResponseWriter, r *http.Request) {
	jsonError(w, http.StatusServiceUnavailable, "role binding repo unavailable")
}

func (h *handler) replaceProjectRoleBindings(w http.ResponseWriter, r *http.Request) {
	jsonError(w, http.StatusServiceUnavailable, "role binding repo unavailable")
}
