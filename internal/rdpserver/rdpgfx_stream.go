package rdpserver

import (
	"os"
	"strings"
)

func rdpgfxStreamingEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_ENABLE_RDPGFX_STREAM"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
