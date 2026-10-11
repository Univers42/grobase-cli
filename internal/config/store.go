package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Store reads and writes the config file.
type Store struct{ Path string }

// DefaultPath returns $XDG_CONFIG_HOME/grobase/config.json (or ~/.config/...).
func DefaultPath(getenv func(string) string, home func() (string, error)) (string, error) {
	if x := getenv("GROBASE_CONFIG"); x != "" {
		return x, nil
	}
	base := getenv("XDG_CONFIG_HOME")
	if base == "" {
		h, err := home()
		if err != nil {
			return "", fmt.Errorf("locate home directory: %w", err)
		}
		base = filepath.Join(h, ".config")
	}
	return filepath.Join(base, "grobase", "config.json"), nil
}

// Load reads the file; a missing file yields an empty configuration.
func (s Store) Load() (*File, error) {
	b, err := os.ReadFile(s.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return &File{Contexts: map[string]Context{}}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", s.Path, err)
	}
	var f File
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", s.Path, err)
	}
	if f.Contexts == nil {
		f.Contexts = map[string]Context{}
	}
	return &f, nil
}

// Save writes the file atomically with mode 0600 inside a 0700 directory.
func (s Store) Save(f *File) error {
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	dir := filepath.Dir(s.Path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return fmt.Errorf("create temp config: %w", err)
	}
	_, werr := tmp.Write(append(b, '\n'))
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		return errors.Join(fmt.Errorf("write config: %w", err), os.Remove(tmp.Name()))
	}
	if err := os.Rename(tmp.Name(), s.Path); err != nil {
		return errors.Join(fmt.Errorf("replace config: %w", err), os.Remove(tmp.Name()))
	}
	return nil
}
