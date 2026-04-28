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

func TestDetectSnapshotType(t *testing.T) {
	tests := []struct {
		name         string
		snapshotType types.SnapshotType
		setup       func(string) error
	}{
		{
			name:         "container snapshot",
			snapshotType: types.SnapshotTypeContainer,
			setup: func(dir string) error {
				manifest := types.Manifest{
					SnapDockVersion: types.SnapDockVersion,
					CreatedAt:    time.Now(),
					SnapshotType: types.SnapshotTypeContainer,
					Container: types.ContainerMeta{
						Name:  "test",
						Image: "nginx:latest",
					},
				}
				data, _ := json.MarshalIndent(manifest, "", "  ")
				return os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644)
			},
		},
		{
			name:         "stack snapshot",
			snapshotType: types.SnapshotTypeStack,
			setup: func(dir string) error {
				manifest := types.StackManifest{
					SnapDockVersion: types.SnapDockVersion,
					CreatedAt:    time.Now(),
					SnapshotType: types.SnapshotTypeStack,
					Project: types.ProjectMeta{
						Name: "testproject",
					},
					Services: []types.ServiceMeta{
						{Name: "web", Image: "nginx:latest"},
					},
				}
				data, _ := json.MarshalIndent(manifest, "", "  ")
				return os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644)
			},
		},
		{
			name:         "legacy no snapshot_type",
			snapshotType: types.SnapshotTypeContainer,
			setup: func(dir string) error {
				manifest := types.Manifest{
					SnapDockVersion: types.SnapDockVersion,
					CreatedAt:    time.Now(),
					Container: types.ContainerMeta{
						Name:  "test",
						Image: "nginx:latest",
					},
				}
				data, _ := json.MarshalIndent(manifest, "", "  ")
				return os.WriteFile(filepath.Join(dir, "manifest.json"), data, 0o644)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, err := os.MkdirTemp("", "snapdock-detect-test-*")
			if err != nil {
				t.Fatalf("create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			if err := tt.setup(tmpDir); err != nil {
				t.Fatalf("setup: %v", err)
			}

			sfxPath := filepath.Join(tmpDir, "test.sfx")
			if err := createTestTarGz(sfxPath, tmpDir); err != nil {
				t.Fatalf("create tar: %v", err)
			}

			detected, err := DetectSnapshotType(sfxPath)
			if err != nil {
				t.Fatalf("DetectSnapshotType: %v", err)
			}

			if detected != tt.snapshotType {
				t.Errorf("expected %q, got %q", tt.snapshotType, detected)
			}
		})
	}
}

func createTestTarGz(sfxPath, sourceDir string) error {
	sfx, err := os.Create(sfxPath)
	if err != nil {
		return err
	}
	defer sfx.Close()

	gz := gzip.NewWriter(sfx)
	tw := tar.NewWriter(gz)

	files := []string{"manifest.json"}
	for _, name := range files {
		data, _ := os.ReadFile(filepath.Join(sourceDir, name))
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(data)),
			ModTime:  time.Now(),
			Typeflag: tar.TypeReg,
		}
		tw.WriteHeader(hdr)
		tw.Write(data)
	}

	tw.Close()
	gz.Close()
	sfx.Close()
	return nil
}

func TestExtractStack_ManifestValidation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "snapdock-extract-stack-test-*")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	manifest := types.StackManifest{
		SnapDockVersion: types.SnapDockVersion,
		CreatedAt:     time.Now(),
		SnapshotType:  types.SnapshotTypeStack,
		Project:      types.ProjectMeta{Name: "test"},
		Services:     []types.ServiceMeta{{Name: "web", Image: "nginx:latest"}},
		Networks:     []types.NetworkMeta{{Name: "default", Driver: "bridge"}},
		Volumes:      []types.VolumeMeta{{Name: "data"}},
	}
	data, _ := json.MarshalIndent(manifest, "", "  ")
	os.WriteFile(filepath.Join(tmpDir, "manifest.json"), data, 0o644)

	os.MkdirAll(filepath.Join(tmpDir, "services", "web"), 0o755)
	containerData, _ := json.Marshal(map[string]string{"name": "web", "image": "nginx:latest"})
	os.WriteFile(filepath.Join(tmpDir, "services", "web", "container.json"), containerData, 0o644)

	os.WriteFile(filepath.Join(tmpDir, "compose.yaml"), []byte("services:\n  web:\n    image: nginx:latest"), 0o644)

	sfxPath := filepath.Join(tmpDir, "test-stack.sfx")
	if err := createStackTarGz(sfxPath, tmpDir); err != nil {
		t.Fatalf("create tar: %v", err)
	}

	extracted, err := ExtractStack(sfxPath)
	if err != nil {
		t.Fatalf("ExtractStack: %v", err)
	}
	defer extracted.Cleanup()

	if extracted.Manifest.Project.Name != "test" {
		t.Errorf("expected project name 'test', got %q", extracted.Manifest.Project.Name)
	}
	if len(extracted.Manifest.Services) != 1 {
		t.Errorf("expected 1 service, got %d", len(extracted.Manifest.Services))
	}
}

func createStackTarGz(sfxPath, sourceDir string) error {
	sfx, err := os.Create(sfxPath)
	if err != nil {
		return err
	}
	defer sfx.Close()

	gz := gzip.NewWriter(sfx)
	tw := tar.NewWriter(gz)

	entries, _ := os.ReadDir(sourceDir)
	for _, entry := range entries {
		srcPath := filepath.Join(sourceDir, entry.Name())
		if entry.IsDir() {
			addDirToTar(tw, entry.Name(), srcPath)
		} else {
			data, _ := os.ReadFile(srcPath)
			hdr := &tar.Header{
				Name:     entry.Name(),
				Mode:     0o644,
				Size:     int64(len(data)),
				ModTime:  time.Now(),
				Typeflag: tar.TypeReg,
			}
			tw.WriteHeader(hdr)
			tw.Write(data)
		}
	}

	tw.Close()
	gz.Close()
	sfx.Close()
	return nil
}

func addDirToTar(tw *tar.Writer, name, srcDir string) {
	parentDir := filepath.Dir(name)
	if parentDir != "." {
		hdr := &tar.Header{
			Name:     parentDir,
			Mode:     0o755,
			Typeflag: tar.TypeDir,
		}
		tw.WriteHeader(hdr)
	}

	entries, _ := os.ReadDir(srcDir)
	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		entryName := filepath.Join(name, entry.Name())

		if entry.IsDir() {
			hdr := &tar.Header{
				Name:     entryName,
				Mode:     0o755,
				Typeflag: tar.TypeDir,
			}
			tw.WriteHeader(hdr)
			addDirToTar(tw, entryName, srcPath)
		} else {
			data, _ := os.ReadFile(srcPath)
			hdr := &tar.Header{
				Name:     entryName,
				Mode:     0o644,
				Size:     int64(len(data)),
				ModTime:  time.Now(),
				Typeflag: tar.TypeReg,
			}
			tw.WriteHeader(hdr)
			tw.Write(data)
		}
	}
}