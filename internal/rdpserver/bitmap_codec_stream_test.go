package rdpserver

import "testing"

func TestBitmapCodecStreamingEnvGate(t *testing.T) {
	if bitmapCodecStreamingEnabledFromEnv() {
		t.Fatal("bitmap codec streaming should be disabled by default")
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_BITMAP_CODEC_STREAM", "1")
	if !bitmapCodecStreamingEnabledFromEnv() {
		t.Fatal("bitmap codec streaming should be enabled by env")
	}
}
