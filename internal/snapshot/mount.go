package snapshot

import (
	"github.com/fadhlidev/snapdock/internal/docker"
)

// MountCatalog is the result of phase-1 mount mapping.
// It records all mounts but does NOT yet read volume data —
// that happens in Phase 4 (fitur 16-17, --with-volumes).
type MountCatalog struct {
	Binds   []docker.MountInfo `json:"binds"`   // type=bind
	Volumes []docker.MountInfo `json:"volumes"` // type=volume
	Tmpfs   []docker.MountInfo `json:"tmpfs"`   // type=tmpfs
}

// CatalogMounts splits the flat MountInfo slice from a ContainerSnapshot
// into typed buckets (bind / volume / tmpfs).
//
// This separation matters for restore:
//   - Binds   → recreated with -v host_path:container_path
//   - Volumes → need docker volume create + optional data restore
//   - Tmpfs   → recreated with --tmpfs, no data to restore
func CatalogMounts(snap *docker.ContainerSnapshot) MountCatalog {
	cat := MountCatalog{}

	for _, m := range snap.Mounts {
		switch m.Type {
		case "bind":
			cat.Binds = append(cat.Binds, m)
		case "volume":
			cat.Volumes = append(cat.Volumes, m)
		case "tmpfs":
			cat.Tmpfs = append(cat.Tmpfs, m)
		default:
			// Unknown type: treat as bind so restore at least attempts it
			cat.Binds = append(cat.Binds, m)
		}
	}

	return cat
}

// MountArgs converts a MountCatalog back into docker run -v / --tmpfs flags.
// Used by the restore command (Phase 3).
func MountArgs(cat MountCatalog) (binds []string, tmpfs []string) {
	for _, b := range cat.Binds {
		arg := b.Source + ":" + b.Destination
		if b.Mode != "" {
			arg += ":" + b.Mode
		}
		binds = append(binds, arg)
	}

	for _, v := range cat.Volumes {
		arg := v.Name + ":" + v.Destination
		if v.Mode != "" {
			arg += ":" + v.Mode
		}
		binds = append(binds, arg)
	}

	for _, t := range cat.Tmpfs {
		tmpfs = append(tmpfs, t.Destination)
	}

	return binds, tmpfs
}
