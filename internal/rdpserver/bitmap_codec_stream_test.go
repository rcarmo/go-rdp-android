package rdpserver

import (
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestBitmapCodecStreamingEnvGate(t *testing.T) {
	if bitmapCodecStreamingEnabledFromEnv() {
		t.Fatal("bitmap codec streaming should be disabled by default")
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_BITMAP_CODEC_STREAM", "1")
	if !bitmapCodecStreamingEnabledFromEnv() {
		t.Fatal("bitmap codec streaming should be enabled by env")
	}
}

func TestStreamExperimentalBitmapCodecUpdatesRecordsRawAndSavedBytes(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID", "9")
	t.Setenv("GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL", "-3")
	frames := make(chan frame.Frame, 1)
	frames <- frame.Frame{Width: 8, Height: 8, Stride: 32, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(8, 8, 0x11, 0x22, 0x33, 0xff)}
	close(frames)
	conn := &captureConn{}
	var pngFrames atomic.Int64
	var pngBytes atomic.Int64
	var pngRawBytes atomic.Int64
	var pngSavedBytes atomic.Int64
	metrics := serverMetrics{pngCodecFrames: &pngFrames, pngCodecBytes: &pngBytes, pngCodecRawBytes: &pngRawBytes, pngCodecSavedBytes: &pngSavedBytes}

	streamExperimentalBitmapCodecUpdates(conn, channelFrameSource{ch: frames}, confirmActiveCapabilities{}, 8, 8, metrics)

	if conn.writes.Load() <= 0 {
		t.Fatalf("writes = %d, want positive", conn.writes.Load())
	}
	if pngFrames.Load() != 1 {
		t.Fatalf("pngFrames = %d, want 1", pngFrames.Load())
	}
	if pngBytes.Load() <= 0 {
		t.Fatalf("pngBytes = %d, want positive", pngBytes.Load())
	}
	if pngRawBytes.Load() != 8*8*4 {
		t.Fatalf("pngRawBytes = %d, want %d", pngRawBytes.Load(), 8*8*4)
	}
	if pngSavedBytes.Load() <= 0 {
		t.Fatalf("pngSavedBytes = %d, want positive", pngSavedBytes.Load())
	}
}

func TestStreamExperimentalBitmapCodecUpdatesRecordsStopOnWriteFailure(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID", "9")
	frames := make(chan frame.Frame, 1)
	frames <- frame.Frame{Width: 2, Height: 2, Stride: 8, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(2, 2, 0xaa, 0xbb, 0xcc, 0xff)}
	close(frames)
	var streamStops atomic.Int64
	metrics := serverMetrics{bitmapCodecStreamStops: &streamStops}

	streamExperimentalBitmapCodecUpdates(failingWriteConn{}, channelFrameSource{ch: frames}, confirmActiveCapabilities{}, 2, 2, metrics)

	if got := streamStops.Load(); got != 1 {
		t.Fatalf("bitmapCodecStreamStops = %d, want 1", got)
	}
}

type failingWriteConn struct{}

func (failingWriteConn) Read(_ []byte) (int, error)         { return 0, net.ErrClosed }
func (failingWriteConn) Write(_ []byte) (int, error)        { return 0, errors.New("write failed") }
func (failingWriteConn) Close() error                       { return nil }
func (failingWriteConn) LocalAddr() net.Addr                { return nil }
func (failingWriteConn) RemoteAddr() net.Addr               { return nil }
func (failingWriteConn) SetDeadline(_ time.Time) error      { return nil }
func (failingWriteConn) SetReadDeadline(_ time.Time) error  { return nil }
func (failingWriteConn) SetWriteDeadline(_ time.Time) error { return nil }

type channelFrameSource struct{ ch <-chan frame.Frame }

func (s channelFrameSource) Frames() <-chan frame.Frame { return s.ch }
func (s channelFrameSource) Close() error               { return nil }

func solidRGBA(width, height int, r, g, b, a byte) []byte {
	data := make([]byte, width*height*4)
	for i := 0; i < len(data); i += 4 {
		data[i] = r
		data[i+1] = g
		data[i+2] = b
		data[i+3] = a
	}
	return data
}
