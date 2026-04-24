package snapshot

import (
	"time"

	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/pkg/types"
)

// BuildManifest creates a Manifest from a ContainerSnapshot and the
// options chosen by the user at CLI time.
// Checksum is left empty here — it is filled by Packager after the
// .sfx archive has been written to disk.
func BuildManifest(snap *docker.ContainerSnapshot, opts types.SnapOptions) types.Manifest {
	return types.Manifest{
		SnapforgeVersion: types.SnapforgeVersion,
		CreatedAt:        time.Now().UTC(),
		Container: types.ContainerMeta{
			ID:        snap.ID,
			Name:      snap.Name,
			Image:     snap.Image,
			ImageID:   snap.ImageID,
			CreatedAt: snap.CreatedAt,
		},
		Options: opts,
	}
}
