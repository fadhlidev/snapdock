package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/compose"
	"github.com/fadhlidev/snapdock/internal/crypto"
	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/internal/output"
	"github.com/fadhlidev/snapdock/internal/snapshot"
	"github.com/fadhlidev/snapdock/pkg/types"
)

var stackCmd = &cobra.Command{
	Use:   "stack",
	Short: "Manage Docker Compose stacks",
	Long:  `Snapshot and restore entire Docker Compose stacks.`,
}

var stackSnapshotCmd = &cobra.Command{
	Use:   "snapshot <project-name>",
	Short: "Snapshot a Compose stack",
	Args:  cobra.ExactArgs(1),
	RunE:  runStackSnapshot,
}

var stackRestoreCmd = &cobra.Command{
	Use:   "restore <stack.sfx>",
	Short: "Restore a Compose stack from snapshot",
	Args:  cobra.ExactArgs(1),
	RunE:  runStackRestore,
}

var (
	flagStackFile        string
	flagStackOutput    string
	flagStackEncrypt bool
	flagStackName    string
	flagStackDryRun  bool
)

func init() {
	stackSnapshotCmd.Flags().StringVarP(&flagStackFile, "file", "f", "", "Compose file path (auto-detect if not specified)")
	stackSnapshotCmd.Flags().StringVarP(&flagStackOutput, "output", "o", ".", "Output directory for .sfx file")
	stackSnapshotCmd.Flags().BoolVar(&flagStackEncrypt, "encrypt", false, "Encrypt environment variables")
	stackCmd.AddCommand(stackSnapshotCmd)

	stackRestoreCmd.Flags().StringVar(&flagStackName, "name", "", "Name prefix for restored stack")
	stackRestoreCmd.Flags().BoolVar(&flagStackDryRun, "dry-run", false, "Print actions without executing")
	stackCmd.AddCommand(stackRestoreCmd)

	rootCmd.AddCommand(stackCmd)
}

func runStackSnapshot(cmd *cobra.Command, args []string) error {
	projectName := args[0]
	socketPath, _ := cmd.Flags().GetString("socket")

	var composePath string
	if flagStackFile != "" {
		composePath = flagStackFile
	} else {
		dir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("get current directory: %w", err)
		}
		composePath, err = compose.FindComposeFile(dir)
		if err != nil {
			return fmt.Errorf("find compose file: %w", err)
		}
	}

	output.Infof("Parsing compose file: %s", color.YellowString(composePath))

	project, err := compose.ParseComposeFile(composePath)
	if err != nil {
		return fmt.Errorf("parse compose file: %w", err)
	}

	if projectName != "" && project.Name != projectName {
		return fmt.Errorf("project name mismatch: expected %q, got %q", projectName, project.Name)
	}
	projectName = project.Name

	s := output.NewSpinner("Connecting to Docker daemon...")
	s.Start()

	client, err := docker.NewClient(socketPath)
	if err != nil {
		s.Stop()
		output.Errorf("%v", err)
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	fmt.Println()
	color.New(color.Bold).Println("  Stack Summary")
	fmt.Printf("  %-16s %s\n", "Project:", project.Name)
	fmt.Printf("  %-16s %d\n", "Services:", len(project.Services))
	fmt.Printf("  %-16s %d\n", "Networks:", len(project.Networks))
	fmt.Printf("  %-16s %d\n", "Volumes:", len(project.Volumes))
	fmt.Println()

	for _, svc := range project.Services {
		svcStatus := color.YellowString("?")
		_, err := client.InspectContainer(ctx, svc.Name)
		if err == nil {
			svcStatus = color.GreenString("running")
		}
		fmt.Printf("  %-16s %s %s\n", "Service "+svc.Name+":", svcStatus, color.HiBlackString("("+svc.Image+")"))
	}
	fmt.Println()

	output.Infof("Creating stack snapshot...")

	var passphrase string
	if flagStackEncrypt {
		s.Stop()
		pass, err := crypto.PromptPassphrase()
		if err != nil {
			output.Errorf("%v", err)
			return err
		}
		passphrase = pass
		s.Start()
	}

	opts := types.SnapOptions{
		Encrypted: flagStackEncrypt,
	}

	result, err := snapshot.PackStack(ctx, client, project, opts, flagStackOutput, passphrase)
	if err != nil {
		s.Stop()
		output.Errorf("Failed to create snapshot: %v", err)
		return err
	}

	s.Stop()
	output.Success("Stack snapshot created")
	fmt.Printf("  %s %s\n", color.HiBlackString("→"), result.SfxPath)
	fmt.Printf("  %s %d bytes\n", color.HiBlackString("→"), result.SizeBytes)
	fmt.Printf("  %s %s\n", color.HiBlackString("→"), result.Checksum)
	fmt.Printf("  %s %d services\n", color.HiBlackString("→"), result.ServiceCount)
	fmt.Println()

	return nil
}

func runStackRestore(cmd *cobra.Command, args []string) error {
	sfxPath := args[0]
	socketPath, _ := cmd.Flags().GetString("socket")

	absPath, err := filepath.Abs(sfxPath)
	if err != nil {
		absPath = sfxPath
	}

	output.Infof("Restoring stack from %s", color.YellowString(filepath.Base(absPath)))

	snapType, err := snapshot.DetectSnapshotType(absPath)
	if err != nil {
		return fmt.Errorf("detect snapshot type: %w", err)
	}

	if snapType != types.SnapshotTypeStack {
		return fmt.Errorf("not a stack snapshot: use 'snapdock restore' for single-container snapshots\n  hint: snapshot_type=%q, expected %q", snapType, types.SnapshotTypeStack)
	}

	if flagStackDryRun {
		output.DryRun("Would verify checksum")
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

	s := output.NewSpinner("Extracting stack snapshot...")
	s.Start()

	extracted, err := snapshot.ExtractStack(sfxPath)
	if err != nil {
		s.Stop()
		output.Errorf("Failed to extract snapshot: %v", err)
		return err
	}
	defer extracted.Cleanup()

	s.Stop()
	output.Successf("Extracted to %s", color.HiBlackString(extracted.TempDir))

	fmt.Println()
	color.New(color.Bold).Println("  Stack Summary")
	fmt.Printf("  %-16s %s\n", "Project:", extracted.Manifest.Project.Name)
	fmt.Printf("  %-16s %d\n", "Services:", len(extracted.Manifest.Services))
	fmt.Printf("  %-16s %d\n", "Networks:", len(extracted.Manifest.Networks))
	fmt.Printf("  %-16s %d\n", "Volumes:", len(extracted.Manifest.Volumes))
	fmt.Println()

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

	output.Infof("Ensuring networks exist...")

	for _, net := range extracted.Manifest.Networks {
		if flagStackDryRun {
			output.DryRunf("Would ensure network %q (driver: %s)", net.Name, net.Driver)
			continue
		}

		exists, err := client.NetworkExists(ctx, net.Name)
		if err != nil {
			output.Errorf("Failed to check network %s: %v", net.Name, err)
			return err
		}

		if exists {
			fmt.Printf("  %s network %q already exists\n", color.HiBlackString("✓"), net.Name)
		} else {
			netCfg := docker.NetworkConfig{Driver: net.Driver}
			_, err := client.CreateNetwork(ctx, net.Name, netCfg)
			if err != nil {
				output.Errorf("Failed to create network %s: %v", net.Name, err)
				return err
			}
			output.Successf("Created network %q", net.Name)
		}
	}

	output.Infof("Ensuring volumes exist...")

	for _, vol := range extracted.Manifest.Volumes {
		if flagStackDryRun {
			output.DryRunf("Would ensure volume %q", vol.Name)
			continue
		}

		exists, err := client.VolumeExists(ctx, vol.Name)
		if err != nil {
			output.Errorf("Failed to check volume %s: %v", vol.Name, err)
			return err
		}

		if exists {
			fmt.Printf("  %s volume %q already exists\n", color.HiBlackString("✓"), vol.Name)
		} else {
			_, err := client.CreateVolume(ctx, vol.Name)
			if err != nil {
				output.Errorf("Failed to create volume %s: %v", vol.Name, err)
				return err
			}
			output.Successf("Created volume %q", vol.Name)
		}
	}

	output.Infof("Restoring services...")

	for _, svc := range extracted.Compose.Services {
		img := svc.Image
		if img == "" {
			if container, ok := extracted.Services[svc.Name]; ok {
				img = container.Image
			}
		}

		if img == "" {
			output.Errorf("No image found for service %s", svc.Name)
			return fmt.Errorf("service %s has no image", svc.Name)
		}

		if !flagStackDryRun {
			pulled, err := client.PullImageIfMissing(ctx, img, os.Stdout)
			if err != nil {
				output.Errorf("Failed to pull image: %v", err)
				return err
			}
			if pulled {
				output.Successf("Pulled image %s", img)
			} else {
				fmt.Printf("  %s image %s already exists locally\n", color.HiBlackString("✓"), img)
			}
		}

		containerName := svc.Name
		if flagStackName != "" {
			containerName = flagStackName + "-" + svc.Name
		}

		if flagStackDryRun {
			output.DryRunf("Would create container %q from image %s", containerName, img)
			continue
		}

		cfg := &docker.ContainerConfig{
			Name:  containerName,
			Image: img,
			Cmd:   strings.Fields(svc.Command),
		}

		if envVars, ok := extracted.Envs[svc.Name]; ok {
			for _, e := range envVars {
				cfg.Env = append(cfg.Env, e.Key+"="+e.Value)
			}
		} else if container, ok := extracted.Services[svc.Name]; ok {
			cfg.Env = container.Env
		}

		if len(svc.Ports) > 0 {
			cfg.PortBindings = make(map[string][]docker.PortBinding)
			for _, p := range svc.Ports {
				cfg.PortBindings[fmt.Sprintf("%d/tcp", p.Target)] = []docker.PortBinding{
					{HostPort: fmt.Sprintf("%d", p.Published)},
				}
			}
		}

		result, err := client.CreateContainer(ctx, *cfg)
		if err != nil {
			output.Errorf("Failed to create container: %v", err)
			return err
		}

		output.Successf("Created container %s", color.HiBlackString(result.ID[:12]))

		if err := client.StartContainer(ctx, result.ID); err != nil {
			output.Errorf("Failed to start container: %v", err)
			return err
		}

		if err := client.WaitForRunning(ctx, result.ID, 30*time.Second); err != nil {
			output.Errorf("Health check failed: %v", err)
			return err
		}

		output.Success("Container is running")
	}

	fmt.Println()
	color.New(color.Bold).Println("  Restore Complete!")
	fmt.Printf("  Project: %s\n", extracted.Manifest.Project.Name)
	fmt.Printf("  Services: %d\n", len(extracted.Compose.Services))
	fmt.Println()

	return nil
}