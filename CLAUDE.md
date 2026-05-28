# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

agent-docs is a self-hostable, LLM-native documentation hub. It exposes an MCP server that coding agents (Claude Code, OpenAI Codex, OpenCode, Cursor, etc.) call to create and maintain a canonical set of project docs — implementation plans, feature docs, ADRs, code reviews — rendered as a dense, navigable HTML site.

The project is in early ideation: no source code, no build, no tests yet. The canonical state of the design lives in `docs-html/`.

Start here:

- `docs-html/index.html` — landing / state of play
- `docs-html/vision.html` — what we're building and why
- `docs-html/architecture.html` — current architectural thinking and open tradeoffs
- `docs-html/_assets/STYLE_AND_TEMPLATE.md` — doc style guide (also ships with the product)

## Documentation policy

This project dogfoods its own approach. Design docs, plans, decisions, and reviews are HTML files in `docs-html/`, not Markdown. Follow `docs-html/_assets/STYLE_AND_TEMPLATE.md` exactly — inline CSS, breadcrumbs at the top, section convention, no JS.

When work lands:

| Trigger | Where to write |
|---|---|
| Plan approved | `docs-html/plans/YYYY-MM-DD-{slug}.html` |
| Feature ships | `docs-html/features/{name}.html` |
| Architectural decision | `docs-html/decisions/NNN-{short-name}.html` (next free number) |
| Code review / post-mortem | `docs-html/reviews/YYYY-MM-DD-{slug}.html` |

Each section's `index.html` must be kept in sync with its contents. The root `docs-html/index.html` and `architecture.html` evolve as design lands — update in place, don't append.

## Status

Pre-code. The next concrete decisions are captured in `docs-html/architecture.html` under "Open questions". Once any of those are resolved, record the resolution as a new ADR under `docs-html/decisions/`.
