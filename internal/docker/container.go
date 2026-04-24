package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

// ContainerConfig holds all configuration needed to create a container.
type ContainerConfig struct {
	Name         string
	Image        string
	Env          []string
	Cmd          []string
	Entrypoint   []string
	WorkingDir   string
	User         string
	Hostname     string
	Labels       map[string]string
	StopSignal   string

	Networks []ContainerNetwork

	PortBindings map[string][]PortBinding

	Binds []string
	Tmpfs []string

	CPUShares  int64
	CPUQuota   int64
	MemoryMB   int64
	MemSwapMB  int64

	AutoRemove bool
	Privileged bool
}

// PortBinding represents a host port binding.
type PortBinding struct {
	HostIP   string
	HostPort string
}

// ContainerNetwork holds network configuration for a container.
type ContainerNetwork struct {
	Name        string
	Aliases     []string
	IPv4Address string
}

// ContainerCreateResult holds the result of container creation.
type ContainerCreateResult struct {
	ID   string
	Name string
}

// CreateContainer creates a new container from the given config.
func (c *Client) CreateContainer(ctx context.Context, cfg ContainerConfig) (*ContainerCreateResult, error) {
	containerCfg := &container.Config{
		Image:        cfg.Image,
		Env:          cfg.Env,
		Cmd:          cfg.Cmd,
		Entrypoint:   cfg.Entrypoint,
		WorkingDir:   cfg.WorkingDir,
		User:         cfg.User,
		Hostname:     cfg.Hostname,
		Labels:       cfg.Labels,
		StopSignal:   cfg.StopSignal,
		Tty:          false,
		OpenStdin:    false,
	}

	hostCfg := &container.HostConfig{
		AutoRemove: cfg.AutoRemove,
		Privileged: cfg.Privileged,
		Binds:      cfg.Binds,
		Tmpfs:      buildTmpfs(cfg.Tmpfs),
	}

	if cfg.CPUShares > 0 || cfg.CPUQuota > 0 || cfg.MemoryMB > 0 || cfg.MemSwapMB > 0 {
		hostCfg.Resources = container.Resources{
			CPUShares:  cfg.CPUShares,
			CPUQuota:   cfg.CPUQuota,
			Memory:     cfg.MemoryMB * 1024 * 1024,
			MemorySwap: cfg.MemSwapMB * 1024 * 1024,
		}
	}

	if len(cfg.PortBindings) > 0 {
		hostCfg.PortBindings = buildPortBindings(cfg.PortBindings)
	}

	networkingCfg := &network.NetworkingConfig{}
	if len(cfg.Networks) > 0 {
		networkingCfg.EndpointsConfig = make(map[string]*network.EndpointSettings)
		for _, net := range cfg.Networks {
			epSettings := &network.EndpointSettings{
				Aliases: net.Aliases,
			}
			if net.IPv4Address != "" {
				epSettings.IPAddress = net.IPv4Address
			}
			networkingCfg.EndpointsConfig[net.Name] = epSettings
		}
	}

	resp, err := c.cli.ContainerCreate(ctx, containerCfg, hostCfg, networkingCfg, nil, cfg.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	return &ContainerCreateResult{
		ID:   resp.ID,
		Name: cfg.Name,
	}, nil
}

// StartContainer starts a container by ID.
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	if err := c.cli.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container %s: %w", containerID, err)
	}
	return nil
}

// WaitForRunning polls the container status until it's running or timeout.
func (c *Client) WaitForRunning(ctx context.Context, containerID string, timeout time.Duration) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for container %s to start", containerID)
			}

			inspect, err := c.cli.ContainerInspect(ctx, containerID)
			if err != nil {
				return fmt.Errorf("failed to inspect container %s: %w", containerID, err)
			}

			if inspect.State.Running {
				return nil
			}

			if inspect.State.Status == "exited" || inspect.State.Status == "dead" {
				return fmt.Errorf("container %s is %s: %s", containerID, inspect.State.Status, inspect.State.Error)
			}
		}
	}
}

func buildTmpfs(tmpfs []string) map[string]string {
	if len(tmpfs) == 0 {
		return nil
	}
	result := make(map[string]string)
	for _, m := range tmpfs {
		result[m] = ""
	}
	return result
}

func buildPortBindings(portBindings map[string][]PortBinding) nat.PortMap {
	result := make(nat.PortMap)
	for port, bindings := range portBindings {
		dockerPort := nat.Port(port)
		hostBindings := make([]nat.PortBinding, len(bindings))
		for i, b := range bindings {
			hostBindings[i] = nat.PortBinding{
				HostIP:   b.HostIP,
				HostPort: b.HostPort,
			}
		}
		result[dockerPort] = hostBindings
	}
	return result
}