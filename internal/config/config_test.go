package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		fixture     string
		wantErr     bool
		errContains string
		check       func(t *testing.T, c *Config)
	}{
		{
			name:    "valid single project",
			fixture: "valid-single.toml",
			check: func(t *testing.T, c *Config) {
				if c.Bind != "127.0.0.1:9090" {
					t.Errorf("Bind = %q, want 127.0.0.1:9090", c.Bind)
				}
				if got := len(c.Projects); got != 1 {
					t.Fatalf("got %d projects, want 1", got)
				}
				p := c.Projects[0]
				if p.Slug != "agent-docs" {
					t.Errorf("Slug = %q, want agent-docs", p.Slug)
				}
				if p.TrunkRef != "main" {
					t.Errorf("TrunkRef = %q, want main", p.TrunkRef)
				}
			},
		},
		{
			name:    "valid multi-project applies defaults",
			fixture: "valid-multi.toml",
			check: func(t *testing.T, c *Config) {
				if c.Bind != "127.0.0.1:8080" {
					t.Errorf("Bind default not applied: got %q", c.Bind)
				}
				if got := len(c.Projects); got != 2 {
					t.Fatalf("got %d projects, want 2", got)
				}
				if c.Projects[0].TrunkRef != "main" {
					t.Errorf("foo TrunkRef = %q, want main (default)", c.Projects[0].TrunkRef)
				}
				if c.Projects[1].TrunkRef != "trunk" {
					t.Errorf("bar TrunkRef = %q, want trunk (explicit)", c.Projects[1].TrunkRef)
				}
			},
		},
		{
			name:        "invalid TOML syntax",
			fixture:     "invalid-syntax.toml",
			wantErr:     true,
			errContains: "parse config",
		},
		{
			name:        "missing slug",
			fixture:     "missing-slug.toml",
			wantErr:     true,
			errContains: "slug is required",
		},
		{
			name:        "missing remote",
			fixture:     "missing-remote.toml",
			wantErr:     true,
			errContains: "remote is required",
		},
		{
			name:        "duplicate slug",
			fixture:     "duplicate-slug.toml",
			wantErr:     true,
			errContains: "duplicate project slug",
		},
		{
			name:        "no projects",
			fixture:     "no-projects.toml",
			wantErr:     true,
			errContains: "at least one project",
		},
		{
			name:        "missing file",
			fixture:     "no-such-file.toml",
			wantErr:     true,
			errContains: "read config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Load(filepath.Join("testdata", tt.fixture))

			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.errContains != "" && (err == nil || !strings.Contains(err.Error(), tt.errContains)) {
				t.Errorf("err = %v, want containing %q", err, tt.errContains)
			}
			if tt.check != nil && err == nil {
				tt.check(t, c)
			}
		})
	}
}

func TestValidate_AppliesDefaults(t *testing.T) {
	c := &Config{
		Projects: []Project{{Slug: "x", Remote: "r", ClonePath: "/c"}},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Bind != defaultBind {
		t.Errorf("Bind = %q, want default %q", c.Bind, defaultBind)
	}
	if c.Projects[0].TrunkRef != defaultTrunkRef {
		t.Errorf("TrunkRef = %q, want default %q", c.Projects[0].TrunkRef, defaultTrunkRef)
	}
}

func TestValidate_Idempotent(t *testing.T) {
	c := &Config{
		Projects: []Project{{Slug: "x", Remote: "r", ClonePath: "/c"}},
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("first validate: %v", err)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("second validate: %v", err)
	}
}
