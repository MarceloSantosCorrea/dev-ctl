package main

import (
	"os"

	devctl "github.com/marcelo/devctl"
	"github.com/marcelo/devctl/cmd/devctl/commands"
	"github.com/marcelo/devctl/internal/api"
)

func main() {
	// Register the embedded frontend
	api.SetDistFS(devctl.DistFS)

	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
