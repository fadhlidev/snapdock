package types

import (
	"time"

	"github.com/fadhlidev/snapdock/internal/docker"
)

const (
	SnapDockVersion = "0.4.0"
	SfxExtension     = ".sfx"
)

// Manifest is written as manifest.json inside every .sfx archive.
// It is the first file read during restore/inspect to understand
// what the archive contains and how to unpack it.
type Manifest struct {
	SnapDockVersion string    `json:"snapdock_version"`
	CreatedAt      time.Time `json:"created_at"`
	Checksum      string    `json:"checksum,omitempty"` // SHA-256 of .sfx, filled after pack
	SnapshotType SnapshotType `json:"snapshot_type,omitempty"`

	Container ContainerMeta `json:"container"`
	Options  SnapOptions   `json:"options"`
}

// ContainerMeta holds identity info extracted from ContainerSnapshot.
type ContainerMeta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Image     string    `json:"image"`
	ImageID   string    `json:"image_id"`
	CreatedAt time.Time `json:"created_at"`
}

// SnapOptions records which optional features were used during snapshot.
type SnapOptions struct {
	WithVolumes bool `json:"with_volumes"`
	Encrypted   bool `json:"encrypted"` // env.json.enc instead of env.json
}

// SnapshotPackage is the in-memory assembly of all snapshot components
// before they are written to disk as a .sfx file.
type SnapshotPackage struct {
	Manifest  Manifest
	Container docker.ContainerSnapshot
	Env       []EnvVar
	Networks  []docker.NetworkInfo
	Mounts    []docker.MountInfo
}

// EnvVar is a parsed KEY=VALUE pair from the container's environment.
type EnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// SnapshotType distinguishes between single-container and compose stack snapshots.
type SnapshotType string

const (
	SnapshotTypeContainer SnapshotType = "container"
	SnapshotTypeStack   SnapshotType = "stack"
)

// StackManifest is written as manifest.json inside stack .sfx archives.
// It includes all services, networks, and volumes from a Compose project.
type StackManifest struct {
	SnapDockVersion string    `json:"snapdock_version"`
	CreatedAt       time.Time `json:"created_at"`
	Checksum       string    `json:"checksum,omitempty"`
	SnapshotType   SnapshotType `json:"snapshot_type"`

	Project    ProjectMeta  `json:"project"`
	Services  []ServiceMeta `json:"services"`
	Networks  []NetworkMeta `json:"networks"`
	Volumes   []VolumeMeta  `json:"volumes"`
	Options   SnapOptions   `json:"options"`
}

// ProjectMeta holds the Compose project identity.
type ProjectMeta struct {
	Name      string    `json:"name"`
	FilePath string    `json:"file_path"`
}

// ServiceMeta holds identity info for a service in the stack.
type ServiceMeta struct {
	Name      string    `json:"name"`
	Image    string    `json:"image"`
	ImageID  string    `json:"image_id"`
	ContainerID string  `json:"container_id"`
}

// NetworkMeta holds network configuration.
type NetworkMeta struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}

// VolumeMeta holds volume configuration.
type VolumeMeta struct {
	Name   string `json:"name"`
	Driver string `json:"driver"`
}
