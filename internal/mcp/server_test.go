package mcp

import (
	"os"
	"path/filepath"
	"strings"
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

func TestDiffEnvVars(t *testing.T) {
	s := &MCPServer{}
	var builder strings.Builder
	
	env1 := map[string]string{"K1": "V1", "K2": "V2"}
	env2 := map[string]string{"K1": "V1", "K2": "V2-new", "K3": "V3"}

	s.diffEnvVars(&builder, env1, env2)
	result := builder.String()

	if !strings.Contains(result, "\n- K2=V2") {
		t.Errorf("missing - K2=V2 in diff")
	}
	if !strings.Contains(result, "\n+ K2=V2-new") {
		t.Errorf("missing + K2=V2-new in diff")
	}
	if !strings.Contains(result, "\n+ K3=V3") {
		t.Errorf("missing + K3=V3 in diff")
	}
}

func TestParseEnvMap(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "env.json")
	envContent := `[{"key": "FOO", "value": "BAR"}]`
	os.WriteFile(envPath, []byte(envContent), 0644)

	envMap := parseEnvMap(tmpDir)
	if envMap["FOO"] != "BAR" {
		t.Errorf("expected FOO=BAR, got %v", envMap)
	}
}

func TestFileExists(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	os.WriteFile(tmpFile, []byte("hello"), 0644)

	if !fileExists(tmpFile) {
		t.Errorf("expected file to exist")
	}
	if fileExists(tmpFile + ".nonexistent") {
		t.Errorf("expected file to NOT exist")
	}
}

