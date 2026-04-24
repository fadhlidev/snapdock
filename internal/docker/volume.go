package docker

import (
	"context"
	"fmt"

	"github.com/docker/docker/api/types/volume"
)

// CreateVolume creates a new Docker volume with the given name.
func (c *Client) CreateVolume(ctx context.Context, name string) (volume.Volume, error) {
	resp, err := c.cli.VolumeCreate(ctx, volume.CreateOptions{
		Name: name,
	})
	if err != nil {
		return volume.Volume{}, fmt.Errorf("failed to create volume %q: %w", name, err)
	}
	return resp, nil
}

// VolumeExists checks if a volume with the given name exists.
func (c *Client) VolumeExists(ctx context.Context, name string) (bool, error) {
	resp, err := c.cli.VolumeList(ctx, volume.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list volumes: %w", err)
	}

	for _, v := range resp.Volumes {
		if v.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// GetVolume retrieves a volume by name.
func (c *Client) GetVolume(ctx context.Context, name string) (volume.Volume, error) {
	return c.cli.VolumeInspect(ctx, name)
}

// RemoveVolume removes a Docker volume.
func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	err := c.cli.VolumeRemove(ctx, name, false)
	if err != nil {
		return fmt.Errorf("failed to remove volume %q: %w", name, err)
	}
	return nil
}