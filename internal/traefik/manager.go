package traefik

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	dockerpkg "github.com/marcelo/devctl/internal/docker"
)

const (
	traefikContainerName = "devctl-traefik"
	traefikImage         = "traefik:v3"
)

type Manager struct {
	cli        *client.Client
	traefikDir string
	certsDir   string
}

func NewManager(cli *client.Client, traefikDir, certsDir string) *Manager {
	return &Manager{
		cli:        cli,
		traefikDir: traefikDir,
		certsDir:   certsDir,
	}
}

// Start ensures Traefik is running. Creates it if it doesn't exist.
func (m *Manager) Start(ctx context.Context) error {
	if err := m.writeStaticConfig(); err != nil {
		return fmt.Errorf("writing traefik config: %w", err)
	}

	running, err := m.IsRunning(ctx)
	if err != nil {
		return err
	}
	if running {
		return nil
	}

	// Check if container exists but is stopped
	exists, err := m.containerExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return m.cli.ContainerStart(ctx, traefikContainerName, container.StartOptions{})
	}

	return m.create(ctx)
}

// Stop stops the Traefik container.
func (m *Manager) Stop(ctx context.Context) error {
	return m.cli.ContainerStop(ctx, traefikContainerName, container.StopOptions{})
}

// Remove stops and removes the Traefik container.
func (m *Manager) Remove(ctx context.Context) error {
	return m.cli.ContainerRemove(ctx, traefikContainerName, container.RemoveOptions{Force: true})
}

// IsRunning checks if Traefik is running.
func (m *Manager) IsRunning(ctx context.Context) (bool, error) {
	f := filters.NewArgs(
		filters.Arg("name", traefikContainerName),
		filters.Arg("status", "running"),
	)
	containers, err := m.cli.ContainerList(ctx, container.ListOptions{Filters: f})
	if err != nil {
		return false, err
	}
	return len(containers) > 0, nil
}

// UpdateCertsConfig regenerates the dynamic TLS config for Traefik with all known certificates.
func (m *Manager) UpdateCertsConfig(domains []string) error {
	dynamicDir := filepath.Join(m.traefikDir, "dynamic")
	if err := os.MkdirAll(dynamicDir, 0755); err != nil {
		return err
	}

	certsFile := filepath.Join(dynamicDir, "certs.yaml")

	type certEntry struct {
		certFile, keyFile string
	}
	var entries []certEntry
	for _, domain := range domains {
		// Only include certs that actually exist on disk
		certOnDisk := filepath.Join(m.certsDir, domain+".pem")
		keyOnDisk := filepath.Join(m.certsDir, domain+"-key.pem")
		if _, err := os.Stat(certOnDisk); err != nil {
			continue
		}
		if _, err := os.Stat(keyOnDisk); err != nil {
			continue
		}
		entries = append(entries, certEntry{
			certFile: filepath.Join("/certs", domain+".pem"),
			keyFile:  filepath.Join("/certs", domain+"-key.pem"),
		})
	}

	// No certs: remove the file to avoid Traefik parse errors
	if len(entries) == 0 {
		os.Remove(certsFile)
		return nil
	}

	var certs strings.Builder
	certs.WriteString("tls:\n  certificates:\n")
	for _, e := range entries {
		certs.WriteString(fmt.Sprintf("    - certFile: %s\n      keyFile: %s\n", e.certFile, e.keyFile))
	}

	return os.WriteFile(certsFile, []byte(certs.String()), 0644)
}

func (m *Manager) writeStaticConfig() error {
	if err := os.MkdirAll(m.traefikDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(m.traefikDir, "dynamic"), 0755); err != nil {
		return err
	}

	config := `api:
  dashboard: false
  insecure: false

entryPoints:
  web:
    address: ":80"
    http:
      redirections:
        entryPoint:
          to: websecure
          scheme: https
  websecure:
    address: ":443"

providers:
  docker:
    endpoint: "unix:///var/run/docker.sock"
    exposedByDefault: false
    network: devctl-proxy
  file:
    directory: "/etc/traefik/dynamic"
    watch: true

log:
  level: WARN
`

	return os.WriteFile(filepath.Join(m.traefikDir, "traefik.yaml"), []byte(config), 0644)
}

func (m *Manager) containerExists(ctx context.Context) (bool, error) {
	f := filters.NewArgs(filters.Arg("name", traefikContainerName))
	containers, err := m.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return false, err
	}
	return len(containers) > 0, nil
}

func (m *Manager) create(ctx context.Context) error {
	// Pull image
	reader, err := m.cli.ImagePull(ctx, traefikImage, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling traefik image: %w", err)
	}
	defer reader.Close()
	// Drain the reader to complete the pull
	_, _ = fmt.Fprintf(os.Stderr, "Pulling %s...\n", traefikImage)
	buf := make([]byte, 4096)
	for {
		_, err := reader.Read(buf)
		if err != nil {
			break
		}
	}

	resp, err := m.cli.ContainerCreate(ctx,
		&container.Config{
			Image: traefikImage,
		},
		&container.HostConfig{
			RestartPolicy: container.RestartPolicy{Name: "unless-stopped"},
			PortBindings: nat.PortMap{
				"80/tcp":  []nat.PortBinding{{HostPort: "80"}},
				"443/tcp": []nat.PortBinding{{HostPort: "443"}},
			},
			Mounts: []mount.Mount{
				{
					Type:   mount.TypeBind,
					Source: "/var/run/docker.sock",
					Target: "/var/run/docker.sock",
				},
				{
					Type:   mount.TypeBind,
					Source: m.traefikDir,
					Target: "/etc/traefik",
				},
				{
					Type:   mount.TypeBind,
					Source: m.certsDir,
					Target: "/certs",
				},
			},
			ExtraHosts: []string{"host.docker.internal:host-gateway"},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				dockerpkg.ProxyNetworkName: {},
			},
		},
		nil,
		traefikContainerName,
	)
	if err != nil {
		return fmt.Errorf("creating traefik container: %w", err)
	}

	return m.cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
}
