package rdpserver

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServerListenAddrAndClose(t *testing.T) {
	srv, err := New(Config{Addr: "127.0.0.1:0", Width: 320, Height: 240}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- srv.Listen(ctx) }()

	deadline := time.Now().Add(time.Second)
	for srv.Addr() == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == nil {
		t.Fatal("server did not expose listener address")
	}
	if _, err := net.DialTimeout("tcp", srv.Addr().String(), time.Second); err != nil {
		t.Fatal(err)
	}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
}

func TestServerCloseWithoutListener(t *testing.T) {
	srv, err := New(Config{Width: 320, Height: 240}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
}
