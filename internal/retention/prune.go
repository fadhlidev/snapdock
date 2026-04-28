package retention

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/fadhlidev/snapdock/internal/output"
	"github.com/fadhlidev/snapdock/internal/snapshot"
	"github.com/fadhlidev/snapdock/pkg/types"
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
		m, snapType, err := snapshot.PeekManifest(fullPath)
		if err != nil {
			output.Warningf("Skipping %s during prune: failed to read manifest", entry.Name())
			continue
		}

		var name string
		var createdAt int64
		if snapType == types.SnapshotTypeStack {
			sm := m.(*types.StackManifest)
			name = sm.Project.Name
			createdAt = sm.CreatedAt.Unix()
		} else {
			cm := m.(*types.Manifest)
			name = cm.Container.Name
			createdAt = cm.CreatedAt.Unix()
		}

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
