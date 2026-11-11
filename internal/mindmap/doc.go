// Package mindmap holds the data model of a wiki mind-map page plus the pure
// transformations around it: validation/normalization, the derived Markdown and
// HTML outlines that feed search, history diffs and export, node-level edit
// operations used by the REST API, and import/export of the formats other
// mind-map tools speak (Markdown outline, OPML, FreeMind/Freeplane, XMind,
// Mermaid, plain text).
//
// A mind map is a tree of nodes rooted at Doc.Root plus optional cross-link
// "relationships" between arbitrary nodes. The JSON document is the canonical
// stored form (wiki_pages.data); everything else is derived from it.
package mindmap

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Limits keep a single map editable in the browser and bounded on disk.
const (
	Version     = 1
	MaxNodes    = 5000
	MaxDepth    = 64
	MaxTextLen  = 1000
	MaxNoteLen  = 20000
	MaxURLLen   = 2000
	MaxTags     = 20
	MaxTagLen   = 40
	MaxLinks    = 500
	MaxLabelLen = 200
	MaxDocBytes = 2 * 1024 * 1024
)

// Layouts and styles the renderer understands. The first entry is the default.
var (
	Layouts  = []string{"balanced", "right", "left", "down", "up", "tree"}
	Lines    = []string{"curve", "elbow", "straight"}
	Palettes = []string{"rainbow", "mono", "warm", "cool", "pastel"}
	Shapes   = []string{"", "rect", "pill", "underline"}
)

// Node is one topic of the map. Only id/text are required; every other field
// is an optional decoration or marker (omitted from JSON when empty).
type Node struct {
	ID        string   `json:"id"`
	Text      string   `json:"text"`
	Note      string   `json:"note,omitempty"`     // markdown
	URL       string   `json:"url,omitempty"`      // external hyperlink
	Icon      string   `json:"icon,omitempty"`     // emoji / short glyph
	Color     string   `json:"color,omitempty"`    // #rrggbb branch colour override
	Shape     string   `json:"shape,omitempty"`    // "", rect, pill, underline
	Image     string   `json:"image,omitempty"`    // image URL (uploaded file)
	Tags      []string `json:"tags,omitempty"`     // labels
	Priority  int      `json:"priority,omitempty"` // 0 = none, 1..6
	Progress  *int     `json:"progress,omitempty"` // 0..100, nil = no marker
	Done      *bool    `json:"done,omitempty"`     // nil = not a task; else checkbox state
	Collapsed bool     `json:"collapsed,omitempty"`
	Boundary  bool     `json:"boundary,omitempty"` // shaded area around the subtree
	Side      string   `json:"side,omitempty"`     // left|right (root children, balanced layout)
	PageID    int64    `json:"page_id,omitempty"`  // link to a wiki page
	TaskRef   string   `json:"task_ref,omitempty"` // KEY-123 task reference
	Children  []*Node  `json:"children"`
}

// Link is a relationship line between two nodes anywhere in the tree.
type Link struct {
	ID    string `json:"id"`
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label,omitempty"`
}

// Doc is the whole map.
type Doc struct {
	Version int    `json:"version"`
	Root    *Node  `json:"root"`
	Layout  string `json:"layout"`
	Line    string `json:"line"`
	Palette string `json:"palette"`
	Links   []Link `json:"links"`
}

// New returns a map with a single root node titled text.
func New(text string) *Doc {
	return &Doc{Version: Version, Root: &Node{ID: NewID(), Text: text, Children: []*Node{}},
		Layout: Layouts[0], Line: Lines[0], Palette: Palettes[0], Links: []Link{}}
}

const idAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// NewID returns a short random node id (8 chars, base36).
func NewID() string {
	b := make([]byte, 8)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(idAlphabet))))
		if err != nil {
			n = big.NewInt(int64(i))
		}
		b[i] = idAlphabet[n.Int64()]
	}
	return string(b)
}

var (
	idRe    = regexp.MustCompile(`^[A-Za-z0-9_-]{1,40}$`)
	colorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
	taskRe  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*-\d+$`)
)

// Parse decodes and normalizes a JSON document. It is lenient about missing
// optional fields (a bare {"root":{"text":"x"}} is valid) and strict about the
// limits above.
func Parse(data []byte) (*Doc, error) {
	if len(data) > MaxDocBytes {
		return nil, errors.New("mind map too large")
	}
	var d Doc
	if err := json.Unmarshal(data, &d); err != nil {
		return nil, fmt.Errorf("bad mind map json: %w", err)
	}
	if err := d.Normalize(); err != nil {
		return nil, err
	}
	return &d, nil
}

// Marshal encodes the document compactly.
func (d *Doc) Marshal() []byte {
	b, _ := json.Marshal(d)
	return b
}

// Normalize validates the tree, fills defaults (ids, layout, enums), trims and
// clamps every field, and drops links that point at unknown nodes. Returns an
// error only for structural problems (no root, too many nodes, too deep, a
// cycle via a repeated node pointer).
func (d *Doc) Normalize() error {
	if d.Root == nil {
		return errors.New("mind map has no root")
	}
	d.Version = Version
	d.Layout = pick(d.Layout, Layouts)
	d.Line = pick(d.Line, Lines)
	d.Palette = pick(d.Palette, Palettes)
	seen := map[string]bool{}
	count := 0
	var walk func(n *Node, depth int, isRoot bool) error
	walk = func(n *Node, depth int, isRoot bool) error {
		if n == nil {
			return errors.New("null node")
		}
		if depth > MaxDepth {
			return errors.New("mind map too deep")
		}
		count++
		if count > MaxNodes {
			return fmt.Errorf("too many nodes (max %d)", MaxNodes)
		}
		n.ID = strings.TrimSpace(n.ID)
		if !idRe.MatchString(n.ID) || seen[n.ID] {
			n.ID = NewID()
			for seen[n.ID] {
				n.ID = NewID()
			}
		}
		seen[n.ID] = true
		n.normalizeFields(isRoot)
		if n.Children == nil {
			n.Children = []*Node{}
		}
		kept := n.Children[:0]
		for _, c := range n.Children {
			if c == nil {
				continue
			}
			if err := walk(c, depth+1, false); err != nil {
				return err
			}
			kept = append(kept, c)
		}
		n.Children = kept
		return nil
	}
	if err := walk(d.Root, 0, true); err != nil {
		return err
	}
	// Links: keep only well-formed ones between existing, distinct nodes.
	links := d.Links[:0]
	linkIDs := map[string]bool{}
	for _, l := range d.Links {
		if len(links) >= MaxLinks {
			break
		}
		if !seen[l.From] || !seen[l.To] || l.From == l.To {
			continue
		}
		l.ID = strings.TrimSpace(l.ID)
		if !idRe.MatchString(l.ID) || linkIDs[l.ID] {
			l.ID = NewID()
		}
		linkIDs[l.ID] = true
		l.Label = clip(strings.TrimSpace(l.Label), MaxLabelLen)
		links = append(links, l)
	}
	if links == nil {
		links = []Link{}
	}
	d.Links = links
	return nil
}

func (n *Node) normalizeFields(isRoot bool) {
	n.Text = clip(strings.TrimSpace(strings.ReplaceAll(n.Text, "\r\n", "\n")), MaxTextLen)
	if n.Text == "" && isRoot {
		n.Text = "…"
	}
	n.Note = clip(strings.TrimSpace(strings.ReplaceAll(n.Note, "\r\n", "\n")), MaxNoteLen)
	n.URL = SafeURL(clip(strings.TrimSpace(n.URL), MaxURLLen))
	n.Image = SafeURL(clip(strings.TrimSpace(n.Image), MaxURLLen))
	n.Icon = clip(strings.TrimSpace(n.Icon), 16)
	if !colorRe.MatchString(n.Color) {
		n.Color = ""
	}
	n.Shape = pick(n.Shape, Shapes)
	if n.Priority < 0 || n.Priority > 6 {
		n.Priority = 0
	}
	if n.Progress != nil {
		v := *n.Progress
		if v < 0 {
			v = 0
		}
		if v > 100 {
			v = 100
		}
		n.Progress = &v
	}
	if n.Side != "left" && n.Side != "right" {
		n.Side = ""
	}
	if isRoot {
		n.Side = ""
		n.Collapsed = false
	}
	if n.PageID < 0 {
		n.PageID = 0
	}
	n.TaskRef = strings.ToUpper(strings.TrimSpace(n.TaskRef))
	if !taskRe.MatchString(n.TaskRef) {
		n.TaskRef = ""
	}
	if len(n.Tags) > 0 {
		tags := n.Tags[:0]
		dup := map[string]bool{}
		for _, t := range n.Tags {
			t = clip(strings.TrimSpace(t), MaxTagLen)
			if t == "" || dup[strings.ToLower(t)] {
				continue
			}
			dup[strings.ToLower(t)] = true
			tags = append(tags, t)
			if len(tags) >= MaxTags {
				break
			}
		}
		n.Tags = tags
	}
}

// SafeURL keeps http(s)/mailto and site-relative URLs; anything else (javascript:,
// data:, vbscript:…) is dropped.
func SafeURL(u string) string {
	if u == "" {
		return ""
	}
	low := strings.ToLower(u)
	switch {
	case strings.HasPrefix(low, "http://"), strings.HasPrefix(low, "https://"), strings.HasPrefix(low, "mailto:"):
		return u
	case strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//"):
		return u
	}
	return ""
}

func pick(v string, allowed []string) string {
	for _, a := range allowed {
		if v == a {
			return v
		}
	}
	return allowed[0]
}

// clip truncates s to at most n runes.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

// === Tree helpers ===

// Walk calls fn for every node in pre-order with its parent (nil for root) and
// depth. Returning false from fn stops the walk.
func (d *Doc) Walk(fn func(n, parent *Node, depth int) bool) {
	var rec func(n, p *Node, depth int) bool
	rec = func(n, p *Node, depth int) bool {
		if !fn(n, p, depth) {
			return false
		}
		for _, c := range n.Children {
			if !rec(c, n, depth+1) {
				return false
			}
		}
		return true
	}
	rec(d.Root, nil, 0)
}

// Find returns the node with the id and its parent (nil parent for root).
func (d *Doc) Find(id string) (node, parent *Node) {
	d.Walk(func(n, p *Node, _ int) bool {
		if n.ID == id {
			node, parent = n, p
			return false
		}
		return true
	})
	return
}

// Count returns the number of nodes in the map.
func (d *Doc) Count() int {
	n := 0
	d.Walk(func(*Node, *Node, int) bool { n++; return true })
	return n
}

// IsDescendant reports whether id lies inside the subtree of ancestorID
// (a node is considered its own descendant).
func (d *Doc) IsDescendant(id, ancestorID string) bool {
	anc, _ := d.Find(ancestorID)
	if anc == nil {
		return false
	}
	found := false
	var rec func(n *Node)
	rec = func(n *Node) {
		if n.ID == id {
			found = true
		}
		for _, c := range n.Children {
			if found {
				return
			}
			rec(c)
		}
	}
	rec(anc)
	return found
}

// FlatNode is the list form of a node used by the nodes API: no children, but
// the parent id, depth and sibling index so scripts can rebuild the tree.
type FlatNode struct {
	Node
	ParentID string `json:"parent_id"`
	Depth    int    `json:"depth"`
	Index    int    `json:"index"`
	Count    int    `json:"child_count"`
}

// Flatten returns every node in pre-order as a FlatNode.
func (d *Doc) Flatten() []FlatNode {
	out := make([]FlatNode, 0, 64)
	var rec func(n *Node, parentID string, depth, idx int)
	rec = func(n *Node, parentID string, depth, idx int) {
		f := FlatNode{Node: *n, ParentID: parentID, Depth: depth, Index: idx, Count: len(n.Children)}
		f.Children = nil
		out = append(out, f)
		for i, c := range n.Children {
			rec(c, n.ID, depth+1, i)
		}
	}
	rec(d.Root, "", 0, 0)
	return out
}

// Clone returns a deep copy of the document.
func (d *Doc) Clone() *Doc {
	c, _ := Parse(d.Marshal())
	return c
}

// CloneNode deep-copies a subtree with fresh ids (for copy/paste and
// duplication), so the copy never collides with the original.
func CloneNode(n *Node) *Node {
	b, _ := json.Marshal(n)
	var c Node
	json.Unmarshal(b, &c)
	var reid func(x *Node)
	reid = func(x *Node) {
		x.ID = NewID()
		if x.Children == nil {
			x.Children = []*Node{}
		}
		for _, k := range x.Children {
			reid(k)
		}
	}
	reid(&c)
	return &c
}
