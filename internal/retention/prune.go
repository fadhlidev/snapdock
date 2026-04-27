package retention

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fadhlidev/snapdock/internal/output"
	"github.com/fadhlidev/snapdock/internal/snapshot"
)

type SnapshotInfo struct {
	Path      string
	Container string
	CreatedAt int64 // timestamp
}

// PruneDir applies retention rules to a directory.
func PruneDir(dirPath string, keepLast int) error {
	if keepLast <= 0 {
		return nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}

	// Group snapshots by container name
	containerGroups := make(map[string][]SnapshotInfo)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sfx") {
			continue
		}

		fullPath := filepath.Join(dirPath, entry.Name())
		
		// We need to peek into the manifest to get the container name and created date.
		// For efficiency, we'll try to use the reader logic.
		extracted, err := snapshot.Extract(fullPath)
		if err != nil {
			output.Warningf("Skipping %s during prune: failed to read manifest", entry.Name())
			continue
		}
		
		name := extracted.Manifest.Container.Name
		createdAt := extracted.Manifest.CreatedAt.Unix()
		extracted.Cleanup()

		containerGroups[name] = append(containerGroups[name], SnapshotInfo{
			Path:      fullPath,
			Container: name,
			CreatedAt: createdAt,
		})
	}

	for containerName, snapshots := range containerGroups {
		if len(snapshots) <= keepLast {
			continue
		}

		// Sort by date (oldest first)
		sort.Slice(snapshots, func(i, j int) bool {
			return snapshots[i].CreatedAt < snapshots[j].CreatedAt
		})

		// Identify files to delete
		toDelete := snapshots[:len(snapshots)-keepLast]
		
		output.Infof("Pruning %d old snapshots for container '%s'...", len(toDelete), containerName)
		for _, s := range toDelete {
			if err := os.Remove(s.Path); err != nil {
				output.Errorf("Failed to delete %s: %v", filepath.Base(s.Path), err)
			} else {
				fmt.Printf("  %s %s\n", "🗑️", filepath.Base(s.Path))
			}
		}
	}

	return nil
}
