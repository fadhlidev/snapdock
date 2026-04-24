package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/network"
)

// NetworkConfig holds configuration for creating a new network.
type NetworkConfig struct {
	Driver    string
	Subnet    string
	Gateway   string
	Scope     string
	Labels    map[string]string
	Options   map[string]string
}

// ListNetworks returns all networks on the Docker host.
func (c *Client) ListNetworks(ctx context.Context) ([]network.Inspect, error) {
	networks, err := c.cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list networks: %w", err)
	}
	return networks, nil
}

// NetworkExists checks if a network with the given name exists.
func (c *Client) NetworkExists(ctx context.Context, name string) (bool, error) {
	networks, err := c.ListNetworks(ctx)
	if err != nil {
		return false, err
	}

	for _, n := range networks {
		if n.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// GetNetworkByName retrieves a network resource by name.
func (c *Client) GetNetworkByName(ctx context.Context, name string) (*network.Inspect, error) {
	networks, err := c.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}

	for _, n := range networks {
		if n.Name == name {
			return &n, nil
		}
	}
	return nil, nil
}

// CreateNetwork creates a new network with the given name and config.
func (c *Client) CreateNetwork(ctx context.Context, name string, cfg NetworkConfig) (string, error) {
	exists, err := c.NetworkExists(ctx, name)
	if err != nil {
		return "", err
	}
	if exists {
		net, err := c.GetNetworkByName(ctx, name)
		if err != nil {
			return "", err
		}
		return net.ID, nil
	}

	var ipam *network.IPAM
	if cfg.Subnet != "" || cfg.Gateway != "" {
		ipam = &network.IPAM{
			Driver: "default",
			Config: []network.IPAMConfig{
				{
					Subnet:  cfg.Subnet,
					Gateway: cfg.Gateway,
				},
			},
		}
	}

	driver := cfg.Driver
	if driver == "" {
		driver = "bridge"
	}

	resp, err := c.cli.NetworkCreate(ctx, name, network.CreateOptions{
		Driver: driver,
		Scope:  cfg.Scope,
		Labels: cfg.Labels,
		Options: cfg.Options,
		IPAM: ipam,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create network %q: %w", name, err)
	}

	return resp.ID, nil
}