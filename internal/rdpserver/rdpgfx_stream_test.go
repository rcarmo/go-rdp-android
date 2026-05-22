package rdpserver

import "testing"

func TestRDPGFXStreamingEnvGate(t *testing.T) {
	if rdpgfxStreamingEnabledFromEnv() {
		t.Fatal("RDPGFX streaming should be disabled by default until client soak evidence is expanded")
	}
	t.Setenv("GO_RDP_ANDROID_ENABLE_RDPGFX_STREAM", "1")
	if !rdpgfxStreamingEnabledFromEnv() {
		t.Fatal("RDPGFX streaming should be enabled by env")
	}
}
