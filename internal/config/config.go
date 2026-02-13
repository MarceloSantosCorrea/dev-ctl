package config

import (
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DataDir      string
	HostDataDir  string
	CertsDir     string
	TraefikDir   string
	TemplatesDir string
	DBPath       string
	APIPort      int
}

func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".devctl")

	hostDataDir := os.Getenv("DEVCTL_HOST_DATA_DIR")
	if hostDataDir == "" {
		hostDataDir = dataDir
	}

	return &Config{
		DataDir:      dataDir,
		HostDataDir:  hostDataDir,
		CertsDir:     filepath.Join(dataDir, "certs"),
		TraefikDir:   filepath.Join(dataDir, "traefik"),
		TemplatesDir: filepath.Join(dataDir, "templates"),
		DBPath:       filepath.Join(dataDir, "devctl.db"),
		APIPort:      19800,
	}
}

// HostPath traduz um caminho do container para o caminho equivalente no host.
func (c *Config) HostPath(containerPath string) string {
	if c.HostDataDir == c.DataDir {
		return containerPath
	}
	rel, err := filepath.Rel(c.DataDir, containerPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return containerPath
	}
	return filepath.Join(c.HostDataDir, rel)
}

func (c *Config) EnsureDirs() error {
	dirs := []string{
		c.DataDir,
		c.CertsDir,
		c.TraefikDir,
		c.TemplatesDir,
		filepath.Join(c.TraefikDir, "dynamic"),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}

// ResolveTemplatesDir returns the best available templates directory.
// Priority: 1) ~/.devctl/templates/ (if contains .yaml files)
//
//	2) next to executable (if contains .yaml files)
//	3) CWD "templates" (fallback)
func ResolveTemplatesDir(cfg *Config) string {
	if HasTemplates(cfg.TemplatesDir) {
		return cfg.TemplatesDir
	}

	if execPath, err := os.Executable(); err == nil {
		execDir := filepath.Join(filepath.Dir(execPath), "templates")
		if HasTemplates(execDir) {
			return execDir
		}
	}

	return "templates"
}

// HasTemplates checks whether dir contains at least one .yaml file.
func HasTemplates(dir string) bool {
	entries, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return false
	}
	return len(entries) > 0
}
