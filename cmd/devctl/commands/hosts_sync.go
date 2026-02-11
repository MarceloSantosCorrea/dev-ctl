package commands

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/marcelo/devctl/internal/config"
	"github.com/marcelo/devctl/internal/core/project"
	"github.com/marcelo/devctl/internal/database"
	"github.com/marcelo/devctl/internal/hosts"
)

func init() {
	rootCmd.AddCommand(hostsSyncCmd)
}

var hostsSyncCmd = &cobra.Command{
	Use:   "hosts-sync",
	Short: "Synchronize all project domains to /etc/hosts",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		cfg := config.DefaultConfig()

		db, err := database.Open(cfg.DBPath)
		if err != nil {
			return fmt.Errorf("opening database: %w", err)
		}
		defer db.Close()

		templateDir := config.ResolveTemplatesDir(cfg)

		// Query projects with their services to check for web services
		rows, err := db.QueryContext(ctx, "SELECT p.domain, s.template_name FROM projects p INNER JOIN services s ON s.project_id = p.id")
		if err != nil {
			return fmt.Errorf("querying projects: %w", err)
		}
		defer rows.Close()

		// Collect domains that have at least one web/proxy service
		webDomains := map[string]bool{}
		for rows.Next() {
			var domain, templateName string
			if err := rows.Scan(&domain, &templateName); err != nil {
				return err
			}
			if webDomains[domain] {
				continue
			}
			tmpl, err := project.LoadTemplate(templateDir, templateName)
			if err == nil && (tmpl.Category == "web" || tmpl.Category == "proxy") {
				webDomains[domain] = true
			}
		}
		if err := rows.Err(); err != nil {
			return err
		}

		domains := []string{"devctl.local"}
		for d := range webDomains {
			domains = append(domains, d)
		}

		fmt.Printf("Syncing %d domain(s) to /etc/hosts...\n", len(domains))
		hostsMgr := hosts.NewManager()
		if err := hostsMgr.SetDomains(domains); err != nil {
			return fmt.Errorf("setting domains: %w", err)
		}

		for _, d := range domains {
			fmt.Printf("  127.0.0.1 %s\n", d)
		}
		fmt.Println("done")

		return nil
	},
}
