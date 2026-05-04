package rdpserver

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"testing"
)

func TestTLSPublicKeyFromConfigUsesSubjectPublicKey(t *testing.T) {
	cfg, err := defaultTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	got := tlsPublicKeyFromConfig(cfg)
	if len(got) == 0 {
		t.Fatal("expected TLS public key bytes")
	}
	cert, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, cert.RawSubjectPublicKeyInfo) {
		t.Fatal("pubKeyAuth binding must use SubjectPublicKey, not full SubjectPublicKeyInfo")
	}
	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("expected RSA public key, got %T", cert.PublicKey)
	}
	want := x509.MarshalPKCS1PublicKey(rsaPub)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected subject public key bytes: got %d bytes want %d bytes", len(got), len(want))
	}
}

func TestExtractSubjectPublicKeyRejectsInvalidSPKI(t *testing.T) {
	if _, err := extractSubjectPublicKey([]byte{0x30, 0x03, 0x01, 0x02, 0x03}); err == nil {
		t.Fatal("expected invalid SPKI to fail")
	}
}
