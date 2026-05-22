package rdpserver

import "testing"

func TestNegotiatedPNGCodecIDRequiresExplicitID(t *testing.T) {
	if id, ok := negotiatedPNGCodecID(); ok || id != 0 {
		t.Fatalf("negotiatedPNGCodecID without env = %d,%t", id, ok)
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID", "0")
	if id, ok := negotiatedPNGCodecID(); ok || id != 0 {
		t.Fatalf("negotiatedPNGCodecID zero = %d,%t", id, ok)
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID", "17")
	if id, ok := negotiatedPNGCodecID(); !ok || id != 17 {
		t.Fatalf("negotiatedPNGCodecID decimal = %d,%t", id, ok)
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID", "0x22")
	if id, ok := negotiatedPNGCodecID(); !ok || id != 0x22 {
		t.Fatalf("negotiatedPNGCodecID hex = %d,%t", id, ok)
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID", "300")
	if id, ok := negotiatedPNGCodecID(); ok || id != 0 {
		t.Fatalf("negotiatedPNGCodecID overflow = %d,%t", id, ok)
	}
}
