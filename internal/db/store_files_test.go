package db

import (
	"bytes"
	"strings"
	"testing"
)

func TestFileBlobCompression(t *testing.T) {
	s := newTestStore(t)

	// Compressible text blob: stored gzipped, returned intact.
	text := []byte(strings.Repeat("hello world this is compressible text\n", 500))
	id, err := s.SaveFile("notes.txt", text, "text/plain")
	if err != nil {
		t.Fatalf("SaveFile: %v", err)
	}
	var compressed int
	var stored []byte
	s.db.QueryRow("SELECT compressed, data FROM files WHERE id=?", id).Scan(&compressed, &stored)
	if compressed != 1 {
		t.Errorf("text blob should be stored compressed")
	}
	if len(stored) >= len(text) {
		t.Errorf("stored size %d not smaller than original %d", len(stored), len(text))
	}
	got, _, _, err := s.GetFile(id)
	if err != nil || !bytes.Equal(got, text) {
		t.Fatalf("GetFile round-trip failed: err=%v equal=%v", err, bytes.Equal(got, text))
	}

	// Already-compressed MIME: stored raw.
	jpeg := bytes.Repeat([]byte{0x12, 0x34, 0x56, 0x78}, 500)
	jid, _ := s.SaveFile("photo.jpg", jpeg, "image/jpeg")
	var jc int
	s.db.QueryRow("SELECT compressed FROM files WHERE id=?", jid).Scan(&jc)
	if jc != 0 {
		t.Errorf("jpeg should be stored raw, not compressed")
	}
	jgot, _, _, _ := s.GetFile(jid)
	if !bytes.Equal(jgot, jpeg) {
		t.Error("jpeg round-trip mismatch")
	}

	// Export yields the ORIGINAL (decompressed) bytes for portability.
	exp, err := s.ExportAll()
	if err != nil {
		t.Fatalf("ExportAll: %v", err)
	}
	for _, f := range exp.Files {
		if f.ID == id && !bytes.Equal(f.Data, text) {
			t.Error("exported file data is not the original (decompressed) bytes")
		}
	}
}
