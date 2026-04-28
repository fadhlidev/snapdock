package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/crypto"
	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/internal/output"
	"github.com/fadhlidev/snapdock/internal/snapshot"
	"github.com/fadhlidev/snapdock/pkg/types"
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

	absPath, err := filepath.Abs(sfxPath)
	if err != nil {
		absPath = sfxPath
	}

	output.Infof("Restoring from %s", color.YellowString(filepath.Base(absPath)))

	snapType, err := snapshot.DetectSnapshotType(absPath)
	if err != nil {
		return fmt.Errorf("detect snapshot type: %w", err)
	}

	if snapType == types.SnapshotTypeStack {
		return fmt.Errorf("this is a stack snapshot: use 'snapdock stack restore' instead\n  hint: snapshot_type=%q", types.SnapshotTypeStack)
	}

	// Step 1: Verify checksum
	if flagRestoreDryRun {
		if _, err := os.Stat(sfxPath + ".sha256"); err == nil {
			output.DryRun("Checksum file exists, would verify")
		} else {
			output.DryRun("No checksum file found, skipping verification")
		}
	} else {
		s := output.NewSpinner("Verifying checksum...")
		s.Start()
		if err := snapshot.VerifyChecksum(sfxPath); err != nil {
			s.Stop()
			output.Errorf("Checksum verification failed: %v", err)
			return err
		}
		s.Stop()
		output.Success("Checksum verified")
	}

	// Step 2: Extract snapshot
	s := output.NewSpinner("Extracting snapshot...")
	s.Start()

	// Always extract for dry-run to show summary
	extracted, err := snapshot.Extract(sfxPath)
	if err != nil {
		s.Stop()
		output.Errorf("Failed to extract snapshot: %v", err)
		return err
	}
	// We don't defer Cleanup here because we need it for the whole function, 
	// and we might return early. Defer is fine as long as we don't return early before it.
	defer extracted.Cleanup()

	s.Stop()
	if flagRestoreDryRun {
		output.DryRunf("Extracted to %s", extracted.TempDir)
	} else {
		output.Successf("Extracted to %s", color.HiBlackString(extracted.TempDir))
	}

	// Step 2b: Check for encrypted env and decrypt if needed
	encPath := filepath.Join(extracted.TempDir, "env.json.enc")
	if _, err := os.Stat(encPath); err == nil {
		output.Infof("Encrypted environment variables detected")

		if flagRestoreDryRun {
			output.DryRun("Would prompt for passphrase to decrypt env")
		} else {
			passphrase, err := crypto.PromptPassphraseSingle()
			if err != nil {
				output.Errorf("%v", err)
				return err
			}

			decrypted, err := snapshot.DecryptEnv(extracted.TempDir, passphrase)
			if err != nil {
				output.Errorf("Failed to decrypt environment: %v", err)
				return err
			}
			if decrypted {
				output.Success("Environment variables decrypted")
			}
		}
	}

	// Print summary if not dry-run
	if !flagRestoreDryRun {
		manifest := extracted.Manifest
		container := extracted.Container

		fmt.Println()
		color.New(color.Bold).Println("  Snapshot Summary")
		fmt.Printf("  %-16s %s\n", "Container:", container.Name)
		fmt.Printf("  %-16s %s\n", "Image:", container.Image)
		fmt.Printf("  %-16s %s\n", "Created:", manifest.CreatedAt.Format(time.RFC1123))
		fmt.Printf("  %-16s %d\n", "Networks:", len(extracted.Networks))
		fmt.Printf("  %-16s %d\n", "Mounts:", len(extracted.Mounts.Binds)+len(extracted.Mounts.Volumes)+len(extracted.Mounts.Tmpfs))
		fmt.Println()
	}

	// Step 3: Connect to Docker
	s = output.NewSpinner("Connecting to Docker daemon...")
	s.Start()

	client, err := docker.NewClient(socketPath)
	if err != nil {
		s.Stop()
		output.Errorf("%v", err)
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if err := client.Ping(ctx); err != nil {
		s.Stop()
		output.Errorf("%v", err)
		return err
	}

	version, _ := client.Version(ctx)
	s.Stop()
	if version != "" {
		output.Successf("Connected %s", color.HiBlackString("(Docker %s)", version))
	} else {
		output.Success("Connected")
	}

	// Step 4: Recreate networks
	output.Infof("Ensuring networks exist...")

	if flagRestoreDryRun {
		for _, net := range extracted.Networks {
			output.DryRunf("Would ensure network %q (driver: %s, subnet: %s)", net.Name, net.Driver, net.Subnet)
		}
	} else {
		for _, net := range extracted.Networks {
			exists, err := client.NetworkExists(ctx, net.Name)
			if err != nil {
				output.Errorf("Failed to check network %s: %v", net.Name, err)
				return err
			}

			if exists {
				fmt.Printf("  %s network %q already exists\n", color.HiBlackString("✓"), net.Name)
			} else {
				netCfg := docker.NetworkConfig{
					Driver: net.Driver,
					Subnet: net.Subnet,
					Gateway: net.Gateway,
					Scope: net.Scope,
				}
				_, err := client.CreateNetwork(ctx, net.Name, netCfg)
				if err != nil {
					output.Errorf("Failed to create network %s: %v", net.Name, err)
					return err
				}
				output.Successf("Created network %q", net.Name)
			}
		}
	}

	// Step 5: Pull image
	imageName := extracted.Container.Image
	output.Infof("Ensuring image %s exists...", color.YellowString(imageName))

	if flagRestoreDryRun {
		output.DryRunf("Would pull image %s if missing", imageName)
	} else {
		pulled, err := client.PullImageIfMissing(ctx, imageName, os.Stdout)
		if err != nil {
			output.Errorf("Failed to pull image: %v", err)
			return err
		}

		if pulled {
			output.Successf("Pulled image %s", imageName)
		} else {
			fmt.Printf("  %s image %s already exists locally\n", color.HiBlackString("✓"), imageName)
		}
	}

	// Step 6: Restore volumes (if --with-volumes)
	output.Infof("Restoring volumes...")

	if flagRestoreWithVolumes {
		if flagRestoreDryRun {
			// Check if mounts directory exists in extracted snapshot
			mountsDir := filepath.Join(extracted.TempDir, "mounts")
			if info, err := os.Stat(mountsDir); err == nil && info.IsDir() {
				entries, _ := os.ReadDir(mountsDir)
				for _, e := range entries {
					if !e.IsDir() && strings.HasSuffix(e.Name(), ".tar.gz") {
						volumeName := strings.TrimSuffix(e.Name(), ".tar.gz")
						output.DryRunf("Would restore volume %q", volumeName)
					}
				}
			}
		} else {
			if err := snapshot.RestoreVolumes(ctx, client, extracted, extracted.TempDir); err != nil {
				output.Errorf("Failed to restore volumes: %v", err)
				return err
			}
			output.Success("Volumes restored")
		}
	} else {
		fmt.Printf("  %s skipping volumes (use --with-volumes to restore)\n", color.HiBlackString("→"))
	}

	// Step 7: Create and start container
	containerName := flagRestoreName
	if containerName == "" {
		containerName = extracted.Container.Name + "-restored"
	}

	output.Infof("Creating container %s...", color.YellowString(containerName))

	containerCfg := buildContainerConfig(extracted, containerName)

	if flagRestoreDryRun {
		output.DryRunf("Would create container with:")
		fmt.Printf("    image:    %s\n", containerCfg.Image)
		fmt.Printf("    networks: %v\n", containerCfg.Networks)
		fmt.Printf("    binds:    %v\n", containerCfg.Binds)
		fmt.Printf("    tmpfs:    %v\n", containerCfg.Tmpfs)
		fmt.Printf("    env:      %d variables\n", len(containerCfg.Env))
		output.Info("Dry run complete. No changes were made.")
	} else {
		result, err := client.CreateContainer(ctx, *containerCfg)
		if err != nil {
			output.Errorf("Failed to create container: %v", err)
			return err
		}

		output.Successf("Created container %s", color.HiBlackString(result.ID[:12]))

		output.Infof("Starting container...")

		if err := client.StartContainer(ctx, result.ID); err != nil {
			output.Errorf("Failed to start container: %v", err)
			return err
		}

		output.Success("Container started")

		// Step 7: Health check
		output.Infof("Waiting for container to be healthy...")

		if err := client.WaitForRunning(ctx, result.ID, 30*time.Second); err != nil {
			output.Errorf("Health check failed: %v", err)
			return err
		}

		output.Success("Container is running")

		fmt.Println()
		color.New(color.Bold).Println("  Restore Complete!")
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

	// Try to read env.json (might be decrypted from env.json.enc)
	envPath := filepath.Join(extracted.TempDir, "env.json")
	if envData, err := os.ReadFile(envPath); err == nil {
		// Parse env.json
		var envVars []types.EnvVar
		if err := json.Unmarshal(envData, &envVars); err == nil {
			// Convert back to []string for container config
			for _, e := range envVars {
				cfg.Env = append(cfg.Env, e.Key+"="+e.Value)
			}
		}
	}

	// Fallback to container.Env if env.json not found or parse failed
	if len(cfg.Env) == 0 {
		cfg.Env = container.Env
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