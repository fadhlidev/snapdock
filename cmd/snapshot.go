package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/docker"
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

	// Step 3: Print summary
	fmt.Println()
	bold.Println("  Container Summary")
	fmt.Printf("  %-16s %s\n", "Image:", snap.Image)
	fmt.Printf("  %-16s %s\n", "Created:", snap.CreatedAt.Format(time.RFC1123))
	fmt.Printf("  %-16s %d vars\n", "Environment:", len(snap.Env))
	fmt.Printf("  %-16s %d\n", "Networks:", len(snap.Networks))
	fmt.Printf("  %-16s %d\n", "Ports:", len(snap.Ports))
	fmt.Printf("  %-16s %d\n", "Mounts:", len(snap.Mounts))

	if verbose {
		fmt.Println()
		bold.Println("  Full Snapshot Data (verbose):")
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("  ", "  ")
		// Print without Raw field to keep output readable
		snap.Raw = snap.Raw // placeholder — will omit Raw in next phase
		_ = enc.Encode(map[string]any{
			"id":        snap.ID,
			"name":      snap.Name,
			"image":     snap.Image,
			"env":       snap.Env,
			"networks":  snap.Networks,
			"ports":     snap.Ports,
			"mounts":    snap.Mounts,
			"resources": snap.Resources,
		})
	}

	fmt.Println()

	return nil
}
