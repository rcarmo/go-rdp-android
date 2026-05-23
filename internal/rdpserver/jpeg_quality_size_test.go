package rdpserver

import "testing"

func TestJPEGQualityAffectsPayloadSize(t *testing.T) {
	fr := benchmarkCodecFrame(96, 96)
	low, ok := buildJPEGSurfaceBitsCommand(fr, bitmapCodecJPEGDefaultID, 30)
	if !ok {
		t.Fatal("buildJPEGSurfaceBitsCommand low quality ok = false")
	}
	high, ok := buildJPEGSurfaceBitsCommand(fr, bitmapCodecJPEGDefaultID, 95)
	if !ok {
		t.Fatal("buildJPEGSurfaceBitsCommand high quality ok = false")
	}
	if len(high) <= len(low) {
		t.Fatalf("high quality JPEG size = %d, want greater than low quality %d", len(high), len(low))
	}
	if len(high) >= fr.Width*fr.Height*4 {
		t.Fatalf("high quality JPEG size = %d, want less than raw %d", len(high), fr.Width*fr.Height*4)
	}
}
