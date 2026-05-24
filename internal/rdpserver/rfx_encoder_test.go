package rdpserver

import (
	"sync/atomic"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
)

func TestWriteInitialBitmapUpdateUsesRFXEncoderWhenAvailable(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_RFX_CODEC", "1")
	frames := make(chan frame.Frame, 1)
	frames <- frame.Frame{Width: 4, Height: 4, Stride: 16, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(4, 4, 0x11, 0x22, 0x33, 0xff)}
	close(frames)
	var calls atomic.Int64
	var rfxFrames atomic.Int64
	var rfxBytes atomic.Int64
	var rfxRawBytes atomic.Int64
	var rfxSavedBytes atomic.Int64
	metrics := serverMetrics{rfxCodecFrames: &rfxFrames, rfxCodecBytes: &rfxBytes, rfxCodecRawBytes: &rfxRawBytes, rfxCodecSavedBytes: &rfxSavedBytes, rfxEncoder: rfxEncoderFunc(func(src frame.Frame, width, height int) ([]byte, bool) {
		calls.Add(1)
		if src.Width != 4 || src.Height != 4 || width != 4 || height != 4 {
			t.Fatalf("encoder dimensions src=%dx%d desktop=%dx%d", src.Width, src.Height, width, height)
		}
		return []byte{1, 2, 3, 4}, true
	})}
	caps := confirmActiveCapabilities{BitmapCodecs: bitmapCodecsCapabilityInfo{Present: true, Codecs: []bitmapCodecInfo{{GUID: rdpcodec.RemoteFXGUID, ID: 4, Name: rdpcodec.BitmapCodecNameRemoteFX}}}}
	conn := &captureConn{}

	if err := writeInitialBitmapUpdate(conn, channelFrameSource{ch: frames}, 4, 4, caps, metrics); err != nil {
		t.Fatalf("writeInitialBitmapUpdate: %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("encoder calls = %d, want 1", calls.Load())
	}
	if conn.writes.Load() <= 0 {
		t.Fatalf("writes = %d, want positive", conn.writes.Load())
	}
	if rfxFrames.Load() != 1 || rfxBytes.Load() <= 0 || rfxRawBytes.Load() != 64 || rfxSavedBytes.Load() == 0 {
		t.Fatalf("rfx metrics frames=%d bytes=%d raw=%d saved=%d, want frame/bytes/raw/saved evidence", rfxFrames.Load(), rfxBytes.Load(), rfxRawBytes.Load(), rfxSavedBytes.Load())
	}
}

func TestWriteInitialBitmapUpdateFallsBackWhenRFXEncoderRejects(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_RFX_CODEC", "1")
	frames := make(chan frame.Frame, 1)
	frames <- frame.Frame{Width: 2, Height: 2, Stride: 8, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(2, 2, 0x11, 0x22, 0x33, 0xff)}
	close(frames)
	var bitmapFrames atomic.Int64
	metrics := serverMetrics{bitmapBytes: &bitmapFrames, rfxEncoder: rfxEncoderFunc(func(frame.Frame, int, int) ([]byte, bool) { return nil, false })}
	caps := confirmActiveCapabilities{BitmapCodecs: bitmapCodecsCapabilityInfo{Present: true, Codecs: []bitmapCodecInfo{{GUID: rdpcodec.RemoteFXGUID, ID: 4, Name: rdpcodec.BitmapCodecNameRemoteFX}}}}
	conn := &captureConn{}

	if err := writeInitialBitmapUpdate(conn, channelFrameSource{ch: frames}, 2, 2, caps, metrics); err != nil {
		t.Fatalf("writeInitialBitmapUpdate: %v", err)
	}
	if conn.writes.Load() <= 0 {
		t.Fatalf("writes = %d, want fallback write", conn.writes.Load())
	}
}
