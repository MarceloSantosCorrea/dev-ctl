package devctl

import (
	"embed"
	"io/fs"
)

//go:embed web/dist/*
var distFiles embed.FS

// DistFS returns the embedded frontend filesystem.
func DistFS() (fs.FS, error) {
	return fs.Sub(distFiles, "web/dist")
}
