package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/fatih/color"
	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/snapshot"
	"github.com/fadhlidev/snapdock/pkg/types"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect <snapshot.sfx>",
	Short: "Display contents of a snapshot without restoring",
	Args:  cobra.ExactArgs(1),
	RunE:  runInspect,
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

func runInspect(cmd *cobra.Command, args []string) error {
	sfxPath := args[0]

	bold   := color.New(color.Bold)
	cyan   := color.New(color.FgCyan)
	yellow := color.New(color.FgYellow)
	dim    := color.New(color.Faint)

	// Validate file exists
	if _, err := os.Stat(sfxPath); err != nil {
		return fmt.Errorf("snapshot file not found: %w", err)
	}

	// Check .sfx extension
	if !strings.HasSuffix(sfxPath, ".sfx") {
		return fmt.Errorf("file must have .sfx extension")
	}

	fmt.Printf("  %s inspecting %s\n", dim.Sprint("→"), yellow.Sprint(filepath.Base(sfxPath)))

	// Extract to temp dir
	extracted, err := snapshot.Extract(sfxPath)
	if err != nil {
		return fmt.Errorf("failed to extract snapshot: %w", err)
	}
	defer extracted.Cleanup()

	// Display header
	fmt.Println()
	bold.Println("  Snapshot:", filepath.Base(sfxPath))
	fmt.Println()

	// Display manifest info
	displayManifest(extracted.Manifest, bold, cyan, yellow)

	// Display environment
	displayEnv(extracted.TempDir, bold, cyan, yellow)

	// Display networks
	displayNetworks(extracted.Container, bold, cyan)

	// Display mounts
	displayMounts(extracted.TempDir, extracted.Container, bold, cyan)

	fmt.Println()

	return nil
}

func displayManifest(m *types.Manifest, bold, cyan, yellow *color.Color) {
	bold.Println("  Container:")
	fmt.Printf("    %-12s %s\n", "Name:", cyan.Sprint(m.Container.Name))
	fmt.Printf("    %-12s %s\n", "Image:", m.Container.Image)
	fmt.Printf("    %-12s %s\n", "ID:", m.Container.ID[:12])
	fmt.Println()

	fmt.Printf("    %-12s %s\n", "Created:", m.CreatedAt.Format("Jan 2, 2006 03:04 PM"))
	fmt.Printf("    %-12s %s\n", "Version:", m.SnapforgeVersion)
	fmt.Println()
}

func displayEnv(tempDir string, bold, cyan, yellow *color.Color) {
	bold.Println("  Environment Variables:")

	encPath := filepath.Join(tempDir, "env.json.enc")
	envPath := filepath.Join(tempDir, "env.json")

	// Check for encrypted env
	if _, err := os.Stat(encPath); err == nil {
		fmt.Printf("    %s\n", yellow.Sprint("(Encrypted)"))
		fmt.Println()
		return
	}

	// Read env.json
	if _, err := os.Stat(envPath); err != nil {
		fmt.Printf("    %s\n", dim.Sprint("(none)"))
		fmt.Println()
		return
	}

	data, err := os.ReadFile(envPath)
	if err != nil {
		fmt.Printf("    %s\n", dim.Sprint("(error reading)"))
		fmt.Println()
		return
	}

	var envVars []types.EnvVar
	if err := json.Unmarshal(data, &envVars); err != nil {
		fmt.Printf("    %s\n", dim.Sprint("(error parsing)"))
		fmt.Println()
		return
	}

	if len(envVars) == 0 {
		fmt.Printf("    %s\n", dim.Sprint("(none)"))
	} else {
		for _, e := range envVars {
			fmt.Printf("    %-20s %s\n", e.Key+":", e.Value)
		}
	}
	fmt.Println()
}

func displayNetworks(container *snapshot.ContainerJSONExport, bold, cyan *color.Color) {
	bold.Println("  Networks:")

	if len(container.Ports) == 0 && len(container.Mounts) == 0 {
		// Check if there are any network info in the extracted container
		// For now, just show none if no ports
		fmt.Printf("    %s\n", dim.Sprint("(none)"))
		fmt.Println()
		return
	}

	// Display port mappings as network info proxy
	seen := make(map[string]bool)
	for _, p := range container.Ports {
		if p.HostPort != "" {
			key := p.ContainerPort + " → " + p.HostPort
			if !seen[key] {
				seen[key] = true
				fmt.Printf("    %s → %s\n", cyan.Sprint(p.ContainerPort), p.HostIP+":"+p.HostPort)
			}
		}
	}

	if len(seen) == 0 {
		fmt.Printf("    %s\n", dim.Sprint("(none)"))
	}
	fmt.Println()
}

func displayMounts(tempDir string, container *snapshot.ContainerJSONExport, bold, cyan *color.Color) {
	bold.Println("  Mounts / Volumes:")

	if len(container.Mounts) == 0 {
		fmt.Printf("    %s\n", dim.Sprint("(none)"))
		fmt.Println()
		return
	}

	// Check for volume data in mounts directory
	mountsDir := filepath.Join(tempDir, "mounts")
	volumeData := make(map[string]bool)
	if info, err := os.Stat(mountsDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(mountsDir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".tar.gz") {
				volName := strings.TrimSuffix(e.Name(), ".tar.gz")
				volumeData[volName] = true
			}
		}
	}

	for _, m := range container.Mounts {
		dataIncluded := "No"
		if m.Type == "volume" && volumeData[m.Name] {
			dataIncluded = cyan.Sprint("Yes")
		}

		source := m.Source
		if m.Type == "volume" {
			source = m.Name
		}

		fmt.Printf("    %s: %s\n", m.Type, cyan.Sprint(source))
		fmt.Printf("      → %s (%s)\n", m.Destination, m.Mode)
		if m.Type == "volume" {
			fmt.Printf("      [Data: %s]\n", dataIncluded)
		}
	}
	fmt.Println()
}

var dim = color.New(color.Faint)
