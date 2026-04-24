package snapshot

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/pkg/types"
)

// PackageResult holds the output paths produced by Pack.
type PackageResult struct {
	SfxPath      string // absolute path to .sfx file
	ChecksumPath string // absolute path to .sha256 file
	Checksum     string // hex SHA-256 digest
	SizeBytes    int64
}

// Pack assembles a complete .sfx snapshot archive for the given container.
//
// The archive layout:
//
//	manifest.json    — Manifest struct
//	container.json   — Full ContainerSnapshot (without Raw field)
//	env.json         — []EnvVar (plaintext; or encrypted if opts.Encrypted)
//	env.json.enc     — encrypted env (if opts.Encrypted)
//	networks.json    — []NetworkDetail
//	mounts.json      — MountCatalog
//	mounts/          — Volume tar files (if --with-volumes)
//
// After writing the archive, SHA-256 is computed and:
//   - Written to <name>.sfx.sha256 on disk
//   - Stored in Manifest.Checksum (for inspection without the .sha256 file)
func Pack(
	ctx context.Context,
	client *docker.Client,
	snap *docker.ContainerSnapshot,
	opts types.SnapOptions,
	outputDir string,
	passphrase string,
) (*PackageResult, error) {
	// 1. Build each component
	manifest := BuildManifest(snap, opts)
	envVars  := ExtractEnv(snap)

	networks, warnings, err := ResolveNetworks(ctx, client, snap)
	if err != nil {
		return nil, fmt.Errorf("network resolution failed: %w", err)
	}
	for _, w := range warnings {
		fmt.Fprintf(os.Stderr, "  ⚠  %s\n", w)
	}

	mounts := CatalogMounts(snap)

	// Create temp directory for volume tars and env encryption
	tempDir, err := os.MkdirTemp("", "snapdock-pack-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write env.json to temp dir for potential encryption
	envPath := filepath.Join(tempDir, "env.json")
	envJSON, err := json.MarshalIndent(envVars, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal env: %w", err)
	}
	if err := os.WriteFile(envPath, envJSON, 0o644); err != nil {
		return nil, fmt.Errorf("write env.json: %w", err)
	}

	// Encrypt env.json if requested
	if opts.Encrypted && passphrase != "" {
		encrypted, err := EncryptEnv(tempDir, passphrase)
		if err != nil {
			return nil, fmt.Errorf("encrypt env: %w", err)
		}
		if encrypted {
			fmt.Fprintf(os.Stderr, "  ✓ environment variables encrypted\n")
		}
	}

	// Snapshot volumes if requested
	var volumeTars VolumeTarInfo
	if opts.WithVolumes {
		volumeTars, err = SnapshotVolumes(ctx, client, snap, tempDir)
		if err != nil {
			return nil, fmt.Errorf("snapshot volumes failed: %w", err)
		}
	}

	// 2. Determine output path
	ts      := time.Now().UTC().Format("2006-01-02T150405")
	sfxName := fmt.Sprintf("%s-%s%s", sanitizeName(snap.Name), ts, types.SfxExtension)
	sfxPath := filepath.Join(outputDir, sfxName)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("cannot create output directory: %w", err)
	}

	// 3. Write tar.gz
	sfxFile, err := os.Create(sfxPath)
	if err != nil {
		return nil, fmt.Errorf("cannot create .sfx file: %w", err)
	}
	defer sfxFile.Close()

	gz := gzip.NewWriter(sfxFile)
	tw := tar.NewWriter(gz)

	// helper: add a JSON-encoded value as a file inside the archive
	addJSON := func(name string, v any) error {
		data, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal %s: %w", name, err)
		}
		hdr := &tar.Header{
			Name:     name,
			Mode:     0o644,
			Size:     int64(len(data)),
			ModTime:  time.Now().UTC(),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write header %s: %w", name, err)
		}
		if _, err := tw.Write(data); err != nil {
			return fmt.Errorf("write data %s: %w", name, err)
		}
		return nil
	}

	// helper: add a binary file to the archive
	addFile := func(archivePath, hostPath string) error {
		data, err := os.ReadFile(hostPath)
		if err != nil {
			return fmt.Errorf("read file %s: %w", hostPath, err)
		}
		hdr := &tar.Header{
			Name:     archivePath,
			Mode:     0o644,
			Size:     int64(len(data)),
			ModTime:  time.Now().UTC(),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("write header %s: %w", archivePath, err)
		}
		if _, err := tw.Write(data); err != nil {
			return fmt.Errorf("write data %s: %w", archivePath, err)
		}
		return nil
	}

	// container.json — strip Raw to keep archive lean
	containerExport := exportableSnapshot(snap)

	files := []struct {
		name string
		val  any
	}{
		{"manifest.json", manifest},   // written first — readers check this first
		{"container.json", containerExport},
		{"networks.json", networks},
		{"mounts.json", mounts},
	}

	for _, f := range files {
		if err := addJSON(f.name, f.val); err != nil {
			return nil, err
		}
	}

	// Add env.json or env.json.enc to archive
	envFiles := []string{"env.json", "env.json.enc"}
	for _, envFile := range envFiles {
		envFilePath := filepath.Join(tempDir, envFile)
		if _, err := os.Stat(envFilePath); err == nil {
			archivePath := envFile
			if err := addFile(archivePath, envFilePath); err != nil {
				return nil, err
			}
			break // only one of them exists
		}
	}

	// Add volume tar files to mounts/ directory
	if len(volumeTars) > 0 {
		for volumeName, tarPath := range volumeTars {
			archivePath := fmt.Sprintf("mounts/%s.tar.gz", volumeName)
			if err := addFile(archivePath, tarPath); err != nil {
				return nil, err
			}
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("finalize tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("finalize gzip: %w", err)
	}
	if err := sfxFile.Close(); err != nil {
		return nil, fmt.Errorf("close .sfx file: %w", err)
	}

	// 4. File size
	info, err := os.Stat(sfxPath)
	if err != nil {
		return nil, fmt.Errorf("stat .sfx file: %w", err)
	}

	// 5. Checksum
	digest, err := WriteChecksum(sfxPath)
	if err != nil {
		return nil, err
	}

	return &PackageResult{
		SfxPath:      sfxPath,
		ChecksumPath: sfxPath + ".sha256",
		Checksum:     digest,
		SizeBytes:    info.Size(),
	}, nil
}

// exportableSnapshot returns a copy of ContainerSnapshot with the Raw field
// zeroed out so it is not double-serialised inside the archive.
// container.json already has all the structured data we need.
func exportableSnapshot(snap *docker.ContainerSnapshot) any {
	return struct {
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
	}{
		ID:         snap.ID,
		Name:       snap.Name,
		Image:      snap.Image,
		ImageID:    snap.ImageID,
		CreatedAt:  snap.CreatedAt.String(),
		Env:        snap.Env,
		Labels:     snap.Labels,
		Cmd:        snap.Cmd,
		Entrypoint: snap.Entrypoint,
		WorkingDir: snap.WorkingDir,
		User:       snap.User,
		Hostname:   snap.Hostname,
		StopSignal: snap.StopSignal,
		Ports:      snap.Ports,
		Mounts:     snap.Mounts,
		Resources:  snap.Resources,
	}
}

// sanitizeName replaces characters that are invalid in filenames.
func sanitizeName(name string) string {
	r := strings.NewReplacer("/", "-", ":", "-", " ", "_")
	return r.Replace(name)
}
