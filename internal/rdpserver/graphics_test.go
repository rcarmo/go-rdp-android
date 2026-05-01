package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestBuildSolidBitmapUpdate(t *testing.T) {
	payload := buildSolidBitmapUpdate(8, 4, 0xff336699)
	rects, err := parseBitmapUpdateHeader(payload)
	if err != nil {
		t.Fatal(err)
	}
	if rects != 1 {
		t.Fatalf("expected one rectangle, got %d", rects)
	}
	// update header (4) + rect header (18) + 8*4*4 bytes.
	if want := 4 + 18 + 128; len(payload) != want {
		t.Fatalf("unexpected payload length: got %d want %d", len(payload), want)
	}
}

func TestBuildSolidBitmapUpdateClamp(t *testing.T) {
	payload := buildSolidBitmapUpdate(1000, 1000, 0)
	if want := 4 + 18 + 64*64*4; len(payload) != want {
		t.Fatalf("unexpected clamped payload length: got %d want %d", len(payload), want)
	}
}

func TestBuildFrameBitmapUpdateRGBA(t *testing.T) {
	payload, ok := buildFrameBitmapUpdate(frame.Frame{
		Width:  1,
		Height: 1,
		Stride: 4,
		Format: frame.PixelFormatRGBA8888,
		Data:   []byte{0x11, 0x22, 0x33, 0x44},
	})
	if !ok {
		t.Fatal("expected frame conversion")
	}
	if got := payload[len(payload)-4:]; got[0] != 0x33 || got[1] != 0x22 || got[2] != 0x11 || got[3] != 0x44 {
		t.Fatalf("unexpected BGRA bytes: %x", got)
	}
}

func TestBuildFrameBitmapUpdatesTilesLargeFrame(t *testing.T) {
	width, height := 200, 90
	data := make([]byte, width*height*4)
	for i := range data {
		data[i] = byte(i)
	}
	updates, ok := buildFrameBitmapUpdates(frame.Frame{
		Width:  width,
		Height: height,
		Stride: width * 4,
		Format: frame.PixelFormatBGRA8888,
		Data:   data,
	})
	if !ok {
		t.Fatal("expected tiled frame conversion")
	}
	if len(updates) != 3 {
		t.Fatalf("expected 3 horizontal tiles, got %d", len(updates))
	}
	for _, update := range updates {
		rects, err := parseBitmapUpdateHeader(update)
		if err != nil {
			t.Fatal(err)
		}
		if rects != 1 {
			t.Fatalf("expected one rect per update, got %d", rects)
		}
		if len(update) > 60000 {
			t.Fatalf("update too large for safe TPKT envelope: %d", len(update))
		}
	}
}
