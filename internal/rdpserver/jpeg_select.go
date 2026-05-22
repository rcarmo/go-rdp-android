package rdpserver

import (
	"os"
	"strings"
)

func jpegCodecEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_ENABLE_JPEG_CODEC"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func negotiatedJPEGCodecID(caps confirmActiveCapabilities) (byte, bool) {
	if !jpegCodecEnabledFromEnv() || !caps.BitmapCodecs.Present {
		return 0, false
	}
	return caps.BitmapCodecs.jpegCodecID()
}
