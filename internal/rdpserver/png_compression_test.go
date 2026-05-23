package rdpserver

import (
	"image/png"
	"testing"
)

func TestPNGCompressionLevelFromEnv(t *testing.T) {
	if got := pngCompressionLevelFromEnv(); got != png.DefaultCompression {
		t.Fatalf("pngCompressionLevelFromEnv default = %d", got)
	}
	for _, value := range []string{"-4", "1", "9", "bad"} {
		t.Setenv("GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL", value)
		if got := pngCompressionLevelFromEnv(); got != png.DefaultCompression {
			t.Fatalf("pngCompressionLevelFromEnv(%q) = %d", value, got)
		}
	}
	t.Setenv("GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL", "-1")
	if got := pngCompressionLevelFromEnv(); got != png.NoCompression {
		t.Fatalf("pngCompressionLevelFromEnv no compression = %d", got)
	}
	t.Setenv("GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL", "-3")
	if got := pngCompressionLevelFromEnv(); got != png.BestCompression {
		t.Fatalf("pngCompressionLevelFromEnv best compression = %d", got)
	}
}
