// Command agent-docs is the entry point for the agent-docs server and CLI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	addr := fs.String("addr", "127.0.0.1:8080", "HTTP listen address (loopback only unless --unsafe-no-auth is set)")
	cfgPath := fs.String("config", defaultConfigPath(), "Path to the config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "would serve on %s with config %s\n", *addr, *cfgPath)
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
	target := *project
	if target == "" {
		target = "<all projects>"
	}
	fmt.Fprintf(stdout, "would fetch %s with config %s\n", target, *cfgPath)
	return fmt.Errorf("fetch: %w (tracker step 3)", errNotImplemented)
}

func defaultConfigPath() string {
	if dir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(dir, ".agent-docs", "config.toml")
	}
	return "agent-docs.toml"
}
