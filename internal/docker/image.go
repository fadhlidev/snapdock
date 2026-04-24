package docker

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types/image"
)

// ImageExists checks if the given image exists locally.
func (c *Client) ImageExists(ctx context.Context, repoTag string) (bool, error) {
	images, err := c.cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to list images: %w", err)
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == repoTag {
				return true, nil
			}
		}
	}
	return false, nil
}

// PullImage pulls an image from a registry.
func (c *Client) PullImage(ctx context.Context, repoTag string, writer io.Writer) error {
	exists, err := c.ImageExists(ctx, repoTag)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	pullOpts := image.PullOptions{}
	reader, err := c.cli.ImagePull(ctx, repoTag, pullOpts)
	if err != nil {
		return fmt.Errorf("failed to pull image %q: %w", repoTag, err)
	}
	defer reader.Close()

	if writer != nil {
		_, err := io.Copy(writer, reader)
		if err != nil {
			return fmt.Errorf("error streaming pull output: %w", err)
		}
	} else {
		_, err := io.Copy(io.Discard, reader)
		if err != nil {
			return fmt.Errorf("error draining pull stream: %w", err)
		}
	}

	return nil
}

// PullImageIfMissing pulls the image only if it doesn't exist locally.
func (c *Client) PullImageIfMissing(ctx context.Context, repoTag string, writer io.Writer) (bool, error) {
	exists, err := c.ImageExists(ctx, repoTag)
	if err != nil {
		return false, err
	}
	if exists {
		return false, nil
	}

	if err := c.PullImage(ctx, repoTag, writer); err != nil {
		return false, err
	}

	return true, nil
}