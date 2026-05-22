package rdpserver

import (
	"os"
	"strings"
)

func nsCodecEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_ENABLE_NSCODEC"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func negotiatedNSCodecID(caps confirmActiveCapabilities) (byte, bool) {
	if !nsCodecEnabledFromEnv() || !caps.BitmapCodecs.Present {
		return 0, false
	}
	return caps.BitmapCodecs.nsCodecID()
}
