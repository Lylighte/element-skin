package util

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("no entropy")
}

func TestUUIDNoDashFromReaderSetsVersionAndVariant(t *testing.T) {
	id, err := uuidNoDashFromReader(strings.NewReader("abcdefghijklmnop"))
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 32 {
		t.Fatalf("uuid should be 32 hex chars, got %q", id)
	}
	if id[12] != '4' {
		t.Fatalf("uuid version should be 4, got %q from %s", id[12], id)
	}
	if !strings.Contains("89ab", string(id[16])) {
		t.Fatalf("uuid variant should be RFC 4122, got %q from %s", id[16], id)
	}
}

func TestGenerateUUIDNoDashReturnsRFC4122Version4Shape(t *testing.T) {
	id, err := GenerateUUIDNoDash()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := hex.DecodeString(id)
	if err != nil || len(decoded) != 16 {
		t.Fatalf("GenerateUUIDNoDash produced non-hex UUID: id=%q decoded=%x err=%v", id, decoded, err)
	}
	if id[12] != '4' {
		t.Fatalf("GenerateUUIDNoDash version nibble=%q, want 4 in %s", id[12], id)
	}
	if !strings.Contains("89ab", string(id[16])) {
		t.Fatalf("GenerateUUIDNoDash variant nibble=%q, want RFC 4122 in %s", id[16], id)
	}
}

func TestUUIDNoDashFromReaderReturnsEntropyError(t *testing.T) {
	if _, err := uuidNoDashFromReader(failingReader{}); err == nil || !strings.Contains(err.Error(), "no entropy") {
		t.Fatalf("expected entropy error, got %v", err)
	}
}

func TestOfflineUUIDNoDashIsStableAndDashedInputStrips(t *testing.T) {
	a := OfflineUUIDNoDash("Steve")
	b := OfflineUUIDNoDash("Steve")
	if a != b || len(a) != 32 {
		t.Fatalf("offline uuid should be stable 32 hex chars: %q %q", a, b)
	}
	if StripUUIDDashes("12345678-1234-1234-1234-123456789abc") != "12345678123412341234123456789abc" {
		t.Fatal("StripUUIDDashes did not remove dashes exactly")
	}
}
