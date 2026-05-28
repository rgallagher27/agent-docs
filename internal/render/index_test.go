package render

import (
	"strings"
	"testing"
)

func TestRenderIndex_BasicShape(t *testing.T) {
	page := IndexPage{
		Title:    "demo / plans @ main",
		H1:       "plans/",
		RefLabel: "main",
		Breadcrumb: []Crumb{
			{Label: "demo", Href: "/p/demo/"},
			{Label: "main", Href: "/p/demo/main/"},
			{Label: "plans"},
		},
		Entries: []Entry{
			{Label: "2026-05-28-mvp.html", Href: "2026-05-28-mvp.html"},
			{Label: "subdir/", Href: "subdir/", IsDir: true},
		},
	}

	out, err := RenderIndex(page)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	html := string(out)

	for _, want := range []string{
		"<!DOCTYPE html>",
		"<title>demo / plans @ main</title>",
		`<h1>plans/</h1>`,
		`<a href="/p/demo/">demo</a>`,
		`<a href="/p/demo/main/">main</a>`,
		`<span>plans</span>`,
		`href="2026-05-28-mvp.html"`,
		`href="subdir/"`,
		`<span class="badge badge-muted">dir</span>`,
		"@main",
		"2 entries",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("html missing %q\n--- html ---\n%s", want, html)
		}
	}

	// Non-dir entries must not get the dir badge.
	planLine := lineContaining(html, "2026-05-28-mvp.html")
	if strings.Contains(planLine, "badge-muted") {
		t.Errorf("file entry got dir badge: %s", planLine)
	}
}

func TestRenderIndex_EmptyEntries(t *testing.T) {
	page := IndexPage{
		Title:      "demo @ main",
		H1:         "demo",
		RefLabel:   "main",
		Breadcrumb: []Crumb{{Label: "demo"}},
	}
	out, err := RenderIndex(page)
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)

	if !strings.Contains(html, "Empty directory") {
		t.Errorf("missing empty-state block:\n%s", html)
	}
	if strings.Contains(html, `class="grid-2"`) {
		t.Errorf("empty render should not include grid block")
	}
	if !strings.Contains(html, "0 entries") {
		t.Errorf("missing zero-entry count")
	}
}

func TestRenderIndex_EscapesUserContent(t *testing.T) {
	page := IndexPage{
		Title:    "evil @ main",
		H1:       "<script>x</script>",
		RefLabel: "main",
		Entries: []Entry{
			{Label: `oops".html`, Href: `"><img src=x>`},
		},
	}
	out, err := RenderIndex(page)
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)

	if strings.Contains(html, "<script>x</script>") {
		t.Errorf("html-injection in H1 not escaped:\n%s", html)
	}
	if strings.Contains(html, `"><img src=x>`) {
		t.Errorf("html-injection in href not escaped:\n%s", html)
	}
}

// lineContaining returns the line in s that contains needle. Empty if none.
func lineContaining(s, needle string) string {
	for line := range strings.SplitSeq(s, "\n") {
		if strings.Contains(line, needle) {
			return line
		}
	}
	return ""
}

func TestMergedBanner(t *testing.T) {
	got := string(MergedBanner("main", "/p/demo/reviews/done.html", "abc1234"))

	for _, want := range []string{
		"Merged",
		"abc1234",
		"main",
		`href="/p/demo/reviews/done.html"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("banner missing %q:\n%s", want, got)
		}
	}
}

func TestMergedBanner_EscapesInputs(t *testing.T) {
	// A hostile ref name must not break out of the markup.
	got := string(MergedBanner(`x"><script>alert(1)</script>`, "/p/demo/", "deadbee"))
	if strings.Contains(got, "<script>") {
		t.Errorf("script tag not escaped:\n%s", got)
	}
}
