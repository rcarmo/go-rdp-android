package rdpserver

import (
	"testing"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func TestSelectRDPGFXEncodedPathPriority(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_CLEARCODEC", "1")
	t.Setenv("GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC", "1")
	t.Setenv("GO_RDP_ANDROID_ENABLE_AVC444", "1")
	t.Setenv("GO_RDP_ANDROID_ENABLE_AVC444V2", "1")
	encoder := rdpgfxFrameEncoderFunc(func(frame.Frame, int, int) ([]byte, bool) { return []byte{1}, true })
	metrics := serverMetrics{clearCodecEncoder: encoder, progressiveEncoder: encoder, avc444Encoder: encoder, avc444v2Encoder: encoder}
	cap := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion104, Flags: 0}
	selected, ok := selectRDPGFXEncodedPath(cap, metrics)
	if !ok || selected.Name != "rdpgfx-clearcodec" || selected.CodecID != rdpgfxCodecClearCodec {
		t.Fatalf("selected = %#v ok=%t, want clearcodec first", selected, ok)
	}
}

func TestSelectRDPGFXEncodedPathAVC444Fallback(t *testing.T) {
	t.Setenv("GO_RDP_ANDROID_ENABLE_AVC444", "1")
	encoder := rdpgfxFrameEncoderFunc(func(frame.Frame, int, int) ([]byte, bool) { return []byte{1}, true })
	metrics := serverMetrics{avc444Encoder: encoder}
	cap := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion10, Flags: 0}
	selected, ok := selectRDPGFXEncodedPath(cap, metrics)
	if !ok || selected.Name != "rdpgfx-avc444" || selected.CodecID != rdpgfxCodecAVC444 {
		t.Fatalf("selected = %#v ok=%t, want avc444", selected, ok)
	}
}
