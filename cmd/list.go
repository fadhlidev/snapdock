package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/fadhlidev/snapdock/internal/output"
	"github.com/fadhlidev/snapdock/internal/snapshot"
	"github.com/fadhlidev/snapdock/pkg/types"
)

var listCmd = &cobra.Command{
	Use:   "list [directory]",
	Short: "List snapshots in a directory",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runList,
}

func init() {
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	dirPath := "."
	if len(args) > 0 {
		dirPath = args[0]
	}

	// Validate directory
	info, err := os.Stat(dirPath)
	if err != nil {
		return fmt.Errorf("directory not found: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory: %s", dirPath)
	}

	// Scan for .sfx files
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return fmt.Errorf("failed to read directory: %w", err)
	}

	type snapshotInfo struct {
		name      string
		container string
		id        string
		size      string
		date      string
	}

	var snapshots []snapshotInfo

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".sfx") {
			continue
		}

		sfxPath := filepath.Join(dirPath, name)

		// Get file info
		fileInfo, err := os.Stat(sfxPath)
		if err != nil {
			continue
		}

		// Extract manifest
		extracted, err := extractManifest(sfxPath)
		if err != nil {
			continue
		}

		// Format size
		size := formatSize(fileInfo.Size())

		// Format date
		date := fileInfo.ModTime().Format("Jan 2, 2006 03:04 PM")

		// Shorten ID
		id := extracted.Container.ID
		if len(id) > 12 {
			id = id[:12]
		}

		snapshots = append(snapshots, snapshotInfo{
			name:      name,
			container: extracted.Container.Name,
			id:        id,
			size:      size,
			date:      date,
		})
	}

	if len(snapshots) == 0 {
		output.Info("No snapshots found")
		return nil
	}

	var data [][]string
	for _, s := range snapshots {
		data = append(data, []string{s.name, s.container, s.id, s.size, s.date})
	}

	output.PrintTable([]string{"NAME", "CONTAINER", "ID", "SIZE", "CREATED"}, data)
	fmt.Println()

	return nil
}

func extractManifest(sfxPath string) (*types.Manifest, error) {
	extracted, err := snapshot.Extract(sfxPath)
	if err != nil {
		return nil, err
	}
	defer extracted.Cleanup()
	return extracted.Manifest, nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
