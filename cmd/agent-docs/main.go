// Command agent-docs is the entry point for the agent-docs server and CLI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/rgallagher/agent-docs/internal/config"
	"github.com/rgallagher/agent-docs/internal/gitstore"
)

// version is the build-time version string. Set via -ldflags "-X main.version=…" at release time.
var version = "dev"

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "agent-docs:", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "serve":
		return cmdServe(rest, stdout, stderr)
	case "fetch":
		return cmdFetch(rest, stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version)
		return nil
	case "help", "--help", "-h":
		printUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q (try `agent-docs help`)", cmd)
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `agent-docs — self-hostable LLM-native docs hub.

Usage:
  agent-docs <command> [flags]

Commands:
  serve     Start the HTTP and MCP servers.
  fetch     Fetch latest refs for one or all configured projects.
  version   Print the version and exit.
  help      Show this message.

Run "agent-docs <command> -h" for command-specific flags.`)
}

// errNotImplemented marks subcommands whose plumbing is scaffolded but whose
// behavior lands in a later tracker step. Returning a concrete error keeps
// the binary honest about its state and makes the exit code non-zero.
var errNotImplemented = errors.New("not implemented yet — see docs-html/plans/2026-05-28-walking-skeleton-mvp.html")

func cmdServe(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(stderr)
	addr := fs.String("addr", "", "HTTP listen address (overrides config 'bind'; loopback only unless --unsafe-no-auth is set)")
	cfgPath := fs.String("config", defaultConfigPath(), "Path to the config file")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}
	bind := cfg.Bind
	if *addr != "" {
		bind = *addr
	}

	fmt.Fprintf(stdout, "would serve on %s; %d project(s) loaded from %s\n", bind, len(cfg.Projects), *cfgPath)
	return fmt.Errorf("serve: %w (tracker steps 4–6)", errNotImplemented)
}

func cmdFetch(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("fetch", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", defaultConfigPath(), "Path to the config file")
	project := fs.String("project", "", "Project slug to fetch (empty fetches all configured projects)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}

	targets, err := selectProjects(cfg, *project)
	if err != nil {
		return err
	}

	ctx := context.Background()
	for _, p := range targets {
		store, err := gitstore.Open(p.Remote, p.ClonePath)
		if err != nil {
			return fmt.Errorf("fetch %s: %w", p.Slug, err)
		}
		if err := store.Fetch(ctx); err != nil {
			return fmt.Errorf("fetch %s: %w", p.Slug, err)
		}
		fmt.Fprintf(stdout, "fetched %s\n", p.Slug)
	}
	return nil
}

// selectProjects returns the subset of cfg.Projects matching slug, or
// all projects if slug is empty. Returns an error if slug is non-empty
// and no matching project is configured.
func selectProjects(cfg *config.Config, slug string) ([]config.Project, error) {
	if slug == "" {
		return cfg.Projects, nil
	}
	for _, p := range cfg.Projects {
		if p.Slug == slug {
			return []config.Project{p}, nil
		}
	}
	return nil, fmt.Errorf("project %q not found in config", slug)
}

func defaultConfigPath() string {
	if dir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(dir, ".agent-docs", "config.toml")
	}
	return "agent-docs.toml"
}
