package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/marcelo/devctl/internal/config"
	"github.com/marcelo/devctl/internal/core/project"
	"github.com/marcelo/devctl/internal/database"
	"github.com/marcelo/devctl/internal/docker"
	"github.com/marcelo/devctl/internal/hosts"
	"github.com/marcelo/devctl/internal/ports"
	"github.com/marcelo/devctl/internal/ssl"
	"github.com/marcelo/devctl/internal/traefik"
)

func init() {
	rootCmd.AddCommand(upCmd)
}

var upCmd = &cobra.Command{
	Use:   "up [project-name]",
	Short: "Start a project's containers",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		ctx := context.Background()

		svc, cleanup, err := buildProjectService()
		if err != nil {
			return err
		}
		defer cleanup()

		// Find project by name
		projects, err := svc.ListProjects(ctx)
		if err != nil {
			return err
		}

		var found *project.Project
		for _, p := range projects {
			if p.Name == projectName {
				found = &p
				break
			}
		}

		if found == nil {
			return fmt.Errorf("project %q not found", projectName)
		}

		fmt.Printf("Starting project %q...\n", projectName)
		if err := svc.ProjectUp(ctx, found.ID); err != nil {
			return fmt.Errorf("starting project: %w", err)
		}

		fmt.Printf("Project %q is running at https://%s\n", projectName, found.Domain)
		return nil
	},
}

// buildProjectService creates a project.Service with all dependencies.
// Returns the service and a cleanup function.
func buildProjectService() (*project.Service, func(), error) {
	cfg := config.DefaultConfig()
	if err := cfg.EnsureDirs(); err != nil {
		return nil, nil, err
	}

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		return nil, nil, fmt.Errorf("opening database: %w", err)
	}

	dockerCli, err := docker.NewClient()
	if err != nil {
		db.Close()
		return nil, nil, fmt.Errorf("connecting to Docker: %w", err)
	}

	portMgr := ports.NewManager(db)
	hostsMgr := hosts.NewManager()
	sslMgr := ssl.NewManager(cfg.CertsDir)
	traefikMgr := traefik.NewManager(dockerCli.Raw(), cfg.TraefikDir, cfg.CertsDir)
	networkMgr := docker.NewNetworkManager(dockerCli.Raw())

	templateDir := config.ResolveTemplatesDir(cfg)
	svc := project.NewService(
		db, cfg, portMgr, hostsMgr, sslMgr, traefikMgr, dockerCli, networkMgr, templateDir,
	)

	cleanup := func() {
		dockerCli.Close()
		db.Close()
	}

	return svc, cleanup, nil
}
