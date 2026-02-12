package docker

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestGenerateCompose_Basic(t *testing.T) {
	services := []ServiceSpec{
		{
			Name:  "mysql",
			Image: "mysql:8.0",
			InternalPorts: []PortMapping{
				{Internal: 3306, External: 3306, Protocol: "tcp"},
			},
			Environment: map[string]string{
				"MYSQL_ROOT_PASSWORD": "root",
				"MYSQL_DATABASE":      "app",
			},
			Volumes: []string{"mysql-data:/var/lib/mysql"},
			Healthcheck: &Healthcheck{
				Test:     []string{"CMD", "mysqladmin", "ping", "-h", "localhost"},
				Interval: "10s",
				Timeout:  "5s",
				Retries:  3,
			},
		},
		{
			Name:  "redis",
			Image: "redis:7-alpine",
			InternalPorts: []PortMapping{
				{Internal: 6379, External: 6379, Protocol: "tcp"},
			},
		},
	}

	data, err := GenerateCompose("projeto1", services, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(data)

	if !strings.Contains(content, "mysql:8.0") {
		t.Error("missing mysql image")
	}
	if !strings.Contains(content, "redis:7-alpine") {
		t.Error("missing redis image")
	}
	if !strings.Contains(content, "3306:3306") {
		t.Error("missing mysql port mapping")
	}
	if !strings.Contains(content, "devctl-projeto1-mysql") {
		t.Error("missing mysql container name")
	}

	// Verify it's valid YAML
	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		t.Fatalf("generated invalid YAML: %v", err)
	}

	if len(compose.Services) != 2 {
		t.Errorf("expected 2 services, got %d", len(compose.Services))
	}
}

func TestGenerateCompose_WithTraefikLabels(t *testing.T) {
	services := []ServiceSpec{
		{
			Name:  "nginx",
			Image: "nginx:alpine",
			InternalPorts: []PortMapping{
				{Internal: 80, External: 8080, Protocol: "tcp"},
			},
			IsWebEntry: true,
			Domain:     "projeto1.local",
		},
	}

	data, err := GenerateCompose("projeto1", services, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(data)

	if !strings.Contains(content, "traefik.enable=true") {
		t.Error("missing traefik.enable label")
	}
	if !strings.Contains(content, "Host(`projeto1.local`)") {
		t.Error("missing traefik host rule")
	}
	if !strings.Contains(content, "traefik.http.routers.projeto1.tls=true") {
		t.Error("missing traefik TLS setting")
	}
	if !strings.Contains(content, "traefik.http.routers.projeto1.entrypoints=websecure") {
		t.Error("missing websecure entrypoint")
	}

	// Verify networks include devctl-proxy
	var compose ComposeFile
	yaml.Unmarshal(data, &compose)

	nginx := compose.Services["nginx"]
	hasProxy := false
	for _, n := range nginx.Networks {
		if n == proxyNetwork {
			hasProxy = true
		}
	}
	if !hasProxy {
		t.Error("web entry service should be connected to devctl-proxy network")
	}
}

func TestGenerateCompose_WithTraefikLabels_NoSSL(t *testing.T) {
	services := []ServiceSpec{
		{
			Name:  "nginx",
			Image: "nginx:alpine",
			InternalPorts: []PortMapping{
				{Internal: 80, External: 8080, Protocol: "tcp"},
			},
			IsWebEntry: true,
			Domain:     "projeto1.local",
		},
	}

	data, err := GenerateCompose("projeto1", services, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content := string(data)

	if !strings.Contains(content, "traefik.enable=true") {
		t.Error("missing traefik.enable label")
	}
	if !strings.Contains(content, "Host(`projeto1.local`)") {
		t.Error("missing traefik host rule")
	}
	if strings.Contains(content, "tls=true") {
		t.Error("should NOT have tls=true when SSL is disabled")
	}
	if !strings.Contains(content, "traefik.http.routers.projeto1.entrypoints=web") {
		t.Error("missing web entrypoint")
	}
	if strings.Contains(content, "entrypoints=websecure") {
		t.Error("should NOT have websecure entrypoint when SSL is disabled")
	}
}

func TestGenerateCompose_Networks(t *testing.T) {
	services := []ServiceSpec{
		{
			Name:  "redis",
			Image: "redis:7-alpine",
		},
	}

	data, err := GenerateCompose("myproject", services, false)
	if err != nil {
		t.Fatal(err)
	}

	var compose ComposeFile
	yaml.Unmarshal(data, &compose)

	defaultNet, ok := compose.Networks["default"]
	if !ok {
		t.Fatal("missing default network")
	}
	if defaultNet.Name != "devctl-myproject" {
		t.Errorf("expected network name devctl-myproject, got %s", defaultNet.Name)
	}

	proxyNet, ok := compose.Networks[proxyNetwork]
	if !ok {
		t.Fatal("missing devctl-proxy network")
	}
	if !proxyNet.External {
		t.Error("devctl-proxy network should be external")
	}
}
