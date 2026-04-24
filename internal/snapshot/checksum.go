package snapshot

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// WriteChecksum computes SHA-256 of the file at sfxPath and writes the hex
// digest to sfxPath + ".sha256".
// Returns the hex digest string.
func WriteChecksum(sfxPath string) (string, error) {
	digest, err := checksumFile(sfxPath)
	if err != nil {
		return "", err
	}

	sumPath := sfxPath + ".sha256"
	// Format: "<hex>  <filename>" (same as sha256sum output)
	line := fmt.Sprintf("%s  %s\n", digest, sfxPath)

	if err := os.WriteFile(sumPath, []byte(line), 0o644); err != nil {
		return "", fmt.Errorf("failed to write checksum file: %w", err)
	}

	return digest, nil
}

// VerifyChecksum reads sfxPath + ".sha256" and compares it against
// the actual SHA-256 of sfxPath.
// Returns an error if the file is missing, unreadable, or the digest
// does not match (indicating a corrupted or tampered archive).
func VerifyChecksum(sfxPath string) error {
	sumPath := sfxPath + ".sha256"

	data, err := os.ReadFile(sumPath)
	if err != nil {
		return fmt.Errorf("checksum file not found at %s: %w", sumPath, err)
	}

	// Parse: first field before whitespace is the digest
	var stored string
	fmt.Sscanf(string(data), "%s", &stored)
	if stored == "" {
		return fmt.Errorf("checksum file is empty or malformed: %s", sumPath)
	}

	actual, err := checksumFile(sfxPath)
	if err != nil {
		return err
	}

	if actual != stored {
		return fmt.Errorf(
			"checksum mismatch — archive may be corrupted or tampered\n  stored: %s\n  actual: %s",
			stored, actual,
		)
	}

	return nil
}

// checksumFile returns the lowercase hex SHA-256 digest of a file.
func checksumFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("cannot open file for checksum: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}
