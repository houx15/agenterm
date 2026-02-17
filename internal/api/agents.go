package api

import (
	"errors"
	"net/http"
	"os"

	"github.com/user/agenterm/internal/db"
)

func (h *handler) listAgents(w http.ResponseWriter, r *http.Request) {
	if err := h.loadAgents(r); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	agents, err := h.agentRepo.List(r.Context(), db.AgentConfigFilter{})
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, agents)
}

func (h *handler) getAgent(w http.ResponseWriter, r *http.Request) {
	if err := h.loadAgents(r); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	agent, err := h.agentRepo.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if agent == nil {
		jsonError(w, http.StatusNotFound, "agent not found")
		return
	}
	jsonResponse(w, http.StatusOK, agent)
}

func (h *handler) loadAgents(r *http.Request) error {
	dir := defaultAgentsDir()
	if dir == "" {
		return nil
	}
	if _, err := os.Stat(dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	_, err := h.agentRepo.LoadFromYAMLDir(r.Context(), dir)
	return err
}
