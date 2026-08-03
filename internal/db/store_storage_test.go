package db

import (
	"errors"
	"testing"
)

func TestStorageFoldersAndItems(t *testing.T) {
	s := newTestStore(t)
	user := mustUser(t, s, "alice")

	docs, err := s.CreateStorageFolder("Docs", nil, user)
	if err != nil {
		t.Fatalf("CreateStorageFolder: %v", err)
	}
	sub, err := s.CreateStorageFolder("Templates", &docs, user)
	if err != nil {
		t.Fatalf("create subfolder: %v", err)
	}

	// Duplicate folder name in the same parent (case-insensitive) is rejected.
	if _, err := s.CreateStorageFolder("docs", nil, user); !errors.Is(err, ErrStorageNameTaken) {
		t.Errorf("dup folder: want ErrStorageNameTaken, got %v", err)
	}
	// Same name in another parent is fine.
	if _, err := s.CreateStorageFolder("Docs", &docs, user); err != nil {
		t.Errorf("same name in other parent: %v", err)
	}

	// Upload two files with the same name into the root: second gets a suffix.
	f1, _ := s.SaveFile("a.pdf", []byte("one"), "application/pdf")
	f2, _ := s.SaveFile("a.pdf", []byte("two"), "application/pdf")
	_, name1, err := s.CreateStorageItem(nil, f1, "a.pdf", user)
	if err != nil || name1 != "a.pdf" {
		t.Fatalf("item1: name=%q err=%v", name1, err)
	}
	it2, name2, err := s.CreateStorageItem(nil, f2, "a.pdf", user)
	if err != nil || name2 != "a (2).pdf" {
		t.Fatalf("item2: want auto-suffix, got name=%q err=%v", name2, err)
	}

	// Rename onto an existing name → 409 sentinel; onto a free name → ok.
	if err := s.RenameStorageItem(it2, "A.PDF"); !errors.Is(err, ErrStorageNameTaken) {
		t.Errorf("rename dup: want ErrStorageNameTaken, got %v", err)
	}
	if err := s.RenameStorageItem(it2, "b.pdf"); err != nil {
		t.Errorf("rename: %v", err)
	}

	// Move item into a folder; listing reflects it.
	if err := s.MoveStorageItem(it2, &sub); err != nil {
		t.Fatalf("MoveStorageItem: %v", err)
	}
	root, err := s.ListStorageItems(nil)
	if err != nil || len(root) != 1 || root[0].Name != "a.pdf" {
		t.Fatalf("root items: %+v err=%v", root, err)
	}
	inSub, _ := s.ListStorageItems(&sub)
	if len(inSub) != 1 || inSub[0].Name != "b.pdf" || inSub[0].Size != 3 || inSub[0].Mime != "application/pdf" {
		t.Fatalf("sub items: %+v", inSub)
	}
	if inSub[0].UploaderName != "alice" {
		t.Errorf("uploader name: %q", inSub[0].UploaderName)
	}

	// Folder move cycle checks: Docs → Templates (its own child) must fail.
	if err := s.MoveStorageFolder(docs, &sub); !errors.Is(err, ErrStorageCycle) {
		t.Errorf("move into descendant: want ErrStorageCycle, got %v", err)
	}
	if err := s.MoveStorageFolder(docs, &docs); !errors.Is(err, ErrStorageCycle) {
		t.Errorf("move into self: want ErrStorageCycle, got %v", err)
	}

	// Usage counts both items.
	cnt, total := s.StorageUsage()
	if cnt != 2 || total != 6 {
		t.Errorf("usage: cnt=%d total=%d", cnt, total)
	}

	// Recursive folder delete removes subtree rows and blobs.
	removed, err := s.DeleteStorageFolder(docs)
	if err != nil {
		t.Fatalf("DeleteStorageFolder: %v", err)
	}
	if removed != 1 {
		t.Errorf("removed files: want 1, got %d", removed)
	}
	if _, err := s.GetStorageFolder(sub); err == nil {
		t.Errorf("subfolder should be cascade-deleted")
	}
	if _, _, _, err := s.GetFile(f2); err == nil {
		t.Errorf("blob of deleted subtree item should be gone")
	}
	if _, _, _, err := s.GetFile(f1); err != nil {
		t.Errorf("root item blob wrongly deleted: %v", err)
	}
}

func TestStorageItemDeleteAndTokens(t *testing.T) {
	s := newTestStore(t)
	user := mustUser(t, s, "bob")

	fid, _ := s.SaveFile("x.txt", []byte("hello"), "text/plain")
	id, _, err := s.CreateStorageItem(nil, fid, "x.txt", user)
	if err != nil {
		t.Fatalf("CreateStorageItem: %v", err)
	}

	// Share token round-trip.
	if err := s.SetStorageItemToken(id, "tok123"); err != nil {
		t.Fatalf("SetStorageItemToken: %v", err)
	}
	it, err := s.GetStorageItemByToken("tok123")
	if err != nil || it.ID != id {
		t.Fatalf("GetStorageItemByToken: %+v err=%v", it, err)
	}
	// Revoke → not found; empty token never matches.
	if err := s.SetStorageItemToken(id, ""); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := s.GetStorageItemByToken("tok123"); err == nil {
		t.Errorf("revoked token still resolves")
	}
	if _, err := s.GetStorageItemByToken(""); err == nil {
		t.Errorf("empty token must not resolve")
	}

	// Deleting the item deletes the blob.
	if err := s.DeleteStorageItem(id); err != nil {
		t.Fatalf("DeleteStorageItem: %v", err)
	}
	if _, _, _, err := s.GetFile(fid); err == nil {
		t.Errorf("blob should be deleted with the item")
	}
}

func TestSearchStorage(t *testing.T) {
	s := newTestStore(t)
	user := mustUser(t, s, "carol")

	rep, _ := s.CreateStorageFolder("Annual Reports", nil, user)
	f, _ := s.SaveFile("report-2026.pdf", []byte("data"), "application/pdf")
	if _, _, err := s.CreateStorageItem(&rep, f, "report-2026.pdf", user); err != nil {
		t.Fatalf("CreateStorageItem: %v", err)
	}

	folders, items, err := s.SearchStorage("report", 10)
	if err != nil {
		t.Fatalf("SearchStorage: %v", err)
	}
	if len(folders) != 1 || len(items) != 1 {
		t.Fatalf("want 1+1 hits, got %d folders %d items", len(folders), len(items))
	}
	// Multi-term AND: both terms must match.
	_, items, _ = s.SearchStorage("report 2026", 10)
	if len(items) != 1 {
		t.Errorf("multi-term should match item, got %d", len(items))
	}
	folders, items, _ = s.SearchStorage("report nosuch", 10)
	if len(folders) != 0 || len(items) != 0 {
		t.Errorf("no hits expected, got %d folders %d items", len(folders), len(items))
	}
}
