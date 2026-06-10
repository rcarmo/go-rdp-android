package rdpserver

import (
	"os"
	"strings"

	"github.com/rcarmo/go-rdp-android/internal/frame"
	rdpcodec "github.com/rcarmo/go-rdp/pkg/codec"
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
	format, ok := planarPixelFormat(normalized.Format)
	if !ok {
		return nil, false
	}
	pixels, err := rdpcodec.EncodeRDPGFXUncompressed(rdpcodec.BitmapInput{Pixels: normalized.Data, Width: normalized.Width, Height: normalized.Height, Stride: stride, Format: format})
	if err != nil {
		return nil, false
	}
	start, err := rdpcodec.BuildRDPGFXStartFrame(frameID)
	if err != nil {
		return nil, false
	}
	wire, err := rdpcodec.BuildRDPGFXWireToSurface1(surfaceID, rdpgfxCodecUncompressed, rdpgfxPixelFormatXRGB8888, rdpcodec.Rect{Right: uint16(normalized.Width - 1), Bottom: uint16(normalized.Height - 1)}, pixels) // #nosec G115 dimensions bounded above.
	if err != nil {
		return nil, false
	}
	end, err := rdpcodec.BuildRDPGFXEndFrame(frameID)
	if err != nil {
		return nil, false
	}
	return [][]byte{start, wire, end}, true
}
