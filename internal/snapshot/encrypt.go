package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/fadhlidev/snapdock/internal/crypto"
	"github.com/fadhlidev/snapdock/pkg/types"
)

// EncryptEnv encrypts env.json using the provided passphrase and creates env.json.enc.
// Returns true if encryption was performed.
func EncryptEnv(tempDir, passphrase string) (bool, error) {
	envPath := filepath.Join(tempDir, "env.json")
	encPath := filepath.Join(tempDir, "env.json.enc")

	// Check if env.json exists
	if _, err := os.Stat(envPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("check env.json: %w", err)
	}

	// Read env.json
	envData, err := os.ReadFile(envPath)
	if err != nil {
		return false, fmt.Errorf("read env.json: %w", err)
	}

	// Parse to get []types.EnvVar for re-encoding after decrypt
	// (but we encrypt the JSON bytes directly)

	// Encrypt
	encrypted, err := crypto.Encrypt(envData, passphrase)
	if err != nil {
		return false, fmt.Errorf("encrypt env: %w", err)
	}

	// Write encrypted file
	if err := os.WriteFile(encPath, encrypted, 0o644); err != nil {
		return false, fmt.Errorf("write env.json.enc: %w", err)
	}

	// Remove unencrypted env.json
	if err := os.Remove(envPath); err != nil {
		return false, fmt.Errorf("remove env.json: %w", err)
	}

	return true, nil
}

// DecryptEnv decrypts env.json.enc using the provided passphrase and creates env.json.
// Returns true if decryption was performed.
func DecryptEnv(tempDir, passphrase string) (bool, error) {
	encPath := filepath.Join(tempDir, "env.json.enc")
	envPath := filepath.Join(tempDir, "env.json")

	// Check if env.json.enc exists
	if _, err := os.Stat(encPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("check env.json.enc: %w", err)
	}

	// Read encrypted file
	encrypted, err := os.ReadFile(encPath)
	if err != nil {
		return false, fmt.Errorf("read env.json.enc: %w", err)
	}

	// Decrypt
	decrypted, err := crypto.Decrypt(encrypted, passphrase)
	if err != nil {
		return false, fmt.Errorf("decrypt env: %w", err)
	}

	// Verify it's valid JSON
	var envVar []types.EnvVar
	if err := json.Unmarshal(decrypted, &envVar); err != nil {
		return false, fmt.Errorf("decrypted data is not valid JSON: %w", err)
	}

	// Write decrypted env.json
	if err := os.WriteFile(envPath, decrypted, 0o644); err != nil {
		return false, fmt.Errorf("write env.json: %w", err)
	}

	return true, nil
}

func EncryptEnvToFile(srcPath, passphrase, dstPath string) (bool, error) {
	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("check env file: %w", err)
	}

	envData, err := os.ReadFile(srcPath)
	if err != nil {
		return false, fmt.Errorf("read env file: %w", err)
	}

	encrypted, err := crypto.Encrypt(envData, passphrase)
	if err != nil {
		return false, fmt.Errorf("encrypt env: %w", err)
	}

	if err := os.WriteFile(dstPath, encrypted, 0o644); err != nil {
		return false, fmt.Errorf("write encrypted file: %w", err)
	}

	return true, nil
}