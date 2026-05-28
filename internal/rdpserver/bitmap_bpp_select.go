package rdpserver

import (
	"os"
	"strconv"
	"strings"
)

func preferredBitmapBPP(caps confirmActiveCapabilities) uint16 {
	if forced, ok := forcedBitmapBPPFromEnv(); ok {
		return forced
	}
	if caps.Bitmap.Present {
		switch caps.Bitmap.PreferredBitsPerPixel {
		case bitmapBPP8, bitmapBPP15, bitmapBPP16, bitmapBPP24:
			return caps.Bitmap.PreferredBitsPerPixel
		}
	}
	return bitmapBPP24
}

func forcedBitmapBPPFromEnv() (uint16, bool) {
	raw := strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_ENABLE_BITMAP_BPP"))
	if raw == "" {
		return 0, false
	}
	value, err := strconv.ParseUint(raw, 0, 16)
	if err != nil {
		return 0, false
	}
	switch uint16(value) {
	case bitmapBPP8, bitmapBPP15, bitmapBPP16, bitmapBPP24:
		return uint16(value), true
	default:
		return 0, false
	}
}
