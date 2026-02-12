package api

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/marcelo/devctl/internal/api/handlers"
	apimiddleware "github.com/marcelo/devctl/internal/api/middleware"
	"github.com/marcelo/devctl/internal/auth"
	"github.com/marcelo/devctl/internal/config"
	"github.com/marcelo/devctl/internal/core/project"
	"github.com/marcelo/devctl/internal/docker"
)

func NewRouter(projectSvc *project.Service, dockerCli *docker.Client, templateDir string, authSvc *auth.Service, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(apimiddleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware)

	ah := handlers.NewAuthHandler(authSvc)
	ph := handlers.NewProjectHandler(projectSvc)
	sh := handlers.NewServiceHandler(projectSvc)
	th := handlers.NewTemplateHandler(templateDir)
	sysh := handlers.NewSystemHandler(dockerCli, cfg)

	r.Route("/api", func(r chi.Router) {
		// Public auth routes
		r.Post("/auth/register", ah.Register)
		r.Post("/auth/login", ah.Login)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(apimiddleware.RequireAuth(authSvc))

			// Auth
			r.Post("/auth/logout", ah.Logout)
			r.Get("/auth/me", ah.Me)

			// Projects
			r.Get("/projects", ph.List)
			r.Post("/projects", ph.Create)
			r.Get("/projects/{id}", ph.Get)
			r.Put("/projects/{id}", ph.Update)
			r.Delete("/projects/{id}", ph.Delete)
			r.Post("/projects/{id}/up", ph.Up)
			r.Post("/projects/{id}/down", ph.Down)
			r.Get("/projects/{id}/logs", ph.Logs)

			// Services within projects
			r.Post("/projects/{id}/services", sh.Create)
			r.Put("/projects/{id}/services/{sid}", sh.Update)
			r.Delete("/projects/{id}/services/{sid}", sh.Delete)
			r.Patch("/projects/{id}/services/{sid}/toggle", sh.Toggle)

			// Templates
			r.Get("/templates", th.List)

			// System
			r.Get("/system/status", sysh.Status)
		})
	})

	// SPA fallback — serve embedded frontend
	r.Get("/*", spaHandler())

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// spaHandler serves the embedded frontend. Falls back to index.html for client-side routing.
func spaHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Try to get the embedded frontend
		distFS, err := getDistFSFunc()
		if err != nil {
			// No embedded frontend — serve a simple JSON response
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"message":"devctl API running. Frontend not embedded in this build."}`))
			return
		}

		path := strings.TrimPrefix(r.URL.Path, "/")

		// Try to serve the exact file
		f, err := distFS.Open(path)
		if err == nil {
			f.Close()
			http.FileServer(http.FS(distFS)).ServeHTTP(w, r)
			return
		}

		// Fallback to index.html for SPA routing
		r.URL.Path = "/"
		http.FileServer(http.FS(distFS)).ServeHTTP(w, r)
	}
}

// getDistFSFunc returns the embedded frontend filesystem.
var getDistFSFunc = func() (fs.FS, error) {
	return nil, fs.ErrNotExist
}

// SetDistFS sets the embedded frontend filesystem.
func SetDistFS(fn func() (fs.FS, error)) {
	getDistFSFunc = fn
}
