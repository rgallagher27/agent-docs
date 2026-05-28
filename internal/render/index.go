// Package render produces auto-generated chrome for the agent-docs web
// UI — section indexes, breadcrumbs, "merged" banners. Doc content is
// never re-rendered (per ADR-002); only the navigation around it.
package render

import (
	"bytes"
	"html/template"
)

// IndexPage is the structured input to RenderIndex. Callers (the HTTP
// server) compose it from the request and the ref's directory listing.
type IndexPage struct {
	// Title goes into <title>. Typically "{project} / {dir} @ {ref}".
	Title string
	// H1 is the page heading. Typically the directory name or the project slug.
	H1 string
	// Breadcrumb is the navigation trail. Last item should have an empty Href.
	Breadcrumb []Crumb
	// RefLabel is shown as a tag chip. "main", "feat-x", etc.
	RefLabel string
	// Entries are the items to list in the card grid.
	Entries []Entry
	// Banner, if set, is raw HTML inserted at the very top of <body>
	// (e.g. a "merged" lifecycle banner). Trusted server-composed markup.
	Banner template.HTML
}

// Crumb is one breadcrumb segment. Empty Href marks the current item.
type Crumb struct {
	Label string
	Href  string
}

// Entry is one item in the auto-generated listing.
type Entry struct {
	Label string
	Href  string
	IsDir bool
}

// RenderIndex returns the rendered HTML for a section-index page.
// The output is self-contained — inline CSS, no external assets, opens
// in a browser directly.
func RenderIndex(p IndexPage) ([]byte, error) {
	var buf bytes.Buffer
	if err := indexTmpl.Execute(&buf, p); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MergedBanner returns the lifecycle banner shown on a merged branch's
// URLs (ADR-003). trunkRef is the trunk branch name, trunkHref the clean
// trunk URL for the current doc or directory, shortSHA the branch tip's
// abbreviated SHA. The result is self-contained inline-styled HTML with
// no JS, safe to splice verbatim into an LLM-authored doc just after its
// <body> tag. Inputs are escaped via the template.
func MergedBanner(trunkRef, trunkHref, shortSHA string) template.HTML {
	var buf bytes.Buffer
	// Static template, string inputs — Execute cannot fail here.
	_ = bannerTmpl.Execute(&buf, struct{ TrunkRef, TrunkHref, SHA string }{
		TrunkRef:  trunkRef,
		TrunkHref: trunkHref,
		SHA:       shortSHA,
	})
	return template.HTML(buf.String())
}

// bannerTmpl renders the merged-branch banner. Styles are inlined (the
// banner is injected into arbitrary docs that don't share our CSS) and
// mirror the palette in indexCSS.
var bannerTmpl = template.Must(template.New("banner").Parse(
	`<div style="position:sticky;top:0;z-index:9999;background:#232b36;` +
		`border-bottom:1px solid #2d3744;color:#cfd6df;padding:10px 16px;text-align:center;` +
		`font:14px/1.5 -apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;">` +
		`<span style="display:inline-block;background:rgba(16,185,129,0.15);color:#10b981;` +
		`font-size:11px;font-weight:600;letter-spacing:0.04em;text-transform:uppercase;` +
		`padding:2px 8px;border-radius:10px;margin-right:8px;">Merged</span>` +
		`This branch (tip <code style="background:#0a0e13;padding:2px 6px;border-radius:4px;` +
		`color:#f0d090;font-family:'SF Mono',Menlo,Consolas,monospace;">{{.SHA}}</code>) ` +
		`was merged into <code style="background:#0a0e13;padding:2px 6px;border-radius:4px;` +
		`color:#f0d090;font-family:'SF Mono',Menlo,Consolas,monospace;">{{.TrunkRef}}</code> ` +
		`— viewing an archived preview. ` +
		`<a href="{{.TrunkHref}}" style="color:#60a5fa;">View current version on {{.TrunkRef}} &rarr;</a>` +
		`</div>`))

// indexCSS is the shared inline CSS for auto-generated chrome. Mirrors
// the style template's palette and typography so generated pages don't
// visually clash with LLM-authored docs sitting next to them.
const indexCSS = `
:root {
  --bg: #0f1419; --panel: #1a2028; --panel-2: #232b36; --text: #e6e6e6;
  --muted: #9aa5b1; --accent: #f59e0b; --accent-2: #60a5fa;
  --good: #10b981; --warn: #f59e0b; --border: #2d3744; --code-bg: #0a0e13;
}
* { box-sizing: border-box; }
body { margin: 0; font: 15px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: var(--bg); color: var(--text); }
.wrap { max-width: 980px; margin: 0 auto; padding: 24px 32px 96px; }
.breadcrumbs { font-size: 13px; color: var(--muted); margin-bottom: 24px; padding-bottom: 12px; border-bottom: 1px solid var(--border); }
.breadcrumbs a { color: var(--accent-2); text-decoration: none; }
.breadcrumbs a:hover { text-decoration: underline; }
.breadcrumbs .sep { color: var(--border); margin: 0 8px; }
header.doc-head { padding: 24px 0 28px; margin-bottom: 24px; border-bottom: 1px solid var(--border); }
h1 { margin: 0 0 12px; font-size: 30px; letter-spacing: -0.02em; }
h3 { margin: 0 0 6px; font-size: 16px; color: var(--accent-2); }
.meta { color: var(--muted); font-size: 13px; }
.lead { color: #cfd6df; font-size: 16px; margin: 0; }
.tag { display: inline-block; background: var(--panel-2); border: 1px solid var(--border); padding: 2px 8px; border-radius: 4px; font-size: 12px; color: var(--muted); margin-right: 6px; }
.badge { display: inline-block; padding: 2px 8px; border-radius: 10px; font-size: 11px; font-weight: 600; letter-spacing: 0.04em; text-transform: uppercase; margin-left: 6px; }
.badge-muted { background: var(--panel-2); color: var(--muted); }
.grid-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; }
@media (max-width: 720px) { .grid-2 { grid-template-columns: 1fr; } }
a.card-link { display: block; color: inherit; text-decoration: none; }
.card { background: var(--panel); border: 1px solid var(--border); border-radius: 10px; padding: 16px 18px; }
a.card-link:hover .card { border-color: var(--accent-2); background: var(--panel-2); }
code { font: 13px/1.4 "SF Mono", Menlo, Consolas, monospace; background: var(--code-bg); padding: 2px 6px; border-radius: 4px; color: #f0d090; }
.empty { text-align: center; color: var(--muted); padding: 40px 20px; background: var(--panel); border: 1px solid var(--border); border-radius: 10px; }
footer.doc-foot { margin-top: 64px; padding-top: 18px; border-top: 1px solid var(--border); color: var(--muted); font-size: 12px; }
`

// indexTmpl renders a section-index page. The structure mirrors the
// section-index examples in docs-html/_assets/STYLE_AND_TEMPLATE.md.
var indexTmpl = template.Must(template.New("index").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>{{ .Title }}</title>
<style>` + indexCSS + `</style>
</head>
<body>
{{ .Banner }}
<div class="wrap">

<nav class="breadcrumbs">
  {{- range $i, $c := .Breadcrumb }}
  {{- if $i }}<span class="sep">›</span>{{ end }}
  {{- if $c.Href }}<a href="{{ $c.Href }}">{{ $c.Label }}</a>{{ else }}<span>{{ $c.Label }}</span>{{ end }}
  {{- end }}
</nav>

<header class="doc-head">
  <h1>{{ .H1 }}</h1>
  <div class="meta" style="margin-bottom: 14px;">
    <span class="tag">auto-generated</span>
    <span class="tag">@{{ .RefLabel }}</span>
    <span class="tag">{{ len .Entries }} entries</span>
  </div>
  <p class="lead">Auto-generated directory listing — no <code>index.html</code> committed at this path.</p>
</header>

{{ if .Entries -}}
<div class="grid-2">
{{- range .Entries }}
  <a class="card-link" href="{{ .Href }}"><div class="card">
    <h3>{{ .Label }}{{ if .IsDir }}<span class="badge badge-muted">dir</span>{{ end }}</h3>
  </div></a>
{{- end }}
</div>
{{- else -}}
<div class="empty">
  <p style="font-size: 16px; margin: 0;">Empty directory.</p>
</div>
{{- end }}

<footer class="doc-foot">
  Auto-generated by agent-docs · ref <code>{{ .RefLabel }}</code>
</footer>

</div>
</body>
</html>
`))
