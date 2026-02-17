package api

import (
	"errors"
	"net/http"

	"github.com/user/agenterm/internal/playbook"
)

func (h *handler) listPlaybooks(w http.ResponseWriter, r *http.Request) {
	if h.playbookRegistry == nil {
		jsonError(w, http.StatusInternalServerError, "playbook registry unavailable")
		return
	}
	jsonResponse(w, http.StatusOK, h.playbookRegistry.List())
}

func (h *handler) getPlaybook(w http.ResponseWriter, r *http.Request) {
	if h.playbookRegistry == nil {
		jsonError(w, http.StatusInternalServerError, "playbook registry unavailable")
		return
	}
	pb := h.playbookRegistry.Get(r.PathValue("id"))
	if pb == nil {
		jsonError(w, http.StatusNotFound, "playbook not found")
		return
	}
	jsonResponse(w, http.StatusOK, pb)
}

func (h *handler) createPlaybook(w http.ResponseWriter, r *http.Request) {
	if h.playbookRegistry == nil {
		jsonError(w, http.StatusInternalServerError, "playbook registry unavailable")
		return
	}
	var req playbook.Playbook
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if h.playbookRegistry.Get(req.ID) != nil {
		jsonError(w, http.StatusConflict, "playbook already exists")
		return
	}
	if err := h.playbookRegistry.Save(&req); err != nil {
		jsonError(w, playbookStatusCode(err), err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, h.playbookRegistry.Get(req.ID))
}

func (h *handler) updatePlaybook(w http.ResponseWriter, r *http.Request) {
	if h.playbookRegistry == nil {
		jsonError(w, http.StatusInternalServerError, "playbook registry unavailable")
		return
	}
	id := r.PathValue("id")
	if h.playbookRegistry.Get(id) == nil {
		jsonError(w, http.StatusNotFound, "playbook not found")
		return
	}
	var req playbook.Playbook
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.ID != "" && req.ID != id {
		jsonError(w, http.StatusBadRequest, "playbook id in path and body must match")
		return
	}
	req.ID = id
	if err := h.playbookRegistry.Save(&req); err != nil {
		jsonError(w, playbookStatusCode(err), err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, h.playbookRegistry.Get(id))
}

func (h *handler) deletePlaybook(w http.ResponseWriter, r *http.Request) {
	if h.playbookRegistry == nil {
		jsonError(w, http.StatusInternalServerError, "playbook registry unavailable")
		return
	}
	id := r.PathValue("id")
	if h.playbookRegistry.Get(id) == nil {
		jsonError(w, http.StatusNotFound, "playbook not found")
		return
	}
	if err := h.playbookRegistry.Delete(id); err != nil {
		jsonError(w, playbookStatusCode(err), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func playbookStatusCode(err error) int {
	switch {
	case errors.Is(err, playbook.ErrInvalidPlaybook):
		return http.StatusBadRequest
	case errors.Is(err, playbook.ErrPlaybookNotFound):
		return http.StatusNotFound
	case errors.Is(err, playbook.ErrPlaybookStorage):
		return http.StatusInternalServerError
	default:
		return http.StatusInternalServerError
	}
}
