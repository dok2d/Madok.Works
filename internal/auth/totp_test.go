package auth

import (
	"crypto/hmac"
	"testing"
	"time"
)

func TestTOTPRoundTrip(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("GenerateTOTPSecret: %v", err)
	}
	// The current code must verify.
	counter := uint64(time.Now().Unix() / totpPeriod)
	code, err := totpAt(secret, counter)
	if err != nil {
		t.Fatalf("totpAt: %v", err)
	}
	if !VerifyTOTP(secret, code) {
		t.Errorf("current code %q should verify", code)
	}
	// A code from the previous window is accepted (±1 skew).
	prev, _ := totpAt(secret, counter-1)
	if !VerifyTOTP(secret, prev) {
		t.Errorf("previous-window code should verify (skew)")
	}
	// A far-off code must not verify.
	far, _ := totpAt(secret, counter+10)
	if VerifyTOTP(secret, far) && !hmac.Equal([]byte(far), []byte(code)) {
		t.Errorf("far-future code should not verify")
	}
	if VerifyTOTP(secret, "000000") && code != "000000" {
		// extremely unlikely to collide; guard anyway
		t.Errorf("arbitrary code should not verify")
	}
	if VerifyTOTP(secret, "12345") {
		t.Errorf("wrong-length code must be rejected")
	}
}

func TestTOTPURI(t *testing.T) {
	u := TOTPURI("Kanban", "alice", "ABC")
	if u == "" || u[:10] != "otpauth://" {
		t.Errorf("bad URI: %s", u)
	}
}
