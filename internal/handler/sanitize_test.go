package handler

import (
	"strings"
	"testing"
)

// TestSanitizeKeepsMarkedOutput ensures representative HTML produced by the
// client-side markdown renderer (marked) survives sanitization, so wiki pages
// authored in Markdown render correctly after the server sanitizes them.
func TestSanitizeKeepsMarkedOutput(t *testing.T) {
	samples := []string{
		`<h1>Title</h1>`,
		`<p>text with <strong>bold</strong> and <em>em</em> and <code>code</code></p>`,
		`<ul>` + "\n" + `<li>one</li>` + "\n" + `<li>two</li>` + "\n" + `</ul>`,
		`<pre><code class="language-js">const x = 1;</code></pre>`,
		`<table>` + "\n" + `<thead><tr><th>A</th><th>B</th></tr></thead>` + "\n" + `<tbody><tr><td>1</td><td>2</td></tr></tbody>` + "\n" + `</table>`,
		`<ul class="task-list"><li class="task-item"><input type="checkbox" disabled> todo</li></ul>`,
		`<blockquote><p>quoted</p></blockquote>`,
		`<p><a href="https://example.com">link</a></p>`,
	}
	for _, s := range samples {
		out := sanitizeHTML(s)
		if strings.TrimSpace(out) == "" {
			t.Errorf("sanitizer dropped everything for: %s", s)
		}
		// core tags must survive
		for _, tag := range []string{"<h1", "<table", "<pre", "<blockquote", "<ul", "<a "} {
			if strings.Contains(s, tag) && !strings.Contains(out, tag) {
				t.Errorf("sanitizer removed %q from %q -> %q", tag, s, out)
			}
		}
	}
}

func TestSanitizeHTMLKeepsSafeMarkup(t *testing.T) {
	cases := []struct{ in, want string }{
		{"<p>hello</p>", "<p>hello</p>"},
		{"<h1>T</h1><h2>S</h2>", "<h1>T</h1><h2>S</h2>"},
		{"<b>x</b> <i>y</i> <u>z</u>", "<b>x</b> <i>y</i> <u>z</u>"},
		{`<ul><li>a</li><li>b</li></ul>`, `<ul><li>a</li><li>b</li></ul>`},
		{`<a href="https://x.io" title="t">l</a>`, `<a href="https://x.io" title="t" target="_blank" rel="noopener noreferrer">l</a>`},
		{`<a href="/task/5">task</a>`, `<a href="/task/5">task</a>`},
		{`<img src="/api/files/3" alt="pic">`, `<img src="/api/files/3" alt="pic">`},
		{`<pre><code class="language-go">f()</code></pre>`, `<pre><code class="language-go">f()</code></pre>`},
		{`<table><tr><td colspan="2">c</td></tr></table>`, `<table><tr><td colspan="2">c</td></tr></table>`},
		{`<div class="wiki-info">i</div>`, `<div class="wiki-info">i</div>`},
		{`<details><summary>More</summary><p>hidden</p></details>`, `<details><summary>More</summary><p>hidden</p></details>`},
		{`<div class="wiki-subpages"></div>`, `<div class="wiki-subpages"></div>`},
		{`<div class="wiki-subpages wiki-subpages-tree"></div>`, `<div class="wiki-subpages wiki-subpages-tree"></div>`},
		{"plain & <text>", "plain &amp; "},
	}
	for _, c := range cases {
		if got := sanitizeHTML(c.in); got != c.want {
			t.Errorf("sanitizeHTML(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestSanitizeHTMLBlocksXSS(t *testing.T) {
	vectors := []string{
		`<script>alert(1)</script>`,
		`<SCRIPT SRC=//evil.com/x.js></SCRIPT>`,
		`<img src=x onerror=alert(1)>`,
		`<img src="javascript:alert(1)">`,
		`<a href="javascript:alert(1)">x</a>`,
		`<a href="JaVaScRiPt:alert(1)">x</a>`,
		`<a href="data:text/html,<script>alert(1)</script>">x</a>`,
		`<iframe src="https://evil.com"></iframe>`,
		`<object data="x"></object>`,
		`<embed src="x">`,
		`<svg onload=alert(1)>`,
		`<math><mtext></mtext></math>`,
		`<style>body{background:url(javascript:alert(1))}</style>`,
		`<form action="/api/users"><input type="submit"></form>`,
		`<p onclick="alert(1)">x</p>`,
		`<p onmouseover='alert(1)'>x</p>`,
		`<div style="background:url(javascript:alert(1))">x</div>`,
		`<link rel="stylesheet" href="evil.css">`,
		`<meta http-equiv="refresh" content="0;url=evil">`,
		`<img src="data:image/svg+xml;base64,PHN2Zz4=">`,
		`<input type="text" onfocus="alert(1)" autofocus>`,
		`<details ontoggle=alert(1) open>x</details>`,
		`<!--<script>alert(1)</script>-->`,
	}
	for _, v := range vectors {
		got := sanitizeHTML(v)
		lower := strings.ToLower(got)
		for _, bad := range []string{"<script", "javascript:", "onerror", "onclick", "onload", "onfocus", "ontoggle", "onmouseover", "<iframe", "<object", "<embed", "<svg", "<math", "<style", "<form", "<link", "<meta", "style=", "data:image/svg"} {
			if strings.Contains(lower, bad) {
				t.Errorf("sanitizeHTML(%q) kept %q: %q", v, bad, got)
			}
		}
	}
}

func TestSanitizeHTMLClosesTags(t *testing.T) {
	got := sanitizeHTML("<p><b>unclosed")
	if got != "<p><b>unclosed</b></p>" {
		t.Errorf("unbalanced input not closed: %q", got)
	}
	got = sanitizeHTML("<ul><li>a<li>b</ul>")
	if strings.Count(got, "<li>") != strings.Count(got, "</li>") {
		t.Errorf("li not balanced: %q", got)
	}
}

func TestSanitizeHTMLInputCheckbox(t *testing.T) {
	got := sanitizeHTML(`<input type="checkbox" checked>`)
	if !strings.Contains(got, `type="checkbox"`) || !strings.Contains(got, "disabled") {
		t.Errorf("checkbox not preserved/disabled: %q", got)
	}
	got = sanitizeHTML(`<input type="password">`)
	if strings.Contains(got, "password") {
		t.Errorf("non-checkbox input type kept: %q", got)
	}
}
