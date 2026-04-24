package snapshot

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/fadhlidev/snapdock/internal/docker"
)

// VolumeTarInfo holds mapping from volume name to tar file path
type VolumeTarInfo map[string]string // volumeName -> tarPath

// SnapshotVolumes captures the contents of Docker volumes and saves them as tar files.
// Returns a map of volumeName -> tarFilePath for each volume that was captured.
func SnapshotVolumes(
	ctx context.Context,
	client *docker.Client,
	snap *docker.ContainerSnapshot,
	tempDir string,
) (VolumeTarInfo, error) {
	volumeTars := make(VolumeTarInfo)

	for _, mount := range snap.Mounts {
		if mount.Type != "volume" {
			continue
		}

		if mount.Name == "" {
			continue
		}

		fmt.Printf("  %s snapshotting volume %s...\n", "→", mount.Name)

		tarPath, err := snapshotVolume(ctx, client, mount.Name, mount.Destination, tempDir)
		if err != nil {
			return nil, fmt.Errorf("failed to snapshot volume %q: %w", mount.Name, err)
		}

		volumeTars[mount.Name] = tarPath
		fmt.Printf("  %s captured volume %s -> %s\n", "✓", mount.Name, filepath.Base(tarPath))
	}

	return volumeTars, nil
}

// snapshotVolume creates a temporary container to tar the volume contents.
// Returns the path to the tar file on the host.
func snapshotVolume(
	ctx context.Context,
	client *docker.Client,
	volumeName string,
	containerPath string,
	tempDir string,
) (string, error) {
	// Create temp output directory
	outputDir := filepath.Join(tempDir, "volume-output")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return "", fmt.Errorf("create output dir: %w", err)
	}

	containerName := fmt.Sprintf("snapdock-temp-snapshot-%s", volumeName)

	// Pull alpine image if not exists
	_, err := client.PullImageIfMissing(ctx, "alpine:latest", nil)
	if err != nil {
		return "", fmt.Errorf("pull alpine image: %w", err)
	}

	// Create temp container with volume mounted
	containerCfg := &container.Config{
		Image:        "alpine:latest",
		Cmd:          []string{"tar", "-czf", "/output/volume.tar.gz", "-C", containerPath, "."},
		AttachStdout: true,
		AttachStderr: true,
	}

	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/output", outputDir),
			fmt.Sprintf("%s:%s", volumeName, containerPath),
		},
	}

	networkingCfg := &network.NetworkingConfig{}

	resp, err := client.Raw().ContainerCreate(ctx, containerCfg, hostCfg, networkingCfg, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("create temp container: %w", err)
	}
	containerID := resp.ID

	// Cleanup on error
	defer func() {
		client.Raw().ContainerRemove(ctx, containerID, container.RemoveOptions{})
	}()

	// Start container
	if err := client.Raw().ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return "", fmt.Errorf("start temp container: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := client.Raw().ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return "", fmt.Errorf("wait container: %w", err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			// Get logs for debugging
			logs, _ := client.Raw().ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
			if logs != nil {
				defer logs.Close()
				buf, _ := io.ReadAll(logs)
				return "", fmt.Errorf("tar failed with code %d: %s", status.StatusCode, string(buf))
			}
			return "", fmt.Errorf("tar failed with code %d", status.StatusCode)
		}
	}

	tarPath := filepath.Join(outputDir, "volume.tar.gz")
	return tarPath, nil
}

// RestoreVolumes extracts volume data from the .sfx archive into Docker volumes.
func RestoreVolumes(
	ctx context.Context,
	client *docker.Client,
	extracted *ExtractedSnapshot,
	tempDir string,
) error {
	// Check if mounts directory exists in extracted snapshot
	mountsDir := filepath.Join(tempDir, "mounts")
	info, err := os.Stat(mountsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No volumes to restore
		}
		return fmt.Errorf("stat mounts dir: %w", err)
	}
	if !info.IsDir() {
		return nil
	}

	// Get volume info from container.json
	volumeMounts := make(map[string]string) // volumeName -> containerPath
	for _, mount := range extracted.Container.Mounts {
		if mount.Type == "volume" && mount.Name != "" {
			volumeMounts[mount.Name] = mount.Destination
		}
	}

	// Iterate through tar files in mounts directory
	entries, err := os.ReadDir(mountsDir)
	if err != nil {
		return fmt.Errorf("read mounts dir: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		tarName := entry.Name()
		if len(tarName) < 8 || tarName[len(tarName)-7:] != ".tar.gz" {
			continue
		}

		volumeName := tarName[:len(tarName)-7] // remove .tar.gz
		containerPath, ok := volumeMounts[volumeName]
		if !ok {
			fmt.Printf("  ⚠ volume %s in archive but not in container config, skipping\n", volumeName)
			continue
		}

		tarPath := filepath.Join(mountsDir, tarName)

		fmt.Printf("  %s restoring volume %s...\n", "→", volumeName)

		// Create volume if it doesn't exist
		exists, err := client.VolumeExists(ctx, volumeName)
		if err != nil {
			return fmt.Errorf("check volume exists: %w", err)
		}
		if !exists {
			_, err := client.CreateVolume(ctx, volumeName)
			if err != nil {
				return fmt.Errorf("create volume %q: %w", volumeName, err)
			}
			fmt.Printf("  %s created volume %s\n", "✓", volumeName)
		}

		// Restore volume data
		if err := restoreVolume(ctx, client, volumeName, containerPath, tarPath); err != nil {
			return fmt.Errorf("restore volume %q: %w", volumeName, err)
		}

		fmt.Printf("  %s restored volume %s\n", "✓", volumeName)
	}

	return nil
}

// restoreVolume extracts a tar file into a Docker volume using a temporary container.
func restoreVolume(
	ctx context.Context,
	client *docker.Client,
	volumeName string,
	containerPath string,
	tarPath string,
) error {
	containerName := fmt.Sprintf("snapdock-temp-restore-%s", volumeName)

	// Pull alpine image if not exists
	_, err := client.PullImageIfMissing(ctx, "alpine:latest", nil)
	if err != nil {
		return fmt.Errorf("pull alpine image: %w", err)
	}

	containerCfg := &container.Config{
		Image:        "alpine:latest",
		Cmd:          []string{"tar", "-xzf", "/input/volume.tar.gz", "-C", containerPath},
		AttachStdout: true,
		AttachStderr: true,
	}

	hostCfg := &container.HostConfig{
		Binds: []string{
			fmt.Sprintf("%s:/input/volume.tar.gz", tarPath),
			fmt.Sprintf("%s:%s", volumeName, containerPath),
		},
	}

	networkingCfg := &network.NetworkingConfig{}

	resp, err := client.Raw().ContainerCreate(ctx, containerCfg, hostCfg, networkingCfg, nil, containerName)
	if err != nil {
		return fmt.Errorf("create temp container: %w", err)
	}
	containerID := resp.ID

	// Cleanup on error
	defer func() {
		client.Raw().ContainerRemove(ctx, containerID, container.RemoveOptions{})
	}()

	// Start container
	if err := client.Raw().ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("start temp container: %w", err)
	}

	// Wait for container to finish
	statusCh, errCh := client.Raw().ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return fmt.Errorf("wait container: %w", err)
	case status := <-statusCh:
		if status.StatusCode != 0 {
			logs, _ := client.Raw().ContainerLogs(ctx, containerID, container.LogsOptions{ShowStdout: true, ShowStderr: true})
			if logs != nil {
				defer logs.Close()
				buf, _ := io.ReadAll(logs)
				return fmt.Errorf("untar failed with code %d: %s", status.StatusCode, string(buf))
			}
			return fmt.Errorf("untar failed with code %d", status.StatusCode)
		}
	}

	return nil
}

// CleanupVolumeTars removes the temporary tar files created during snapshot.
func CleanupVolumeTars(tarInfo VolumeTarInfo) {
	for _, tarPath := range tarInfo {
		os.RemoveAll(filepath.Dir(tarPath))
	}
}

// waitForContainer is a helper to wait for container to finish with timeout.
func waitForContainer(ctx context.Context, client *docker.Client, containerID string, timeout time.Duration) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	deadline := time.Now().Add(timeout)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for container %s", containerID)
			}

			inspect, err := client.Raw().ContainerInspect(ctx, containerID)
			if err != nil {
				return fmt.Errorf("inspect container: %w", err)
			}

			if !inspect.State.Running {
				if inspect.State.Status == "exited" || inspect.State.Status == "dead" {
					if inspect.State.ExitCode != 0 {
						return fmt.Errorf("container exited with code %d", inspect.State.ExitCode)
					}
					return nil
				}
			}
		}
	}
}