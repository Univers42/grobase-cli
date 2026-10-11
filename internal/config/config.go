// Package config holds the non-secret CLI configuration: named contexts, each
// pointing at one grobase deployment. Credentials never live here; see package secret.
package config

import (
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

// ErrNoContext is returned when no context is selected or the named one is missing.
var ErrNoContext = errors.New("no such context")

// Context describes one grobase deployment and how to trust its TLS certificate.
type Context struct {
	Name string `json:"name"`
	// URL is the Kong edge, e.g. https://host:8443.
	URL string `json:"url"`
	// CAFile is an optional PEM bundle trusted in addition to the system roots.
	CAFile string `json:"ca_file,omitempty"`
	// PinSHA256 is the lowercase hex SHA-256 of the pinned leaf certificate (DER).
	PinSHA256 string `json:"pin_sha256,omitempty"`
	// Insecure disables certificate verification entirely.
	Insecure bool `json:"insecure,omitempty"`
}

// File is the on-disk document.
type File struct {
	Current  string             `json:"current,omitempty"`
	Contexts map[string]Context `json:"contexts"`
}

func validName(s string) bool {
	if s == "" || len(s) > 63 || !isAlnum(s[0]) {
		return false
	}
	for i := 1; i < len(s); i++ {
		if c := s[i]; !isAlnum(c) && c != '_' && c != '.' && c != '-' {
			return false
		}
	}
	return true
}

func isAlnum(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func validPin(s string) bool {
	if len(s) != 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		if c := s[i]; (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Validate checks a context before it is saved.
func (c Context) Validate() error {
	if !validName(c.Name) {
		return fmt.Errorf("invalid context name %q: use letters, digits, '.', '_' or '-' (max 63)", c.Name)
	}
	u, err := url.Parse(c.URL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("invalid url %q: want http(s)://host[:port]", c.URL)
	}
	if u.User != nil {
		return errors.New("url must not embed credentials")
	}
	if c.Insecure && (c.CAFile != "" || c.PinSHA256 != "") {
		return errors.New("insecure cannot be combined with a CA file or a pin: pick one trust mode")
	}
	if c.PinSHA256 != "" && !validPin(c.PinSHA256) {
		return errors.New("pin_sha256 must be 64 lowercase hex characters")
	}
	return nil
}

// Add inserts or replaces a context; the first one added becomes current.
func (f *File) Add(c Context) error {
	if err := c.Validate(); err != nil {
		return err
	}
	c.URL = strings.TrimRight(c.URL, "/")
	if f.Contexts == nil {
		f.Contexts = map[string]Context{}
	}
	f.Contexts[c.Name] = c
	if f.Current == "" {
		f.Current = c.Name
	}
	return nil
}

// Use selects the current context.
func (f *File) Use(name string) error {
	if _, ok := f.Contexts[name]; !ok {
		return fmt.Errorf("%w: %q", ErrNoContext, name)
	}
	f.Current = name
	return nil
}

// Remove deletes a context, clearing Current when it pointed there.
func (f *File) Remove(name string) error {
	if _, ok := f.Contexts[name]; !ok {
		return fmt.Errorf("%w: %q", ErrNoContext, name)
	}
	delete(f.Contexts, name)
	if f.Current == name {
		f.Current = ""
	}
	return nil
}

// Get returns the named context, or the current one when name is empty.
func (f *File) Get(name string) (Context, error) {
	if name == "" {
		name = f.Current
	}
	if name == "" {
		return Context{}, fmt.Errorf("%w: run `grobase context add` first", ErrNoContext)
	}
	c, ok := f.Contexts[name]
	if !ok {
		return Context{}, fmt.Errorf("%w: %q", ErrNoContext, name)
	}
	return c, nil
}

// Names returns the context names, sorted.
func (f *File) Names() []string {
	names := make([]string, 0, len(f.Contexts))
	for n := range f.Contexts {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
