package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestProductionProgressiveV2Encoder(t *testing.T) {
	enc := productionProgressiveV2Encoder{}
	src := frame.Frame{Width: 16, Height: 16, Stride: 64, Format: frame.PixelFormatRGBA8888, Data: solidRGBA(16, 16, 0x11, 0x22, 0x33, 0xff)}
	payload, ok := enc.EncodeRDPGFX(src, 16, 16)
	if !ok || len(payload) == 0 {
		t.Fatalf("EncodeRDPGFX len=%d ok=%t", len(payload), ok)
	}
	parsed, ok := parseProgressivePayload(payload)
	if !ok {
		t.Fatalf("parseProgressivePayload failed for %x", payload)
	}
	if parsed.Quant&0x80 == 0 {
		t.Fatalf("V2 quant marker not set: 0x%02x", parsed.Quant)
	}
}

func TestSelectRDPGFXEncodedPathPrefersProgressiveV2WhenCapable(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC", "1")
	v1 := rdpgfxFrameEncoderFunc(func(frame.Frame, int, int) ([]byte, bool) { return []byte{1}, true })
	v2 := rdpgfxFrameEncoderFunc(func(frame.Frame, int, int) ([]byte, bool) { return []byte{2}, true })
	selected, ok := selectRDPGFXEncodedPath(rdpgfxCapabilitySet{Version: rdpgfxCapsVersion104}, serverMetrics{progressiveEncoder: v1, progressiveV2Encoder: v2})
	if !ok || selected.CodecID != rdpgfxCodecCAProgressiveV2 || selected.Name != "rdpgfx-progressive-v2" {
		t.Fatalf("selected = %#v ok=%t", selected, ok)
	}
}
