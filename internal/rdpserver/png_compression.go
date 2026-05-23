package rdpserver

import (
	"image/png"
	"os"
	"strconv"
	"strings"
)

const defaultPNGCompressionLevel = int(png.DefaultCompression)

func pngCompressionLevelFromEnv() png.CompressionLevel {
	raw := strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_PNG_COMPRESSION_LEVEL"))
	if raw == "" {
		return png.DefaultCompression
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return png.DefaultCompression
	}
	switch png.CompressionLevel(value) {
	case png.NoCompression, png.BestSpeed, png.BestCompression, png.DefaultCompression:
		return png.CompressionLevel(value)
	default:
		return png.DefaultCompression
	}
}
