package rdpserver

import (
	"os"
	"strings"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func rdpgfxUncompressedEnabledFromEnv() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("GO_RDP_ANDROID_ENABLE_RDPGFX_UNCOMPRESSED"))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func buildRDPGFXUncompressedFramePDUs(surfaceID uint16, frameID uint32, src frame.Frame, width, height int) ([][]byte, bool) {
	normalized := normalizeFrameForDesktop(src, width, height)
	stride, ok := normalizedFrameStride(normalized)
	if !ok || normalized.Format != frame.PixelFormatRGBA8888 && normalized.Format != frame.PixelFormatBGRA8888 {
		return nil, false
	}
	if normalized.Width <= 0 || normalized.Height <= 0 || normalized.Width > 8192 || normalized.Height > 8192 {
		return nil, false
	}
	maxInt := int(^uint(0) >> 1)
	if normalized.Width > maxInt/4 || normalized.Height > maxInt/(normalized.Width*4) {
		return nil, false
	}
	payload := make([]byte, normalized.Width*normalized.Height*4)
	for y := 0; y < normalized.Height; y++ {
		for x := 0; x < normalized.Width; x++ {
			si := y*stride + x*4
			di := (y*normalized.Width + x) * 4
			switch normalized.Format {
			case frame.PixelFormatBGRA8888:
				payload[di+0] = normalized.Data[si+0]
				payload[di+1] = normalized.Data[si+1]
				payload[di+2] = normalized.Data[si+2]
			case frame.PixelFormatRGBA8888:
				payload[di+0] = normalized.Data[si+2]
				payload[di+1] = normalized.Data[si+1]
				payload[di+2] = normalized.Data[si+0]
			}
			payload[di+3] = 0xff
		}
	}
	return [][]byte{
		buildRDPGFXStartFramePDU(frameID),
		buildRDPGFXWireToSurface1PDU(surfaceID, rdpgfxCodecUncompressed, rdpgfxPixelFormatXRGB8888, 0, 0, uint16(normalized.Width), uint16(normalized.Height), payload), // #nosec G115 dimensions bounded above.
		buildRDPGFXEndFramePDU(frameID),
	}, true
}
