package snapshot

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

func TestExtractFile(t *testing.T) {
	tmpDir := t.TempDir()
	sfxPath := filepath.Join(tmpDir, "test.sfx")

	// Create a dummy .sfx
	f, _ := os.Create(sfxPath)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	content := "hello world"
	hdr := &tar.Header{
		Name: "test.txt",
		Mode: 0644,
		Size: int64(len(content)),
	}
	tw.WriteHeader(hdr)
	tw.Write([]byte(content))
	tw.Close()
	gz.Close()
	f.Close()

	data, err := ExtractFile(sfxPath, "test.txt")
	if err != nil {
		t.Fatalf("ExtractFile failed: %v", err)
	}
	if string(data) != content {
		t.Errorf("expected %q, got %q", content, string(data))
	}

	_, err = ExtractFile(sfxPath, "nonexistent.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestPeekManifest(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("ContainerSnapshot", func(t *testing.T) {
		sfxPath := filepath.Join(tmpDir, "container.sfx")
		m := types.Manifest{
			SnapDockVersion: "0.3.0",
			CreatedAt:       time.Now().UTC(),
			SnapshotType:    types.SnapshotTypeContainer,
			Container: types.ContainerMeta{
				Name: "test-container",
			},
		}
		createDummySFX(sfxPath, "manifest.json", m)

		peeked, snapType, err := PeekManifest(sfxPath)
		if err != nil {
			t.Fatalf("PeekManifest failed: %v", err)
		}
		if snapType != types.SnapshotTypeContainer {
			t.Errorf("expected type container, got %s", snapType)
		}
		cm := peeked.(*types.Manifest)
		if cm.Container.Name != "test-container" {
			t.Errorf("expected container name test-container, got %s", cm.Container.Name)
		}
	})

	t.Run("StackSnapshot", func(t *testing.T) {
		sfxPath := filepath.Join(tmpDir, "stack.sfx")
		m := types.StackManifest{
			SnapDockVersion: "0.3.0",
			CreatedAt:       time.Now().UTC(),
			SnapshotType:    types.SnapshotTypeStack,
			Project: types.ProjectMeta{
				Name: "test-project",
			},
		}
		createDummySFX(sfxPath, "manifest.json", m)

		peeked, snapType, err := PeekManifest(sfxPath)
		if err != nil {
			t.Fatalf("PeekManifest failed: %v", err)
		}
		if snapType != types.SnapshotTypeStack {
			t.Errorf("expected type stack, got %s", snapType)
		}
		sm := peeked.(*types.StackManifest)
		if sm.Project.Name != "test-project" {
			t.Errorf("expected project name test-project, got %s", sm.Project.Name)
		}
	})
}

func createDummySFX(path, fileName string, v any) {
	f, _ := os.Create(path)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	data, _ := json.Marshal(v)
	hdr := &tar.Header{
		Name: fileName,
		Mode: 0644,
		Size: int64(len(data)),
	}
	tw.WriteHeader(hdr)
	tw.Write(data)
	tw.Close()
	gz.Close()
	f.Close()
}
