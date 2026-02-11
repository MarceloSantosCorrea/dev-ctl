package docker

import (
	"fmt"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const proxyNetwork = "devctl-proxy"

type ComposeFile struct {
	Services map[string]ComposeService      `yaml:"services"`
	Networks map[string]ComposeNetwork       `yaml:"networks"`
	Volumes  map[string]ComposeVolumeConfig  `yaml:"volumes,omitempty"`
}

type ComposeVolumeConfig struct {}

type ComposeBuild struct {
	Context    string `yaml:"context"`
	Dockerfile string `yaml:"dockerfile"`
}

type ComposeService struct {
	Build       *ComposeBuild     `yaml:"build,omitempty"`
	Image       string            `yaml:"image,omitempty"`
	ContainerName string          `yaml:"container_name"`
	Ports       []string          `yaml:"ports,omitempty"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Labels      []string          `yaml:"labels,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	Healthcheck *Healthcheck      `yaml:"healthcheck,omitempty"`
	Restart     string            `yaml:"restart,omitempty"`
}

type Healthcheck struct {
	Test     []string `yaml:"test,flow"`
	Interval string   `yaml:"interval,omitempty"`
	Timeout  string   `yaml:"timeout,omitempty"`
	Retries  int      `yaml:"retries,omitempty"`
}

type ComposeNetwork struct {
	Name     string `yaml:"name,omitempty"`
	External bool   `yaml:"external,omitempty"`
}

// ServiceSpec describes a service to include in the compose file.
type ServiceSpec struct {
	Name           string
	Image          string
	DockerfilePath string // If set, use build instead of image
	InternalPorts  []PortMapping
	Environment    map[string]string
	Volumes        []string
	Healthcheck    *Healthcheck
	IsWebEntry     bool   // If true, gets Traefik labels
	Domain         string // Only used if IsWebEntry
}

type PortMapping struct {
	Internal int
	External int
	Protocol string
}

// GenerateCompose creates a docker-compose.yml content for a project.
func GenerateCompose(projectName string, services []ServiceSpec) ([]byte, error) {
	compose := ComposeFile{
		Services: make(map[string]ComposeService),
		Networks: map[string]ComposeNetwork{
			"default": {
				Name: fmt.Sprintf("devctl-%s", projectName),
			},
			proxyNetwork: {
				External: true,
			},
		},
	}

	for _, spec := range services {
		svc := ComposeService{
			ContainerName: fmt.Sprintf("devctl-%s-%s", projectName, spec.Name),
			Environment:   spec.Environment,
			Volumes:       spec.Volumes,
			Healthcheck:   spec.Healthcheck,
			Restart:       "unless-stopped",
			Networks:      []string{"default"},
		}

		if spec.DockerfilePath != "" {
			svc.Build = &ComposeBuild{
				Context:    filepath.Dir(spec.DockerfilePath),
				Dockerfile: filepath.Base(spec.DockerfilePath),
			}
		} else {
			svc.Image = spec.Image
		}

		for _, p := range spec.InternalPorts {
			proto := p.Protocol
			if proto == "" {
				proto = "tcp"
			}
			if proto == "tcp" {
				svc.Ports = append(svc.Ports, fmt.Sprintf("%d:%d", p.External, p.Internal))
			} else {
				svc.Ports = append(svc.Ports, fmt.Sprintf("%d:%d/%s", p.External, p.Internal, proto))
			}
		}

		if spec.IsWebEntry && spec.Domain != "" {
			routerName := strings.ReplaceAll(projectName, "-", "")
			svc.Labels = []string{
				"traefik.enable=true",
				fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", routerName, spec.Domain),
				fmt.Sprintf("traefik.http.routers.%s.tls=true", routerName),
			}
			// Find the first internal port to use as load balancer port
			if len(spec.InternalPorts) > 0 {
				svc.Labels = append(svc.Labels,
					fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%d",
						routerName, spec.InternalPorts[0].Internal),
				)
			}
			svc.Networks = append(svc.Networks, proxyNetwork)
		}

		compose.Services[spec.Name] = svc
	}

	// Collect named volumes that need top-level declaration
	namedVolumes := make(map[string]ComposeVolumeConfig)
	for _, svc := range compose.Services {
		for _, v := range svc.Volumes {
			// Named volumes don't start with / or .
			parts := strings.SplitN(v, ":", 2)
			if len(parts) == 2 && !strings.HasPrefix(parts[0], "/") && !strings.HasPrefix(parts[0], ".") {
				namedVolumes[parts[0]] = ComposeVolumeConfig{}
			}
		}
	}
	if len(namedVolumes) > 0 {
		compose.Volumes = namedVolumes
	}

	return yaml.Marshal(&compose)
}
