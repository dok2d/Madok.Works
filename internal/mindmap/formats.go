package mindmap

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// Formats the import/export endpoints understand.
const (
	FormatJSON     = "json"
	FormatMarkdown = "md"
	FormatText     = "txt"
	FormatOPML     = "opml"
	FormatFreeMind = "mm"
	FormatXMind    = "xmind"
	FormatMermaid  = "mermaid"
)

// ExportFormats lists what Export can produce, with the file extension and MIME
// type used for downloads.
var ExportFormats = map[string]struct{ Ext, Mime string }{
	FormatJSON:     {".json", "application/json"},
	FormatMarkdown: {".md", "text/markdown; charset=utf-8"},
	FormatText:     {".txt", "text/plain; charset=utf-8"},
	FormatOPML:     {".opml", "text/x-opml; charset=utf-8"},
	FormatFreeMind: {".mm", "application/x-freemind; charset=utf-8"},
	FormatXMind:    {".xmind", "application/vnd.xmind.workbook"},
	FormatMermaid:  {".mmd", "text/plain; charset=utf-8"},
}

// ImportOptions tune the parsers.
type ImportOptions struct {
	// Title is used as the root text when the source has no single root
	// (e.g. an outline with several top-level items).
	Title string
	// ResolvePage maps a [[Title]] token to a wiki page id (0 = unknown).
	ResolvePage func(title string) int64
}

// DetectFormat guesses the format from a filename extension, then the content.
func DetectFormat(filename string, data []byte) string {
	switch strings.ToLower(path.Ext(filename)) {
	case ".mm":
		return FormatFreeMind
	case ".opml", ".xml":
		if bytes.Contains(data[:min(len(data), 4096)], []byte("<map")) {
			return FormatFreeMind
		}
		return FormatOPML
	case ".xmind":
		return FormatXMind
	case ".json":
		return FormatJSON
	case ".mmd", ".mermaid":
		return FormatMermaid
	case ".md", ".markdown":
		return FormatMarkdown
	case ".txt":
		return FormatText
	}
	head := bytes.TrimSpace(data[:min(len(data), 4096)])
	switch {
	case bytes.HasPrefix(head, []byte("PK")):
		return FormatXMind
	case bytes.Contains(head, []byte("<map")):
		return FormatFreeMind
	case bytes.Contains(head, []byte("<opml")):
		return FormatOPML
	case bytes.HasPrefix(head, []byte("{")), bytes.HasPrefix(head, []byte("[")):
		return FormatJSON
	}
	first := strings.TrimSpace(strings.SplitN(string(head), "\n", 2)[0])
	if first == "mindmap" || strings.HasPrefix(first, "mindmap ") {
		return FormatMermaid
	}
	for _, ln := range strings.Split(string(head), "\n") {
		t := strings.TrimLeft(ln, " \t")
		if strings.HasPrefix(t, "- ") || strings.HasPrefix(t, "* ") || strings.HasPrefix(t, "# ") || strings.HasPrefix(t, "+ ") {
			return FormatMarkdown
		}
	}
	return FormatText
}

// Import parses data in the given format ("" = detect from filename/content).
func Import(format, filename string, data []byte, opt ImportOptions) (*Doc, error) {
	if len(data) > MaxDocBytes*4 {
		return nil, errors.New("file too large")
	}
	if format == "" {
		format = DetectFormat(filename, data)
	}
	var d *Doc
	var err error
	switch format {
	case FormatJSON:
		d, err = parseJSON(data, opt)
	case FormatMarkdown:
		d, err = ParseMarkdown(string(data), opt)
	case FormatText:
		d, err = ParseText(string(data), opt)
	case FormatOPML:
		d, err = ParseOPML(data, opt)
	case FormatFreeMind:
		d, err = ParseFreeMind(data, opt)
	case FormatXMind:
		d, err = ParseXMind(data, opt)
	case FormatMermaid:
		d, err = ParseMermaid(string(data), opt)
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
	if err != nil {
		return nil, err
	}
	if err := d.Normalize(); err != nil {
		return nil, err
	}
	return d, nil
}

// Export renders the document in the format; xmind is binary (zip).
func (d *Doc) Export(format string, titles TitleResolver) ([]byte, error) {
	switch format {
	case FormatJSON:
		b, _ := json.MarshalIndent(d, "", "  ")
		return b, nil
	case FormatMarkdown:
		return []byte(d.Markdown(titles)), nil
	case FormatText:
		return []byte(d.PlainText()), nil
	case FormatOPML:
		return []byte(d.OPML()), nil
	case FormatFreeMind:
		return []byte(d.FreeMind()), nil
	case FormatXMind:
		return d.XMind()
	case FormatMermaid:
		return []byte(d.Mermaid()), nil
	}
	return nil, fmt.Errorf("unknown format %q", format)
}

// parseJSON accepts our own document, an XMind content.json array, or a bare
// node tree ({"text": ..., "children": [...]}).
func parseJSON(data []byte, opt ImportOptions) (*Doc, error) {
	t := bytes.TrimSpace(data)
	if bytes.HasPrefix(t, []byte("[")) {
		return parseXMindContent(t, opt)
	}
	var probe struct {
		Root  *Node `json:"root"`
		Text  string
		Topic string
	}
	if err := json.Unmarshal(t, &probe); err != nil {
		return nil, fmt.Errorf("bad json: %w", err)
	}
	if probe.Root != nil {
		return Parse(t)
	}
	// A bare node (ours, or mind-elixir "topic" naming).
	n, err := parseLooseNode(t)
	if err != nil {
		return nil, err
	}
	d := New("")
	d.Root = n
	return d, nil
}

// parseLooseNode reads a node that may use mind-elixir field names.
func parseLooseNode(data []byte) (*Node, error) {
	var m struct {
		Node
		Topic     string            `json:"topic"`
		Expanded  *bool             `json:"expanded"`
		HyperLink string            `json:"hyperLink"`
		Icons     []string          `json:"icons"`
		Kids      []json.RawMessage `json:"children"`
		NodeData  json.RawMessage   `json:"nodeData"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("bad json: %w", err)
	}
	if len(m.NodeData) > 0 { // whole mind-elixir document
		return parseLooseNode(m.NodeData)
	}
	n := m.Node
	n.Children = nil
	if n.Text == "" {
		n.Text = m.Topic
	}
	if n.URL == "" {
		n.URL = m.HyperLink
	}
	if m.Expanded != nil && !*m.Expanded {
		n.Collapsed = true
	}
	if n.Icon == "" && len(m.Icons) > 0 {
		n.Icon = m.Icons[0]
	}
	for _, k := range m.Kids {
		c, err := parseLooseNode(k)
		if err != nil {
			return nil, err
		}
		n.Children = append(n.Children, c)
	}
	return &n, nil
}

// === Markdown / text outlines ===

var (
	headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*)$`)
	listRe    = regexp.MustCompile(`^([ \t]*)([-*+]|\d+[.)])\s+(.*)$`)
	foldRe    = regexp.MustCompile(`<!--\s*(?:markmap:\s*)?fold\s*-->`)
	mdLinkRe  = regexp.MustCompile(`^\[([^\]]*)\]\(([^)\s]+)\)`)
	tagRe     = regexp.MustCompile(`(?:^|\s)#([\p{L}\p{N}_-]+)`)
	prioRe    = regexp.MustCompile(`\s\(P([1-6])\)`)
	progRe    = regexp.MustCompile(`\s\((\d{1,3})%\)`)
	wikiTokRe = regexp.MustCompile(`\[\[\s*([^\]\n|]+?)\s*\]\]`)
)

type outlineItem struct {
	depth int
	node  *Node
}

// ParseMarkdown builds a map from a markmap-style outline: headings become
// branches by level, list items nest by indentation under the current heading,
// blockquote/paragraph lines become the note of the preceding node.
func ParseMarkdown(src string, opt ImportOptions) (*Doc, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	src = stripFrontMatter(src)
	virtual := &Node{ID: NewID(), Text: strings.TrimSpace(opt.Title)}
	// stack of (depth, node): depth -1 is the virtual root; headings use
	// depths 0..5, list items use 100+indentLevel relative to the last heading.
	stack := []outlineItem{{-1, virtual}}
	var last *Node
	headingDepth := -1
	indentUnit := detectIndent(src)
	inCode, inLinks := false, false
	var linkLines []string
	for _, raw := range strings.Split(src, "\n") {
		line := strings.TrimRight(raw, " \t")
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inCode = !inCode
			continue
		}
		if inCode || strings.TrimSpace(line) == "" {
			continue
		}
		if strings.TrimSpace(line) == linksMarker {
			inLinks = true
			continue
		}
		if inLinks {
			if m := listRe.FindStringSubmatch(line); m != nil {
				linkLines = append(linkLines, m[3])
				continue
			}
			inLinks = false
		}
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "<!--") && strings.HasSuffix(t, "-->") {
			continue
		}
		if m := headingRe.FindStringSubmatch(line); m != nil {
			depth := len(m[1]) - 1
			n := inlineNode(m[2], opt)
			for len(stack) > 1 && stack[len(stack)-1].depth >= depth {
				stack = stack[:len(stack)-1]
			}
			parent := stack[len(stack)-1].node
			parent.Children = append(parent.Children, n)
			stack = append(stack, outlineItem{depth, n})
			last, headingDepth = n, depth
			continue
		}
		if m := listRe.FindStringSubmatch(line); m != nil {
			lvl := indentLevel(m[1], indentUnit)
			depth := 100 + lvl
			n := inlineNode(m[3], opt)
			for len(stack) > 1 && stack[len(stack)-1].depth >= depth {
				stack = stack[:len(stack)-1]
			}
			// A list item deeper than its predecessor by more than one level still
			// nests one level down (sloppy indentation).
			parent := stack[len(stack)-1].node
			parent.Children = append(parent.Children, n)
			stack = append(stack, outlineItem{depth, n})
			last = n
			continue
		}
		// Note text: blockquote or plain paragraph under the last node.
		t := strings.TrimSpace(line)
		t = strings.TrimPrefix(t, ">")
		t = strings.TrimPrefix(t, " ")
		if last != nil {
			if last.Note != "" {
				last.Note += "\n"
			}
			last.Note += t
		} else if virtual.Note == "" {
			virtual.Note = t
		} else {
			virtual.Note += "\n" + t
		}
	}
	_ = headingDepth
	d := finishOutline(virtual, opt)
	d.Links = parseLinkLines(d, linkLines)
	return d, nil
}

// linksMarker introduces the relationship list at the end of a Markdown export.
const linksMarker = "<!-- mindmap:links -->"

var linkLineRe = regexp.MustCompile(`^(.*?)\s+→\s+(.*?)(?:\s+\((.*)\))?$`)

// parseLinkLines resolves "A → B (label)" lines by node text (first match wins).
func parseLinkLines(d *Doc, lines []string) []Link {
	var out []Link
	find := func(text string) string {
		id := ""
		d.Walk(func(n, _ *Node, _ int) bool {
			if firstLine(n.Text) == text {
				id = n.ID
				return false
			}
			return true
		})
		return id
	}
	for _, ln := range lines {
		m := linkLineRe.FindStringSubmatch(strings.TrimSpace(ln))
		if m == nil {
			continue
		}
		from, to := find(m[1]), find(m[2])
		if from == "" || to == "" || from == to {
			continue
		}
		out = append(out, Link{ID: NewID(), From: from, To: to, Label: m[3]})
	}
	return out
}

// ParseText builds a map from a plain indented outline (tabs or spaces, one
// node per line). Inline markers are understood like in Markdown.
func ParseText(src string, opt ImportOptions) (*Doc, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	virtual := &Node{ID: NewID(), Text: strings.TrimSpace(opt.Title)}
	stack := []outlineItem{{-1, virtual}}
	unit := detectIndent(src)
	for _, raw := range strings.Split(src, "\n") {
		if strings.TrimSpace(raw) == "" {
			continue
		}
		ws := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
		text := strings.TrimSpace(raw)
		if m := listRe.FindStringSubmatch(raw); m != nil {
			text = m[3]
		}
		depth := indentLevel(ws, unit)
		for len(stack) > 1 && stack[len(stack)-1].depth >= depth {
			stack = stack[:len(stack)-1]
		}
		n := inlineNode(text, opt)
		parent := stack[len(stack)-1].node
		parent.Children = append(parent.Children, n)
		stack = append(stack, outlineItem{depth, n})
	}
	return finishOutline(virtual, opt), nil
}

// finishOutline turns the virtual root into the document root: a single
// top-level item becomes the root itself, otherwise the virtual root (titled
// from the options) holds them all.
func finishOutline(virtual *Node, opt ImportOptions) *Doc {
	d := New("")
	if len(virtual.Children) == 1 && virtual.Note == "" {
		d.Root = virtual.Children[0]
	} else {
		if virtual.Text == "" {
			virtual.Text = "Mind map"
		}
		d.Root = virtual
	}
	return d
}

func stripFrontMatter(src string) string {
	if !strings.HasPrefix(src, "---\n") {
		return src
	}
	if i := strings.Index(src[4:], "\n---"); i >= 0 {
		rest := src[4+i+4:]
		if j := strings.IndexByte(rest, '\n'); j >= 0 {
			return rest[j+1:]
		}
		return ""
	}
	return src
}

// detectIndent returns the smallest non-zero indentation width in spaces (tabs
// count as 4) so both 2- and 4-space outlines nest correctly.
func detectIndent(src string) int {
	unit := 0
	for _, ln := range strings.Split(src, "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		w := indentWidth(ln[:len(ln)-len(strings.TrimLeft(ln, " \t"))])
		if w > 0 && (unit == 0 || w < unit) {
			unit = w
		}
	}
	if unit == 0 {
		unit = 2
	}
	return unit
}

func indentWidth(ws string) int {
	w := 0
	for _, r := range ws {
		if r == '\t' {
			w += 4
		} else {
			w++
		}
	}
	return w
}

func indentLevel(ws string, unit int) int {
	if unit <= 0 {
		return 0
	}
	return indentWidth(ws) / unit
}

// inlineNode parses one outline line into a node: checkbox, leading emoji icon,
// link, (P1)/(50%) markers, #tags, [[page]]/[[KEY-1]] references, fold comment.
func inlineNode(text string, opt ImportOptions) *Node {
	n := &Node{ID: NewID()}
	if foldRe.MatchString(text) {
		n.Collapsed = true
		text = foldRe.ReplaceAllString(text, "")
	}
	text = strings.TrimSpace(text)
	switch {
	case strings.HasPrefix(text, "[ ] "):
		f := false
		n.Done = &f
		text = text[4:]
	case strings.HasPrefix(strings.ToLower(text), "[x] "):
		t := true
		n.Done = &t
		text = text[4:]
	}
	for _, m := range wikiTokRe.FindAllStringSubmatch(text, -1) {
		tok := strings.TrimSpace(m[1])
		if taskRe.MatchString(tok) {
			n.TaskRef = strings.ToUpper(tok)
		} else if opt.ResolvePage != nil && n.PageID == 0 {
			n.PageID = opt.ResolvePage(tok)
		}
	}
	text = strings.TrimSpace(wikiTokRe.ReplaceAllString(text, ""))
	for _, m := range tagRe.FindAllStringSubmatch(text, -1) {
		n.Tags = append(n.Tags, strings.ReplaceAll(m[1], "_", " "))
	}
	text = strings.TrimSpace(tagRe.ReplaceAllString(text, ""))
	if m := prioRe.FindStringSubmatch(text); m != nil {
		n.Priority, _ = strconv.Atoi(m[1])
		text = strings.TrimSpace(prioRe.ReplaceAllString(text, ""))
	}
	if m := progRe.FindStringSubmatch(text); m != nil {
		v, _ := strconv.Atoi(m[1])
		if v >= 0 && v <= 100 {
			n.Progress = &v
			text = strings.TrimSpace(progRe.ReplaceAllString(text, ""))
		}
	}
	if icon, rest, ok := leadingIcon(text); ok {
		n.Icon = icon
		text = rest
	}
	if m := mdLinkRe.FindStringSubmatch(text); m != nil {
		n.URL = m[2]
		text = strings.TrimSpace(m[1] + text[len(m[0]):])
	}
	n.Text = text
	return n
}

// leadingIcon splits off a leading emoji token (no letters/digits in it).
func leadingIcon(s string) (icon, rest string, ok bool) {
	i := strings.IndexByte(s, ' ')
	if i <= 0 {
		return "", s, false
	}
	tok := s[:i]
	hasSymbol := false
	for _, r := range tok {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || unicode.IsPunct(r) && r < 0x2000 {
			return "", s, false
		}
		if r >= 0x2600 {
			hasSymbol = true
		}
	}
	if !hasSymbol || len([]rune(tok)) > 8 {
		return "", s, false
	}
	return tok, strings.TrimSpace(s[i+1:]), true
}

// === OPML ===

type opmlDoc struct {
	XMLName xml.Name      `xml:"opml"`
	Version string        `xml:"version,attr"`
	Title   string        `xml:"head>title"`
	Body    []opmlOutline `xml:"body>outline"`
}

type opmlOutline struct {
	Text     string        `xml:"text,attr"`
	Note     string        `xml:"_note,attr,omitempty"`
	Status   string        `xml:"_status,attr,omitempty"`
	URL      string        `xml:"url,attr,omitempty"`
	HTMLURL  string        `xml:"htmlUrl,attr,omitempty"`
	Children []opmlOutline `xml:"outline"`
}

// ParseOPML reads an OPML 2.0 outline (text/_note/_status/url attributes).
func ParseOPML(data []byte, opt ImportOptions) (*Doc, error) {
	var o opmlDoc
	if err := xml.Unmarshal(data, &o); err != nil {
		return nil, fmt.Errorf("bad opml: %w", err)
	}
	var conv func(x opmlOutline) *Node
	conv = func(x opmlOutline) *Node {
		n := &Node{ID: NewID(), Text: x.Text, Note: x.Note, URL: firstNonEmpty(x.URL, x.HTMLURL)}
		switch strings.ToLower(x.Status) {
		case "checked", "done", "complete":
			t := true
			n.Done = &t
		case "unchecked", "open", "todo":
			f := false
			n.Done = &f
		}
		for _, c := range x.Children {
			n.Children = append(n.Children, conv(c))
		}
		return n
	}
	virtual := &Node{ID: NewID(), Text: firstNonEmpty(strings.TrimSpace(opt.Title), o.Title)}
	for _, x := range o.Body {
		virtual.Children = append(virtual.Children, conv(x))
	}
	if len(virtual.Children) == 0 {
		return nil, errors.New("opml has no outline")
	}
	return finishOutline(virtual, opt), nil
}

// OPML renders the map as an OPML 2.0 outline.
func (d *Doc) OPML() string {
	var conv func(n *Node) opmlOutline
	conv = func(n *Node) opmlOutline {
		o := opmlOutline{Text: strings.ReplaceAll(n.Text, "\n", " "), Note: n.Note, URL: n.URL}
		if n.Done != nil {
			if *n.Done {
				o.Status = "checked"
			} else {
				o.Status = "unchecked"
			}
		}
		for _, c := range n.Children {
			o.Children = append(o.Children, conv(c))
		}
		return o
	}
	o := opmlDoc{Version: "2.0", Title: firstLine(d.Root.Text), Body: []opmlOutline{conv(d.Root)}}
	out, _ := xml.MarshalIndent(o, "", "  ")
	return xml.Header + string(out) + "\n"
}

// === FreeMind / Freeplane ===

type mmMap struct {
	XMLName xml.Name `xml:"map"`
	Version string   `xml:"version,attr"`
	Node    *mmNode  `xml:"node"`
}

type mmNode struct {
	ID         string    `xml:"ID,attr,omitempty"`
	Text       string    `xml:"TEXT,attr"`
	Position   string    `xml:"POSITION,attr,omitempty"`
	Folded     string    `xml:"FOLDED,attr,omitempty"`
	Link       string    `xml:"LINK,attr,omitempty"`
	Color      string    `xml:"COLOR,attr,omitempty"`
	Icons      []mmIcon  `xml:"icon"`
	Rich       []mmRich  `xml:"richcontent"`
	Arrows     []mmArrow `xml:"arrowlink"`
	Cloud      *struct{} `xml:"cloud"`
	Attributes []mmAttr  `xml:"attribute"`
	Nodes      []*mmNode `xml:"node"`
}

type mmIcon struct {
	Builtin string `xml:"BUILTIN,attr"`
}
type mmRich struct {
	Type string `xml:"TYPE,attr"`
	Body string `xml:",innerxml"`
}
type mmArrow struct {
	Destination string `xml:"DESTINATION,attr"`
}
type mmAttr struct {
	Name  string `xml:"NAME,attr"`
	Value string `xml:"VALUE,attr"`
}

// FreeMind built-in icon names ↔ emoji / markers.
var mmIconToEmoji = map[string]string{
	"idea": "💡", "flag": "🚩", "flag-red": "🚩", "flag-green": "🟢", "flag-blue": "🔵", "flag-yellow": "🟡",
	"button_ok": "✅", "button_cancel": "❌", "help": "❓", "messagebox_warning": "⚠️", "stop": "⛔",
	"go": "🟢", "clock": "⏰", "bookmark": "🔖", "attach": "📎", "info": "ℹ️", "star": "⭐",
	"yes": "👍", "smiley-neutral": "😐", "ksmiletris": "🙂", "smily_bad": "🙁", "calendar": "📅",
	"mail": "✉️", "phone": "📞", "pencil": "✏️", "folder": "📁", "list": "📋", "wizard": "🧙",
	"password": "🔑", "bell": "🔔", "bomb": "💣", "broken-line": "⚡", "launch": "🚀", "closed": "🔒",
	"kaddressbook": "📇", "xmag": "🔍", "group": "👥", "female1": "👩", "male1": "👨", "forward": "➡️", "back": "⬅️", "up": "⬆️", "down": "⬇️",
}

// ParseFreeMind reads a FreeMind / Freeplane .mm file.
func ParseFreeMind(data []byte, opt ImportOptions) (*Doc, error) {
	var m mmMap
	if err := xml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("bad freemind xml: %w", err)
	}
	if m.Node == nil {
		return nil, errors.New("freemind map has no root node")
	}
	idMap := map[string]string{} // FreeMind ID -> ours
	var pendingArrows []struct{ from, to string }
	var conv func(x *mmNode) *Node
	conv = func(x *mmNode) *Node {
		n := &Node{ID: NewID(), Text: x.Text, URL: x.Link}
		if x.ID != "" {
			idMap[x.ID] = n.ID
		}
		if strings.EqualFold(x.Folded, "true") {
			n.Collapsed = true
		}
		if x.Position == "left" || x.Position == "right" {
			n.Side = x.Position
		}
		if colorRe.MatchString(x.Color) {
			n.Color = x.Color
		}
		if x.Cloud != nil {
			n.Boundary = true
		}
		for _, r := range x.Rich {
			switch strings.ToUpper(r.Type) {
			case "NOTE":
				n.Note = htmlToText(r.Body)
			case "NODE":
				if n.Text == "" {
					n.Text = htmlToText(r.Body)
				}
			}
		}
		for _, ic := range x.Icons {
			b := ic.Builtin
			if strings.HasPrefix(b, "full-") {
				if p, err := strconv.Atoi(strings.TrimPrefix(b, "full-")); err == nil && p >= 1 && p <= 6 {
					n.Priority = p
					continue
				}
			}
			if p, ok := strings.CutPrefix(b, "percent"); ok {
				if v, err := strconv.Atoi(p); err == nil {
					n.Progress = &v
					continue
				}
			}
			if b == "checked" || b == "button_ok" && n.Done == nil && n.Icon != "" {
				t := true
				n.Done = &t
				continue
			}
			if b == "unchecked" {
				f := false
				n.Done = &f
				continue
			}
			if e, ok := mmIconToEmoji[b]; ok && n.Icon == "" {
				n.Icon = e
			}
		}
		for _, a := range x.Attributes {
			if a.Name != "" {
				n.Tags = append(n.Tags, strings.TrimSpace(a.Name+" "+a.Value))
			}
		}
		for _, a := range x.Arrows {
			pendingArrows = append(pendingArrows, struct{ from, to string }{x.ID, a.Destination})
		}
		for _, c := range x.Nodes {
			n.Children = append(n.Children, conv(c))
		}
		return n
	}
	d := New("")
	d.Root = conv(m.Node)
	for _, a := range pendingArrows {
		from, to := idMap[a.from], idMap[a.to]
		if from != "" && to != "" {
			d.Links = append(d.Links, Link{ID: NewID(), From: from, To: to})
		}
	}
	return d, nil
}

// FreeMind renders the map as FreeMind 1.0.1 XML (readable by Freeplane, XMind,
// MindNode, MindManager…).
func (d *Doc) FreeMind() string {
	emojiToMM := map[string]string{}
	for k, v := range mmIconToEmoji {
		if _, dup := emojiToMM[v]; !dup {
			emojiToMM[v] = k
		}
	}
	linksFrom := map[string][]Link{}
	for _, l := range d.Links {
		linksFrom[l.From] = append(linksFrom[l.From], l)
	}
	var conv func(n *Node, isRoot bool) *mmNode
	conv = func(n *Node, isRoot bool) *mmNode {
		x := &mmNode{ID: "ID_" + n.ID, Text: strings.ReplaceAll(n.Text, "\n", " "), Link: n.URL, Color: n.Color}
		if n.Collapsed {
			x.Folded = "true"
		}
		if !isRoot && n.Side != "" {
			x.Position = n.Side
		}
		if n.Boundary {
			x.Cloud = &struct{}{}
		}
		if n.Priority > 0 {
			x.Icons = append(x.Icons, mmIcon{"full-" + strconv.Itoa(n.Priority)})
		}
		if n.Done != nil {
			if *n.Done {
				x.Icons = append(x.Icons, mmIcon{"button_ok"})
			} else {
				x.Icons = append(x.Icons, mmIcon{"unchecked"})
			}
		}
		if n.Icon != "" {
			if b, ok := emojiToMM[n.Icon]; ok {
				x.Icons = append(x.Icons, mmIcon{b})
			}
		}
		if n.Note != "" {
			x.Rich = append(x.Rich, mmRich{Type: "NOTE", Body: "<html><body><p>" + xmlEscape(strings.ReplaceAll(n.Note, "\n", "</p><p>")) + "</p></body></html>"})
		}
		for _, t := range n.Tags {
			x.Attributes = append(x.Attributes, mmAttr{Name: "tag", Value: t})
		}
		for _, l := range linksFrom[n.ID] {
			x.Arrows = append(x.Arrows, mmArrow{Destination: "ID_" + l.To})
		}
		for _, c := range n.Children {
			x.Nodes = append(x.Nodes, conv(c, false))
		}
		return x
	}
	m := mmMap{Version: "1.0.1", Node: conv(d.Root, true)}
	out, _ := xml.MarshalIndent(m, "", "  ")
	return xml.Header + string(out) + "\n"
}

func xmlEscape(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	// EscapeText escapes "</p><p>" separators too; restore them.
	return strings.ReplaceAll(b.String(), "&lt;/p&gt;&lt;p&gt;", "</p><p>")
}

var tagStripRe = regexp.MustCompile(`<[^>]*>`)

// htmlToText flattens rich content to plain text (paragraph/br → newline).
func htmlToText(h string) string {
	h = regexp.MustCompile(`(?i)<br\s*/?>|</p>|</div>|</li>`).ReplaceAllString(h, "\n")
	h = tagStripRe.ReplaceAllString(h, "")
	h = strings.ReplaceAll(h, "&nbsp;", " ")
	h = strings.ReplaceAll(h, "&lt;", "<")
	h = strings.ReplaceAll(h, "&gt;", ">")
	h = strings.ReplaceAll(h, "&quot;", "\"")
	h = strings.ReplaceAll(h, "&amp;", "&")
	lines := strings.Split(h, "\n")
	out := lines[:0]
	for _, ln := range lines {
		if t := strings.TrimSpace(ln); t != "" {
			out = append(out, t)
		}
	}
	return strings.Join(out, "\n")
}

// === XMind ===

type xmTopic struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Href     string   `json:"href,omitempty"`
	Branch   string   `json:"branch,omitempty"`
	Labels   []string `json:"labels,omitempty"`
	Markers  []xmMark `json:"markers,omitempty"`
	Notes    *xmNotes `json:"notes,omitempty"`
	Image    *xmImage `json:"image,omitempty"`
	Children *xmKids  `json:"children,omitempty"`
	Class    string   `json:"class,omitempty"`
	Struct   string   `json:"structureClass,omitempty"`
}
type xmMark struct {
	MarkerID string `json:"markerId"`
}
type xmNotes struct {
	Plain *struct {
		Content string `json:"content"`
	} `json:"plain,omitempty"`
}
type xmImage struct {
	Src string `json:"src"`
}
type xmKids struct {
	Attached []*xmTopic `json:"attached"`
}
type xmRel struct {
	ID    string `json:"id"`
	End1  string `json:"end1Id"`
	End2  string `json:"end2Id"`
	Title string `json:"title,omitempty"`
}
type xmSheet struct {
	ID            string   `json:"id"`
	Class         string   `json:"class"`
	Title         string   `json:"title"`
	RootTopic     *xmTopic `json:"rootTopic"`
	Relationships []xmRel  `json:"relationships,omitempty"`
}

// ParseXMind reads an XMind workbook (zip with content.json; XMind 8 content.xml
// is understood for titles/structure only).
func ParseXMind(data []byte, opt ImportOptions) (*Doc, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, errors.New("not an xmind file (zip expected)")
	}
	var contentJSON, contentXML []byte
	for _, f := range zr.File {
		if f.Name != "content.json" && f.Name != "content.xml" {
			continue
		}
		if f.UncompressedSize64 > uint64(MaxDocBytes)*4 {
			return nil, errors.New("xmind content too large")
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, int64(MaxDocBytes)*4))
		rc.Close()
		if err != nil {
			return nil, err
		}
		if f.Name == "content.json" {
			contentJSON = b
		} else {
			contentXML = b
		}
	}
	if contentJSON != nil {
		return parseXMindContent(contentJSON, opt)
	}
	if contentXML != nil {
		return parseXMindXML(contentXML, opt)
	}
	return nil, errors.New("xmind file has no content")
}

func parseXMindContent(data []byte, opt ImportOptions) (*Doc, error) {
	var sheets []xmSheet
	if err := json.Unmarshal(data, &sheets); err != nil {
		return nil, fmt.Errorf("bad xmind content.json: %w", err)
	}
	if len(sheets) == 0 || sheets[0].RootTopic == nil {
		return nil, errors.New("xmind content has no sheet")
	}
	idMap := map[string]string{}
	var conv func(t *xmTopic) *Node
	conv = func(t *xmTopic) *Node {
		n := &Node{ID: NewID(), Text: t.Title, URL: t.Href, Tags: t.Labels}
		idMap[t.ID] = n.ID
		if t.Branch == "folded" {
			n.Collapsed = true
		}
		if t.Notes != nil && t.Notes.Plain != nil {
			n.Note = t.Notes.Plain.Content
		}
		if t.Image != nil && !strings.HasPrefix(t.Image.Src, "xap:") {
			n.Image = t.Image.Src
		}
		for _, m := range t.Markers {
			applyXMindMarker(n, m.MarkerID)
		}
		if t.Children != nil {
			for _, c := range t.Children.Attached {
				n.Children = append(n.Children, conv(c))
			}
		}
		return n
	}
	d := New("")
	d.Root = conv(sheets[0].RootTopic)
	for _, r := range sheets[0].Relationships {
		if from, to := idMap[r.End1], idMap[r.End2]; from != "" && to != "" {
			d.Links = append(d.Links, Link{ID: NewID(), From: from, To: to, Label: r.Title})
		}
	}
	return d, nil
}

var xmindMarkerEmoji = map[string]string{
	"flag-red": "🚩", "flag-orange": "🚩", "flag-yellow": "🚩", "flag-green": "🟢", "flag-blue": "🔵", "flag-purple": "🟣", "flag-gray": "🏳️",
	"star-red": "⭐", "star-orange": "⭐", "star-yellow": "⭐", "star-green": "⭐", "star-blue": "⭐", "star-purple": "⭐", "star-gray": "⭐",
	"symbol-exclam": "❗", "symbol-question": "❓", "symbol-info": "ℹ️", "symbol-wrong": "❌", "symbol-right": "✅", "symbol-plus": "➕", "symbol-minus": "➖",
	"symbol-attention": "⚠️", "symbol-pin": "📌", "symbol-bookmark": "🔖", "symbol-forbid": "⛔", "symbol-rightlight": "🟢",
	"smiley-smile": "🙂", "smiley-laugh": "😄", "smiley-angry": "😠", "smiley-cry": "😢", "smiley-surprise": "😮", "smiley-boring": "😐",
	"month-jan": "📅", "week-mon": "📅", "c_symbol_idea": "💡", "c_symbol_telephone": "📞", "c_symbol_mail": "✉️",
	"people-red": "👤", "people-orange": "👤", "people-yellow": "👤", "people-green": "👤", "people-blue": "👤", "people-purple": "👤", "people-gray": "👤",
	"arrow-up": "⬆️", "arrow-down": "⬇️", "arrow-left": "⬅️", "arrow-right": "➡️", "arrow-refresh": "🔄",
}

func applyXMindMarker(n *Node, id string) {
	if p, ok := strings.CutPrefix(id, "priority-"); ok {
		if v, err := strconv.Atoi(p); err == nil && v >= 1 && v <= 6 {
			n.Priority = v
		}
		return
	}
	switch id {
	case "task-start":
		v := 0
		n.Progress = &v
	case "task-oct":
		v := 12
		n.Progress = &v
	case "task-quarter":
		v := 25
		n.Progress = &v
	case "task-3oct":
		v := 37
		n.Progress = &v
	case "task-half":
		v := 50
		n.Progress = &v
	case "task-5oct":
		v := 62
		n.Progress = &v
	case "task-3quar":
		v := 75
		n.Progress = &v
	case "task-7oct":
		v := 87
		n.Progress = &v
	case "task-done":
		v := 100
		n.Progress = &v
	default:
		if e, ok := xmindMarkerEmoji[id]; ok && n.Icon == "" {
			n.Icon = e
		}
	}
}

// XMind 8 (content.xml) — titles, structure, notes, links, folding only.
type xm8Content struct {
	XMLName xml.Name `xml:"xmap-content"`
	Sheets  []struct {
		Topic *xm8Topic `xml:"topic"`
	} `xml:"sheet"`
}
type xm8Topic struct {
	ID       string `xml:"id,attr"`
	Href     string `xml:"href,attr"`
	Branch   string `xml:"branch,attr"`
	Title    string `xml:"title"`
	Note     string `xml:"notes>plain"`
	Children []struct {
		Type   string      `xml:"type,attr"`
		Topics []*xm8Topic `xml:"topic"`
	} `xml:"children>topics"`
}

func parseXMindXML(data []byte, opt ImportOptions) (*Doc, error) {
	var c xm8Content
	if err := xml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("bad xmind content.xml: %w", err)
	}
	if len(c.Sheets) == 0 || c.Sheets[0].Topic == nil {
		return nil, errors.New("xmind content has no sheet")
	}
	var conv func(t *xm8Topic) *Node
	conv = func(t *xm8Topic) *Node {
		n := &Node{ID: NewID(), Text: t.Title, Note: strings.TrimSpace(t.Note), URL: t.Href, Collapsed: t.Branch == "folded"}
		for _, g := range t.Children {
			if g.Type != "" && g.Type != "attached" {
				continue
			}
			for _, k := range g.Topics {
				n.Children = append(n.Children, conv(k))
			}
		}
		return n
	}
	d := New("")
	d.Root = conv(c.Sheets[0].Topic)
	return d, nil
}

// XMind writes an XMind workbook (zip: content.json, metadata.json, manifest.json).
func (d *Doc) XMind() ([]byte, error) {
	var conv func(n *Node) *xmTopic
	conv = func(n *Node) *xmTopic {
		t := &xmTopic{ID: n.ID, Title: strings.ReplaceAll(n.Text, "\n", " "), Href: n.URL, Labels: n.Tags}
		if n.Collapsed {
			t.Branch = "folded"
		}
		if n.Note != "" {
			t.Notes = &xmNotes{Plain: &struct {
				Content string `json:"content"`
			}{n.Note}}
		}
		if n.Priority > 0 {
			t.Markers = append(t.Markers, xmMark{"priority-" + strconv.Itoa(n.Priority)})
		}
		if n.Progress != nil {
			t.Markers = append(t.Markers, xmMark{xmindProgressMarker(*n.Progress)})
		} else if n.Done != nil {
			if *n.Done {
				t.Markers = append(t.Markers, xmMark{"task-done"})
			} else {
				t.Markers = append(t.Markers, xmMark{"task-start"})
			}
		}
		if n.Icon != "" {
			for id, e := range xmindMarkerEmoji {
				if e == n.Icon {
					t.Markers = append(t.Markers, xmMark{id})
					break
				}
			}
		}
		if len(n.Children) > 0 {
			t.Children = &xmKids{Attached: []*xmTopic{}}
			for _, c := range n.Children {
				t.Children.Attached = append(t.Children.Attached, conv(c))
			}
		}
		return t
	}
	root := conv(d.Root)
	root.Class = "topic"
	root.Struct = map[string]string{
		"balanced": "org.xmind.ui.map.unbalanced", "right": "org.xmind.ui.logic.right", "left": "org.xmind.ui.logic.left",
		"down": "org.xmind.ui.org-chart.down", "up": "org.xmind.ui.org-chart.up", "tree": "org.xmind.ui.tree.right",
	}[d.Layout]
	sheet := xmSheet{ID: NewID(), Class: "sheet", Title: firstLine(d.Root.Text), RootTopic: root}
	for _, l := range d.Links {
		sheet.Relationships = append(sheet.Relationships, xmRel{ID: l.ID, End1: l.From, End2: l.To, Title: l.Label})
	}
	content, _ := json.Marshal([]xmSheet{sheet})
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	files := map[string]string{
		"content.json":  string(content),
		"metadata.json": `{"creator":{"name":"Madok.Works","version":"1.0"}}`,
		"manifest.json": `{"file-entries":{"content.json":{},"metadata.json":{}}}`,
	}
	for _, name := range []string{"content.json", "metadata.json", "manifest.json"} {
		w, err := zw.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write([]byte(files[name])); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func xmindProgressMarker(p int) string {
	switch {
	case p >= 100:
		return "task-done"
	case p >= 75:
		return "task-3quar"
	case p >= 50:
		return "task-half"
	case p >= 25:
		return "task-quarter"
	}
	return "task-start"
}

// === Mermaid ===

var mermaidShapeRe = regexp.MustCompile(`^(?:[^()\[\]{}]*?)(?:\(\((.*)\)\)|\)\)(.*)\(\(|\)(.*)\(|\{\{(.*)\}\}|\[(.*)\]|\((.*)\))$`)

// ParseMermaid reads a Mermaid `mindmap` block (indentation defines nesting).
func ParseMermaid(src string, opt ImportOptions) (*Doc, error) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")
	type item struct {
		indent int
		node   *Node
	}
	var stack []item
	var root *Node
	started := false
	for _, raw := range lines {
		t := strings.TrimSpace(raw)
		if t == "" || strings.HasPrefix(t, "%%") {
			continue
		}
		if !started {
			if t == "mindmap" || strings.HasPrefix(t, "mindmap ") {
				started = true
				continue
			}
			if strings.HasPrefix(t, "```") {
				continue
			}
			started = true // tolerate a missing header
		}
		if strings.HasPrefix(t, "```") {
			break
		}
		if strings.HasPrefix(t, "::icon(") || strings.HasPrefix(t, ":::") {
			if len(stack) > 0 && strings.HasPrefix(t, ":::") {
				stack[len(stack)-1].node.Tags = append(stack[len(stack)-1].node.Tags, strings.TrimPrefix(t, ":::"))
			}
			continue
		}
		indent := indentWidth(raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))])
		text := t
		if m := mermaidShapeRe.FindStringSubmatch(t); m != nil {
			for _, g := range m[1:] {
				if g != "" {
					text = g
					break
				}
			}
			if text == t { // shape matched but inner text empty
				text = ""
			}
		}
		text = strings.Trim(text, "\"`")
		n := &Node{ID: NewID(), Text: text}
		for len(stack) > 0 && stack[len(stack)-1].indent >= indent {
			stack = stack[:len(stack)-1]
		}
		if len(stack) == 0 {
			if root == nil {
				root = n
			} else {
				root.Children = append(root.Children, n) // several top-level lines: siblings under the first
				stack = append(stack, item{-1, root})
			}
		} else {
			p := stack[len(stack)-1].node
			p.Children = append(p.Children, n)
		}
		stack = append(stack, item{indent, n})
	}
	if root == nil {
		return nil, errors.New("mermaid mindmap is empty")
	}
	d := New("")
	d.Root = root
	return d, nil
}

// Mermaid renders the map as a Mermaid `mindmap` block (text only — notes,
// links and markers are not representable; brackets are stripped).
func (d *Doc) Mermaid() string {
	var b strings.Builder
	b.WriteString("mindmap\n")
	clean := func(s string) string {
		s = strings.ReplaceAll(s, "\n", " ")
		s = strings.NewReplacer("(", "", ")", "", "[", "", "]", "", "{", "", "}", "", "\"", "'").Replace(s)
		s = strings.TrimSpace(s)
		if s == "" {
			s = "…"
		}
		return s
	}
	d.Walk(func(n, p *Node, depth int) bool {
		ind := strings.Repeat("  ", depth+1)
		if p == nil {
			b.WriteString(ind + "root((" + clean(n.Text) + "))\n")
			return true
		}
		txt := clean(n.Text)
		if n.Icon != "" {
			txt = n.Icon + " " + txt
		}
		b.WriteString(ind + txt + "\n")
		return true
	})
	return b.String()
}

func firstNonEmpty(a, b string) string {
	if strings.TrimSpace(a) != "" {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
