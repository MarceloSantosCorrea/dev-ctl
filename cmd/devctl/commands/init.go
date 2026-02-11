package commands

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/marcelo/devctl/internal/config"
	"github.com/marcelo/devctl/internal/docker"
	"github.com/marcelo/devctl/internal/hosts"
	"github.com/marcelo/devctl/internal/ssl"
	"github.com/marcelo/devctl/internal/traefik"
)

func init() {
	rootCmd.AddCommand(initCmd)
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize devctl (install mkcert CA, create proxy network, start Traefik)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		cfg := config.DefaultConfig()

		fmt.Println("Initializing devctl...")

		// 1. Create directories
		fmt.Print("Creating directories... ")
		if err := cfg.EnsureDirs(); err != nil {
			return fmt.Errorf("creating directories: %w", err)
		}
		fmt.Println("done")

		// 1b. Copy templates to ~/.devctl/templates/
		fmt.Print("Copying templates... ")
		if err := copyTemplatesToDataDir(cfg); err != nil {
			fmt.Printf("warning: %v\n", err)
		} else {
			fmt.Println("done")
		}

		// 2. Install mkcert CA
		if ssl.IsMkcertInstalled() {
			fmt.Print("Installing mkcert CA... ")
			sslMgr := ssl.NewManager(cfg.CertsDir)
			if err := sslMgr.InstallCA(); err != nil {
				fmt.Printf("warning: %v\n", err)
			} else {
				fmt.Println("done")
			}

			// Generate cert for devctl dashboard
			fmt.Print("Generating certificate for devctl.local... ")
			if _, _, err := sslMgr.GenerateCert("devctl.local"); err != nil {
				fmt.Printf("warning: %v\n", err)
			} else {
				fmt.Println("done")
			}
		} else {
			fmt.Println("Warning: mkcert not found. Install it for HTTPS support: https://github.com/FiloSottile/mkcert")
		}

		// 3. Create Docker client
		dockerCli, err := docker.NewClient()
		if err != nil {
			return fmt.Errorf("connecting to Docker: %w", err)
		}
		defer dockerCli.Close()

		if !dockerCli.IsDockerRunning(ctx) {
			return fmt.Errorf("Docker daemon is not running. Please start Docker first")
		}

		// 4. Create proxy network
		fmt.Print("Creating devctl-proxy network... ")
		netMgr := docker.NewNetworkManager(dockerCli.Raw())
		if err := netMgr.EnsureProxyNetwork(ctx); err != nil {
			return fmt.Errorf("creating proxy network: %w", err)
		}
		fmt.Println("done")

		// 5. Start Traefik
		fmt.Print("Starting Traefik... ")
		traefikMgr := traefik.NewManager(dockerCli.Raw(), cfg.TraefikDir, cfg.CertsDir)
		if err := traefikMgr.Start(ctx); err != nil {
			return fmt.Errorf("starting Traefik: %w", err)
		}
		fmt.Println("done")

		// 6. Add devctl.local to /etc/hosts
		fmt.Print("Adding devctl.local to /etc/hosts... ")
		hostsMgr := hosts.NewManager()
		if err := hostsMgr.AddDomain("devctl.local"); err != nil {
			fmt.Printf("failed: %v\n", err)
			fmt.Println("  You can add it manually: sudo sh -c 'echo \"127.0.0.1 devctl.local\" >> /etc/hosts'")
		} else {
			fmt.Println("done")
		}

		fmt.Println("\ndevctl initialized successfully!")
		fmt.Printf("Dashboard will be available at https://devctl.local (run 'devctl serve' to start)\n")

		return nil
	},
}

// copyTemplatesToDataDir copies .yaml templates from the source directory
// (next to executable or CWD) into ~/.devctl/templates/.
func copyTemplatesToDataDir(cfg *config.Config) error {
	// Already populated — skip
	if config.HasTemplates(cfg.TemplatesDir) {
		return nil
	}

	var srcDir string
	if execPath, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(execPath), "templates")
		if config.HasTemplates(candidate) {
			srcDir = candidate
		}
	}
	if srcDir == "" {
		if config.HasTemplates("templates") {
			srcDir = "templates"
		}
	}
	if srcDir == "" {
		return fmt.Errorf("no template source found")
	}

	return copyTemplates(srcDir, cfg.TemplatesDir)
}

// copyTemplates copies all .yaml files from src to dst.
func copyTemplates(src, dst string) error {
	files, err := filepath.Glob(filepath.Join(src, "*.yaml"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := copyFile(f, filepath.Join(dst, filepath.Base(f))); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
