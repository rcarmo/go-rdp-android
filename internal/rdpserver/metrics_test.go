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

	s.bitmapRLEFrames.Store(1)
	if got := s.GraphicsPath(); got != "bitmap-rle" {
		t.Fatalf("GraphicsPath() = %q, want bitmap-rle", got)
	}

	s.pngCodecFrames.Store(1)
	if got := s.GraphicsPath(); got != "png-codec" {
		t.Fatalf("GraphicsPath() = %q, want png-codec", got)
	}

	s.jpegCodecFrames.Store(1)
	if got := s.GraphicsPath(); got != "jpeg-codec" {
		t.Fatalf("GraphicsPath() = %q, want jpeg-codec", got)
	}

	s.nsCodecFrames.Store(1)
	if got := s.GraphicsPath(); got != "nscodec" {
		t.Fatalf("GraphicsPath() = %q, want nscodec", got)
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

func TestServerH264Status(t *testing.T) {
	s := &Server{}
	if got := s.H264Status(); got != "not-observed" {
		t.Fatalf("H264Status() = %q, want not-observed", got)
	}
	metrics := serverMetrics{h264Status: &s.h264Status}
	metrics.recordH264Status("client-avc420-not-advertised")
	if got := s.H264Status(); got != "client-avc420-not-advertised" {
		t.Fatalf("H264Status() = %q", got)
	}
}

func TestServerMetricsRecordBitmapRLEFrame(t *testing.T) {
	var framesSent atomic.Int64
	var bitmapBytes atomic.Int64
	var bitmapRLEFrames atomic.Int64
	var bitmapRLEBytes atomic.Int64
	var bitmapRLESavedBytes atomic.Int64
	metrics := serverMetrics{framesSent: &framesSent, bitmapBytes: &bitmapBytes, bitmapRLEFrames: &bitmapRLEFrames, bitmapRLEBytes: &bitmapRLEBytes, bitmapRLESavedBytes: &bitmapRLESavedBytes}

	update, ok := buildCompressedBitmapRLEUpdate([]bitmapRect{buildSolidBitmapRect(64, 64, 0xff336699)})
	if !ok {
		t.Fatal("buildCompressedBitmapRLEUpdate() ok = false")
	}
	metrics.recordBitmapFrame([][]byte{update})

	if got := framesSent.Load(); got != 1 {
		t.Fatalf("framesSent = %d, want 1", got)
	}
	if got := bitmapBytes.Load(); got != int64(len(update)) {
		t.Fatalf("bitmapBytes = %d, want %d", got, len(update))
	}
	if got := bitmapRLEFrames.Load(); got != 1 {
		t.Fatalf("bitmapRLEFrames = %d, want 1", got)
	}
	if got := bitmapRLEBytes.Load(); got <= 0 {
		t.Fatalf("bitmapRLEBytes = %d, want positive", got)
	}
	if got := bitmapRLESavedBytes.Load(); got <= 0 {
		t.Fatalf("bitmapRLESavedBytes = %d, want positive", got)
	}
}

func TestServerMetricsRecordNSCodecFrame(t *testing.T) {
	var framesSent atomic.Int64
	var nsCodecFrames atomic.Int64
	var nsCodecBytes atomic.Int64
	metrics := serverMetrics{framesSent: &framesSent, nsCodecFrames: &nsCodecFrames, nsCodecBytes: &nsCodecBytes}

	metrics.recordNSCodecFrame([][]byte{[]byte("abc"), []byte("defg")})

	if got := framesSent.Load(); got != 1 {
		t.Fatalf("framesSent = %d, want 1", got)
	}
	if got := nsCodecFrames.Load(); got != 1 {
		t.Fatalf("nsCodecFrames = %d, want 1", got)
	}
	if got := nsCodecBytes.Load(); got != 7 {
		t.Fatalf("nsCodecBytes = %d, want 7", got)
	}
}

func TestServerMetricsRecordJPEGCodecFrame(t *testing.T) {
	var framesSent atomic.Int64
	var jpegCodecFrames atomic.Int64
	var jpegCodecBytes atomic.Int64
	metrics := serverMetrics{framesSent: &framesSent, jpegCodecFrames: &jpegCodecFrames, jpegCodecBytes: &jpegCodecBytes}

	metrics.recordJPEGCodecFrame([][]byte{[]byte("jpeg")})

	if got := framesSent.Load(); got != 1 {
		t.Fatalf("framesSent = %d, want 1", got)
	}
	if got := jpegCodecFrames.Load(); got != 1 {
		t.Fatalf("jpegCodecFrames = %d, want 1", got)
	}
	if got := jpegCodecBytes.Load(); got != 4 {
		t.Fatalf("jpegCodecBytes = %d, want 4", got)
	}
}

func TestServerMetricsRecordPNGCodecFrame(t *testing.T) {
	var framesSent atomic.Int64
	var pngCodecFrames atomic.Int64
	var pngCodecBytes atomic.Int64
	metrics := serverMetrics{framesSent: &framesSent, pngCodecFrames: &pngCodecFrames, pngCodecBytes: &pngCodecBytes}

	metrics.recordPNGCodecFrame([][]byte{[]byte("png")})

	if got := framesSent.Load(); got != 1 {
		t.Fatalf("framesSent = %d, want 1", got)
	}
	if got := pngCodecFrames.Load(); got != 1 {
		t.Fatalf("pngCodecFrames = %d, want 1", got)
	}
	if got := pngCodecBytes.Load(); got != 3 {
		t.Fatalf("pngCodecBytes = %d, want 3", got)
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
