package rdpserver

import (
	"bytes"
	"encoding/binary"
	"errors"
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

func TestReadTPKTIgnoresFastPathPacket(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte{0x0c, 0x06, 0xaa, 0xbb, 0xcc, 0xdd})
	if _, err := readTPKT(&buf); !errors.Is(err, errFastPathPDU) {
		t.Fatalf("expected fast-path sentinel, got %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("fast-path packet was not drained, remaining=%d", buf.Len())
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

func TestSelectNegotiatedProtocol(t *testing.T) {
	cases := []struct {
		name      string
		requested uint32
		want      uint32
	}{
		{name: "rdp-only", requested: protocolRDP, want: protocolRDP},
		{name: "ssl", requested: protocolSSL, want: protocolSSL},
		{name: "hybrid", requested: protocolHybrid, want: protocolHybrid},
		{name: "ssl+hybrid", requested: protocolSSL | protocolHybrid, want: protocolHybrid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := selectNegotiatedProtocol(tc.requested); got != tc.want {
				t.Fatalf("selected protocol = 0x%08x, want 0x%08x", got, tc.want)
			}
		})
	}
}

func TestSelectNegotiatedProtocolWithMode(t *testing.T) {
	cases := []struct {
		name      string
		requested uint32
		mode      SecurityMode
		want      uint32
		wantErr   bool
	}{
		{name: "rdp-only policy", requested: protocolSSL | protocolHybrid, mode: SecurityModeRDPOnly, want: protocolRDP},
		{name: "tls-only policy", requested: protocolSSL, mode: SecurityModeTLSOnly, want: protocolSSL},
		{name: "tls-only with hybrid request", requested: protocolHybrid, mode: SecurityModeTLSOnly, want: protocolSSL},
		{name: "tls-only rejected when tls absent", requested: protocolRDP, mode: SecurityModeTLSOnly, wantErr: true},
		{name: "nla-required", requested: protocolSSL | protocolHybrid, mode: SecurityModeNLARequired, want: protocolHybrid},
		{name: "nla-required rejected", requested: protocolSSL, mode: SecurityModeNLARequired, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := selectNegotiatedProtocolWithMode(tc.requested, tc.mode)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("selected protocol = 0x%08x, want 0x%08x", got, tc.want)
			}
		})
	}
}

func TestPerformInitialHandshake(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	errCh := make(chan error, 1)
	infoCh := make(chan *HandshakeInfo, 1)
	go func() {
		info, _, err := performInitialHandshake(server)
		if err != nil {
			errCh <- err
			return
		}
		infoCh <- info
	}()

	neg := make([]byte, 8)
	neg[0] = rdpNegReq
	binary.LittleEndian.PutUint16(neg[2:4], 8)
	binary.LittleEndian.PutUint32(neg[4:8], protocolRDP)
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
