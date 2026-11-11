package mindmap

import (
	"encoding/json"
	"strings"
	"testing"
)

func sample() *Doc {
	d := New("Project")
	be := &Node{ID: "be", Text: "Backend", Side: "right", Icon: "⚙️", Priority: 1, Tags: []string{"go"}}
	api := &Node{ID: "api", Text: "REST API", URL: "https://example.org/api", Note: "line one\nline two"}
	db := &Node{ID: "db", Text: "SQLite"}
	be.Children = []*Node{api, db}
	fe := &Node{ID: "fe", Text: "Frontend", Side: "left", Collapsed: true}
	done := true
	p := 50
	fe.Children = []*Node{{ID: "ed", Text: "Editor", Done: &done, Progress: &p, TaskRef: "GEN-12"}}
	d.Root.Children = []*Node{be, fe}
	d.Links = []Link{{ID: "l1", From: "api", To: "fe", Label: "uses"}}
	d.Normalize()
	return d
}

func TestNormalize(t *testing.T) {
	raw := `{"root":{"text":"  Root ","children":[{"text":"a","id":"dup","color":"red","priority":9,"progress":150,"side":"up",
	  "url":"javascript:alert(1)","children":[{"text":"b","id":"dup"}]}]},"layout":"nope","links":[{"from":"dup","to":"zzz"}]}`
	d, err := Parse([]byte(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if d.Root.Text != "Root" || d.Layout != "balanced" || d.Line != "curve" || d.Version != Version {
		t.Fatalf("defaults not applied: %+v", d)
	}
	a := d.Root.Children[0]
	if a.Color != "" || a.Priority != 0 || *a.Progress != 100 || a.Side != "" || a.URL != "" {
		t.Fatalf("fields not clamped: %+v", a)
	}
	if a.ID == a.Children[0].ID || a.ID == "" {
		t.Fatalf("duplicate ids not fixed: %q %q", a.ID, a.Children[0].ID)
	}
	if len(d.Links) != 0 {
		t.Fatalf("dangling link kept: %+v", d.Links)
	}
	if _, err := Parse([]byte(`{"layout":"right"}`)); err == nil {
		t.Fatal("expected error for missing root")
	}
	// Cycle via a shared pointer is impossible from JSON, but depth is bounded.
	deep := "{\"root\":{\"text\":\"r\",\"children\":[" + strings.Repeat(`{"text":"x","children":[`, 70) + strings.Repeat("]}", 70) + "]}}"
	if _, err := Parse([]byte(deep)); err == nil {
		t.Fatal("expected too-deep error")
	}
}

func TestDeriveMarkdownHTML(t *testing.T) {
	d := sample()
	titles := func(id int64) string { return "" }
	md := d.Markdown(titles)
	for _, want := range []string{"# Project", "- ⚙️ Backend (P1) #go", "  - [REST API](https://example.org/api)", "    > line one", "    > line two", "- [x] Editor (50%) [[GEN-12]]", "REST API → Frontend (uses)"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
	h := d.HTML(titles)
	for _, want := range []string{"<h1>Project</h1>", `<a href="https://example.org/api">REST API</a>`, `<input type="checkbox" checked disabled>`, "<blockquote><p>line one<br>line two</p></blockquote>", "[[GEN-12]]"} {
		if !strings.Contains(h, want) {
			t.Errorf("html missing %q:\n%s", want, h)
		}
	}
	// Markdown → map round trip keeps structure and markers.
	back, err := ParseMarkdown(md, ImportOptions{})
	if err != nil {
		t.Fatalf("ParseMarkdown: %v", err)
	}
	if back.Root.Text != "Project" || len(back.Root.Children) != 2 {
		t.Fatalf("round trip root: %+v", back.Root)
	}
	if len(back.Links) != 1 || back.Links[0].Label != "uses" {
		t.Errorf("round trip links: %+v", back.Links)
	}
	be := back.Root.Children[0]
	if be.Text != "Backend" || be.Icon != "⚙️" || be.Priority != 1 || len(be.Tags) != 1 || be.Tags[0] != "go" {
		t.Errorf("round trip backend: %+v", be)
	}
	api := be.Children[0]
	if api.Text != "REST API" || api.URL != "https://example.org/api" || api.Note != "line one\nline two" {
		t.Errorf("round trip api: %+v", api)
	}
	ed := back.Root.Children[1].Children[0]
	if ed.Done == nil || !*ed.Done || ed.Progress == nil || *ed.Progress != 50 || ed.TaskRef != "GEN-12" {
		t.Errorf("round trip editor: %+v", ed)
	}
}

func TestParseMarkdownHeadingsAndIndent(t *testing.T) {
	src := "---\nmarkmap:\n  maxWidth: 300\n---\n# Root\n## A <!-- markmap: fold -->\n- a1\n    - a2\n        - a3\n- a4\n## B\nnote for B\n- [ ] todo\n"
	d, err := ParseMarkdown(src, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if d.Root.Text != "Root" || len(d.Root.Children) != 2 {
		t.Fatalf("root: %+v", d.Root)
	}
	a := d.Root.Children[0]
	if a.Text != "A" || !a.Collapsed || len(a.Children) != 2 || a.Children[0].Children[0].Children[0].Text != "a3" {
		t.Fatalf("A subtree: %s", string(a.marshal()))
	}
	b := d.Root.Children[1]
	if b.Note != "note for B" || b.Children[0].Done == nil || *b.Children[0].Done {
		t.Fatalf("B: %+v", b)
	}
	// Several top-level items → virtual root with the given title.
	d2, _ := ParseMarkdown("- x\n- y\n", ImportOptions{Title: "T"})
	if d2.Root.Text != "T" || len(d2.Root.Children) != 2 {
		t.Fatalf("virtual root: %+v", d2.Root)
	}
	d3, _ := ParseText("a\n\tb\n\t\tc\nd\n", ImportOptions{Title: "T"})
	if len(d3.Root.Children) != 2 || d3.Root.Children[0].Children[0].Children[0].Text != "c" {
		t.Fatalf("text outline: %s", string(d3.Root.marshal()))
	}
}

func (n *Node) marshal() []byte { b, _ := json.Marshal(n); return b }

func TestOps(t *testing.T) {
	d := sample()
	idx := 0
	res, err := d.Apply([]Op{
		{Op: "add", ParentID: "be", Node: &Node{Text: "Auth", Children: []*Node{{Text: "TOTP"}}}, Index: &idx},
		{Op: "update", ID: "db", Set: json.RawMessage(`{"text":"PostgreSQL","note":"optional","progress":25,"done":null,"id":"hack"}`)},
		{Op: "move", ID: "ed", ParentID: "be", Index: &idx},
		{Op: "link_add", Link: &Link{From: "db", To: "ed", Label: "stores"}},
		{Op: "settings", Layout: "right", Line: "elbow"},
	})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if len(res) != 5 || res[0].ID == "" {
		t.Fatalf("results: %+v", res)
	}
	be, _ := d.Find("be")
	// Auth was inserted at index 0, then "ed" was moved in front of it.
	if be.Children[1].Text != "Auth" || be.Children[1].Children[0].Text != "TOTP" || be.Children[1].ID != res[0].ID {
		t.Errorf("add with index 0 failed: %s (res %+v)", string(be.marshal()), res)
	}
	db, _ := d.Find("db")
	if db.Text != "PostgreSQL" || db.Note != "optional" || db.Progress == nil || *db.Progress != 25 || db.ID != "db" {
		t.Errorf("update: %+v", db)
	}
	ed, parent := d.Find("ed")
	if parent == nil || parent.ID != "be" || be.Children[0].ID != "ed" {
		t.Errorf("move: parent=%v children=%s", parent, string(be.marshal()))
	}
	_ = ed
	if len(d.Links) != 2 || d.Layout != "right" || d.Line != "elbow" {
		t.Errorf("links/settings: %+v %s %s", d.Links, d.Layout, d.Line)
	}
	// Failing op leaves the document untouched.
	before := string(d.Marshal())
	if _, err := d.Apply([]Op{{Op: "add", ParentID: "be", Node: &Node{Text: "ok"}}, {Op: "delete", ID: d.Root.ID}}); err == nil {
		t.Fatal("expected error deleting root")
	}
	if string(d.Marshal()) != before {
		t.Fatal("document changed after failed batch")
	}
	if _, err := d.Apply([]Op{{Op: "move", ID: "be", ParentID: "api"}}); err == nil {
		t.Fatal("expected cycle error")
	}
	// Delete drops links into the subtree.
	if _, err := d.Apply([]Op{{Op: "delete", ID: "be"}}); err != nil {
		t.Fatal(err)
	}
	if n, _ := d.Find("api"); n != nil {
		t.Error("subtree not deleted")
	}
	if len(d.Links) != 0 {
		t.Errorf("links into deleted subtree kept: %+v", d.Links)
	}
}

func TestFlatten(t *testing.T) {
	d := sample()
	flat := d.Flatten()
	if len(flat) != 6 || flat[0].ParentID != "" || flat[1].ID != "be" || flat[1].ParentID != d.Root.ID || flat[1].Count != 2 || flat[2].Depth != 2 {
		t.Fatalf("flatten: %+v", flat)
	}
	if flat[1].Children != nil {
		t.Error("flat nodes must not carry children")
	}
}

func TestOPMLRoundTrip(t *testing.T) {
	d := sample()
	out := d.OPML()
	if !strings.Contains(out, `<outline text="REST API" _note="line one&#xA;line two" url="https://example.org/api">`) {
		t.Fatalf("opml:\n%s", out)
	}
	back, err := ParseOPML([]byte(out), ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if back.Root.Text != "Project" || len(back.Root.Children) != 2 || back.Root.Children[0].Children[0].Note != "line one\nline two" {
		t.Fatalf("opml round trip: %s", string(back.Root.marshal()))
	}
	ed := back.Root.Children[1].Children[0]
	if ed.Done == nil || !*ed.Done {
		t.Errorf("opml _status lost: %+v", ed)
	}
}

func TestFreeMindRoundTrip(t *testing.T) {
	d := sample()
	out := d.FreeMind()
	for _, want := range []string{`<map version="1.0.1">`, `TEXT="Backend"`, `POSITION="right"`, `FOLDED="true"`, `<icon BUILTIN="full-1">`, `<arrowlink DESTINATION="ID_fe">`, `<richcontent TYPE="NOTE">`} {
		if !strings.Contains(out, want) {
			t.Errorf("freemind missing %q:\n%s", want, out)
		}
	}
	back, err := ParseFreeMind([]byte(out), ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	be := back.Root.Children[0]
	if be.Text != "Backend" || be.Side != "right" || be.Priority != 1 || be.Children[0].Note != "line one\nline two" || be.Children[0].URL != "https://example.org/api" {
		t.Errorf("freemind round trip: %s", string(be.marshal()))
	}
	if !back.Root.Children[1].Collapsed || len(back.Links) != 1 {
		t.Errorf("freemind fold/links: collapsed=%v links=%+v", back.Root.Children[1].Collapsed, back.Links)
	}
	// Freeplane-style file with richcontent node text and an icon.
	fp := `<map version="freeplane 1.9.0"><node ID="ID_1"><richcontent TYPE="NODE"><html><body><p>Rich <b>root</b></p></body></html></richcontent>
	<node ID="ID_2" TEXT="child"><icon BUILTIN="idea"/><cloud/></node></node></map>`
	fd, err := ParseFreeMind([]byte(fp), ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if fd.Root.Text != "Rich root" || fd.Root.Children[0].Icon != "💡" || !fd.Root.Children[0].Boundary {
		t.Errorf("freeplane parse: %s", string(fd.Root.marshal()))
	}
}

func TestXMindRoundTrip(t *testing.T) {
	d := sample()
	zipped, err := d.XMind()
	if err != nil {
		t.Fatal(err)
	}
	if DetectFormat("", zipped) != FormatXMind {
		t.Error("xmind zip not detected")
	}
	back, err := ParseXMind(zipped, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	be := back.Root.Children[0]
	if back.Root.Text != "Project" || be.Priority != 1 || be.Tags[0] != "go" || be.Children[0].Note != "line one\nline two" || be.Children[0].URL != "https://example.org/api" {
		t.Errorf("xmind round trip: %s", string(be.marshal()))
	}
	if !back.Root.Children[1].Collapsed || len(back.Links) != 1 || back.Links[0].Label != "uses" {
		t.Errorf("xmind fold/relationships: %+v", back.Links)
	}
	ed := back.Root.Children[1].Children[0]
	if ed.Progress == nil || *ed.Progress != 50 {
		t.Errorf("xmind progress marker: %+v", ed)
	}
	// A bare content.json array imports as JSON too.
	cj := `[{"rootTopic":{"id":"r","title":"R","children":{"attached":[{"id":"c","title":"C","markers":[{"markerId":"priority-2"}]}]}}}]`
	jd, err := Import(FormatJSON, "content.json", []byte(cj), ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if jd.Root.Text != "R" || jd.Root.Children[0].Priority != 2 {
		t.Errorf("content.json import: %s", string(jd.Root.marshal()))
	}
}

func TestMermaid(t *testing.T) {
	d := sample()
	out := d.Mermaid()
	if !strings.HasPrefix(out, "mindmap\n  root((Project))\n    ⚙️ Backend\n      REST API\n") {
		t.Fatalf("mermaid:\n%s", out)
	}
	src := "```mermaid\nmindmap\n  root((Origins))\n    Long history\n    ::icon(fa fa-book)\n    Popularisation\n      British author[Tony Buzan]\n      Research\n```"
	back, err := ParseMermaid(src, ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if back.Root.Text != "Origins" || len(back.Root.Children) != 2 || back.Root.Children[1].Children[0].Text != "Tony Buzan" {
		t.Fatalf("mermaid parse: %s", string(back.Root.marshal()))
	}
}

func TestDetectAndMindElixir(t *testing.T) {
	cases := map[string]string{
		"mindmap\n  root((x))": FormatMermaid, "<map version=\"1.0.1\"><node TEXT=\"a\"/></map>": FormatFreeMind,
		"<opml version=\"2.0\"></opml>": FormatOPML, "{\"root\":{}}": FormatJSON, "# T\n- a": FormatMarkdown, "a\n\tb": FormatText,
	}
	for src, want := range cases {
		if got := DetectFormat("", []byte(src)); got != want {
			t.Errorf("DetectFormat(%q) = %s, want %s", src, got, want)
		}
	}
	if DetectFormat("map.mm", nil) != FormatFreeMind || DetectFormat("x.opml", []byte("")) != FormatOPML {
		t.Error("extension detection")
	}
	me := `{"nodeData":{"id":"root","topic":"ME","expanded":true,"children":[{"id":"a","topic":"A","expanded":false,"hyperLink":"https://x.y","icons":["🚀"]}]}}`
	d, err := Import(FormatJSON, "", []byte(me), ImportOptions{})
	if err != nil {
		t.Fatal(err)
	}
	a := d.Root.Children[0]
	if d.Root.Text != "ME" || a.Text != "A" || !a.Collapsed || a.URL != "https://x.y" || a.Icon != "🚀" {
		t.Errorf("mind-elixir import: %s", string(d.Root.marshal()))
	}
}

func TestCloneNodeFreshIDs(t *testing.T) {
	d := sample()
	be, _ := d.Find("be")
	c := CloneNode(be)
	if c.ID == be.ID || c.Children[0].ID == be.Children[0].ID || c.Children[0].Text != "REST API" {
		t.Errorf("clone ids/text: %s", string(c.marshal()))
	}
}
