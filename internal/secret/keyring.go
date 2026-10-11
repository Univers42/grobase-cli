package secret

import (
	"errors"
	"fmt"

	"github.com/zalando/go-keyring"
)

const keyringService = "grobase-cli"

// Keyring stores secrets in the operating system keyring.
type Keyring struct{}

// Get implements Store.
func (Keyring) Get(name string) (string, error) {
	v, err := keyring.Get(keyringService, name)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("keyring get: %w", err)
	}
	return v, nil
}

// Set implements Store.
func (Keyring) Set(name, value string) error {
	if err := keyring.Set(keyringService, name, value); err != nil {
		return fmt.Errorf("keyring set: %w", err)
	}
	return nil
}

// Delete implements Store.
func (Keyring) Delete(name string) error {
	err := keyring.Delete(keyringService, name)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("keyring delete: %w", err)
	}
	return nil
}

// Available probes whether the keyring answers (a headless box often has none).
// Ponytail: a probe write/delete of one throwaway entry; a keyring that works but
// prompts for unlock blocks here, in which case choose the file backend explicitly.
func (Keyring) Available() bool {
	const probe = "probe/availability"
	if err := keyring.Set(keyringService, probe, "x"); err != nil {
		return false
	}
	return keyring.Delete(keyringService, probe) == nil
}
