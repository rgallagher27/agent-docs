// Command agent-docs is the entry point for the agent-docs server and CLI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rgallagher27/agent-docs/internal/config"
	"github.com/rgallagher27/agent-docs/internal/gitstore"
	"github.com/rgallagher27/agent-docs/internal/mcp"
	"github.com/rgallagher27/agent-docs/internal/server"
)

// version is the build-time version string. Set via -ldflags "-X main.version=…" at release time.
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "agent-docs:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		printUsage(stdout)
		return nil
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "serve":
		return cmdServe(ctx, rest, stdout, stderr)
	case "fetch":
		return cmdFetch(ctx, rest, stdout, stderr)
	case "mcp":
		return cmdMCP(ctx, rest, stdout, stderr)
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
  serve     Start the HTTP server.
  fetch     Fetch latest refs for one or all configured projects.
  mcp       Run the MCP server over stdio (for LLM harness integration).
  version   Print the version and exit.
  help      Show this message.

Run "agent-docs <command> -h" for command-specific flags.`)
}

func cmdMCP(ctx context.Context, args []string, _, stderr io.Writer) error {
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("config", defaultConfigPath(), "Path to the config file")
	authorName := fs.String("author-name", "agent-docs", "Name recorded on commits made through MCP write_doc")
	authorEmail := fs.String("author-email", "agent-docs@local.invalid", "Email recorded on commits made through MCP write_doc")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		return err
	}

	srv, err := mcp.New(cfg, gitstore.Author{Name: *authorName, Email: *authorEmail})
	if err != nil {
		return fmt.Errorf("init mcp: %w", err)
	}

	// MCP stdio: stdin/stdout are reserved for the protocol; logs to stderr.
	fmt.Fprintf(stderr, "agent-docs mcp: ready (%d project(s))\n", len(cfg.Projects))
	return srv.Serve(ctx, os.Stdin, os.Stdout, stderr)
}

func cmdServe(ctx context.Context, args []string, stdout, stderr io.Writer) error {
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

	srv, err := server.New(cfg)
	if err != nil {
		return fmt.Errorf("init server: %w", err)
	}

	httpSrv := &http.Server{
		Addr:              bind,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		fmt.Fprintf(stdout, "agent-docs listening on %s (%d project(s))\n", bind, len(cfg.Projects))
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		fmt.Fprintln(stdout, "shutting down")
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("listen: %w", err)
		}
		return nil
	}

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpSrv.Shutdown(shutCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}

func cmdFetch(ctx context.Context, args []string, stdout, stderr io.Writer) error {
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
