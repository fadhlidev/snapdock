package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/pkg/types"
)

// ExtractedSnapshot holds the parsed contents of a .sfx archive.
type ExtractedSnapshot struct {
	TempDir   string
	Manifest  *types.Manifest
	Container *ContainerJSONExport
	Env       []types.EnvVar
	Networks  []NetworkDetail
	Mounts    MountCatalog
}

// ContainerJSONExport is the structure stored in container.json inside .sfx.
// Mirrors the exportable snapshot structure from packager.go.
type ContainerJSONExport struct {
	ID         string                `json:"id"`
	Name       string                `json:"name"`
	Image      string                `json:"image"`
	ImageID    string                `json:"image_id"`
	CreatedAt  string                `json:"created_at"`
	Env        []string              `json:"env"`
	Labels     map[string]string     `json:"labels"`
	Cmd        []string              `json:"cmd"`
	Entrypoint []string              `json:"entrypoint"`
	WorkingDir string                `json:"working_dir"`
	User       string                `json:"user"`
	Hostname   string                `json:"hostname"`
	StopSignal string                `json:"stop_signal"`
	Ports      []docker.PortMapping  `json:"ports"`
	Mounts     []docker.MountInfo    `json:"mounts"`
	Resources  docker.ResourceConfig `json:"resources"`
}

// Extract unpacks the .sfx archive into a temporary directory and parses
// the essential files (manifest.json, container.json, env.json, networks.json, mounts.json).
//
// The caller is responsible for cleaning up the temporary directory
// with os.RemoveAll(ExtractedSnapshot.TempDir) when done.
func Extract(sfxPath string) (*ExtractedSnapshot, error) {
	if _, err := os.Stat(sfxPath); err != nil {
		return nil, fmt.Errorf("snapshot file not found: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "snapdock-restore-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	if err := extractTarGz(sfxPath, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to extract snapshot: %w", err)
	}

	manifest, err := parseManifest(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	container, err := parseContainerConfig(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	env, err := parseEnv(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	networks, err := parseNetworks(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	mounts, err := parseMounts(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	return &ExtractedSnapshot{
		TempDir:   tmpDir,
		Manifest:  manifest,
		Container: container,
		Env:       env,
		Networks:  networks,
		Mounts:    mounts,
	}, nil
}

// ExtractFile reads a single file from the .sfx archive.
func ExtractFile(sfxPath, fileName string) ([]byte, error) {
	f, err := os.Open(sfxPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if hdr.Name == fileName {
			return io.ReadAll(tr)
		}
	}

	return nil, fmt.Errorf("file %s not found in archive", fileName)
}

// PeekManifest reads only the manifest.json from a snapshot and returns the type.
func PeekManifest(sfxPath string) (any, types.SnapshotType, error) {
	data, err := ExtractFile(sfxPath, "manifest.json")
	if err != nil {
		return nil, "", err
	}

	var header struct {
		SnapshotType types.SnapshotType `json:"snapshot_type"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return nil, "", err
	}

	if header.SnapshotType == types.SnapshotTypeStack {
		var m types.StackManifest
		json.Unmarshal(data, &m)
		return &m, types.SnapshotTypeStack, nil
	}

	var m types.Manifest
	json.Unmarshal(data, &m)
	if m.SnapshotType == "" {
		m.SnapshotType = types.SnapshotTypeContainer
	}
	return &m, types.SnapshotTypeContainer, nil
}

// extractTarGz decompresses a .sfx (tar.gz) file into targetDir.
func extractTarGz(sfxPath, targetDir string) error {
	f, err := os.Open(sfxPath)
	if err != nil {
		return fmt.Errorf("cannot open .sfx file: %w", err)
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("not a valid gzip file: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %w", err)
		}

		cleanPath := filepath.Clean(hdr.Name)
		if cleanPath != hdr.Name || cleanPath == ".." || len(cleanPath) > 100 {
			return fmt.Errorf("invalid file path in archive: %s", hdr.Name)
		}

		targetPath := filepath.Join(targetDir, cleanPath)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, 0o755); err != nil {
				return fmt.Errorf("create directory %s: %w", hdr.Name, err)
			}
		case tar.TypeReg:
			parentDir := filepath.Dir(targetPath)
			if err := os.MkdirAll(parentDir, 0o755); err != nil {
				return fmt.Errorf("create parent dir for %s: %w", hdr.Name, err)
			}
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("create file %s: %w", hdr.Name, err)
			}
			if _, err := io.Copy(outFile, tr); err != nil {
				outFile.Close()
				return fmt.Errorf("write file %s: %w", hdr.Name, err)
			}
			outFile.Close()
		}
	}

	return nil
}

// parseManifest reads manifest.json from the extracted directory.
func parseManifest(tmpDir string) (*types.Manifest, error) {
	data, err := os.ReadFile(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("manifest.json not found in archive: %w", err)
	}

	var manifest types.Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest.json: %w", err)
	}

	return &manifest, nil
}

// parseContainerConfig reads container.json from the extracted directory.
func parseContainerConfig(tmpDir string) (*ContainerJSONExport, error) {
	data, err := os.ReadFile(filepath.Join(tmpDir, "container.json"))
	if err != nil {
		return nil, fmt.Errorf("container.json not found in archive: %w", err)
	}

	var container ContainerJSONExport
	if err := json.Unmarshal(data, &container); err != nil {
		return nil, fmt.Errorf("parse container.json: %w", err)
	}

	return &container, nil
}

// parseEnv reads env.json from the extracted directory.
func parseEnv(tmpDir string) ([]types.EnvVar, error) {
	data, err := os.ReadFile(filepath.Join(tmpDir, "env.json"))
	if err != nil {
		return []types.EnvVar{}, nil
	}

	var env []types.EnvVar
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("parse env.json: %w", err)
	}

	return env, nil
}

// parseNetworks reads networks.json from the extracted directory.
func parseNetworks(tmpDir string) ([]NetworkDetail, error) {
	data, err := os.ReadFile(filepath.Join(tmpDir, "networks.json"))
	if err != nil {
		return []NetworkDetail{}, nil
	}

	var networks []NetworkDetail
	if err := json.Unmarshal(data, &networks); err != nil {
		return nil, fmt.Errorf("parse networks.json: %w", err)
	}

	return networks, nil
}

// parseMounts reads mounts.json from the extracted directory.
func parseMounts(tmpDir string) (MountCatalog, error) {
	data, err := os.ReadFile(filepath.Join(tmpDir, "mounts.json"))
	if err != nil {
		return MountCatalog{}, nil
	}

	var mounts MountCatalog
	if err := json.Unmarshal(data, &mounts); err != nil {
		return MountCatalog{}, fmt.Errorf("parse mounts.json: %w", err)
	}

	return mounts, nil
}

// Cleanup removes the temporary extraction directory.
func (e *ExtractedSnapshot) Cleanup() {
	if e.TempDir != "" {
		os.RemoveAll(e.TempDir)
	}
}