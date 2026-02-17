package api

import (
	"net/http"
	"strings"

	"github.com/user/agenterm/internal/registry"
)

func (h *handler) listAgents(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	jsonResponse(w, http.StatusOK, h.registry.List())
}

func (h *handler) getAgent(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	agent := h.registry.Get(r.PathValue("id"))
	if agent == nil {
		jsonError(w, http.StatusNotFound, "agent not found")
		return
	}
	jsonResponse(w, http.StatusOK, agent)
}

func (h *handler) createAgent(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	var req registry.AgentConfig
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if h.registry.Get(req.ID) != nil {
		jsonError(w, http.StatusConflict, "agent already exists")
		return
	}
	if err := h.registry.Save(&req); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "write agent config") {
			status = http.StatusInternalServerError
		}
		jsonError(w, status, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, h.registry.Get(req.ID))
}

func (h *handler) updateAgent(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	id := r.PathValue("id")
	if h.registry.Get(id) == nil {
		jsonError(w, http.StatusNotFound, "agent not found")
		return
	}
	var req registry.AgentConfig
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ID != "" && req.ID != id {
		jsonError(w, http.StatusBadRequest, "agent id in path and body must match")
		return
	}
	req.ID = id
	if err := h.registry.Save(&req); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "write agent config") {
			status = http.StatusInternalServerError
		}
		jsonError(w, status, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, h.registry.Get(id))
}

func (h *handler) deleteAgent(w http.ResponseWriter, r *http.Request) {
	if h.registry == nil {
		jsonError(w, http.StatusInternalServerError, "agent registry unavailable")
		return
	}
	id := r.PathValue("id")
	if h.registry.Get(id) == nil {
		jsonError(w, http.StatusNotFound, "agent not found")
		return
	}
	if err := h.registry.Delete(id); err != nil {
		jsonError(w, http.StatusBadRequest, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
