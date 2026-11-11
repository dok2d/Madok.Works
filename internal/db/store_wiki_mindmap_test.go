package db

import (
	"testing"

	"kanban/internal/model"
)

// TestWikiMindmapRevisions covers kind/data storage, the autosave squash and
// that squashing never hides a commented revision or another author's save.
func TestWikiMindmapRevisions(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")
	sp, _ := s.CreateWikiSpace("DOC", "Docs", "", "open", nil, alice)
	id, err := s.CreateWikiPageFull(sp, nil, "Map", "<h1>Map</h1>", "# Map", model.WikiKindMindmap, `{"root":{"text":"Map"}}`, alice)
	if err != nil {
		t.Fatalf("CreateWikiPageFull: %v", err)
	}
	p, _ := s.GetWikiPage(id)
	if p.Kind != model.WikiKindMindmap || p.Data != `{"root":{"text":"Map"}}` {
		t.Fatalf("page kind/data: %q %q", p.Kind, p.Data)
	}
	tree, _ := s.ListWikiPageTree(sp)
	if tree[0].Kind != model.WikiKindMindmap {
		t.Fatalf("tree kind: %q", tree[0].Kind)
	}

	// Three autosaves by alice collapse into one revision (rev_no keeps growing).
	rev := 1
	for i := 0; i < 3; i++ {
		var err error
		rev, err = s.UpdateWikiPageFull(id, "Map", "", "", `{"i":`+string(rune('0'+i))+`}`, "", alice, rev, true)
		if err != nil {
			t.Fatalf("autosave %d: %v", i, err)
		}
	}
	if rev != 4 {
		t.Fatalf("rev after autosaves = %d, want 4", rev)
	}
	revs, _ := s.ListWikiRevisions(id)
	if len(revs) != 2 || revs[0].RevNo != 4 || revs[1].RevNo != 1 {
		t.Fatalf("squashed revisions: %+v", revs)
	}
	// A commented save is never squashed away by the next autosave.
	rev, _ = s.UpdateWikiPageFull(id, "Map", "", "", `{"named":1}`, "milestone", alice, rev, true)
	rev, _ = s.UpdateWikiPageFull(id, "Map", "", "", `{"after":1}`, "", alice, rev, true)
	revs, _ = s.ListWikiRevisions(id)
	if len(revs) != 4 || revs[1].Comment != "milestone" {
		t.Fatalf("commented revision squashed: %+v", revs)
	}
	// Another author's autosave keeps alice's last revision.
	rev, _ = s.UpdateWikiPageFull(id, "Map", "", "", `{"bob":1}`, "", bob, rev, true)
	revs, _ = s.ListWikiRevisions(id)
	if len(revs) != 5 || revs[0].AuthorID == nil || *revs[0].AuthorID != bob || *revs[1].AuthorID != alice {
		t.Fatalf("cross-author squash: %+v", revs)
	}
	// Conflict detection still works with squash on.
	if _, err := s.UpdateWikiPageFull(id, "Map", "", "", `{}`, "", alice, rev-1, true); err == nil {
		t.Fatal("expected conflict")
	}
	// The revision carries kind and data for restore.
	r, _ := s.GetWikiRevision(revs[0].ID)
	if r.Kind != model.WikiKindMindmap || r.Data != `{"bob":1}` {
		t.Fatalf("revision kind/data: %q %q", r.Kind, r.Data)
	}
	if s.WikiPageTitle(id) != "Map" || s.WikiPageIDByTitle(sp, "map") != id || s.WikiPageIDByTitle(sp, "DOC:Map") != id || s.WikiPageIDByTitle(sp, "nope") != 0 {
		t.Fatal("title helpers")
	}
}
