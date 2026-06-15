package handler

import (
	"bytes"
	"testing"
)

func TestBackupGzipRoundTrip(t *testing.T) {
	orig := []byte(`{"columns":[],"tasks":[],"note":"` + string(bytes.Repeat([]byte("x"), 1000)) + `"}`)
	gz := gzipBytes(orig)
	if len(gz) < 2 || gz[0] != 0x1f || gz[1] != 0x8b {
		t.Fatalf("not gzip output")
	}
	if len(gz) >= len(orig) {
		t.Errorf("gzip did not shrink repetitive json: %d >= %d", len(gz), len(orig))
	}
	if back := gunzipIfNeeded(gz); !bytes.Equal(back, orig) {
		t.Errorf("gunzip round-trip mismatch")
	}
	// Legacy plain JSON passes through unchanged.
	if back := gunzipIfNeeded(orig); !bytes.Equal(back, orig) {
		t.Errorf("plain json should pass through unchanged")
	}
}
