package main

import (
	"context"
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/rdpserver"
)

func TestRunStopsWhenContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, "127.0.0.1:0", 320, 240, false, 0, "", "", "", string(rdpserver.SecurityModeNegotiate), nil, nil)
	}()
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("run did not stop after context cancellation")
	}
}

func TestRunRejectsInvalidConfig(t *testing.T) {
	if err := run(context.Background(), "bad address with spaces", 320, 240, false, 0, "", "", "", string(rdpserver.SecurityModeNegotiate), nil, nil); err == nil {
		t.Fatal("expected listen error")
	}
}

func TestRunRejectsMixedPasswordAndHash(t *testing.T) {
	if err := run(context.Background(), "127.0.0.1:0", 320, 240, false, 0, "rui", "secret", "$2a$10$abc", string(rdpserver.SecurityModeNegotiate), nil, nil); err == nil {
		t.Fatal("expected mixed password/password-hash rejection")
	}
}
