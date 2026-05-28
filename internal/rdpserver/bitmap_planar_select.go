package rdpserver

import (
	"os"
	"strings"
)

func classicBitmapPlanarEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_ENABLE_BITMAP_PLANAR"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
