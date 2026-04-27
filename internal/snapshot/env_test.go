package snapshot

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/fadhlidev/snapdock/internal/docker"
	"github.com/fadhlidev/snapdock/pkg/types"
)

func TestExtractEnv(t *testing.T) {
	snap := &docker.ContainerSnapshot{
		Env: []string{
			"KEY1=VALUE1",
			"KEY2=VALUE2=WITH=EQUALS",
			"MALFORMED_NO_EQUALS",
			"EMPTY_VALUE=",
		},
	}

	vars := ExtractEnv(snap)

	if len(vars) != 4 {
		t.Errorf("expected 4 env vars, got %d", len(vars))
	}

	expected := []types.EnvVar{
		{Key: "KEY1", Value: "VALUE1"},
		{Key: "KEY2", Value: "VALUE2=WITH=EQUALS"},
		{Key: "MALFORMED_NO_EQUALS", Value: ""},
		{Key: "EMPTY_VALUE", Value: ""},
	}

	for i, v := range vars {
		if v.Key != expected[i].Key || v.Value != expected[i].Value {
			t.Errorf("mismatch at index %d: expected %+v, got %+v", i, expected[i], v)
		}
	}
}

func TestEnvToRaw(t *testing.T) {
	vars := []types.EnvVar{
		{Key: "KEY1", Value: "VALUE1"},
		{Key: "KEY2", Value: "VALUE2"},
	}

	raw := EnvToRaw(vars)

	if len(raw) != 2 {
		t.Errorf("expected 2 raw strings, got %d", len(raw))
	}

	if raw[0] != "KEY1=VALUE1" || raw[1] != "KEY2=VALUE2" {
		t.Errorf("mismatch: got %+v", raw)
	}
}

func TestEncryptDecryptEnv(t *testing.T) {
	tmpDir := t.TempDir()
	passphrase := "secret"

	envVars := []types.EnvVar{
		{Key: "DB_PASS", Value: "hunter2"},
	}
	envData, _ := json.Marshal(envVars)
	envPath := filepath.Join(tmpDir, "env.json")
	os.WriteFile(envPath, envData, 0644)

	// Test EncryptEnv
	performed, err := EncryptEnv(tmpDir, passphrase)
	if err != nil {
		t.Fatalf("EncryptEnv failed: %v", err)
	}
	if !performed {
		t.Errorf("expected EncryptEnv to perform encryption")
	}

	// Verify env.json is gone and env.json.enc exists
	if _, err := os.Stat(envPath); err == nil {
		t.Errorf("env.json should have been removed")
	}
	encPath := filepath.Join(tmpDir, "env.json.enc")
	if _, err := os.Stat(encPath); os.IsNotExist(err) {
		t.Errorf("env.json.enc should have been created")
	}

	// Test DecryptEnv
	performed, err = DecryptEnv(tmpDir, passphrase)
	if err != nil {
		t.Fatalf("DecryptEnv failed: %v", err)
	}
	if !performed {
		t.Errorf("expected DecryptEnv to perform decryption")
	}

	// Verify env.json exists and content matches
	decryptedData, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read decrypted env.json: %v", err)
	}

	var decryptedVars []types.EnvVar
	json.Unmarshal(decryptedData, &decryptedVars)

	if len(decryptedVars) != 1 || decryptedVars[0].Key != "DB_PASS" {
		t.Errorf("decrypted data mismatch: %+v", decryptedVars)
	}

	// Test DecryptEnv with wrong passphrase
	os.Remove(envPath)
	_, err = DecryptEnv(tmpDir, "wrong")
	if err == nil {
		t.Errorf("expected error with wrong passphrase, got nil")
	}
}
