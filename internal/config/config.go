// Package config stores provider configuration and imported sessions locally.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/icco/where/internal/provider"
)

// Config describes the enabled providers. No Apple password is stored.
type Config struct {
	Google *provider.GoogleSession `json:"google,omitempty"`
	Apple  bool                    `json:"apple"`
}

// DefaultPath follows XDG on all platforms, including macOS.
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	if !filepath.IsAbs(dir) {
		return "", errors.New("XDG_CONFIG_HOME must be an absolute path")
	}
	return filepath.Join(dir, "where", "config.json"), nil
}

// Load returns an empty configuration when no session has been configured.
func Load(path string) (Config, error) {
	var c Config
	data, err := os.ReadFile(path) // #nosec G304 -- explicit user-selected local configuration.
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, fmt.Errorf("read configuration: %w", err)
	}
	if json.Unmarshal(data, &c) != nil {
		return c, errors.New("invalid configuration JSON")
	}
	return c, nil
}

// Save atomically replaces configuration using an owner-only temporary file.
func Save(path string, c Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".where-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.Write(append(data, '\n')); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
