package project

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/marcelo/devctl/internal/config"
	"github.com/marcelo/devctl/internal/docker"
	"github.com/marcelo/devctl/internal/hosts"
	"github.com/marcelo/devctl/internal/ports"
	"github.com/marcelo/devctl/internal/ssl"
	"github.com/marcelo/devctl/internal/traefik"
)

type Service struct {
	db          *sql.DB
	cfg         *config.Config
	portMgr     *ports.Manager
	hostsMgr    *hosts.Manager
	sslMgr      *ssl.Manager
	traefikMgr  *traefik.Manager
	dockerCli   *docker.Client
	networkMgr  *docker.NetworkManager
	templateDir string
}

type Project struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Domain     string    `json:"domain"`
	Path       string    `json:"path"`
	SSLEnabled bool      `json:"ssl_enabled"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Services   []Svc     `json:"services,omitempty"`
	Warnings   []string  `json:"warnings,omitempty"`
}

type Svc struct {
	ID             string                 `json:"id"`
	ProjectID      string                 `json:"project_id"`
	TemplateName   string                 `json:"template_name"`
	Name           string                 `json:"name"`
	Enabled        bool                   `json:"enabled"`
	Config         map[string]interface{} `json:"config"`
	CreatedAt      time.Time              `json:"created_at"`
	Ports          []ports.PortAllocation `json:"ports,omitempty"`
	ConnectionInfo map[string]string      `json:"connection_info,omitempty"`
}

type CreateProjectInput struct {
	Name       string               `json:"name"`
	Path       string               `json:"path"`
	SSLEnabled bool                 `json:"ssl_enabled"`
	Services   []CreateServiceInput `json:"services"`
}

type CreateServiceInput struct {
	TemplateName string                 `json:"template_name"`
	Name         string                 `json:"name"`
	Config       map[string]interface{} `json:"config,omitempty"`
}

func NewService(
	db *sql.DB,
	cfg *config.Config,
	portMgr *ports.Manager,
	hostsMgr *hosts.Manager,
	sslMgr *ssl.Manager,
	traefikMgr *traefik.Manager,
	dockerCli *docker.Client,
	networkMgr *docker.NetworkManager,
	templateDir string,
) *Service {
	return &Service{
		db:          db,
		cfg:         cfg,
		portMgr:     portMgr,
		hostsMgr:    hostsMgr,
		sslMgr:      sslMgr,
		traefikMgr:  traefikMgr,
		dockerCli:   dockerCli,
		networkMgr:  networkMgr,
		templateDir: templateDir,
	}
}

// CreateProject creates a new project with its services, allocates ports, and generates SSL certs.
func (s *Service) CreateProject(ctx context.Context, input CreateProjectInput, userID string) (*Project, error) {
	projectID := uuid.New().String()
	domain := input.Name + ".local"

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx,
		"INSERT INTO projects (id, name, domain, path, user_id, ssl_enabled, status) VALUES (?, ?, ?, ?, ?, ?, 'stopped')",
		projectID, input.Name, domain, input.Path, userID, input.SSLEnabled,
	)
	if err != nil {
		return nil, fmt.Errorf("inserting project: %w", err)
	}

	for _, svcInput := range input.Services {
		svcID := uuid.New().String()
		cfgJSON, _ := json.Marshal(svcInput.Config)
		if svcInput.Config == nil {
			cfgJSON = []byte("{}")
		}

		svcName := svcInput.Name
		if svcName == "" {
			svcName = svcInput.TemplateName
		}

		_, err = tx.ExecContext(ctx,
			"INSERT INTO services (id, project_id, template_name, name, config) VALUES (?, ?, ?, ?, ?)",
			svcID, projectID, svcInput.TemplateName, svcName, string(cfgJSON),
		)
		if err != nil {
			return nil, fmt.Errorf("inserting service %s: %w", svcName, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var warnings []string
	if s.hasWebService(input.Services) {
		// Generate SSL certificate (only when SSL is enabled for this project)
		if input.SSLEnabled && ssl.IsMkcertInstalled() {
			if _, _, err := s.sslMgr.GenerateCert(domain); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to generate SSL cert for %s: %v\n", domain, err)
			}
		}

		// Add domain to /etc/hosts
		if err := s.hostsMgr.AddDomain(domain); err != nil {
			msg := fmt.Sprintf("Failed to add domain to /etc/hosts: %v. Run manually: sudo sh -c 'echo \"127.0.0.1 %s\" >> /etc/hosts'", err, domain)
			fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
			warnings = append(warnings, msg)
		}

		// Update Traefik certs config
		if input.SSLEnabled {
			s.refreshTraefikCerts()
		}
	}

	proj, err := s.GetProject(ctx, projectID, userID)
	if err != nil {
		return nil, err
	}
	proj.Warnings = warnings
	return proj, nil
}

// GetProject returns a project with its services and port allocations, filtered by userID.
func (s *Service) GetProject(ctx context.Context, id string, userID string) (*Project, error) {
	p := &Project{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, domain, path, ssl_enabled, status, created_at, updated_at FROM projects WHERE id = ? AND user_id = ?", id, userID,
	).Scan(&p.ID, &p.Name, &p.Domain, &p.Path, &p.SSLEnabled, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("fetching project: %w", err)
	}

	svcs, err := s.getProjectServices(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Services = svcs

	return p, nil
}

// getProjectByID returns a project without user filtering (for internal use).
func (s *Service) getProjectByID(ctx context.Context, id string) (*Project, error) {
	p := &Project{}
	err := s.db.QueryRowContext(ctx,
		"SELECT id, name, domain, path, ssl_enabled, status, created_at, updated_at FROM projects WHERE id = ?", id,
	).Scan(&p.ID, &p.Name, &p.Domain, &p.Path, &p.SSLEnabled, &p.Status, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("fetching project: %w", err)
	}

	svcs, err := s.getProjectServices(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Services = svcs

	return p, nil
}

// ListProjects returns all projects for a given user.
func (s *Service) ListProjects(ctx context.Context, userID string) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, domain, path, ssl_enabled, status, created_at, updated_at FROM projects WHERE user_id = ? ORDER BY created_at DESC", userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Domain, &p.Path, &p.SSLEnabled, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		svcs, _ := s.getProjectServices(ctx, p.ID)
		p.Services = svcs
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// DeleteProject removes a project and all associated resources.
func (s *Service) DeleteProject(ctx context.Context, id string, userID string) error {
	p, err := s.GetProject(ctx, id, userID)
	if err != nil {
		return err
	}

	// Stop containers first
	_ = s.projectDown(ctx, id)

	// Release ports for all services
	for _, svc := range p.Services {
		s.portMgr.ReleaseServicePorts(svc.ID)
	}

	// Remove from database (cascades to services and port_allocations)
	_, err = s.db.ExecContext(ctx, "DELETE FROM projects WHERE id = ?", id)
	if err != nil {
		return err
	}

	if s.hasWebServiceFromSvcs(p.Services) {
		// Remove domain from hosts
		s.hostsMgr.RemoveDomain(p.Domain)

		// Remove SSL cert
		if p.SSLEnabled {
			s.sslMgr.RemoveCert(p.Domain)
			s.refreshTraefikCerts()
		}
	}

	// Clean up compose file
	composePath := filepath.Join(s.cfg.DataDir, "projects", p.Name, "docker-compose.yml")
	os.RemoveAll(filepath.Dir(composePath))

	return nil
}

// UpdateProject updates a project's mutable fields.
func (s *Service) UpdateProject(ctx context.Context, id string, name string, path string, sslEnabled *bool, userID string) (*Project, error) {
	// Fetch current project to detect SSL changes
	oldProject, err := s.GetProject(ctx, id, userID)
	if err != nil {
		return nil, err
	}

	domain := name + ".local"
	_, err = s.db.ExecContext(ctx,
		"UPDATE projects SET name = ?, domain = ?, path = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?",
		name, domain, path, id, userID,
	)
	if err != nil {
		return nil, err
	}

	// Handle SSL toggle
	if sslEnabled != nil {
		_, err = s.db.ExecContext(ctx,
			"UPDATE projects SET ssl_enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ? AND user_id = ?",
			*sslEnabled, id, userID,
		)
		if err != nil {
			return nil, err
		}

		if *sslEnabled && !oldProject.SSLEnabled {
			// Turned ON: generate cert
			if ssl.IsMkcertInstalled() {
				s.sslMgr.GenerateCert(domain)
			}
			s.refreshTraefikCerts()
		} else if !*sslEnabled && oldProject.SSLEnabled {
			// Turned OFF: remove cert
			s.sslMgr.RemoveCert(domain)
			s.refreshTraefikCerts()
		}

		// If project is running, restart to apply new compose labels
		if oldProject.Status == "running" {
			_ = s.projectUp(ctx, id)
		}
	}

	return s.GetProject(ctx, id, userID)
}

// ProjectUp generates the compose file, allocates ports, and starts all containers.
func (s *Service) ProjectUp(ctx context.Context, id string, userID string) error {
	p, err := s.GetProject(ctx, id, userID)
	if err != nil {
		return err
	}

	projectDir := filepath.Join(s.cfg.DataDir, "projects", p.Name)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	// Load templates and build compose specs
	specs, err := s.buildComposeSpecs(ctx, p, projectDir)
	if err != nil {
		return fmt.Errorf("building compose specs: %w", err)
	}

	// Generate compose file
	composeData, err := docker.GenerateCompose(p.Name, specs, p.SSLEnabled)
	if err != nil {
		return fmt.Errorf("generating compose: %w", err)
	}

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, composeData, 0644); err != nil {
		return err
	}

	// Ensure proxy network exists (required as external in compose)
	if err := s.networkMgr.EnsureProxyNetwork(ctx); err != nil {
		return fmt.Errorf("ensuring proxy network: %w", err)
	}

	// Run docker compose up
	if err := docker.ComposeUp(ctx, composePath, p.Name); err != nil {
		s.updateStatus(ctx, id, "error")
		return err
	}

	s.updateStatus(ctx, id, "running")
	return nil
}

// ProjectDown stops all containers for a project.
func (s *Service) ProjectDown(ctx context.Context, id string, userID string) error {
	p, err := s.GetProject(ctx, id, userID)
	if err != nil {
		return err
	}

	composePath := filepath.Join(s.cfg.DataDir, "projects", p.Name, "docker-compose.yml")
	if _, err := os.Stat(composePath); err != nil {
		// No compose file, nothing to stop
		s.updateStatus(ctx, id, "stopped")
		return nil
	}

	if err := docker.ComposeDown(ctx, composePath, p.Name); err != nil {
		return err
	}

	s.updateStatus(ctx, id, "stopped")
	return nil
}

// ListAllProjects returns all projects regardless of user (for CLI use).
func (s *Service) ListAllProjects(ctx context.Context) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, name, domain, path, ssl_enabled, status, created_at, updated_at FROM projects ORDER BY created_at DESC",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []Project
	for rows.Next() {
		var p Project
		if err := rows.Scan(&p.ID, &p.Name, &p.Domain, &p.Path, &p.SSLEnabled, &p.Status, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		svcs, _ := s.getProjectServices(ctx, p.ID)
		p.Services = svcs
		projects = append(projects, p)
	}
	return projects, rows.Err()
}

// ProjectUpByID starts a project without user filtering (for CLI use).
func (s *Service) ProjectUpByID(ctx context.Context, id string) error {
	return s.projectUp(ctx, id)
}

// ProjectDownByID stops a project without user filtering (for CLI use).
func (s *Service) ProjectDownByID(ctx context.Context, id string) error {
	return s.projectDown(ctx, id)
}

// projectDown stops containers without user filtering (internal use).
func (s *Service) projectDown(ctx context.Context, id string) error {
	p, err := s.getProjectByID(ctx, id)
	if err != nil {
		return err
	}

	composePath := filepath.Join(s.cfg.DataDir, "projects", p.Name, "docker-compose.yml")
	if _, err := os.Stat(composePath); err != nil {
		s.updateStatus(ctx, id, "stopped")
		return nil
	}

	if err := docker.ComposeDown(ctx, composePath, p.Name); err != nil {
		return err
	}

	s.updateStatus(ctx, id, "stopped")
	return nil
}

// projectUp starts containers without user filtering (internal use).
func (s *Service) projectUp(ctx context.Context, id string) error {
	p, err := s.getProjectByID(ctx, id)
	if err != nil {
		return err
	}

	projectDir := filepath.Join(s.cfg.DataDir, "projects", p.Name)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return err
	}

	specs, err := s.buildComposeSpecs(ctx, p, projectDir)
	if err != nil {
		return fmt.Errorf("building compose specs: %w", err)
	}

	composeData, err := docker.GenerateCompose(p.Name, specs, p.SSLEnabled)
	if err != nil {
		return fmt.Errorf("generating compose: %w", err)
	}

	composePath := filepath.Join(projectDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, composeData, 0644); err != nil {
		return err
	}

	if err := s.networkMgr.EnsureProxyNetwork(ctx); err != nil {
		return fmt.Errorf("ensuring proxy network: %w", err)
	}

	if err := docker.ComposeUp(ctx, composePath, p.Name); err != nil {
		s.updateStatus(ctx, id, "error")
		return err
	}

	s.updateStatus(ctx, id, "running")
	return nil
}

// AddService adds a service to an existing project.
func (s *Service) AddService(ctx context.Context, projectID string, input CreateServiceInput) (*Svc, error) {
	svcID := uuid.New().String()
	cfgJSON, _ := json.Marshal(input.Config)
	if input.Config == nil {
		cfgJSON = []byte("{}")
	}

	name := input.Name
	if name == "" {
		name = input.TemplateName
	}

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO services (id, project_id, template_name, name, config) VALUES (?, ?, ?, ?, ?)",
		svcID, projectID, input.TemplateName, name, string(cfgJSON),
	)
	if err != nil {
		return nil, err
	}

	// If this is a web service, set up hosts + SSL
	tmpl, tmplErr := LoadTemplate(s.templateDir, input.TemplateName)
	if tmplErr == nil && (tmpl.Category == "web" || tmpl.Category == "proxy") {
		p, pErr := s.getProjectByID(ctx, projectID)
		if pErr == nil && p != nil {
			if p.SSLEnabled && ssl.IsMkcertInstalled() {
				s.sslMgr.GenerateCert(p.Domain)
			}
			s.hostsMgr.AddDomain(p.Domain)
			if p.SSLEnabled {
				s.refreshTraefikCerts()
			}
		}
	}

	return s.getService(ctx, svcID)
}

// UpdateService updates a service's configuration.
func (s *Service) UpdateService(ctx context.Context, serviceID string, name string, cfg map[string]interface{}) (*Svc, error) {
	cfgJSON, _ := json.Marshal(cfg)
	_, err := s.db.ExecContext(ctx,
		"UPDATE services SET name = ?, config = ? WHERE id = ?",
		name, string(cfgJSON), serviceID,
	)
	if err != nil {
		return nil, err
	}

	// Restart project if running
	svc, err := s.getService(ctx, serviceID)
	if err != nil {
		return nil, err
	}
	p, err := s.getProjectByID(ctx, svc.ProjectID)
	if err == nil && p.Status == "running" {
		_ = s.projectUp(ctx, p.ID)
	}

	return s.getService(ctx, serviceID)
}

// DeleteService removes a service and its port allocations.
func (s *Service) DeleteService(ctx context.Context, serviceID string) error {
	// Load service before deleting to check if it's a web service
	svc, _ := s.getService(ctx, serviceID)

	s.portMgr.ReleaseServicePorts(serviceID)
	_, err := s.db.ExecContext(ctx, "DELETE FROM services WHERE id = ?", serviceID)
	if err != nil {
		return err
	}

	// If we removed a web service, check if any remaining services are web
	if svc != nil {
		tmpl, _ := LoadTemplate(s.templateDir, svc.TemplateName)
		if tmpl != nil && (tmpl.Category == "web" || tmpl.Category == "proxy") {
			p, _ := s.getProjectByID(ctx, svc.ProjectID)
			if p != nil && !s.hasWebServiceFromSvcs(p.Services) {
				s.hostsMgr.RemoveDomain(p.Domain)
				if p.SSLEnabled {
					s.sslMgr.RemoveCert(p.Domain)
					s.refreshTraefikCerts()
				}
			}
		}
	}

	return nil
}

// ToggleService enables or disables a service.
func (s *Service) ToggleService(ctx context.Context, serviceID string) (*Svc, error) {
	_, err := s.db.ExecContext(ctx,
		"UPDATE services SET enabled = NOT enabled WHERE id = ?",
		serviceID,
	)
	if err != nil {
		return nil, err
	}
	return s.getService(ctx, serviceID)
}

func (s *Service) getProjectServices(ctx context.Context, projectID string) ([]Svc, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, project_id, template_name, name, enabled, config, created_at FROM services WHERE project_id = ?",
		projectID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var svcs []Svc
	for rows.Next() {
		var svc Svc
		var cfgStr string
		if err := rows.Scan(&svc.ID, &svc.ProjectID, &svc.TemplateName, &svc.Name, &svc.Enabled, &cfgStr, &svc.CreatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal([]byte(cfgStr), &svc.Config)

		allocs, _ := s.portMgr.GetServicePorts(svc.ID)
		svc.Ports = allocs

		s.resolveConnectionInfo(&svc)

		svcs = append(svcs, svc)
	}
	return svcs, rows.Err()
}

func (s *Service) getService(ctx context.Context, serviceID string) (*Svc, error) {
	var svc Svc
	var cfgStr string
	err := s.db.QueryRowContext(ctx,
		"SELECT id, project_id, template_name, name, enabled, config, created_at FROM services WHERE id = ?",
		serviceID,
	).Scan(&svc.ID, &svc.ProjectID, &svc.TemplateName, &svc.Name, &svc.Enabled, &cfgStr, &svc.CreatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(cfgStr), &svc.Config)

	allocs, _ := s.portMgr.GetServicePorts(svc.ID)
	svc.Ports = allocs

	s.resolveConnectionInfo(&svc)

	return &svc, nil
}

func (s *Service) buildComposeSpecs(ctx context.Context, p *Project, projectDir string) ([]docker.ServiceSpec, error) {
	var specs []docker.ServiceSpec
	phpUpstream, phpMountPath, hasPHP := s.findEnabledPHPFPM(p.Services)

	for _, svc := range p.Services {
		if !svc.Enabled {
			continue
		}

		tmpl, err := LoadTemplate(s.templateDir, svc.TemplateName)
		if err != nil {
			return nil, fmt.Errorf("loading template %s: %w", svc.TemplateName, err)
		}

		// Resolve image
		img := tmpl.DefaultImage
		if v, ok := svc.Config["image"]; ok {
			img = fmt.Sprintf("%v", v)
		}

		// Build environment
		env := make(map[string]string)
		for key, envDef := range tmpl.Environment {
			env[key] = envDef.Default
		}
		if cfgEnv, ok := svc.Config["environment"]; ok {
			if envMap, ok := cfgEnv.(map[string]interface{}); ok {
				for k, v := range envMap {
					env[k] = fmt.Sprintf("%v", v)
				}
			}
		}

		// Build volumes
		var volumes []string
		hasBind := tmpl.MountProjectPath != "" && p.Path != ""
		if hasBind {
			volumes = append(volumes, fmt.Sprintf("%s:%s", p.Path, tmpl.MountProjectPath))
		}
		for _, vol := range tmpl.Volumes {
			// Skip named volume if bind mount covers the same target
			if hasBind && vol.Target == tmpl.MountProjectPath {
				continue
			}
			volName := fmt.Sprintf("devctl-%s-%s-data", p.Name, svc.Name)
			volumes = append(volumes, fmt.Sprintf("%s:%s", volName, vol.Target))
		}
		if svc.TemplateName == "nginx" {
			nginxMountPath := tmpl.MountProjectPath
			if nginxMountPath == "" {
				nginxMountPath = "/usr/share/nginx/html"
			}

			docRoot := s.resolveNginxDocumentRoot(p.Path, nginxMountPath, svc.Config)
			phpScriptRoot := phpMountPath
			if hasPHP {
				phpScriptRoot = mapContainerRoot(docRoot, nginxMountPath, phpMountPath)
			}

			confPath, err := s.writeNginxConfig(projectDir, svc.Name, docRoot, phpUpstream, phpScriptRoot)
			if err != nil {
				return nil, fmt.Errorf("writing nginx config for %s: %w", svc.Name, err)
			}
			volumes = append(volumes, fmt.Sprintf("%s:/etc/nginx/conf.d/default.conf:ro", confPath))
		}

		// Allocate ports (reuse existing allocations)
		existingPorts, _ := s.portMgr.GetServicePorts(svc.ID)
		existingByInternal := make(map[string]int) // "port/proto" -> external
		for _, a := range existingPorts {
			key := fmt.Sprintf("%d/%s", a.InternalPort, a.Protocol)
			existingByInternal[key] = a.ExternalPort
		}

		var portMappings []docker.PortMapping
		for _, portDef := range tmpl.Ports {
			protocol := portDef.Protocol
			if protocol == "" {
				protocol = "tcp"
			}

			key := fmt.Sprintf("%d/%s", portDef.Internal, protocol)
			extPort, exists := existingByInternal[key]
			if !exists {
				var err error
				extPort, err = s.portMgr.AllocatePort(svc.ID, portDef.Internal, protocol)
				if err != nil {
					return nil, fmt.Errorf("allocating port for %s:%d: %w", svc.Name, portDef.Internal, err)
				}
			}

			portMappings = append(portMappings, docker.PortMapping{
				Internal: portDef.Internal,
				External: extPort,
				Protocol: protocol,
			})
		}

		// Extra ports from service config
		if rawExtra, ok := svc.Config["extra_ports"]; ok {
			if extraList, ok := rawExtra.([]interface{}); ok {
				for _, item := range extraList {
					ep, ok := item.(map[string]interface{})
					if !ok {
						continue
					}
					internal := 0
					switch v := ep["internal"].(type) {
					case float64:
						internal = int(v)
					case int:
						internal = v
					case json.Number:
						if n, err := v.Int64(); err == nil {
							internal = int(n)
						}
					}
					if internal < 1 || internal > 65535 {
						continue
					}

					protocol := "tcp"
					if p, ok := ep["protocol"].(string); ok && p != "" {
						protocol = p
					}

					key := fmt.Sprintf("%d/%s", internal, protocol)
					extPort, exists := existingByInternal[key]
					if !exists {
						var err error
						extPort, err = s.portMgr.AllocatePort(svc.ID, internal, protocol)
						if err != nil {
							return nil, fmt.Errorf("allocating extra port for %s:%d: %w", svc.Name, internal, err)
						}
					}

					portMappings = append(portMappings, docker.PortMapping{
						Internal: internal,
						External: extPort,
						Protocol: protocol,
					})
				}
			}
		}

		// Build healthcheck
		var hc *docker.Healthcheck
		if tmpl.Healthcheck != nil {
			hc = &docker.Healthcheck{
				Test:     tmpl.Healthcheck.Test,
				Interval: tmpl.Healthcheck.Interval,
				Timeout:  tmpl.Healthcheck.Timeout,
				Retries:  tmpl.Healthcheck.Retries,
			}
		}

		isWeb := tmpl.Category == "web" || tmpl.Category == "proxy"

		var dockerfilePath string
		if tmpl.Dockerfile != "" {
			var err error
			dockerfilePath, err = s.writeDockerfile(projectDir, svc.Name, img, tmpl.Dockerfile)
			if err != nil {
				return nil, fmt.Errorf("writing Dockerfile for %s: %w", svc.Name, err)
			}
		}

		specs = append(specs, docker.ServiceSpec{
			Name:           svc.Name,
			Image:          img,
			DockerfilePath: dockerfilePath,
			InternalPorts:  portMappings,
			Environment:    env,
			Volumes:        volumes,
			Healthcheck:    hc,
			IsWebEntry:     isWeb,
			Domain:         p.Domain,
		})
	}

	return specs, nil
}

func (s *Service) findEnabledPHPFPM(services []Svc) (string, string, bool) {
	for _, svc := range services {
		if !svc.Enabled || (svc.TemplateName != "php-fpm" && svc.TemplateName != "supervisord") {
			continue
		}

		tmpl, err := LoadTemplate(s.templateDir, svc.TemplateName)
		if err != nil {
			continue
		}

		phpMountPath := tmpl.MountProjectPath
		if phpMountPath == "" {
			phpMountPath = "/var/www/html"
		}

		return svc.Name + ":9000", path.Clean(phpMountPath), true
	}

	return "", "", false
}

func (s *Service) resolveNginxDocumentRoot(projectPath, nginxMountPath string, svcConfig map[string]interface{}) string {
	baseRoot := path.Clean(nginxMountPath)
	if baseRoot == "." || baseRoot == "/" {
		baseRoot = "/usr/share/nginx/html"
	}

	if override, ok := svcConfig["document_root"]; ok {
		val := strings.TrimSpace(fmt.Sprintf("%v", override))
		if val != "" {
			if strings.HasPrefix(val, "/") {
				return path.Clean(val)
			}
			return path.Clean(path.Join(baseRoot, val))
		}
	}

	return baseRoot
}

func mapContainerRoot(documentRoot, fromBase, toBase string) string {
	doc := path.Clean(documentRoot)
	from := path.Clean(fromBase)
	to := path.Clean(toBase)

	if doc == from {
		return to
	}
	if strings.HasPrefix(doc, from+"/") {
		suffix := strings.TrimPrefix(doc, from+"/")
		return path.Clean(path.Join(to, suffix))
	}
	return to
}

func (s *Service) writeNginxConfig(projectDir, serviceName, docRoot, phpUpstream, phpScriptRoot string) (string, error) {
	nginxDir := filepath.Join(projectDir, "nginx")
	if err := os.MkdirAll(nginxDir, 0755); err != nil {
		return "", err
	}

	safeName := strings.NewReplacer("/", "-", "\\", "-", " ", "-").Replace(serviceName)
	confPath := filepath.Join(nginxDir, safeName+"-default.conf")
	conf := buildNginxConfig(docRoot, phpUpstream, phpScriptRoot)

	if err := os.WriteFile(confPath, []byte(conf), 0644); err != nil {
		return "", err
	}
	return confPath, nil
}

func (s *Service) writeDockerfile(projectDir, serviceName, baseImage, instructions string) (string, error) {
	dfPath := filepath.Join(projectDir, "Dockerfile."+serviceName)
	content := fmt.Sprintf("FROM %s\n%s\n", baseImage, instructions)
	return dfPath, os.WriteFile(dfPath, []byte(content), 0644)
}

func buildNginxConfig(docRoot, phpUpstream, phpScriptRoot string) string {
	root := path.Clean(docRoot)
	if phpUpstream == "" {
		return fmt.Sprintf(`server {
    listen 80;
    server_name localhost;
    root %s;
    index index.html index.htm;

    location / {
        try_files $uri $uri/ =404;
    }
}
`, root)
	}

	scriptRoot := path.Clean(phpScriptRoot)
	return fmt.Sprintf(`server {
    listen 80;
    server_name localhost;
    root %s;
    index index.php index.html index.htm;

    location / {
        try_files $uri $uri/ /index.php?$query_string;
    }

    location ~ \.php$ {
        include fastcgi_params;
        fastcgi_index index.php;
        fastcgi_param SCRIPT_FILENAME %s$fastcgi_script_name;
        fastcgi_param DOCUMENT_ROOT %s;
        fastcgi_pass %s;
    }

    location ~ /\.(?!well-known).* {
        deny all;
    }
}
`, root, scriptRoot, root, phpUpstream)
}

func (s *Service) updateStatus(ctx context.Context, id, status string) {
	s.db.ExecContext(ctx,
		"UPDATE projects SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?",
		status, id,
	)
}

func (s *Service) resolveConnectionInfo(svc *Svc) {
	tmpl, err := LoadTemplate(s.templateDir, svc.TemplateName)
	if err != nil || tmpl.ConnectionInfo == nil {
		return
	}

	ci := tmpl.ConnectionInfo
	info := make(map[string]string)

	if ci.Host != "" {
		info["host"] = ci.Host
	}

	// Resolve port: find the external port allocated for the internal port_ref
	if ci.PortRef > 0 {
		for _, p := range svc.Ports {
			if p.InternalPort == ci.PortRef {
				info["port"] = fmt.Sprintf("%d", p.ExternalPort)
				break
			}
		}
		if info["port"] == "" {
			info["port"] = fmt.Sprintf("%d", ci.PortRef)
		}
	}

	if ci.User != "" {
		info["user"] = ci.User
	}

	// Resolve env var values from service config
	resolveEnv := func(envKey string) string {
		if cfgEnv, ok := svc.Config["environment"]; ok {
			if envMap, ok := cfgEnv.(map[string]interface{}); ok {
				if v, ok := envMap[envKey]; ok {
					return fmt.Sprintf("%v", v)
				}
			}
		}
		// Fallback to template defaults
		if tmpl.Environment != nil {
			if envDef, ok := tmpl.Environment[envKey]; ok {
				return envDef.Default
			}
		}
		return ""
	}

	if ci.PasswordEnv != "" {
		info["password"] = resolveEnv(ci.PasswordEnv)
	}

	if ci.DatabaseEnv != "" {
		info["database"] = resolveEnv(ci.DatabaseEnv)
	}

	// Build connection string with substitutions
	if ci.ConnectionString != "" {
		connStr := ci.ConnectionString
		for k, v := range info {
			connStr = strings.ReplaceAll(connStr, "{"+k+"}", v)
		}
		info["connection_string"] = connStr
	}

	svc.ConnectionInfo = info
}

func (s *Service) hasWebService(services []CreateServiceInput) bool {
	for _, svcInput := range services {
		tmpl, err := LoadTemplate(s.templateDir, svcInput.TemplateName)
		if err == nil && (tmpl.Category == "web" || tmpl.Category == "proxy") {
			return true
		}
	}
	return false
}

func (s *Service) hasWebServiceFromSvcs(services []Svc) bool {
	for _, svc := range services {
		tmpl, err := LoadTemplate(s.templateDir, svc.TemplateName)
		if err == nil && (tmpl.Category == "web" || tmpl.Category == "proxy") {
			return true
		}
	}
	return false
}

// StreamLogs streams logs from all running containers of a project into out.
// It blocks until ctx is cancelled (client disconnect). The caller must close out.
func (s *Service) StreamLogs(ctx context.Context, projectID string, out chan<- string) error {
	p, err := s.getProjectByID(ctx, projectID)
	if err != nil {
		out <- fmt.Sprintf("Erro ao buscar projeto: %v", err)
		return fmt.Errorf("fetching project: %w", err)
	}

	containers, err := s.dockerCli.ListProjectContainers(ctx, p.Name)
	if err != nil {
		out <- fmt.Sprintf("Erro ao listar containers: %v", err)
		return fmt.Errorf("listing containers: %w", err)
	}

	// Filter only running containers
	var running []docker.ContainerStatus
	for _, c := range containers {
		if c.State == "running" {
			running = append(running, c)
		}
	}
	if len(running) == 0 {
		out <- "Aguardando containers..."
		return nil
	}

	var wg sync.WaitGroup
	for _, ctr := range running {
		wg.Add(1)
		go func(c docker.ContainerStatus) {
			defer wg.Done()
			s.streamContainerLogs(ctx, c, p.Name, out)
		}(ctr)
	}

	wg.Wait()
	return nil
}

// streamContainerLogs reads the Docker multiplexed log stream for a single container
// and sends prefixed lines to out.
func (s *Service) streamContainerLogs(ctx context.Context, ctr docker.ContainerStatus, projectName string, out chan<- string) {
	reader, err := s.dockerCli.GetContainerLogs(ctx, ctr.Name, true)
	if err != nil {
		select {
		case <-ctx.Done():
		case out <- fmt.Sprintf("[%s] Erro ao obter logs: %v", ctr.Name, err):
		}
		return
	}
	defer reader.Close()

	// Derive short service name: "devctl-myproject-nginx-1" → "nginx"
	label := ctr.Name
	prefix := "devctl-" + projectName + "-"
	if strings.HasPrefix(label, prefix) {
		label = strings.TrimPrefix(label, prefix)
		// Remove trailing "-1", "-2" replica suffix
		if idx := strings.LastIndex(label, "-"); idx > 0 {
			label = label[:idx]
		}
	}

	// Docker multiplexed stream: each frame has an 8-byte header.
	// [0]: stream type (1=stdout, 2=stderr), [4:8]: big-endian uint32 payload size.
	header := make([]byte, 8)
	for {
		if ctx.Err() != nil {
			return
		}

		_, err := io.ReadFull(reader, header)
		if err != nil {
			return
		}

		size := binary.BigEndian.Uint32(header[4:8])
		if size == 0 {
			continue
		}

		payload := make([]byte, size)
		_, err = io.ReadFull(reader, payload)
		if err != nil {
			return
		}

		scanner := bufio.NewScanner(strings.NewReader(string(payload)))
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			select {
			case <-ctx.Done():
				return
			case out <- fmt.Sprintf("[%s] %s", label, line):
			}
		}
	}
}

func (s *Service) refreshTraefikCerts() {
	rows, _ := s.db.Query("SELECT domain FROM projects WHERE ssl_enabled = 1")
	if rows == nil {
		s.traefikMgr.UpdateCertsConfig(nil)
		return
	}
	defer rows.Close()
	var domains []string
	for rows.Next() {
		var d string
		if rows.Scan(&d) == nil {
			domains = append(domains, d)
		}
	}
	s.traefikMgr.UpdateCertsConfig(domains)
}
