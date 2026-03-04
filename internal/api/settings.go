package api

import (
	"net/http"
)

type settingsResponse struct {
	OrchestratorLanguage string `json:"orchestrator_language"`
}

type settingsUpdateRequest struct {
	OrchestratorLanguage *string `json:"orchestrator_language,omitempty"`
}

func (h *handler) getSettings(w http.ResponseWriter, _ *http.Request) {
	jsonResponse(w, http.StatusOK, settingsResponse{
		OrchestratorLanguage: "en",
	})
}

func (h *handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	// Orchestrator language setting removed; return current settings.
	h.getSettings(w, r)
}
