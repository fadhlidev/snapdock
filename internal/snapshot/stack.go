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

	"github.com/fadhlidev/snapdock/internal/compose"
	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/pkg/types"
)

type StackPackageResult struct {
	SfxPath       string
	ChecksumPath string
	Checksum     string
	SizeBytes    int64
	ServiceCount int
}

type RestoreOptions struct {
	NewName      string
	WithVolumes bool
	DryRun       bool
}

func PackStack(
	ctx context.Context,
	client *docker.Client,
	project *compose.Project,
	opts types.SnapOptions,
	outputDir string,
	passphrase string,
) (*StackPackageResult, error) {
	tempDir, err := os.MkdirTemp("", "snapdock-stack-pack-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	var serviceMetas []types.ServiceMeta
	var allEnvVars []map[string][]types.EnvVar
	var allNetworks [][]NetworkDetail
	var allMounts []MountCatalog

	for _, svc := range project.Services {
		snap, err := client.InspectContainer(ctx, svc.Name)
		if err != nil {
			return nil, fmt.Errorf("inspect service %s: %w", svc.Name, err)
		}

		serviceMetas = append(serviceMetas, types.ServiceMeta{
			Name:      snap.Name,
			Image:    snap.Image,
			ImageID:  snap.ImageID,
			ContainerID: snap.ID,
		})

		envVars := ExtractEnv(snap)
		envPath := filepath.Join(tempDir, fmt.Sprintf("env-%s.json", svc.Name))
		envJSON, _ := json.MarshalIndent(envVars, "", "  ")
		os.WriteFile(envPath, envJSON, 0o644)

		if opts.Encrypted && passphrase != "" {
			encrypted, _ := EncryptEnvToFile(envPath, passphrase, envPath+".enc")
			if encrypted {
				os.Remove(envPath)
			}
		}

		networks, _, err := ResolveNetworks(ctx, client, snap)
		if err != nil {
			return nil, fmt.Errorf("resolve networks for %s: %w", svc.Name, err)
		}

		mounts := CatalogMounts(snap)

		allEnvVars = append(allEnvVars, map[string][]types.EnvVar{svc.Name: envVars})
		allNetworks = append(allNetworks, networks)
		allMounts = append(allMounts, mounts)
	}

	var networkMetas []types.NetworkMeta
	for _, net := range project.Networks {
		networkMetas = append(networkMetas, types.NetworkMeta{
			Name:   net.Name,
			Driver: net.Driver,
		})
	}

	var volumeMetas []types.VolumeMeta
	for _, vol := range project.Volumes {
		volumeMetas = append(volumeMetas, types.VolumeMeta{
			Name:   vol.Name,
			Driver: vol.Driver,
		})
	}

	manifest := types.StackManifest{
		SnapDockVersion: types.SnapDockVersion,
		CreatedAt:       time.Now().UTC(),
		SnapshotType:   types.SnapshotTypeStack,
		Project: types.ProjectMeta{
			Name:     project.Name,
			FilePath: project.FilePath,
		},
		Services: serviceMetas,
		Networks: networkMetas,
		Volumes:  volumeMetas,
		Options:  opts,
	}

	composeCopy := filepath.Join(tempDir, "compose.yaml")
	if err := copyFile(project.FilePath, composeCopy); err != nil {
		return nil, fmt.Errorf("copy compose file: %w", err)
	}

	ts := time.Now().UTC().Format("2006-01-02T150405")
	sfxName := fmt.Sprintf("%s-stack-%s%s", project.Name, ts, types.SfxExtension)
	sfxPath := filepath.Join(outputDir, sfxName)

	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return nil, fmt.Errorf("create output dir: %w", err)
	}

	sfxFile, err := os.Create(sfxPath)
	if err != nil {
		return nil, fmt.Errorf("create sfx file: %w", err)
	}
	defer sfxFile.Close()

	gz := gzip.NewWriter(sfxFile)
	tw := tar.NewWriter(gz)

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

	if err := addJSON("manifest.json", manifest); err != nil {
		return nil, err
	}

	composeData, _ := os.ReadFile(composeCopy)
	composeJSON := make(map[string]any)
	json.Unmarshal(composeData, &composeJSON)
	if err := addJSON("compose.json", composeJSON); err != nil {
		return nil, err
	}

	for i, svc := range project.Services {
		serviceExport := exportableSnapshot(&docker.ContainerSnapshot{
			ID:         serviceMetas[i].ContainerID,
			Name:       svc.Name,
			Image:      svc.Image,
			ImageID:    serviceMetas[i].ImageID,
			CreatedAt:  time.Now().UTC(),
		})

		if err := addJSON(fmt.Sprintf("services/%s.json", svc.Name), serviceExport); err != nil {
			return nil, err
		}

		envFiles := []string{
			fmt.Sprintf("env-%s.json", svc.Name),
			fmt.Sprintf("env-%s.json.enc", svc.Name),
		}
		for _, envFile := range envFiles {
			envPath := filepath.Join(tempDir, envFile)
			if _, err := os.Stat(envPath); err == nil {
				archivePath := fmt.Sprintf("services/%s/%s", svc.Name, envFile)
				if err := addFile(archivePath, envPath); err != nil {
					return nil, err
				}
				break
			}
		}

		if err := addJSON(fmt.Sprintf("networks-%s.json", svc.Name), allNetworks[i]); err != nil {
			return nil, err
		}

		if err := addJSON(fmt.Sprintf("mounts-%s.json", svc.Name), allMounts[i]); err != nil {
			return nil, err
		}
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("close gzip: %w", err)
	}
	if err := sfxFile.Close(); err != nil {
		return nil, fmt.Errorf("close sfx: %w", err)
	}

	info, err := os.Stat(sfxPath)
	if err != nil {
		return nil, fmt.Errorf("stat sfx: %w", err)
	}

	digest, err := WriteChecksum(sfxPath)
	if err != nil {
		return nil, err
	}

	return &StackPackageResult{
		SfxPath:       sfxPath,
		ChecksumPath: sfxPath + ".sha256",
		Checksum:     digest,
		SizeBytes:    info.Size(),
		ServiceCount: len(project.Services),
	}, nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

type ExtractedStack struct {
	TempDir   string
	Manifest  *types.StackManifest
	Compose   *compose.Project
	Services  map[string]*ContainerJSONExport
	Envs      map[string][]types.EnvVar
	Networks  map[string][]NetworkDetail
	Mounts    map[string]MountCatalog
}

func ExtractStack(sfxPath string) (*ExtractedStack, error) {
	if _, err := os.Stat(sfxPath); err != nil {
		return nil, fmt.Errorf("snapshot file not found: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "snapdock-stack-restore-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	if err := extractTarGz(sfxPath, tmpDir); err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("failed to extract snapshot: %w", err)
	}

	manifest, err := parseStackManifest(tmpDir)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	composeFile := filepath.Join(tmpDir, "compose.yaml")
	project, err := compose.ParseComposeFile(composeFile)
	if err != nil {
		os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("parse compose: %w", err)
	}

	services := make(map[string]*ContainerJSONExport)
	envs := make(map[string][]types.EnvVar)
	networks := make(map[string][]NetworkDetail)
	mounts := make(map[string]MountCatalog)

	for _, svc := range manifest.Services {
		svcDir := filepath.Join(tmpDir, "services", svc.Name)
		containerPath := filepath.Join(svcDir, "container.json")
		if data, err := os.ReadFile(containerPath); err == nil {
			var c ContainerJSONExport
			json.Unmarshal(data, &c)
			services[svc.Name] = &c
		}

		envPath := filepath.Join(svcDir, "env-"+svc.Name+".json")
		encEnvPath := filepath.Join(svcDir, "env-"+svc.Name+".json.enc")
		if _, err := os.Stat(encEnvPath); err == nil {
		} else if _, err := os.Stat(envPath); err == nil {
			if data, err := os.ReadFile(envPath); err == nil {
				var e []types.EnvVar
				json.Unmarshal(data, &e)
				envs[svc.Name] = e
			}
		}

		netPath := filepath.Join(tmpDir, fmt.Sprintf("networks-%s.json", svc.Name))
		if data, err := os.ReadFile(netPath); err == nil {
			var n []NetworkDetail
			json.Unmarshal(data, &n)
			networks[svc.Name] = n
		}

		mountPath := filepath.Join(tmpDir, fmt.Sprintf("mounts-%s.json", svc.Name))
		if data, err := os.ReadFile(mountPath); err == nil {
			var m MountCatalog
			json.Unmarshal(data, &m)
			mounts[svc.Name] = m
		}
	}

	return &ExtractedStack{
		TempDir:  tmpDir,
		Manifest: manifest,
		Compose:  project,
		Services: services,
		Envs:     envs,
		Networks: networks,
		Mounts:  mounts,
	}, nil
}

func parseStackManifest(tmpDir string) (*types.StackManifest, error) {
	data, err := os.ReadFile(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		return nil, fmt.Errorf("manifest.json not found in archive: %w", err)
	}

	var manifest types.StackManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("parse manifest.json: %w", err)
	}

	if manifest.SnapshotType != types.SnapshotTypeStack {
		return nil, fmt.Errorf("not a stack snapshot: expected snapshot_type=%q, got %q", types.SnapshotTypeStack, manifest.SnapshotType)
	}

	return &manifest, nil
}

func (e *ExtractedStack) Cleanup() {
	if e.TempDir != "" {
		os.RemoveAll(e.TempDir)
	}
}

func DetectSnapshotType(sfxPath string) (types.SnapshotType, error) {
	tmpDir, err := os.MkdirTemp("", "snapdock-detect-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if err := extractTarGz(sfxPath, tmpDir); err != nil {
		return "", fmt.Errorf("extract: %w", err)
	}

	data, err := os.ReadFile(filepath.Join(tmpDir, "manifest.json"))
	if err != nil {
		return "", fmt.Errorf("read manifest: %w", err)
	}

	var m struct {
		SnapshotType types.SnapshotType `json:"snapshot_type"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("parse manifest: %w", err)
	}

	if m.SnapshotType == "" {
		return types.SnapshotTypeContainer, nil
	}
	return m.SnapshotType, nil
}

func RestoreStack(
	ctx context.Context,
	client *docker.Client,
	extracted *ExtractedStack,
	opts RestoreOptions,
	passphrase string,
) error {
	for _, net := range extracted.Manifest.Networks {
		exists, err := client.NetworkExists(ctx, net.Name)
		if err != nil {
			return fmt.Errorf("check network %s: %w", net.Name, err)
		}
		if !exists {
			netCfg := docker.NetworkConfig{Driver: net.Driver}
			_, err := client.CreateNetwork(ctx, net.Name, netCfg)
			if err != nil {
				return fmt.Errorf("create network %s: %w", net.Name, err)
			}
		}
	}

	for _, vol := range extracted.Manifest.Volumes {
		exists, err := client.VolumeExists(ctx, vol.Name)
		if err != nil {
			return fmt.Errorf("check volume %s: %w", vol.Name, err)
		}
		if !exists {
			_, err := client.CreateVolume(ctx, vol.Name)
			if err != nil {
				return fmt.Errorf("create volume %s: %w", vol.Name, err)
			}
		}
	}

	for _, svc := range extracted.Compose.Services {
		svcDir := filepath.Join(extracted.TempDir, "services", svc.Name)
		containerPath := filepath.Join(svcDir, "container.json")

		var container ContainerJSONExport
		if data, err := os.ReadFile(containerPath); err == nil {
			json.Unmarshal(data, &container)
		}

		img := svc.Image
		if img == "" && container.Image != "" {
			img = container.Image
		}
		if img != "" {
			_, err := client.PullImageIfMissing(ctx, img, os.Stdout)
			if err != nil {
				return fmt.Errorf("pull image %s: %w", img, err)
			}
		}

		containerName := svc.Name
		if opts.NewName != "" {
			containerName = opts.NewName + "-" + svc.Name
		}

		cfg := &docker.ContainerConfig{
			Name:    containerName,
			Image:   img,
			Cmd:    splitCmd(svc.Command),
		}

		if envVars, ok := extracted.Envs[svc.Name]; ok {
			for _, e := range envVars {
				cfg.Env = append(cfg.Env, e.Key+"="+e.Value)
			}
		} else if len(container.Env) > 0 {
			cfg.Env = container.Env
		}

		ports := extractPorts(svc)
		if len(ports) > 0 {
			cfg.PortBindings = make(map[string][]docker.PortBinding)
			for _, p := range ports {
				cfg.PortBindings[fmt.Sprintf("%d/tcp", p.Target)] = []docker.PortBinding{
					{HostPort: fmt.Sprintf("%d", p.Published)},
				}
			}
		}

		result, err := client.CreateContainer(ctx, *cfg)
		if err != nil {
			return fmt.Errorf("create container %s: %w", containerName, err)
		}

		if err := client.StartContainer(ctx, result.ID); err != nil {
			return fmt.Errorf("start container %s: %w", containerName, err)
		}
	}

	return nil
}

func splitCmd(cmd string) []string {
	if cmd == "" {
		return nil
	}
	return strings.Fields(cmd)
}

func extractPorts(svc compose.Service) []compose.PortMapping {
	if len(svc.Ports) > 0 {
		return svc.Ports
	}
	return nil
}