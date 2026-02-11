package commands

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/marcelo/devctl/internal/api"
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
	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the API server and web dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg := config.DefaultConfig()
		if err := cfg.EnsureDirs(); err != nil {
			return err
		}

		db, err := database.Open(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("opening database: %w", err)
		}
		defer db.Close()

		dockerCli, err := docker.NewClient()
		if err != nil {
			return fmt.Errorf("connecting to Docker: %w", err)
		}
		defer dockerCli.Close()

		portMgr := ports.NewManager(db)
		hostsMgr := hosts.NewManager()
		sslMgr := ssl.NewManager(cfg.CertsDir)
		traefikMgr := traefik.NewManager(dockerCli.Raw(), cfg.TraefikDir, cfg.CertsDir)
		networkMgr := docker.NewNetworkManager(dockerCli.Raw())

		templateDir := config.ResolveTemplatesDir(cfg)

		projectSvc := project.NewService(
			db, cfg, portMgr, hostsMgr, sslMgr, traefikMgr, dockerCli, networkMgr, templateDir,
		)

		router := api.NewRouter(projectSvc, dockerCli, templateDir)

		addr := fmt.Sprintf(":%d", cfg.APIPort)
		srv := &http.Server{
			Addr:    addr,
			Handler: router,
		}

		// Graceful shutdown
		ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
		defer stop()

		go func() {
			fmt.Printf("devctl server running on http://localhost%s\n", addr)
			fmt.Printf("Dashboard: http://localhost%s\n", addr)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
				os.Exit(1)
			}
		}()

		<-ctx.Done()
		fmt.Println("\nShutting down...")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		return srv.Shutdown(shutdownCtx)
	},
}
