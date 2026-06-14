package handler

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"

	"github.com/emersion/go-msgauth/dkim"
)

func TestBuildRFC822(t *testing.T) {
	msg := buildRFC822("alice@example.com", "Alice", []string{"bob@remote.test"}, "Hi", "line one\nline two", "Mon, 01 Jan 2026 00:00:00 +0000")
	s := string(msg)
	for _, want := range []string{"From: Alice <alice@example.com>\r\n", "To: bob@remote.test\r\n", "Subject: Hi\r\n", "line one\r\nline two"} {
		if !strings.Contains(s, want) {
			t.Errorf("message missing %q\n---\n%s", want, s)
		}
	}
}

func TestDKIMSignVerify(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	h := &Handler{outboundDomain: "example.com", dkimSelector: "sel1", dkimKey: key}

	raw := buildRFC822("alice@example.com", "Alice", []string{"bob@remote.test"}, "Signed hello", "body text", "Mon, 01 Jan 2026 00:00:00 +0000")
	signed, err := h.dkimSign(raw)
	if err != nil {
		t.Fatalf("dkimSign: %v", err)
	}
	if !bytes.Contains(signed, []byte("DKIM-Signature")) {
		t.Fatal("no DKIM-Signature header added")
	}

	// Verify offline by supplying the public key via LookupTXT.
	der, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	txt := "v=DKIM1; k=rsa; p=" + base64.StdEncoding.EncodeToString(der)
	verifs, err := dkim.VerifyWithOptions(bytes.NewReader(signed), &dkim.VerifyOptions{
		LookupTXT: func(domain string) ([]string, error) { return []string{txt}, nil },
	})
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(verifs) != 1 {
		t.Fatalf("expected 1 verification, got %d", len(verifs))
	}
	if verifs[0].Err != nil {
		t.Errorf("DKIM verification failed: %v", verifs[0].Err)
	}
	if verifs[0].Domain != "example.com" {
		t.Errorf("signed domain = %q", verifs[0].Domain)
	}
}

func TestDKIMSignNoKeyPassthrough(t *testing.T) {
	h := &Handler{} // no key configured
	raw := []byte("From: a@b\r\n\r\nbody")
	out, err := h.dkimSign(raw)
	if err != nil {
		t.Fatalf("dkimSign: %v", err)
	}
	if !bytes.Equal(out, raw) {
		t.Error("expected passthrough when no DKIM key configured")
	}
}
