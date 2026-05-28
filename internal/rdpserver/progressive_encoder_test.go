package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestProductionProgressiveEncoder(t *testing.T) {
	enc := productionProgressiveEncoder{}
	src := frame.Frame{Width: 16, Height: 16, Stride: 64, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(16, 16, 0x11, 0x22, 0x33, 0xff)}
	payload, ok := enc.EncodeRDPGFX(src, 16, 16)
	if !ok || len(payload) == 0 {
		t.Fatalf("EncodeRDPGFX len=%d ok=%t", len(payload), ok)
	}
	parsed, ok := parseProgressivePayload(payload)
	if !ok {
		t.Fatalf("parseProgressivePayload failed for %x", payload)
	}
	if parsed.Width != 16 || parsed.Height != 16 || parsed.LayerCount != 1 || len(parsed.RegionRects) != 1 {
		t.Fatalf("unexpected parsed payload: %#v", parsed)
	}
	if got, want := len(parsed.Data), 16*16*2; got != want {
		t.Fatalf("data len=%d want=%d", got, want)
	}
}

func TestProductionProgressiveEncoderRejectsInvalid(t *testing.T) {
	enc := productionProgressiveEncoder{}
	if payload, ok := enc.EncodeRDPGFX(frame.Frame{}, 0, 0); ok || payload != nil {
		t.Fatal("expected invalid frame rejection")
	}
	if payload, ok := enc.EncodeRDPGFX(frame.Frame{Width: 1, Height: 1, Stride: 3, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3}}, 1, 1); ok || payload != nil {
		t.Fatal("expected short data rejection")
	}
}
