package rdpserver

import "testing"

func TestPreferredBitmapBPP(t *testing.T) {
	if got := preferredBitmapBPP(confirmActiveCapabilities{}); got != bitmapBPP24 {
		t.Fatalf("default bpp = %d, want 24", got)
	}
	caps := confirmActiveCapabilities{Bitmap: bitmapCapabilityInfo{Present: true, PreferredBitsPerPixel: bitmapBPP16}}
	if got := preferredBitmapBPP(caps); got != bitmapBPP16 {
		t.Fatalf("preferred bpp = %d, want 16", got)
	}
	caps.Bitmap.PreferredBitsPerPixel = 8
	if got := preferredBitmapBPP(caps); got != bitmapBPP8 {
		t.Fatalf("negotiated bpp = %d, want 8", got)
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_BITMAP_BPP", "15")
	if got := preferredBitmapBPP(caps); got != bitmapBPP15 {
		t.Fatalf("forced bpp = %d, want 15", got)
	}
}

func TestBuildFrameBitmapUpdatesForDesktopBPP(t *testing.T) {
	fr := frameOfSolidBGRAForRLETest(2, 1)
	updates, ok := buildFrameBitmapUpdatesForDesktopBPP(fr, nil, false, 2, 1, bitmapBPP16)
	if !ok || len(updates) != 1 {
		t.Fatalf("buildFrameBitmapUpdatesForDesktopBPP ok=%t len=%d", ok, len(updates))
	}
	if got := le16ForTest(updates[0][4+12 : 4+14]); got != bitmapBPP16 {
		t.Fatalf("bpp = %d, want 16", got)
	}
}
