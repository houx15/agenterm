package api

import (
	"net/http"
	"strings"

	"github.com/user/agenterm/internal/db"
)

type createPermissionTemplateRequest struct {
	AgentType string `json:"agent_type"`
	Name      string `json:"name"`
	Config    string `json:"config"`
}

type updatePermissionTemplateRequest struct {
	AgentType *string `json:"agent_type"`
	Name      *string `json:"name"`
	Config    *string `json:"config"`
}

func (h *handler) listPermissionTemplates(w http.ResponseWriter, r *http.Request) {
	templates, err := h.permissionTemplateRepo.List(r.Context())
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, templates)
}

func (h *handler) listPermissionTemplatesByAgent(w http.ResponseWriter, r *http.Request) {
	agentType := r.PathValue("agent_type")
	if strings.TrimSpace(agentType) == "" {
		jsonError(w, http.StatusBadRequest, "agent_type is required")
		return
	}
	templates, err := h.permissionTemplateRepo.ListByAgent(r.Context(), agentType)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, templates)
}

func (h *handler) createPermissionTemplate(w http.ResponseWriter, r *http.Request) {
	var req createPermissionTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if strings.TrimSpace(req.AgentType) == "" {
		jsonError(w, http.StatusBadRequest, "agent_type is required")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		jsonError(w, http.StatusBadRequest, "name is required")
		return
	}
	if strings.TrimSpace(req.Config) == "" {
		jsonError(w, http.StatusBadRequest, "config is required")
		return
	}

	tmpl := &db.PermissionTemplate{
		AgentType: strings.TrimSpace(req.AgentType),
		Name:      strings.TrimSpace(req.Name),
		Config:    strings.TrimSpace(req.Config),
	}
	if err := h.permissionTemplateRepo.Create(r.Context(), tmpl); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusCreated, tmpl)
}

func (h *handler) updatePermissionTemplate(w http.ResponseWriter, r *http.Request) {
	tmpl, ok := h.mustGetPermissionTemplate(w, r, r.PathValue("id"))
	if !ok {
		return
	}

	var req updatePermissionTemplateRequest
	if err := decodeJSON(r, &req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.AgentType != nil {
		tmpl.AgentType = strings.TrimSpace(*req.AgentType)
	}
	if req.Name != nil {
		tmpl.Name = strings.TrimSpace(*req.Name)
	}
	if req.Config != nil {
		tmpl.Config = strings.TrimSpace(*req.Config)
	}

	if tmpl.AgentType == "" {
		jsonError(w, http.StatusBadRequest, "agent_type cannot be empty")
		return
	}
	if tmpl.Name == "" {
		jsonError(w, http.StatusBadRequest, "name cannot be empty")
		return
	}

	if err := h.permissionTemplateRepo.Update(r.Context(), tmpl); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusOK, tmpl)
}

func (h *handler) deletePermissionTemplate(w http.ResponseWriter, r *http.Request) {
	tmpl, ok := h.mustGetPermissionTemplate(w, r, r.PathValue("id"))
	if !ok {
		return
	}
	if err := h.permissionTemplateRepo.Delete(r.Context(), tmpl.ID); err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonResponse(w, http.StatusNoContent, nil)
}

func (h *handler) mustGetPermissionTemplate(w http.ResponseWriter, r *http.Request, id string) (*db.PermissionTemplate, bool) {
	tmpl, err := h.permissionTemplateRepo.Get(r.Context(), id)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return nil, false
	}
	if tmpl == nil {
		jsonError(w, http.StatusNotFound, "permission template not found")
		return nil, false
	}
	return tmpl, true
}
