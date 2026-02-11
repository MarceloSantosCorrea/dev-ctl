package docker

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

type Client struct {
	cli *client.Client
}

type ContainerStatus struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
}

func NewClient() (*Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &Client{cli: cli}, nil
}

func (c *Client) Raw() *client.Client {
	return c.cli
}

func (c *Client) Close() error {
	return c.cli.Close()
}

// ListProjectContainers returns the status of all containers for a project.
func (c *Client) ListProjectContainers(ctx context.Context, projectName string) ([]ContainerStatus, error) {
	prefix := fmt.Sprintf("devctl-%s-", projectName)
	f := filters.NewArgs(filters.Arg("name", prefix))

	containers, err := c.cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: f,
	})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	var statuses []ContainerStatus
	for _, ctr := range containers {
		name := ""
		if len(ctr.Names) > 0 {
			name = strings.TrimPrefix(ctr.Names[0], "/")
		}
		statuses = append(statuses, ContainerStatus{
			ID:     ctr.ID[:12],
			Name:   name,
			Image:  ctr.Image,
			State:  ctr.State,
			Status: ctr.Status,
		})
	}

	return statuses, nil
}

// GetContainerLogs returns a reader for container logs.
func (c *Client) GetContainerLogs(ctx context.Context, containerID string, follow bool) (io.ReadCloser, error) {
	return c.cli.ContainerLogs(ctx, containerID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     follow,
		Timestamps: true,
	})
}

// ComposeUp runs docker compose up for a project directory.
func ComposeUp(ctx context.Context, composeFilePath string, projectName string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose",
		"-f", composeFilePath,
		"-p", "devctl-"+projectName,
		"up", "-d",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose up failed: %s: %w", string(output), err)
	}
	return nil
}

// ComposeDown runs docker compose down for a project directory.
func ComposeDown(ctx context.Context, composeFilePath string, projectName string) error {
	cmd := exec.CommandContext(ctx, "docker", "compose",
		"-f", composeFilePath,
		"-p", "devctl-"+projectName,
		"down",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker compose down failed: %s: %w", string(output), err)
	}
	return nil
}

// IsDockerRunning checks if the Docker daemon is reachable.
func (c *Client) IsDockerRunning(ctx context.Context) bool {
	_, err := c.cli.Ping(ctx)
	return err == nil
}
