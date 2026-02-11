package commands

import (
	"context"
	"fmt"

	"github.com/marcelo/devctl/internal/core/project"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(downCmd)
}

var downCmd = &cobra.Command{
	Use:   "down [project-name]",
	Short: "Stop a project's containers",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectName := args[0]
		ctx := context.Background()

		svc, cleanup, err := buildProjectService()
		if err != nil {
			return err
		}
		defer cleanup()

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

		fmt.Printf("Stopping project %q...\n", projectName)
		if err := svc.ProjectDown(ctx, found.ID); err != nil {
			return fmt.Errorf("stopping project: %w", err)
		}

		fmt.Printf("Project %q stopped.\n", projectName)
		return nil
	},
}
