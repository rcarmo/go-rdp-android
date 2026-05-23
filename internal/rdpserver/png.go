package rdpserver

import (
	"bytes"
	"image/png"

	"github.com/rcarmo/go-rdp-android/internal/frame"
)

func buildPNGSurfaceBitsCommand(src frame.Frame, codecID byte) ([]byte, bool) {
	if codecID == 0 {
		return nil, false
	}
	img, ok := frameToRGBAImage(src)
	if !ok {
		return nil, false
	}
	var buf bytes.Buffer
	encoder := png.Encoder{CompressionLevel: pngCompressionLevelFromEnv()}
	if err := encoder.Encode(&buf, img); err != nil {
		return nil, false
	}
	encoded := buf.Bytes()
	if len(encoded) == 0 || len(encoded) > rdpgfxMaxPDUSize {
		return nil, false
	}
	return buildSurfaceBitsCommand(src.Width, src.Height, codecID, encoded)
}
