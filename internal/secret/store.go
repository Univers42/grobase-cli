// Package secret stores credentials. Two backends exist: the OS keyring and a
// passphrase-encrypted file. Neither ever holds a secret in plaintext on disk.
package secret

import (
	"errors"
	"sync"
)

// ErrNotFound is returned by Get when no secret is stored under the name.
var ErrNotFound = errors.New("secret not found")

// Store is a flat name -> secret map. Names are "<context>/<kind>".
type Store interface {
	Get(name string) (string, error)
	Set(name, value string) error
	Delete(name string) error
}

// Name builds the store key for one credential kind of one context.
func Name(contextName, kind string) string { return contextName + "/" + kind }

// Credential kinds.
const (
	KindAnon        = "anon"
	KindServiceRole = "service_role"
	KindAPIKey      = "api_key"
	KindSession     = "session"
)

// Memory is an in-process Store for tests.
type Memory struct {
	mu sync.Mutex
	m  map[string]string
}

// NewMemory returns an empty Memory store.
func NewMemory() *Memory { return &Memory{m: map[string]string{}} }

// Get implements Store.
func (s *Memory) Get(name string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.m[name]
	if !ok {
		return "", ErrNotFound
	}
	return v, nil
}

// Set implements Store.
func (s *Memory) Set(name, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[name] = value
	return nil
}

// Delete implements Store; deleting a missing name is not an error.
func (s *Memory) Delete(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, name)
	return nil
}
