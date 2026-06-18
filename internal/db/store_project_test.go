package db

import "testing"

// TestKanbanProjectIsolation verifies that columns, tasks, tags and per-project
// task numbering are fully scoped to their owning project.
func TestKanbanProjectIsolation(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")

	p2, err := s.CreateProject("Beta", "", "open", alice)
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}

	// Both projects get the seeded starter columns, independently.
	c1, err := s.ListColumns(1)
	if err != nil || len(c1) == 0 {
		t.Fatalf("ListColumns(1): %v (n=%d)", err, len(c1))
	}
	c2, err := s.ListColumns(p2)
	if err != nil || len(c2) == 0 {
		t.Fatalf("ListColumns(p2): %v (n=%d)", err, len(c2))
	}
	if c1[0].ID == c2[0].ID {
		t.Fatal("projects must not share columns")
	}

	// Tasks land in their project and get a per-project sequential number.
	t1a, err := s.CreateTask(1, "a", "", "", "", c1[0].ID, nil, nil, nil, 0, nil, "")
	if err != nil {
		t.Fatalf("CreateTask p1: %v", err)
	}
	t2a, err := s.CreateTask(p2, "x", "", "", "", c2[0].ID, nil, nil, nil, 0, nil, "")
	if err != nil {
		t.Fatalf("CreateTask p2: %v", err)
	}
	t1b, err := s.CreateTask(1, "b", "", "", "", c1[0].ID, nil, nil, nil, 0, nil, "")
	if err != nil {
		t.Fatalf("CreateTask p1 #2: %v", err)
	}

	tasks1, _ := s.ListTasks(1)
	tasks2, _ := s.ListTasks(p2)
	if len(tasks1) != 2 || len(tasks2) != 1 {
		t.Fatalf("isolation broken: p1=%d p2=%d", len(tasks1), len(tasks2))
	}
	num := map[int64]int{}
	for _, tk := range tasks1 {
		num[tk.ID] = tk.Number
	}
	if num[t1a] != 1 || num[t1b] != 2 {
		t.Errorf("per-project numbering wrong: t1a=%d t1b=%d", num[t1a], num[t1b])
	}
	for _, tk := range tasks2 {
		if tk.ID == t2a && tk.Number != 1 {
			t.Errorf("p2 first task number = %d, want 1", tk.Number)
		}
	}

	// The same tag name is allowed in different projects (UNIQUE(project_id,name)).
	if _, err := s.CreateTag(1, "urgent", "#f00"); err != nil {
		t.Fatalf("CreateTag p1: %v", err)
	}
	if _, err := s.CreateTag(p2, "urgent", "#f00"); err != nil {
		t.Fatalf("CreateTag p2 (same name, other project): %v", err)
	}
	tg1, _ := s.ListTags(1)
	tg2, _ := s.ListTags(p2)
	if len(tg1) != 1 || len(tg2) != 1 {
		t.Fatalf("tag isolation broken: p1=%d p2=%d", len(tg1), len(tg2))
	}
}

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
