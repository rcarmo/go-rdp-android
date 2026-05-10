package rdpserver

import (
	"testing"
)

func TestNewValidatesSize(t *testing.T) {
	_, err := New(Config{Addr: ":0"}, nil, nil)
	if err == nil {
		t.Fatal("expected error for missing dimensions")
	}
}

func TestNewDefaultsAddr(t *testing.T) {
	s, err := New(Config{Width: 100, Height: 100}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.cfg.Addr != ":3390" {
		t.Fatalf("expected default addr :3390, got %q", s.cfg.Addr)
	}
}

func TestNewRejectsInvalidPolicy(t *testing.T) {
	if _, err := New(Config{Width: 100, Height: 100, Policy: AccessPolicy{AllowedCIDRs: []string{"bad"}}}, nil, nil); err == nil {
		t.Fatal("expected invalid policy error")
	}
}

func TestNewSetsTLSFingerprint(t *testing.T) {
	s, err := New(Config{Width: 100, Height: 100}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if s.TLSFingerprintSHA256() == "" {
		t.Fatal("expected TLS fingerprint")
	}
}
