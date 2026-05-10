package rdpserver

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TLSSettings controls certificate persistence and rotation behavior.
type TLSSettings struct {
	CertFile      string
	KeyFile       string
	RotateOnStart bool
	CommonName    string
}

var defaultTLSConfigOnce struct {
	sync.Once
	cfg *tls.Config
	err error
}

func defaultTLSConfig() (*tls.Config, error) {
	defaultTLSConfigOnce.Do(func() {
		cert, err := generateSelfSignedCertWithCommonName("go-rdp-android")
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

func resolveTLSConfig(settings TLSSettings) (*tls.Config, string, error) {
	if settings.CertFile == "" && settings.KeyFile == "" {
		cfg, err := defaultTLSConfig()
		if err != nil {
			return nil, "", err
		}
		fingerprint, err := tlsCertificateFingerprintSHA256(cfg)
		if err != nil {
			return nil, "", err
		}
		return cfg, fingerprint, nil
	}
	if settings.CertFile == "" || settings.KeyFile == "" {
		return nil, "", fmt.Errorf("tls cert and key paths must be set together")
	}
	commonName := settings.CommonName
	if commonName == "" {
		commonName = "go-rdp-android"
	}
	certFile := settings.CertFile
	keyFile := settings.KeyFile
	if settings.RotateOnStart || !fileExists(certFile) || !fileExists(keyFile) {
		cert, err := generateSelfSignedCertWithCommonName(commonName)
		if err != nil {
			return nil, "", err
		}
		certPEM, keyPEM, err := encodeTLSCertificatePEM(cert)
		if err != nil {
			return nil, "", err
		}
		if err := os.MkdirAll(filepath.Dir(certFile), 0o700); err != nil {
			return nil, "", err
		}
		if err := os.MkdirAll(filepath.Dir(keyFile), 0o700); err != nil {
			return nil, "", err
		}
		if err := os.WriteFile(certFile, certPEM, 0o600); err != nil {
			return nil, "", err
		}
		if err := os.WriteFile(keyFile, keyPEM, 0o600); err != nil {
			return nil, "", err
		}
	}
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, "", err
	}
	cfg := &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
	fingerprint, err := tlsCertificateFingerprintSHA256(cfg)
	if err != nil {
		return nil, "", err
	}
	return cfg, fingerprint, nil
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

func tlsCertificateFingerprintSHA256(cfg *tls.Config) (string, error) {
	if cfg == nil || len(cfg.Certificates) == 0 || len(cfg.Certificates[0].Certificate) == 0 {
		return "", fmt.Errorf("missing tls certificate")
	}
	sum := sha256.Sum256(cfg.Certificates[0].Certificate[0])
	return hex.EncodeToString(sum[:]), nil
}

func encodeTLSCertificatePEM(cert tls.Certificate) ([]byte, []byte, error) {
	if len(cert.Certificate) == 0 {
		return nil, nil, fmt.Errorf("missing certificate chain")
	}
	priv, ok := cert.PrivateKey.(*rsa.PrivateKey)
	if !ok {
		return nil, nil, fmt.Errorf("unsupported private key type %T", cert.PrivateKey)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Certificate[0]})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)})
	return certPEM, keyPEM, nil
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
	return generateSelfSignedCertWithCommonName("go-rdp-android")
}

func generateSelfSignedCertWithCommonName(commonName string) (tls.Certificate, error) {
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
			CommonName: commonName,
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

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}
