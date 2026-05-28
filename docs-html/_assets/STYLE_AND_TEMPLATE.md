# agent-docs docs-html — Style & template guide

**You are maintaining a running set of HTML docs in `docs-html/` for human
review. This is the canonical home for project plans, feature docs,
architecture decisions, and status. Update it as work lands — see "When to
update" at the bottom.**

This file is also the spec for the HTML that the agent-docs *product* will
generate. The conventions here are what the MCP server's tools will emit and
what the web UI will render against. Keep them consistent.

## Hard rules

1. **Inline ALL CSS** in every file. No external stylesheets, no `<link rel>`,
   no JS frameworks. Files must open directly in a browser via `file://`.
2. **Every doc has a breadcrumb nav** at the very top: `agent-docs Docs ›
   Section › Doc title`. Ancestor links are relative paths. The root
   `index.html` shows just `agent-docs Docs` with no separator.
3. **Cross-doc links go to the `.html` equivalent.** Internal anchors stay as
   `#anchor`.
4. **Restructure freely.** Where prose is dense, pull tables out, group
   related content, add a contents nav, convert nested bullet trees into
   definition lists or tables. Keep the *content* accurate — change only the
   *form*.
5. **Status badges** — convert words like "DONE", "Partial", "Pending", "Not
   started", "Open" into `<span class="badge badge-good|warn|bad|info|muted">`.
   Convert `[x]` / `[ ]` checklists into `<ul class="checklist">` (CSS handles
   the box).
6. **Code blocks** — wrap in
   `<pre data-lang="go"><code>…</code></pre>` (or `sql`, `bash`, `json`,
   `yaml`, `html`). No syntax highlighter — the CSS just gives a frame and a
   language tag.
7. **No emoji** unless the source uses them meaningfully. Don't decorate.
8. **HTML-escape** `<`, `>`, `&` inside code and inline text. Especially
   generics like `chan<- T` and `<details>`.

## File scaffold

Every doc starts with this exact boilerplate. Replace `{{TITLE}}`,
`{{TAGS}}` (1–4 small tag chips for category/area/date/status),
`{{ROOT_REL}}` (relative path back to docs-html root, e.g. `../index.html`),
`{{SECTION_REL}}` (relative path to the section index, e.g. `index.html` for
same-dir, `../index.html` for nested folders), `{{SECTION_NAME}}` (e.g.
"Plans"), `{{LEAD}}` (one-paragraph summary), and `{{BODY}}`.

```html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>{{TITLE}} — agent-docs</title>
<style>
  :root {
    --bg: #0f1419;
    --panel: #1a2028;
    --panel-2: #232b36;
    --text: #e6e6e6;
    --muted: #9aa5b1;
    --accent: #f59e0b;
    --accent-2: #60a5fa;
    --accent-3: #a78bfa;
    --good: #10b981;
    --warn: #f59e0b;
    --bad: #ef4444;
    --border: #2d3744;
    --code-bg: #0a0e13;
  }
  * { box-sizing: border-box; }
  html { scroll-behavior: smooth; }
  body { margin: 0; font: 15px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: var(--bg); color: var(--text); }
  .wrap { max-width: 980px; margin: 0 auto; padding: 24px 32px 96px; }
  .breadcrumbs { font-size: 13px; color: var(--muted); margin-bottom: 24px; padding-bottom: 12px; border-bottom: 1px solid var(--border); }
  .breadcrumbs a { color: var(--accent-2); text-decoration: none; }
  .breadcrumbs a:hover { text-decoration: underline; }
  .breadcrumbs .sep { color: var(--border); margin: 0 8px; }
  header.doc-head { padding: 24px 0 28px; margin-bottom: 24px; border-bottom: 1px solid var(--border); }
  h1 { margin: 0 0 12px; font-size: 30px; letter-spacing: -0.02em; line-height: 1.2; }
  h2 { margin: 44px 0 14px; font-size: 22px; letter-spacing: -0.01em; padding-bottom: 6px; border-bottom: 1px solid var(--border); }
  h3 { margin: 28px 0 10px; font-size: 17px; color: var(--accent-2); }
  h4 { margin: 18px 0 6px; font-size: 14px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.06em; }
  p, ul, ol { margin: 0 0 12px; }
  ul, ol { padding-left: 22px; }
  li { margin-bottom: 4px; }
  a { color: var(--accent-2); }
  a:hover { color: #93c5fd; }
  hr { border: 0; border-top: 1px solid var(--border); margin: 32px 0; }
  blockquote { margin: 12px 0; padding: 10px 16px; border-left: 3px solid var(--accent-2); background: rgba(96,165,250,0.06); color: #cfd6df; font-size: 14px; border-radius: 3px; }
  code { font: 13px/1.4 "SF Mono", Menlo, Consolas, monospace; background: var(--code-bg); padding: 2px 6px; border-radius: 4px; color: #f0d090; }
  pre { background: var(--code-bg); border: 1px solid var(--border); border-radius: 8px; padding: 14px 16px; overflow-x: auto; margin: 10px 0 16px; position: relative; }
  pre code { background: transparent; padding: 0; color: var(--text); font-size: 13px; }
  pre[data-lang]::before { content: attr(data-lang); position: absolute; top: 6px; right: 10px; font-size: 10px; color: var(--muted); text-transform: uppercase; letter-spacing: 0.08em; }
  table { width: 100%; border-collapse: collapse; margin: 10px 0 18px; font-size: 14px; }
  th, td { text-align: left; padding: 10px 12px; border-bottom: 1px solid var(--border); vertical-align: top; }
  th { background: var(--panel-2); font-weight: 600; font-size: 12px; text-transform: uppercase; letter-spacing: 0.04em; color: var(--muted); }
  tr:hover td { background: var(--panel); }
  .meta { color: var(--muted); font-size: 13px; }
  .lead { color: #cfd6df; font-size: 16px; margin: 0; }
  .tag { display: inline-block; background: var(--panel-2); border: 1px solid var(--border); padding: 2px 8px; border-radius: 4px; font-size: 12px; color: var(--muted); margin-right: 6px; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: 600; letter-spacing: 0.04em; text-transform: uppercase; }
  .badge-good { background: rgba(16,185,129,0.15); color: var(--good); }
  .badge-warn { background: rgba(245,158,11,0.18); color: var(--warn); }
  .badge-bad  { background: rgba(239,68,68,0.15); color: var(--bad); }
  .badge-info { background: rgba(96,165,250,0.15); color: var(--accent-2); }
  .badge-purple { background: rgba(167,139,250,0.15); color: var(--accent-3); }
  .badge-muted { background: var(--panel-2); color: var(--muted); }
  .panel { background: var(--panel); border: 1px solid var(--border); border-radius: 10px; padding: 18px 22px; margin: 14px 0; }
  .panel-note { background: rgba(96,165,250,0.06); border-left: 3px solid var(--accent-2); padding: 12px 16px; border-radius: 4px; margin: 12px 0; color: #cfd6df; font-size: 14px; }
  .panel-warn { background: rgba(245,158,11,0.06); border-left: 3px solid var(--warn); padding: 12px 16px; border-radius: 4px; margin: 12px 0; color: #cfd6df; font-size: 14px; }
  .panel-bad  { background: rgba(239,68,68,0.06); border-left: 3px solid var(--bad); padding: 12px 16px; border-radius: 4px; margin: 12px 0; color: #cfd6df; font-size: 14px; }
  .grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
  .grid-3 { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; }
  @media (max-width: 720px) { .grid-2, .grid-3 { grid-template-columns: 1fr; } }
  .card { background: var(--panel); border: 1px solid var(--border); border-radius: 10px; padding: 18px 20px; }
  .card h3 { margin-top: 0; }
  .card a.card-link { display: block; color: inherit; text-decoration: none; }
  a.card-link:hover .card { border-color: var(--accent-2); background: var(--panel-2); }
  .kv { display: grid; grid-template-columns: 200px 1fr; gap: 8px 16px; margin: 8px 0; }
  .kv dt { color: var(--muted); }
  .kv dd { margin: 0; }
  .toc { background: var(--panel); border: 1px solid var(--border); border-radius: 10px; padding: 14px 20px; margin-bottom: 32px; }
  .toc h3 { margin: 0 0 8px; color: var(--muted); font-size: 12px; text-transform: uppercase; letter-spacing: 0.06em; }
  .toc ol, .toc ul { margin: 0; padding-left: 18px; }
  .toc a { color: var(--accent-2); text-decoration: none; }
  .toc a:hover { text-decoration: underline; }
  ul.checklist { list-style: none; padding-left: 0; }
  ul.checklist li { padding-left: 28px; position: relative; margin-bottom: 6px; }
  ul.checklist li::before { content: ""; position: absolute; left: 0; top: 4px; width: 16px; height: 16px; border: 1px solid var(--border); border-radius: 3px; background: var(--code-bg); }
  ul.checklist li.done::before { background: var(--good); border-color: var(--good); }
  ul.checklist li.done::after { content: "\2713"; position: absolute; left: 3px; top: 1px; font-size: 13px; color: var(--bg); font-weight: 700; }
  ul.checklist li.done { color: var(--muted); }
  details { background: var(--panel); border: 1px solid var(--border); border-radius: 8px; padding: 8px 14px; margin: 8px 0; }
  details summary { cursor: pointer; font-weight: 600; padding: 4px 0; color: var(--text); }
  details[open] summary { border-bottom: 1px solid var(--border); margin-bottom: 8px; padding-bottom: 8px; }
  svg { max-width: 100%; height: auto; }
  .crossout { color: var(--muted); text-decoration: line-through; }
  footer.doc-foot { margin-top: 64px; padding-top: 18px; border-top: 1px solid var(--border); color: var(--muted); font-size: 12px; display: flex; justify-content: space-between; }
  footer.doc-foot a { color: var(--accent-2); text-decoration: none; }
  .stat { display: inline-block; background: var(--panel-2); border: 1px solid var(--border); padding: 8px 14px; border-radius: 6px; margin-right: 10px; margin-bottom: 8px; font-size: 13px; }
  .stat strong { color: var(--accent); font-size: 16px; display: block; }
</style>
</head>
<body>
<div class="wrap">

<nav class="breadcrumbs">
  <a href="{{ROOT_REL}}">agent-docs Docs</a>
  <span class="sep">›</span>
  <a href="{{SECTION_REL}}">{{SECTION_NAME}}</a>
  <span class="sep">›</span>
  <span>{{TITLE}}</span>
</nav>

<header class="doc-head">
  <h1>{{TITLE}}</h1>
  <div class="meta" style="margin-bottom: 14px;">{{TAGS}}</div>
  <p class="lead">{{LEAD}}</p>
</header>

{{BODY}}

<footer class="doc-foot">
  <div>Native doc — no markdown source.</div>
  <div><a href="{{SECTION_REL}}">↑ {{SECTION_NAME}}</a> · <a href="{{ROOT_REL}}">agent-docs Docs</a></div>
</footer>

</div>
</body>
</html>
```

## Section index scaffold

Each section folder (`plans/`, `features/`, `decisions/`, `reviews/`) gets an
`index.html` that lists its docs. Use the same boilerplate, but the body is a
card grid:

```html
<div class="grid-2">
  <a class="card-link" href="some-doc.html"><div class="card">
    <h3>Doc title <span class="badge badge-good">Done</span></h3>
    <p style="color: var(--muted); margin: 0;">One-line summary.</p>
  </div></a>
</div>
```

If the section is small (≤ 4 docs), use a vertical list instead of grid:

```html
<ul style="list-style: none; padding: 0;">
  <li style="margin-bottom: 14px;">
    <a href="…" style="display: block; padding: 14px 18px; background: var(--panel); border: 1px solid var(--border); border-radius: 8px; text-decoration: none;">
      <strong style="color: var(--accent-2); display: block; font-size: 16px;">Doc title</strong>
      <span style="color: var(--muted); font-size: 14px;">One-line summary.</span>
    </a>
  </li>
</ul>
```

If the section is empty, render an empty-state panel — don't omit the section
index file.

## Sections

| Folder | Purpose | Filename convention |
|---|---|---|
| `plans/` | Implementation plans + their running trackers. One per planned chunk of work. | `YYYY-MM-DD-slug.html` |
| `features/` | Per-feature docs (after a feature ships): what it is, key files, edge cases. | `feature-name.html` |
| `decisions/` | ADRs — Context / Decision / Consequences. Numbered. | `NNN-short-name.html` |
| `reviews/` | Code review notes, audit findings, post-mortems. | `YYYY-MM-DD-slug.html` |

Top-level files (no folder):

- `index.html` — root landing / TOC.
- `vision.html` — product vision.
- `architecture.html` — design summary; evolves until the system stabilises,
  then becomes a snapshot pointing at the ADRs that froze each decision.
- `PROJECT_STATUS.html` — current state of the project (add when there is
  meaningful state to report; not required pre-code).

## Restructuring heuristics

- **Walls of "Recent changes" prose** → collapse into a timeline-style
  `<details>` per date, summary closed by default.
- **Implementation status checklists** → `<ul class="checklist">` with
  `class="done"` on completed items; add a status badge at the top showing
  X / Y complete.
- **Numbered "1. Backend / 2. Service / 3. Handler" layer lists** → table
  with columns Layer | Files | Status.
- **ADR doc with Context / Decision / Consequences** → keep the three-section
  structure but use colored panels (info for Context, default for Decision,
  panel-warn for Consequences/Tradeoffs).
- **Cross-doc references** — compute relative paths from the current file's
  location (e.g. from `plans/foo.html`, the architecture doc is
  `../architecture.html`).

## When to update docs-html

Update docs-html as part of the work, not as a follow-up chore.

1. **After ExitPlanMode is approved** — create
   `plans/YYYY-MM-DD-{slug}.html` mirroring the plan. Any `.claude/plans/*.md`
   scratch file is internal scaffolding; the HTML here is the canonical
   record.
2. **After each implementation step lands** — update the plan's tracker
   table (mark step done, add notes / commit SHAs), and update
   `PROJECT_STATUS.html` (once it exists) with the new state.
3. **After a feature ships end-to-end** — create or update
   `features/{name}.html` with what it is, key files, gotchas.
4. **After a non-obvious architectural decision** — create
   `decisions/NNN-{name}.html` (next free number).
5. **After a code review or post-mortem** — `reviews/YYYY-MM-DD-{slug}.html`.

The root `index.html` lists the entry points. The section `index.html` files
list everything in that section. Both must be kept in sync when adding new
docs.

## Tone

The audience is the project owner (and, eventually, contributors) reviewing
their own work. Information density beats decoration. Don't add headings the
source didn't have, don't pad with summary paragraphs. If a section is
short, let it be short.
