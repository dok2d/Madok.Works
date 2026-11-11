package mindmap

import (
	"fmt"
	"html"
	"strings"
)

// TitleResolver maps a wiki page id to its title ("" when unknown). Used when
// rendering node → page links as [[Title]] tokens.
type TitleResolver func(pageID int64) string

// Markdown renders the map as a markmap-style outline: the root as an H1, every
// other node as a nested list item. Markers are encoded in a readable, mostly
// reversible way (ParseMarkdown understands all of them):
//
//   - [x] done task        - [ ] open task
//   - [text](url)          - 🚀 icon then text
//   - text #tag #tag2      - text (P1) (50%)
//   - [[Wiki page]] / [[KEY-12]] links appended
//     > note lines (indented blockquote under the item)
//
// This is the text that goes into wiki_pages.content_md, so revision diffs,
// Markdown export and LLM summaries all see a plain outline.
func (d *Doc) Markdown(titles TitleResolver) string {
	var b strings.Builder
	b.WriteString("# " + nodeInline(d.Root, titles) + "\n")
	if d.Root.Note != "" {
		for _, ln := range strings.Split(d.Root.Note, "\n") {
			b.WriteString("> " + ln + "\n")
		}
	}
	if len(d.Root.Children) > 0 {
		b.WriteString("\n")
	}
	var rec func(n *Node, depth int)
	rec = func(n *Node, depth int) {
		ind := strings.Repeat("  ", depth)
		b.WriteString(ind + "- " + nodeInline(n, titles) + "\n")
		if n.Note != "" {
			for _, ln := range strings.Split(n.Note, "\n") {
				b.WriteString(ind + "  > " + ln + "\n")
			}
		}
		for _, c := range n.Children {
			rec(c, depth+1)
		}
	}
	for _, c := range d.Root.Children {
		rec(c, 0)
	}
	if len(d.Links) > 0 {
		b.WriteString("\n" + linksMarker + "\n")
		for _, l := range d.Links {
			from, _ := d.Find(l.From)
			to, _ := d.Find(l.To)
			if from == nil || to == nil {
				continue
			}
			lab := ""
			if l.Label != "" {
				lab = " (" + l.Label + ")"
			}
			b.WriteString(fmt.Sprintf("- %s → %s%s\n", firstLine(from.Text), firstLine(to.Text), lab))
		}
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// nodeInline renders one node's text plus its inline markers.
func nodeInline(n *Node, titles TitleResolver) string {
	var parts []string
	if n.Done != nil {
		if *n.Done {
			parts = append(parts, "[x]")
		} else {
			parts = append(parts, "[ ]")
		}
	}
	if n.Icon != "" {
		parts = append(parts, n.Icon)
	}
	text := strings.ReplaceAll(n.Text, "\n", " ")
	if n.URL != "" {
		text = "[" + text + "](" + n.URL + ")"
	}
	parts = append(parts, text)
	if n.Priority > 0 {
		parts = append(parts, fmt.Sprintf("(P%d)", n.Priority))
	}
	if n.Progress != nil {
		parts = append(parts, fmt.Sprintf("(%d%%)", *n.Progress))
	}
	for _, t := range n.Tags {
		parts = append(parts, "#"+strings.ReplaceAll(t, " ", "_"))
	}
	if n.PageID > 0 && titles != nil {
		if t := titles(n.PageID); t != "" {
			parts = append(parts, "[["+t+"]]")
		}
	}
	if n.TaskRef != "" {
		parts = append(parts, "[["+n.TaskRef+"]]")
	}
	return strings.Join(parts, " ")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// HTML renders the map as a nested list. It is what wiki_pages.content holds
// for a mind-map page: the search index, snippets, printing and the fallback
// view for clients without the canvas all read this. The output only uses tags
// the wiki sanitizer allows.
func (d *Doc) HTML(titles TitleResolver) string {
	var b strings.Builder
	b.WriteString("<h1>" + html.EscapeString(firstLine(d.Root.Text)) + "</h1>\n")
	if d.Root.Note != "" {
		b.WriteString("<blockquote><p>" + strings.ReplaceAll(html.EscapeString(d.Root.Note), "\n", "<br>") + "</p></blockquote>\n")
	}
	var rec func(nodes []*Node)
	rec = func(nodes []*Node) {
		if len(nodes) == 0 {
			return
		}
		b.WriteString("<ul>")
		for _, n := range nodes {
			b.WriteString("<li>")
			if n.Done != nil {
				if *n.Done {
					b.WriteString(`<input type="checkbox" checked disabled> `)
				} else {
					b.WriteString(`<input type="checkbox" disabled> `)
				}
			}
			if n.Icon != "" {
				b.WriteString(html.EscapeString(n.Icon) + " ")
			}
			text := html.EscapeString(strings.ReplaceAll(n.Text, "\n", " "))
			if n.URL != "" {
				b.WriteString(`<a href="` + html.EscapeString(n.URL) + `">` + text + "</a>")
			} else {
				b.WriteString(text)
			}
			if n.Priority > 0 {
				b.WriteString(fmt.Sprintf(" <span>(P%d)</span>", n.Priority))
			}
			if n.Progress != nil {
				b.WriteString(fmt.Sprintf(" <span>(%d%%)</span>", *n.Progress))
			}
			for _, t := range n.Tags {
				b.WriteString(" <span>#" + html.EscapeString(t) + "</span>")
			}
			if n.PageID > 0 && titles != nil {
				if t := titles(n.PageID); t != "" {
					b.WriteString(" [[" + html.EscapeString(t) + "]]")
				}
			}
			if n.TaskRef != "" {
				b.WriteString(" [[" + html.EscapeString(n.TaskRef) + "]]")
			}
			if n.Note != "" {
				b.WriteString("<blockquote><p>" + strings.ReplaceAll(html.EscapeString(n.Note), "\n", "<br>") + "</p></blockquote>")
			}
			rec(n.Children)
			b.WriteString("</li>")
		}
		b.WriteString("</ul>")
	}
	rec(d.Root.Children)
	b.WriteString("\n")
	return b.String()
}

// PlainText renders the map as a tab-indented outline (one node per line).
func (d *Doc) PlainText() string {
	var b strings.Builder
	d.Walk(func(n, _ *Node, depth int) bool {
		b.WriteString(strings.Repeat("\t", depth) + strings.ReplaceAll(n.Text, "\n", " ") + "\n")
		return true
	})
	return b.String()
}
