package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fadhlidev/snapdock/pkg/types"
)

func TestLoadSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "snapdock.yaml")

	originalCfg := &types.Config{
		Jobs: []types.JobConfig{
			{
				Name:      "test-job",
				Container: "test-container",
				Schedule:  "@daily",
				Options: types.JobOptions{
					WithVolumes: true,
					Encrypt:     false,
				},
				Retention: types.RetentionConfig{
					KeepLast: 5,
				},
			},
		},
	}

	// Test Save
	err := Save(configPath, originalCfg)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatalf("config file was not created")
	}

	// Test Load
	loadedCfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Verify content
	if len(loadedCfg.Jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(loadedCfg.Jobs))
	}
	if loadedCfg.Jobs[0].Name != "test-job" {
		t.Errorf("expected job name 'test-job', got '%s'", loadedCfg.Jobs[0].Name)
	}
	if loadedCfg.Jobs[0].Options.WithVolumes != true {
		t.Errorf("expected WithVolumes to be true")
	}

	// Test Load non-existent file
	_, err = Load(filepath.Join(tmpDir, "missing.yaml"))
	if err == nil {
		t.Errorf("expected error when loading missing file, got nil")
	}

	// Test Load invalid YAML
	invalidPath := filepath.Join(tmpDir, "invalid.yaml")
	err = os.WriteFile(invalidPath, []byte("invalid: yaml: : content"), 0644)
	if err != nil {
		t.Fatalf("failed to create invalid yaml file: %v", err)
	}
	_, err = Load(invalidPath)
	if err == nil {
		t.Errorf("expected error when loading invalid YAML, got nil")
	}
}
