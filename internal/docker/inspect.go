package docker

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	dockerclient "github.com/docker/docker/client"
)

// ─── Internal snapshot types ────────────────────────────────────────────────

// ContainerSnapshot is SnapDock's normalised representation of a container's
// full runtime state. All fields map 1-to-1 to docker inspect output so that
// restoring from this struct produces an equivalent container.
type ContainerSnapshot struct {
	// Identity
	ID        string    `json:"id"`
	Name      string    `json:"name"`       // without leading "/"
	Image     string    `json:"image"`      // repository:tag
	ImageID   string    `json:"image_id"`
	CreatedAt time.Time `json:"created_at"`

	// Runtime config
	Env         []string          `json:"env"`          // KEY=VALUE pairs
	Labels      map[string]string `json:"labels"`
	Cmd         []string          `json:"cmd"`
	Entrypoint  []string          `json:"entrypoint"`
	WorkingDir  string            `json:"working_dir"`
	User        string            `json:"user"`
	Hostname    string            `json:"hostname"`
	StopSignal  string            `json:"stop_signal"`

	// Networking
	Networks []NetworkInfo `json:"networks"`
	Ports    []PortMapping `json:"ports"`

	// Storage
	Mounts []MountInfo `json:"mounts"`

	// Resource limits
	Resources ResourceConfig `json:"resources"`

	// Raw docker inspect payload — kept for full-fidelity restore
	Raw types.ContainerJSON `json:"raw"`
}

// NetworkInfo describes a single network the container is attached to.
type NetworkInfo struct {
	Name      string   `json:"name"`
	NetworkID string   `json:"network_id"`
	IPAddress string   `json:"ip_address"`
	Aliases   []string `json:"aliases"`
	Driver    string   `json:"driver"`
}

// PortMapping describes a host↔container port binding.
type PortMapping struct {
	ContainerPort string `json:"container_port"` // e.g. "8080/tcp"
	HostIP        string `json:"host_ip"`
	HostPort      string `json:"host_port"`
}

// MountInfo describes a volume or bind-mount.
type MountInfo struct {
	Type        string `json:"type"`         // "bind" | "volume" | "tmpfs"
	Source      string `json:"source"`       // host path or volume name
	Destination string `json:"destination"`  // container path
	Mode        string `json:"mode"`         // "ro" | "rw"
	Name        string `json:"name"`         // volume name (empty for bind)
}

// ResourceConfig holds CPU / memory limits.
type ResourceConfig struct {
	CPUShares  int64 `json:"cpu_shares"`
	CPUQuota   int64 `json:"cpu_quota"`
	MemoryMB   int64 `json:"memory_mb"`   // 0 = unlimited
	MemSwapMB  int64 `json:"mem_swap_mb"` // 0 = unlimited
}

// ─── Inspect ────────────────────────────────────────────────────────────────

// InspectContainer fetches full docker inspect data for the given container
// name or ID, then maps it into a ContainerSnapshot.
func (c *Client) InspectContainer(ctx context.Context, nameOrID string) (*ContainerSnapshot, error) {
	raw, err := c.cli.ContainerInspect(ctx, nameOrID)
	if err != nil {
		if dockerclient.IsErrNotFound(err) {
			return nil, fmt.Errorf("container %q not found — is it running?", nameOrID)
		}
		return nil, fmt.Errorf("inspect failed: %w", err)
	}

	snap := &ContainerSnapshot{Raw: raw}

	if err := mapIdentity(snap, raw); err != nil {
		return nil, err
	}
	mapRuntimeConfig(snap, raw)
	mapNetworks(snap, raw)
	mapPorts(snap, raw)
	mapMounts(snap, raw)
	mapResources(snap, raw)

	return snap, nil
}

func mapIdentity(s *ContainerSnapshot, raw types.ContainerJSON) error {
	s.ID = raw.ID

	// Strip leading "/" from container name
	name := raw.Name
	if len(name) > 0 && name[0] == '/' {
		name = name[1:]
	}
	s.Name = name

	s.ImageID = raw.Image

	// Prefer the RepoTag; fall back to image ID if image has no tags yet
	if raw.Config != nil {
		s.Image = raw.Config.Image
	}
	if s.Image == "" {
		s.Image = raw.Image
	}

	t, err := time.Parse(time.RFC3339Nano, raw.Created)
	if err != nil {
		// Non-fatal: keep zero time
		t = time.Time{}
	}
	s.CreatedAt = t

	return nil
}

func mapRuntimeConfig(s *ContainerSnapshot, raw types.ContainerJSON) {
	cfg := raw.Config
	if cfg == nil {
		return
	}

	s.Env        = cfg.Env
	s.Labels     = cfg.Labels
	s.Cmd        = []string(cfg.Cmd)
	s.Entrypoint = []string(cfg.Entrypoint)
	s.WorkingDir = cfg.WorkingDir
	s.User       = cfg.User
	s.Hostname   = cfg.Hostname
	s.StopSignal = cfg.StopSignal
}

func mapNetworks(s *ContainerSnapshot, raw types.ContainerJSON) {
	if raw.NetworkSettings == nil {
		return
	}

	for name, ep := range raw.NetworkSettings.Networks {
		if ep == nil {
			continue
		}
		s.Networks = append(s.Networks, NetworkInfo{
			Name:      name,
			NetworkID: ep.NetworkID,
			IPAddress: ep.IPAddress,
			Aliases:   ep.Aliases,
		})
	}
}

func mapPorts(s *ContainerSnapshot, raw types.ContainerJSON) {
	if raw.NetworkSettings == nil {
		return
	}

	for containerPort, bindings := range raw.NetworkSettings.Ports {
		if bindings == nil {
			// Exposed but not published — still record it
			s.Ports = append(s.Ports, PortMapping{
				ContainerPort: string(containerPort),
			})
			continue
		}
		for _, b := range bindings {
			s.Ports = append(s.Ports, PortMapping{
				ContainerPort: string(containerPort),
				HostIP:        b.HostIP,
				HostPort:      b.HostPort,
			})
		}
	}
}

func mapMounts(s *ContainerSnapshot, raw types.ContainerJSON) {
	for _, m := range raw.Mounts {
		s.Mounts = append(s.Mounts, MountInfo{
			Type:        string(m.Type),
			Source:      m.Source,
			Destination: m.Destination,
			Mode:        m.Mode,
			Name:        m.Name,
		})
	}
}

func mapResources(s *ContainerSnapshot, raw types.ContainerJSON) {
	if raw.HostConfig == nil {
		return
	}
	res := raw.HostConfig.Resources

	s.Resources = ResourceConfig{
		CPUShares: res.CPUShares,
		CPUQuota:  res.CPUQuota,
		MemoryMB:  bytesToMB(res.Memory),
		MemSwapMB: bytesToMB(res.MemorySwap),
	}
}

func bytesToMB(b int64) int64 {
	if b <= 0 {
		return 0
	}
	return b / (1024 * 1024)
}

// ListContainers returns a summary of running containers (name + ID + image).
// Useful for the CLI's autocompletion and picker UX.
func (c *Client) ListContainers(ctx context.Context) ([]container.Summary, error) {
	containers, err := c.cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("status", "running")),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	return containers, nil
}
