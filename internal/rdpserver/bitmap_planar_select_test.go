package rdpserver

import (
	"sync/atomic"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestClassicBitmapPlanarEnvGate(t *testing.T) {
	if classicBitmapPlanarEnabledFromEnv() {
		t.Fatal("classic bitmap planar unexpectedly enabled by default")
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_BITMAP_PLANAR", "1")
	if !classicBitmapPlanarEnabledFromEnv() {
		t.Fatal("classic bitmap planar env gate not enabled")
	}
}

func TestWriteInitialBitmapUpdateUsesClassicBitmapPlanarWhenEnabled(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_BITMAP_PLANAR", "1")
	frames := make(chan frame.Frame, 1)
	frames <- solidRGBAFrameForClassicPlanarTest(16, 16, 0x10, 0x20, 0x30)
	close(frames)
	var bitmapBytes atomic.Int64
	metrics := serverMetrics{bitmapBytes: &bitmapBytes}
	conn := &captureConn{}

	if err := writeInitialBitmapUpdate(conn, channelFrameSource{ch: frames}, 16, 16, confirmActiveCapabilities{}, metrics); err != nil {
		t.Fatalf("writeInitialBitmapUpdate: %v", err)
	}
	if conn.writes.Load() <= 0 {
		t.Fatalf("writes = %d, want positive", conn.writes.Load())
	}
	if bitmapBytes.Load() <= 0 {
		t.Fatalf("bitmapBytes = %d, want positive", bitmapBytes.Load())
	}
}
