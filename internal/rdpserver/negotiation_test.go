package rdpserver

import (
	"bytes"
	"encoding/binary"
	"net"
	"testing"
)

func TestTPKTRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	payload := []byte{1, 2, 3}
	if err := writeTPKT(&buf, payload); err != nil {
		t.Fatal(err)
	}
	got, err := readTPKT(&buf)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("got %x want %x", got, payload)
	}
}

func TestParseNegotiationUserData(t *testing.T) {
	neg := make([]byte, 8)
	neg[0] = rdpNegReq
	binary.LittleEndian.PutUint16(neg[2:4], 8)
	binary.LittleEndian.PutUint32(neg[4:8], protocolSSL)
	info := parseNegotiationUserData(append([]byte("Cookie: mstshash=user\r\n"), neg...))
	if info.Cookie != "mstshash=user" {
		t.Fatalf("unexpected cookie %q", info.Cookie)
	}
	if info.RequestedProtocols != protocolSSL {
		t.Fatalf("unexpected requested protocols 0x%x", info.RequestedProtocols)
	}
}

func TestPerformInitialHandshake(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	errCh := make(chan error, 1)
	infoCh := make(chan *HandshakeInfo, 1)
	go func() {
		info, err := performInitialHandshake(server)
		if err != nil {
			errCh <- err
			return
		}
		infoCh <- info
	}()

	neg := make([]byte, 8)
	neg[0] = rdpNegReq
	binary.LittleEndian.PutUint16(neg[2:4], 8)
	binary.LittleEndian.PutUint32(neg[4:8], protocolSSL)
	userData := append([]byte("Cookie: mstshash=user\r\n"), neg...)
	li := byte(6 + len(userData))
	x224 := []byte{li, x224TypeConnectionRequest, 0, 0, 0, 1, 0}
	x224 = append(x224, userData...)
	if err := writeTPKT(client, x224); err != nil {
		t.Fatal(err)
	}

	resp, err := readTPKT(client)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp) < 15 || resp[1] != x224TypeConnectionConfirm || resp[7] != rdpNegResp {
		t.Fatalf("unexpected response: %x", resp)
	}

	select {
	case err := <-errCh:
		t.Fatal(err)
	case info := <-infoCh:
		if info.Cookie != "mstshash=user" || info.SelectedProtocol != protocolRDP {
			t.Fatalf("unexpected info: %#v", info)
		}
	}
}
