package rdpserver

import (
	"sync/atomic"
	"testing"
)

func TestServerGraphicsPathPrefersH264(t *testing.T) {
	s := &Server{}
	if got := s.GraphicsPath(); got != "pending" {
		t.Fatalf("GraphicsPath() = %q, want pending", got)
	}

	s.bitmapBytes.Store(128)
	if got := s.GraphicsPath(); got != "bitmap-fallback" {
		t.Fatalf("GraphicsPath() = %q, want bitmap-fallback", got)
	}

	s.rdpgfxFrames.Store(1)
	if got := s.GraphicsPath(); got != "rdpgfx-planar" {
		t.Fatalf("GraphicsPath() = %q, want rdpgfx-planar", got)
	}

	s.h264Frames.Store(1)
	if got := s.GraphicsPath(); got != "h264-avc" {
		t.Fatalf("GraphicsPath() = %q, want h264-avc", got)
	}
}

func TestServerMetricsRecordH264Frame(t *testing.T) {
	var framesSent atomic.Int64
	var h264Frames atomic.Int64
	var h264Bytes atomic.Int64
	metrics := serverMetrics{framesSent: &framesSent, h264Frames: &h264Frames, h264Bytes: &h264Bytes}

	metrics.recordH264Frame([][]byte{[]byte("abc"), []byte("de")})

	if got := framesSent.Load(); got != 1 {
		t.Fatalf("framesSent = %d, want 1", got)
	}
	if got := h264Frames.Load(); got != 1 {
		t.Fatalf("h264Frames = %d, want 1", got)
	}
	if got := h264Bytes.Load(); got != 5 {
		t.Fatalf("h264Bytes = %d, want 5", got)
	}
}
