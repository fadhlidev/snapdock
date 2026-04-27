package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fadhlidev/snapdock/internal/snapshot"
)

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1024 * 1024, "1.0 MB"},
		{1024 * 1024 * 1024, "1.0 GB"},
		{1536, "1.5 KB"},
	}

	for _, tt := range tests {
		result := formatSize(tt.bytes)
		if result != tt.expected {
			t.Errorf("formatSize(%d): expected %s, got %s", tt.bytes, tt.expected, result)
		}
	}
}

func TestBuildContainerConfig(t *testing.T) {
	tmpDir := t.TempDir()
	
	// Create dummy env.json
	envPath := filepath.Join(tmpDir, "env.json")
	envContent := `[{"key": "FOO", "value": "BAR"}]`
	os.WriteFile(envPath, []byte(envContent), 0644)

	extracted := &snapshot.ExtractedSnapshot{
		TempDir: tmpDir,
		Container: &snapshot.ContainerJSONExport{
			Name:  "orig-name",
			Image: "orig-image",
			Env:   []string{"EXISTING=VALUE"},
		},
	}

	s := &MCPServer{}
	cfg := s.buildContainerConfig(extracted, "new-name")

	if cfg.Name != "new-name" {
		t.Errorf("expected name 'new-name', got '%s'", cfg.Name)
	}

	if cfg.Image != "orig-image" {
		t.Errorf("expected image 'orig-image', got '%s'", cfg.Image)
	}

	// Verify env was loaded from env.json
	found := false
	for _, e := range cfg.Env {
		if e == "FOO=BAR" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected FOO=BAR in env, got %v", cfg.Env)
	}
}
