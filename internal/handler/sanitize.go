package handler

import (
	"html"
	"strings"
	"unicode"
)

// sanitizeHTML is a strict whitelist sanitizer for WYSIWYG content.
//
// It re-emits only known-safe tags with known-safe attributes; everything
// else — unknown tags, comments, processing instructions, event handlers,
// javascript: URLs — is dropped or escaped. Combined with the global CSP it
// keeps stored content inert. No external dependencies.

// wikiAllowedTags maps tag -> allowed attributes for that tag.
var wikiAllowedTags = map[string]map[string]bool{
	"p": {}, "br": {}, "hr": {},
	"h1": {}, "h2": {}, "h3": {}, "h4": {},
	"b": {}, "strong": {}, "i": {}, "em": {}, "u": {}, "s": {}, "del": {}, "mark": {}, "sub": {}, "sup": {},
	"blockquote": {},
	"pre":        {},
	"code":       {"class": true}, // class="language-xxx" for highlight.js
	"ul":         {}, "ol": {"start": true}, "li": {},
	"a":   {"href": true, "title": true},
	"img": {"src": true, "alt": true, "title": true, "width": true, "height": true},
	"table": {}, "thead": {}, "tbody": {}, "tr": {},
	"th": {"colspan": true, "rowspan": true}, "td": {"colspan": true, "rowspan": true},
	"div":  {"class": true},
	"span": {"class": true},
	"figure": {}, "figcaption": {},
	"details": {"open": true}, "summary": {},
	"input": {"type": true, "checked": true, "disabled": true}, // task-list checkboxes only
}

// wikiVoidTags need no closing tag.
var wikiVoidTags = map[string]bool{"br": true, "hr": true, "img": true, "input": true}

// wikiAllowedClasses restricts class attribute values to a known set
// (anything else is removed). Language classes for code blocks pass через
// префикс language-.
var wikiAllowedClasses = map[string]bool{
	"wiki-info": true, "wiki-warn": true, "wiki-note": true, "wiki-success": true, // callout panels
	"wiki-subpages": true, "wiki-subpages-tree": true, // subpage table-of-contents placeholder (filled live on render)
	"task-list": true, "task-item": true,
	"hljs": true,
}

func classAllowed(c string) bool {
	return wikiAllowedClasses[c] || strings.HasPrefix(c, "language-")
}

// safeURL reports whether an href/src value uses an allowed scheme.
func safeURL(v string, isImage bool) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return false
	}
	if strings.HasPrefix(v, "/") || strings.HasPrefix(v, "#") {
		return true // internal links and uploads
	}
	for _, p := range []string{"http://", "https://", "mailto:"} {
		if strings.HasPrefix(v, p) {
			return !isImage || p != "mailto:"
		}
	}
	if isImage && strings.HasPrefix(v, "data:image/") {
		// allow small inline pasted images; reject anything else data:
		return !strings.Contains(v, "svg")
	}
	return false
}

// sanitizeHTML parses input loosely and rebuilds a safe document.
func sanitizeHTML(input string) string {
	var b strings.Builder
	b.Grow(len(input))
	var openStack []string
	i := 0
	n := len(input)

	for i < n {
		c := input[i]
		if c != '<' {
			// text run until next '<'
			j := strings.IndexByte(input[i:], '<')
			if j < 0 {
				j = n - i
			}
			b.WriteString(escapeTextChunk(input[i : i+j]))
			i += j
			continue
		}
		// comment or doctype/PI: skip entirely
		if strings.HasPrefix(input[i:], "<!--") {
			end := strings.Index(input[i+4:], "-->")
			if end < 0 {
				break
			}
			i += 4 + end + 3
			continue
		}
		if i+1 < n && (input[i+1] == '!' || input[i+1] == '?') {
			end := strings.IndexByte(input[i:], '>')
			if end < 0 {
				break
			}
			i += end + 1
			continue
		}
		// find tag end
		end := strings.IndexByte(input[i:], '>')
		if end < 0 {
			b.WriteString("&lt;")
			i++
			continue
		}
		raw := input[i+1 : i+end] // between < and >
		i += end + 1

		closing := strings.HasPrefix(raw, "/")
		if closing {
			raw = raw[1:]
		}
		selfClose := strings.HasSuffix(raw, "/")
		if selfClose {
			raw = strings.TrimSuffix(raw, "/")
		}
		name, attrs := parseTag(raw)
		name = strings.ToLower(name)

		// script/style/iframe etc: drop tag AND its text content
		if name == "script" || name == "style" || name == "iframe" || name == "object" || name == "embed" ||
			name == "svg" || name == "math" || name == "form" || name == "link" || name == "meta" || name == "base" {
			if !closing && !wikiVoidTags[name] {
				// skip to matching close tag (best effort)
				lower := strings.ToLower(input)
				closeTag := "</" + name
				idx := strings.Index(lower[i:], closeTag)
				if idx >= 0 {
					skip := i + idx
					gt := strings.IndexByte(input[skip:], '>')
					if gt >= 0 {
						i = skip + gt + 1
					} else {
						i = n
					}
				}
			}
			continue
		}

		allowedAttrs, ok := wikiAllowedTags[name]
		if !ok {
			continue // unknown tag: drop the tag, keep surrounding text flow
		}

		if closing {
			if wikiVoidTags[name] {
				continue
			}
			// pop until matching open tag (tolerates misnesting)
			for k := len(openStack) - 1; k >= 0; k-- {
				top := openStack[k]
				openStack = openStack[:k]
				b.WriteString("</" + top + ">")
				if top == name {
					break
				}
			}
			continue
		}

		// build sanitized open tag
		var tag strings.Builder
		tag.WriteByte('<')
		tag.WriteString(name)
		for _, a := range attrs {
			key := strings.ToLower(a.name)
			if !allowedAttrs[key] {
				continue
			}
			val := a.value
			switch key {
			case "href":
				if !safeURL(val, false) {
					continue
				}
			case "src":
				if !safeURL(val, true) {
					continue
				}
			case "class":
				var keep []string
				for _, cl := range strings.Fields(val) {
					if classAllowed(cl) {
						keep = append(keep, cl)
					}
				}
				if len(keep) == 0 {
					continue
				}
				val = strings.Join(keep, " ")
			case "type":
				if name == "input" && val != "checkbox" {
					continue
				}
			case "colspan", "rowspan", "width", "height", "start":
				if !allDigits(val) {
					continue
				}
			}
			tag.WriteByte(' ')
			tag.WriteString(key)
			tag.WriteString(`="`)
			tag.WriteString(html.EscapeString(val))
			tag.WriteByte('"')
		}
		if name == "input" {
			tag.WriteString(` disabled`)
		}
		if name == "a" {
			// external links open safely
			for _, a := range attrs {
				if strings.ToLower(a.name) == "href" && strings.HasPrefix(strings.ToLower(strings.TrimSpace(a.value)), "http") {
					tag.WriteString(` target="_blank" rel="noopener noreferrer"`)
					break
				}
			}
		}
		tag.WriteByte('>')
		b.WriteString(tag.String())
		if !wikiVoidTags[name] && !selfClose {
			openStack = append(openStack, name)
		} else if selfClose && !wikiVoidTags[name] {
			b.WriteString("</" + name + ">")
		}
	}
	// close anything left open
	for k := len(openStack) - 1; k >= 0; k-- {
		b.WriteString("</" + openStack[k] + ">")
	}
	return b.String()
}

// escapeTextChunk normalizes a text run: existing entities pass through,
// stray < > & quotes are escaped.
func escapeTextChunk(s string) string {
	// Unescape then re-escape to neutralize crafted entity sequences while
	// preserving literal text.
	return html.EscapeString(html.UnescapeString(s))
}

type tagAttr struct{ name, value string }

// parseTag splits `name attr="v" attr2='v' attr3=v flag` into parts.
func parseTag(raw string) (string, []tagAttr) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	// tag name
	end := 0
	for end < len(raw) && !unicode.IsSpace(rune(raw[end])) {
		end++
	}
	name := raw[:end]
	rest := raw[end:]
	var attrs []tagAttr
	i := 0
	n := len(rest)
	for i < n {
		for i < n && unicode.IsSpace(rune(rest[i])) {
			i++
		}
		if i >= n {
			break
		}
		// attr name
		st := i
		for i < n && rest[i] != '=' && !unicode.IsSpace(rune(rest[i])) {
			i++
		}
		aname := rest[st:i]
		for i < n && unicode.IsSpace(rune(rest[i])) {
			i++
		}
		if i >= n || rest[i] != '=' {
			if aname != "" {
				attrs = append(attrs, tagAttr{aname, ""}) // boolean attr
			}
			continue
		}
		i++ // skip '='
		for i < n && unicode.IsSpace(rune(rest[i])) {
			i++
		}
		if i >= n {
			attrs = append(attrs, tagAttr{aname, ""})
			break
		}
		var aval string
		if rest[i] == '"' || rest[i] == '\'' {
			q := rest[i]
			i++
			st = i
			for i < n && rest[i] != q {
				i++
			}
			aval = rest[st:i]
			if i < n {
				i++
			}
		} else {
			st = i
			for i < n && !unicode.IsSpace(rune(rest[i])) {
				i++
			}
			aval = rest[st:i]
		}
		attrs = append(attrs, tagAttr{aname, html.UnescapeString(aval)})
	}
	return name, attrs
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
