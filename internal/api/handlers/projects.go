package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/marcelo/devctl/internal/auth"
	"github.com/marcelo/devctl/internal/core/project"
)

type ProjectHandler struct {
	svc *project.Service
}

func NewProjectHandler(svc *project.Service) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

func (h *ProjectHandler) List(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	projects, err := h.svc.ListProjects(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if projects == nil {
		projects = []project.Project{}
	}
	writeJSON(w, http.StatusOK, projects)
}

func (h *ProjectHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	p, err := h.svc.GetProject(r.Context(), id, user.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}
	writeJSON(w, http.StatusOK, p)
}

func (h *ProjectHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	var input project.CreateProjectInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	p, err := h.svc.CreateProject(r.Context(), input, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, p)
}

func (h *ProjectHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")

	var body struct {
		Name       string `json:"name"`
		Path       string `json:"path"`
		SSLEnabled *bool  `json:"ssl_enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	p, err := h.svc.UpdateProject(r.Context(), id, body.Name, body.Path, body.SSLEnabled, user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, p)
}

func (h *ProjectHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.DeleteProject(r.Context(), id, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *ProjectHandler) Up(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.ProjectUp(r.Context(), id, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func (h *ProjectHandler) Down(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.ProjectDown(r.Context(), id, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (h *ProjectHandler) Rebuild(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	if err := h.svc.ProjectRebuild(r.Context(), id, user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "running"})
}

func (h *ProjectHandler) Logs(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	_, err := h.svc.GetProject(r.Context(), id, user.ID)
	if err != nil {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	pingTicker := time.NewTicker(15 * time.Second)
	defer pingTicker.Stop()

	for {
		logCh := make(chan string, 64)

		// Start streaming logs in background
		go func() {
			defer close(logCh)
			h.svc.StreamLogs(r.Context(), id, logCh)
		}()

		// Read from logCh until it closes or client disconnects
	drain:
		for {
			select {
			case <-r.Context().Done():
				return
			case line, ok := <-logCh:
				if !ok {
					// Stream ended — break to outer loop to retry
					break drain
				}
				msg, _ := json.Marshal(map[string]string{"type": "log", "message": line})
				fmt.Fprintf(w, "data: %s\n\n", msg)
				flusher.Flush()
			case <-pingTicker.C:
				fmt.Fprintf(w, "data: {\"type\":\"ping\"}\n\n")
				flusher.Flush()
			}
		}

		// Wait before retrying so we don't spin
		select {
		case <-r.Context().Done():
			return
		case <-time.After(3 * time.Second):
		}
	}
}
