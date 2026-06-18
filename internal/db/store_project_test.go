package db

import "testing"

func TestEffectiveProjectRole(t *testing.T) {
	uid := int64(7)
	other := int64(9)
	cases := []struct {
		name       string
		visibility string
		memberRole string
		createdBy  *int64
		isAdmin    bool
		want       string
	}{
		{"global admin", "restricted", "", nil, true, "admin"},
		{"creator", "restricted", "", &uid, false, "admin"},
		{"explicit member", "restricted", "viewer", &other, false, "viewer"},
		{"open implicit editor", "open", "", &other, false, "editor"},
		{"restricted non-member denied", "restricted", "", &other, false, ""},
	}
	for _, c := range cases {
		if got := effectiveProjectRole(c.visibility, c.memberRole, uid, c.createdBy, c.isAdmin); got != c.want {
			t.Errorf("%s: got %q want %q", c.name, got, c.want)
		}
	}
}

func TestProjectAccess(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")

	pid, err := s.CreateProject("Secret", "", "restricted", alice)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	contains := func(userID int64, isAdmin bool, projectID int64) bool {
		ps, err := s.ListProjects(userID, isAdmin)
		if err != nil {
			t.Fatalf("ListProjects: %v", err)
		}
		for _, p := range ps {
			if p.ID == projectID {
				return true
			}
		}
		return false
	}

	// Default project 1 "Общее" is open → everyone sees it.
	if !contains(bob, false, 1) {
		t.Error("bob should see the open default project")
	}
	// Restricted project: creator (alice) sees it, bob (non-member) does not.
	if !contains(alice, false, pid) {
		t.Error("creator should see their restricted project")
	}
	if contains(bob, false, pid) {
		t.Error("non-member must NOT see a restricted project")
	}
	// Global admin sees everything.
	if !contains(bob, true, pid) {
		t.Error("global admin should see the restricted project")
	}
	// Roles.
	if r, _ := s.ProjectRole(pid, alice, false); r != "admin" {
		t.Errorf("alice role on own project = %q, want admin", r)
	}
	if r, _ := s.ProjectRole(pid, bob, false); r != "" {
		t.Errorf("bob role on restricted = %q, want empty", r)
	}
	if r, _ := s.ProjectRole(1, bob, false); r != "editor" {
		t.Errorf("bob role on open default = %q, want editor", r)
	}
	// Add bob as viewer → now visible with viewer role.
	if err := s.SetProjectMember(pid, bob, "viewer"); err != nil {
		t.Fatalf("SetProjectMember: %v", err)
	}
	if r, _ := s.ProjectRole(pid, bob, false); r != "viewer" {
		t.Errorf("bob role after add = %q, want viewer", r)
	}
	if !contains(bob, false, pid) {
		t.Error("bob should see the project after being added")
	}
	// Default project cannot be deleted.
	if err := s.DeleteProject(1); err == nil {
		t.Error("deleting default project must fail")
	}
}
