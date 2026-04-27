package docker

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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
