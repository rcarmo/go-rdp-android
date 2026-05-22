package rdpserver

import (
	"os"
	"strconv"
	"strings"
)

func negotiatedPNGCodecID() (byte, bool) {
	raw := strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_ENABLE_PNG_CODEC_ID"))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseUint(raw, 0, 8)
	if err != nil || value == 0 {
		return 0, false
	}
	return byte(value), true
}
