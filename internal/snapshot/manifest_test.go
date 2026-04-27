package snapshot

import (
	"testing"
	"time"

	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/pkg/types"
)

func TestBuildManifest(t *testing.T) {
	now := time.Now().UTC()
	snap := &docker.ContainerSnapshot{
		ID:        "id123",
		Name:      "name123",
		Image:     "image123",
		ImageID:   "image-id-123",
		CreatedAt: now,
	}

	opts := types.SnapOptions{
		WithVolumes: true,
		Encrypted:   false,
	}

	manifest := BuildManifest(snap, opts)

	if manifest.SnapDockVersion != types.SnapDockVersion {
		t.Errorf("expected version %s, got %s", types.SnapDockVersion, manifest.SnapDockVersion)
	}

	if manifest.Container.ID != snap.ID {
		t.Errorf("expected ID %s, got %s", snap.ID, manifest.Container.ID)
	}

	if manifest.Options.WithVolumes != true {
		t.Errorf("expected WithVolumes to be true")
	}

	// CreatedAt should be recent
	if time.Since(manifest.CreatedAt) > time.Second {
		t.Errorf("manifest CreatedAt is too old: %v", manifest.CreatedAt)
	}
}
