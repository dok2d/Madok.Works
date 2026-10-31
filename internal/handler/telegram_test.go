package handler

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestTgRedactHidesToken ensures a failed Bot API request logs without the
// token: net/http quotes the whole request URL, token included, in its error.
func TestTgRedactHidesToken(t *testing.T) {
	const token = "123456:SECRET-token_value"
	client := &http.Client{Timeout: time.Millisecond}
	_, err := client.Get("http://127.0.0.1:1/bot" + token + "/getUpdates?offset=0")
	if err == nil {
		t.Fatal("expected a request error")
	}
	if !strings.Contains(err.Error(), token) {
		t.Fatalf("precondition: raw error should carry the token, got %q", err)
	}
	got := tgRedact(err, token)
	if strings.Contains(got, token) || strings.Contains(got, "SECRET") {
		t.Fatalf("token leaked: %q", got)
	}
	if !strings.Contains(got, "/bot<token>/getUpdates") {
		t.Fatalf("redacted error lost its context: %q", got)
	}
}
