package snapshot

import (
	"testing"

	"github.com/fadhlidev/snapdock/internal/docker"
)

func TestCatalogMounts(t *testing.T) {
	snap := &docker.ContainerSnapshot{
		Mounts: []docker.MountInfo{
			{Type: "bind", Source: "/host/path", Destination: "/container/path", Mode: "rw"},
			{Type: "volume", Name: "myvol", Destination: "/data", Mode: "ro"},
			{Type: "tmpfs", Destination: "/tmp"},
			{Type: "unknown", Source: "/src", Destination: "/dst"},
		},
	}

	cat := CatalogMounts(snap)

	if len(cat.Binds) != 2 { // bind + unknown
		t.Errorf("expected 2 binds, got %d", len(cat.Binds))
	}
	if len(cat.Volumes) != 1 {
		t.Errorf("expected 1 volume, got %d", len(cat.Volumes))
	}
	if len(cat.Tmpfs) != 1 {
		t.Errorf("expected 1 tmpfs, got %d", len(cat.Tmpfs))
	}

	if cat.Binds[0].Source != "/host/path" {
		t.Errorf("mismatch in bind source")
	}
	if cat.Volumes[0].Name != "myvol" {
		t.Errorf("mismatch in volume name")
	}
}

func TestMountArgs(t *testing.T) {
	cat := MountCatalog{
		Binds: []docker.MountInfo{
			{Source: "/host", Destination: "/container", Mode: "rw"},
		},
		Volumes: []docker.MountInfo{
			{Name: "vol1", Destination: "/data", Mode: "ro"},
		},
		Tmpfs: []docker.MountInfo{
			{Destination: "/tmp"},
		},
	}

	binds, tmpfs := MountArgs(cat)

	if len(binds) != 2 {
		t.Errorf("expected 2 bind args, got %d", len(binds))
	}
	if len(tmpfs) != 1 {
		t.Errorf("expected 1 tmpfs arg, got %d", len(tmpfs))
	}

	expectedBinds := []string{"/host:/container:rw", "vol1:/data:ro"}
	for i, b := range binds {
		if b != expectedBinds[i] {
			t.Errorf("expected bind %s, got %s", expectedBinds[i], b)
		}
	}

	if tmpfs[0] != "/tmp" {
		t.Errorf("expected tmpfs /tmp, got %s", tmpfs[0])
	}
}
