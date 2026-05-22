package rdpserver

import "testing"

func TestRDPGFXDeferredCodecCapabilityGates(t *testing.T) {
	cap8 := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion8}
	cap10 := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion10}
	cap104 := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion104}
	cap104AVCDisabled := rdpgfxCapabilitySet{Version: rdpgfxCapsVersion104, Flags: rdpgfxCapsFlagAVCDisabled}

	if rdpgfxCapabilitySupportsClearCodec(cap8) || rdpgfxCapabilitySupportsProgressive(cap10) || rdpgfxCapabilitySupportsAVC444(cap10) || rdpgfxCapabilitySupportsAVC444v2(cap104) {
		t.Fatal("deferred codec gates should be disabled by default")
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_CLEARCODEC", "1")
	if !rdpgfxCapabilitySupportsClearCodec(cap8) {
		t.Fatal("ClearCodec gate should accept RDPGFX 8.0+ when enabled")
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_PROGRESSIVE_CODEC", "1")
	if rdpgfxCapabilitySupportsProgressive(cap8) || !rdpgfxCapabilitySupportsProgressive(cap10) {
		t.Fatal("Progressive gate should require RDPGFX 10.0+ when enabled")
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_AVC444", "1")
	if rdpgfxCapabilitySupportsAVC444(cap8) || !rdpgfxCapabilitySupportsAVC444(cap10) || rdpgfxCapabilitySupportsAVC444(cap104AVCDisabled) {
		t.Fatal("AVC444 gate should require RDPGFX 10.0+ without AVC_DISABLED")
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_AVC444V2", "1")
	if rdpgfxCapabilitySupportsAVC444v2(cap10) || !rdpgfxCapabilitySupportsAVC444v2(cap104) || rdpgfxCapabilitySupportsAVC444v2(cap104AVCDisabled) {
		t.Fatal("AVC444v2 gate should require RDPGFX 10.4+ without AVC_DISABLED")
	}
}
