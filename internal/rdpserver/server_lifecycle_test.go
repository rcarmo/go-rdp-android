package rdpserver

import (
	"context"
	"errors"
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
	conn, err := net.DialTimeout("tcp", srv.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	deadline = time.Now().Add(time.Second)
	for srv.ActiveConnections() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := srv.ActiveConnections(); got != 1 {
		t.Fatalf("expected one active connection, got %d", got)
	}
	if err := srv.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected listen error after close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop")
	}
	if srv.Addr() != nil {
		t.Fatalf("expected nil addr after close, got %v", srv.Addr())
	}
}

func TestServerAddrClearedOnContextCancel(t *testing.T) {
	srv, err := New(Config{Addr: "127.0.0.1:0", Width: 320, Height: 240}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- srv.Listen(ctx) }()

	deadline := time.Now().Add(time.Second)
	for srv.Addr() == nil && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == nil {
		t.Fatal("server did not expose listener address")
	}

	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("server did not stop on context cancel")
	}
	if srv.Addr() != nil {
		t.Fatalf("expected nil addr after context cancel, got %v", srv.Addr())
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

func TestChooseSessionDesktopSizePrefersClientCore(t *testing.T) {
	w, h := chooseSessionDesktopSize(1280, 720, 1920, 1080)
	if w != 1920 || h != 1080 {
		t.Fatalf("unexpected chosen desktop size %dx%d", w, h)
	}
}

func TestChooseSessionDesktopSizeClampsClientCore(t *testing.T) {
	w, h := chooseSessionDesktopSize(1280, 720, 16, 20000)
	if w != 64 || h != 8192 {
		t.Fatalf("unexpected clamped desktop size %dx%d", w, h)
	}
}
