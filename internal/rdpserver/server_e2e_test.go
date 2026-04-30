package rdpserver

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestServerLoopbackInitialHandshakeAndMCSProbe(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv, err := New(Config{Width: 800, Height: 600}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ctx, ln) }()

	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	if err := sendTestX224ConnectionRequest(conn); err != nil {
		t.Fatal(err)
	}
	resp, err := readTPKT(conn)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp) < 15 || resp[1] != x224TypeConnectionConfirm || resp[7] != rdpNegResp {
		t.Fatalf("unexpected X.224 confirm: %x", resp)
	}

	if err := sendTestMCSConnectInitial(conn); err != nil {
		t.Fatal(err)
	}
	// The MVP server reads the MCS envelope then closes without response.
	buf := make([]byte, 1)
	_, _ = conn.Read(buf)

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}

func sendTestX224ConnectionRequest(conn net.Conn) error {
	neg := make([]byte, 8)
	neg[0] = rdpNegReq
	binary.LittleEndian.PutUint16(neg[2:4], 8)
	binary.LittleEndian.PutUint32(neg[4:8], protocolSSL)
	userData := append([]byte("Cookie: mstshash=test\r\n"), neg...)
	li := byte(6 + len(userData))
	x224 := []byte{li, x224TypeConnectionRequest, 0, 0, 0, 1, 0}
	x224 = append(x224, userData...)
	return writeTPKT(conn, x224)
}

func sendTestMCSConnectInitial(conn net.Conn) error {
	// X.224 Data TPDU + minimal BER [APPLICATION 101] length 0.
	return writeTPKT(conn, []byte{0x02, x224TypeData, 0x80, 0x7f, 0x65, 0x00})
}
