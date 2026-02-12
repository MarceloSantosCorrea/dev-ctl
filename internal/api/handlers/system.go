package handlers

import (
	"net/http"

	"github.com/marcelo/devctl/internal/config"
	"github.com/marcelo/devctl/internal/docker"
)

type SystemHandler struct {
	dockerCli *docker.Client
	cfg       *config.Config
}

func NewSystemHandler(dockerCli *docker.Client, cfg *config.Config) *SystemHandler {
	return &SystemHandler{dockerCli: dockerCli, cfg: cfg}
}

func (h *SystemHandler) Status(w http.ResponseWriter, r *http.Request) {
	dockerRunning := h.dockerCli.IsDockerRunning(r.Context())

	status := map[string]interface{}{
		"docker":  dockerRunning,
		"version": "0.1.0",
	}

	writeJSON(w, http.StatusOK, status)
}
