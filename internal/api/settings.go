package api

import (
	"net/http"
	"strings"
)

type settingsResponse struct {
	OrchestratorLanguage string `json:"orchestrator_language"`
}

type settingsUpdateRequest struct {
	OrchestratorLanguage *string `json:"orchestrator_language,omitempty"`
}

func (h *handler) getSettings(w http.ResponseWriter, _ *http.Request) {
	lang := "en"
	if h.orchestrator != nil {
		if l := h.orchestrator.UserLanguage(); l != "" {
			lang = l
		}
	}
	jsonResponse(w, http.StatusOK, settingsResponse{
		OrchestratorLanguage: lang,
	})
}

func (h *handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req settingsUpdateRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.OrchestratorLanguage != nil {
		lang := strings.TrimSpace(*req.OrchestratorLanguage)
		if lang == "" {
			lang = "en"
		}
		if h.orchestrator != nil {
			h.orchestrator.SetUserLanguage(lang)
		}
		if h.demandOrchestrator != nil {
			h.demandOrchestrator.SetUserLanguage(lang)
		}
	}

	// Return updated settings
	h.getSettings(w, r)
}
