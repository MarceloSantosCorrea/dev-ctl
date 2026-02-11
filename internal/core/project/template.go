package project

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Template struct {
	Name             string                    `yaml:"name" json:"name"`
	DisplayName      string                    `yaml:"display_name" json:"display_name"`
	Category         string                    `yaml:"category" json:"category"`
	Description      string                    `yaml:"description" json:"description"`
	DefaultImage     string                    `yaml:"default_image" json:"default_image"`
	Dockerfile       string                    `yaml:"dockerfile,omitempty" json:"dockerfile,omitempty"`
	Ports            []TemplatePort            `yaml:"ports" json:"ports"`
	Environment      map[string]TemplateEnvVar `yaml:"environment" json:"environment"`
	Volumes          []TemplateVolume          `yaml:"volumes" json:"volumes"`
	Healthcheck      *TemplateHealthcheck      `yaml:"healthcheck,omitempty" json:"healthcheck,omitempty"`
	MountProjectPath string                    `yaml:"mount_project_path,omitempty" json:"mount_project_path,omitempty"`
	ConnectionInfo   *ConnectionInfo           `yaml:"connection_info,omitempty" json:"connection_info,omitempty"`
}

type ConnectionInfo struct {
	Host             string `yaml:"host,omitempty" json:"host,omitempty"`
	PortRef          int    `yaml:"port_ref,omitempty" json:"port_ref,omitempty"`
	User             string `yaml:"user,omitempty" json:"user,omitempty"`
	PasswordEnv      string `yaml:"password_env,omitempty" json:"password_env,omitempty"`
	DatabaseEnv      string `yaml:"database_env,omitempty" json:"database_env,omitempty"`
	ConnectionString string `yaml:"connection_string,omitempty" json:"connection_string,omitempty"`
}

type TemplatePort struct {
	Internal    int    `yaml:"internal" json:"internal"`
	Protocol    string `yaml:"protocol" json:"protocol"`
	Description string `yaml:"description" json:"description"`
}

type TemplateEnvVar struct {
	Default     string `yaml:"default" json:"default"`
	Description string `yaml:"description" json:"description"`
}

type TemplateVolume struct {
	Target      string `yaml:"target" json:"target"`
	Description string `yaml:"description" json:"description"`
}

type TemplateHealthcheck struct {
	Test     []string `yaml:"test" json:"test"`
	Interval string   `yaml:"interval" json:"interval"`
	Timeout  string   `yaml:"timeout" json:"timeout"`
	Retries  int      `yaml:"retries" json:"retries"`
}

// LoadTemplate loads a service template from the templates directory.
func LoadTemplate(templateDir, name string) (*Template, error) {
	path := filepath.Join(templateDir, name+".yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading template %s: %w", name, err)
	}

	var tmpl Template
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parsing template %s: %w", name, err)
	}

	return &tmpl, nil
}

// ListTemplates returns all available templates from the templates directory.
func ListTemplates(templateDir string) ([]Template, error) {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		return nil, fmt.Errorf("reading templates directory: %w", err)
	}

	var templates []Template
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".yaml")
		tmpl, err := LoadTemplate(templateDir, name)
		if err != nil {
			continue
		}
		templates = append(templates, *tmpl)
	}

	return templates, nil
}
