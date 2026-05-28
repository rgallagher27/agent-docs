// Package config loads and validates the agent-docs server configuration.
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

// Config is the on-disk configuration for an agent-docs instance.
type Config struct {
	// Bind is the HTTP listen address. Defaults to defaultBind if empty.
	Bind string `toml:"bind"`

	// Projects is the list of registered projects served by this instance.
	Projects []Project `toml:"project"`
}

// Project describes one registered project's git remote and local bare clone.
type Project struct {
	// Slug is the URL-safe identifier used in /p/{slug}/... URLs.
	Slug string `toml:"slug"`

	// Remote is the git remote URL or local path to clone from.
	Remote string `toml:"remote"`

	// ClonePath is the absolute path on disk to the project's bare clone.
	ClonePath string `toml:"clone_path"`

	// TrunkRef is the project's default branch. Defaults to defaultTrunkRef if empty.
	TrunkRef string `toml:"trunk_ref"`
}

const (
	defaultBind     = "127.0.0.1:8080"
	defaultTrunkRef = "main"
)

// Load reads, parses, and validates a TOML config file at path.
// Defaults are applied during validation.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	var c Config
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}

	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("validate config %s: %w", path, err)
	}
	return &c, nil
}

// Validate applies defaults and reports any problems with c.
// It mutates c to fill in default values where fields are empty.
// Safe to call repeatedly; idempotent once defaults are populated.
func (c *Config) Validate() error {
	if c.Bind == "" {
		c.Bind = defaultBind
	}
	if len(c.Projects) == 0 {
		return errors.New("at least one project is required")
	}

	seen := make(map[string]struct{}, len(c.Projects))
	for i := range c.Projects {
		p := &c.Projects[i]
		if p.Slug == "" {
			return fmt.Errorf("project at index %d: slug is required", i)
		}
		if p.Remote == "" {
			return fmt.Errorf("project %q: remote is required", p.Slug)
		}
		if p.ClonePath == "" {
			return fmt.Errorf("project %q: clone_path is required", p.Slug)
		}
		if p.TrunkRef == "" {
			p.TrunkRef = defaultTrunkRef
		}
		if _, dup := seen[p.Slug]; dup {
			return fmt.Errorf("duplicate project slug %q", p.Slug)
		}
		seen[p.Slug] = struct{}{}
	}
	return nil
}
