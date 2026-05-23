package rdpserver

import "testing"

func TestPNGCompressionLevelAffectsPayloadSize(t *testing.T) {
	fr := solidCodecFrame(96, 96, [4]byte{0x33, 0x66, 0x99, 0xff})
	t.Setenv("GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL", "-1")
	plain, ok := buildPNGSurfaceBitsCommand(fr, 10)
	if !ok {
		t.Fatal("buildPNGSurfaceBitsCommand no compression ok = false")
	}
	t.Setenv("GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL", "-3")
	compressed, ok := buildPNGSurfaceBitsCommand(fr, 10)
	if !ok {
		t.Fatal("buildPNGSurfaceBitsCommand best compression ok = false")
	}
	if len(compressed) >= len(plain) {
		t.Fatalf("best-compression PNG size = %d, want less than no-compression size %d", len(compressed), len(plain))
	}
	if len(compressed) >= fr.Width*fr.Height*4 {
		t.Fatalf("best-compression PNG size = %d, want less than raw %d", len(compressed), fr.Width*fr.Height*4)
	}
}
