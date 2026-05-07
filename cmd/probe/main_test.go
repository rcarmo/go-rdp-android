package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"image"
	"math/big"
	"net"
	"os"
	"path/filepath"
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

func TestBuildMCSConnectInitialWithDRDYNVC(t *testing.T) {
	wire := buildMCSConnectInitial(true)
	if len(wire) < 3 || wire[0] != 0x02 || wire[1] != 0xf0 || wire[2] != 0x80 {
		t.Fatalf("bad x224 wrapper: %x", wire)
	}
	if !bytes.Contains(wire, []byte{0x03, 0xc0}) {
		t.Fatalf("missing CS_NET block: %x", wire)
	}
	if !bytes.Contains(wire, []byte("drdynvc")) {
		t.Fatalf("missing drdynvc channel name: %x", wire)
	}
}

func TestScenePlanRequiresRDPEI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "scene.json")
	if err := os.WriteFile(path, []byte(`[
  {"name":"a","actions":[{"type":"tap"}]},
  {"name":"b","actions":[{"type":"rdpei-tap"}]}
]`), 0o644); err != nil {
		t.Fatal(err)
	}
	requires, err := scenePlanRequiresRDPEI(path)
	if err != nil {
		t.Fatal(err)
	}
	if !requires {
		t.Fatal("expected rdpei requirement")
	}
}

func TestApplyBitmapUpdatePacketSupports24BPP(t *testing.T) {
	update := appendLE16(nil, 0x0001)
	update = appendLE16(update, 1)
	update = appendLE16(update, 0) // left
	update = appendLE16(update, 0) // top
	update = appendLE16(update, 0) // right
	update = appendLE16(update, 0) // bottom
	update = appendLE16(update, 1) // width
	update = appendLE16(update, 1) // height
	update = appendLE16(update, 24)
	update = appendLE16(update, 0)
	update = appendLE16(update, 4)
	update = append(update, 0x11, 0x22, 0x33, 0x00) // BGR + alignment padding

	share := buildShareDataPDU(0x02, update)
	mcsData := buildMCSSendDataRequest(probeMCSUserID, probeGlobalChannelID, share)
	pkt := append([]byte{0x02, 0xf0, 0x80, byte(26 << 2)}, mcsData...)

	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	if _, err := applyBitmapUpdatePacket(img, pkt); err != nil {
		t.Fatalf("applyBitmapUpdatePacket: %v", err)
	}
	got := img.RGBAAt(0, 0)
	if got.R != 0x33 || got.G != 0x22 || got.B != 0x11 || got.A != 0xff {
		t.Fatalf("unexpected pixel %#v", got)
	}
}

func TestRunRDPSceneActionsRequiresNegotiatedRDPEI(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	actions := []probeSceneAction{{Type: "rdpei-tap", X: 10, Y: 20}}
	if err := runRDPSceneActions(client, actions, 0); err == nil {
		t.Fatal("expected rdpei-tap negotiation error")
	}
}

func TestRegressionLicensingSkipFixture(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	license := regressionLicensePDU()
	demand := []byte{0x02, 0xf0, 0x80, 0x01, 0x02, 0x03}
	go func() {
		defer server.Close()
		_ = writeTPKT(server, license)
		_ = writeTPKT(server, demand)
	}()

	got, err := readDemandActiveOrSkipLicense(client)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, demand) {
		t.Fatalf("unexpected demand payload: %x", got)
	}
}

func TestRegressionNLALicensingSkipFixture(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	demand := []byte{0x02, 0xf0, 0x80, 0xaa, 0xbb, 0xcc}
	go func() {
		defer server.Close()
		_ = writeTPKT(server, regressionLicensePDU())
		_ = writeTPKT(server, regressionLicensePDU())
		_ = writeTPKT(server, demand)
	}()

	got, err := readDemandActiveOrSkipLicense(client)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, demand) {
		t.Fatalf("unexpected demand payload: %x", got)
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

func regressionLicensePDU() []byte {
	return []byte{0x10, 0x80, 0x00, 0x00, 0x00, 0xff, 0x03, 0x20}
}
