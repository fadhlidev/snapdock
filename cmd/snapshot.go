package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/crypto"
	"github.com/fadhlidev/snapdock/internal/docker"
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

	bold   := color.New(color.Bold)
	green  := color.New(color.FgGreen, color.Bold)
	red    := color.New(color.FgRed, color.Bold)
	yellow := color.New(color.FgYellow)
	dim    := color.New(color.Faint)

	// Step 1: Connect to Docker daemon
	fmt.Printf("  %s connecting to Docker daemon...\n", dim.Sprint("→"))

	client, err := docker.NewClient(socketPath)
	if err != nil {
		red.Fprintf(os.Stderr, "✗ %v\n", err)
		return err
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

	// Step 2: Inspect container
	fmt.Printf("  %s inspecting container %s...\n",
		dim.Sprint("→"), yellow.Sprint(containerName))

	snap, err := client.InspectContainer(ctx, containerName)
	if err != nil {
		red.Fprintf(os.Stderr, "✗ %v\n", err)
		return err
	}

	green.Printf("  ✓ found container ")
	bold.Printf("%s", snap.Name)
	fmt.Printf(" %s\n", dim.Sprintf("(%s)", snap.ID[:12]))

	// Step 3: Print summary (if not verbose, we just show basic info)
	fmt.Println()
	bold.Println("  Container Summary")
	fmt.Printf("  %-16s %s\n", "Image:", snap.Image)
	fmt.Printf("  %-16s %s\n", "Created:", snap.CreatedAt.Format(time.RFC1123))
	fmt.Printf("  %-16s %d vars\n", "Environment:", len(snap.Env))
	fmt.Printf("  %-16s %d\n", "Networks:", len(snap.Networks))
	fmt.Printf("  %-16s %d\n", "Ports:", len(snap.Ports))
	fmt.Printf("  %-16s %d\n", "Mounts:", len(snap.Mounts))
	fmt.Println()

	// Step 4: Create snapshot package
	fmt.Printf("  %s creating snapshot package...\n", dim.Sprint("→"))

	opts := types.SnapOptions{
		WithVolumes: withVolumes,
		Encrypted:   encrypt,
	}

	// Prompt for passphrase if encryption is requested
	var passphrase string
	if encrypt {
		pass, err := crypto.PromptPassphrase()
		if err != nil {
			red.Fprintf(os.Stderr, "✗ %v\n", err)
			return err
		}
		passphrase = pass
	}

	result, err := snapshot.Pack(ctx, client, snap, opts, outputDir, passphrase)
	if err != nil {
		red.Fprintf(os.Stderr, "✗ failed to create snapshot: %v\n", err)
		return err
	}

	green.Printf("  ✓ snapshot created\n")
	fmt.Printf("  %s %s\n", dim.Sprint("→"), result.SfxPath)
	fmt.Printf("  %s %d bytes\n", dim.Sprint("→"), result.SizeBytes)
	fmt.Printf("  %s %s\n", dim.Sprint("→"), result.Checksum)

	if verbose {
		fmt.Println()
		bold.Println("  Full Snapshot Data (verbose):")
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