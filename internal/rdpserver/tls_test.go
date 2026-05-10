package rdpserver

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"path/filepath"
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

func TestTLSPublicKeyCandidatesFromConfig(t *testing.T) {
	cfg, err := defaultTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	candidates := tlsPublicKeyCandidatesFromConfig(cfg)
	if len(candidates) < 4 {
		t.Fatalf("expected SubjectPublicKey variants, SubjectPublicKeyInfo, and cert candidates, got %d", len(candidates))
	}
	cert, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates[1]) != len(candidates[0])+1 || candidates[1][0] != 0 || !bytes.Equal(candidates[1][1:], candidates[0]) {
		t.Fatal("second candidate should be BIT STRING content including unused-bits prefix")
	}
	if !bytes.Equal(candidates[2], cert.RawSubjectPublicKeyInfo) {
		t.Fatal("third candidate should be SubjectPublicKeyInfo for compatibility fallback")
	}
	if !bytes.Equal(candidates[3], cert.Raw) {
		t.Fatal("fourth candidate should be certificate DER for compatibility fallback")
	}
}

func TestExtractSubjectPublicKeyRejectsInvalidSPKI(t *testing.T) {
	if _, err := extractSubjectPublicKey([]byte{0x30, 0x03, 0x01, 0x02, 0x03}); err == nil {
		t.Fatal("expected invalid SPKI to fail")
	}
}

func TestResolveTLSConfigPersistentAndRotate(t *testing.T) {
	dir := t.TempDir()
	certPath := filepath.Join(dir, "server.crt")
	keyPath := filepath.Join(dir, "server.key")

	cfg, fp1, err := resolveTLSConfig(TLSSettings{CertFile: certPath, KeyFile: keyPath, CommonName: "test-cn"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg == nil || fp1 == "" {
		t.Fatal("expected tls config and fingerprint")
	}
	cfg2, fp2, err := resolveTLSConfig(TLSSettings{CertFile: certPath, KeyFile: keyPath, CommonName: "test-cn"})
	if err != nil {
		t.Fatal(err)
	}
	if cfg2 == nil || fp2 == "" {
		t.Fatal("expected tls config and fingerprint")
	}
	if fp1 != fp2 {
		t.Fatalf("expected persisted cert fingerprint to match: %s vs %s", fp1, fp2)
	}
	_, fp3, err := resolveTLSConfig(TLSSettings{CertFile: certPath, KeyFile: keyPath, CommonName: "test-cn", RotateOnStart: true})
	if err != nil {
		t.Fatal(err)
	}
	if fp3 == fp2 {
		t.Fatal("expected rotated certificate fingerprint to change")
	}
}

func TestResolveTLSConfigRequiresCertAndKeyTogether(t *testing.T) {
	if _, _, err := resolveTLSConfig(TLSSettings{CertFile: "only-cert"}); err == nil {
		t.Fatal("expected cert/key pair validation error")
	}
}
