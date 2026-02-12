package commands

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show status of all projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		svc, _, cleanup, err := buildProjectService()
		if err != nil {
			return err
		}
		defer cleanup()

		projects, err := svc.ListAllProjects(ctx)
		if err != nil {
			return err
		}

		if len(projects) == 0 {
			fmt.Println("No projects found. Create one using the dashboard or API.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tDOMAIN\tSTATUS\tSERVICES")

		for _, p := range projects {
			svcCount := len(p.Services)
			fmt.Fprintf(w, "%s\t%s\t%s\t%d\n", p.Name, p.Domain, p.Status, svcCount)
		}

		return w.Flush()
	},
}
