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
		m, err := s.CreateChatMessage(ch, alice, "msg", nil, 0)
		if err != nil {
			t.Fatalf("CreateChatMessage: %v", err)
		}
		ids = append(ids, m.ID)
	}

	// default: all five, ascending
	all, err := s.ListChatMessages(ch, 0, 0, 0, 0)
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
	before, err := s.ListChatMessages(ch, ids[4], 0, 2, 0)
	if err != nil {
		t.Fatalf("ListChatMessages before: %v", err)
	}
	if len(before) != 2 || before[0].ID != ids[2] || before[1].ID != ids[3] {
		t.Errorf("before paging wrong: %+v (want ids %d,%d)", msgIDs(before), ids[2], ids[3])
	}

	// after the second id -> the three newer ones
	after, err := s.ListChatMessages(ch, 0, ids[1], 50, 0)
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
	m, err := s.CreateChatMessage(ch, alice, "to be deleted", nil, 0)
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
	list, _ := s.ListChatMessages(ch, 0, 0, 0, 0)
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
		m, _ := s.CreateChatMessage(ch, alice, "hi", nil, 0)
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
	s.CreateChatMessage(grp, alice, "private treasure", nil, 0)
	pub, _ := s.CreateChatChannel("public", "", "channel", alice, nil)
	s.CreateChatMessage(pub, alice, "public treasure", nil, 0)

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
	m, err := s.CreateChatMessage(ch, alice, "see attached", []model.ChatAttachment{{FileID: fid, Filename: "doc.pdf"}}, 0)
	if err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}
	if len(m.Attachments) != 1 || m.Attachments[0].FileID != fid || m.Attachments[0].URL != "/api/files/"+itoaTest(fid) {
		t.Fatalf("attachment not returned on create: %+v", m.Attachments)
	}
	// batch-loaded via list
	list, _ := s.ListChatMessages(ch, 0, 0, 0, 0)
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

func TestChatReply(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	ch, _ := s.CreateChatChannel("c", "", "channel", alice, nil)
	parent, err := s.CreateChatMessage(ch, alice, "original", nil, 0)
	if err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}
	reply, err := s.CreateChatMessage(ch, alice, "answer", nil, parent.ID)
	if err != nil {
		t.Fatalf("CreateChatMessage reply: %v", err)
	}
	if reply.ReplyTo == nil || *reply.ReplyTo != parent.ID {
		t.Fatalf("reply_to not set: %+v", reply.ReplyTo)
	}
	if reply.ReplyToAuthor != "alice" || reply.ReplyToText != "original" {
		t.Errorf("reply preview wrong: author=%q text=%q", reply.ReplyToAuthor, reply.ReplyToText)
	}
	// Preview also present when listing.
	list, _ := s.ListChatMessages(ch, 0, 0, 0, alice)
	var found bool
	for _, m := range list {
		if m.ID == reply.ID {
			found = true
			if m.ReplyToText != "original" {
				t.Errorf("list reply preview wrong: %q", m.ReplyToText)
			}
		}
	}
	if !found {
		t.Error("reply not in list")
	}
}

func TestChatPin(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	ch, _ := s.CreateChatChannel("c", "", "channel", alice, nil)
	m, _ := s.CreateChatMessage(ch, alice, "pin me", nil, 0)
	if err := s.SetChatPinned(m.ID, alice, true); err != nil {
		t.Fatalf("SetChatPinned: %v", err)
	}
	pins, err := s.ListPinnedMessages(ch, alice)
	if err != nil {
		t.Fatalf("ListPinnedMessages: %v", err)
	}
	if len(pins) != 1 || pins[0].ID != m.ID || !pins[0].Pinned {
		t.Fatalf("expected 1 pinned message, got %+v", pins)
	}
	if err := s.SetChatPinned(m.ID, alice, false); err != nil {
		t.Fatalf("unpin: %v", err)
	}
	if pins, _ := s.ListPinnedMessages(ch, alice); len(pins) != 0 {
		t.Errorf("expected 0 pins after unpin, got %d", len(pins))
	}
}

func TestChatReactions(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")
	ch, _ := s.CreateChatChannel("c", "", "channel", alice, []int64{bob})
	m, _ := s.CreateChatMessage(ch, alice, "react", nil, 0)

	// alice and bob both add 👍; bob adds ❤️ too.
	for _, u := range []int64{alice, bob} {
		if err := s.ToggleChatReaction(m.ID, u, "👍"); err != nil {
			t.Fatalf("ToggleChatReaction: %v", err)
		}
	}
	if err := s.ToggleChatReaction(m.ID, bob, "❤️"); err != nil {
		t.Fatalf("ToggleChatReaction heart: %v", err)
	}

	got, _ := s.GetChatMessageFor(m.ID, alice)
	thumbs := reactionFor(got.Reactions, "👍")
	if thumbs == nil || thumbs.Count != 2 || !thumbs.Mine {
		t.Fatalf("👍 should be count=2 mine=true, got %+v", thumbs)
	}
	heart := reactionFor(got.Reactions, "❤️")
	if heart == nil || heart.Count != 1 || heart.Mine {
		t.Fatalf("❤️ should be count=1 mine=false for alice, got %+v", heart)
	}

	// Toggling off removes alice's 👍.
	if err := s.ToggleChatReaction(m.ID, alice, "👍"); err != nil {
		t.Fatalf("toggle off: %v", err)
	}
	got, _ = s.GetChatMessageFor(m.ID, alice)
	thumbs = reactionFor(got.Reactions, "👍")
	if thumbs == nil || thumbs.Count != 1 || thumbs.Mine {
		t.Fatalf("👍 should be count=1 mine=false after toggle off, got %+v", thumbs)
	}
}

func TestChatChannelListPreviewAndPeerRead(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")
	dm, err := s.GetOrCreateDM(alice, bob)
	if err != nil {
		t.Fatalf("GetOrCreateDM: %v", err)
	}
	m, _ := s.CreateChatMessage(dm, alice, "hello bob", nil, 0)

	// Alice's view: last-message preview present, peer (bob) hasn't read yet.
	chans, err := s.ListChatChannels(alice)
	if err != nil {
		t.Fatalf("ListChatChannels: %v", err)
	}
	var ch *model.ChatChannel
	for i := range chans {
		if chans[i].ID == dm {
			ch = &chans[i]
		}
	}
	if ch == nil {
		t.Fatal("dm not in list")
	}
	if ch.LastMsgText != "hello bob" || ch.LastMsgFrom != "alice" {
		t.Errorf("preview wrong: text=%q from=%q", ch.LastMsgText, ch.LastMsgFrom)
	}
	if ch.PeerLastRead >= m.ID {
		t.Errorf("peer should not have read yet: peerLastRead=%d msg=%d", ch.PeerLastRead, m.ID)
	}

	// Bob reads; now Alice sees peer_last_read >= message id (read tick).
	if err := s.MarkChatRead(dm, bob, m.ID); err != nil {
		t.Fatalf("MarkChatRead: %v", err)
	}
	got, err := s.GetChatChannelFor(dm, alice)
	if err != nil {
		t.Fatalf("GetChatChannelFor: %v", err)
	}
	if got.PeerLastRead < m.ID {
		t.Errorf("peer read tick not reflected: peerLastRead=%d msg=%d", got.PeerLastRead, m.ID)
	}
}

func TestChatSystemMessage(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	ch, _ := s.CreateChatChannel("c", "", "channel", alice, nil)
	m, err := s.CreateSystemMessage(ch, "join|bob")
	if err != nil {
		t.Fatalf("CreateSystemMessage: %v", err)
	}
	if m.Kind != "system" || m.Text != "join|bob" || m.AuthorID != nil {
		t.Fatalf("bad system message: %+v", m)
	}
	list, _ := s.ListChatMessages(ch, 0, 0, 0, alice)
	var found bool
	for _, x := range list {
		if x.ID == m.ID {
			found = true
			if x.Kind != "system" {
				t.Errorf("kind not returned in list: %q", x.Kind)
			}
		}
	}
	if !found {
		t.Error("system message not in list")
	}
}

func reactionFor(rs []model.ChatReaction, emoji string) *model.ChatReaction {
	for i := range rs {
		if rs[i].Emoji == emoji {
			return &rs[i]
		}
	}
	return nil
}
