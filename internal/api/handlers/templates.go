package handlers

import (
	"net/http"

	"github.com/marcelo/devctl/internal/core/project"
)

type TemplateHandler struct {
	templateDir string
}

func NewTemplateHandler(templateDir string) *TemplateHandler {
	return &TemplateHandler{templateDir: templateDir}
}

func (h *TemplateHandler) List(w http.ResponseWriter, r *http.Request) {
	templates, err := project.ListTemplates(h.templateDir)
	if err != nil {
		writeJSON(w, http.StatusOK, []project.Template{})
		return
	}
	writeJSON(w, http.StatusOK, templates)
}
