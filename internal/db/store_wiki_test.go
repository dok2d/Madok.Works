package db

import "testing"

// TestWikiCommentsReactions covers threaded comments, cascade delete and
// page/comment reactions toggle + aggregation.
func TestWikiCommentsReactions(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")
	sp, _ := s.CreateWikiSpace("DOC", "Docs", "", "open", nil, alice)
	page, _ := s.CreateWikiPage(sp, nil, "Home", "", "", alice)

	c1, err := s.AddWikiComment(page, nil, "hello", alice)
	if err != nil {
		t.Fatalf("AddWikiComment: %v", err)
	}
	if _, err := s.AddWikiComment(page, &c1, "re: hello", bob); err != nil {
		t.Fatalf("AddWikiComment reply: %v", err)
	}
	cs, _ := s.ListWikiComments(page, alice)
	if len(cs) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(cs))
	}

	// Page reaction toggles on, then off.
	if err := s.ToggleWikiReaction("page", page, alice, "👍"); err != nil {
		t.Fatalf("ToggleWikiReaction: %v", err)
	}
	r, _ := s.WikiReactions("page", page, alice)
	if len(r) != 1 || r[0].Count != 1 || !r[0].Mine {
		t.Fatalf("page reaction = %+v, want count 1 mine", r)
	}
	s.ToggleWikiReaction("page", page, alice, "👍")
	if r, _ := s.WikiReactions("page", page, alice); len(r) != 0 {
		t.Errorf("page reaction should be cleared, got %+v", r)
	}

	// Comment reaction from two users aggregates to count 2.
	s.ToggleWikiReaction("comment", c1, alice, "🎉")
	s.ToggleWikiReaction("comment", c1, bob, "🎉")
	if r, _ := s.WikiReactions("comment", c1, bob); len(r) != 1 || r[0].Count != 2 || !r[0].Mine {
		t.Errorf("comment reaction = %+v, want count 2 mine", r)
	}

	// Deleting the parent comment cascades to its reply.
	if err := s.DeleteWikiComment(c1); err != nil {
		t.Fatalf("DeleteWikiComment: %v", err)
	}
	if cs, _ := s.ListWikiComments(page, alice); len(cs) != 0 {
		t.Errorf("expected 0 comments after cascade delete, got %d", len(cs))
	}
}

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
