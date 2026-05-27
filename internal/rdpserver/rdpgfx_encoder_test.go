package rdpserver

import (
	"encoding/binary"
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestBuildRDPGFXFrameUpdatePDUsUsesClearCodecEncoder(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_CLEARCODEC", "1")
	fr := frame.Frame{Width: 4, Height: 4, Stride: 16, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(4, 4, 0x11, 0x22, 0x33, 0xff)}
	metrics := serverMetrics{clearCodecEncoder: rdpgfxFrameEncoderFunc(func(src frame.Frame, width, height int) ([]byte, bool) {
		if src.Width != 4 || src.Height != 4 || width != 4 || height != 4 {
			t.Fatalf("encoder dimensions src=%dx%d desktop=%dx%d", src.Width, src.Height, width, height)
		}
		return []byte{1, 2, 3, 4}, true
	})}
	cap := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion8, Flags: 0}

	pdus, path, ok := buildRDPGFXFrameUpdatePDUs(0, 7, fr, 4, 4, metrics, cap)
	if !ok || path != "rdpgfx-clearcodec" || len(pdus) != 3 {
		t.Fatalf("buildRDPGFXFrameUpdatePDUs len=%d path=%q ok=%t", len(pdus), path, ok)
	}
	codecID, ok := wireToSurfaceCodecIDForTest(pdus[1])
	if !ok || codecID != rdpgfxCodecClearCodec {
		t.Fatalf("wire codec=0x%04x ok=%t", codecID, ok)
	}
}

func wireToSurfaceCodecIDForTest(pdu []byte) (uint16, bool) {
	if len(pdu) < 12 {
		return 0, false
	}
	return binary.LittleEndian.Uint16(pdu[10:12]), true
}

func TestBuildRDPGFXFrameUpdatePDUsUsesProgressiveEncoder(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC", "1")
	fr := frame.Frame{Width: 4, Height: 4, Stride: 16, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(4, 4, 0x11, 0x22, 0x33, 0xff)}
	metrics := serverMetrics{progressiveEncoder: rdpgfxFrameEncoderFunc(func(frame.Frame, int, int) ([]byte, bool) { return []byte{5, 6, 7, 8}, true })}
	cap := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion10, Flags: 0}

	pdus, path, ok := buildRDPGFXFrameUpdatePDUs(0, 7, fr, 4, 4, metrics, cap)
	if !ok || path != "rdpgfx-progressive" || len(pdus) != 3 {
		t.Fatalf("buildRDPGFXFrameUpdatePDUs len=%d path=%q ok=%t", len(pdus), path, ok)
	}
	codecID, ok := wireToSurfaceCodecIDForTest(pdus[1])
	if !ok || codecID != rdpgfxCodecCAProgressive {
		t.Fatalf("wire codec=0x%04x ok=%t", codecID, ok)
	}
}

func TestBuildRDPGFXFrameUpdatePDUsUsesAVC444Encoder(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_AVC444", "1")
	fr := frame.Frame{Width: 4, Height: 4, Stride: 16, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(4, 4, 0x11, 0x22, 0x33, 0xff)}
	metrics := serverMetrics{avc444Encoder: rdpgfxFrameEncoderFunc(func(frame.Frame, int, int) ([]byte, bool) { return []byte{9, 10, 11, 12}, true })}
	cap := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion10, Flags: 0}

	pdus, path, ok := buildRDPGFXFrameUpdatePDUs(0, 7, fr, 4, 4, metrics, cap)
	if !ok || path != "rdpgfx-avc444" || len(pdus) != 3 {
		t.Fatalf("buildRDPGFXFrameUpdatePDUs len=%d path=%q ok=%t", len(pdus), path, ok)
	}
	codecID, ok := wireToSurfaceCodecIDForTest(pdus[1])
	if !ok || codecID != rdpgfxCodecAVC444 {
		t.Fatalf("wire codec=0x%04x ok=%t", codecID, ok)
	}
}

func TestBuildRDPGFXFrameUpdatePDUsAVC444CapabilityGatedFallback(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_AVC444", "1")
	fr := frame.Frame{Width: 4, Height: 4, Stride: 16, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(4, 4, 0x11, 0x22, 0x33, 0xff)}
	metrics := serverMetrics{avc444Encoder: rdpgfxFrameEncoderFunc(func(frame.Frame, int, int) ([]byte, bool) { return []byte{9, 10, 11, 12}, true })}
	cap := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion8, Flags: 0}

	pdus, path, ok := buildRDPGFXFrameUpdatePDUs(0, 7, fr, 4, 4, metrics, cap)
	if !ok || path != "rdpgfx-planar" || len(pdus) != 3 {
		t.Fatalf("capability fallback len=%d path=%q ok=%t", len(pdus), path, ok)
	}
}

func TestBuildRDPGFXFrameUpdatePDUsUsesAVC444v2Encoder(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_AVC444V2", "1")
	fr := frame.Frame{Width: 4, Height: 4, Stride: 16, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(4, 4, 0x11, 0x22, 0x33, 0xff)}
	metrics := serverMetrics{avc444v2Encoder: rdpgfxFrameEncoderFunc(func(frame.Frame, int, int) ([]byte, bool) { return []byte{13, 14, 15, 16}, true })}
	cap := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion104, Flags: 0}

	pdus, path, ok := buildRDPGFXFrameUpdatePDUs(0, 7, fr, 4, 4, metrics, cap)
	if !ok || path != "rdpgfx-avc444v2" || len(pdus) != 3 {
		t.Fatalf("buildRDPGFXFrameUpdatePDUs len=%d path=%q ok=%t", len(pdus), path, ok)
	}
	codecID, ok := wireToSurfaceCodecIDForTest(pdus[1])
	if !ok || codecID != rdpgfxCodecAVC444v2 {
		t.Fatalf("wire codec=0x%04x ok=%t", codecID, ok)
	}
}

func TestBuildRDPGFXFrameUpdatePDUsFallsBackWhenClearCodecEncoderRejects(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_CLEARCODEC", "1")
	fr := frame.Frame{Width: 2, Height: 1, Stride: 8, Format: frame.PixelFormatRGBA8888, Data: []byte{1, 2, 3, 4, 5, 6, 7, 8}}
	metrics := serverMetrics{clearCodecEncoder: rdpgfxFrameEncoderFunc(func(frame.Frame, int, int) ([]byte, bool) { return nil, false })}
	cap := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion8, Flags: 0}

	pdus, path, ok := buildRDPGFXFrameUpdatePDUs(0, 7, fr, 2, 1, metrics, cap)
	if !ok || path != "rdpgfx-planar" || len(pdus) != 3 {
		t.Fatalf("fallback len=%d path=%q ok=%t", len(pdus), path, ok)
	}
}
