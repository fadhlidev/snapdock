package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChecksum(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.sfx")
	content := []byte("some snapshot content")

	if err := os.WriteFile(testFile, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Test WriteChecksum
	digest, err := WriteChecksum(testFile)
	if err != nil {
		t.Fatalf("WriteChecksum failed: %v", err)
	}

	if digest == "" {
		t.Errorf("expected non-empty digest")
	}

	// Verify checksum file exists
	sumPath := testFile + ".sha256"
	if _, err := os.Stat(sumPath); os.IsNotExist(err) {
		t.Fatalf("checksum file was not created")
	}

	// Test VerifyChecksum
	err = VerifyChecksum(testFile)
	if err != nil {
		t.Fatalf("VerifyChecksum failed: %v", err)
	}

	// Test VerifyChecksum with mismatch
	if err := os.WriteFile(testFile, []byte("different content"), 0644); err != nil {
		t.Fatalf("failed to modify test file: %v", err)
	}
	err = VerifyChecksum(testFile)
	if err == nil {
		t.Errorf("expected error when verifying modified file, got nil")
	}

	// Test VerifyChecksum with missing checksum file
	os.Remove(sumPath)
	err = VerifyChecksum(testFile)
	if err == nil {
		t.Errorf("expected error when checksum file is missing, got nil")
	}

	// Test VerifyChecksum with missing file
	err = VerifyChecksum("non-existent-file")
	if err == nil {
		t.Errorf("expected error when file is missing, got nil")
	}
}
