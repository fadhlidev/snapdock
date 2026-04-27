package crypto

import (
	"bytes"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	passphrase := "super-secret-passphrase"
	data := []byte("Hello, SnapDock!")

	// Test Encrypt
	encrypted, err := Encrypt(data, passphrase)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if len(encrypted) <= SaltSize+NonceSize {
		t.Errorf("encrypted data too short")
	}

	// Test Decrypt
	decrypted, err := Decrypt(encrypted, passphrase)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(data, decrypted) {
		t.Errorf("decrypted data does not match original: expected %s, got %s", string(data), string(decrypted))
	}

	// Test Decrypt with wrong passphrase
	_, err = Decrypt(encrypted, "wrong-passphrase")
	if err == nil {
		t.Errorf("expected error with wrong passphrase, got nil")
	}

	// Test Decrypt with corrupted data
	corrupted := make([]byte, len(encrypted))
	copy(corrupted, encrypted)
	corrupted[len(corrupted)-1] ^= 0xFF // Flip a bit
	_, err = Decrypt(corrupted, passphrase)
	if err == nil {
		t.Errorf("expected error with corrupted data, got nil")
	}

	// Test empty passphrase
	_, err = Encrypt(data, "")
	if err == nil {
		t.Errorf("expected error with empty passphrase in Encrypt, got nil")
	}

	_, err = Decrypt(encrypted, "")
	if err == nil {
		t.Errorf("expected error with empty passphrase in Decrypt, got nil")
	}

	// Test too short data
	_, err = Decrypt([]byte("too-short"), passphrase)
	if err == nil {
		t.Errorf("expected error with too short data in Decrypt, got nil")
	}
}
