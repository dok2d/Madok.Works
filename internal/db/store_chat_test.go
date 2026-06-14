package db

import (
	"fmt"
	"path/filepath"
	"testing"

	"kanban/internal/model"
)

// newTestStore opens a fresh migrated SQLite database in a temp dir.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func mustUser(t *testing.T, s *Store, name string) int64 {
	t.Helper()
	id, err := s.CreateUser(name, "password123", "user")
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", name, err)
	}
	return id
}

func TestCreateChatChannel(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")

	// invalid type rejected
	if _, err := s.CreateChatChannel("x", "", "dm", alice, nil); err == nil {
		t.Error("expected error for type 'dm' via CreateChatChannel")
	}
	// empty name rejected
	if _, err := s.CreateChatChannel("  ", "", "channel", alice, nil); err == nil {
		t.Error("expected error for empty name")
	}

	id, err := s.CreateChatChannel("general", "topic", "channel", alice, []int64{bob})
	if err != nil {
		t.Fatalf("CreateChatChannel: %v", err)
	}
	// creator and listed member are both members
	for _, uid := range []int64{alice, bob} {
		ok, err := s.IsChatMember(id, uid)
		if err != nil || !ok {
			t.Errorf("user %d should be a member (ok=%v err=%v)", uid, ok, err)
		}
	}
}

func TestGetOrCreateDM(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")

	if _, err := s.GetOrCreateDM(alice, alice); err == nil {
		t.Error("expected error for DM with self")
	}

	id1, err := s.GetOrCreateDM(alice, bob)
	if err != nil {
		t.Fatalf("GetOrCreateDM: %v", err)
	}
	// idempotent regardless of argument order
	id2, err := s.GetOrCreateDM(bob, alice)
	if err != nil {
		t.Fatalf("GetOrCreateDM (reversed): %v", err)
	}
	if id1 != id2 {
		t.Errorf("DM should be reused: id1=%d id2=%d", id1, id2)
	}
}

func TestChatMessagesPagination(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	ch, err := s.CreateChatChannel("c", "", "channel", alice, nil)
	if err != nil {
		t.Fatalf("CreateChatChannel: %v", err)
	}
	var ids []int64
	for i := 0; i < 5; i++ {
		m, err := s.CreateChatMessage(ch, alice, "msg", nil)
		if err != nil {
			t.Fatalf("CreateChatMessage: %v", err)
		}
		ids = append(ids, m.ID)
	}

	// default: all five, ascending
	all, err := s.ListChatMessages(ch, 0, 0, 0)
	if err != nil {
		t.Fatalf("ListChatMessages: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(all))
	}
	for i := 1; i < len(all); i++ {
		if all[i].ID <= all[i-1].ID {
			t.Errorf("messages not ascending at %d: %d <= %d", i, all[i].ID, all[i-1].ID)
		}
	}

	// before the last id, limit 2 -> the two immediately preceding, ascending
	before, err := s.ListChatMessages(ch, ids[4], 0, 2)
	if err != nil {
		t.Fatalf("ListChatMessages before: %v", err)
	}
	if len(before) != 2 || before[0].ID != ids[2] || before[1].ID != ids[3] {
		t.Errorf("before paging wrong: %+v (want ids %d,%d)", msgIDs(before), ids[2], ids[3])
	}

	// after the second id -> the three newer ones
	after, err := s.ListChatMessages(ch, 0, ids[1], 50)
	if err != nil {
		t.Fatalf("ListChatMessages after: %v", err)
	}
	if len(after) != 3 || after[0].ID != ids[2] {
		t.Errorf("after paging wrong: %v", msgIDs(after))
	}
}

func TestDeleteChatMessageSoftDelete(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	ch, _ := s.CreateChatChannel("c", "", "channel", alice, nil)
	m, err := s.CreateChatMessage(ch, alice, "to be deleted", nil)
	if err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}
	if err := s.DeleteChatMessage(m.ID); err != nil {
		t.Fatalf("DeleteChatMessage: %v", err)
	}
	got, err := s.GetChatMessage(m.ID)
	if err != nil {
		t.Fatalf("GetChatMessage: %v", err)
	}
	if !got.Deleted {
		t.Error("message should be marked deleted")
	}
	// soft delete keeps the row but clears the text in the timeline
	list, _ := s.ListChatMessages(ch, 0, 0, 0)
	if len(list) != 1 {
		t.Fatalf("expected row to remain, got %d", len(list))
	}
	if list[0].Text != "" {
		t.Errorf("deleted message text should be empty, got %q", list[0].Text)
	}
}

func TestMarkChatReadUnread(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")
	ch, _ := s.CreateChatChannel("c", "", "channel", alice, []int64{bob})

	// alice posts two messages; her own messages must not count for her
	var last int64
	for i := 0; i < 2; i++ {
		m, _ := s.CreateChatMessage(ch, alice, "hi", nil)
		last = m.ID
	}
	if n := s.TotalChatUnread(alice); n != 0 {
		t.Errorf("author should have 0 unread, got %d", n)
	}
	if n := s.TotalChatUnread(bob); n != 2 {
		t.Errorf("bob should have 2 unread, got %d", n)
	}

	// after bob reads up to the last id, unread clears
	if err := s.MarkChatRead(ch, bob, last); err != nil {
		t.Fatalf("MarkChatRead: %v", err)
	}
	if n := s.TotalChatUnread(bob); n != 0 {
		t.Errorf("bob should have 0 unread after read, got %d", n)
	}
}

func TestListChatChannelsAccess(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")

	pub, _ := s.CreateChatChannel("public", "", "channel", alice, nil)
	grp, _ := s.CreateChatChannel("secret", "", "group", alice, nil) // bob not a member

	bobChans, err := s.ListChatChannels(bob)
	if err != nil {
		t.Fatalf("ListChatChannels: %v", err)
	}
	seen := map[int64]bool{}
	for _, c := range bobChans {
		seen[c.ID] = true
	}
	if !seen[pub] {
		t.Error("bob should see the public channel")
	}
	if seen[grp] {
		t.Error("bob should NOT see a private group he doesn't belong to")
	}

	// alice (creator) sees both
	aliceChans, _ := s.ListChatChannels(alice)
	if len(aliceChans) != 2 {
		t.Errorf("alice should see 2 channels, got %d", len(aliceChans))
	}
}

func TestSearchChatMessagesAccess(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")

	grp, _ := s.CreateChatChannel("secret", "", "group", alice, nil)
	s.CreateChatMessage(grp, alice, "private treasure", nil)
	pub, _ := s.CreateChatChannel("public", "", "channel", alice, nil)
	s.CreateChatMessage(pub, alice, "public treasure", nil)

	// bob cannot find the private message but can find the public one
	bobHits, err := s.SearchChatMessages(bob, "treasure", 50)
	if err != nil {
		t.Fatalf("SearchChatMessages: %v", err)
	}
	if len(bobHits) != 1 || bobHits[0].ChannelID != pub {
		t.Errorf("bob should find only the public hit, got %d hits", len(bobHits))
	}

	// alice (member of both) finds both
	aliceHits, _ := s.SearchChatMessages(alice, "treasure", 50)
	if len(aliceHits) != 2 {
		t.Errorf("alice should find 2 hits, got %d", len(aliceHits))
	}
}

func TestChatAttachments(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	ch, _ := s.CreateChatChannel("c", "", "channel", alice, nil)
	fid, err := s.SaveFile("doc.pdf", []byte("pdf"), "application/pdf")
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	m, err := s.CreateChatMessage(ch, alice, "see attached", []model.ChatAttachment{{FileID: fid, Filename: "doc.pdf"}})
	if err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].FileID != fid || m.Attachments[0].URL != "/api/files/"+itoaTest(fid) {
		t.Fatalf("attachment not returned on create: %+v", m.Attachments)
	}
	// batch-loaded via list
	list, _ := s.ListChatMessages(ch, 0, 0, 0)
	if len(list) != 1 || len(list[0].Attachments) != 1 || list[0].Attachments[0].Filename != "doc.pdf" {
		t.Fatalf("attachment not batch-loaded in list: %+v", list)
	}
	// GC must keep a chat-referenced file
	n, _ := s.CleanupOrphanFiles()
	if _, _, _, err := s.GetFile(fid); err != nil {
		t.Errorf("chat-referenced file deleted by GC (removed %d): %v", n, err)
	}
}

func itoaTest(n int64) string {
	return fmt.Sprintf("%d", n)
}

func msgIDs(msgs []model.ChatMessage) []int64 {
	ids := make([]int64, len(msgs))
	for i, m := range msgs {
		ids[i] = m.ID
	}
	return ids
}
