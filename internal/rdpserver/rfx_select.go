package rdpserver

import (
	"os"
	"strings"
)

func remoteFXEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_ENABLE_RFX_CODEC"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func negotiatedRemoteFXCodecID(caps confirmActiveCapabilities) (byte, bool) {
	if !remoteFXEnabledFromEnv() || !caps.BitmapCodecs.Present {
		return 0, false
	}
	if id, ok := caps.BitmapCodecs.remoteFXCodecID(); ok {
		return id, true
	}
	return caps.BitmapCodecs.remoteFXImageCodecID()
}
