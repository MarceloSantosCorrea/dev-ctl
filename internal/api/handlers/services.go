package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/marcelo/devctl/internal/core/project"
)

type ServiceHandler struct {
	svc *project.Service
}

func NewServiceHandler(svc *project.Service) *ServiceHandler {
	return &ServiceHandler{svc: svc}
}

func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "id")

	var input project.CreateServiceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.TemplateName == "" {
		writeError(w, http.StatusBadRequest, "template_name is required")
		return
	}

	svc, err := h.svc.AddService(r.Context(), projectID, input)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, svc)
}

func (h *ServiceHandler) Update(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "sid")

	var body struct {
		Name   string                 `json:"name"`
		Config map[string]interface{} `json:"config"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	svc, err := h.svc.UpdateService(r.Context(), sid, body.Name, body.Config)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, svc)
}

func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "sid")
	if err := h.svc.DeleteService(r.Context(), sid); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ServiceHandler) Toggle(w http.ResponseWriter, r *http.Request) {
	sid := chi.URLParam(r, "sid")
	svc, err := h.svc.ToggleService(r.Context(), sid)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, svc)
}
