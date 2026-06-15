package db

import (
	"testing"

	"kanban/internal/model"
)

func TestInternalMailFlow(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")

	mid, err := s.SendInternalMail(alice, "alice", []int64{bob}, "bob", "", "Hello", "plain body", "<p>hi</p>", "", 0, nil)
	if err != nil {
		t.Fatalf("SendInternalMail: %v", err)
	}

	// sender sees it in Sent (read), recipient in Inbox (unread)
	sent, _ := s.ListMailbox(alice, "sent", 50, 0)
	if len(sent) != 1 || sent[0].ID != mid {
		t.Fatalf("sender Sent wrong: %+v", sent)
	}
	inbox, _ := s.ListMailbox(bob, "inbox", 50, 0)
	if len(inbox) != 1 || inbox[0].Seen {
		t.Fatalf("recipient inbox wrong: %+v", inbox)
	}
	if n := s.UnreadMailCount(bob); n != 1 {
		t.Errorf("bob unread = %d, want 1", n)
	}

	// thread id is the root message id
	if inbox[0].ThreadID != mid {
		t.Errorf("thread id = %d, want %d", inbox[0].ThreadID, mid)
	}

	// open + mark seen
	full, err := s.GetMailBox(bob, inbox[0].BoxID)
	if err != nil || full.BodyHTML != "<p>hi</p>" {
		t.Fatalf("GetMailBox: %v (%+v)", err, full)
	}
	s.SetMailSeen(inbox[0].BoxID, bob, true)
	if n := s.UnreadMailCount(bob); n != 0 {
		t.Errorf("bob unread after read = %d, want 0", n)
	}

	// bob replies into the same thread
	if _, err := s.SendInternalMail(bob, "bob", []int64{alice}, "alice", "", "Re: Hello", "reply body", "", "", mid, nil); err != nil {
		t.Fatalf("reply: %v", err)
	}
	thread, _ := s.ThreadForUser(alice, mid)
	if len(thread) != 2 {
		t.Errorf("alice thread should have 2 messages, got %d", len(thread))
	}

	// move bob's message to trash; folder counts reflect it
	s.MoveMail(inbox[0].BoxID, bob, "trash")
	if box, _ := s.ListMailbox(bob, "inbox", 50, 0); len(box) != 0 {
		t.Errorf("inbox should be empty after trashing, got %d", len(box))
	}
	if box, _ := s.ListMailbox(bob, "trash", 50, 0); len(box) != 1 {
		t.Errorf("trash should have 1, got %d", len(box))
	}

	// search by subject
	hits, _ := s.SearchMail(alice, "Hello", 50)
	if len(hits) == 0 {
		t.Error("search should find the message for alice")
	}
}

func TestMailCustomFolders(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")
	mid, _ := s.SendInternalMail(alice, "alice", []int64{bob}, "bob", "", "Hi", "x", "", "", 0, nil)
	_ = mid

	fid, err := s.CreateMailFolder(bob, "Work")
	if err != nil {
		t.Fatalf("CreateMailFolder: %v", err)
	}
	if _, err := s.CreateMailFolder(bob, "inbox"); err == nil {
		t.Error("reserved folder name should be rejected")
	}
	box, _ := s.ListMailbox(bob, "inbox", 50, 0)
	if len(box) != 1 {
		t.Fatalf("expected 1 inbox msg, got %d", len(box))
	}
	if err := s.MoveMail(box[0].BoxID, bob, "Work"); err != nil {
		t.Fatalf("MoveMail to custom: %v", err)
	}
	if err := s.MoveMail(box[0].BoxID, bob, "bogus"); err == nil {
		t.Error("move to unknown folder should fail")
	}
	if w, _ := s.ListMailbox(bob, "Work", 50, 0); len(w) != 1 {
		t.Errorf("Work should have 1 msg, got %d", len(w))
	}
	// deleting the folder returns its messages to inbox
	if err := s.DeleteMailFolder(fid, bob); err != nil {
		t.Fatalf("DeleteMailFolder: %v", err)
	}
	if in, _ := s.ListMailbox(bob, "inbox", 50, 0); len(in) != 1 {
		t.Errorf("messages should return to inbox after folder delete, got %d", len(in))
	}
}

func TestSettingsExportImportRoundTrip(t *testing.T) {
	s := newTestStore(t)
	s.SetSetting("mail_dkim_key", "PRIVATE-KEY-PEM")
	s.SetSetting("telegram_bot_token", "123:ABC")
	s.SetSetting("mail_domain", "example.com")

	exp, err := s.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	got := map[string]string{}
	for _, st := range exp.Settings {
		got[st.Key] = st.Value
	}
	// Export is a complete backup: secrets are included (migratable).
	if got["mail_dkim_key"] != "PRIVATE-KEY-PEM" || got["telegram_bot_token"] != "123:ABC" {
		t.Fatalf("secrets missing from export: %+v", got)
	}
	// Restore into a fresh instance reproduces every setting.
	b := newTestStore(t)
	if err := b.ImportAll(exp); err != nil {
		t.Fatalf("ImportAll: %v", err)
	}
	for _, k := range []string{"mail_dkim_key", "telegram_bot_token", "mail_domain"} {
		if b.GetSetting(k) != got[k] {
			t.Errorf("setting %q not restored: got %q want %q", k, b.GetSetting(k), got[k])
		}
	}
}

func TestEvaluateRules(t *testing.T) {
	rules := []model.MailRule{
		{Enabled: true, MatchType: "all",
			Conditions: []model.MailRuleCondition{{Field: "subject", Op: "contains", Value: "invoice"}},
			Actions:    []model.MailRuleAction{{Type: "move_to_folder", Arg: "Work"}}},
		{Enabled: false, MatchType: "all",
			Conditions: []model.MailRuleCondition{{Field: "from", Op: "contains", Value: "spam"}},
			Actions:    []model.MailRuleAction{{Type: "delete"}}},
		{Enabled: true, MatchType: "any",
			Conditions: []model.MailRuleCondition{{Field: "from", Op: "equals", Value: "boss@x"}, {Field: "subject", Op: "starts_with", Value: "URGENT"}},
			Actions:    []model.MailRuleAction{{Type: "flag"}}},
	}
	acts := EvaluateRules(rules, RuleContext{From: "billing@x", Subject: "Your invoice #5"})
	if len(acts) != 1 || acts[0].Type != "move_to_folder" || acts[0].Arg != "Work" {
		t.Fatalf("expected move_to_folder Work, got %+v", acts)
	}
	// disabled rule must not fire even if it would match
	acts = EvaluateRules(rules, RuleContext{From: "spammer", Subject: "nothing"})
	if len(acts) != 0 {
		t.Errorf("disabled rule fired: %+v", acts)
	}
	// any-match rule fires on one condition
	acts = EvaluateRules(rules, RuleContext{From: "boss@x", Subject: "hi"})
	if len(acts) != 1 || acts[0].Type != "flag" {
		t.Errorf("any-rule should flag, got %+v", acts)
	}
}

func TestMailFolderCounts(t *testing.T) {
	s := newTestStore(t)
	alice := mustUser(t, s, "alice")
	bob := mustUser(t, s, "bob")
	s.SendInternalMail(alice, "alice", []int64{bob}, "bob", "", "A", "a", "", "", 0, nil)
	s.SendInternalMail(alice, "alice", []int64{bob}, "bob", "", "B", "b", "", "", 0, nil)

	counts, _ := s.MailFolderCounts(bob)
	var inbox *struct{ u, total int }
	for _, c := range counts {
		if c.Folder == "inbox" {
			inbox = &struct{ u, total int }{c.Unread, c.Total}
		}
	}
	if inbox == nil || inbox.u != 2 || inbox.total != 2 {
		t.Errorf("inbox counts wrong: %+v", counts)
	}
}
