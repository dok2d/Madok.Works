package handler

import (
	"testing"
	"time"
)

func TestSMTPRateLimiter(t *testing.T) {
	l := &smtpRateLimiter{hits: map[string][]time.Time{}}
	now := time.Now()
	for i := 0; i < 5; i++ {
		if !l.allow("1.2.3.4", now, 5, time.Minute) {
			t.Fatalf("hit %d should be allowed", i)
		}
	}
	if l.allow("1.2.3.4", now, 5, time.Minute) {
		t.Error("6th hit should be blocked")
	}
	// different IP unaffected
	if !l.allow("5.6.7.8", now, 5, time.Minute) {
		t.Error("other IP should be allowed")
	}
	// window slides: a later time frees the budget
	if !l.allow("1.2.3.4", now.Add(2*time.Minute), 5, time.Minute) {
		t.Error("after the window the IP should be allowed again")
	}
}
