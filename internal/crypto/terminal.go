package crypto

import (
	"errors"
	"fmt"

	"golang.org/x/term"
)

// PromptPassphrase prompts the user for a passphrase with confirmation.
// Returns the passphrase if both entries match, otherwise returns an error.
func PromptPassphrase() (string, error) {
	fmt.Print("Enter passphrase: ")
	pass1, err := term.ReadPassword(0)
	if err != nil {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	fmt.Println()

	fmt.Print("Confirm passphrase: ")
	pass2, err := term.ReadPassword(0)
	if err != nil {
		return "", fmt.Errorf("read confirmation: %w", err)
	}
	fmt.Println()

	if string(pass1) != string(pass2) {
		return "", errors.New("passphrases do not match")
	}

	if len(pass1) == 0 {
		return "", errors.New("passphrase cannot be empty")
	}

	return string(pass1), nil
}

// PromptPassphraseSingle prompts the user for a passphrase once (no confirmation).
// Used during restore when decryption is needed.
func PromptPassphraseSingle() (string, error) {
	fmt.Print("Enter passphrase: ")
	pass, err := term.ReadPassword(0)
	if err != nil {
		return "", fmt.Errorf("read passphrase: %w", err)
	}
	fmt.Println()

	if len(pass) == 0 {
		return "", errors.New("passphrase cannot be empty")
	}

	return string(pass), nil
}