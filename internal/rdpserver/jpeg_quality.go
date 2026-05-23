package rdpserver

import (
	"os"
	"strconv"
	"strings"
)

const defaultJPEGQuality = 80

func jpegQualityFromEnv() int {
	raw := strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_JPEG_QUALITY"))
	if raw == "" {
		return defaultJPEGQuality
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > 100 {
		return defaultJPEGQuality
	}
	return value
}
