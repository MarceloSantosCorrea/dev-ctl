package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

const ProxyNetworkName = "devctl-proxy"

type NetworkManager struct {
	cli *client.Client
}

func NewNetworkManager(cli *client.Client) *NetworkManager {
	return &NetworkManager{cli: cli}
}

// EnsureProxyNetwork creates the shared proxy network if it doesn't exist.
func (nm *NetworkManager) EnsureProxyNetwork(ctx context.Context) error {
	return nm.EnsureNetwork(ctx, ProxyNetworkName)
}

// EnsureNetwork creates a Docker network if it doesn't exist.
func (nm *NetworkManager) EnsureNetwork(ctx context.Context, name string) error {
	exists, err := nm.NetworkExists(ctx, name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	_, err = nm.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: "bridge",
	})
	if err != nil {
		return fmt.Errorf("creating network %s: %w", name, err)
	}

	return nil
}

// RemoveNetwork removes a Docker network.
func (nm *NetworkManager) RemoveNetwork(ctx context.Context, name string) error {
	return nm.cli.NetworkRemove(ctx, name)
}

// NetworkExists checks if a Docker network exists.
func (nm *NetworkManager) NetworkExists(ctx context.Context, name string) (bool, error) {
	f := filters.NewArgs(filters.Arg("name", name))
	networks, err := nm.cli.NetworkList(ctx, network.ListOptions{Filters: f})
	if err != nil {
		return false, fmt.Errorf("listing networks: %w", err)
	}

	for _, n := range networks {
		if n.Name == name {
			return true, nil
		}
	}
	return false, nil
}
