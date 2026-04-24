package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/pbkdf2"
)

const (
	SaltSize    = 16
	NonceSize   = 12
	KeySize    = 32 // 256 bits
	Iterations = 32768
)

// Encrypt encrypts data using AES-256-GCM with the given passphrase.
// Returns salt+nonce+ciphertext combined.
func Encrypt(data []byte, passphrase string) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("passphrase cannot be empty")
	}

	// Generate random salt
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}

	// Derive key using PBKDF2
	key := pbkdf2.Key([]byte(passphrase), salt, Iterations, KeySize, sha256.New)

	// Generate random nonce
	nonce := make([]byte, NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// Encrypt (nonce is prepended to ciphertext by Seal)
	ciphertext := gcm.Seal(nil, nonce, data, nil)

	// Combine: salt + nonce + ciphertext
	result := make([]byte, SaltSize+NonceSize+len(ciphertext))
	copy(result[:SaltSize], salt)
	copy(result[SaltSize:SaltSize+NonceSize], nonce)
	copy(result[SaltSize+NonceSize:], ciphertext)

	return result, nil
}

// Decrypt decrypts data using AES-256-GCM with the given passphrase.
// Input must be salt+nonce+ciphertext combined.
func Decrypt(encrypted []byte, passphrase string) ([]byte, error) {
	if len(passphrase) == 0 {
		return nil, errors.New("passphrase cannot be empty")
	}

	minLen := SaltSize + NonceSize + 1 // minimum: salt + nonce + 1 byte ciphertext
	if len(encrypted) < minLen {
		return nil, errors.New("encrypted data too short")
	}

	// Extract salt
	salt := encrypted[:SaltSize]

	// Extract nonce
	nonce := encrypted[SaltSize : SaltSize+NonceSize]

	// Extract ciphertext
	ciphertext := encrypted[SaltSize+NonceSize:]

	// Derive key using PBKDF2 with same salt
	key := pbkdf2.Key([]byte(passphrase), salt, Iterations, KeySize, sha256.New)

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: incorrect passphrase or corrupted data")
	}

	return plaintext, nil
}