package mobile

import (
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestFrameQueueDropsOldest(t *testing.T) {
	q := NewFrameQueue(1)
	if err := q.Submit(frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3, 4}}); err != nil {
		t.Fatal(err)
	}
	if err := q.Submit(frame.Frame{Width: 1, Height: 1, Stride: 4, Format: frame.PixelFormatRGBA8888, Data: []byte{5, 6, 7, 8}}); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-q.Frames():
		if got.Data[0] != 5 {
			t.Fatalf("expected newest frame, got %#v", got.Data)
		}
	default:
		t.Fatal("expected queued frame")
	}
}

func TestFrameQueueValidationAndClose(t *testing.T) {
	q := NewFrameQueue(0)
	if err := q.Submit(frame.Frame{}); err == nil {
		t.Fatal("expected invalid frame error")
	}
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
	if err := q.Submit(frame.Frame{Width: 1, Height: 1, Data: []byte{1}}); err == nil {
		t.Fatal("expected closed queue error")
	}
	if err := q.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestMobileServerLifecycleAndSubmitFrame(t *testing.T) {
	srv := NewServer()
	if err := srv.Start(0); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for srv.Addr() == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if srv.Addr() == "" {
		t.Fatal("server did not expose address")
	}
	if err := srv.SubmitFrame(1, 1, 4, 4, []byte{1, 2, 3, 4}); err != nil {
		t.Fatal(err)
	}
	if err := srv.SubmitFrame(1, 1, 2, 2, []byte{1, 2}); err == nil {
		t.Fatal("expected unsupported pixel stride")
	}
	if err := srv.Stop(); err != nil {
		t.Fatal(err)
	}
	if srv.Addr() != "" {
		t.Fatal("expected empty address after stop")
	}
	if err := srv.Stop(); err != nil {
		t.Fatal(err)
	}
}
