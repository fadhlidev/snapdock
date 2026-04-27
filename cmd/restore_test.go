package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fadhlidev/snapdock/internal/snapshot"
)

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

	cfg := buildContainerConfig(extracted, "new-name")

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
