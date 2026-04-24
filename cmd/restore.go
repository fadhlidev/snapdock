package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/internal/snapshot"
)

var restoreCmd = &cobra.Command{
	Use:   "restore <snapshot.sfx>",
	Short: "Restore a container from a .sfx snapshot",
	Args:  cobra.ExactArgs(1),
	RunE:  runRestore,
}

var (
	flagRestoreName        string
	flagRestoreDryRun      bool
	flagRestoreWithVolumes bool
)

func init() {
	restoreCmd.Flags().StringVar(&flagRestoreName, "name", "", "Name for the restored container (default: original name with '-restored' suffix)")
	restoreCmd.Flags().BoolVar(&flagRestoreDryRun, "dry-run", false, "Print actions without executing")
	restoreCmd.Flags().BoolVar(&flagRestoreWithVolumes, "with-volumes", false, "Restore volume data")

	rootCmd.AddCommand(restoreCmd)
}

func runRestore(cmd *cobra.Command, args []string) error {
	sfxPath := args[0]
	socketPath, _ := cmd.Flags().GetString("socket")

	bold   := color.New(color.Bold)
	green  := color.New(color.FgGreen, color.Bold)
	red    := color.New(color.FgRed, color.Bold)
	yellow := color.New(color.FgYellow)
	dim    := color.New(color.Faint)
	cyan   := color.New(color.FgCyan)

	absPath, err := filepath.Abs(sfxPath)
	if err != nil {
		absPath = sfxPath
	}

	fmt.Printf("  %s restoring from %s\n", dim.Sprint("→"), yellow.Sprint(filepath.Base(absPath)))

	// Step 1: Verify checksum
	fmt.Printf("  %s verifying checksum...\n", dim.Sprint("→"))

	if flagRestoreDryRun {
		// For dry-run, just check if the checksum file exists
		if _, err := os.Stat(sfxPath + ".sha256"); err == nil {
			fmt.Printf("  %s [dry-run] checksum file exists, would verify\n", cyan.Sprint("→"))
		} else {
			fmt.Printf("  %s [dry-run] no checksum file found, skipping verification\n", cyan.Sprint("→"))
		}
	} else {
		if err := snapshot.VerifyChecksum(sfxPath); err != nil {
			red.Fprintf(os.Stderr, "✗ checksum verification failed: %v\n", err)
			return err
		}
		green.Printf("  ✓ checksum verified\n")
	}

	// Step 2: Extract snapshot
	fmt.Printf("  %s extracting snapshot...\n", dim.Sprint("→"))

	var extracted *snapshot.ExtractedSnapshot

	// Always extract for dry-run to show summary
	extracted, err = snapshot.Extract(sfxPath)
	if err != nil {
		red.Fprintf(os.Stderr, "✗ failed to extract snapshot: %v\n", err)
		return err
	}
	defer extracted.Cleanup()

	if flagRestoreDryRun {
		fmt.Printf("  %s [dry-run] extracted to %s\n", cyan.Sprint("→"), extracted.TempDir)
	} else {
		green.Printf("  ✓ extracted to %s\n", dim.Sprint(extracted.TempDir))
	}

	// Print summary if not dry-run
	if !flagRestoreDryRun {
		manifest := extracted.Manifest
		container := extracted.Container

		fmt.Println()
		bold.Println("  Snapshot Summary")
		fmt.Printf("  %-16s %s\n", "Container:", container.Name)
		fmt.Printf("  %-16s %s\n", "Image:", container.Image)
		fmt.Printf("  %-16s %s\n", "Created:", manifest.CreatedAt.Format(time.RFC1123))
		fmt.Printf("  %-16s %d\n", "Networks:", len(extracted.Networks))
		fmt.Printf("  %-16s %d\n", "Mounts:", len(extracted.Mounts.Binds)+len(extracted.Mounts.Volumes)+len(extracted.Mounts.Tmpfs))
		fmt.Println()
	}

	// Step 3: Connect to Docker
	fmt.Printf("  %s connecting to Docker daemon...\n", dim.Sprint("→"))

	client, err := docker.NewClient(socketPath)
	if err != nil {
		red.Fprintf(os.Stderr, "✗ %v\n", err)
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		red.Fprintf(os.Stderr, "✗ %v\n", err)
		return err
	}

	version, _ := client.Version(ctx)
	green.Printf("  ✓ connected")
	if version != "" {
		fmt.Printf("  %s\n", dim.Sprintf("(Docker %s)", version))
	} else {
		fmt.Println()
	}

	// Step 4: Recreate networks
	fmt.Printf("  %s ensuring networks exist...\n", dim.Sprint("→"))

	if flagRestoreDryRun {
		for _, net := range extracted.Networks {
			fmt.Printf("  %s [dry-run] would ensure network %q (driver: %s, subnet: %s)\n",
				cyan.Sprint("→"), net.Name, net.Driver, net.Subnet)
		}
	} else {
		for _, net := range extracted.Networks {
			exists, err := client.NetworkExists(ctx, net.Name)
			if err != nil {
				red.Fprintf(os.Stderr, "✗ failed to check network %s: %v\n", net.Name, err)
				return err
			}

			if exists {
				fmt.Printf("  %s network %q already exists\n", dim.Sprint("✓"), net.Name)
			} else {
				netCfg := docker.NetworkConfig{
					Driver: net.Driver,
					Subnet: net.Subnet,
					Gateway: net.Gateway,
					Scope: net.Scope,
				}
				_, err := client.CreateNetwork(ctx, net.Name, netCfg)
				if err != nil {
					red.Fprintf(os.Stderr, "✗ failed to create network %s: %v\n", net.Name, err)
					return err
				}
				green.Printf("  ✓ created network %q\n", net.Name)
			}
		}
	}

	// Step 5: Pull image
	imageName := extracted.Container.Image

	fmt.Printf("  %s ensuring image %s exists...\n", dim.Sprint("→"), yellow.Sprint(imageName))

	if flagRestoreDryRun {
		fmt.Printf("  %s [dry-run] would pull image %s if missing\n", cyan.Sprint("→"), imageName)
	} else {
		pulled, err := client.PullImageIfMissing(ctx, imageName, os.Stdout)
		if err != nil {
			red.Fprintf(os.Stderr, "✗ failed to pull image: %v\n", err)
			return err
		}

		if pulled {
			green.Printf("  ✓ pulled image %s\n", imageName)
		} else {
			fmt.Printf("  %s image %s already exists locally\n", dim.Sprint("✓"), imageName)
		}
	}

	// Step 6: Restore volumes (if --with-volumes)
	fmt.Printf("  %s restoring volumes...\n", dim.Sprint("→"))

	if flagRestoreWithVolumes {
		if flagRestoreDryRun {
			// Check if mounts directory exists in extracted snapshot
			mountsDir := filepath.Join(extracted.TempDir, "mounts")
			if info, err := os.Stat(mountsDir); err == nil && info.IsDir() {
				entries, _ := os.ReadDir(mountsDir)
				for _, e := range entries {
					if !e.IsDir() && len(e.Name()) > 7 && e.Name()[len(e.Name())-7:] == ".tar.gz" {
						volumeName := e.Name()[:len(e.Name())-7]
						fmt.Printf("  %s [dry-run] would restore volume %q\n", cyan.Sprint("→"), volumeName)
					}
				}
			}
		} else {
			if err := snapshot.RestoreVolumes(ctx, client, extracted, extracted.TempDir); err != nil {
				red.Fprintf(os.Stderr, "✗ failed to restore volumes: %v\n", err)
				return err
			}
			green.Printf("  ✓ volumes restored\n")
		}
	} else {
		fmt.Printf("  %s skipping volumes (use --with-volumes to restore)\n", dim.Sprint("→"))
	}

	// Step 7: Create and start container
	containerName := flagRestoreName
	if containerName == "" {
		containerName = extracted.Container.Name + "-restored"
	}

	fmt.Printf("  %s creating container %s...\n", dim.Sprint("→"), yellow.Sprint(containerName))

	containerCfg := buildContainerConfig(extracted, containerName)

	if flagRestoreDryRun {
		fmt.Printf("  %s [dry-run] would create container with:\n", cyan.Sprint("→"))
		fmt.Printf("  %s   image:    %s\n", dim.Sprint("→"), containerCfg.Image)
		fmt.Printf("  %s   networks: %v\n", dim.Sprint("→"), containerCfg.Networks)
		fmt.Printf("  %s   binds:    %v\n", dim.Sprint("→"), containerCfg.Binds)
		fmt.Printf("  %s   tmpfs:    %v\n", dim.Sprint("→"), containerCfg.Tmpfs)
		fmt.Printf("  %s   env:      %d variables\n", dim.Sprint("→"), len(containerCfg.Env))
	} else {
		result, err := client.CreateContainer(ctx, *containerCfg)
		if err != nil {
			red.Fprintf(os.Stderr, "✗ failed to create container: %v\n", err)
			return err
		}

		green.Printf("  ✓ created container %s\n", result.ID[:12])

		fmt.Printf("  %s starting container...\n", dim.Sprint("→"))

		if err := client.StartContainer(ctx, result.ID); err != nil {
			red.Fprintf(os.Stderr, "✗ failed to start container: %v\n", err)
			return err
		}

		green.Printf("  ✓ container started\n")

		// Step 7: Health check
		fmt.Printf("  %s waiting for container to be healthy...\n", dim.Sprint("→"))

		if err := client.WaitForRunning(ctx, result.ID, 30*time.Second); err != nil {
			red.Fprintf(os.Stderr, "✗ health check failed: %v\n", err)
			return err
		}

		green.Printf("  ✓ container is running\n")

		fmt.Println()
		bold.Println("  Restore Complete!")
		fmt.Printf("  Container ID:   %s\n", result.ID[:12])
		fmt.Printf("  Container Name: %s\n", containerName)
	}

	fmt.Println()

	return nil
}

func buildContainerConfig(extracted *snapshot.ExtractedSnapshot, containerName string) *docker.ContainerConfig {
	container := extracted.Container

	cfg := &docker.ContainerConfig{
		Name:         containerName,
		Image:        container.Image,
		Env:          container.Env,
		Cmd:          container.Cmd,
		Entrypoint:   container.Entrypoint,
		WorkingDir:   container.WorkingDir,
		User:         container.User,
		Hostname:     container.Hostname,
		Labels:       container.Labels,
		StopSignal:   container.StopSignal,
		AutoRemove:   false,
		Privileged:   false,
	}

	for _, net := range extracted.Networks {
		cfg.Networks = append(cfg.Networks, docker.ContainerNetwork{
			Name:        net.Name,
			Aliases:     net.Aliases,
			IPv4Address: net.IPAddress,
		})
	}

	if len(container.Ports) > 0 {
		cfg.PortBindings = make(map[string][]docker.PortBinding)
		for _, p := range container.Ports {
			if p.HostPort != "" {
				cfg.PortBindings[p.ContainerPort] = append(cfg.PortBindings[p.ContainerPort], docker.PortBinding{
					HostIP:   p.HostIP,
					HostPort: p.HostPort,
				})
			}
		}
	}

	binds, tmpfs := snapshot.MountArgs(extracted.Mounts)
	cfg.Binds = binds
	cfg.Tmpfs = tmpfs

	if container.Resources.CPUShares > 0 {
		cfg.CPUShares = container.Resources.CPUShares
	}
	if container.Resources.CPUQuota > 0 {
		cfg.CPUQuota = container.Resources.CPUQuota
	}
	if container.Resources.MemoryMB > 0 {
		cfg.MemoryMB = container.Resources.MemoryMB
	}
	if container.Resources.MemSwapMB > 0 {
		cfg.MemSwapMB = container.Resources.MemSwapMB
	}

	return cfg
}