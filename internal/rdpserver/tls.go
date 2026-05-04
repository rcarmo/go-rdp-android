package rdpserver

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"math/big"
	"sync"
	"time"
)

var defaultTLSConfigOnce struct {
	sync.Once
	cfg *tls.Config
	err error
}

func defaultTLSConfig() (*tls.Config, error) {
	defaultTLSConfigOnce.Do(func() {
		cert, err := generateSelfSignedCert()
		if err != nil {
			defaultTLSConfigOnce.err = err
			return
		}
		defaultTLSConfigOnce.cfg = &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
	})
	return defaultTLSConfigOnce.cfg, defaultTLSConfigOnce.err
}

func tlsPublicKeyFromConfig(cfg *tls.Config) []byte {
	candidates := tlsPublicKeyCandidatesFromConfig(cfg)
	if len(candidates) == 0 {
		return nil
	}
	return candidates[0]
}

func tlsPublicKeyCandidatesFromConfig(cfg *tls.Config) [][]byte {
	if cfg == nil || len(cfg.Certificates) == 0 || len(cfg.Certificates[0].Certificate) == 0 {
		return nil
	}
	cert, err := x509.ParseCertificate(cfg.Certificates[0].Certificate[0])
	if err != nil {
		return nil
	}
	var candidates [][]byte
	if pubKey, err := extractSubjectPublicKey(cert.RawSubjectPublicKeyInfo); err == nil && len(pubKey) > 0 {
		candidates = append(candidates, pubKey)
		candidates = append(candidates, append([]byte{0}, pubKey...))
	}
	if len(cert.RawSubjectPublicKeyInfo) > 0 {
		candidates = append(candidates, append([]byte(nil), cert.RawSubjectPublicKeyInfo...))
	}
	if len(cert.Raw) > 0 {
		candidates = append(candidates, append([]byte(nil), cert.Raw...))
	}
	return candidates
}

func extractSubjectPublicKey(rawSubjectPublicKeyInfo []byte) ([]byte, error) {
	var spki struct {
		Algorithm        pkix.AlgorithmIdentifier
		SubjectPublicKey asn1.BitString
	}
	if _, err := asn1.Unmarshal(rawSubjectPublicKeyInfo, &spki); err != nil {
		return nil, err
	}
	return append([]byte(nil), spki.SubjectPublicKey.Bytes...), nil
}

func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return tls.Certificate{}, err
	}
	tmpl := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: "go-rdp-android",
		},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return tls.X509KeyPair(certPEM, keyPEM)
}
