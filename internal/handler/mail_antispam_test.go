package handler

import (
	"path/filepath"
	"strings"
	"testing"

	"kanban/internal/db"
	"kanban/internal/model"
)

func modelMailRule(field, op, value, actType, actArg string) model.MailRule {
	return model.MailRule{
		Enabled: true, MatchType: "all",
		Conditions: []model.MailRuleCondition{{Field: field, Op: op, Value: value}},
		Actions:    []model.MailRuleAction{{Type: actType, Arg: actArg}},
	}
}

func TestScoreSpam(t *testing.T) {
	clean, _ := scoreSpam(SpamSignals{Subject: "Lunch tomorrow?", Body: "Are you free at noon?"})
	if clean >= spamThreshold {
		t.Errorf("clean message scored spam: %d", clean)
	}
	spammy, reasons := scoreSpam(SpamSignals{
		Subject:     "YOU WON THE LOTTERY",
		Body:        "Click here for free money, act now! viagra",
		Attachments: []string{"invoice.pdf.exe"},
	})
	if spammy < spamThreshold {
		t.Errorf("spammy message under threshold: %d (%v)", spammy, reasons)
	}
}

func TestInboundSpamToJunk(t *testing.T) {
	store, err := db.New(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	defer store.Close()
	bob, _ := store.CreateUser("bob", "password123", "user")
	h := &Handler{store: store}

	raw := strings.ReplaceAll(`From: Spammer <x@evil.test>
To: bob@kanban.local
Subject: YOU WON THE LOTTERY WINNER
Content-Type: text/plain; charset=utf-8

Click here for free money, act now! viagra lottery you won bitcoin doubler
`, "\n", "\r\n")
	if err := h.ingestInbound("x@evil.test", []string{"bob@kanban.local"}, strings.NewReader(raw), "9.9.9.9"); err != nil {
		t.Fatalf("ingestInbound: %v", err)
	}
	if junk, _ := store.ListMailbox(bob, "junk", 50, 0); len(junk) != 1 {
		t.Errorf("spam should be in junk, got %d", len(junk))
	}
	if inbox, _ := store.ListMailbox(bob, "inbox", 50, 0); len(inbox) != 0 {
		t.Errorf("spam should not be in inbox, got %d", len(inbox))
	}
	rows, _ := store.ListMailAudit("spam", 10, 0)
	if len(rows) != 1 {
		t.Errorf("expected 1 spam audit row, got %d", len(rows))
	}
}

func TestInboundRuleApplied(t *testing.T) {
	store, err := db.New(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	defer store.Close()
	bob, _ := store.CreateUser("bob", "password123", "user")
	store.CreateMailFolder(bob, "Promo")
	store.CreateMailRule(bob, modelMailRule("from", "contains", "shop@", "move_to_folder", "Promo"))
	h := &Handler{store: store}

	raw := strings.ReplaceAll(`From: Shop <shop@store.test>
To: bob@kanban.local
Subject: weekly deals
Content-Type: text/plain; charset=utf-8

hello
`, "\n", "\r\n")
	if err := h.ingestInbound("shop@store.test", []string{"bob@kanban.local"}, strings.NewReader(raw), "2.2.2.2"); err != nil {
		t.Fatalf("ingestInbound: %v", err)
	}
	if p, _ := store.ListMailbox(bob, "Promo", 50, 0); len(p) != 1 {
		t.Errorf("rule should move message to Promo, got %d", len(p))
	}
	if in, _ := store.ListMailbox(bob, "inbox", 50, 0); len(in) != 0 {
		t.Errorf("message should have left inbox, got %d", len(in))
	}
}
