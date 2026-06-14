package db

import (
	"fmt"
	"testing"
)

// TestCleanupOrphanFiles verifies that attachments referenced from wiki pages,
// wiki revision history, and chat messages are NOT garbage-collected, while
// genuinely unreferenced uploads are. Regression test: the GC previously scanned
// only kanban comments/task descriptions and silently deleted wiki/chat files.
func TestCleanupOrphanFiles(t *testing.T) {
	s := newTestStore(t)
	author := mustUser(t, s, "alice")

	imgRef := func(id int64) string { return fmt.Sprintf(`<img src="/api/images/%d">`, id) }
	fileRef := func(id int64) string { return fmt.Sprintf(`<a href="/api/files/%d">f</a>`, id) }

	// Image referenced only by a wiki page's current content.
	wikiImg, err := s.SaveImage([]byte("png"), "image/png")
	if err != nil {
		t.Fatalf("SaveImage: %v", err)
	}
	spaceID, err := s.CreateWikiSpace("DOC", "Docs", "", "open", author)
	if err != nil {
		t.Fatalf("CreateWikiSpace: %v", err)
	}
	if _, err := s.CreateWikiPage(spaceID, nil, "Page", "<p>"+imgRef(wikiImg)+"</p>", author); err != nil {
		t.Fatalf("CreateWikiPage: %v", err)
	}

	// File referenced only by an old wiki revision (current content drops it).
	revPageImg, _ := s.SaveImage([]byte("png2"), "image/png")
	pid, err := s.CreateWikiPage(spaceID, nil, "Hist", "<p>"+imgRef(revPageImg)+"</p>", author)
	if err != nil {
		t.Fatalf("CreateWikiPage hist: %v", err)
	}
	if _, err := s.UpdateWikiPage(pid, "Hist", "<p>no image now</p>", "drop", author, 1); err != nil {
		t.Fatalf("UpdateWikiPage: %v", err)
	}

	// File referenced only by a chat message.
	chatFile, _ := s.SaveFile("doc.pdf", []byte("pdf"), "application/pdf")
	ch, _ := s.CreateChatChannel("c", "", "channel", author, nil)
	if _, err := s.CreateChatMessage(ch, author, "see "+fileRef(chatFile)); err != nil {
		t.Fatalf("CreateChatMessage: %v", err)
	}

	// A genuinely orphaned upload that nothing references.
	orphan, _ := s.SaveImage([]byte("orphan"), "image/png")

	n, err := s.CleanupOrphanFiles()
	if err != nil {
		t.Fatalf("CleanupOrphanFiles: %v", err)
	}
	if n != 1 {
		t.Errorf("expected exactly 1 orphan removed, got %d", n)
	}

	// Referenced uploads must survive.
	for _, id := range []int64{wikiImg, revPageImg} {
		if _, _, err := s.GetImage(id); err != nil {
			t.Errorf("referenced image %d was wrongly deleted: %v", id, err)
		}
	}
	if _, _, _, err := s.GetFile(chatFile); err != nil {
		t.Errorf("chat-referenced file %d was wrongly deleted: %v", chatFile, err)
	}
	// The orphan must be gone.
	if _, _, err := s.GetImage(orphan); err == nil {
		t.Errorf("orphan image %d should have been deleted", orphan)
	}
}
