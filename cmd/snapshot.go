package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/crypto"
	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/internal/output"
	"github.com/fadhlidev/snapdock/internal/snapshot"
	"github.com/fadhlidev/snapdock/pkg/types"
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot <container>",
	Short: "Snapshot a running container's full state",
	Args:  cobra.ExactArgs(1),
	RunE:  runSnapshot,
}

func init() {
	snapshotCmd.Flags().Bool("with-volumes", false, "Include volume data in snapshot")
	snapshotCmd.Flags().Bool("encrypt", false, "Encrypt environment variables with AES-256")
	snapshotCmd.Flags().StringP("output", "o", ".", "Output directory for .sfx file")

	rootCmd.AddCommand(snapshotCmd)
}

func runSnapshot(cmd *cobra.Command, args []string) error {
	containerName := args[0]
	socketPath, _ := cmd.Flags().GetString("socket")
	verbose, _   := cmd.Flags().GetBool("verbose")
	withVolumes, _ := cmd.Flags().GetBool("with-volumes")
	encrypt, _ := cmd.Flags().GetBool("encrypt")
	outputDir, _ := cmd.Flags().GetString("output")

	// Step 1: Connect to Docker daemon
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

	// Step 2: Inspect container
	output.Infof("Inspecting container %s...", color.YellowString(containerName))

	snap, err := client.InspectContainer(ctx, containerName)
	if err != nil {
		output.Errorf("%v", err)
		return err
	}

	output.Successf("Found container %s %s", color.New(color.Bold).Sprint(snap.Name), color.HiBlackString("(%s)", snap.ID[:12]))

	// Step 3: Print summary
	fmt.Println()
	color.New(color.Bold).Println("  Container Summary")
	fmt.Printf("  %-16s %s\n", "Image:", snap.Image)
	fmt.Printf("  %-16s %s\n", "Created:", snap.CreatedAt.Format(time.RFC1123))
	fmt.Printf("  %-16s %d vars\n", "Environment:", len(snap.Env))
	fmt.Printf("  %-16s %d\n", "Networks:", len(snap.Networks))
	fmt.Printf("  %-16s %d\n", "Ports:", len(snap.Ports))
	fmt.Printf("  %-16s %d\n", "Mounts:", len(snap.Mounts))
	fmt.Println()

	// Step 4: Create snapshot package
	s = output.NewSpinner("Creating snapshot package...")
	s.Start()

	opts := types.SnapOptions{
		WithVolumes: withVolumes,
		Encrypted:   encrypt,
	}

	// Prompt for passphrase if encryption is requested (stop spinner during prompt)
	var passphrase string
	if encrypt {
		s.Stop()
		pass, err := crypto.PromptPassphrase()
		if err != nil {
			output.Errorf("%v", err)
			return err
		}
		passphrase = pass
		s.Start()
	}

	result, err := snapshot.Pack(ctx, client, snap, opts, outputDir, passphrase)
	if err != nil {
		s.Stop()
		output.Errorf("Failed to create snapshot: %v", err)
		return err
	}

	s.Stop()
	output.Success("Snapshot created")
	fmt.Printf("  %s %s\n", color.HiBlackString("→"), result.SfxPath)
	fmt.Printf("  %s %d bytes\n", color.HiBlackString("→"), result.SizeBytes)
	fmt.Printf("  %s %s\n", color.HiBlackString("→"), result.Checksum)

	if verbose {
		fmt.Println()
		color.New(color.Bold).Println("  Full Snapshot Data (verbose):")
		fmt.Printf("  %-16s %s\n", "ID:", snap.ID)
		fmt.Printf("  %-16s %v\n", "Env:", snap.Env)
		fmt.Printf("  %-16s %v\n", "Networks:", snap.Networks)
		fmt.Printf("  %-16s %v\n", "Ports:", snap.Ports)
		fmt.Printf("  %-16s %v\n", "Mounts:", snap.Mounts)
		fmt.Printf("  %-16s %v\n", "Resources:", snap.Resources)
	}

	fmt.Println()

	return nil
}