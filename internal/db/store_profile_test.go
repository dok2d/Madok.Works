package db

import "testing"

func TestProfileNameAndAvatar(t *testing.T) {
	s := newTestStore(t)
	uid := mustUser(t, s, "alice")

	if err := s.UpdateUserDisplayName(uid, "  Alice Liddell "); err != nil {
		t.Fatal(err)
	}
	s.FillDisplayNameIfEmpty(uid, "From SSO") // must not overwrite the user's choice
	u, _ := s.GetUser(uid)
	if u.DisplayName != "Alice Liddell" {
		t.Fatalf("display name = %q", u.DisplayName)
	}
	bob := mustUser(t, s, "bob")
	s.FillDisplayNameIfEmpty(bob, "Bob Builder")
	if b, _ := s.GetUser(bob); b.DisplayName != "Bob Builder" {
		t.Fatalf("SSO name not seeded: %q", b.DisplayName)
	}

	first, err := s.SetUserAvatar(uid, []byte("img1"), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SetUserAvatar(uid, []byte("img2"), "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.GetImage(first); err == nil {
		t.Error("replaced avatar image was not deleted")
	}
	if !s.IsAvatar(second) || s.IsAvatar(first) {
		t.Error("IsAvatar mismatch")
	}
	if !s.BlobUsedByServices("images", second, []string{"wiki"}) {
		t.Error("avatar should be readable by any scoped token")
	}

	// The orphan sweep must keep avatars (they are referenced by users, not text).
	if _, err := s.CleanupOrphanFiles(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.GetImage(second); err != nil {
		t.Fatalf("orphan cleanup deleted a live avatar: %v", err)
	}

	// Export/import round trip keeps the profile.
	data, err := s.ExportAll()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ImportAll(data); err != nil {
		t.Fatalf("ImportAll: %v", err)
	}
	u, _ = s.GetUser(uid)
	if u.DisplayName != "Alice Liddell" || u.AvatarID != second {
		t.Fatalf("after round trip: name %q avatar %d (want %d)", u.DisplayName, u.AvatarID, second)
	}

	if err := s.ClearUserAvatar(uid); err != nil {
		t.Fatal(err)
	}
	u, _ = s.GetUser(uid)
	if _, _, err := s.GetImage(second); u.AvatarID != 0 || err == nil {
		t.Fatalf("avatar not cleared: id %d, image err %v", u.AvatarID, err)
	}
}
