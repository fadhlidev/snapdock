package docker

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
)

func TestMapIdentity(t *testing.T) {
	raw := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:      "test-id",
			Name:    "/test-name",
			Image:   "test-image-id",
			Created: "2026-04-27T10:00:00Z",
		},
		Config: &container.Config{
			Image: "test-repo:tag",
		},
	}

	s := &ContainerSnapshot{}
	err := mapIdentity(s, raw)
	if err != nil {
		t.Fatalf("mapIdentity failed: %v", err)
	}

	if s.ID != "test-id" {
		t.Errorf("expected ID test-id, got %s", s.ID)
	}
	if s.Name != "test-name" {
		t.Errorf("expected Name test-name, got %s", s.Name)
	}
	if s.Image != "test-repo:tag" {
		t.Errorf("expected Image test-repo:tag, got %s", s.Image)
	}
	
	expectedTime, _ := time.Parse(time.RFC3339Nano, "2026-04-27T10:00:00Z")
	if !s.CreatedAt.Equal(expectedTime) {
		t.Errorf("expected CreatedAt %v, got %v", expectedTime, s.CreatedAt)
	}
}

func TestBytesToMB(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected int64
	}{
		{0, 0},
		{1024 * 1024, 1},
		{2048 * 1024 * 1024, 2048},
		{-1, 0},
	}

	for _, tt := range tests {
		result := bytesToMB(tt.bytes)
		if result != tt.expected {
			t.Errorf("bytesToMB(%d): expected %d, got %d", tt.bytes, tt.expected, result)
		}
	}
}

func TestMapResources(t *testing.T) {
	raw := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			HostConfig: &container.HostConfig{
				Resources: container.Resources{
					CPUShares: 512,
					CPUQuota:  50000,
					Memory:    100 * 1024 * 1024,
				},
			},
		},
	}

	s := &ContainerSnapshot{}
	mapResources(s, raw)

	if s.Resources.CPUShares != 512 {
		t.Errorf("expected CPUShares 512, got %d", s.Resources.CPUShares)
	}
	if s.Resources.MemoryMB != 100 {
		t.Errorf("expected MemoryMB 100, got %d", s.Resources.MemoryMB)
	}
}

func TestMapRuntimeConfig(t *testing.T) {
	raw := types.ContainerJSON{
		Config: &container.Config{
			Env:        []string{"FOO=BAR"},
			Labels:     map[string]string{"label1": "value1"},
			WorkingDir: "/work",
		},
	}

	s := &ContainerSnapshot{}
	mapRuntimeConfig(s, raw)

	if len(s.Env) != 1 || s.Env[0] != "FOO=BAR" {
		t.Errorf("mismatch in Env")
	}
	if s.Labels["label1"] != "value1" {
		t.Errorf("mismatch in Labels")
	}
	if s.WorkingDir != "/work" {
		t.Errorf("mismatch in WorkingDir")
	}
}

func TestMapNetworks(t *testing.T) {
	raw := types.ContainerJSON{
		NetworkSettings: &types.NetworkSettings{
			Networks: map[string]*network.EndpointSettings{
				"bridge": {
					NetworkID: "net123",
					IPAddress: "172.17.0.2",
					Aliases:   []string{"alias1"},
				},
			},
		},
	}

	s := &ContainerSnapshot{}
	mapNetworks(s, raw)

	if len(s.Networks) != 1 {
		t.Errorf("expected 1 network, got %d", len(s.Networks))
	}
	if s.Networks[0].Name != "bridge" || s.Networks[0].NetworkID != "net123" {
		t.Errorf("mismatch in Network settings")
	}
}

func TestMapPorts(t *testing.T) {
	raw := types.ContainerJSON{
		NetworkSettings: &types.NetworkSettings{
			NetworkSettingsBase: types.NetworkSettingsBase{
				Ports: nat.PortMap{
					"80/tcp": {
						{HostIP: "0.0.0.0", HostPort: "8080"},
					},
					"443/tcp": nil, // exposed but not published
				},
			},
		},
	}

	s := &ContainerSnapshot{}
	mapPorts(s, raw)

	if len(s.Ports) != 2 {
		t.Errorf("expected 2 port mappings, got %d", len(s.Ports))
	}
	
	found80 := false
	found443 := false
	for _, p := range s.Ports {
		if p.ContainerPort == "80/tcp" && p.HostPort == "8080" {
			found80 = true
		}
		if p.ContainerPort == "443/tcp" && p.HostPort == "" {
			found443 = true
		}
	}
	if !found80 || !found443 {
		t.Errorf("mismatch in Port mappings")
	}
}

func TestMapMounts(t *testing.T) {
	raw := types.ContainerJSON{
		Mounts: []types.MountPoint{
			{
				Type:        "bind",
				Source:      "/src",
				Destination: "/dst",
				Mode:        "rw",
			},
		},
	}

	s := &ContainerSnapshot{}
	mapMounts(s, raw)

	if len(s.Mounts) != 1 {
		t.Errorf("expected 1 mount, got %d", len(s.Mounts))
	}
	if s.Mounts[0].Source != "/src" || s.Mounts[0].Type != "bind" {
		t.Errorf("mismatch in Mount settings")
	}
}


