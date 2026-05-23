package rdpserver

import "testing"

func TestJPEGQualityFromEnv(t *testing.T) {
	if got := jpegQualityFromEnv(); got != defaultJPEGQuality {
		t.Fatalf("jpegQualityFromEnv default = %d", got)
	}
	for _, value := range []string{"0", "101", "bad", "-1"} {
		t.Setenv("GO_RDP_ANDROID_JPEG_QUALITY", value)
		if got := jpegQualityFromEnv(); got != defaultJPEGQuality {
			t.Fatalf("jpegQualityFromEnv(%q) = %d", value, got)
		}
	}
	t.Setenv("GO_RDP_ANDROID_JPEG_QUALITY", "42")
	if got := jpegQualityFromEnv(); got != 42 {
		t.Fatalf("jpegQualityFromEnv valid = %d", got)
	}
}
