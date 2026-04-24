package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/client"
)

type Client struct {
	cli *client.Client
}

// NewClient creates a Docker client connected to the given socket path.
// Pass an empty string to use the default Docker socket.
func NewClient(socketPath string) (*Client, error) {
	opts := []client.Opt{
		client.WithAPIVersionNegotiation(),
	}

	if socketPath != "" && socketPath != "/var/run/docker.sock" {
		opts = append(opts, client.WithHost("unix://"+socketPath))
	} else {
		// Use default host from DOCKER_HOST env or /var/run/docker.sock
		opts = append(opts, client.FromEnv)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Client{cli: cli}, nil
}

// Ping checks connectivity and returns server info.
// Always call this after NewClient to confirm the daemon is reachable.
func (c *Client) Ping(ctx context.Context) error {
	ping, err := c.cli.Ping(ctx)
	if err != nil {
		return fmt.Errorf(
			"cannot reach Docker daemon — is it running?\n  hint: check socket permissions or run with sudo\n  %w",
			err,
		)
	}

	// Warn if API version mismatch was negotiated
	if ping.APIVersion != "" {
		_ = ping.APIVersion // negotiation succeeded, nothing to do
	}

	return nil
}

// Version returns the Docker server version string.
func (c *Client) Version(ctx context.Context) (string, error) {
	sv, err := c.cli.ServerVersion(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to get server version: %w", err)
	}
	return sv.Version, nil
}

// Raw exposes the underlying SDK client for use inside other internal packages.
func (c *Client) Raw() *client.Client {
	return c.cli
}

// Close cleans up the underlying HTTP transport.
func (c *Client) Close() error {
	return c.cli.Close()
}
