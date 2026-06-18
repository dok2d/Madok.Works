package db

import "testing"

// TestWikiLinks covers [[link]] resolution (same space, KEY:Title cross-space,
// unresolved) and the backlinks graph.
func TestWikiLinks(t *testing.T) {
	s := newTestStore(t)
	author := mustUser(t, s, "alice")

	docs, err := s.CreateWikiSpace("DOC", "Docs", "", "open", nil, author)
	if err != nil {
		t.Fatalf("CreateWikiSpace: %v", err)
	}
	api, _ := s.CreateWikiSpace("API", "Api", "", "open", nil, author)

	home, _ := s.CreateWikiPage(docs, nil, "Home", "", "", author)
	guide, _ := s.CreateWikiPage(docs, nil, "Guide", "", "", author)
	ref, _ := s.CreateWikiPage(api, nil, "Ref", "", "", author)

	md := "See [[Guide]] and [[guide]], cross [[API:Ref]], also [[DOC:Guide]], missing [[Nope]], marker [[subpages]]."
	links, ids := s.ResolveWikiLinks(docs, md)
	if links["Guide"] != guide {
		t.Errorf("[[Guide]] -> %d, want %d", links["Guide"], guide)
	}
	if links["API:Ref"] != ref {
		t.Errorf("[[API:Ref]] -> %d, want %d", links["API:Ref"], ref)
	}
	if links["DOC:Guide"] != guide {
		t.Errorf("[[DOC:Guide]] -> %d, want %d", links["DOC:Guide"], guide)
	}
	if v, ok := links["Nope"]; !ok || v != 0 {
		t.Errorf("[[Nope]] should resolve to 0, got %d (present=%v)", v, ok)
	}
	if _, ok := links["subpages"]; ok {
		t.Error("[[subpages]] marker must be ignored")
	}

	if err := s.SetWikiLinks(home, ids); err != nil {
		t.Fatalf("SetWikiLinks: %v", err)
	}
	bl, err := s.WikiBacklinks(guide)
	if err != nil {
		t.Fatalf("WikiBacklinks: %v", err)
	}
	if len(bl) != 1 || bl[0].ID != home {
		t.Fatalf("backlinks of Guide = %+v, want [Home(%d)]", bl, home)
	}
	if bl[0].SpaceKey != "DOC" || bl[0].Title != "Home" {
		t.Errorf("backlink ref wrong: %+v", bl[0])
	}

	// Re-saving with no links clears the graph.
	if err := s.SetWikiLinks(home, nil); err != nil {
		t.Fatalf("SetWikiLinks clear: %v", err)
	}
	if bl, _ := s.WikiBacklinks(guide); len(bl) != 0 {
		t.Errorf("backlinks should be empty after clear, got %d", len(bl))
	}
}
