package retention

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fadhlidev/snapdock/pkg/types"
)

func createDummySfx(t *testing.T, path, containerName string, createdAt time.Time) {
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("failed to create dummy sfx: %v", err)
	}
	defer f.Close()

	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	manifest := types.Manifest{
		CreatedAt: createdAt,
		Container: types.ContainerMeta{
			Name: containerName,
		},
	}
	data, _ := json.Marshal(manifest)

	hdr := &tar.Header{
		Name: "manifest.json",
		Mode: 0644,
		Size: int64(len(data)),
	}
	tw.WriteHeader(hdr)
	tw.Write(data)
	
	// Add other required files to avoid extraction errors
	emptyList, _ := json.Marshal([]any{})
	emptyObj, _ := json.Marshal(map[string]any{})
	
	files := []struct {
		name string
		data []byte
	}{
		{"container.json", emptyObj},
		{"env.json", emptyList},
		{"networks.json", emptyList},
		{"mounts.json", emptyObj},
	}

	for _, f := range files {
		hdr := &tar.Header{
			Name: f.name,
			Mode: 0644,
			Size: int64(len(f.data)),
		}
		tw.WriteHeader(hdr)
		tw.Write(f.data)
	}

	tw.Close()
	gw.Close()
}

func TestPruneDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Create 3 snapshots for container A, 2 for container B
	now := time.Now()
	createDummySfx(t, filepath.Join(tmpDir, "a1.sfx"), "container-a", now.Add(-3*time.Hour))
	createDummySfx(t, filepath.Join(tmpDir, "a2.sfx"), "container-a", now.Add(-2*time.Hour))
	createDummySfx(t, filepath.Join(tmpDir, "a3.sfx"), "container-a", now.Add(-1*time.Hour))
	
	createDummySfx(t, filepath.Join(tmpDir, "b1.sfx"), "container-b", now.Add(-2*time.Hour))
	createDummySfx(t, filepath.Join(tmpDir, "b2.sfx"), "container-b", now.Add(-1*time.Hour))

	// Prune container A to keep 2, container B to keep 2
	err := PruneDir(tmpDir, 2)
	if err != nil {
		t.Fatalf("PruneDir failed: %v", err)
	}

	// Verify a1.sfx is gone, others remain
	entries, _ := os.ReadDir(tmpDir)
	files := make(map[string]bool)
	for _, e := range entries {
		files[e.Name()] = true
	}

	if files["a1.sfx"] {
		t.Errorf("a1.sfx should have been pruned")
	}
	if !files["a2.sfx"] || !files["a3.sfx"] {
		t.Errorf("a2.sfx and a3.sfx should remain")
	}
	if !files["b1.sfx"] || !files["b2.sfx"] {
		t.Errorf("b1.sfx and b2.sfx should remain")
	}
}
