package types

import (
	"time"

	"github.com/fadhlidev/snapdock/internal/docker"
)

const (
	SnapforgeVersion = "0.1.0"
	SfxExtension     = ".sfx"
)

// Manifest is written as manifest.json inside every .sfx archive.
// It is the first file read during restore/inspect to understand
// what the archive contains and how to unpack it.
type Manifest struct {
	SnapforgeVersion string    `json:"snapforge_version"`
	CreatedAt        time.Time `json:"created_at"`
	Checksum         string    `json:"checksum,omitempty"` // SHA-256 of .sfx, filled after pack

	Container ContainerMeta `json:"container"`
	Options   SnapOptions   `json:"options"`
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
