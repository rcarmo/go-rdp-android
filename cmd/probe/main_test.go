package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"math/big"
	"net"
	"testing"
	"time"
)

func TestTPKTReadWrite(t *testing.T) {
	buf := new(bytes.Buffer)
	payload := []byte{1, 2, 3}
	if err := writeTPKT(buf, payload); err != nil {
		t.Fatal(err)
	}
	got, err := readTPKT(buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %x want %x", got, payload)
	}
}

func TestProbeBuilders(t *testing.T) {
	if got := encodePERInteger16(1003, 1001); binary.BigEndian.Uint16(got) != 2 {
		t.Fatalf("bad PER integer: %x", got)
	}
	if got := encodePERLength(0x80); len(got) != 2 || got[0] != 0x80 || got[1] != 0x80 {
		t.Fatalf("bad PER length: %x", got)
	}
	mcs := buildMCSSendDataRequest(1001, 1003, []byte{1, 2})
	if len(mcs) < 8 || mcs[4] != 0x70 {
		t.Fatalf("bad MCS send data: %x", mcs)
	}
	share := buildShareDataPDU(0x1f, synchronizePayload())
	if len(share) < 18 || share[2] != 0x17 || share[14] != 0x1f {
		t.Fatalf("bad share data: %x", share)
	}
	confirm := buildConfirmActivePDU(0x000103ea, 1001)
	if len(confirm) < 20 || confirm[2] != 0x13 {
		t.Fatalf("bad confirm active: %x", confirm)
	}
}

func TestExtractSubjectPublicKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.CreateCertificate(rand.Reader, &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "probe-test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}, &x509.Certificate{SerialNumber: big.NewInt(1)}, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	got, err := extractSubjectPublicKey(cert.RawSubjectPublicKeyInfo)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(got, cert.RawSubjectPublicKeyInfo) {
		t.Fatal("expected SubjectPublicKey, not SubjectPublicKeyInfo")
	}
	rsaPub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		t.Fatalf("expected RSA public key, got %T", cert.PublicKey)
	}
	want := x509.MarshalPKCS1PublicKey(rsaPub)
	if !bytes.Equal(got, want) {
		t.Fatalf("unexpected public key length: got %d want %d", len(got), len(want))
	}
}

func TestSendHelpersWriteExpectedPackets(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	done := make(chan error, 3)
	go func() { done <- sendX224ConnectionRequest(client, false) }()
	payload, err := readTPKT(server)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) < 8 || payload[1] != 0xe0 {
		t.Fatalf("bad X.224 request: %x", payload)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	go func() { done <- sendMCSConnectInitial(client) }()
	payload, err = readTPKT(server)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(payload, []byte{0x02, 0xf0, 0x80, 0x7f, 0x65, 0x00}) {
		t.Fatalf("bad MCS initial: %x", payload)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	go func() { done <- sendShareData(client, 0x14, controlPayload(0x0004)) }()
	payload, err = readTPKT(server)
	if err != nil {
		t.Fatal(err)
	}
	if len(payload) < 12 || payload[1] != 0xf0 || payload[3] != byte(25<<2) {
		t.Fatalf("bad share wrapper: %x", payload)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestReadTPKTShortRead(t *testing.T) {
	_, err := readTPKT(bytes.NewReader([]byte{3, 0}))
	if err == nil {
		t.Fatal("expected short read error")
	}
}

func TestReadAndPrintReadsPacket(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	go func() {
		_ = writeTPKT(server, []byte{1, 2, 3})
	}()
	done := make(chan struct{})
	go func() {
		readAndPrint(client, "test")
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("readAndPrint did not return")
	}
}
