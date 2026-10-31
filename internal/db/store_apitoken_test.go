package db

import (
	"strings"
	"testing"
	"time"
)

func TestAPITokenLifecycle(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")

	tok, meta, err := s.CreateAPIToken(alice, "ci", "readonly", []string{"kanban", "wiki"}, 0)
	if err != nil {
		t.Fatalf("CreateAPIToken: %v", err)
	}
	if !strings.HasPrefix(tok, APITokenPrefix) || !strings.HasPrefix(tok, meta.Prefix) || len(meta.Prefix) != 11 {
		t.Fatalf("token %q / prefix %q", tok, meta.Prefix)
	}
	var stored string
	s.db.QueryRow("SELECT token_hash FROM api_tokens WHERE id=?", meta.ID).Scan(&stored)
	if stored == "" || strings.Contains(stored, tok[3:]) {
		t.Fatalf("token stored in the clear or not at all: %q", stored)
	}

	u, got, err := s.ValidateAPIToken(tok)
	if err != nil {
		t.Fatalf("ValidateAPIToken: %v", err)
	}
	if u.ID != alice || u.Username != "alice" || got.Role != "readonly" || strings.Join(got.Services, ",") != "kanban,wiki" {
		t.Fatalf("owner %+v token %+v", u, got)
	}
	if got.LastUsedAt == 0 {
		t.Fatal("last_used_at not recorded")
	}
	if _, _, err := s.ValidateAPIToken(tok + "x"); err == nil {
		t.Fatal("tampered token accepted")
	}

	if list, _ := s.ListAPITokens(alice); len(list) != 1 || list[0].Name != "ci" || list[0].Username != "alice" {
		t.Fatalf("ListAPITokens(alice) = %+v", list)
	}
	s.CreateAPIToken(bob, "b", "regular", nil, 0)
	if list, _ := s.ListAPITokens(0); len(list) != 2 {
		t.Fatalf("ListAPITokens(0) = %d tokens, want 2", len(list))
	}

	if ok, _ := s.DeleteAPIToken(meta.ID, bob); ok {
		t.Fatal("bob revoked alice's token")
	}
	if ok, _ := s.DeleteAPIToken(meta.ID, alice); !ok {
		t.Fatal("alice could not revoke her token")
	}
	if _, _, err := s.ValidateAPIToken(tok); err == nil {
		t.Fatal("revoked token still valid")
	}

	old, _, _ := s.CreateAPIToken(alice, "old", "readonly", nil, time.Now().Add(-time.Second).Unix())
	if _, _, err := s.ValidateAPIToken(old); err == nil {
		t.Fatal("expired token accepted")
	}

	// Tokens go away with their owner.
	if err := s.DeleteUser(bob); err != nil {
		t.Fatalf("DeleteUser: %v", err)
	}
	if list, _ := s.ListAPITokens(bob); len(list) != 0 {
		t.Fatalf("bob's tokens survived his deletion: %+v", list)
	}
}
