package secret

import (
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/zalando/go-keyring"
)

const keyringService = "grobase-cli"

// The probe reaches go-keyring through these so a test can serialise its mock (a bare
// map) and line concurrent probes up between write and delete.
var (
	keyringSet    = keyring.Set
	keyringDelete = keyring.Delete
)

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

// Probe reports whether a keyring answers (a headless box often has none): nil once a
// throwaway item was written. Each call writes its own item, named after the pid plus
// 128 random bits: with one shared item, two grobase processes in a pipeline deleted
// each other's, and the loser fell back to the credential file (2026-10-15).
// Once the write succeeded the keyring is the answer, whatever the delete says: it
// holds the credentials, and a keyring that is really broken fails the next Get under
// its own name, where choosing the file would fail it as a missing passphrase.
// Caveat: a keyring that works but prompts for unlock blocks here (choose the file
// backend explicitly), and a delete that fails leaves its probe item behind.
func (Keyring) Probe() error {
	name := "probe/" + strconv.Itoa(os.Getpid()) + "-" + rand.Text()
	if err := keyringSet(keyringService, name, "x"); err != nil {
		return fmt.Errorf("keyring set: %w", err)
	}
	_ = keyringDelete(keyringService, name)
	return nil
}
